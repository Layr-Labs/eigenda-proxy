package server

import (
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/Layr-Labs/eigenda-proxy/flags"
	"github.com/Layr-Labs/eigenda-proxy/flags/eigendaflags"
	"github.com/Layr-Labs/eigenda-proxy/store"
	"github.com/Layr-Labs/eigenda-proxy/store/generated_key/memstore"
	"github.com/Layr-Labs/eigenda-proxy/verify/v1"
	"github.com/Layr-Labs/eigenda/api/clients"

	clients_v2 "github.com/Layr-Labs/eigenda/api/clients/v2"

	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
)

type Config struct {
	Host string
	Port int

	EdaV1ClientConfig clients.EigenDAClientConfig
	MemstoreConfig    memstore.Config
	StorageConfig     store.Config
	VerifierConfig    verify.Config
	PutRetries        uint

	MemstoreEnabled bool

	EigenDAV2Enabled  bool
	V2DispersalConfig clients_v2.PayloadDisperserConfig
	V2RetrievalConfig clients_v2.RelayPayloadRetrieverConfig
}

// ReadConfig ... parses the Config from the provided flags or environment variables.
func ReadConfig(ctx *cli.Context) Config {
	edaClientConfig := eigendaflags.ReadV1ClientConfig(ctx)

	v2Enabled := ctx.Bool(eigendaflags.V2Enabled)

	cfg := Config{
		Host:              ctx.String(flags.ListenAddrFlagName),
		Port:              ctx.Int(flags.PortFlagName),
		EdaV1ClientConfig: edaClientConfig,
		VerifierConfig:    verify.ReadConfig(ctx, edaClientConfig),
		PutRetries:        ctx.Uint(eigendaflags.PutRetriesFlagName),
		MemstoreEnabled:   ctx.Bool(memstore.EnabledFlagName),
		MemstoreConfig:    memstore.ReadConfig(ctx),
		StorageConfig:     store.ReadConfig(ctx),
		EigenDAV2Enabled:  ctx.Bool(eigendaflags.V2Enabled),
	}

	if v2Enabled {
		cfg.V2DispersalConfig = eigendaflags.ReadV2DispersalConfig(ctx)
		cfg.V2RetrievalConfig = eigendaflags.ReadV2RetrievalConfig(ctx)
	}

	return cfg
}

// Check ... verifies that configuration values are adequately set
func (cfg *Config) Check() error {
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
	if cfg.VerifierConfig.VerifyCerts {
		if cfg.MemstoreEnabled {
			return fmt.Errorf("cannot enable cert verification when memstore is enabled. use --%s", verify.CertVerificationDisabledFlagName)
		}
		if cfg.VerifierConfig.RPCURL == "" {
			return fmt.Errorf("cert verification enabled but eth rpc is not set")
		}
		if cfg.VerifierConfig.SvcManagerAddr == "" {
			return fmt.Errorf("cert verification enabled but svc manager address is not set")
		}
	}

	// V2 dispersal/retrieval enabled
	if cfg.EigenDAV2Enabled {
		dc := cfg.V2DispersalConfig

		if dc.EthRpcUrl == "" {
			return fmt.Errorf("eth rpc is required for interacting with EigenDA V2")
		}

		if dc.EigenDACertVerifierAddr == "" {
			return fmt.Errorf("cert verifier contract address is required for interacting with EigenDA V2")
		}

	}

	return cfg.StorageConfig.Check()
}

type CLIConfig struct {
	EigenDAConfig Config
	MetricsCfg    opmetrics.CLIConfig
}

func ReadCLIConfig(ctx *cli.Context) CLIConfig {
	config := ReadConfig(ctx)
	return CLIConfig{
		EigenDAConfig: config,
		MetricsCfg:    opmetrics.ReadCLIConfig(ctx),
	}
}

func (c CLIConfig) Check() error {
	err := c.EigenDAConfig.Check()
	if err != nil {
		return err
	}
	return nil
}
