package verify

import "fmt"

// This error is returned when the hash of the batch metadata in the cert
// does not match the hash stored in the EigenDAServiceManager.
//
// This error can currently occur on op-stack when a L1 reorg happens (but not always!).
// The cert's confirmation block number can changed by the reorg, whereas the cert still contains the old
// block number. This causes the hash of the batch metadata to change, which causes the error.
// See https://github.com/Layr-Labs/eigenda-proxy/blob/main/docs/troubleshooting_v1.md#batch-hash-mismatch-error
// for more details.
//
// We define a structured error because this error is expected to be handled.
// The proxy should query the disperser for the latest confirmation block number and
// update the cert.
// Note that this means that it is possible for the cert in the op's batcher inbox to be invalid.
// However, to prevent an L2 reorg from happening, we prefer to update the cert's confirmation block number
// to make it valid!
type BatchMetadataHashMismatchError struct {
	// batch metadata hash that is stored in the EigenDAServiceManager
	OnchainHash []byte
	// batch metadata hash that is computed from the cert's batch metadata
	ComputedHash []byte
}

// Implement the Error interface
func (e *BatchMetadataHashMismatchError) Error() string {
	return fmt.Sprintf("batch hash mismatch, onchain: %x, computed: %x; did an L1 reorg happen?", e.OnchainHash, e.ComputedHash)
}

// Sentry error for the error type BatchMetadataHashMismatchError
// We follow the naming convention outlined in https://github.com/Antonboom/errname?tab=readme-ov-file#motivation
// Example usage:
//
//	if errors.Is(err, verify.ErrHashMismatchSentry) {
//	    // handle error
//	}
var ErrBatchMetadataHashMismatch = &BatchMetadataHashMismatchError{}

// Is only checks that the error is of the correct type. It does not check the contents of the error.
func (e *BatchMetadataHashMismatchError) Is(target error) bool {
	_, ok := target.(*BatchMetadataHashMismatchError)
	return ok
}
