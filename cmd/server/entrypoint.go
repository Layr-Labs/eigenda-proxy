package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Layr-Labs/eigenda-proxy/config"

	proxy_logging "github.com/Layr-Labs/eigenda-proxy/logging"
	"github.com/Layr-Labs/eigenda-proxy/store"
	"github.com/Layr-Labs/eigensdk-go/logging"

	"github.com/Layr-Labs/eigenda-proxy/metrics"
	"github.com/Layr-Labs/eigenda-proxy/server"
	"github.com/urfave/cli/v2"

	"github.com/ethereum-optimism/optimism/op-service/ctxinterrupt"
)

func StartProxySvr(cliCtx *cli.Context) error {
	logCfg, err := proxy_logging.ReadLoggerCLIConfig(cliCtx)
	if err != nil {
		return err
	}

	log, err := proxy_logging.NewLogger(*logCfg)
	if err != nil {
		return err
	}

	log.Info("Starting EigenDA Proxy Server", "version", Version, "date", Date, "commit", Commit)

	cfg := config.ReadCLIConfig(cliCtx)
	if err := cfg.Check(); err != nil {
		return err
	}
	err = prettyPrintConfig(cliCtx, log)
	if err != nil {
		return fmt.Errorf("failed to pretty print config: %w", err)
	}

	m := metrics.NewMetrics("default")

	ctx, ctxCancel := context.WithCancel(cliCtx.Context)
	defer ctxCancel()

	sm, err := store.NewStoreLoader(ctx, cfg.EigenDAConfig.StorageConfig,
		cfg.EigenDAConfig.EdaV1VerifierConfig, cfg.EigenDAConfig.EdaV1ClientConfig,
		cfg.EigenDAConfig.EdaV2ClientConfig, log, m).LoadManager()
	if err != nil {
		return fmt.Errorf("failed to create store: %w", err)
	}
	server := server.NewServer(&cfg.EigenDAConfig.ServerConfig, sm, log, m)

	if err := server.Start(); err != nil {
		return fmt.Errorf("failed to start the DA server: %w", err)
	}

	log.Info("Started EigenDA proxy server")

	defer func() {
		if err := server.Stop(); err != nil {
			log.Error("failed to stop DA server", "err", err)
		}

		log.Info("successfully shutdown API server")
	}()

	if cfg.MetricsCfg.Enabled {
		log.Debug("starting metrics server", "addr", cfg.MetricsCfg.Host, "port", cfg.MetricsCfg.Port)
		svr, err := m.StartServer(cfg.MetricsCfg.Host, cfg.MetricsCfg.Port)
		if err != nil {
			return fmt.Errorf("failed to start metrics server: %w", err)
		}
		defer func() {
			if err := svr.Stop(context.Background()); err != nil {
				log.Error("failed to stop metrics server", "err", err)
			}
		}()
		log.Info("started metrics server", "addr", svr.Addr())
		m.RecordUp()
	}

	return ctxinterrupt.Wait(cliCtx.Context)
}

// TODO: we should probably just change EdaClientConfig struct definition in eigenda-client
func prettyPrintConfig(cliCtx *cli.Context, log logging.Logger) error {
	// we read a new config which we modify to hide private info in order to log the rest
	cfg := config.ReadCLIConfig(cliCtx)
	if cfg.EigenDAConfig.EdaV1ClientConfig.SignerPrivateKeyHex != "" {
		cfg.EigenDAConfig.EdaV1ClientConfig.SignerPrivateKeyHex = "*****" // marshaling defined in client config
	}
	if cfg.EigenDAConfig.EdaV1ClientConfig.EthRpcUrl != "" {
		cfg.EigenDAConfig.EdaV1ClientConfig.EthRpcUrl = "*****" // hiding as RPC providers typically use sensitive API keys within
	}

	configJSON, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	log.Info(fmt.Sprintf("Initializing EigenDA proxy server with config (\"*****\" fields are hidden): %v", string(configJSON)))
	return nil
}
