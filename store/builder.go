package store

import (
	"context"
	"fmt"
	"time"

	"github.com/Layr-Labs/eigenda-proxy/common"
	"github.com/Layr-Labs/eigenda-proxy/metrics"
	"github.com/Layr-Labs/eigenda-proxy/store/generated_key/eigenda"
	"github.com/Layr-Labs/eigenda-proxy/store/generated_key/memstore"
	"github.com/Layr-Labs/eigenda-proxy/store/generated_key/memstore/memconfig"
	memstorev2 "github.com/Layr-Labs/eigenda-proxy/store/generated_key/memstore/v2"
	eigendav2 "github.com/Layr-Labs/eigenda-proxy/store/generated_key/v2"
	"github.com/Layr-Labs/eigenda-proxy/store/precomputed_key/redis"
	"github.com/Layr-Labs/eigenda-proxy/store/precomputed_key/s3"
	"github.com/Layr-Labs/eigenda-proxy/verify/v1"
	"github.com/Layr-Labs/eigenda/api/clients"
	clients_v2 "github.com/Layr-Labs/eigenda/api/clients/v2"
	"github.com/Layr-Labs/eigenda/api/clients/v2/verification"

	"github.com/Layr-Labs/eigenda/common/geth"
	eigenda_eth "github.com/Layr-Labs/eigenda/core/eth"
	"github.com/Layr-Labs/eigenda/encoding/kzg"
	"github.com/Layr-Labs/eigensdk-go/logging"
	geth_common "github.com/ethereum/go-ethereum/common"
)

// Builder centralizes dependency initialization
// It ensures proper typing and avoids redundant logic scattered across functions.
type Builder struct {
	ctx     context.Context
	log     logging.Logger
	metrics metrics.Metricer

	memConfig *memconfig.SafeConfig

	// configs
	managerCfg     Config
	v1VerifierCfg  verify.Config
	v1EdaClientCfg clients.EigenDAClientConfig
	v2ClientCfg    common.V2ClientConfig
}

func NewBuilder(ctx context.Context, cfg Config,
	v1VerifierCfg verify.Config,
	v1EdaClientCfg clients.EigenDAClientConfig,
	v2ClientCfg common.V2ClientConfig,
	memConfig *memconfig.SafeConfig,
	log logging.Logger, metrics metrics.Metricer) *Builder {
	return &Builder{ctx, log, metrics, memConfig, cfg, v1VerifierCfg, v1EdaClientCfg, v2ClientCfg}
}

