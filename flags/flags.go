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
