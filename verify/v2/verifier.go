package verify

import (
	"github.com/Layr-Labs/eigenda/api/clients/v2/verification"
)

type Config struct {
	ethRPCURl           string
	blobVerifierAddress string
}

type Verifier struct {
	blobVerifier verification.BlobVerifier
}

func NewVerifier(cfg *Config) (*Verifier, error) {
	// ethClient, err := ethclient.Dial(cfg.ethRPCURl)
	// if err != nil {
	// 	return nil, err
	// }

	// v, err := verification.NewBlobVerifier(ethClient, cfg.blobVerifierAddress)

	return nil, nil
}
