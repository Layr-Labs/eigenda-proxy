package common

import (
	"fmt"
	"time"

	clients_v2 "github.com/Layr-Labs/eigenda/api/clients/v2"
)

// ClientConfigV2 contains all non-sensitive configuration to construct V2 clients
type ClientConfigV2 struct {
	Enabled                         bool
	DisperserClientCfg              clients_v2.DisperserClientConfig
	PayloadDisperserCfg             clients_v2.PayloadDisperserConfig
	RelayPayloadRetrieverCfg        clients_v2.RelayPayloadRetrieverConfig
	ServiceManagerAddress           string
	PutRetries                      uint
	BlockNumberPollIntervalDuration time.Duration
}

// Check checks config invariants, and returns an error if there is a problem with the config struct
func (cfg *ClientConfigV2) Check() error {
	if cfg.ServiceManagerAddress == "" {
		return fmt.Errorf("service manager address is required for using EigenDA V2 backend")
	}

	if cfg.DisperserClientCfg.Hostname == "" {
		return fmt.Errorf("disperser hostname is required for using EigenDA V2 backend")
	}

	return nil
}
