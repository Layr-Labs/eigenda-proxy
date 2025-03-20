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
}

// TestSuiteWithLogger returns a function which overrides the logger for a TestSuite
func TestSuiteWithLogger(log logging.Logger) func(*TestSuite) {
	return func(ts *TestSuite) {
		ts.Log = log
	}
}

// CreateTestSuiteWithFlagOverrides creates a test suite.
//
// In addition to the options provided by CreateTestSuite, this method also accepts a list of FlagConfigs. The
// configurations passed in as part of this parameter are used to override the default test flag configurations.
func CreateTestSuiteWithFlagOverrides(
	backend Backend,
	disperseToV2 bool,
	flagOverrides []FlagConfig,
	options ...func(*TestSuite),
) (TestSuite, func()) {
	ts := &TestSuite{
		Ctx:     context.Background(),
		Log:     logging.NewTextSLogger(os.Stdout, &logging.SLoggerOptions{}),
		Metrics: proxy_metrics.NewEmulatedMetricer(),
	}
	// Override the defaults with the provided options, if present.
	for _, option := range options {
		option(ts)
	}

	appConfig := buildTestAppConfig(backend, disperseToV2, flagOverrides)

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

// CreateTestSuite constructs a new TestSuite
//
// It accepts parameters indicating which type of Backend to use, and whether it should disperse to v2.
// It also accepts a variadic options parameter, which contains functions that operate on a TestSuite object.
// These options allow for configuration control over the TestSuite.
func CreateTestSuite(backend Backend, disperseToV2 bool, options ...func(*TestSuite)) (TestSuite, func()) {
	return CreateTestSuiteWithFlagOverrides(backend, disperseToV2, nil, options...)
}

func (ts *TestSuite) Address() string {
	// read port from listener
	port := ts.Server.Port()

	return fmt.Sprintf("%s://%s:%d", transport, host, port)
}
