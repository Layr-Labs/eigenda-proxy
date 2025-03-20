package testutils

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Layr-Labs/eigenda-proxy/config"
	"github.com/ethereum/go-ethereum/log"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"golang.org/x/exp/rand"

	miniotc "github.com/testcontainers/testcontainers-go/modules/minio"
	redistc "github.com/testcontainers/testcontainers-go/modules/redis"
)

const (
	minioAdmin    = "minioadmin"
	backendEnvVar = "BACKEND"
)

type Backend int

const (
	TestnetBackend Backend = iota + 1
	PreprodBackend
	MemstoreBackend
)

// ParseBackend converts a string to a Backend enum (case insensitive)
func ParseBackend(inputString string) (Backend, error) {
	switch strings.ToLower(inputString) {
	case "testnet":
		return TestnetBackend, nil
	case "preprod":
		return PreprodBackend, nil
	case "memstore":
		return MemstoreBackend, nil
	default:
		return 0, fmt.Errorf("invalid backend: %s", inputString)
	}
}

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

func GetBackend() Backend {
	backend, err := ParseBackend(os.Getenv(backendEnvVar))
	if err != nil {
		panic(fmt.Sprintf("BACKEND must be = memstore|testnet|preprod. parse backend error: %v", err))
	}
	return backend
}

func buildTestAppConfig(backend Backend, disperseToV2 bool, overriddenFlags []FlagConfig) config.AppConfig {
	cliFlags := config.CreateCLIFlags()

	flagConfigs := getDefaultTestFlags(backend, disperseToV2)
	flagConfigs = append(flagConfigs, overriddenFlags...)

	cliContext, err := configureContextFromFlags(flagConfigs, cliFlags)
	if err != nil {
		panic(fmt.Errorf("configure context from flags: %w", err))
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
