package store

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/Layr-Labs/eigenda-proxy/common"
	"github.com/Layr-Labs/eigenda-proxy/metrics"
	"github.com/Layr-Labs/eigenda-proxy/store/generated_key/eigenda"
	"github.com/Layr-Labs/eigenda-proxy/store/generated_key/memstore"
	"github.com/Layr-Labs/eigenda-proxy/store/generated_key/memstore/memconfig"
	memstore_v2 "github.com/Layr-Labs/eigenda-proxy/store/generated_key/memstore/v2"
	eigenda_v2 "github.com/Layr-Labs/eigenda-proxy/store/generated_key/v2"
	"github.com/Layr-Labs/eigenda-proxy/store/precomputed_key/redis"
	"github.com/Layr-Labs/eigenda-proxy/store/precomputed_key/s3"
	"github.com/Layr-Labs/eigenda-proxy/verify/v1"
	"github.com/Layr-Labs/eigenda/api/clients"
	clients_v2 "github.com/Layr-Labs/eigenda/api/clients/v2"
	"github.com/Layr-Labs/eigenda/api/clients/v2/relay"
	"github.com/Layr-Labs/eigenda/api/clients/v2/verification"
	common_eigenda "github.com/Layr-Labs/eigenda/common"
	"github.com/Layr-Labs/eigenda/common/geth"
	auth "github.com/Layr-Labs/eigenda/core/auth/v2"
	eigenda_eth "github.com/Layr-Labs/eigenda/core/eth"
	core_v2 "github.com/Layr-Labs/eigenda/core/v2"
	"github.com/Layr-Labs/eigenda/encoding/kzg/prover"
	"github.com/Layr-Labs/eigensdk-go/logging"
	"github.com/consensys/gnark-crypto/ecc/bn254"
	geth_common "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
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

func NewBuilder(
	ctx context.Context, cfg Config,
	v1VerifierCfg verify.Config,
	v1EdaClientCfg clients.EigenDAClientConfig,
	v2ClientCfg common.V2ClientConfig,
	memConfig *memconfig.SafeConfig,
	log logging.Logger, metrics metrics.Metricer,
) *Builder {
	return &Builder{ctx, log, metrics, memConfig, cfg, v1VerifierCfg, v1EdaClientCfg, v2ClientCfg}
}

