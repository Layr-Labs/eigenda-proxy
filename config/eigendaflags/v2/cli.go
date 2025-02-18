package eigendaflags

import (
	"net"
	"time"

	"github.com/Layr-Labs/eigenda-proxy/common"
	"github.com/Layr-Labs/eigenda/api/clients/codecs"
	v2_clients "github.com/Layr-Labs/eigenda/api/clients/v2"
	"github.com/urfave/cli/v2"
)

var (
	// This is a temporary feature flag that will be deprecated once all client
	// dependencies migrate to using EigenDA V2 network
	V2EnabledFlagName       = withFlagPrefix("enabled")
	DisperserFlagName       = withFlagPrefix("disperser-rpc")
	DisableTLSFlagName      = withFlagPrefix("disable-tls")
	CustomQuorumIDsFlagName = withFlagPrefix("custom-quorum-ids")
	BlobStatusPollInterval  = withFlagPrefix("blob-status-poll-interval")
	// TODO: Determine whether we should change this to something like PaymentPrivateKeyHex
	PutBlobEncodingVersionFlagName = withFlagPrefix("put-blob-encoding-version")
	// TODO: Consider renaming this to FFT mode or something pseudo-similar
	DisablePointVerificationModeFlagName = withFlagPrefix("disable-point-verification-mode")

	PutRetriesFlagName           = withFlagPrefix("put-retries")
	SignerPaymentKeyHexFlagName  = withFlagPrefix("signer-payment-key-hex")
	DisperseBlobTimeoutFlagName  = withFlagPrefix("disperse-blob-timeout")
	BlobCertifiedTimeoutFlagName = withFlagPrefix("blob-certified-timeout")
	CertVerifierAddrFlagName     = withFlagPrefix("cert-verifier-addr")
	RelayTimeoutFlagName         = withFlagPrefix("relay-timeout")
	ContractCallTimeoutFlagName  = withFlagPrefix("contract-call-timeout")
	BlobVersionFlagName          = withFlagPrefix("blob-version")
	EthRPCURLFlagName            = withFlagPrefix("eth-rpc")
	SvcManagerAddrFlagName       = withFlagPrefix("svc-manager-addr")
)

func withFlagPrefix(s string) string {
	return "eigenda.v2." + s
}

func withEnvPrefix(envPrefix, s string) string {
	return envPrefix + "_EIGENDA_V2_" + s
}
func CLIFlags(envPrefix, category string) []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     DisperserFlagName,
			Usage:    "RPC endpoint of the EigenDA disperser.",
			EnvVars:  []string{withEnvPrefix(envPrefix, "DISPERSER_RPC")},
			Category: category,
		},
		&cli.BoolFlag{
			Name:     DisableTLSFlagName,
			Usage:    "Disable TLS for gRPC communication with the EigenDA disperser. Default is false.",
			Value:    false,
			EnvVars:  []string{withEnvPrefix(envPrefix, "GRPC_DISABLE_TLS")},
			Category: category,
		},
		&cli.UintSliceFlag{
			Name:     CustomQuorumIDsFlagName,
			Usage:    "Custom quorum IDs for writing blobs. Should not include default quorums 0 or 1.",
			Value:    cli.NewUintSlice(),
			EnvVars:  []string{withEnvPrefix(envPrefix, "CUSTOM_QUORUM_IDS")},
			Category: category,
		},
		&cli.StringFlag{
			Name:     SignerPaymentKeyHexFlagName,
			Usage:    "Hex-encoded signer private key. Used for authn/authz and rate limits on EigenDA disperser. Should not be associated with an Ethereum address holding any funds.",
			EnvVars:  []string{withEnvPrefix(envPrefix, "SIGNER_PRIVATE_KEY_HEX")},
			Category: category,
		},
		&cli.UintFlag{
			Name:     PutBlobEncodingVersionFlagName,
			Usage:    "Blob encoding version to use when writing blobs from the high-level interface.",
			EnvVars:  []string{withEnvPrefix(envPrefix, "PUT_BLOB_ENCODING_VERSION")},
			Value:    0,
			Category: category,
		},
		&cli.BoolFlag{
			Name:     DisablePointVerificationModeFlagName,
			Usage:    "Disable point verification mode. This mode performs IFFT on data before writing and FFT on data after reading. Disabling requires supplying the entire blob for verification against the KZG commitment.",
			EnvVars:  []string{withEnvPrefix(envPrefix, "DISABLE_POINT_VERIFICATION_MODE")},
			Value:    false,
			Category: category,
		},
		&cli.StringFlag{
			Name:     EthRPCURLFlagName,
			Usage:    "URL of the Ethereum RPC endpoint. Needed to confirm blobs landed onchain.",
			EnvVars:  []string{withEnvPrefix(envPrefix, "ETH_RPC")},
			Category: category,
			Required: false,
		},
		&cli.StringFlag{
			Name:     SvcManagerAddrFlagName,
			Usage:    "Address of the EigenDAServiceManager contract. Required to confirm blobs landed onchain. See https://github.com/Layr-Labs/eigenlayer-middleware/?tab=readme-ov-file#current-mainnet-deployment",
			EnvVars:  []string{withEnvPrefix(envPrefix, "SERVICE_MANAGER_ADDR")},
			Category: category,
			Required: false,
		},
		&cli.UintFlag{
			Name:     PutRetriesFlagName,
			Usage:    "Number of times to retry blob dispersals.",
			Value:    3,
			EnvVars:  []string{withEnvPrefix(envPrefix, "PUT_RETRIES")},
			Category: category,
		},
		&cli.DurationFlag{
			Name:     DisperseBlobTimeoutFlagName,
			Usage:    "Maximum amount of time to wait for a blob to disperse against v2 protocol.",
			EnvVars:  []string{withEnvPrefix(envPrefix, "DISPERSE_BLOB_TIMEOUT")},
			Category: category,
			Required: false,
			Value:    time.Minute * 2,
		},
		&cli.DurationFlag{
			Name:     BlobCertifiedTimeoutFlagName,
			Usage:    "Maximum amount of time to wait for blob certification against the on-chain CertVerifier.",
			EnvVars:  []string{withEnvPrefix(envPrefix, "CERTIFY_BLOB_TIMEOUT")},
			Category: category,
			Required: false,
			Value:    time.Second * 30,
		},
		&cli.BoolFlag{
			Name:     V2EnabledFlagName,
			Usage:    "Enable blob dispersal and retrieval against EigenDA v2 protocol.",
			EnvVars:  []string{withEnvPrefix(envPrefix, "V2_ENABLED")},
			Category: category,
			Required: false,
		},
		&cli.StringFlag{
			Name:     CertVerifierAddrFlagName,
			Usage:    "Address of the EigenDACertVerifier contract. Required for performing eth_calls to verify EigenDA certificates.",
			EnvVars:  []string{withEnvPrefix(envPrefix, "CERT_VERIFIER_ADDR")},
			Category: category,
			Required: false,
		},
		&cli.DurationFlag{
			Name:     ContractCallTimeoutFlagName,
			Usage:    "Timeout used when performing smart contract eth_calls.",
			EnvVars:  []string{withEnvPrefix(envPrefix, "CONTRACT_CALL_TIMEOUT")},
			Category: category,
			Value:    10 * time.Second,
			Required: false,
		},
		&cli.DurationFlag{
			Name:     RelayTimeoutFlagName,
			Usage:    "Timeout used when querying a relay for blob contents.",
			EnvVars:  []string{withEnvPrefix(envPrefix, "RELAY_TIMEOUT")},
			Category: category,
			Value:    10 * time.Second,
			Required: false,
		},
		&cli.DurationFlag{
			Name:     BlobStatusPollInterval,
			Usage:    "Duration to query for blob status updates during dispersal.",
			EnvVars:  []string{withEnvPrefix(envPrefix, "BLOB_STATUS_POLL_INTERVAL")},
			Category: category,
			Value:    1 * time.Second,
			Required: false,
		},
		&cli.UintFlag{
			Name:     BlobVersionFlagName,
			Usage:    "Blob version used when dispersing. Currently only supports (0).",
			EnvVars:  []string{withEnvPrefix(envPrefix, "BLOB_VERSION")},
			Category: category,
			Value:    uint(0),
			Required: false,
		},
	}
}

