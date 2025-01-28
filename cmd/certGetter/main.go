package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/Layr-Labs/eigenda/api/clients/v2"
	"github.com/Layr-Labs/eigenda/api/clients/v2/verification"
	dispgrpc "github.com/Layr-Labs/eigenda/api/grpc/disperser/v2"
	"github.com/Layr-Labs/eigenda/common/geth"
	auth "github.com/Layr-Labs/eigenda/core/auth/v2"
	core "github.com/Layr-Labs/eigenda/core/v2"

	"github.com/Layr-Labs/eigensdk-go/logging"
	gethcommon "github.com/ethereum/go-ethereum/common"
)

func main() {
	ctx := context.Background()
	privateKeyHex := "966d6501da9ff16d1b460ea17e474b0efa27b0baf505af1162878b14e33afdf7"
	signer := auth.NewLocalBlobRequestSigner(privateKeyHex)

	RPCURLs := make([]string, 1)
	RPCURLs[0] = string("https://ethereum-holesky.publicnode.com") // public rpc

	ethConfig := geth.EthClientConfig{
		RPCURLs:          RPCURLs,
		PrivateKeyString: "966d6501da9ff16d1b460ea17e474b0efa27b0baf505af1162878b14e33afdf7", // just random private key without any funds
		NumConfirmations: 1,
		NumRetries:       1,
	}

	senderAddress := gethcommon.HexToAddress("0x39F511BFdd750173B83E04367339cE4aFa590836")

	logger := logging.NewTextSLogger(os.Stdout, &logging.SLoggerOptions{
		Level: slog.LevelDebug,
	})

	ethClient, err := geth.NewClient(ethConfig, senderAddress, 0, logger)
	if err != nil {
		fmt.Println("dial ETH RPC node")
		panic("")
	}

	disperserClient, err := clients.NewDisperserClient(&clients.DisperserClientConfig{
		Hostname:          "disperser-preprod-holesky.eigenda.xyz",
		Port:              "443",
		UseSecureGrpcFlag: true,
	}, signer, nil, nil)

	if err != nil {
		fmt.Println("NewDisperserClient err", err)
		panic("cannot create disperser client")
	}
	dur, _ := time.ParseDuration("20s")

	timeoutCtx, cancel := context.WithTimeout(ctx, dur)
	defer cancel()

	blobKeyHex := "f656168a4f7d4201444fc1f2aa567c815e4b91b03e251275a20617a8c69d011f"
	blobKey1, err := hex.DecodeString(blobKeyHex)
	if err != nil {
		fmt.Println("blobKey1 err")
		panic(err)
	}

	// holesky cert verifier address
	certVerifierAddress := "0xAfd0b614EB381800D6C35933Da8BAfbA1bDd255f"

	blobKey := core.BlobKey(blobKey1)

	blobStatusReply, err := disperserClient.GetBlobStatus(timeoutCtx, blobKey)
	if err != nil {
		panic("poll blob status until certified")
	}
	fmt.Println("Blob status CERTIFIED", "blobKey", blobKey)

	certVerifier, err := verification.NewCertVerifier(ethClient, certVerifierAddress)
	if err != nil {
		fmt.Println("NewCertVerifier err", err)
		panic("")
	}

	eigenDACert, err := buildEigenDACert(ctx, blobKey, blobStatusReply, certVerifier)
	fmt.Println("err", err)

	timeoutCtx, cancel = context.WithTimeout(ctx, dur)
	defer cancel()
	err = certVerifier.VerifyCertV2(timeoutCtx, eigenDACert)

	fmt.Println("verify cert for blobKey %v: %w", blobKey, err)
}

// buildEigenDACert makes a call to the getNonSignerStakesAndSignature view function on the EigenDACertVerifier
// contract, and then assembles an EigenDACert
func buildEigenDACert(
	ctx context.Context,
	blobKey core.BlobKey,
	blobStatusReply *dispgrpc.BlobStatusReply,
	certVerifier verification.ICertVerifier,
) (*verification.EigenDACert, error) {
	dur, _ := time.ParseDuration("20s")
	timeoutCtx, cancel := context.WithTimeout(ctx, dur)
	defer cancel()

	nonSignerStakesAndSignature, err := certVerifier.GetNonSignerStakesAndSignature(
		timeoutCtx, blobStatusReply.GetSignedBatch())
	if err != nil {
		return nil, fmt.Errorf("get non signer stake and signature: %w", err)
	}
	fmt.Println("Retrieved NonSignerStakesAndSignature", "blobKey", blobKey)

	eigenDACert, err := verification.BuildEigenDACert(blobStatusReply, nonSignerStakesAndSignature)
	if err != nil {
		return nil, fmt.Errorf("build eigen da cert: %w", err)
	}
	fmt.Println("Constructed EigenDACert", "blobKey", blobKey)

	return eigenDACert, nil
}
