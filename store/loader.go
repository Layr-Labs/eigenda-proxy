package store

import (
	"context"
	"fmt"
	"time"

	"github.com/Layr-Labs/eigenda-proxy/common"
	"github.com/Layr-Labs/eigenda-proxy/metrics"
	"github.com/Layr-Labs/eigenda-proxy/store/generated_key/eigenda"
	eigendav2 "github.com/Layr-Labs/eigenda-proxy/store/generated_key/eigenda_v2"
	"github.com/Layr-Labs/eigenda-proxy/store/generated_key/memstore"
	"github.com/Layr-Labs/eigenda-proxy/store/precomputed_key/redis"
	"github.com/Layr-Labs/eigenda-proxy/store/precomputed_key/s3"
	"github.com/Layr-Labs/eigenda-proxy/verify/v1"
	"github.com/Layr-Labs/eigenda/api/clients"
	clients_v2 "github.com/Layr-Labs/eigenda/api/clients/v2"
	"github.com/Layr-Labs/eigenda/api/clients/v2/verification"
	"github.com/rs/zerolog/log"

	"github.com/Layr-Labs/eigenda/common/geth"
	eigenda_eth "github.com/Layr-Labs/eigenda/core/eth"
	"github.com/Layr-Labs/eigenda/encoding/kzg"
	"github.com/Layr-Labs/eigensdk-go/logging"
	geth_common "github.com/ethereum/go-ethereum/common"
)

// StoreLoader centralizes dependency initialization
// It ensures proper typing and avoids redundant logic scattered across functions.
type StoreLoader struct {
	ctx     context.Context
	log     logging.Logger
	metrics metrics.Metricer

	memConfig *memstore.Config

	// configs
	managerCfg     Config
	v1VerifierCfg  verify.Config
	v1EdaClientCfg clients.EigenDAClientConfig
	v2ClientCfg    common.V2ClientConfig
}

func NewStoreLoader(ctx context.Context, cfg Config,
	v1VerifierCfg verify.Config,
	v1EdaClientCfg clients.EigenDAClientConfig,
	v2ClientCfg common.V2ClientConfig,
	memConfig *memstore.Config,
	log logging.Logger, metrics metrics.Metricer) *StoreLoader {
	return &StoreLoader{ctx, log, metrics, memConfig, cfg, v1VerifierCfg, v1EdaClientCfg, v2ClientCfg}
}

func (d *StoreLoader) loadSecondaries(targets []string, s3Store common.PrecomputedKeyStore, redisStore *redis.Store) []common.PrecomputedKeyStore {
	stores := make([]common.PrecomputedKeyStore, len(targets))

	for i, target := range targets {
		switch common.StringToBackendType(target) {
		case common.RedisBackendType:
			if redisStore == nil {
				panic(fmt.Sprintf("Redis backend not configured: %s", target))
			}
			stores[i] = redisStore
		case common.S3BackendType:
			if s3Store == nil {
				panic(fmt.Sprintf("S3 backend not configured: %s", target))
			}
			stores[i] = s3Store
		default:
			panic(fmt.Sprintf("Invalid backend target: %s", target))
		}
	}
	return stores
}

func (d *StoreLoader) loadEigenDAV2Store() (*eigendav2.Store, error) {
	gethCfg := geth.EthClientConfig{
		RPCURLs: []string{d.v1EdaClientCfg.EthRpcUrl},
	}

	ethClient, err := geth.NewClient(gethCfg, geth_common.Address{}, 0, d.log)
	if err != nil {
		return nil, err
	}

	g1Points, err := kzg.ReadG1Points(
		d.v1VerifierCfg.KzgConfig.G1Path,
		d.v1VerifierCfg.KzgConfig.SRSNumberToLoad, 4)
	if err != nil {
		return nil, err
	}

	reader, err := eigenda_eth.NewReader(d.log, ethClient, "0x0", d.v2ClientCfg.ServiceManagerAddress)
	if err != nil {
		return nil, err
	}

	relayURLs, err := reader.GetRelayURLs(d.ctx)
	if err != nil {
		return nil, err
	}

	relayCfg := clients_v2.RelayClientConfig{
		UseSecureGrpcFlag:  true,
		MaxGRPCMessageSize: d.v2ClientCfg.PutRetries,
		Sockets:            relayURLs,
	}

	retriever, err := clients_v2.BuildRelayPayloadRetriever(d.log, d.v2ClientCfg.RetrievalConfig, &relayCfg, g1Points)
	if err != nil {
		return nil, err
	}

	disperser, err := clients_v2.BuildPayloadDisperser(d.log, d.v2ClientCfg.PayloadClientCfg, &d.v2ClientCfg.DisperserClientCfg, &gethCfg, nil, nil)
	if err != nil {
		return nil, err
	}

	verifier, err := verification.NewCertVerifier(d.log, ethClient, d.v2ClientCfg.PayloadClientCfg.EigenDACertVerifierAddr, time.Second*1)
	if err != nil {
		return nil, err
	}

	return eigendav2.NewStore(d.log, &eigendav2.Config{
		// TODO: understand how to manage MaxBlobSizeBytes field
		// MaxBlobSizeBytes:   d.v2ClientCfg.DisperserClientCfg.MaxBlobSizeBytes,
		PutRetries: d.v2ClientCfg.PutRetries,
	}, disperser, retriever, verifier)
}

