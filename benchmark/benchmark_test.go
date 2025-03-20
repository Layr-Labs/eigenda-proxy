package benchmark

import (
	"context"
	"os"
	"testing"

	"github.com/Layr-Labs/eigenda-proxy/clients/standard_client"
	"github.com/Layr-Labs/eigenda-proxy/store"
	"github.com/Layr-Labs/eigenda-proxy/testutils"
)

// BenchmarkPutsWithSecondaryV1  ... Takes in an async worker count and profiles blob insertions using
// constant blob sizes in parallel. Exercises V1 code pathways
func BenchmarkPutsWithSecondaryV1(b *testing.B) {
	putsWithSecondary(b, false)
}

// BenchmarkPutsWithSecondaryV2  ... Takes in an async worker count and profiles blob insertions using
// constant blob sizes in parallel. Exercises V2 code pathways
func BenchmarkPutsWithSecondaryV2(b *testing.B) {
	putsWithSecondary(b, true)
}

func putsWithSecondary(b *testing.B, disperseToV2 bool) {
	flagsToOverride := testutils.GetFlagsToEnableS3Caching()
	writeThreadCount := os.Getenv("WRITE_THREAD_COUNT")
	if writeThreadCount != "" {
		flagsToOverride = append(
			flagsToOverride,
			testutils.FlagConfig{
				Name:  store.ConcurrentWriteThreads,
				Value: writeThreadCount})
	}

	ts, kill := testutils.CreateTestSuiteWithFlagOverrides(
		testutils.MemstoreBackend,
		disperseToV2,
		flagsToOverride)
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
}
