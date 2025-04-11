package utils

// MapPutRetries converts a user-provided PutRetries value to a value for the retry-go library.
// This function handles the translation between the user-facing API and the internal retry-go library.
// IMPORTANT: In retry-go, the "attempts" parameter represents the TOTAL number of attempts, including the first try.
//
// The mapping works as follows:
// - putRetries > 0: Maps to that value directly (e.g., 3 means "try once, then retry twice if needed")
// - putRetries = 0: Maps to 1 (means "try once with no retries"). This mapping exists to provide sane default behavior
// - putRetries < 0: Maps to 0 (means "retry indefinitely until success")
//
// Note: This function is used by both v1 and v2 EigenDA backends to maintain consistent retry behavior.
func MapPutRetries(putRetries int) uint {
	switch {
	case putRetries > 0:
		return uint(putRetries)
	case putRetries == 0:
		return 1
	default: // putRetries < 0
		return 0 // 0 means "retry forever" in the retry-go library
	}
}
