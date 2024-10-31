package eigenda

import (
	"context"
	"fmt"
	"time"

	"github.com/Layr-Labs/eigenda-proxy/store"
	"github.com/Layr-Labs/eigenda-proxy/verify"
	"github.com/Layr-Labs/eigenda/api/clients"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
)

type StoreConfig struct {
	MaxBlobSizeBytes uint64
	// the # of Ethereum blocks to wait after the EigenDA L1BlockReference # before attempting to verify
	// & accredit a blob
	EthConfirmationDepth uint64

	// total duration time that client waits for blob to confirm
	StatusQueryTimeout time.Duration
}

// Store does storage interactions and verifications for blobs with DA.
type Store struct {
	client   *clients.EigenDAClient
	verifier *verify.Verifier
	cfg      *StoreConfig
	log      log.Logger
}

var _ store.GeneratedKeyStore = (*Store)(nil)

func NewStore(client *clients.EigenDAClient,
	v *verify.Verifier, log log.Logger, cfg *StoreConfig) (*Store, error) {
	return &Store{
		client:   client,
		verifier: v,
		log:      log,
		cfg:      cfg,
	}, nil
}

// Get fetches a blob from DA using certificate fields and verifies blob
// against commitment to ensure data is valid and non-tampered.
func (e Store) Get(ctx context.Context, key []byte) ([]byte, error) {
	var cert verify.Certificate
	err := rlp.DecodeBytes(key, &cert)
	if err != nil {
		return nil, fmt.Errorf("failed to decode DA cert to RLP format: %w", err)
	}

	decodedBlob, err := e.client.GetBlob(ctx, cert.BlobVerificationProof.BatchMetadata.BatchHeaderHash, cert.BlobVerificationProof.BlobIndex)
	if err != nil {
		return nil, fmt.Errorf("EigenDA client failed to retrieve decoded blob: %w", err)
	}

	return decodedBlob, nil
}

// Put disperses a blob for some pre-image and returns the associated RLP encoded certificate commit.
func (e Store) Put(ctx context.Context, value []byte) ([]byte, error) {
	encodedBlob, err := e.client.GetCodec().EncodeBlob(value)
	if err != nil {
		return nil, fmt.Errorf("EigenDA client failed to re-encode blob: %w", err)
	}
	// WVM: check that the data is lower than 100kb - Set it in configs via proxy config
	// TODO: We should move this length check inside PutBlob
	if uint64(len(encodedBlob)) > e.cfg.MaxBlobSizeBytes {
		return nil, fmt.Errorf("%w: blob length %d, max blob size %d", store.ErrProxyOversizedBlob, len(value), e.cfg.MaxBlobSizeBytes)
	}

	blobInfo, err := e.client.PutBlob(ctx, value)
	if err != nil {
		// TODO: we will want to filter for errors here and return a 503 when needed
		// ie when dispersal itself failed, or that we timed out waiting for batch to land onchain
		return nil, err
	}
	cert := (*verify.Certificate)(blobInfo)

	err = e.verifier.VerifyCommitment(cert.BlobHeader.Commitment, encodedBlob)
	if err != nil {
		return nil, fmt.Errorf("failed to verify commitment: %w", err)
	}

	err = e.verifier.VerifyCert(ctx, cert)
	if err != nil {
		return nil, fmt.Errorf("failed to verify DA cert: %w", err)
	}

	// WVM: we store the encoded blob in wvm
	// 	err = e.wvmClient.Store(ctx, cert, encodedBlob)
	// 	if err != nil {
	// 		return nil, err
	// 	}

	bytes, err := rlp.EncodeToBytes(cert)
	if err != nil {
		return nil, fmt.Errorf("failed to encode DA cert to RLP format: %w", err)
	}

	return bytes, nil
}

// Entries are a no-op for EigenDA Store
func (e Store) Stats() *store.Stats {
	return nil
}

// Backend returns the backend type for EigenDA Store
func (e Store) BackendType() store.BackendType {
	return store.EigenDABackendType
}

// Key is used to recover certificate fields and that verifies blob
// against commitment to ensure data is valid and non-tampered.
func (e Store) Verify(ctx context.Context, key []byte, value []byte) error {
	var cert verify.Certificate
	err := rlp.DecodeBytes(key, &cert)
	if err != nil {
		return fmt.Errorf("failed to decode DA cert to RLP format: %w", err)
	}

	// re-encode blob for verification
	encodedBlob, err := e.client.GetCodec().EncodeBlob(value)
	if err != nil {
		return fmt.Errorf("EigenDA client failed to re-encode blob: %w", err)
	}

	// verify kzg data commitment
	err = e.verifier.VerifyCommitment(cert.BlobHeader.Commitment, encodedBlob)
	if err != nil {
		return fmt.Errorf("failed to verify commitment: %w", err)
	}

	// verify DA certificate against EigenDA's batch metadata that's bridged to Ethereum
	return e.verifier.VerifyCert(ctx, &cert)
}

// GetWvmTxHashByCommitment uses commitment to get wvm tx hash from the internal map(temprorary hack)
// and returns it to the caller

/* TODO: make it nice
func (e EigenDAStore) GetWvmTxHashByCommitment(ctx context.Context, key []byte) (string, error) {
	e.log.Info("try get wvm tx hash using provided commitment")
	var cert verify.Certificate
	err := rlp.DecodeBytes(key, &cert)
	if err != nil {
		return "", fmt.Errorf("failed to decode DA cert to RLP format: %w", err)
	}

	wvmTxHash, err := e.wvmClient.GetWvmTxHashByCommitment(ctx, &cert)
	if err != nil {
		return "", err
	}

	return wvmTxHash, nil
}

func (e EigenDAStore) GetBlobFromWvm(ctx context.Context, key []byte) ([]byte, error) {
	var cert verify.Certificate
	err := rlp.DecodeBytes(key, &cert)
	if err != nil {
		return nil, fmt.Errorf("failed to decode DA cert to RLP format: %w", err)
	}

	wvmTxHash, err := e.wvmClient.GetWvmTxHashByCommitment(ctx, &cert)
	if err != nil {
		return nil, err
	}

	e.log.Info("found wvm tx hash using provided commitment", "provided key", commitmentKey(cert.BlobVerificationProof.BatchId, cert.BlobVerificationProof.BlobIndex))

	wvmDecodedBlob, err := e.wvmClient.GetBlobFromWvm(ctx, wvmTxHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get eigenda blob from wvm: %w", err)
	}

	decodedData, err := e.client.Codec.DecodeBlob(wvmDecodedBlob)
	if err != nil {
		return nil, fmt.Errorf("error decoding eigen blob: %w", err)
	}

	return decodedData, nil
}
*/
