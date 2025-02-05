package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Layr-Labs/eigenda-proxy/common"
	"github.com/Layr-Labs/eigenda-proxy/metrics"
	"github.com/Layr-Labs/eigenda-proxy/store"
	"github.com/Layr-Labs/eigenda-proxy/store/generated_key/eigenda"
	"github.com/Layr-Labs/eigenda-proxy/store/generated_key/eigenda_v2"
	"github.com/Layr-Labs/eigenda-proxy/store/generated_key/memstore"
	"github.com/Layr-Labs/eigenda-proxy/store/precomputed_key/redis"
	"github.com/Layr-Labs/eigenda-proxy/store/precomputed_key/s3"
	"github.com/Layr-Labs/eigenda-proxy/verify/v1"
	"github.com/Layr-Labs/eigenda/api/clients"
	clients_v2 "github.com/Layr-Labs/eigenda/api/clients/v2"
	"github.com/Layr-Labs/eigenda/api/clients/v2/verification"

	"github.com/Layr-Labs/eigenda/common/geth"
	"github.com/Layr-Labs/eigenda/encoding/kzg"
	geth_common "github.com/ethereum/go-ethereum/common"

	"github.com/Layr-Labs/eigensdk-go/logging"

	eigenda_eth "github.com/Layr-Labs/eigenda/core/eth"
)

// TODO - create structured abstraction for dependency injection vs. overloading stateless functions

// loadBackends ... creates a list of storage backends based on the user provided target strings
func loadBackends(targets []string, s3 common.PrecomputedKeyStore, redis *redis.Store) []common.PrecomputedKeyStore {
	stores := make([]common.PrecomputedKeyStore, len(targets))

	for i, f := range targets {
		b := common.StringToBackendType(f)

		switch b {
		case common.RedisBackendType:
			if redis == nil {
				panic(fmt.Sprintf("Redis backend is not configured but specified in targets: %s", f))
			}
			stores[i] = redis

		case common.S3BackendType:
			if s3 == nil {
				panic(fmt.Sprintf("S3 backend is not configured but specified in targets: %s", f))
			}
			stores[i] = s3

		case common.EigenDABackendType, common.MemoryBackendType:
			panic(fmt.Sprintf("Invalid target for fallback: %s", f))

		case common.UnknownBackendType:
			fallthrough

		default:
			panic(fmt.Sprintf("Unknown fallback target: %s", f))
		}
	}

	return stores
}

func loadEigenDAV2Store(ctx context.Context, cfg CLIConfig, log logging.Logger) (*eigenda_v2.Store, error) {
	// TODO: Replace with real logger once dependency PRs are merged

	gethCfg := geth.EthClientConfig{
		RPCURLs: []string{cfg.EigenDAConfig.EdaV1ClientConfig.EthRpcUrl},
	}

	ethClient, err := geth.NewClient(gethCfg, geth_common.Address{0x0}, 0, log)
	if err != nil {
		return nil, err
	}

	g1Points, err := kzg.ReadG1Points(cfg.EigenDAConfig.VerifierConfig.KzgConfig.G1Path, cfg.EigenDAConfig.VerifierConfig.KzgConfig.SRSNumberToLoad, 4)
	if err != nil {
		return nil, err
	}

	reader, err := eigenda_eth.NewReader(log, ethClient,
		"0x0", cfg.EigenDAConfig.EdaV1ClientConfig.SvcManagerAddr)

	if err != nil {
		return nil, err
	}

	log.Info("Reading relays")
	relayURLs, err := reader.GetRelayURLs(ctx)
	if err != nil {
		return nil, err
	}

	log.Info(fmt.Sprintf("Read relays %+v", relayURLs))

	relayCfg := clients_v2.RelayClientConfig{
		UseSecureGrpcFlag:  true,
		MaxGRPCMessageSize: 10000000000,
		Sockets:            relayURLs,
	}

	retriever, err := clients_v2.BuildRelayPayloadRetriever(log,
		cfg.EigenDAConfig.V2RetrievalConfig,
		gethCfg,
		&relayCfg,
		g1Points,
	)

	if err != nil {
		return nil, err
	}

	splits := strings.Split(cfg.EigenDAConfig.EdaV1ClientConfig.RPC, ":")
	println(fmt.Sprintf("%v", splits))

	cfg.EigenDAConfig.V2DispersalConfig.SignerPaymentKey = cfg.EigenDAConfig.EdaV1ClientConfig.SignerPrivateKeyHex

	cfg.EigenDAConfig.V2DispersalConfig.BlobCertifiedTimeout = time.Second * 100
	cfg.EigenDAConfig.V2DispersalConfig.Quorums = []uint8{0, 1, 2}

	log.Info("Building payload disperser")
	disperser, err := clients_v2.BuildPayloadDisperser(
		log,
		cfg.EigenDAConfig.V2DispersalConfig,
		&clients_v2.DisperserClientConfig{
			Hostname:          splits[0],
			Port:              splits[1],
			UseSecureGrpcFlag: !cfg.EigenDAConfig.EdaV1ClientConfig.DisableTLS,
		},
		&gethCfg,
		nil,
		nil,
	)

	if err != nil {
		return nil, err
	}

	verifier, err := verification.NewCertVerifier(log, ethClient, cfg.EigenDAConfig.V2DispersalConfig.EigenDACertVerifierAddr, time.Second * 1)
	if err != nil {
		return nil, err
	}

	return eigenda_v2.NewStore(nil, &eigenda_v2.Config{
		ServiceManagerAddr: cfg.EigenDAConfig.EdaV1ClientConfig.SvcManagerAddr,
		MaxBlobSizeBytes:   cfg.EigenDAConfig.MemstoreConfig.MaxBlobSizeBytes,
		StatusQueryTimeout: cfg.EigenDAConfig.EdaV1ClientConfig.StatusQueryTimeout,
		PutRetries:         cfg.EigenDAConfig.PutRetries,
	}, ethClient, disperser, retriever, verifier)
}

