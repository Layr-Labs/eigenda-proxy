package e2e

import "fmt"

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
