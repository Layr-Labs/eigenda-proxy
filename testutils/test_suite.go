package testutils

import (
	"context"
	"fmt"
	"os"

	proxy_metrics "github.com/Layr-Labs/eigenda-proxy/metrics"
	"github.com/Layr-Labs/eigenda-proxy/server"
	"github.com/Layr-Labs/eigensdk-go/logging"
	"github.com/ethereum/go-ethereum/log"
)

// TestSuite contains necessary objects, to be able to execute a proxy test
type TestSuite struct {
	Ctx     context.Context
	Log     logging.Logger
	Metrics *proxy_metrics.EmulatedMetricer
	Server  *server.Server
	// OverriddenEnvVars are the environment variable configurations that should override the default configurations
	OverriddenEnvVars []EnvVar
}

// TestSuiteWithLogger returns a function which overrides the logger for a TestSuite
func TestSuiteWithLogger(log logging.Logger) func(*TestSuite) {
	return func(ts *TestSuite) {
		ts.Log = log
	}
}

// TestSuiteWithOverriddenEnvVars returns a function which sets the OverriddenEnvVars for a TestSuite
func TestSuiteWithOverriddenEnvVars(envVars ...EnvVar) func(*TestSuite) {
	return func(ts *TestSuite) {
		ts.OverriddenEnvVars = envVars
	}
}

// CreateTestSuite constructs a new TestSuite
//
// It accepts flags indicating whether memstore and/or v2 should be enabled.
// It also accepts a variadic options parameter, which contains functions that operate on a TestSuite object.
// These options allow for configuration control over the TestSuite.
func CreateTestSuite(backend Backend, useV2 bool, options ...func(*TestSuite)) (TestSuite, func()) {
	ts := &TestSuite{
		Ctx:     context.Background(),
		Log:     logging.NewTextSLogger(os.Stdout, &logging.SLoggerOptions{}),
		Metrics: proxy_metrics.NewEmulatedMetricer(),
	}
	// Override the defaults with the provided options, if present.
	for _, option := range options {
		option(ts)
	}

	appConfig := buildTestAppConfig(backend, useV2, ts.OverriddenEnvVars)

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
