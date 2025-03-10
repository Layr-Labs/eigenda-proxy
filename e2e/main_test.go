package e2e_test

import (
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/Layr-Labs/eigenda-proxy/clients/standard_client"
	"github.com/Layr-Labs/eigenda-proxy/commitments"
	"github.com/Layr-Labs/eigenda-proxy/common"
	"github.com/Layr-Labs/eigenda-proxy/e2e"
	"github.com/Layr-Labs/eigenda-proxy/store"
	altda "github.com/ethereum-optimism/optimism/op-alt-da"

	"github.com/Layr-Labs/eigenda-proxy/metrics"
	"github.com/stretchr/testify/require"
)

// TestType is an enum describing various types of tests. This TestType is used to determine whether
// a given test should run in the configured test environment.
type TestType int

const (
	// StandardIntegration is a test that should run in any integration environment (local, or testnet)
	StandardIntegration TestType = iota // 0
	// LocalOnlyIntegration is a test that should run only in a local integration environment, NOT on testnet
	LocalOnlyIntegration
	// Fuzz is a fuzz test
	Fuzz
)

var (
	config testFlagConfig
)

// testFlagConfig contains the boolean flags used to configure test execution
type testFlagConfig struct {
	runTestnetIntegrationTests bool // holesky tests
	runIntegrationTests        bool // memstore tests
	runFuzzTests               bool // fuzz tests
	enableV2                   bool
}

// validate checks that the values in testFlagConfig are sensical, and prints the configured values for convenience
func (tfc *testFlagConfig) validate() {
	if tfc.runIntegrationTests && tfc.runTestnetIntegrationTests {
		panic("only one of INTEGRATION=true or TESTNET=true env var can be set")
	}

	fmt.Printf(
		"runFuzzTests: %v, runIntegrationTests: %v, runTestnetIntegrationTests: %v, enableV2: %v\n",
		tfc.runFuzzTests, tfc.runIntegrationTests, tfc.runTestnetIntegrationTests, tfc.enableV2,
	)
}

// flagActivated returns true if a given environment variable is active, otherwise false
func flagActivated(envVar string) bool {
	return os.Getenv(envVar) == "true" || os.Getenv(envVar) == "1"
}

// shouldRunTest accepts a parameter testType, which represents the type of test that is calling shouldRunTest. Based
// on the testType, this method returns whether the test in question should run based on the configured values.
func shouldRunTest(testType TestType) bool {
	switch testType {
	case StandardIntegration:
		return config.runIntegrationTests || config.runTestnetIntegrationTests
	case LocalOnlyIntegration:
		return config.runIntegrationTests
	case Fuzz:
		return config.runFuzzTests
	default:
		panic("unknown test type")
	}
}

// useMemory returns true if the test should use a memstore backend for local testing
//
// Local integration tests are run against memstore whereas testnet integration tests are run against eigenda backend,
// talking to testnet disperser.
func useMemory() bool {
	return !config.runTestnetIntegrationTests
}

// v2Enabled returns whether v2 should be enabled in a test
func v2Enabled() bool {
	return config.enableV2
}

// ParseEnv ... reads testing cfg fields. Go test flags don't work for this library due to the dependency on Optimism's
// E2E framework
// which initializes test flags per init function which is called before an init in this package.
func ParseEnv() {
	config = testFlagConfig{
		runTestnetIntegrationTests: flagActivated("TESTNET"),
		runIntegrationTests:        flagActivated("INTEGRATION"),
		enableV2:                   flagActivated("ENABLE_V2"),
		runFuzzTests:               flagActivated("FUZZ"),
	}

	config.validate()
}

// TestMain ... run main controller
func TestMain(m *testing.M) {
	ParseEnv()

	code := m.Run()
	os.Exit(code)
}

// requireDispersalRetrievalEigenDA ... ensure that blob was successfully dispersed/read to/from EigenDA
func requireDispersalRetrievalEigenDA(t *testing.T, cm *metrics.CountMap, mode commitments.CommitmentMode) {
	writeCount, err := cm.Get(string(mode), http.MethodPost)
	require.NoError(t, err)
	require.True(t, writeCount > 0)

	readCount, err := cm.Get(string(mode), http.MethodGet)
	require.NoError(t, err)
	require.True(t, readCount > 0)
}

// requireWriteReadSecondary ... ensure that secondary backend was successfully written/read to/from
func requireWriteReadSecondary(t *testing.T, cm *metrics.CountMap, bt common.BackendType) {
	writeCount, err := cm.Get(http.MethodPut, store.Success, bt.String())
	require.NoError(t, err)
	require.True(t, writeCount > 0)

	readCount, err := cm.Get(http.MethodGet, store.Success, bt.String())
	require.NoError(t, err)
	require.True(t, readCount > 0)
}

// requireStandardClientSetGet ... ensures that std proxy client can disperse and read a blob
func requireStandardClientSetGet(t *testing.T, ts e2e.TestSuite, blob []byte) {
	cfg := &standard_client.Config{
		URL: ts.Address(),
	}
	daClient := standard_client.New(cfg)

	t.Log("Setting input data on proxy server...")
	blobInfo, err := daClient.SetData(ts.Ctx, blob)
	require.NoError(t, err)

	t.Log("Getting input data from proxy server...")
	preimage, err := daClient.GetData(ts.Ctx, blobInfo)
	require.NoError(t, err)
	require.Equal(t, blob, preimage)

}

// requireOPClientSetGet ... ensures that alt-da client can disperse and read a blob
func requireOPClientSetGet(t *testing.T, ts e2e.TestSuite, blob []byte, precompute bool) {
	daClient := altda.NewDAClient(ts.Address(), false, precompute)

	commit, err := daClient.SetInput(ts.Ctx, blob)
	require.NoError(t, err)

	preimage, err := daClient.GetInput(ts.Ctx, commit)
	require.NoError(t, err)
	require.Equal(t, blob, preimage)

}
