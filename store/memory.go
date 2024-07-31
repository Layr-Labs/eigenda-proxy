package store

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/Layr-Labs/eigenda-proxy/utils"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/Layr-Labs/eigenda-proxy/verify"
	"github.com/Layr-Labs/eigenda/api/clients/codecs"
	"github.com/Layr-Labs/eigenda/api/grpc/common"
	"github.com/Layr-Labs/eigenda/api/grpc/disperser"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	MemStoreFlagName   = "memstore.enabled"
	ExpirationFlagName = "memstore.expiration"
	FaultFlagName      = "memstore.fault-config"

	DefaultPruneInterval = 500 * time.Millisecond
)

type MemStoreConfig struct {
	Enabled              bool
	BlobExpiration       time.Duration
	EthConfirmationDepth uint64
	FaultCfgPath         string
	FaultCfg             *FaultConfig
}

// MemStore is a simple in-memory store for blobs which uses an expiration
// time to evict blobs to best emulate the ephemeral nature of blobs dispersed to
// EigenDA operators.
type MemStore struct {
	sync.RWMutex
	faultCfg *FaultConfig

	l         log.Logger
	keyStarts map[string]time.Time
	store     map[string][]byte
	verifier  *verify.Verifier
	codec     codecs.BlobCodec

	maxBlobSizeBytes uint64
	blobExpiration   time.Duration
	reads            *utils.AtomicInt64
}

var _ Store = (*MemStore)(nil)

// NewMemStore ... constructor
func NewMemStore(ctx context.Context, verifier *verify.Verifier, l log.Logger, maxBlobSizeBytes uint64, blobExpiration time.Duration, fc *FaultConfig) (*MemStore, error) {
	store := &MemStore{
		faultCfg:         fc,
		l:                l,
		keyStarts:        make(map[string]time.Time),
		store:            make(map[string][]byte),
		verifier:         verifier,
		codec:            codecs.NewIFFTCodec(codecs.NewDefaultBlobCodec()),
		maxBlobSizeBytes: maxBlobSizeBytes,
		blobExpiration:   blobExpiration,
		reads: 		  utils.NewAtomicInt64(0),
	}

	if store.blobExpiration != 0 {
		go store.EventLoop(ctx)
	}

	return store, nil
}

func (e *MemStore) SetFaultConfig(fc *FaultConfig) {
	e.Lock()
	defer e.Unlock()
	e.faultCfg = fc
}

func (e *MemStore) EventLoop(ctx context.Context) {
	timer := time.NewTicker(DefaultPruneInterval)

	for {
		select {
		case <-ctx.Done():
			return

		case <-timer.C:
			e.l.Debug("pruning expired blobs")
			e.pruneExpired()
		}
	}
}

func (e *MemStore) pruneExpired() {
	e.Lock()
	defer e.Unlock()

	for cert, dur := range e.keyStarts {
		if time.Since(dur) >= e.blobExpiration {
			delete(e.keyStarts, cert)
			delete(e.store, cert)

			e.l.Info("blob expired and pruned from RAM", "key", fmt.Sprintf("%x", cert))
		}
	}

}

// Get fetches a value from the store.
func (e *MemStore) Get(ctx context.Context, commit []byte) ([]byte, error) {
	e.RLock()
	defer func(){
		e.reads.Increment()
		e.RUnlock()
	}()

	behavior, err := e.GetReturnBehavior(ctx)
	if err != nil {
		return nil, err
	}

	// iFFT or generic encoded blob
	encodedBlob, err := e.fetch(commit)
	if err != nil {
		return nil, err
	}
	decodedBlob, err := e.codec.DecodeBlob(encodedBlob)
	if err != nil {
		return nil, err
	}

	switch behavior.Mode {
		case Honest:
			return decodedBlob, nil
		case Byzantine:
			return e.corruptBlob(decodedBlob), nil

		case IntervalByzantine:
			if (e.reads.ValueUnsignedInt() >= behavior.Interval) && (e.reads.ValueUnsignedInt() % behavior.Interval == 0) {
				return e.corruptBlob(decodedBlob), nil
			}
			return decodedBlob, nil

		default:
			return nil, fmt.Errorf("unknown fault mode")
	}
}