// LoadStoreManager ... creates storage backend clients and instruments them into a storage routing abstraction
func LoadStoreManager(ctx context.Context, cfg CLIConfig, log logging.Logger, m metrics.Metricer) (store.IManager, error) {
	// create S3 backend store (if enabled)
	var err error
	var s3Store *s3.Store
	var redisStore *redis.Store
	var eigenDAV2Store *eigenda_v2.Store

	// TODO: Replace with real logger once dependency PRs are merged

	if cfg.EigenDAConfig.StorageConfig.S3Config.Bucket != "" && cfg.EigenDAConfig.StorageConfig.S3Config.Endpoint != "" {
		log.Info("Using S3 backend")
		s3Store, err = s3.NewStore(cfg.EigenDAConfig.StorageConfig.S3Config)
		if err != nil {
			return nil, fmt.Errorf("failed to create S3 store: %w", err)
		}
	}

	if cfg.EigenDAConfig.StorageConfig.RedisConfig.Endpoint != "" {
		log.Info("Using Redis backend")
		// create Redis backend store
		redisStore, err = redis.NewStore(&cfg.EigenDAConfig.StorageConfig.RedisConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create Redis store: %w", err)
		}
	}

	// create cert/data verification type
	daCfg := cfg.EigenDAConfig
	vCfg := daCfg.VerifierConfig

	verifier, err := verify.NewVerifier(&vCfg, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create verifier: %w", err)
	}

	if vCfg.VerifyCerts {
		log.Info("Certificate verification with Ethereum enabled")
	} else {
		log.Warn("Verification disabled")
	}

	// create EigenDA backend store
	var eigenDA common.GeneratedKeyStore
	if cfg.EigenDAConfig.MemstoreEnabled {
		log.Info("Using memstore backend for EigenDA")
		eigenDA, err = memstore.New(ctx, verifier, log, cfg.EigenDAConfig.MemstoreConfig)
	} else {
		// EigenDAV1 backend dependency injection
		var client *clients.EigenDAClient
		log.Warn("Using EigenDA backend.. This backend type will be deprecated soon. Please migrate to V2.")
		client, err = clients.NewEigenDAClient(log, daCfg.EdaV1ClientConfig)
		if err != nil {
			return nil, err
		}

		eigenDA, err = eigenda.NewStore(
			client,
			verifier,
			log,
			&eigenda.StoreConfig{
				MaxBlobSizeBytes:     cfg.EigenDAConfig.MemstoreConfig.MaxBlobSizeBytes,
				EthConfirmationDepth: cfg.EigenDAConfig.VerifierConfig.EthConfirmationDepth,
				StatusQueryTimeout:   cfg.EigenDAConfig.EdaV1ClientConfig.StatusQueryTimeout,
				PutRetries:           cfg.EigenDAConfig.PutRetries,
			},
		)
	}

	if err != nil {
		return nil, err
	}

	if cfg.EigenDAConfig.EigenDAV2Enabled {
		log.Info("Using EigenDA V2 backend")
		eigenDAV2Store, err = loadEigenDAV2Store(ctx, cfg, log)
		if err != nil {
			return nil, err
		}
	}

	// create secondary storage router
	fallbacks := loadBackends(cfg.EigenDAConfig.StorageConfig.FallbackTargets, s3Store, redisStore)
	caches := loadBackends(cfg.EigenDAConfig.StorageConfig.CacheTargets, s3Store, redisStore)
	secondary := store.NewSecondaryManager(log, m, caches, fallbacks)

	if secondary.Enabled() { // only spin-up go routines if secondary storage is enabled
		log.Debug("Starting secondary write loop(s)", "count", cfg.EigenDAConfig.StorageConfig.AsyncPutWorkers)

		for i := 0; i < cfg.EigenDAConfig.StorageConfig.AsyncPutWorkers; i++ {
			go secondary.WriteSubscriptionLoop(ctx)
		}
	}

	log.Info("Created storage backends",
		"eigenda", eigenDA != nil,
		"eigenda_v2", eigenDAV2Store != nil,
		"s3", s3Store != nil,
		"redis", redisStore != nil,
	)
	return store.NewManager(eigenDA, eigenDAV2Store, s3Store, log, secondary, cfg.EigenDAConfig.EigenDAV2Enabled)
}
