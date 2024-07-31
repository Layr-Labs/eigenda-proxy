package store

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/Layr-Labs/eigenda-proxy/verify"
	"github.com/Layr-Labs/eigenda/encoding/kzg"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

const (
	testPreimage = "Four score and seven years ago"
)

func TestGetSet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	kzgConfig := &kzg.KzgConfig{
		G1Path:          "../resources/g1.point",
		G2PowerOf2Path:  "../resources/g2.point.powerOf2",
		CacheDir:        "../resources/SRSTables",
		SRSOrder:        3000,
		SRSNumberToLoad: 3000,
		NumWorker:       uint64(runtime.GOMAXPROCS(0)),
	}

	cfg := &verify.Config{
		Verify:    false,
		KzgConfig: kzgConfig,
	}

	verifier, err := verify.NewVerifier(cfg, nil)
	require.NoError(t, err)

	ms, err := NewMemStore(
		ctx,
		verifier,
		log.New(),
		1024*1024*2,
		time.Hour*1000,
		nil,
	)

	require.NoError(t, err)

	expected := []byte(testPreimage)
	key, err := ms.Put(ctx, expected)
	require.NoError(t, err)

	actual, err := ms.Get(ctx, key)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}


func TestByzantineReading(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	kzgConfig := &kzg.KzgConfig{
		G1Path:          "../resources/g1.point",
		G2PowerOf2Path:  "../resources/g2.point.powerOf2",
		CacheDir:        "../resources/SRSTables",
		SRSOrder:        3000,
		SRSNumberToLoad: 3000,
		NumWorker:       uint64(runtime.GOMAXPROCS(0)),
	}

	cfg := &verify.Config{
		Verify:    false,
		KzgConfig: kzgConfig,
	}

	verifier, err := verify.NewVerifier(cfg, nil)
	require.NoError(t, err)

	ms, err := NewMemStore(
		ctx,
		verifier,
		log.New(),
		1024*1024*2,
		0,
		&FaultConfig{
			Actors: map[string]Behavior{
				"all": {
					Mode:     Byzantine,
				},
			},
		},
	)

	require.NoError(t, err)
	key, err := ms.Put(ctx, []byte(testPreimage))
	require.NoError(t, err)

	readPreimage, err := ms.Get(ctx, key)
	require.NoError(t, err)
	require.NotEqual(t, []byte(testPreimage), readPreimage)

}


func TestIntervalByzantineReading(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	kzgConfig := &kzg.KzgConfig{
		G1Path:          "../resources/g1.point",
		G2PowerOf2Path:  "../resources/g2.point.powerOf2",
		CacheDir:        "../resources/SRSTables",
		SRSOrder:        3000,
		SRSNumberToLoad: 3000,
		NumWorker:       uint64(runtime.GOMAXPROCS(0)),
	}

	cfg := &verify.Config{
		Verify:    false,
		KzgConfig: kzgConfig,
	}

	verifier, err := verify.NewVerifier(cfg, nil)
	require.NoError(t, err)

	ms, err := NewMemStore(
		ctx,
		verifier,
		log.New(),
		1024*1024*2,
		0,
		&FaultConfig{
			Actors: map[string]Behavior{
				"all": {
					Mode:     IntervalByzantine,
					Interval: 1,
				},
			},
		},
	)

	require.NoError(t, err)
	key, err := ms.Put(ctx, []byte(testPreimage))
	require.NoError(t, err)

	// 1 - honest fetch
	readPreimage, err := ms.Get(ctx, key)
	require.NoError(t, err)
	require.Equal(t, []byte(testPreimage), readPreimage)

	// 2 - byzantine fetch
	readPreimage, err = ms.Get(ctx, key)
	require.NoError(t, err)
	require.NotEqual(t, []byte(testPreimage), readPreimage)

}
