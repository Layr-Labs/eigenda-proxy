package cli

import (
	"fmt"
	"runtime"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/Layr-Labs/eigenda-proxy/store"
	"github.com/Layr-Labs/eigenda-proxy/utils"
	"github.com/Layr-Labs/eigenda-proxy/verify"
	"github.com/Layr-Labs/eigenda/api/clients"
	"github.com/Layr-Labs/eigenda/api/clients/codecs"
	"github.com/Layr-Labs/eigenda/encoding/kzg"
)

const (
	BytesPerSymbol = 31
	MaxCodingRatio = 8
)

var (
	MaxSRSPoints       = 1 << 28 // 2^28
	MaxAllowedBlobSize = uint64(MaxSRSPoints * BytesPerSymbol / MaxCodingRatio)
)

type Config struct {
	// eigenda
	ClientConfig clients.EigenDAClientConfig

	// the blob encoding version to use when writing blobs from the high level interface.
	PutBlobEncodingVersion codecs.BlobEncodingVersion

	// eth verification vars
	// TODO: right now verification and confirmation depth are tightly coupled
	//       we should decouple them
	CertVerificationEnabled bool
	EthRPC                  string
	SvcManagerAddr          string
	EthConfirmationDepth    int64

	// kzg vars
	CacheDir         string
	G1Path           string
	G2Path           string
	G2PowerOfTauPath string

	// size constraints
	MaxBlobLength      string
	maxBlobLengthBytes uint64

	// memstore
	MemstoreEnabled        bool
	MemstoreBlobExpiration time.Duration
	MemstoreGetLatency     time.Duration
	MemstorePutLatency     time.Duration

	// routing
	FallbackTargets []string
	CacheTargets    []string

	// secondary storage
	RedisConfig store.RedisConfig
	S3Config    store.S3Config
}

// GetMaxBlobLength ... returns the maximum blob length in bytes
func (cfg *Config) GetMaxBlobLength() (uint64, error) {
	if cfg.maxBlobLengthBytes == 0 {
		numBytes, err := utils.ParseBytesAmount(cfg.MaxBlobLength)
		if err != nil {
			return 0, err
		}

		if numBytes > MaxAllowedBlobSize {
			return 0, fmt.Errorf("excluding disperser constraints on max blob size, SRS points constrain the maxBlobLength configuration parameter to be less than than %d bytes", MaxAllowedBlobSize)
		}

		cfg.maxBlobLengthBytes = numBytes
	}

	return cfg.maxBlobLengthBytes, nil
}

// VerificationCfg ... returns certificate config used to verify blobs from eigenda
func (cfg *Config) VerificationCfg() *verify.Config {
	numBytes, err := cfg.GetMaxBlobLength()
	if err != nil {
		panic(fmt.Errorf("failed to read max blob length: %w", err))
	}

	kzgCfg := &kzg.KzgConfig{
		G1Path:          cfg.G1Path,
		G2PowerOf2Path:  cfg.G2PowerOfTauPath,
		CacheDir:        cfg.CacheDir,
		SRSOrder:        268435456,                     // 2 ^ 32
		SRSNumberToLoad: numBytes / 32,                 // # of fr.Elements
		NumWorker:       uint64(runtime.GOMAXPROCS(0)), // #nosec G115
	}

	return &verify.Config{
		KzgConfig:            kzgCfg,
		VerifyCerts:          cfg.CertVerificationEnabled,
		RPCURL:               cfg.EthRPC,
		SvcManagerAddr:       cfg.SvcManagerAddr,
		EthConfirmationDepth: uint64(cfg.EthConfirmationDepth), // #nosec G115
	}
}

