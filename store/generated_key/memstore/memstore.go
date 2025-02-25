package memstore

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/rlp"

	"github.com/Layr-Labs/eigenda-proxy/common"
	"github.com/Layr-Labs/eigenda-proxy/store/generated_key/memstore/ephemeral_db"
	"github.com/Layr-Labs/eigenda-proxy/store/generated_key/memstore/memconfig"
	"github.com/Layr-Labs/eigenda-proxy/verify/v1"
	"github.com/Layr-Labs/eigenda/api/clients/codecs"
	eigenda_common "github.com/Layr-Labs/eigenda/api/grpc/common"
	"github.com/Layr-Labs/eigenda/api/grpc/disperser"
	"github.com/Layr-Labs/eigensdk-go/logging"
	"github.com/ethereum/go-ethereum/crypto"
)

func randomBytes(size uint) []byte {
	entropy := make([]byte, size)
	_, _ = rand.Read(entropy)
	return entropy
}

const (
	BytesPerFieldElement = 32
)

/*
MemStore is a simple in-memory store for blobs which uses an expiration
time to evict blobs to best emulate the ephemeral nature of blobs dispersed to
EigenDA V1 operators.
*/
type MemStore struct {
	*ephemeral_db.DB
	log logging.Logger

	// We only use the verifier for kzgCommitment verification.
	// MemStore generates random certs which can't be verified.
	// TODO: we should probably refactor the Verifier to be able to only take in a BlobVerifier here.
	verifier *verify.Verifier
	codec    codecs.BlobCodec
}

var _ common.GeneratedKeyStore = (*MemStore)(nil)

// New ... constructor
func New(
	ctx context.Context, verifier *verify.Verifier, log logging.Logger, config *memconfig.SafeConfig,
) (*MemStore, error) {
	return &MemStore{
		ephemeral_db.New(ctx, config, log),
		log,
		verifier,
		codecs.NewIFFTCodec(codecs.NewDefaultBlobCodec()),
	}, nil
}

// Get fetches a value from the store.
func (e *MemStore) Get(_ context.Context, commit []byte) ([]byte, error) {
	encodedBlob, err := e.FetchEntry(crypto.Keccak256Hash(commit).Bytes())
	if err != nil {
		return nil, fmt.Errorf("fetching entry via v1 memstore: %w", err)
	}

	return e.codec.DecodeBlob(encodedBlob)
}

// Put inserts a value into the store.
func (e *MemStore) Put(_ context.Context, value []byte) ([]byte, error) {
	encodedVal, err := e.codec.EncodeBlob(value)
	if err != nil {
		return nil, err
	}

	commitment, err := e.verifier.Commit(encodedVal)
	if err != nil {
		return nil, err
	}

	entropy := randomBytes(10)
	mockBatchRoot := crypto.Keccak256Hash(entropy)
	blockNum, _ := rand.Int(rand.Reader, big.NewInt(1000))

	num := uint32(blockNum.Uint64()) // #nosec G115

	cert := &verify.Certificate{
		BlobHeader: &disperser.BlobHeader{
			Commitment: &eigenda_common.G1Commitment{
				X: commitment.X.Marshal(),
				Y: commitment.Y.Marshal(),
			},
			DataLength: uint32((len(encodedVal) + BytesPerFieldElement - 1) / BytesPerFieldElement), // #nosec G115
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

	certKey := crypto.Keccak256Hash(certBytes).Bytes()

	err = e.InsertEntry(certKey, encodedVal)
	if err != nil {
		return nil, err
	}

	return certBytes, nil
}

func (e *MemStore) Verify(_ context.Context, _, _ []byte) error {
	return nil
}

func (e *MemStore) BackendType() common.BackendType {
	return common.MemstoreV1BackendType
}
