package e2e_test

import (
	"os"
	"testing"
)

// only difference between integrationTests and testnetIntegrationTests is
// that integration tests are run against memstore whereas testnetintegration tests
// are run against eigenda backend talking to testnet disperser
var (
	runTestnetIntegrationTests bool
	runIntegrationTests        bool
)

func ParseEnv() {
	runIntegrationTests = os.Getenv("INTEGRATION") == "true" || os.Getenv("INTEGRATION") == "1"
	runTestnetIntegrationTests = os.Getenv("TESTNET") == "true" || os.Getenv("TESTNET") == "1"
}

func TestMain(m *testing.M) {
	ParseEnv()
	code := m.Run()
	os.Exit(code)
}
