package fuzz_test

import (
	"github.com/Layr-Labs/eigenda-proxy/testutils"
	"github.com/stretchr/testify/assert"

	"testing"
	"unicode"

	"github.com/Layr-Labs/eigenda-proxy/clients/standard_client"
)

// FuzzProxyClientServerV1 will fuzz the proxy client server integration
// and op client keccak256 with malformed inputs. This is never meant to be fuzzed with EigenDA.
func FuzzProxyClientServerV1(f *testing.F) {
	testCfg := testutils.NewTestConfig(true, false)
	fuzzProxyClientServer(f, testCfg)
}

func FuzzProxyClientServerV2(f *testing.F) {
	testCfg := testutils.NewTestConfig(true, true)
	fuzzProxyClientServer(f, testCfg)
}

func fuzzProxyClientServer(f *testing.F, testCfg testutils.TestConfig) {
	tsConfig := testutils.BuildTestSuiteConfig(testCfg)
	tsSecretConfig := testutils.TestSuiteSecretConfig(testCfg)
	ts, kill := testutils.CreateTestSuite(tsConfig, tsSecretConfig)

	for r := rune(0); r <= unicode.MaxRune; r++ {
		if unicode.IsPrint(r) {
			f.Add([]byte(string(r))) // Add each printable Unicode character as a seed
		}
	}

	cfg := &standard_client.Config{
		URL: ts.Address(),
	}

	daClient := standard_client.New(cfg)

	// seed and data are expected. `seed` value is seed: {rune} and data is the one with the random byte(s)
	f.Fuzz(
		func(t *testing.T, data []byte) {
			_, err := daClient.SetData(ts.Ctx, data)
			assert.NoError(t, err)
			if err != nil {
				t.Errorf("Failed to set data: %v", err)
			}
		})

	f.Cleanup(kill)
}
