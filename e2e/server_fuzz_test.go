package e2e_test

import (
	"github.com/Layr-Labs/eigenda-proxy/client"
	"github.com/Layr-Labs/eigenda-proxy/e2e"
	op_plasma "github.com/ethereum-optimism/optimism/op-plasma"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func FuzzProxyClientServerIntegration(f *testing.F) {
	//if !runIntegrationTests && !runTestnetIntegrationTests {
	//	f.Skip("Skipping test as INTEGRATION or TESTNET env var not set")
	//}

	testCfg := e2e.TestConfig(useMemory())
	testCfg.UseKeccak256ModeS3 = true
	tsConfig := e2e.TestSuiteConfigF(f, testCfg)
	ts, kill := e2e.CreateTestSuiteF(f, tsConfig)
	defer kill()

	cfg := &client.Config{
		URL: ts.Address(),
	}
	daClient := client.New(cfg)

	// Add each printable Unicode character as a seed including ascii
	//for r := rune(0); r <= unicode.MaxRune; r++ {
	//	if unicode.IsPrint(r) {
	//		f.Add(fmt.Sprintf("seed: %s", string(r)), []byte(string(r))) // Add each printable Unicode character as a seed
	//	}
	//}

	f.Fuzz(func(t *testing.T, seed string, data []byte) {
		_, err := daClient.SetData(ts.Ctx, data)
		require.NoError(t, err)
	})
}

func FuzzOpClientKeccak256MalformedInputs(f *testing.F) {
	if !runIntegrationTests || runTestnetIntegrationTests {
		f.Skip("Skipping test as INTEGRATION var not set")
	}

	testCfg := e2e.TestConfig(useMemory())
	testCfg.UseKeccak256ModeS3 = true
	tsConfig := e2e.TestSuiteConfigF(f, testCfg)
	ts, kill := e2e.CreateTestSuiteF(f, tsConfig)
	defer kill()

	daClientPcFalse := op_plasma.NewDAClient(ts.Address(), false, false)

	// Fuzz the SetInput function with random data
	// seed and data are expected. `seed` value is seed: {i} and data is the one with the random string
	f.Fuzz(func(t *testing.T, seed string, data []byte) {

		_, err := daClientPcFalse.SetInput(ts.Ctx, data)
		// should fail with proper error message as is now, and cannot contain panics or nils
		if err != nil {
			assert.True(t, !isNilPtrDerefPanic(err.Error()))
		}

	})

}
