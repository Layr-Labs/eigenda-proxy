package config

import (
	"github.com/Layr-Labs/eigenda-proxy/config/eigendaflags"
	eigenda_v2_flags "github.com/Layr-Labs/eigenda-proxy/config/v2/eigendaflags"
	"github.com/Layr-Labs/eigenda-proxy/store"
	"github.com/Layr-Labs/eigenda-proxy/verify"

	"github.com/Layr-Labs/eigenda-proxy/logging"
	"github.com/Layr-Labs/eigenda-proxy/metrics"
	"github.com/Layr-Labs/eigenda-proxy/store/generated_key/memstore"
	"github.com/Layr-Labs/eigenda-proxy/store/precomputed_key/redis"
	"github.com/Layr-Labs/eigenda-proxy/store/precomputed_key/s3"
	"github.com/urfave/cli/v2"

	"github.com/Layr-Labs/eigenda-proxy/common"
)

const (
	EigenDAClientCategory   = "EigenDA Client"
	EigenDAV2ClientCategory = "EigenDA V2 Client"
	LoggingFlagsCategory    = "Logging"
	MetricsFlagCategory     = "Metrics"
	MemstoreFlagsCategory   = "Memstore (for testing purposes - replaces EigenDA backend)"
	StorageFlagsCategory    = "Storage"
	RedisCategory           = "Redis Cache/Fallback"
	S3Category              = "S3 Cache/Fallback"
	VerifierCategory        = "KZG and Cert Verifier"
)

const (
	ListenAddrFlagName = "addr"
	PortFlagName       = "port"
)

func CLIFlags() []cli.Flag {
	// TODO: Decompose all flags into constituent parts based on their respective category / usage
	flags := []cli.Flag{
		&cli.StringFlag{
			Name:    ListenAddrFlagName,
			Usage:   "Server listening address",
			Value:   "0.0.0.0",
			EnvVars: common.PrefixEnvVar(common.GlobalPrefix, "ADDR"),
		},
		&cli.IntFlag{
			Name:    PortFlagName,
			Usage:   "Server listening port",
			Value:   3100,
			EnvVars: common.PrefixEnvVar(common.GlobalPrefix, "PORT"),
		},
	}

	return flags
}

// Flags contains the list of configuration options available to the binary.
var Flags = []cli.Flag{}

func init() {
	Flags = CreateCLIFlags()
}

func CreateCLIFlags() []cli.Flag {
	flags := CLIFlags()
	flags = append(flags, logging.CLIFlags(common.GlobalPrefix, LoggingFlagsCategory)...)
	flags = append(flags, metrics.CLIFlags(common.GlobalPrefix, MetricsFlagCategory)...)
	flags = append(flags, eigendaflags.CLIFlags(common.GlobalPrefix, EigenDAClientCategory)...)
	flags = append(flags, eigenda_v2_flags.CLIFlags(common.GlobalPrefix, EigenDAV2ClientCategory)...)
	flags = append(flags, store.CLIFlags(common.GlobalPrefix, StorageFlagsCategory)...)
	flags = append(flags, redis.CLIFlags(common.GlobalPrefix, RedisCategory)...)
	flags = append(flags, s3.CLIFlags(common.GlobalPrefix, S3Category)...)
	flags = append(flags, memstore.CLIFlags(common.GlobalPrefix, MemstoreFlagsCategory)...)
	flags = append(flags, verify.CLIFlags(common.GlobalPrefix, VerifierCategory)...)

	flags = append(flags, verify.DeprecatedCLIFlags(common.GlobalPrefix, VerifierCategory)...)
	flags = append(flags, store.DeprecatedCLIFlags(common.GlobalPrefix, StorageFlagsCategory)...)

	return flags
}