// buildSecondaries ... Creates a slice of secondary targets used for either read
// failover or caching
func (d *Builder) buildSecondaries(targets []string, s3Store common.PrecomputedKeyStore, redisStore *redis.Store) []common.PrecomputedKeyStore {
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

// buildEigenDAV2Backend ... Builds EigenDA V2 storage backend
func (d *Builder) buildEigenDAV2Backend(maxBlobSizeBytes uint) (common.GeneratedKeyStore, error) {
	// TODO: Figure out how to better manage the v1 verifier
	// may make sense to live in some global kzg config that's passed
	// down across EigenDA versions
	g1Points, err := kzg.ReadG1Points(
		d.v1VerifierCfg.KzgConfig.G1Path,
		d.v1VerifierCfg.KzgConfig.SRSNumberToLoad, 4)
	if err != nil {
		return nil, err
	}

	if d.memConfig != nil {
		return memstorev2.New(d.ctx, d.log, d.memConfig, g1Points)
	}

	gethCfg := geth.EthClientConfig{
		RPCURLs: []string{d.v1EdaClientCfg.EthRpcUrl},
	}

	ethClient, err := geth.NewClient(gethCfg, geth_common.Address{}, 0, d.log)
	if err != nil {
		return nil, err
	}

	// initialize eth reader object to fetch relay urls from
	// onchain EigenDARelayRegistry contract
	reader, err := eigenda_eth.NewReader(d.log, ethClient, "0x0", d.v2ClientCfg.ServiceManagerAddress)
	if err != nil {
		return nil, err
	}

	relayURLs, err := reader.GetRelayURLs(d.ctx)
	if err != nil {
		return nil, err
	}

	relayCfg := clients_v2.RelayClientConfig{
		UseSecureGrpcFlag: d.v2ClientCfg.DisperserClientCfg.UseSecureGrpcFlag,
		// we should never expect a message greater than our allowed max blob size.
		// 10% of max blob size is added for additional safety
		MaxGRPCMessageSize: maxBlobSizeBytes + (maxBlobSizeBytes / 10),
		Sockets:            relayURLs,
	}

	retriever, err := clients_v2.BuildRelayPayloadRetriever(d.log, d.v2ClientCfg.RetrievalConfig, &relayCfg, g1Points)
	if err != nil {
		return nil, err
	}

	// TODO: https://github.com/Layr-Labs/eigenda-proxy/issues/274
	// ^ why prover/encoder fields are nil'd out
	disperser, err := clients_v2.BuildPayloadDisperser(d.log, d.v2ClientCfg.PayloadClientCfg, &d.v2ClientCfg.DisperserClientCfg, &gethCfg, nil, nil)
	if err != nil {
		return nil, err
	}

	verifier, err := verification.NewCertVerifier(d.log, ethClient, time.Second*1)
	if err != nil {
		return nil, err
	}

	return eigendav2.NewStore(d.log, &eigendav2.Config{
		CertVerifierAddress: d.v2ClientCfg.PayloadClientCfg.EigenDACertVerifierAddr,
		MaxBlobSizeBytes:    uint64(maxBlobSizeBytes),
		PutRetries:          d.v2ClientCfg.PutRetries,
	}, disperser, retriever, verifier)
}

// buildEigenDAV1Backend ... Builds EigenDA V1 storage backend
func (d *Builder) buildEigenDAV1Backend(ctx context.Context, putRetries uint, maxBlobSize uint) (common.GeneratedKeyStore, error) {
	verifier, err := verify.NewVerifier(&d.v1VerifierCfg, d.log)
	if err != nil {
		return nil, fmt.Errorf("failed to create verifier: %w", err)
	}

	if d.v1VerifierCfg.VerifyCerts {
		d.log.Info("Certificate verification with Ethereum enabled")
	} else {
		d.log.Warn("Certificate verification disabled. This can result in invalid EigenDA certificates being accredited.")
	}

	if d.memConfig != nil {
		d.log.Info("Using memstore backend for EigenDA V1")
		return memstore.New(ctx, verifier, d.log, d.memConfig)
	}
	// EigenDAV1 backend dependency injection
	var client *clients.EigenDAClient
	d.log.Warn("Using EigenDA backend.. This backend type will be deprecated soon. Please migrate to V2.")
	client, err = clients.NewEigenDAClient(d.log, d.v1EdaClientCfg)
	if err != nil {
		return nil, err
	}

	return eigenda.NewStore(
		client,
		verifier,
		d.log,
		&eigenda.StoreConfig{
			MaxBlobSizeBytes:     uint64(maxBlobSize),
			EthConfirmationDepth: d.v1VerifierCfg.EthConfirmationDepth,
			StatusQueryTimeout:   d.v1EdaClientCfg.StatusQueryTimeout,
			PutRetries:           putRetries,
		},
	)

}

// BuildManager ... Builds storage manager object
func (d *Builder) BuildManager(ctx context.Context, putRetries uint, maxBlobSize uint) (IManager, error) {
	var err error
	var s3Store *s3.Store
	var redisStore *redis.Store
	var eigenDAV1Store, eigenDAV2Store common.GeneratedKeyStore

	if d.managerCfg.S3Config.Bucket != "" {
		d.log.Debug("Using S3 storage backend")
		s3Store, err = s3.NewStore(d.managerCfg.S3Config)
	}

	if err != nil {
		return nil, err
	}

	if d.managerCfg.RedisConfig.Endpoint != "" {
		d.log.Debug("Using Redis storage backend")
		redisStore, err = redis.NewStore(&d.managerCfg.RedisConfig)
	}

	if err != nil {
		return nil, err
	}

	if d.v2ClientCfg.Enabled {
		d.log.Debug("Using EigenDA V2 storage backend")
		eigenDAV2Store, err = d.buildEigenDAV2Backend(maxBlobSize)
		if err != nil {
			return nil, err
		}
	}

	eigenDAV1Store, err = d.buildEigenDAV1Backend(ctx, putRetries, maxBlobSize)
	if err != nil {
		return nil, err
	}

	fallbacks := d.buildSecondaries(d.managerCfg.FallbackTargets, s3Store, redisStore)
	caches := d.buildSecondaries(d.managerCfg.CacheTargets, s3Store, redisStore)
	secondary := NewSecondaryManager(d.log, d.metrics, caches, fallbacks)

	if secondary.Enabled() { // only spin-up go routines if secondary storage is enabled
		d.log.Debug("Starting secondary write loop(s)", "count", d.managerCfg.AsyncPutWorkers)

		for i := 0; i < d.managerCfg.AsyncPutWorkers; i++ {
			go secondary.WriteSubscriptionLoop(ctx)
		}
	}

	d.log.Info("Created storage backends",
		"eigenda_v1", eigenDAV1Store != nil,
		"eigenda_v2", eigenDAV2Store != nil,
		"s3", s3Store != nil,
		"redis", redisStore != nil,
		"read_fallback", len(fallbacks) > 0,
		"caching", len(caches) > 0,
		"async_secondary_writes", (secondary.Enabled() && d.managerCfg.AsyncPutWorkers > 0),
		"verify_v1_certs", d.v1VerifierCfg.VerifyCerts,
	)
	return NewManager(eigenDAV1Store, eigenDAV2Store, s3Store, d.log, secondary, d.v2ClientCfg.Enabled)
}
