package ephemeraldb

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Layr-Labs/eigenda-proxy/common/proxyerrors"
	"github.com/Layr-Labs/eigenda-proxy/store/generated_key/memstore/memconfig"
	eigenda "github.com/Layr-Labs/eigenda-proxy/store/generated_key/v2"
	"github.com/Layr-Labs/eigenda/api"
	"github.com/Layr-Labs/eigenda/api/clients/v2/coretypes"
	"github.com/Layr-Labs/eigenda/api/clients/v2/verification"
	"github.com/Layr-Labs/eigensdk-go/logging"
)

const (
	DefaultPruneInterval = 500 * time.Millisecond
)

// DB ... An ephemeral && simple in-memory database used to emulate
// an EigenDA network for dispersal/retrieval operations.
type DB struct {
	// knobs used to express artificial conditions for testing
	config *memconfig.SafeConfig
	log    logging.Logger

	// mu guards the below fields
	mu                             sync.RWMutex
	keyStarts                      map[string]time.Time // used for managing expiration
	store                          map[string][]byte    // db
	instructedNonSuccessStatusCode map[string]int       // a map storing keys with instructed status code
}

// New ... constructor
func New(ctx context.Context, cfg *memconfig.SafeConfig, log logging.Logger) *DB {
	db := &DB{
		config:                         cfg,
		keyStarts:                      make(map[string]time.Time),
		store:                          make(map[string][]byte),
		instructedNonSuccessStatusCode: make(map[string]int),
		log:                            log,
	}

	// if no expiration set then blobs will be persisted indefinitely
	if cfg.BlobExpiration() != 0 {
		db.log.Info("ephemeral db expiration enabled for payload entries.", "time", cfg.BlobExpiration)
		go db.pruningLoop(ctx)
	}

	return db
}

// InsertEntry ... inserts a value into the db provided a key
func (db *DB) InsertEntry(key []byte, value []byte) error {
	if db.config.PutReturnsFailoverError() {
		return api.NewErrorFailover(errors.New("ephemeral db in failover simulation mode"))
	}
	if uint64(len(value)) > db.config.MaxBlobSizeBytes() {
		return fmt.Errorf(
			"%w: blob length %d, max blob size %d",
			proxyerrors.ErrProxyOversizedBlob,
			len(value),
			db.config.MaxBlobSizeBytes())
	}

	time.Sleep(db.config.LatencyPUTRoute())
	db.mu.Lock()
	defer db.mu.Unlock()

	strKey := string(key)

	activated, statusCode := db.config.GetInstructedMode()
	// if isActivatedNormalOps, the data is stored in
	isActivatedNormalOps := activated && statusCode == int(coretypes.StatusSuccess)

	if !activated || isActivatedNormalOps {
		_, exists := db.store[strKey]
		if exists {
			return fmt.Errorf("payload key already exists in ephemeral db: %s", strKey)
		}

		db.store[strKey] = value

		// add expiration if applicable
		if db.config.BlobExpiration() > 0 {
			db.keyStarts[strKey] = time.Now()
		}
	} else {
		db.log.Info("InsertEntry to memstore with special status code", "statusCode", statusCode)
		_, exists := db.instructedNonSuccessStatusCode[strKey]
		if exists {
			return fmt.Errorf("memstore returns instructed status code, but key already exists: %s", strKey)
		}
		// not BlobExpiration
		db.instructedNonSuccessStatusCode[strKey] = statusCode
	}

	// write always succeed even instructed to return error on Get
	return nil
}

// FetchEntry ... looks up a value from the db provided a key
func (db *DB) FetchEntry(key []byte) ([]byte, error) {
	time.Sleep(db.config.LatencyGETRoute())
	db.mu.RLock()
	defer db.mu.RUnlock()

	statusCode, instructedExists := db.instructedNonSuccessStatusCode[string(key)]
	if instructedExists {
		// should have been defended in the SET http router path
		if statusCode < -1 || statusCode > int(coretypes.StatusRequiredQuorumsNotSubset) {
			panic("memstore is configured to return an unknown status code in FetchEntry. Unable to serve the get")
		}

		if statusCode == -1 {
			return nil, eigenda.RBNRecencyCheckFailedError{}
		}

		// the error msg the retriever would receive on the get path
		err := verification.CertVerificationFailedError{
			// #nosec G115 - statusCode already checked within range
			StatusCode: coretypes.VerificationStatusCode(uint8(statusCode)),
			Msg:        fmt.Sprintf("cert verification failed: status code (%d)", statusCode),
		}

		return nil, &err
	}

	payload, exists := db.store[string(key)]

	if !exists {
		return nil, fmt.Errorf("payload not found for key: %s", string(key))
	}

	return payload, nil
}

// pruningLoop ... runs a background goroutine to prune expired blobs from the store on a regular interval.
func (db *DB) pruningLoop(ctx context.Context) {
	timer := time.NewTicker(DefaultPruneInterval)

	for {
		select {
		case <-ctx.Done():
			return

		case <-timer.C:
			db.pruneExpired()
		}
	}
}

// pruneExpired ... removes expired blobs from the store based on the expiration time.
func (db *DB) pruneExpired() {
	db.mu.Lock()
	defer db.mu.Unlock()

	for commit, dur := range db.keyStarts {
		if time.Since(dur) >= db.config.BlobExpiration() {
			delete(db.keyStarts, commit)
			delete(db.store, commit)

			db.log.Debug("blob pruned", "commit", commit)
		}
	}
}
