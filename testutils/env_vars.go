package testutils

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Layr-Labs/eigenda-proxy/config"
	"github.com/Layr-Labs/eigenda-proxy/config/eigendaflags"
	eigendaflagsv2 "github.com/Layr-Labs/eigenda-proxy/config/v2/eigendaflags"
	"github.com/Layr-Labs/eigenda-proxy/store"
	"github.com/Layr-Labs/eigenda-proxy/store/generated_key/memstore"
	"github.com/Layr-Labs/eigenda-proxy/store/precomputed_key/redis"
	"github.com/Layr-Labs/eigenda-proxy/store/precomputed_key/s3"
	"github.com/Layr-Labs/eigenda-proxy/verify"
	"github.com/urfave/cli/v2"
)

const (
	privateKey    = "SIGNER_PRIVATE_KEY"
	ethRPC        = "ETHEREUM_RPC"
	transport     = "http"
	host          = "127.0.0.1"
	disperserPort = "443"

	disperserPreprodHostname   = "disperser-preprod-holesky.eigenda.xyz"
	preprodCertVerifierAddress = "0xd973fA62E22BC2779F8489258F040C0344B03C21"
	preprodSvcManagerAddress   = "0x54A03db2784E3D0aCC08344D05385d0b62d4F432"

	disperserTestnetHostname   = "disperser-testnet-holesky.eigenda.xyz"
	testnetCertVerifierAddress = "0xFe52fE1940858DCb6e12153E2104aD0fDFbE1162"
	testnetSvcManagerAddress   = "0xD4A7E1Bd8015057293f0D0A557088c286942e84b"
)

// EnvVar represents an individual env var configuration, with a flag name and value
type EnvVar struct {
	Name  string
	Value string
}

// configureContextFromEnvMap accepts a list of env vars, as well as a list of all cli flags in
// the system. It creates a new cli.Context, with the input env flags set to the desired values.
func configureContextFromEnvVars(envVars []EnvVar, flags []cli.Flag) (*cli.Context, error) {
	// Create an app with the provided flags
	app := &cli.App{
		Flags: flags,
	}

	// Create a flag set and populate it with the flags from the app
	set := flag.NewFlagSet(app.Name, flag.ContinueOnError)
	for _, f := range app.Flags {
		if err := f.Apply(set); err != nil {
			return nil, fmt.Errorf("apply flag %v to flag set: %w", f.Names(), err)
		}
	}

	ctx := cli.NewContext(app, set, nil)

	// Set values from the env vars
	for _, envVar := range envVars {
		if err := ctx.Set(envVar.Name, envVar.Value); err != nil {
			return nil, fmt.Errorf("set flag %s to value %s: %w", envVar.Name, envVar.Value, err)
		}
	}

	return ctx, nil
}

// getDefaultTestEnvVars builds a slice of default env var definitions
//
// Env vars are used to configure tests, since that's how it's done in production. We want to exercise as many prod
// code pathways as possible in e2e tests.
func getDefaultTestEnvVars(backend Backend, useV2 bool) []EnvVar {
	signingKey := os.Getenv(privateKey)
	ethRPCURL := os.Getenv(ethRPC)
	maxBlobLengthString := "1mib"
	expiration := 14 * 24 * time.Hour
	writeThreadCount := 0

	outputVars := make([]EnvVar, 0)
	outputVars = append(outputVars, getV1EnvVars(backend, signingKey, ethRPCURL, maxBlobLengthString)...)
	outputVars = append(outputVars, getV2EnvVars(backend, useV2, signingKey, ethRPCURL, maxBlobLengthString)...)
	outputVars = append(outputVars, getKZGEnvVars()...)

	// Memstore flags
	outputVars = append(outputVars, EnvVar{memstore.EnabledFlagName, fmt.Sprintf("%t", backend == MemstoreBackend)})
	outputVars = append(outputVars, EnvVar{memstore.ExpirationFlagName, expiration.String()})

	// Verifier flags
	outputVars = append(
		outputVars,
		EnvVar{verify.CertVerificationDisabledFlagName, fmt.Sprintf("%t", backend == MemstoreBackend)})

	// Server flags
	outputVars = append(outputVars, EnvVar{config.ListenAddrFlagName, host})
	outputVars = append(outputVars, EnvVar{config.PortFlagName, "0"})

	// Store flags
	outputVars = append(outputVars, EnvVar{store.ConcurrentWriteThreads, fmt.Sprintf("%v", writeThreadCount)})

	return outputVars
}

