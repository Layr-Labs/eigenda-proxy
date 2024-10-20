package e2e_test

import (
	"net/http"
	"os"
	"testing"

	"github.com/Layr-Labs/eigenda-proxy/commitments"
	"github.com/Layr-Labs/eigenda-proxy/store"

	"github.com/Layr-Labs/eigenda-proxy/metrics"
	"github.com/stretchr/testify/require"
)

var (
	runTestnetIntegrationTests bool // holesky tests
	runIntegrationTests        bool // memstore tests
)

// ParseEnv ... reads testing cfg fields. Go test flags don't work for this library due to the dependency on Optimism's E2E framework
// which initializes test flags per init function which is called before an init in this package.
func ParseEnv() {
	runIntegrationTests = os.Getenv("INTEGRATION") == "true" || os.Getenv("INTEGRATION") == "1"
	runTestnetIntegrationTests = os.Getenv("TESTNET") == "true" || os.Getenv("TESTNET") == "1"
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

// requireSecondaryWriteRead ... ensure that secondary backend was successfully written/read to/from
func requireSecondaryWriteRead(t *testing.T, cm *metrics.CountMap, bt store.BackendType) {
	writeCount, err := cm.Get(http.MethodPut, store.Success, bt.String())
	require.NoError(t, err)
	require.True(t, writeCount > 0)

	readCount, err := cm.Get(http.MethodGet, store.Success, bt.String())
	require.NoError(t, err)
	require.True(t, readCount > 0)
}
