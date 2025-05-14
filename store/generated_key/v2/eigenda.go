package eigenda

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Layr-Labs/eigenda-proxy/common"
	"github.com/Layr-Labs/eigenda-proxy/store/generated_key/utils"
	"github.com/Layr-Labs/eigenda/api/clients/v2"
	"github.com/Layr-Labs/eigenda/api/clients/v2/coretypes"
	"github.com/Layr-Labs/eigenda/api/clients/v2/payloaddispersal"
	"github.com/Layr-Labs/eigensdk-go/logging"
	"github.com/avast/retry-go/v4"
	"github.com/ethereum/go-ethereum/rlp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Store does storage interactions and verifications for blobs with the EigenDA V2 protocol.
type Store struct {
	log logging.Logger

	// Number of times to try blob dispersals:
	// - If > 0: Try N times total
	// - If < 0: Retry indefinitely until success
	// - If = 0: Not permitted
	putTries int
	// Allowed distance (in L1 blocks) between the eigenDA cert's reference block number (RBN)
	// and the L1 block number at which the cert was included in the rollup's batch inbox.
	// If batch.RBN + rbnRecencyWindowSize < cert.L1InclusionBlock, the batch is considered
	// stale and verification will fail.
	// This check is optional and will be skipped when rbnRecencyWindowSize is set to 0.
	rbnRecencyWindowSize uint64

	disperser  *payloaddispersal.PayloadDisperser
	retrievers []clients.PayloadRetriever
	verifier   clients.ICertVerifier
}

var _ common.EigenDAStore = (*Store)(nil)

func NewStore(
	log logging.Logger,
	putTries int,
	rbnRecencyWindowSize uint64,
	disperser *payloaddispersal.PayloadDisperser,
	retrievers []clients.PayloadRetriever,
	verifier clients.ICertVerifier,
) (*Store, error) {
	if putTries == 0 {
		return nil, fmt.Errorf(
			"putTries==0 is not permitted. >0 means 'try N times', <0 means 'retry indefinitely'")
	}

	return &Store{
		log:        log,
		putTries:   putTries,
		disperser:  disperser,
		retrievers: retrievers,
		verifier:   verifier,
	}, nil
}

// Get fetches a blob from DA using certificate fields and verifies blob
// against commitment to ensure data is valid and non-tampered.
func (e Store) Get(ctx context.Context, key []byte) ([]byte, error) {
	var cert coretypes.EigenDACert
	err := rlp.DecodeBytes(key, &cert)
	if err != nil {
		return nil, fmt.Errorf("RLP decoding EigenDA v2 cert: %w", err)
	}

	// Try each retriever in sequence until one succeeds
	var errs []error
	for _, retriever := range e.retrievers {
		payload, err := retriever.GetPayload(ctx, &cert)
		if err == nil {
			return payload.Serialize(), nil
		}

		e.log.Debugf("Payload retriever failed: %v", err)
	}

	return nil, fmt.Errorf("all retrievers failed: %w", errors.Join(errs...))
}

