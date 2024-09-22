package flags

import (
	"time"

	"github.com/Layr-Labs/eigenda-proxy/store/precomputed_key/redis"
	"github.com/urfave/cli/v2"

	opservice "github.com/ethereum-optimism/optimism/op-service"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
)

const (
	MemstoreFlagsCategory = "Memstore"
	RedisCategory         = "Redis"
)

const (
	ListenAddrFlagName = "addr"
	PortFlagName       = "port"

	// eigenda client flags
	EigenDADisperserRPCFlagName          = "eigenda-disperser-rpc"
	StatusQueryRetryIntervalFlagName     = "eigenda-status-query-retry-interval"
	StatusQueryTimeoutFlagName           = "eigenda-status-query-timeout"
	DisableTLSFlagName                   = "eigenda-disable-tls"
	ResponseTimeoutFlagName              = "eigenda-response-timeout"
	CustomQuorumIDsFlagName              = "eigenda-custom-quorum-ids"
	SignerPrivateKeyHexFlagName          = "eigenda-signer-private-key-hex"
	PutBlobEncodingVersionFlagName       = "eigenda-put-blob-encoding-version"
	DisablePointVerificationModeFlagName = "eigenda-disable-point-verification-mode"

	// cert verification flags
	// TODO: should we remove the eigenda prefix since these are not eigenda-client flags?
	CertVerificationEnabledFlagName = "eigenda-cert-verification-enabled"
	EthRPCFlagName                  = "eigenda-eth-rpc"
	SvcManagerAddrFlagName          = "eigenda-svc-manager-addr"
	EthConfirmationDepthFlagName    = "eigenda-eth-confirmation-depth"

	// kzg flags
	G1PathFlagName        = "eigenda-g1-path"
	G2TauFlagName         = "eigenda-g2-tau-path"
	CachePathFlagName     = "eigenda-cache-path"
	MaxBlobLengthFlagName = "eigenda-max-blob-length"

	// memstore flags
	MemstoreFlagName           = "memstore.enabled"
	MemstoreExpirationFlagName = "memstore.expiration"
	MemstorePutLatencyFlagName = "memstore.put-latency"
	MemstoreGetLatencyFlagName = "memstore.get-latency"

	// S3 client flags
	S3CredentialTypeFlagName  = "s3.credential-type" // #nosec G101
	S3BucketFlagName          = "s3.bucket"          // #nosec G101
	S3PathFlagName            = "s3.path"
	S3EndpointFlagName        = "s3.endpoint"
	S3AccessKeyIDFlagName     = "s3.access-key-id"     // #nosec G101
	S3AccessKeySecretFlagName = "s3.access-key-secret" // #nosec G101
	S3BackupFlagName          = "s3.backup"
	S3TimeoutFlagName         = "s3.timeout"

	// routing flags
	FallbackTargetsFlagName = "routing.fallback-targets"
	CacheTargetsFlagName    = "routing.cache-targets"
)

const EnvVarPrefix = "EIGENDA_PROXY"

func prefixEnvVars(name string) []string {
	return opservice.PrefixEnvVar(EnvVarPrefix, name)
}

// Flags contains the list of configuration options available to the binary.
var Flags = []cli.Flag{
	&cli.StringFlag{
		Name:    ListenAddrFlagName,
		Usage:   "server listening address",
		Value:   "0.0.0.0",
		EnvVars: prefixEnvVars("ADDR"),
	},
	&cli.IntFlag{
		Name:    PortFlagName,
		Usage:   "server listening port",
		Value:   3100,
		EnvVars: prefixEnvVars("PORT"),
	},
}

