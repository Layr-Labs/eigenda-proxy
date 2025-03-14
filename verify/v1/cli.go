package verify

import (
	"github.com/Layr-Labs/eigenda-proxy/common"
	"github.com/urfave/cli/v2"
)

var (
	// cert verification flags
	CertVerificationDisabledFlagName = withFlagPrefix("cert-verification-disabled")

	// kzg flags
	G1PathFlagName         = withFlagPrefix("g1-path")
	G2PowerOf2PathFlagName = withFlagPrefix("g2-power-of-2-path")
	CachePathFlagName      = withFlagPrefix("cache-path")
)

// we keep the eigenda prefix like eigenda client flags, because we
// plan to upstream this verification logic into the eigenda client
func withFlagPrefix(s string) string {
	return "eigenda." + s
}

func withEnvPrefix(envPrefix, s string) string {
	return envPrefix + "_EIGENDA_" + s
}

// CLIFlags ... used for Verifier configuration
// category is used to group the flags in the help output (see https://cli.urfave.org/v2/examples/flags/#grouping)
func CLIFlags(envPrefix, category string) []cli.Flag {
	return []cli.Flag{
		&cli.BoolFlag{
			Name:     CertVerificationDisabledFlagName,
			Usage:    "Whether to verify certificates received from EigenDA disperser.",
			EnvVars:  []string{withEnvPrefix(envPrefix, "CERT_VERIFICATION_DISABLED")},
			Value:    false,
			Category: category,
		},
		// kzg flags
		// we use a relative path for these so that the path works for both the binary and the docker container
		// aka we assume the binary is run from root dir, and that the resources/ dir is copied into the working dir of
		// the container
		&cli.StringFlag{
			Name:     G1PathFlagName,
			Usage:    "path to g1.point file.",
			EnvVars:  []string{withEnvPrefix(envPrefix, "TARGET_KZG_G1_PATH")},
			Value:    "resources/g1.point",
			Category: category,
		},
		&cli.StringFlag{
			Name:     G2PowerOf2PathFlagName,
			Usage:    "path to g2.point.powerOf2 file. This resource is not currently used, but needed because of the shared eigenda KZG library that we use. We will eventually fix this.",
			EnvVars:  []string{withEnvPrefix(envPrefix, "TARGET_KZG_G2_POWER_OF_2_PATH")},
			Value:    "resources/g2.point.powerOf2",
			Category: category,
		},
		&cli.StringFlag{
			Name:     CachePathFlagName,
			Usage:    "path to SRS tables for caching. This resource is not currently used, but needed because of the shared eigenda KZG library that we use. We will eventually fix this.",
			EnvVars:  []string{withEnvPrefix(envPrefix, "TARGET_CACHE_PATH")},
			Value:    "resources/SRSTables/",
			Category: category,
		},
	}
}

// ReadConfig takes an eigendaClientConfig as input because the verifier config reuses some configs that are already
// defined in the client config
func ReadConfig(ctx *cli.Context, clientConfigV1 common.ClientConfigV1) Config {
	return Config{
		VerifyCerts: !ctx.Bool(CertVerificationDisabledFlagName),
		// reuse some configs from the eigenda client
		RPCURL:               clientConfigV1.EdaClientCfg.EthRpcUrl,
		SvcManagerAddr:       clientConfigV1.EdaClientCfg.SvcManagerAddr,
		EthConfirmationDepth: clientConfigV1.EdaClientCfg.WaitForConfirmationDepth,
		WaitForFinalization:  clientConfigV1.EdaClientCfg.WaitForFinalization,
	}
}
