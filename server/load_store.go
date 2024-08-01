package server

import (
	"context"
	"fmt"

	"github.com/Layr-Labs/eigenda-proxy/store"
	"github.com/Layr-Labs/eigenda-proxy/verify"
	"github.com/Layr-Labs/eigenda/api/clients"
	"github.com/ethereum/go-ethereum/log"
)

func LoadStoreRouter(cfg CLIConfig, ctx context.Context, log log.Logger) (*store.Router, error) {
	var err error
	var s3 *store.S3Store
	if cfg.S3Config.Bucket != "" && cfg.S3Config.Endpoint != "" {
		log.Info("Using S3 backend")
		s3, err = store.NewS3(cfg.S3Config)
		if err != nil {
			return nil, err
		}
	}

	daCfg := cfg.EigenDAConfig
	vCfg := daCfg.VerificationCfg()

	if cfg.EigenDAConfig.MemstoreEnabled {
		vCfg.Verify = false
	}

	verifier, err := verify.NewVerifier(vCfg, log)
	if err != nil {
		return nil, err
	}

	maxBlobLength, err := daCfg.GetMaxBlobLength()
	if err != nil {
		return nil, err
	}
	var memstore *store.MemStore
	if cfg.EigenDAConfig.MemstoreEnabled {
		faultMode := cfg.EigenDAConfig.FaultConfigPath != ""
		expiration := cfg.EigenDAConfig.MemstoreBlobExpiration

		log.Info("Using memstore backend", "fault_mode", faultMode, "expiration", expiration.String())
		var fc *store.FaultConfig

		if faultMode {
			faultCfg, err := store.LoadFaultConfig(cfg.EigenDAConfig.FaultConfigPath)
			if err != nil {
				panic(fmt.Errorf("failed to load fault config: %w", err))
			}

			fc = faultCfg
		}

		memstore, err = store.NewMemStore(ctx, verifier, log, maxBlobLength, expiration, fc)
		if err != nil {
			return nil, err
		}
	} else {
		log.Info("Using EigenDA backend")
	}

	if vCfg.Verify {
		log.Info("Certificate verification with Ethereum enabled")
	} else {
		log.Warn("Verification disabled")
	}

	client, err := clients.NewEigenDAClient(log, daCfg.ClientConfig)
	if err != nil {
		return nil, err
	}
	
	eigenda, err := store.NewEigenDAStore(
		ctx,
		client,
		verifier,
		log,
		&store.EigenDAStoreConfig{
			MaxBlobSizeBytes:     maxBlobLength,
			EthConfirmationDepth: uint64(cfg.EigenDAConfig.EthConfirmationDepth),
			StatusQueryTimeout:   cfg.EigenDAConfig.ClientConfig.StatusQueryTimeout,
		},
	)

	if err != nil {
		return nil, err
	}

	log.Info("Creating storage router", "eigenda", eigenda != nil, "memstore", memstore != nil, "s3", s3 != nil)
	return store.NewRouter(eigenda, memstore, s3, log)
}
