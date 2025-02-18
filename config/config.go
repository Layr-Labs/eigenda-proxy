package config

import (
	"fmt"

	"github.com/Layr-Labs/eigenda-proxy/common"
	"github.com/Layr-Labs/eigenda-proxy/config/eigendaflags"
	eigendaflags_v2 "github.com/Layr-Labs/eigenda-proxy/config/eigendaflags/v2"
	"github.com/Layr-Labs/eigenda-proxy/metrics"
	"github.com/Layr-Labs/eigenda-proxy/server"
	"github.com/Layr-Labs/eigenda-proxy/store"
	"github.com/Layr-Labs/eigenda-proxy/store/generated_key/memstore"
	"github.com/Layr-Labs/eigenda-proxy/verify/v1"
	"github.com/urfave/cli/v2"

	"github.com/Layr-Labs/eigenda/api/clients"
	clients_v2 "github.com/Layr-Labs/eigenda/api/clients/v2"
)

// AppConfig ... Highest order config. Stores
// all relevant fields necessary for running both proxy
// & metrics servers.
type AppConfig struct {
	EigenDAConfig ProxyConfig
	MetricsCfg    metrics.Config
}

func (c AppConfig) Check() error {
	err := c.EigenDAConfig.Check()
	if err != nil {
		return err
	}
	return nil
}

func ReadCLIConfig(ctx *cli.Context) AppConfig {
	return AppConfig{
		EigenDAConfig: ReadProxyConfig(ctx),
		MetricsCfg:    metrics.ReadConfig(ctx),
	}
}

// ProxyConfig ... Higher order config which bundles all configs for instrumenting
// the proxy server with necessary client context
type ProxyConfig struct {
	ServerConfig        server.Config
	EdaV1ClientConfig   clients.EigenDAClientConfig
	EdaV1VerifierConfig verify.Config

	EdaV2ClientConfig common.V2ClientConfig

	MemstoreConfig memstore.Config
	StorageConfig  store.Config
	PutRetries     uint

	MemstoreEnabled bool

	EigenDAV2Enabled bool
}

type V2ClientConfig struct {
	DisperserClientCfg clients_v2.DisperserClientConfig
	PayloadClientCfg   clients_v2.PayloadDisperserConfig
	RetrievalConfig    clients_v2.RelayPayloadRetrieverConfig
}

// ReadProxyConfig ... parses the Config from the provided flags or environment variables.
func ReadProxyConfig(ctx *cli.Context) ProxyConfig {
	edaClientV1Config := eigendaflags.ReadConfig(ctx)
	edaClientV2Config := eigendaflags_v2.ReadConfig(ctx)

	cfg := ProxyConfig{
		ServerConfig: server.Config{
			DisperseV2: edaClientV2Config.Enabled,
			Host:       ctx.String(ListenAddrFlagName),
			Port:       ctx.Int(PortFlagName),
		},
		EdaV1ClientConfig:   edaClientV1Config,
		EdaV1VerifierConfig: verify.ReadConfig(ctx, edaClientV1Config),
		PutRetries:          ctx.Uint(eigendaflags.PutRetriesFlagName),
		MemstoreEnabled:     ctx.Bool(memstore.EnabledFlagName),
		MemstoreConfig:      memstore.ReadConfig(ctx),
		StorageConfig:       store.ReadConfig(ctx),
		EigenDAV2Enabled:    edaClientV2Config.Enabled,
	}

	return cfg
}

// Check ... verifies that configuration values are adequately set
func (cfg *ProxyConfig) Check() error {
	if !cfg.MemstoreEnabled {
		if cfg.EdaV1ClientConfig.RPC == "" {
			return fmt.Errorf("using eigenda backend (memstore.enabled=false) but eigenda disperser rpc url is not set")
		}
	}

	// provide dummy values to eigenda client config. Since the client won't be called in this
	// mode it doesn't matter.
	if cfg.MemstoreEnabled {
		cfg.EdaV1ClientConfig.SvcManagerAddr = "0x0000000000000000000000000000000000000000"
		cfg.EdaV1ClientConfig.EthRpcUrl = "http://0.0.0.0:666"
	}

	if !cfg.MemstoreEnabled {
		if cfg.EdaV1ClientConfig.SvcManagerAddr == "" {
			return fmt.Errorf("service manager address is required for communication with EigenDA")
		}
		if cfg.EdaV1ClientConfig.EthRpcUrl == "" {
			return fmt.Errorf("eth prc url is required for communication with EigenDA")
		}
	}

	// cert verification is enabled
	// TODO: move this verification logic to verify/cli.go
	if cfg.EdaV1VerifierConfig.VerifyCerts {
		if cfg.MemstoreEnabled {
			return fmt.Errorf("cannot enable cert verification when memstore is enabled. use --%s", verify.CertVerificationDisabledFlagName)
		}
		if cfg.EdaV1VerifierConfig.RPCURL == "" {
			return fmt.Errorf("cert verification enabled but eth rpc is not set")
		}
		if cfg.EdaV1VerifierConfig.SvcManagerAddr == "" {
			return fmt.Errorf("cert verification enabled but svc manager address is not set")
		}
	}

	// V2 dispersal/retrieval enabled
	if cfg.EigenDAV2Enabled {

		if cfg.EdaV1ClientConfig.SvcManagerAddr == "" {
			return fmt.Errorf("service manager address is required for interacting with EigenDA V2")
		}

		if cfg.EdaV1ClientConfig.EthRpcUrl == "" {
			return fmt.Errorf("eth rpc is required for interacting with EigenDA V2")
		}

		if cfg.EdaV2ClientConfig.ServiceManagerAddress == "" {
			return fmt.Errorf("cert verifier contract address is required for interacting with EigenDA V2")
		}
	}

	return cfg.StorageConfig.Check()
}
