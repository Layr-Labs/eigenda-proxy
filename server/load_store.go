package server

import (
	"context"
	"fmt"

	"github.com/Layr-Labs/eigenda-proxy/cli"
	"github.com/Layr-Labs/eigenda-proxy/store"
	"github.com/Layr-Labs/eigenda-proxy/store/generated_key/eigenda"
	"github.com/Layr-Labs/eigenda-proxy/store/generated_key/memstore"
	"github.com/Layr-Labs/eigenda-proxy/store/precomputed_key/redis"
	"github.com/Layr-Labs/eigenda-proxy/store/precomputed_key/s3"
	"github.com/Layr-Labs/eigenda-proxy/verify"
	"github.com/Layr-Labs/eigenda/api/clients"
	"github.com/ethereum/go-ethereum/log"
)

// populateTargets ... creates a list of storage backends based on the provided target strings
func populateTargets(targets []string, s3 store.PrecomputedKeyStore, redis *redis.Store) []store.PrecomputedKeyStore {
	stores := make([]store.PrecomputedKeyStore, len(targets))

	for i, f := range targets {
		b := store.StringToBackendType(f)

		switch b {
		case store.RedisBackendType:
			stores[i] = redis

		case store.S3BackendType:
			stores[i] = s3

		case store.EigenDABackendType, store.MemoryBackendType:
			panic(fmt.Sprintf("Invalid target for fallback: %s", f))

		case store.Unknown:
			fallthrough

		default:
			panic(fmt.Sprintf("Unknown fallback target: %s", f))
		}
	}

	return stores
}

// LoadStoreRouter ... creates storage backend clients and instruments them into a storage routing abstraction
func LoadStoreRouter(ctx context.Context, cfg cli.CLIConfig, log log.Logger) (store.IRouter, error) {
	// create S3 backend store (if enabled)
	var err error
	var s3Store store.PrecomputedKeyStore
	var redisStore *redis.Store

	if cfg.EigenDAConfig.S3Config.Bucket != "" && cfg.EigenDAConfig.S3Config.Endpoint != "" {
		log.Info("Using S3 backend")
		s3Store, err = s3.NewS3(cfg.EigenDAConfig.S3Config)
		if err != nil {
			return nil, fmt.Errorf("failed to create S3 store: %w", err)
		}
	}

	if cfg.EigenDAConfig.RedisConfig.Endpoint != "" {
		log.Info("Using Redis backend")
		// create Redis backend store
		redisStore, err = redis.NewStore(&cfg.EigenDAConfig.RedisConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create Redis store: %w", err)
		}
	}

	// create cert/data verification type
	daCfg := cfg.EigenDAConfig
	vCfg := daCfg.VerificationCfg()

	verifier, err := verify.NewVerifier(vCfg, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create verifier: %w", err)
	}

	if vCfg.VerifyCerts {
		log.Info("Certificate verification with Ethereum enabled")
	} else {
		log.Warn("Verification disabled")
	}

	// TODO: change this logic... we shouldn't need to calculate this here.
	// It should already be part of the config
	maxBlobLength, err := daCfg.GetMaxBlobLength()
	if err != nil {
		return nil, err
	}

	// create EigenDA backend store
	var eigenDA store.KeyGeneratedStore
	if cfg.EigenDAConfig.MemstoreEnabled {
		log.Info("Using mem-store backend for EigenDA")
		eigenDA, err = memstore.New(ctx, verifier, log, memstore.Config{
			MaxBlobSizeBytes: maxBlobLength,
			BlobExpiration:   cfg.EigenDAConfig.MemstoreBlobExpiration,
			PutLatency:       cfg.EigenDAConfig.MemstorePutLatency,
			GetLatency:       cfg.EigenDAConfig.MemstoreGetLatency,
		})
	} else {
		var client *clients.EigenDAClient
		log.Info("Using EigenDA backend")
		client, err = clients.NewEigenDAClient(log.With("subsystem", "eigenda-client"), daCfg.ClientConfig)
		if err != nil {
			return nil, err
		}

		eigenDA, err = eigenda.NewStore(
			client,
			verifier,
			log,
			&eigenda.StoreConfig{
				MaxBlobSizeBytes:     maxBlobLength,
				EthConfirmationDepth: uint64(cfg.EigenDAConfig.EthConfirmationDepth), // #nosec G115
				StatusQueryTimeout:   cfg.EigenDAConfig.ClientConfig.StatusQueryTimeout,
			},
		)
	}

	if err != nil {
		return nil, err
	}

	// determine read fallbacks
	fallbacks := populateTargets(cfg.EigenDAConfig.FallbackTargets, s3Store, redisStore)
	caches := populateTargets(cfg.EigenDAConfig.CacheTargets, s3Store, redisStore)

	log.Info("Creating storage router", "eigenda backend type", eigenDA != nil, "s3 backend type", s3Store != nil)
	return store.NewRouter(eigenDA, s3Store, log, caches, fallbacks)
}
