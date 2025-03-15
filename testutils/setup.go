package testutils

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Layr-Labs/eigenda-proxy/config"
	"github.com/Layr-Labs/eigenda-proxy/config/eigendaflags"
	eigendaflagsv2 "github.com/Layr-Labs/eigenda-proxy/config/v2/eigendaflags"
	proxy_metrics "github.com/Layr-Labs/eigenda-proxy/metrics"
	"github.com/Layr-Labs/eigenda-proxy/server"
	"github.com/Layr-Labs/eigenda-proxy/store"
	"github.com/Layr-Labs/eigenda-proxy/store/generated_key/memstore"
	"github.com/Layr-Labs/eigenda-proxy/store/precomputed_key/redis"
	"github.com/Layr-Labs/eigenda-proxy/store/precomputed_key/s3"
	"github.com/Layr-Labs/eigenda-proxy/verify"
	"github.com/Layr-Labs/eigensdk-go/logging"
	"github.com/ethereum/go-ethereum/log"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/urfave/cli/v2"
	"golang.org/x/exp/rand"

	miniotc "github.com/testcontainers/testcontainers-go/modules/minio"
	redistc "github.com/testcontainers/testcontainers-go/modules/redis"
)

const (
	privateKey               = "SIGNER_PRIVATE_KEY"
	ethRPC                   = "ETHEREUM_RPC"
	transport                = "http"
	host                     = "127.0.0.1"
	holeskyDisperserHostname = "disperser-holesky.eigenda.xyz"
	holeskyDisperserPort     = "443"
	minioAdmin               = "minioadmin"
)

var (
	// set by startMinioContainer
	bucketName = ""
	// set by startMinioContainer
	minioEndpoint = ""

	// set by startRedisContainer
	redisEndpoint = ""
)

// TODO: we shouldn't start the containers in the init function like this.
// Need to find a better way to start the containers and set the endpoints.
// Even better would be for the endpoints not to be global variables injected into the test configs.
// Starting the containers on init like this also makes it harder to import this file into other tests.
func init() {
	err := startMinIOContainer()
	if err != nil {
		panic(err)
	}
	err = startRedisContainer()
	if err != nil {
		panic(err)
	}
}

// startMinIOContainer starts a MinIO container and sets the minioEndpoint global variable
func startMinIOContainer() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	minioContainer, err := miniotc.Run(
		ctx,
		"minio/minio:RELEASE.2024-10-02T17-50-41Z",
		miniotc.WithUsername(minioAdmin),
		miniotc.WithPassword(minioAdmin),
	)
	if err != nil {
		return fmt.Errorf("failed to start MinIO container: %w", err)
	}

	endpoint, err := minioContainer.Endpoint(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to get MinIO endpoint: %w", err)
	}

	minioEndpoint = strings.TrimPrefix(endpoint, "http://")

	// generate random string
	bucketName = "eigenda-proxy-test-" + RandStr(10)
	createS3Bucket(bucketName)

	return nil
}

// startRedisContainer starts a Redis container and sets the redisEndpoint global variable
func startRedisContainer() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	redisContainer, err := redistc.Run(
		ctx,
		"docker.io/redis:7",
	)
	if err != nil {
		return fmt.Errorf("failed to start Redis container: %w", err)
	}

	endpoint, err := redisContainer.Endpoint(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to get Redis endpoint: %w", err)
	}
	redisEndpoint = endpoint
	return nil
}

func UseMemstore() bool {
	envVar := "MEMSTORE"
	return os.Getenv(envVar) == fmt.Sprintf("%v", true) || os.Getenv(envVar) == "1"
}

type TestConfig struct {
	UseV2            bool
	UseMemory        bool
	Expiration       time.Duration
	WriteThreadCount int
	// at most one of the below options should be true
	UseKeccak256ModeS3 bool
	UseS3Caching       bool
	UseRedisCaching    bool
	UseS3Fallback      bool
}

func NewTestConfig(useMemory bool, useV2 bool) TestConfig {
	return TestConfig{
		UseV2:              useV2,
		UseMemory:          useMemory,
		Expiration:         14 * 24 * time.Hour,
		UseKeccak256ModeS3: false,
		UseS3Caching:       false,
		UseRedisCaching:    false,
		UseS3Fallback:      false,
		WriteThreadCount:   0,
	}
}