// s3Flags ... used for S3 backend configuration
func s3Flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    S3CredentialTypeFlagName,
			Usage:   "The way to authenticate to S3, options are [iam, static]",
			EnvVars: prefixEnvVars("S3_CREDENTIAL_TYPE"),
		},
		&cli.StringFlag{
			Name:    S3BucketFlagName,
			Usage:   "bucket name for S3 storage",
			EnvVars: prefixEnvVars("S3_BUCKET"),
		},
		&cli.StringFlag{
			Name:    S3PathFlagName,
			Usage:   "path for S3 storage",
			EnvVars: prefixEnvVars("S3_PATH"),
		},
		&cli.StringFlag{
			Name:    S3EndpointFlagName,
			Usage:   "endpoint for S3 storage",
			Value:   "",
			EnvVars: prefixEnvVars("S3_ENDPOINT"),
		},
		&cli.StringFlag{
			Name:    S3AccessKeyIDFlagName,
			Usage:   "access key id for S3 storage",
			Value:   "",
			EnvVars: prefixEnvVars("S3_ACCESS_KEY_ID"),
		},
		&cli.StringFlag{
			Name:    S3AccessKeySecretFlagName,
			Usage:   "access key secret for S3 storage",
			Value:   "",
			EnvVars: prefixEnvVars("S3_ACCESS_KEY_SECRET"),
		},
		&cli.BoolFlag{
			Name:    S3BackupFlagName,
			Usage:   "whether to use S3 as a backup store to ensure resiliency in case of EigenDA read failure",
			Value:   false,
			EnvVars: prefixEnvVars("S3_BACKUP"),
		},
		&cli.DurationFlag{
			Name:    S3TimeoutFlagName,
			Usage:   "timeout for S3 storage operations (e.g. get, put)",
			Value:   5 * time.Second,
			EnvVars: prefixEnvVars("S3_TIMEOUT"),
		},
	}
}

