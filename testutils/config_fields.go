package testutils

import (
	"github.com/Layr-Labs/eigenda-proxy/testutils/testmatrix"
)

type TestBackend int

const (
	// Memstore uses a local memstore analog in place of an actual eigenDA network
	Memstore TestBackend = iota + 1
	// Testnet runs tests against holesky testnet
	Testnet
)

var V2Enabled = "v2Enabled"
var Backend = "backend"

// TestConfigFromConfigurationSet is a helper method which accepts a ConfigurationSet, and returns a TestConfig
//
// This method expects that the input configurationSet has `V2Enabled` and `Backend` fields configured. If either
// of these fields isn't configured, this method will panic.
func TestConfigFromConfigurationSet(configurationSet testmatrix.ConfigurationSet) TestConfig {
	backend, _ := configurationSet.GetValue(Backend).(TestBackend)
	v2Enabled, _ := configurationSet.GetValue(V2Enabled).(bool)

	return NewTestConfig(backend == Memstore, v2Enabled)
}
