package flags

import (
	"time"

	"github.com/Layr-Labs/eigenda-proxy/flags/eigenda_flags"
	"github.com/Layr-Labs/eigenda-proxy/store/precomputed_key/redis"
	"github.com/Layr-Labs/eigenda-proxy/store/precomputed_key/s3"
	"github.com/urfave/cli/v2"

	opservice "github.com/ethereum-optimism/optimism/op-service"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
)

const (
	EigenDAClientCategory = "EigenDA Client"
	MemstoreFlagsCategory = "Memstore (replaces EigenDA when enabled)"
	RedisCategory         = "Redis Cache/Fallback"
	S3Category            = "S3 Cache/Fallback"
)

const (
	ListenAddrFlagName = "addr"
	PortFlagName       = "port"

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

	// routing flags
	FallbackTargetsFlagName = "routing.fallback-targets"
	CacheTargetsFlagName    = "routing.cache-targets"
)

const EnvVarPrefix = "EIGENDA_PROXY"

func prefixEnvVars(name string) []string {
	return opservice.PrefixEnvVar(EnvVarPrefix, name)
}

func CLIFlags() []cli.Flag {
	// TODO: Decompose all flags into constituent parts based on their respective category / usage
	flags := []cli.Flag{
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

	return flags
}

// Flags contains the list of configuration options available to the binary.
var Flags = []cli.Flag{}

func init() {
	Flags = CLIFlags()
	Flags = append(Flags, oplog.CLIFlags(EnvVarPrefix)...)
	Flags = append(Flags, opmetrics.CLIFlags(EnvVarPrefix)...)
	Flags = append(Flags, eigenda_flags.CLIFlags(EnvVarPrefix, EigenDAClientCategory)...)
	Flags = append(Flags, redis.CLIFlags(EnvVarPrefix, RedisCategory)...)
	Flags = append(Flags, s3.CLIFlags(EnvVarPrefix, S3Category)...)
}