// buildSecondaries ... Creates a slice of secondary targets used for either read
// failover or caching
func (d *Builder) buildSecondaries(
	targets []string,
	s3Store common.PrecomputedKeyStore,
	redisStore *redis.Store,
) []common.PrecomputedKeyStore {
	stores := make([]common.PrecomputedKeyStore, len(targets))

	for i, target := range targets {
		//nolint:exhaustive // TODO: implement additional secondaries
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
func (d *Builder) buildEigenDAV2Backend(
	ctx context.Context,
	maxBlobSizeBytes uint,
	eigenDACertVerifierAddress string,
) (common.GeneratedKeyStore, error) {
	// TODO: Figure out how to better manage the v1 verifier
	//  may make sense to live in some global kzg config that's passed down across EigenDA versions
	kzgProver, err := prover.NewProver(d.v1VerifierCfg.KzgConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("new KZG prover: %w", err)
	}

	if d.memConfig != nil {
		return memstore_v2.New(d.ctx, d.log, d.memConfig, kzgProver.Srs.G1)
	}

	ethClient, err := d.buildEthClient()
	if err != nil {
		return nil, fmt.Errorf("build eth client: %w", err)
	}

	relayPayloadRetriever, err := d.buildRelayPayloadRetriever(ethClient, maxBlobSizeBytes, kzgProver.Srs.G1)
	if err != nil {
		return nil, fmt.Errorf("build relay payload retriever: %w", err)
	}

	certVerifier, err := verification.NewCertVerifier(
		d.log, ethClient, d.v2ClientCfg.PayloadDisperserCfg.BlockNumberPollInterval)
	if err != nil {
		return nil, fmt.Errorf("new cert verifier: %w", err)
	}

	payloadDisperser, err := d.buildPayloadDisperser(ctx, ethClient, kzgProver, certVerifier)
	if err != nil {
		return nil, fmt.Errorf("build payload disperser: %w", err)
	}

	v2Config := &eigenda_v2.Config{
		CertVerifierAddress: eigenDACertVerifierAddress,
		MaxBlobSizeBytes:    uint64(maxBlobSizeBytes),
		PutRetries:          d.v2ClientCfg.PutRetries,
	}

	return eigenda_v2.NewStore(d.log, v2Config, payloadDisperser, relayPayloadRetriever, certVerifier)
}

// buildEigenDAV1Backend ... Builds EigenDA V1 storage backend
func (d *Builder) buildEigenDAV1Backend(
	ctx context.Context,
	putRetries uint,
	maxBlobSize uint) (common.GeneratedKeyStore, error) {
	verifier, err := verify.NewVerifier(&d.v1VerifierCfg, d.log)
	if err != nil {
		return nil, fmt.Errorf("new verifier: %w", err)
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
func (d *Builder) BuildManager(
	ctx context.Context,
	putRetries uint,
	maxBlobSize uint,
	eigenDaCertVerifierAddress string,
) (IManager, error) {
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
		eigenDAV2Store, err = d.buildEigenDAV2Backend(ctx, maxBlobSize, eigenDaCertVerifierAddress)
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

	d.log.Info(
		"Created storage backends",
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

func (d *Builder) buildEthClient() (common_eigenda.EthClient, error) {
	gethCfg := geth.EthClientConfig{
		RPCURLs: []string{d.v1EdaClientCfg.EthRpcUrl},
	}

	ethClient, err := geth.NewClient(gethCfg, geth_common.Address{}, 0, d.log)
	if err != nil {
		return nil, fmt.Errorf("create geth client: %w", err)
	}

	return ethClient, nil
}

func (d *Builder) buildRelayPayloadRetriever(
	ethClient common_eigenda.EthClient,
	maxBlobSizeBytes uint,
	g1Srs []bn254.G1Affine,
) (*clients_v2.RelayPayloadRetriever, error) {
	relayClient, err := d.buildRelayClient(ethClient, maxBlobSizeBytes)
	if err != nil {
		return nil, fmt.Errorf("build relay client: %w", err)
	}

	relayPayloadRetriever, err := clients_v2.NewRelayPayloadRetriever(
		d.log,
		//nolint:gosec // disable G404: this doesn't need to be cryptographically secure
		rand.New(rand.NewSource(time.Now().UnixNano())),
		d.v2ClientCfg.RelayPayloadRetrieverCfg,
		relayClient,
		g1Srs)
	if err != nil {
		return nil, fmt.Errorf("new relay payload retriever: %w", err)
	}

	return relayPayloadRetriever, nil
}

func (d *Builder) buildRelayClient(
	ethClient common_eigenda.EthClient,
	maxBlobSizeBytes uint) (clients_v2.RelayClient, error) {
	reader, err := eigenda_eth.NewReader(d.log, ethClient, "0x0", d.v2ClientCfg.ServiceManagerAddress)
	if err != nil {
		return nil, fmt.Errorf("new eth reader: %w", err)
	}

	relayURLProvider, err := relay.NewRelayUrlProvider(ethClient, reader.GetRelayRegistryAddress())
	if err != nil {
		return nil, fmt.Errorf("new relay url provider: %w", err)
	}

	relayCfg := &clients_v2.RelayClientConfig{
		UseSecureGrpcFlag: d.v2ClientCfg.DisperserClientCfg.UseSecureGrpcFlag,
		// we should never expect a message greater than our allowed max blob size.
		// 10% of max blob size is added for additional safety
		MaxGRPCMessageSize: maxBlobSizeBytes + (maxBlobSizeBytes / 10),
	}

	relayClient, err := clients_v2.NewRelayClient(relayCfg, d.log, relayURLProvider)
	if err != nil {
		return nil, fmt.Errorf("new relay client: %w", err)
	}

	return relayClient, nil
}

func (d *Builder) buildPayloadDisperser(
	ctx context.Context,
	ethClient common_eigenda.EthClient,
	kzgProver *prover.Prover,
	certVerifier *verification.CertVerifier,
) (*clients_v2.PayloadDisperser, error) {
	signer, err := d.buildLocalSigner(ctx, ethClient)
	if err != nil {
		return nil, fmt.Errorf("build local signer: %w", err)
	}

	disperserClient, err := clients_v2.NewDisperserClient(&d.v2ClientCfg.DisperserClientCfg, signer, kzgProver, nil)
	if err != nil {
		return nil, fmt.Errorf("new disperser client: %w", err)
	}

	payloadDisperser, err := clients_v2.NewPayloadDisperser(
		d.log,
		d.v2ClientCfg.PayloadDisperserCfg,
		disperserClient,
		certVerifier)
	if err != nil {
		return nil, fmt.Errorf("new payload disperser: %w", err)
	}

	return payloadDisperser, nil
}

// buildLocalSigner attempts to check the pending balance of the created signer account. If the check fails, or if the
// balance is determined to be 0, the user is warned with a log. This method doesn't return an error based on this check:
// it's possible that a user could want to set up a signer before it's actually ready to be used
func (d *Builder) buildLocalSigner(
	ctx context.Context,
	ethClient common_eigenda.EthClient,
) (core_v2.BlobRequestSigner, error) {
	signer, err := auth.NewLocalBlobRequestSigner(d.v2ClientCfg.PayloadDisperserCfg.SignerPaymentKey)
	if err != nil {
		return nil, fmt.Errorf("new local blob request signer: %w", err)
	}

	accountID := crypto.PubkeyToAddress(signer.PrivateKey.PublicKey)
	pendingBalance, err := ethClient.PendingBalanceAt(ctx, accountID)

	switch {
	case err != nil:
		d.log.Errorf("get pending balance for accountID %v: %v", accountID, err)
	case pendingBalance == nil:
		d.log.Errorf(
			"get pending balance for accountID %v didn't return an error, but pending balance is nil", accountID)
	case pendingBalance.Sign() <= 0:
		d.log.Warnf("pending balance for accountID %v is zero", accountID)
	}

	return signer, nil
}