// Put disperses a blob for some pre-image and returns the associated RLP encoded certificate commit.
// TODO: Client polling for different status codes, Mapping status codes to 503 failover
func (e Store) Put(ctx context.Context, value []byte) ([]byte, error) {
	e.log.Debug("Dispersing payload to EigenDA V2 network")

	// TODO: https://github.com/Layr-Labs/eigenda/issues/1271

	// We attempt to disperse the blob to EigenDA up to PutRetries times, unless we get a 400 error on any attempt.

	payload := coretypes.NewPayload(value)

	cert, err := retry.DoWithData(
		func() (*coretypes.EigenDACert, error) {
			return e.disperser.SendPayload(ctx, payload)
		},
		retry.RetryIf(
			func(err error) bool {
				grpcStatus, isGRPCError := status.FromError(err)
				if !isGRPCError {
					// api.ErrorFailover is returned, so we should retry
					return true
				}
				//nolint:exhaustive // we only care about a few grpc error codes
				switch grpcStatus.Code() {
				case codes.InvalidArgument:
					// we don't retry 400 errors because there is no point, we are passing invalid data
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
		// it to an http 503 to signify to the client (batcher) to failover to ethda b/c eigenda is temporarily down.
		retry.LastErrorOnly(true),
		// retry.Attempts uses different semantics than our config field. ConvertToRetryGoAttempts converts between
		// these two semantics.
		retry.Attempts(utils.ConvertToRetryGoAttempts(e.putTries)),
	)
	if err != nil {
		// TODO: we will want to filter for errors here and return a 503 when needed, i.e. when dispersal itself failed,
		//  or that we timed out waiting for batch to land onchain
		return nil, err
	}

	return rlp.EncodeToBytes(cert)
}

// BackendType returns the backend type for EigenDA Store
func (e Store) BackendType() common.BackendType {
	return common.EigenDAV2BackendType
}

// Verify verifies an EigenDACert by calling the verifyEigenDACertV2 view function
//
// Since v2 methods for fetching a payload are responsible for verifying the received bytes against the certificate,
// this Verify method only needs to check the cert on chain. That is why the third parameter is ignored.
func (e Store) Verify(ctx context.Context, certBytes []byte, _ []byte, opts common.VerifyOpts) error {
	var eigenDACert coretypes.EigenDACert
	err := rlp.DecodeBytes(certBytes, &eigenDACert)
	if err != nil {
		return fmt.Errorf("RLP decoding EigenDA v2 cert: %w", err)
	}
	err = e.verifyRBNRecencyCheck(eigenDACert.BatchHeader.ReferenceBlockNumber, opts)
	if err != nil {
		return fmt.Errorf("rbn recency check failed: %w", err)
	}

	return e.verifier.VerifyCertV2(ctx, &eigenDACert)
}

// Certs in the rollup batcher-inbox that do not respect the below equation are discarded.
//
//	cert.RBN < cert.L1InclusionBlock <= cert.RBN + RBNRecencyWindowSize
//
// where ReferenceBlockNumber (RBN) is the block number at which operator stakes are referenced
// to verify the signature thresholds, and L1InclusionBlock is the block at which the cert
// was included in the L1 block.
//
// This check serves 2 purposes:
//  1. liveness: prevents derivation pipeline from stalling on blobs that are no longer available on the DA layer
//  2. safety: prevents a malicious EigenDA sequencer from using a very stale RBN whose operator distribution
//     does not represent the actual stake distribution. Operators that withdrew a lot of stake would
//     not be slashable anymore, even though because of the old RBN their signature would count for a lot of stake.
//
// Note that for a secure integration, this same check needs to be verified onchain.
// There are 2 approaches to doing this:
//  1. Pessimistic approach: use a smart batcher inbox to dissalow stale blobs from even beign included
//     in the batcher inbox (see https://github.com/ethereum-optimism/design-docs/pull/229)
//  2. Optimistic approach: verify the check in op-program or hokulea (kona)'s derivation pipeline. See
//
// https://github.com/Layr-Labs/hokulea/blob/8c4c89bc4f35d56a3cec2220575a9681d987105c/crates/eigenda/src/eigenda.rs#L90
func (e Store) verifyRBNRecencyCheck(certRBN uint32, opts common.VerifyOpts) error {
	if opts.RollupL1InclusionBlockNum > 0 && e.rbnRecencyWindowSize > 0 {
		rollupInclusionBlock := opts.RollupL1InclusionBlockNum
		if !(uint64(certRBN) < rollupInclusionBlock) {
			return fmt.Errorf(
				"eigenda batch reference block number (%d) needs to be < rollup inclusion block number (%d): this is a serious bug, please report it",
				certRBN,
				rollupInclusionBlock,
			)
		}
		if !(rollupInclusionBlock <= uint64(certRBN)+e.rbnRecencyWindowSize) {
			return fmt.Errorf(
				"rollup inclusion block number (%d) needs to be <= eigenda cert.RBN (%d) + RBNRecencyWindowSize (%d)",
				rollupInclusionBlock,
				certRBN,
				e.rbnRecencyWindowSize,
			)
		}
	}
	return nil
}
