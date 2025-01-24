package eigenda_v2

import (
	"context"
	"fmt"
	"time"

	"github.com/Layr-Labs/eigenda-proxy/common"
	"github.com/Layr-Labs/eigenda-proxy/verify/v2"

	"github.com/ethereum/go-ethereum/log"
)

type StoreConfig struct {
	MaxBlobSizeBytes uint64

	// total duration time that client waits for blob to confirm
	StatusQueryTimeout time.Duration

	// number of times to retry eigenda blob dispersals
	PutRetries uint
}

// Store does storage interactions and verifications for blobs with the
// EigenDA V2 protocol.
type Store struct {
	// TODO: disperserClient, retrieverClient usage
	verifier *verify.Verifier
	cfg      *StoreConfig
	log      log.Logger
}

var _ common.GeneratedKeyStore = (*Store)(nil)

func NewStore(v *verify.Verifier, log log.Logger, cfg *StoreConfig) (*Store, error) {
	return &Store{
		verifier: v,
		log:      log,
		cfg:      cfg,
	}, nil
}

// Get fetches a blob from DA using certificate fields and verifies blob
// against commitment to ensure data is valid and non-tampered.
func (e Store) Get(ctx context.Context, key []byte) ([]byte, error) {
	return nil, fmt.Errorf("unimplemented")
}

// Put disperses a blob for some pre-image and returns the associated RLP encoded certificate commit.
// TODO: Client polling for different status codes
//
//	Mapping status codes to 503 failover
func (e Store) Put(ctx context.Context, value []byte) ([]byte, error) {
	return nil, fmt.Errorf("unimplemented")
}

// Backend returns the backend type for EigenDA Store
func (e Store) BackendType() common.BackendType {
	return common.EigenDAV2BackendType
}

// Key is used to recover certificate fields and that verifies blob
// against commitment to ensure data is valid and non-tampered.
// TODO: develop segmented routes for read/write;
// commitment generation should only happen again when reading and not writing
func (e Store) Verify(ctx context.Context, key []byte, value []byte) error {
	return fmt.Errorf("unimplemented")
}
