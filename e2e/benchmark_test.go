package e2e

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/Layr-Labs/eigenda-proxy/clients/standard_client"
	"github.com/Layr-Labs/eigenda-proxy/testutils"
	"github.com/Layr-Labs/eigenda-proxy/testutils/testmatrix"
)

// BenchmarkPutsWithSecondary  ... Takes in an async worker count and profiles blob insertions using
// constant blob sizes in parallel
func BenchmarkPutsWithSecondary(b *testing.B) {
	testMatrix := testmatrix.NewTestMatrix()
	testMatrix.AddDimension(testmatrix.NewDimension(testutils.V2Enabled, []any{true, false}))
	testMatrix.AddDimension(testmatrix.NewDimension(testutils.Backend, []any{testutils.Memstore}))

	for _, configurationSet := range testMatrix.GenerateConfigurationSets() {
		b.Run(
			configurationSet.ToString(), func(b *testing.B) {
				testCfg := testutils.TestConfigFromConfigurationSet(configurationSet)
				testCfg.UseS3Caching = true
				writeThreadCount := os.Getenv("WRITE_THREAD_COUNT")
				threadInt, err := strconv.Atoi(writeThreadCount)
				if err != nil {
					panic(fmt.Errorf("Could not parse WRITE_THREAD_COUNT field %w", err))
				}
				testCfg.WriteThreadCount = threadInt

				tsConfig := testutils.BuildTestSuiteConfig(testCfg)
				tsSecretConfig := testutils.TestSuiteSecretConfig(testCfg)
				ts, kill := testutils.CreateTestSuite(tsConfig, tsSecretConfig)
				defer kill()

				cfg := &standard_client.Config{
					URL: ts.Address(),
				}
				daClient := standard_client.New(cfg)

				for i := 0; i < b.N; i++ {
					_, err := daClient.SetData(
						context.Background(),
						[]byte("I am a blob and I only live for 14 days on EigenDA"))
					if err != nil {
						panic(err)
					}
				}
			})
	}
}
