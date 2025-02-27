package eigendaflags

import (
	"fmt"
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
	V2EnabledFlagName = withFlagPrefix("enabled")

	DisperserFlagName               = withFlagPrefix("disperser-rpc")
	DisableTLSFlagName              = withFlagPrefix("disable-tls")
	CustomQuorumIDsFlagName         = withFlagPrefix("custom-quorum-ids")
	BlobStatusPollIntervalFlagName  = withFlagPrefix("blob-status-poll-interval")
	PayloadEncodingVersionFlagName  = withFlagPrefix("payload-encoding-version")
	PointEvaluationDisabledFlagName = withFlagPrefix("polynomial-form")

	PutRetriesFlagName              = withFlagPrefix("put-retries")
	SignerPaymentKeyHexFlagName     = withFlagPrefix("signer-payment-key-hex")
	DisperseBlobTimeoutFlagName     = withFlagPrefix("disperse-blob-timeout")
	BlobCertifiedTimeoutFlagName    = withFlagPrefix("blob-certified-timeout")
	CertVerifierAddrFlagName        = withFlagPrefix("cert-verifier-addr")
	RelayTimeoutFlagName            = withFlagPrefix("relay-timeout")
	ContractCallTimeoutFlagName     = withFlagPrefix("contract-call-timeout")
	BlobParamsVersionFlagName       = withFlagPrefix("blob-version")
	BlockNumberPollIntervalFlagName = withFlagPrefix("block-number-poll-interval")
	EthRPCURLFlagName               = withFlagPrefix("eth-rpc")
	SvcManagerAddrFlagName          = withFlagPrefix("svc-manager-addr")
)

func withFlagPrefix(s string) string {
	return "eigenda.v2." + s
}