func getV1EnvVars(
	backend Backend,
	signingKey string,
	ethRPCURL string,
	maxBlobLengthString string,
) []EnvVar {
	var pollInterval time.Duration
	if backend == MemstoreBackend {
		pollInterval = time.Second * 1
	} else {
		pollInterval = time.Minute * 1
	}

	envVars := []EnvVar{
		{eigendaflags.SignerPrivateKeyHexFlagName, signingKey},
		{eigendaflags.EthRPCURLFlagName, ethRPCURL},
		{eigendaflags.StatusQueryRetryIntervalFlagName, pollInterval.String()},
		{eigendaflags.DisableTLSFlagName, fmt.Sprintf("%v", false)},
		{eigendaflags.ConfirmationDepthFlagName, "1"},
		{eigendaflags.MaxBlobLengthFlagName, maxBlobLengthString},
		{eigendaflags.StatusQueryTimeoutFlagName, "45m"},
	}

	switch backend {
	case MemstoreBackend:
		// no need to set these fields for local tests
		break
	case PreprodBackend:
		envVars = append(
			envVars,
			EnvVar{eigendaflags.DisperserRPCFlagName, disperserPreprodHostname + ":" + disperserPort})
		envVars = append(envVars, EnvVar{eigendaflags.SvcManagerAddrFlagName, preprodSvcManagerAddress})
	case TestnetBackend:
		envVars = append(
			envVars,
			EnvVar{eigendaflags.DisperserRPCFlagName, disperserTestnetHostname + ":" + disperserPort})
		envVars = append(envVars, EnvVar{eigendaflags.SvcManagerAddrFlagName, testnetSvcManagerAddress})
	default:
		panic("Unsupported backend")
	}

	return envVars
}

func getV2EnvVars(
	backend Backend,
	useV2 bool,
	signingKey string,
	ethRPCURL string,
	maxBlobLengthString string,
) []EnvVar {
	envVars := []EnvVar{
		{eigendaflagsv2.SignerPaymentKeyHexFlagName, signingKey},
		{eigendaflagsv2.EthRPCURLFlagName, ethRPCURL},
		{eigendaflagsv2.V2EnabledFlagName, fmt.Sprintf("%t", useV2)},

		{eigendaflagsv2.DisableTLSFlagName, fmt.Sprintf("%v", false)},
		{eigendaflagsv2.BlobStatusPollIntervalFlagName, "1s"},
		{eigendaflagsv2.PutRetriesFlagName, "1"},
		{eigendaflagsv2.DisperseBlobTimeoutFlagName, "2m"},
		{eigendaflagsv2.BlobCertifiedTimeoutFlagName, "2m"},

		{eigendaflagsv2.RelayTimeoutFlagName, "5s"},
		{eigendaflagsv2.ContractCallTimeoutFlagName, "5s"},
		{eigendaflagsv2.BlobParamsVersionFlagName, "0"},
		{eigendaflagsv2.MaxBlobLengthFlagName, maxBlobLengthString},
	}

	switch backend {
	case MemstoreBackend:
		// no need to set these fields for local tests
		break
	case PreprodBackend:
		envVars = append(
			envVars,
			EnvVar{eigendaflagsv2.DisperserFlagName, disperserPreprodHostname + ":" + disperserPort})
		envVars = append(envVars, EnvVar{eigendaflagsv2.CertVerifierAddrFlagName, preprodCertVerifierAddress})
	case TestnetBackend:
		envVars = append(
			envVars,
			EnvVar{eigendaflagsv2.DisperserFlagName, disperserTestnetHostname + ":" + disperserPort})
		envVars = append(envVars, EnvVar{eigendaflagsv2.CertVerifierAddrFlagName, testnetCertVerifierAddress})
	default:
		panic("Unsupported backend")
	}

	return envVars
}

func getKZGEnvVars() []EnvVar {
	envVars := []EnvVar{
		{verify.G1PathFlagName, "../resources/g1.point"},
		{verify.G2PathFlagName, "../resources/g2.point"},
		{verify.G2PowerOf2PathFlagName, "../resources/g2.point.powerOf2"},
		{verify.ReadG2PointsFlagName, "true"},
		{verify.CachePathFlagName, "../resources/SRSTables"},
	}

	return envVars
}

// GetS3EnvVars gets a list of the necessary EnvVar definitions, to enable an S3 backend
func GetS3EnvVars() []EnvVar {
	envVars := []EnvVar{
		{s3.EnableTLSFlagName, fmt.Sprintf("%v", false)},
		{s3.CredentialTypeFlagName, string(s3.CredentialTypeStatic)},
		{s3.AccessKeyIDFlagName, minioAdmin},
		{s3.AccessKeySecretFlagName, minioAdmin},
		{s3.BucketFlagName, bucketName},
		{s3.EndpointFlagName, minioEndpoint},
		{store.CacheTargetsFlagName, "S3"},
	}

	return envVars
}

// GetRedisEnvVars gets a list of the necessary EnvVar definitions, to enable a redis backend
func GetRedisEnvVars() []EnvVar {
	envVars := []EnvVar{
		{redis.DBFlagName, "0"},
		{redis.EvictionFlagName, "10m"},
		{redis.EndpointFlagName, redisEndpoint},
		{redis.PasswordFlagName, ""},
		{store.CacheTargetsFlagName, "redis"},
	}

	return envVars
}