func ReadConfig(ctx *cli.Context) common.V2ClientConfig {
	return common.V2ClientConfig{
		Enabled:               ctx.Bool(V2EnabledFlagName),
		ServiceManagerAddress: ctx.String(SvcManagerAddrFlagName),

		DisperserClientCfg: readDisperserCfg(ctx),
		PayloadClientCfg:   readPayloadDisperserCfg(ctx),
		RetrievalConfig:    readRetrievalConfig(ctx),
		EthRPC:             ctx.String(EthRPCURLFlagName),
		PutRetries:         ctx.Uint(PutRetriesFlagName),
	}
}

func readPayloadClientConfig(ctx *cli.Context) v2_clients.PayloadClientConfig {
	noPolynomial := ctx.Bool(DisablePointVerificationModeFlagName)
	polyMode := codecs.PolynomialFormCoeff

	// if point verification mode is disabled then blob is treated as evaluations and
	// not FFT'd before dispersal
	if noPolynomial {
		polyMode = codecs.PolynomialFormEval
	}

	return v2_clients.PayloadClientConfig{
		// TODO: Support proper user env injection
		BlockNumberPollInterval: 1 * time.Second,
		BlobEncodingVersion:     codecs.DefaultBlobEncoding,
		EigenDACertVerifierAddr: ctx.String(CertVerifierAddrFlagName),
		PayloadPolynomialForm:   polyMode,
		BlobVersion:             uint16(ctx.Int(BlobVersionFlagName)),
	}
}

func readPayloadDisperserCfg(ctx *cli.Context) v2_clients.PayloadDisperserConfig {
	payCfg := readPayloadClientConfig(ctx)

	return v2_clients.PayloadDisperserConfig{
		SignerPaymentKey:    ctx.String(SignerPaymentKeyHexFlagName),
		PayloadClientConfig: payCfg,
		DisperseBlobTimeout: ctx.Duration(DisperseBlobTimeoutFlagName),
		// TODO: Explore making these user defined
		BlobCertifiedTimeout:   ctx.Duration(BlobCertifiedTimeoutFlagName),
		BlobStatusPollInterval: ctx.Duration(BlobStatusPollInterval),
		Quorums:                []uint8{0, 1},
	}
}

func readDisperserCfg(ctx *cli.Context) v2_clients.DisperserClientConfig {
	hostStr, portStr, err := net.SplitHostPort(ctx.String(DisperserFlagName))
	if err != nil {
		panic(err)
	}

	return v2_clients.DisperserClientConfig{
		Hostname:          hostStr,
		Port:              portStr,
		UseSecureGrpcFlag: !ctx.Bool(DisableTLSFlagName),
	}

}

func readRetrievalConfig(ctx *cli.Context) v2_clients.RelayPayloadRetrieverConfig {
	return v2_clients.RelayPayloadRetrieverConfig{
		PayloadClientConfig: readPayloadClientConfig(ctx),
		RelayTimeout:        ctx.Duration(RelayTimeoutFlagName),
	}
}