// getTestEnvVars build a map from env var flag name to the default value used in tests
//
// Env vars are used to configure tests, since that's how it's done in production. We want to exercise as many prod
// code pathways as possible in e2e tests.
func getTestEnvVars(testCfg TestConfig) map[string]string {
	signingKey := os.Getenv(privateKey)
	ethRPCURL := os.Getenv(ethRPC)
	maxBlobLengthString := "16mib"

	outputMap := make(map[string]string)

	setV1EnvVars(outputMap, testCfg.UseMemory, signingKey, ethRPCURL, maxBlobLengthString)
	setV2EnvVars(outputMap, testCfg.UseV2, signingKey, ethRPCURL, maxBlobLengthString)
	setKZGEnvVars(outputMap)

	// Memstore flags
	outputMap[memstore.EnabledFlagName] = fmt.Sprintf("%t", testCfg.UseMemory)
	outputMap[memstore.ExpirationFlagName] = testCfg.Expiration.String()

	// Verifier flags
	outputMap[verify.CertVerificationDisabledFlagName] = fmt.Sprintf("%t", testCfg.UseMemory)

	// Server flags
	outputMap[config.ListenAddrFlagName] = host
	outputMap[config.PortFlagName] = "0"

	// Store flags
	outputMap[store.ConcurrentWriteThreads] = fmt.Sprintf("%v", testCfg.WriteThreadCount)

	switch {
	case testCfg.UseKeccak256ModeS3, testCfg.UseS3Caching, testCfg.UseS3Fallback:
		setS3EnvVars(outputMap)
	case testCfg.UseRedisCaching:
		setRedisEnvVars(outputMap)
	}

	return outputMap
}

func setV1EnvVars(
	envVars map[string]string,
	useMemstore bool,
	signingKey string,
	ethRPCURL string,
	maxBlobLengthString string,
) {
	var pollInterval time.Duration
	if useMemstore {
		pollInterval = time.Second * 1
	} else {
		pollInterval = time.Minute * 1
	}

	envVars[eigendaflags.SignerPrivateKeyHexFlagName] = signingKey
	envVars[eigendaflags.EthRPCURLFlagName] = ethRPCURL
	envVars[eigendaflags.DisperserRPCFlagName] = holeskyDisperserHostname + ":" + holeskyDisperserPort
	envVars[eigendaflags.StatusQueryRetryIntervalFlagName] = pollInterval.String()
	envVars[eigendaflags.DisableTLSFlagName] = fmt.Sprintf("%v", false)
	envVars[eigendaflags.ConfirmationDepthFlagName] = "1"
	envVars[eigendaflags.SvcManagerAddrFlagName] = "0xD4A7E1Bd8015057293f0D0A557088c286942e84b" // holesky testnet
	envVars[eigendaflags.MaxBlobLengthFlagName] = maxBlobLengthString
	envVars[eigendaflags.StatusQueryTimeoutFlagName] = "45m"
}

func setV2EnvVars(
	envVars map[string]string,
	useV2 bool,
	signingKey string,
	ethRPCURL string,
	maxBlobLengthString string,
) {
	envVars[eigendaflagsv2.SignerPaymentKeyHexFlagName] = signingKey
	envVars[eigendaflagsv2.EthRPCURLFlagName] = ethRPCURL
	envVars[eigendaflagsv2.V2EnabledFlagName] = fmt.Sprintf("%t", useV2)
	envVars[eigendaflagsv2.DisperserFlagName] = holeskyDisperserHostname + ":" + holeskyDisperserPort
	envVars[eigendaflagsv2.DisableTLSFlagName] = fmt.Sprintf("%v", false)
	envVars[eigendaflagsv2.BlobStatusPollIntervalFlagName] = "1s"
	envVars[eigendaflagsv2.PutRetriesFlagName] = "1"
	envVars[eigendaflagsv2.DisperseBlobTimeoutFlagName] = "2m"
	envVars[eigendaflagsv2.BlobCertifiedTimeoutFlagName] = "2m"
	envVars[eigendaflagsv2.CertVerifierAddrFlagName] = "0xFe52fE1940858DCb6e12153E2104aD0fDFbE1162" // holesky testnet
	envVars[eigendaflagsv2.RelayTimeoutFlagName] = "5s"
	envVars[eigendaflagsv2.ContractCallTimeoutFlagName] = "5s"
	envVars[eigendaflagsv2.BlobParamsVersionFlagName] = "0"
	envVars[eigendaflagsv2.MaxBlobLengthFlagName] = maxBlobLengthString
}

func setKZGEnvVars(envVars map[string]string) {
	envVars[verify.G1PathFlagName] = "../resources/g1.point"
	envVars[verify.G2PowerOf2PathFlagName] = "../resources/g2.point.powerOf2"
	envVars[verify.CachePathFlagName] = "../resources/SRSTables"
}

