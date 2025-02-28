package config

import (
	"github.com/Layr-Labs/eigenda-proxy/metrics"
	"github.com/urfave/cli/v2"
)

// AppConfig ... Highest order config. Stores
// all relevant fields necessary for running both proxy
// & metrics servers.
type AppConfig struct {
	EigenDAConfig ProxyConfig
	MetricsCfg    metrics.Config
}

// Check checks config invariants, and returns an error if there is a problem with the config struct
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
