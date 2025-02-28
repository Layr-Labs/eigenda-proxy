package common

import (
	"fmt"
)

// SecretConfigV2 contains sensitive config data that must be protected from leakage
type SecretConfigV2 struct {
	// SignerPaymentKey is the hex representation of the private payment key, that pays for payload dispersal
	SignerPaymentKey string
	// EthRPCUrl is the URL of the eth node. This is included in Secrets since RPC providers typically use sensitive
	// API keys within
	EthRPCUrl string
}

// Check checks config invariants, and returns an error if there is a problem with the config struct
func (s *SecretConfigV2) Check() error {
	if s.EthRPCUrl == "" {
		return fmt.Errorf("eth rpc is required for using EigenDA V2 backend")
	}

	if s.SignerPaymentKey == "" {
		return fmt.Errorf("signer payment private key is required for using EigenDA V2 backend")
	}

	return nil
}