func setS3EnvVars(envVars map[string]string) {
	envVars[s3.EnableTLSFlagName] = fmt.Sprintf("%v", false)
	envVars[s3.CredentialTypeFlagName] = string(s3.CredentialTypeStatic)
	envVars[s3.AccessKeyIDFlagName] = minioAdmin
	envVars[s3.AccessKeySecretFlagName] = minioAdmin
	envVars[s3.BucketFlagName] = bucketName
	envVars[s3.EndpointFlagName] = minioEndpoint
	envVars[store.CacheTargetsFlagName] = "S3"
}

func setRedisEnvVars(envVars map[string]string) {
	envVars[redis.DBFlagName] = "0"
	envVars[redis.EvictionFlagName] = "10m"
	envVars[redis.EndpointFlagName] = redisEndpoint
	envVars[redis.PasswordFlagName] = ""
	envVars[store.CacheTargetsFlagName] = "redis"
}

// configureContextFromEnvMap accepts a map from env flag name, to flag value, as well as a list of all cli flags in
// the system. It creates a new cli.Context, with the input env flags set to the desired values.
func configureContextFromEnvMap(envMap map[string]string, flags []cli.Flag) (*cli.Context, error) {
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

	// Set values from the env map
	for name, value := range envMap {
		if err := ctx.Set(name, value); err != nil {
			return nil, fmt.Errorf("set flag %s to value %s: %w", name, value, err)
		}
	}

	return ctx, nil
}

type TestSuite struct {
	Ctx     context.Context
	Log     logging.Logger
	Metrics *proxy_metrics.EmulatedMetricer
	Server  *server.Server
}

func TestSuiteWithLogger(log logging.Logger) func(*TestSuite) {
	return func(ts *TestSuite) {
		ts.Log = log
	}
}

func buildTestAppConfig(testCfg TestConfig) config.AppConfig {
	envVars := getTestEnvVars(testCfg)
	cliFlags := config.CreateCLIFlags()
	cliContext, err := configureContextFromEnvMap(envVars, cliFlags)
	if err != nil {
		panic(fmt.Errorf("configure context from env map: %w", err))
	}
	appConfig, err := config.ReadCLIConfig(cliContext)
	if err != nil {
		panic(fmt.Errorf("read cli config: %w", err))
	}

	if err := appConfig.Check(); err != nil {
		panic(fmt.Errorf("check app config: %w", err))
	}
	configString, err := appConfig.EigenDAConfig.ToString()
	if err != nil {
		panic(fmt.Errorf("convert config string to json: %w", err))
	}

	println("Initializing EigenDA proxy server with config (\"*****\" fields are hidden): %v", configString)

	return appConfig
}

func CreateTestSuite(testCfg TestConfig, options ...func(*TestSuite)) (TestSuite, func()) {
	appConfig := buildTestAppConfig(testCfg)

	ts := &TestSuite{
		Ctx:     context.Background(),
		Log:     logging.NewTextSLogger(os.Stdout, &logging.SLoggerOptions{}),
		Metrics: proxy_metrics.NewEmulatedMetricer(),
	}
	// Override the defaults with the provided options, if present.
	for _, option := range options {
		option(ts)
	}
	ctx, logger, metrics := ts.Ctx, ts.Log, ts.Metrics

	proxyServer, err := server.BuildAndStartProxyServer(ctx, logger, metrics, appConfig)
	if err != nil {
		panic(fmt.Errorf("build and start proxy server: %w", err))
	}

	kill := func() {
		if err := proxyServer.Stop(); err != nil {
			log.Error("failed to stop proxy server", "err", err)
		}
	}

	return TestSuite{
		Ctx:     ctx,
		Log:     logger,
		Metrics: metrics,
		Server:  proxyServer,
	}, kill
}

func (ts *TestSuite) Address() string {
	// read port from listener
	port := ts.Server.Port()

	return fmt.Sprintf("%s://%s:%d", transport, host, port)
}

func createS3Bucket(bucketName string) {
	// Initialize minio client object.
	endpoint := minioEndpoint
	accessKeyID := minioAdmin
	secretAccessKey := minioAdmin
	useSSL := false

	minioClient, err := minio.New(
		endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
			Secure: useSSL,
		})
	if err != nil {
		panic(err)
	}

	location := "us-east-1"

	ctx := context.Background()
	err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{Region: location})
	if err != nil {
		// Check to see if we already own this bucket (which happens if you run this twice)
		exists, errBucketExists := minioClient.BucketExists(ctx, bucketName)
		if errBucketExists == nil && exists {
			log.Info(fmt.Sprintf("We already own %s\n", bucketName))
		} else {
			panic(err)
		}
	} else {
		log.Info(fmt.Sprintf("Successfully created %s\n", bucketName))
	}
}

func RandStr(n int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz")
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func RandBytes(n int) []byte {
	return []byte(RandStr(n))
}
