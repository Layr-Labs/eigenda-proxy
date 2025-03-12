package testutils

import (
	"fmt"

	"github.com/Layr-Labs/eigenda-proxy/testutils/testmatrix"
)

type TestEnvironment int

const (
	// Local environment uses a local memstore analog for the actual eigenDA network
	Local TestEnvironment = iota + 1
	// Testnet runs tests against holesky testnet
	Testnet
)

var V2Enabled = "v2Enabled"
var Environment = "environment"

// UseMemstore returns whether a test should use a local memstore, based on the test environment
func UseMemstore(environment TestEnvironment) bool {
	switch environment {
	case Local:
		return true
	case Testnet:
		return false
	default:
		panic(fmt.Sprintf("unknown test environment: %d", environment))
	}
}

// TestConfigFromConfigurationSet is a helper method which accepts a ConfigurationSet, and returns a TestConfig
//
// This method expects that the input configurationSet has `V2Enabled` and `Environment` fields configured. If either
// of these fields isn't configured, this method will panic.
func TestConfigFromConfigurationSet(configurationSet testmatrix.ConfigurationSet) TestConfig {
	v2Enabled, _ := configurationSet.GetValue(V2Enabled).(bool)
	backend, _ := configurationSet.GetValue(Environment).(TestEnvironment)
	useMemstore := UseMemstore(backend)

	return NewTestConfig(v2Enabled, useMemstore)
}