func CLIFlags() []cli.Flag {
	// TODO: Decompose all flags into constituent parts based on their respective category / usage
	flags := []cli.Flag{
		&cli.StringFlag{
			Name:    EigenDADisperserRPCFlagName,
			Usage:   "RPC endpoint of the EigenDA disperser.",
			EnvVars: prefixEnvVars("EIGENDA_DISPERSER_RPC"),
		},
		&cli.DurationFlag{
			Name:    StatusQueryTimeoutFlagName,
			Usage:   "Duration to wait for a blob to finalize after being sent for dispersal. Default is 30 minutes.",
			Value:   30 * time.Minute,
			EnvVars: prefixEnvVars("STATUS_QUERY_TIMEOUT"),
		},
		&cli.DurationFlag{
			Name:    StatusQueryRetryIntervalFlagName,
			Usage:   "Interval between retries when awaiting network blob finalization. Default is 5 seconds.",
			Value:   5 * time.Second,
			EnvVars: prefixEnvVars("STATUS_QUERY_INTERVAL"),
		},
		&cli.BoolFlag{
			Name:    DisableTLSFlagName,
			Usage:   "Disable TLS for gRPC communication with the EigenDA disperser. Default is false.",
			Value:   false,
			EnvVars: prefixEnvVars("GRPC_DISABLE_TLS"),
		},
		&cli.DurationFlag{
			Name:    ResponseTimeoutFlagName,
			Usage:   "Total time to wait for a response from the EigenDA disperser. Default is 60 seconds.",
			Value:   60 * time.Second,
			EnvVars: prefixEnvVars("RESPONSE_TIMEOUT"),
		},
		&cli.UintSliceFlag{
			Name:    CustomQuorumIDsFlagName,
			Usage:   "Custom quorum IDs for writing blobs. Should not include default quorums 0 or 1.",
			Value:   cli.NewUintSlice(),
			EnvVars: prefixEnvVars("CUSTOM_QUORUM_IDS"),
		},
		&cli.StringFlag{
			Name:    SignerPrivateKeyHexFlagName,
			Usage:   "Hex-encoded signer private key. This key should not be associated with an Ethereum address holding any funds.",
			EnvVars: prefixEnvVars("SIGNER_PRIVATE_KEY_HEX"),
		},
		&cli.UintFlag{
			Name:    PutBlobEncodingVersionFlagName,
			Usage:   "Blob encoding version to use when writing blobs from the high-level interface.",
			EnvVars: prefixEnvVars("PUT_BLOB_ENCODING_VERSION"),
			Value:   0,
		},
		&cli.BoolFlag{
			Name:    DisablePointVerificationModeFlagName,
			Usage:   "Disable point verification mode. This mode performs IFFT on data before writing and FFT on data after reading. Disabling requires supplying the entire blob for verification against the KZG commitment.",
			EnvVars: prefixEnvVars("DISABLE_POINT_VERIFICATION_MODE"),
			Value:   false,
		},
		&cli.StringFlag{
			Name:    MaxBlobLengthFlagName,
			Usage:   "Maximum blob length to be written or read from EigenDA. Determines the number of SRS points loaded into memory for KZG commitments. Example units: '30MiB', '4Kb', '30MB'. Maximum size slightly exceeds 1GB.",
			EnvVars: prefixEnvVars("MAX_BLOB_LENGTH"),
			Value:   "16MiB",
		},
		&cli.StringFlag{
			Name:    G1PathFlagName,
			Usage:   "Directory path to g1.point file.",
			EnvVars: prefixEnvVars("TARGET_KZG_G1_PATH"),
			Value:   "resources/g1.point",
		},
		&cli.StringFlag{
			Name:    G2TauFlagName,
			Usage:   "Directory path to g2.point.powerOf2 file.",
			EnvVars: prefixEnvVars("TARGET_G2_TAU_PATH"),
			Value:   "resources/g2.point.powerOf2",
		},
		&cli.StringFlag{
			Name:    CachePathFlagName,
			Usage:   "Directory path to SRS tables for caching.",
			EnvVars: prefixEnvVars("TARGET_CACHE_PATH"),
			Value:   "resources/SRSTables/",
		},
		&cli.BoolFlag{
			Name:    CertVerificationEnabledFlagName,
			Usage:   "Whether to verify certificates received from EigenDA disperser.",
			EnvVars: prefixEnvVars("CERT_VERIFICATION_ENABLED"),
			// TODO: ideally we'd want this to be turned on by default when eigenda backend is used (memstore.enabled=false)
			Value: false,
		},
		&cli.StringFlag{
			Name:    EthRPCFlagName,
			Usage:   "JSON RPC node endpoint for the Ethereum network used for finalizing DA blobs. See available list here: https://docs.eigenlayer.xyz/eigenda/networks/",
			EnvVars: prefixEnvVars("ETH_RPC"),
		},
		&cli.StringFlag{
			Name:    SvcManagerAddrFlagName,
			Usage:   "The deployed EigenDA service manager address. The list can be found here: https://github.com/Layr-Labs/eigenlayer-middleware/?tab=readme-ov-file#current-mainnet-deployment",
			EnvVars: prefixEnvVars("SERVICE_MANAGER_ADDR"),
		},
		&cli.Int64Flag{
			Name:    EthConfirmationDepthFlagName,
			Usage:   "The number of Ethereum blocks to wait before considering a submitted blob's DA batch submission confirmed. `0` means wait for inclusion only. `-1` means wait for finality.",
			EnvVars: prefixEnvVars("ETH_CONFIRMATION_DEPTH"),
			Value:   -1,
		},
		&cli.BoolFlag{
			Name:     MemstoreFlagName,
			Usage:    "Whether to use mem-store for DA logic.",
			EnvVars:  prefixEnvVars("MEMSTORE_ENABLED"),
			Category: MemstoreFlagsCategory,
		},
		&cli.DurationFlag{
			Name:     MemstoreExpirationFlagName,
			Usage:    "Duration that a mem-store blob/commitment pair are allowed to live.",
			Value:    25 * time.Minute,
			EnvVars:  prefixEnvVars("MEMSTORE_EXPIRATION"),
			Category: MemstoreFlagsCategory,
		},
		&cli.DurationFlag{
			Name:     MemstorePutLatencyFlagName,
			Usage:    "Artificial latency added for memstore backend to mimic EigenDA's dispersal latency.",
			Value:    0,
			EnvVars:  prefixEnvVars("MEMSTORE_PUT_LATENCY"),
			Category: MemstoreFlagsCategory,
		},
		&cli.DurationFlag{
			Name:     MemstoreGetLatencyFlagName,
			Usage:    "Artificial latency added for memstore backend to mimic EigenDA's retrieval latency.",
			Value:    0,
			EnvVars:  prefixEnvVars("MEMSTORE_GET_LATENCY"),
			Category: MemstoreFlagsCategory,
		},
		&cli.StringSliceFlag{
			Name:    FallbackTargetsFlagName,
			Usage:   "List of read fallback targets to rollover to if cert can't be read from EigenDA.",
			Value:   cli.NewStringSlice(),
			EnvVars: prefixEnvVars("FALLBACK_TARGETS"),
		},
		&cli.StringSliceFlag{
			Name:    CacheTargetsFlagName,
			Usage:   "List of caching targets to use fast reads from EigenDA.",
			Value:   cli.NewStringSlice(),
			EnvVars: prefixEnvVars("CACHE_TARGETS"),
		},
	}

	flags = append(flags, s3Flags()...)
	return flags
}

func init() {
	Flags = CLIFlags()
	Flags = append(Flags, oplog.CLIFlags(EnvVarPrefix)...)
	Flags = append(Flags, opmetrics.CLIFlags(EnvVarPrefix)...)
	Flags = append(Flags, redis.CLIFlags(EnvVarPrefix, RedisCategory)...)
}