// Put inserts a value into the store.
func (e *MemStore) Put(ctx context.Context, value []byte) ([]byte, error) {
	if uint64(len(value)) > e.maxBlobSizeBytes {
		return nil, fmt.Errorf("blob is larger than max blob size: blob length %d, max blob size %d", len(value), e.maxBlobSizeBytes)
	}

	e.Lock()
	defer e.Unlock()

	encodedVal, err := e.codec.EncodeBlob(value)
	if err != nil {
		return nil, err
	}

	commitment, err := e.verifier.Commit(encodedVal)
	if err != nil {
		return nil, err
	}

	// generate batch header hash
	// TODO: generate a real batch header hash based on the randomly generated batch header fields
	// this will be useful in integrations like nitro where the batch header hash isn't persisted to the inbox tx calldata
	// requiring any actor running canonical derivation to recompute it when querying a blob
	entropy := make([]byte, 10)
	_, err = rand.Read(entropy)
	if err != nil {
		return nil, err
	}
	mockBatchRoot := crypto.Keccak256Hash(entropy)
	blockNum, _ := rand.Int(rand.Reader, big.NewInt(1000))

	num := uint32(blockNum.Uint64())

	cert := &verify.Certificate{
		BlobHeader: &disperser.BlobHeader{
			Commitment: &common.G1Commitment{
				X: commitment.X.Marshal(),
				Y: commitment.Y.Marshal(),
			},
			DataLength: uint32(len(encodedVal)),
			BlobQuorumParams: []*disperser.BlobQuorumParam{
				{
					QuorumNumber:                    1,
					AdversaryThresholdPercentage:    29,
					ConfirmationThresholdPercentage: 30,
					ChunkLength:                     300,
				},
			},
		},
		BlobVerificationProof: &disperser.BlobVerificationProof{
			BatchMetadata: &disperser.BatchMetadata{
				BatchHeader: &disperser.BatchHeader{
					BatchRoot:               mockBatchRoot[:],
					QuorumNumbers:           []byte{0x1, 0x0},
					QuorumSignedPercentages: []byte{0x60, 0x90},
					ReferenceBlockNumber:    num,
				},
				SignatoryRecordHash:     mockBatchRoot[:],
				Fee:                     []byte{},
				ConfirmationBlockNumber: num,
				BatchHeaderHash:         []byte{},
			},
			BatchId:        69,
			BlobIndex:      420,
			InclusionProof: entropy,
			QuorumIndexes:  []byte{0x1, 0x0},
		},
	}

	certBytes, err := rlp.EncodeToBytes(cert)
	if err != nil {
		return nil, err
	}
	// construct key
	bytesKeys := cert.BlobVerificationProof.InclusionProof
	certStr := string(bytesKeys)

	if _, exists := e.store[certStr]; exists {
		return nil, fmt.Errorf("commitment key already exists")
	}

	e.store[certStr] = encodedVal
	// add expiration
	e.keyStarts[certStr] = time.Now()

	return certBytes, nil
}

// fetch reads the actual data from the store
func (e *MemStore) fetch(commit []byte) ([]byte, error) {
	var cert verify.Certificate
	err := rlp.DecodeBytes(commit, &cert)
	if err != nil {
		return nil, fmt.Errorf("failed to decode DA cert to RLP format: %w", err)
	}

	bytesKeys := cert.BlobVerificationProof.InclusionProof

	var encodedBlob []byte
	var exists bool
	if encodedBlob, exists = e.store[string(bytesKeys)]; !exists {
		return nil, fmt.Errorf("commitment key not found")
	}
	
	return encodedBlob, nil
}

func (e *MemStore) GetReturnBehavior(ctx context.Context) (*Behavior, error){
	if e.faultCfg == nil {
		return &Behavior{
			Mode: Honest,
		}, nil
	}

	actor, ok := ctx.Value("actor").(string)
	if !ok {
		actor = ""
	}

	if actor == "" && !e.faultCfg.AllPolicyExists() {
		return nil, fmt.Errorf("actor not found in context nor is `all` policy configured")
	}

	if actor == "" {
		actor = AllPolicyKey
	}

	 behavior, exists := e.faultCfg.Actors[actor]
	 
	 if !exists {
		return nil, fmt.Errorf("actor not found in fault config")
	}

	return &behavior, nil
}

func (e *MemStore) corruptBlob(b []byte) []byte {
		// flip 3 bits in the blob to corrupt the original data
		b[0] = ^b[0]
		mid := len(b) / 2
		b[mid] = ^b[mid]
		end := len(b) - 1
		b[end] = ^b[end]
		return b
}


func (e *MemStore) Stats() *Stats {
	e.RLock()
	defer e.RUnlock()
	return &Stats{
		Entries: uint(len(e.store)),
		Reads:   e.reads.ValueUnsignedInt(),
	}
}