// ReadConfig ... parses the Config from the provided flags or environment variables.
func ReadConfig(ctx *cli.Context) Config {
	cfg := Config{
		RedisConfig: store.RedisConfig{
			Endpoint: ctx.String(RedisEndpointFlagName),
			Password: ctx.String(RedisPasswordFlagName),
			DB:       ctx.Int(RedisDBFlagName),
			Eviction: ctx.Duration(RedisEvictionFlagName),
		},
		S3Config: store.S3Config{
			S3CredentialType: store.StringToS3CredentialType(ctx.String(S3CredentialTypeFlagName)),
			Bucket:           ctx.String(S3BucketFlagName),
			Path:             ctx.String(S3PathFlagName),
			Endpoint:         ctx.String(S3EndpointFlagName),
			AccessKeyID:      ctx.String(S3AccessKeyIDFlagName),
			AccessKeySecret:  ctx.String(S3AccessKeySecretFlagName),
			Backup:           ctx.Bool(S3BackupFlagName),
			Timeout:          ctx.Duration(S3TimeoutFlagName),
		},
		ClientConfig: clients.EigenDAClientConfig{
			RPC:                          ctx.String(EigenDADisperserRPCFlagName),
			StatusQueryRetryInterval:     ctx.Duration(StatusQueryRetryIntervalFlagName),
			StatusQueryTimeout:           ctx.Duration(StatusQueryTimeoutFlagName),
			DisableTLS:                   ctx.Bool(DisableTLSFlagName),
			ResponseTimeout:              ctx.Duration(ResponseTimeoutFlagName),
			CustomQuorumIDs:              ctx.UintSlice(CustomQuorumIDsFlagName),
			SignerPrivateKeyHex:          ctx.String(SignerPrivateKeyHexFlagName),
			PutBlobEncodingVersion:       codecs.BlobEncodingVersion(ctx.Uint(PutBlobEncodingVersionFlagName)),
			DisablePointVerificationMode: ctx.Bool(DisablePointVerificationModeFlagName),
		},
		G1Path:                  ctx.String(G1PathFlagName),
		G2PowerOfTauPath:        ctx.String(G2TauFlagName),
		CacheDir:                ctx.String(CachePathFlagName),
		MaxBlobLength:           ctx.String(MaxBlobLengthFlagName),
		CertVerificationEnabled: ctx.Bool(CertVerificationEnabledFlagName),
		SvcManagerAddr:          ctx.String(SvcManagerAddrFlagName),
		EthRPC:                  ctx.String(EthRPCFlagName),
		EthConfirmationDepth:    ctx.Int64(EthConfirmationDepthFlagName),
		MemstoreEnabled:         ctx.Bool(MemstoreFlagName),
		MemstoreBlobExpiration:  ctx.Duration(MemstoreExpirationFlagName),
		MemstoreGetLatency:      ctx.Duration(MemstoreGetLatencyFlagName),
		MemstorePutLatency:      ctx.Duration(MemstorePutLatencyFlagName),
		FallbackTargets:         ctx.StringSlice(FallbackTargetsFlagName),
		CacheTargets:            ctx.StringSlice(CacheTargetsFlagName),
	}
	// the eigenda client can only wait for 0 confirmations or finality
	// the da-proxy has a more fine-grained notion of confirmation depth
	// we use -1 to let the da client wait for finality, and then need to set the confirmation depth
	// for the da-proxy to 0 (because negative confirmation depth doesn't mean anything and leads to errors)
	// TODO: should the eigenda-client implement this feature for us instead?
	if cfg.EthConfirmationDepth < 0 {
		cfg.ClientConfig.WaitForFinalization = true
		cfg.EthConfirmationDepth = 0
	}

	return cfg
}

// checkTargets ... verifies that a backend target slice is constructed correctly
func (cfg *Config) checkTargets(targets []string) error {
	if len(targets) == 0 {
		return nil
	}

	if utils.ContainsDuplicates(targets) {
		return fmt.Errorf("duplicate targets provided: %+v", targets)
	}

	for _, t := range targets {
		if store.StringToBackendType(t) == store.Unknown {
			return fmt.Errorf("unknown fallback target provided: %s", t)
		}
	}

	return nil
}

// Check ... verifies that configuration values are adequately set
func (cfg *Config) Check() error {
	l, err := cfg.GetMaxBlobLength()
	if err != nil {
		return err
	}

	if l == 0 {
		return fmt.Errorf("max blob length is 0")
	}

	if !cfg.MemstoreEnabled {
		if cfg.ClientConfig.RPC == "" {
			return fmt.Errorf("using eigenda backend (memstore.enabled=false) but eigenda disperser rpc url is not set")
		}
	}

	if cfg.CertVerificationEnabled {
		if cfg.MemstoreEnabled {
			return fmt.Errorf("cannot enable cert verification when memstore is enabled")
		}
		if cfg.EthRPC == "" {
			return fmt.Errorf("cert verification enabled but eth rpc is not set")
		}
		if cfg.SvcManagerAddr == "" {
			return fmt.Errorf("cert verification enabled but svc manager address is not set")
		}
	}

	if cfg.S3Config.S3CredentialType == store.S3CredentialUnknown && cfg.S3Config.Endpoint != "" {
		return fmt.Errorf("s3 credential type must be set")
	}
	if cfg.S3Config.S3CredentialType == store.S3CredentialStatic {
		if cfg.S3Config.Endpoint != "" && (cfg.S3Config.AccessKeyID == "" || cfg.S3Config.AccessKeySecret == "") {
			return fmt.Errorf("s3 endpoint is set, but access key id or access key secret is not set")
		}
	}

	if cfg.RedisConfig.Endpoint == "" && cfg.RedisConfig.Password != "" {
		return fmt.Errorf("redis password is set, but endpoint is not")
	}

	err = cfg.checkTargets(cfg.FallbackTargets)
	if err != nil {
		return err
	}

	err = cfg.checkTargets(cfg.CacheTargets)
	if err != nil {
		return err
	}

	// verify that same target is not in both fallback and cache targets
	for _, t := range cfg.FallbackTargets {
		if utils.Contains(cfg.CacheTargets, t) {
			return fmt.Errorf("target %s is in both fallback and cache targets", t)
		}
	}

	return nil
}
