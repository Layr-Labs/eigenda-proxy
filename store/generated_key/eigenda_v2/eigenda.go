package eigenda_v2

import (
	"context"
	"fmt"
	"time"

	"github.com/Layr-Labs/eigenda-proxy/common"
	"github.com/avast/retry-go/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Layr-Labs/eigenda/api/clients/v2"
	"github.com/Layr-Labs/eigenda/api/clients/v2/verification"
	eigenda_common "github.com/Layr-Labs/eigenda/common"
	v2 "github.com/Layr-Labs/eigenda/core/v2"
	"github.com/Layr-Labs/eigenda/encoding"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
)

type Config struct {
	// address of service manager - used for loading chain state
	ServiceManagerAddr string

	MaxBlobSizeBytes uint64

	// total duration time that client waits for blob to confirm
	StatusQueryTimeout time.Duration

	// number of times to retry eigenda blob dispersals
	PutRetries uint
}

// Store does storage interactions and verifications for blobs with the
// EigenDA V2 protocol.
type Store struct {
	cfg *Config
	log log.Logger

	disperser *clients.PayloadDisperser
	retriever *clients.PayloadRetriever
	verifier  verification.ICertVerifier
}

var _ common.GeneratedKeyStore = (*Store)(nil)

func NewStore(log log.Logger, cfg *Config, ethClient eigenda_common.EthClient,
	disperser *clients.PayloadDisperser, retriever *clients.PayloadRetriever, verifier verification.ICertVerifier) (*Store, error) {

	return &Store{
		log:       log,
		cfg:       cfg,
		disperser: disperser,
		retriever: retriever,
		verifier:  verifier,
	}, nil
}

// ComputeBlobKey computes the BlobKey of the blob that belongs to the EigenDACert
// Temporarily stolen from https://github.com/Layr-Labs/eigenda/pull/1187
func computeBlobKey(c *verification.EigenDACert) (*v2.BlobKey, error) {
	blobHeader := c.BlobInclusionInfo.BlobCertificate.BlobHeader

	blobCommitmentsProto := verification.BlobCommitmentBindingToProto(&blobHeader.Commitment)
	blobCommitments, err := encoding.BlobCommitmentsFromProtobuf(blobCommitmentsProto)
	if err != nil {
		return nil, fmt.Errorf("blob commitments from protobuf: %w", err)
	}

	blobKeyBytes, err := v2.ComputeBlobKey(
		blobHeader.Version,
		*blobCommitments,
		blobHeader.QuorumNumbers,
		blobHeader.PaymentHeaderHash,
		blobHeader.Salt)

	if err != nil {
		return nil, fmt.Errorf("compute blob key: %w", err)
	}

	blobKey, err := v2.BytesToBlobKey(blobKeyBytes[:])
	if err != nil {
		return nil, fmt.Errorf("bytes to blob key: %w", err)
	}

	return &blobKey, nil
}

// Get fetches a blob from DA using certificate fields and verifies blob
// against commitment to ensure data is valid and non-tampered.
func (e Store) Get(ctx context.Context, key []byte) ([]byte, error) {
	var cert verification.EigenDACert
	err := rlp.DecodeBytes(key, cert)
	if err != nil {
		return nil, fmt.Errorf("RLP decoding EigenDA v2 cert: %w", err)
	}

	// TODO: Update this to use changes from
	// https://github.com/Layr-Labs/eigenda/pull/1187 once merged
	blobKey, err := computeBlobKey(&cert)
	if err != nil {
		return nil, fmt.Errorf("computing blob key from cert: %w", err)
	}

	payload, err := e.retriever.GetPayload(ctx, *blobKey, &cert)
	if err != nil {
		return nil, fmt.Errorf("getting payload: %w", err)
	}

	return payload, nil
}

// Put disperses a blob for some pre-image and returns the associated RLP encoded certificate commit.
// TODO: Client polling for different status codes
//
//	Mapping status codes to 503 failover
func (e Store) Put(ctx context.Context, value []byte) ([]byte, error) {
	// salt := 0
	log.Info("Put EigenDA V2 backend")

	// TODO: Verify this retry or failover code for correctness against V2
	// protocol

	// We attempt to disperse the blob to EigenDA up to 3 times, unless we get a 400 error on any attempt.
	cert, err := retry.DoWithData(
		func() (*verification.EigenDACert, error) {
			// TODO: Figure out salt mgmt
			return e.disperser.SendPayload(ctx, value, 0)
		},
		retry.RetryIf(func(err error) bool {
			st, isGRPCError := status.FromError(err)
			if !isGRPCError {
				// api.ErrorFailover is returned, so we should retry
				return true
			}
			//nolint:exhaustive // we only care about a few grpc error codes
			switch st.Code() {
			case codes.InvalidArgument:
				// we don't retry 400 errors because there is no point,
				// we are passing invalid data
				return false
			case codes.ResourceExhausted:
				// we retry on 429s because *can* mean we are being rate limited
				// we sleep 1 second... very arbitrarily, because we don't have more info.
				// grpc error itself should return a backoff time,
				// see https://github.com/Layr-Labs/eigenda/issues/845 for more details
				time.Sleep(1 * time.Second)
				return true
			default:
				return true
			}
		}),
		// only return the last error. If it is an api.ErrorFailover, then the handler will convert
		// it to an http 503 to signify to the client (batcher) to failover to ethda
		// b/c eigenda is temporarily down.
		retry.LastErrorOnly(true),
		retry.Attempts(e.cfg.PutRetries),
	)
	if err != nil {
		// TODO: we will want to filter for errors here and return a 503 when needed
		// ie when dispersal itself failed, or that we timed out waiting for batch to land onchain
		return nil, err
	}

	return rlp.EncodeToBytes(cert)
}

// Backend returns the backend type for EigenDA Store
func (e Store) BackendType() common.BackendType {
	return common.EigenDAV2BackendType
}

// Key is used to recover certificate fields and that verifies blob
// against commitment to ensure data is valid and non-tampered.
func (e Store) Verify(ctx context.Context, key []byte, value []byte) error {
	var cert verification.EigenDACert
	err := rlp.DecodeBytes(key, cert)
	if err != nil {
		return fmt.Errorf("RLP decoding EigenDA v2 cert: %w", err)
	}

	return e.verifier.VerifyCertV2(ctx, &cert)
}
