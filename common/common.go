package common

import (
	"fmt"
	"strconv"
	"strings"

	clients_v2 "github.com/Layr-Labs/eigenda/api/clients/v2"
	"github.com/Layr-Labs/eigenda/core"
)

var (
	DefaultQuorums = []core.QuorumID{0, 1}
)

type V2ClientConfig struct {
	Enabled               bool
	DisperserClientCfg    clients_v2.DisperserClientConfig
	PayloadClientCfg      clients_v2.PayloadDisperserConfig
	RetrievalConfig       clients_v2.RelayPayloadRetrieverConfig
	ServiceManagerAddress string
	EthRPC                string
	PutRetries            uint
}

func (cfg *V2ClientConfig) Check() error {
	if cfg.ServiceManagerAddress == "" {
		return fmt.Errorf("service manager address is required for using EigenDA V2 backend")
	}

	if cfg.EthRPC == "" {
		return fmt.Errorf("eth rpc is required for using EigenDA V2 backend")
	}

	if cfg.PayloadClientCfg.EigenDACertVerifierAddr == "" {
		return fmt.Errorf("cert verifier contract address is required for using EigenDA V2 backend")
	}

	if cfg.PayloadClientCfg.SignerPaymentKey == "" {
		return fmt.Errorf("signer payment private key hex is required for using EigenDA V2 backend")
	}

	if cfg.DisperserClientCfg.Hostname == "" {
		return fmt.Errorf("disperser hostname is required for using EigenDA V2 backend")
	}

	return nil
}

const GlobalPrefix = "EIGENDA_PROXY"

func PrefixEnvVar(prefix, suffix string) []string {
	return []string{prefix + "_" + suffix}
}

// Helper utility functions //

func ContainsDuplicates[P comparable](s []P) bool {
	seen := make(map[P]struct{})
	for _, v := range s {
		if _, ok := seen[v]; ok {
			return true
		}
		seen[v] = struct{}{}
	}
	return false
}

func Contains[P comparable](s []P, e P) bool {
	for _, v := range s {
		if v == e {
			return true
		}
	}
	return false
}

func ParseBytesAmount(s string) (uint64, error) {
	s = strings.TrimSpace(s)

	// Extract numeric part and unit
	numStr := s
	unit := ""
	for i, r := range s {
		if !('0' <= r && r <= '9' || r == '.') {
			numStr = s[:i]
			unit = s[i:]
			break
		}
	}

	// Convert numeric part to float64
	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid numeric value: %w", err)
	}

	unit = strings.ToLower(strings.TrimSpace(unit))

	// Convert to uint64 based on the unit (case-insensitive)
	switch unit {
	case "b", "":
		return uint64(num), nil
	case "kib":
		return uint64(num * 1024), nil
	case "kb":
		return uint64(num * 1000), nil // Decimal kilobyte
	case "mib":
		return uint64(num * 1024 * 1024), nil
	case "mb":
		return uint64(num * 1000 * 1000), nil // Decimal megabyte
	case "gib":
		return uint64(num * 1024 * 1024 * 1024), nil
	case "gb":
		return uint64(num * 1000 * 1000 * 1000), nil // Decimal gigabyte
	case "tib":
		return uint64(num * 1024 * 1024 * 1024 * 1024), nil
	case "tb":
		return uint64(num * 1000 * 1000 * 1000 * 1000), nil // Decimal terabyte
	default:
		return 0, fmt.Errorf("unsupported unit: %s", unit)
	}
}
