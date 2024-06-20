package e2e_test

import (
	"testing"
	"time"

	"github.com/Layr-Labs/eigenda-proxy/client"
	"github.com/Layr-Labs/eigenda-proxy/fault"

	"github.com/Layr-Labs/eigenda-proxy/common"
	"github.com/Layr-Labs/eigenda-proxy/e2e"
	"github.com/Layr-Labs/eigenda/api/clients/codecs"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	op_plasma "github.com/ethereum-optimism/optimism/op-plasma"

	"github.com/stretchr/testify/require"
)

func useMemory() bool {
	return !runTestnetIntegrationTests
}

func TestPlasmaClient(t *testing.T) {
	if !runIntegrationTests && !runTestnetIntegrationTests {
		t.Skip("Skipping test as INTEGRATION or TESTNET env var not set")
	}

	t.Parallel()

	ts, kill := e2e.CreateTestSuite(t, useMemory(), nil)
	defer kill()

	daClient := op_plasma.NewDAClient(ts.Address(), false, false)
	t.Log("Waiting for client to establish connection with plasma server...")
	// wait for the server to come online after starting
	time.Sleep(5 * time.Second)

	// 1 - write arbitrary data to EigenDA

	var testPreimage = []byte("feel the rain on your skin!")

	t.Log("Setting input data on proxy server...")
	commit, err := daClient.SetInput(ts.Ctx, testPreimage)
	require.NoError(t, err)

	// 2 - fetch data from EigenDA for generated commitment key
	t.Log("Getting input data from proxy server...")
	preimage, err := daClient.GetInput(ts.Ctx, commit)
	require.NoError(t, err)
	require.Equal(t, testPreimage, preimage)
}

func TestProxyClient(t *testing.T) {
	if !runIntegrationTests && !runTestnetIntegrationTests {
		t.Skip("Skipping test as INTEGRATION or TESTNET env var not set")
	}

	t.Parallel()

	ts, kill := e2e.CreateTestSuite(t, useMemory(), nil)
	defer kill()

	cfg := &client.Config{
		URL: ts.Address(),
	}
	daClient := client.New(cfg)
	t.Log("Waiting for client to establish connection with plasma server...")
	// wait for server to come online after starting
	wait.For(ts.Ctx, time.Second*1, func() (bool, error) {
		err := daClient.Health()
		if err != nil {
			return false, nil
		}

		return true, nil
	})

	// 1 - write arbitrary data to EigenDA

	var testPreimage = []byte("inter-subjective and not objective!")

	t.Log("Setting input data on proxy server...")
	blobInfo, err := daClient.SetData(ts.Ctx, testPreimage)
	require.NoError(t, err)

	// 2 - fetch data from EigenDA for generated commitment key
	t.Log("Getting input data from proxy server...")
	preimage, err := daClient.GetData(ts.Ctx, blobInfo, common.BinaryDomain)
	require.NoError(t, err)
	require.Equal(t, testPreimage, preimage)

	// 3 - fetch iFFT representation of preimage
	iFFTPreimage, err := daClient.GetData(ts.Ctx, blobInfo, common.PolyDomain)
	require.NoError(t, err)
	require.NotEqual(t, preimage, iFFTPreimage)

	// 4 - Assert domain transformations

	ifftCodec := codecs.NewIFFTCodec(codecs.DefaultBlobCodec{})

	decodedBlob, err := ifftCodec.DecodeBlob(iFFTPreimage)
	require.NoError(t, err)

	require.Equal(t, decodedBlob, preimage)
}

func TestProxyClientWithFaultMode(t *testing.T) {
	if !runIntegrationTests && !runTestnetIntegrationTests {
		t.Skip("Skipping test as INTEGRATION or TESTNET env var not set")
	}

	t.Parallel()

	fc := &fault.Config{
		Actors: map[string]fault.Behavior{
			"sequencer": {
				Mode: fault.Honest,
			},

			"challenger": {
				Mode: fault.Byzantine,
			},
		},
	}

	ts, kill := e2e.CreateTestSuite(t, useMemory(), fc)
	defer kill()

	cfg := &client.Config{
		Actor: "sequencer",
		URL:   ts.Address(),
	}
	sequencerClient := client.New(cfg)

	cfg2 := &client.Config{
		Actor: "challenger",
		URL:   ts.Address(),
	}
	challengerClient := client.New(cfg2)

	// 1 - write arbitrary data to EigenDA

	var testPreimage = []byte("inter-subjective and not objective!")

	t.Log("Setting input data on proxy server...")
	blobInfo, err := sequencerClient.SetData(ts.Ctx, testPreimage)
	require.NoError(t, err)

	// 2 - fetch data from EigenDA for generated commitment key
	t.Log("Getting input data from proxy server...")
	preimage, err := sequencerClient.GetData(ts.Ctx, blobInfo, common.BinaryDomain)
	require.NoError(t, err)
	require.Equal(t, testPreimage, preimage)

	// 3 - fetch iFFT representation of preimage
	preimage, err = challengerClient.GetData(ts.Ctx, blobInfo, common.PolyDomain)
	require.NoError(t, err)
	require.NotEqual(t, testPreimage, preimage)
}