func withEnvPrefix(envPrefix, s string) string {
	return envPrefix + "_EIGENDA_V2_" + s
}
func CLIFlags(envPrefix, category string) []cli.Flag {
	return []cli.Flag{
		&cli.BoolFlag{
			Name:     V2EnabledFlagName,
			Usage:    "Enable blob dispersal and retrieval against EigenDA V2 protocol.",
			EnvVars:  []string{withEnvPrefix(envPrefix, "ENABLED")},
			Category: category,
			Required: false,
		},
		&cli.StringFlag{
			Name:     DisperserFlagName,
			Usage:    "RPC endpoint of the EigenDA disperser.",
			EnvVars:  []string{withEnvPrefix(envPrefix, "DISPERSER_RPC")},
			Category: category,
		},
		&cli.BoolFlag{
			Name:     DisableTLSFlagName,
			Usage:    "Disable TLS for gRPC communication with the EigenDA disperser and retrieval subnet.",
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
			Usage:    "Hex-encoded signer private key. Used for authorizing payments with EigenDA disperser. Should not be associated with an Ethereum address holding any funds.",
			EnvVars:  []string{withEnvPrefix(envPrefix, "SIGNER_PRIVATE_KEY_HEX")},
			Category: category,
		},
		&cli.UintFlag{
			Name:     PayloadEncodingVersionFlagName,
			Usage:    "Payload encoding version used for transforming payloads into an EigenDA blob representation. Currently only supports (0).",
			EnvVars:  []string{withEnvPrefix(envPrefix, "PAYLOAD_ENCODING_VERSION")},
			Value:    0,
			Category: category,
		},
		&cli.BoolFlag{
			Name:     PointEvaluationDisabledFlagName,
			Usage:    "Disables IFFT transformation done during payload encoding. Using this mode results in blobs that can't be proven.",
			EnvVars:  []string{withEnvPrefix(envPrefix, "COEFFICIENT_PAYLOAD_DISABLED")},
			Value:    false,
			Category: category,
		},
		&cli.StringFlag{
			Name:     EthRPCURLFlagName,
			Usage:    "URL of the Ethereum RPC endpoint. Needed to verify EigenDA certs against an onchain EigenDACertVerifier contract.",
			EnvVars:  []string{withEnvPrefix(envPrefix, "ETH_RPC")},
			Category: category,
			Required: false,
		},
		&cli.StringFlag{
			Name: SvcManagerAddrFlagName,
			Usage: `Address of the EigenDAServiceManager contract. Required for initializing onchain system context and reading relay states from registry.
					   See https://github.com/Layr-Labs/eigenlayer-middleware/?tab=readme-ov-file#current-mainnet-deployment`,
			EnvVars:  []string{withEnvPrefix(envPrefix, "SERVICE_MANAGER_ADDR")},
			Category: category,
			Required: false,
		},
		&cli.UintFlag{
			Name:     PutRetriesFlagName,
			Usage:    "Number of times to retry blob dispersals before serving an error response.",
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
			Usage:    "Maximum amount of time to wait for blob certification against the on-chain EigenDACertVerifier.",
			EnvVars:  []string{withEnvPrefix(envPrefix, "CERTIFY_BLOB_TIMEOUT")},
			Category: category,
			Required: false,
			Value:    time.Second * 30,
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
			Usage:    "Timeout used when performing smart contract call operation (i.e, eth_call).",
			EnvVars:  []string{withEnvPrefix(envPrefix, "CONTRACT_CALL_TIMEOUT")},
			Category: category,
			Value:    10 * time.Second,
			Required: false,
		},
		&cli.DurationFlag{
			Name:     RelayTimeoutFlagName,
			Usage:    "Timeout used when querying an individual relay for blob contents.",
			EnvVars:  []string{withEnvPrefix(envPrefix, "RELAY_TIMEOUT")},
			Category: category,
			Value:    10 * time.Second,
			Required: false,
		},
		&cli.DurationFlag{
			Name:     BlobStatusPollIntervalFlagName,
			Usage:    "Duration to query for blob status updates during dispersal.",
			EnvVars:  []string{withEnvPrefix(envPrefix, "BLOB_STATUS_POLL_INTERVAL")},
			Category: category,
			Value:    1 * time.Second,
			Required: false,
		},
		&cli.UintFlag{
			Name: BlobParamsVersionFlagName,
			Usage: `Blob params version used when dispersing. 
					   This refers to a global version maintained by EigenDA governance 
					   and is injected in the BlobHeader before dispersing.
					   Currently only supports (0).`,
			EnvVars:  []string{withEnvPrefix(envPrefix, "BLOB_PARAMS_VERSION")},
			Category: category,
			Value:    uint(0),
			Required: false,
		},
		&cli.UintFlag{
			Name: BlockNumberPollIntervalFlagName,
			Usage: `Polling interval used for querying latest block from ETH RPC provider.
					   Latest blocks are queried as a precondition to ensure the node is up-to-date
					   or >= reference block number that the EigenDA disperser accredited the certificate.`,
			EnvVars:  []string{withEnvPrefix(envPrefix, "BLOCK_NUMBER_POLL_INTERVAL")},
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
	polyForm := codecs.PolynomialFormEval

	// if point evaluation mode is disabled then blob is treated as coefficients and
	// not iFFT'd before dispersal and FFT'd on retrieval
	if ctx.Bool(PointEvaluationDisabledFlagName) {
		polyForm = codecs.PolynomialFormCoeff
	}

	return v2_clients.PayloadClientConfig{
		BlockNumberPollInterval: ctx.Duration(BlockNumberPollIntervalFlagName),
		PayloadEncodingVersion:  codecs.PayloadEncodingVersion0,
		PayloadPolynomialForm:   polyForm,
		// #nosec G115 - only overflow on incorrect user input
		BlobVersion: uint16(ctx.Int(BlobParamsVersionFlagName)),
	}
}

func readPayloadDisperserCfg(ctx *cli.Context) v2_clients.PayloadDisperserConfig {
	payCfg := readPayloadClientConfig(ctx)

	return v2_clients.PayloadDisperserConfig{
		SignerPaymentKey:       ctx.String(SignerPaymentKeyHexFlagName),
		PayloadClientConfig:    payCfg,
		DisperseBlobTimeout:    ctx.Duration(DisperseBlobTimeoutFlagName),
		BlobCertifiedTimeout:   ctx.Duration(BlobCertifiedTimeoutFlagName),
		BlobStatusPollInterval: ctx.Duration(BlobStatusPollIntervalFlagName),
	}
}

func readDisperserCfg(ctx *cli.Context) v2_clients.DisperserClientConfig {
	hostStr, portStr, err := net.SplitHostPort(ctx.String(DisperserFlagName))
	if err != nil {
		panic(fmt.Sprintf("could not read disperser RPC port from provided endpoint: %s", err.Error()))
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