func (d *StoreLoader) loadEigenDAV1Store(ctx context.Context, putRetries uint) (*eigenda.Store, error) {
	verifier, err := verify.NewVerifier(&d.v1VerifierCfg, d.log)
	if err != nil {
		return nil, fmt.Errorf("failed to create verifier: %w", err)
	}

	if d.v1VerifierCfg.VerifyCerts {
		log.Info("Certificate verification with Ethereum enabled")
	} else {
		log.Warn("Verification disabled")
	}
	// create EigenDA backend store
	var eigenDA common.GeneratedKeyStore
	if d.memConfig != nil {
		log.Info("Using memstore backend for EigenDA")
		eigenDA, err = memstore.New(ctx, verifier, d.log, *d.memConfig)
	} else {
		// EigenDAV1 backend dependency injection
		var client *clients.EigenDAClient
		log.Warn("Using EigenDA backend.. This backend type will be deprecated soon. Please migrate to V2.")
		client, err = clients.NewEigenDAClient(d.log, d.v1EdaClientCfg)
		if err != nil {
			return nil, err
		}

		eigenDA, err = eigenda.NewStore(
			client,
			verifier,
			d.log,
			&eigenda.StoreConfig{
				MaxBlobSizeBytes:     d.memConfig.MaxBlobSizeBytes,
				EthConfirmationDepth: d.v1VerifierCfg.EthConfirmationDepth,
				StatusQueryTimeout:   d.v1EdaClientCfg.StatusQueryTimeout,
				PutRetries:           putRetries,
			},
		)
	}

	return eigenDA, nil
}

func (d *StoreLoader) LoadManager(ctx context.Context, putRetries uint) (IManager, error) {
	var err error
	var s3Store *s3.Store
	var redisStore *redis.Store
	var eigenDAV2Store *eigendav2.Store
	var eigenDAV1Store *eigenda.Store

	if d.managerCfg.S3Config.Bucket != "" {
		d.log.Info("Using S3 backend")
		s3Store, err = s3.NewStore(d.managerCfg.S3Config)
	}

	if err != nil {
		return nil, err
	}

	if d.managerCfg.RedisConfig.Endpoint != "" {
		d.log.Info("Using Redis backend")
		redisStore, err = redis.NewStore(&d.managerCfg.RedisConfig)
	}

	if err != nil {
		return nil, err
	}

	if d.v2ClientCfg.Enabled {
		eigenDAV2Store, err = d.loadEigenDAV2Store()
		if err != nil {
			return nil, err
		}
	}

	eigenDAV1Store, err = d.loadEigenDAV1Store(ctx, putRetries)

	fallbacks := d.loadSecondaries(d.managerCfg.FallbackTargets, s3Store, redisStore)
	caches := d.loadSecondaries(d.managerCfg.CacheTargets, s3Store, redisStore)
	secondary := NewSecondaryManager(d.log, d.metrics, caches, fallbacks)

	if secondary.Enabled() { // only spin-up go routines if secondary storage is enabled
		log.Debug("Starting secondary write loop(s)", "count", d.managerCfg.AsyncPutWorkers)

		for i := 0; i < d.managerCfg.AsyncPutWorkers; i++ {
			go secondary.WriteSubscriptionLoop(ctx)
		}
	}

	log.Info("Created storage backends",
		"eigenda_v1", eigenDAV1Store != nil,
		"eigenda_v2", eigenDAV2Store != nil,
		"s3", s3Store != nil,
		"redis", redisStore != nil,
	)
	return NewManager(nil, eigenDAV2Store, s3Store, d.log, secondary, d.v2ClientCfg.Enabled)
}
