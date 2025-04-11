package common

import (
	"github.com/Layr-Labs/eigenda/api/clients"
)

// ClientConfigV1 wraps all the configuration values necessary to configure v1 eigenDA clients
//
// This struct wraps around an instance of clients.EigenDAClientConfig, and adds additional required values. Ideally,
// the extra values would just be part of clients.EigenDAClientConfig. Since these additions would require core changes,
// though, and v1 is slated for deprecation, this wrapper is just a stopgap to better organize configs in the proxy
// repo in the short term.
type ClientConfigV1 struct {
	EdaClientCfg     clients.EigenDAClientConfig
	MaxBlobSizeBytes uint64
	// Determines number of times to try blob dispersals:
	// - If > 0: Try up to that many times total (first attempt + up to N-1 retries)
	// - If = 0: Try only once (no retries)
	// - If < 0: Try indefinitely until success
	PutRetries int
}
