package e2e_test

import (
	"fmt"
	"testing"
	"unicode"
)

func addAllUnicodeTestCases(f *testing.F) {
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if unicode.IsPrint(r) {
			f.Add(fmt.Sprintf("seed: %s", string(r)), []byte(string(r))) // Add each printable Unicode character as a seed
		}
	}
}

// FuzzProxyClientServerIntegrationAndOpClientKeccak256MalformedInputs will fuzz the proxy client server integration
// and op client keccak256 with malformed inputs. This is never meant to be fuzzed with EigenDA.
//func FuzzProxyClientServerIntegrationAndOpClientKeccak256MalformedInputs(f *testing.F) {
//	if !runFuzzTests {
//		f.Skip("Skipping test as FUZZ env var not set")
//	}
//
//	testCfg := e2e.TestConfig(true)
//	testCfg.UseKeccak256ModeS3 = true
//	tsConfig := e2e.TestSuiteConfig(testCfg)
//	ts, kill := e2e.CreateTestSuite(tsConfig)
//	defer kill()
//
//	// Add each printable Unicode character as a seed
//	addAllUnicodeTestCases(f)
//
//	cfg := &client.Config{
//		URL: ts.Address(),
//	}
//	daClient := client.New(cfg)
//	daClientPcFalse := op_plasma.NewDAClient(ts.Address(), false, false)
//
//	// seed and data are expected. `seed` value is seed: {rune} and data is the one with the random byte(s)
//	f.Fuzz(func(t *testing.T, _ string, data []byte) {
//
//		_, err := daClient.SetData(ts.Ctx, data)
//		require.NoError(t, err)
//
//		_, err = daClientPcFalse.SetInput(ts.Ctx, data)
//		// should fail with proper error message as is now, and cannot contain panics or nils
//		if err != nil {
//			assert.True(t, !isNilPtrDerefPanic(err.Error()))
//		}
//	})
//}
