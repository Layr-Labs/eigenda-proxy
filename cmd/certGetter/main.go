package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
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
	"github.com/ethereum/go-ethereum/rlp"
)

func main() {
	ctx := context.Background()
	privateKeyHex := "966d6501da9ff16d1b460ea17e474b0efa27b0baf505af1162878b14e33afdf7"
	signer := auth.NewLocalBlobRequestSigner(privateKeyHex)

	RPCURLs := make([]string, 1)
	RPCURLs[0] = string("https://ethereum-holesky.publicnode.com") // public rpc  https://ethereum-holesky.publicnode.com

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

	blobKeyHex := "e286fabf623bbc07cd3e25cf7efa4d148d62838ffca8edae6cb6db5fe86902d1"
	blobKey1, err := hex.DecodeString(blobKeyHex)
	if err != nil {
		fmt.Println("blobKey1 err")
		panic(err)
	}

	// holesky cert verifier address
	certVerifierAddress := "0x5c33Ce64EE04400fD593F960d63336F1B65bF77B"

	blobKey := core.BlobKey(blobKey1)

	blobStatusReply, err := disperserClient.GetBlobStatus(timeoutCtx, blobKey)
	if err != nil {
		panic("poll blob status until certified")
	}
	fmt.Println("Blob status CERTIFIED", "blobKey", blobKey)
	fmt.Println("Blob status CERTIFIED", "blobStatusReply", blobStatusReply)

	certVerifier, err := verification.NewCertVerifier(*ethClient, certVerifierAddress)
	if err != nil {
		fmt.Println("NewCertVerifier err", err)
		panic("")
	}

	eigenDACert, err := buildEigenDACert(ctx, blobKey, blobStatusReply, certVerifier)
	if err != nil {
		fmt.Errorf("failed to build eigenDACert: %w", err)
		panic("")
	}

	// write batch header
	jsonBytes, err := rlp.EncodeToBytes(eigenDACert.BatchHeader)
	if err != nil {
		fmt.Errorf("failed to encode DA cert to RLP format: %w", err)
		panic("")
	}
	fmt.Println("BatchHeader")
	data, err := json.MarshalIndent(eigenDACert.BatchHeader, "", "  ")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println(string(data))

	f, err := os.Create("batch_header.rlp")
	if err != nil {
		fmt.Println("Create err", err)
		panic("")
	}
	defer f.Close()
	_, err = f.Write(jsonBytes)
	if err != nil {
		fmt.Println("Write err", err)
		panic("")
	}

	// write nonsign
	fmt.Println("NonSignerStakesAndSignature")
	data, err = json.MarshalIndent(eigenDACert.NonSignerStakesAndSignature, "", "  ")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println(string(data))

	jsonBytes, err = rlp.EncodeToBytes(eigenDACert.NonSignerStakesAndSignature)
	if err != nil {
		fmt.Errorf("failed to encode DA cert to RLP format: %w", err)
		panic("")
	}

	f, err = os.Create("non_signer.rlp")
	if err != nil {
		fmt.Println("Create err", err)
		panic("")
	}
	defer f.Close()
	_, err = f.Write(jsonBytes)
	if err != nil {
		fmt.Println("Write err", err)
		panic("")
	}

	// write
	fmt.Println("BlobInclusionInfo")
	jsonBytes, err = rlp.EncodeToBytes(eigenDACert.BlobInclusionInfo)
	if err != nil {
		fmt.Errorf("failed to encode DA cert to RLP format: %w", err)
		panic("")
	}

	data, err = json.MarshalIndent(eigenDACert.BlobInclusionInfo, "", "  ")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println(string(data))

	f, err = os.Create("blob_inclusion.rlp")
	if err != nil {
		fmt.Println("Create err", err)
		panic("")
	}
	defer f.Close()
	_, err = f.Write(jsonBytes)
	if err != nil {
		fmt.Println("Write err", err)
		panic("")
	}
	return

	timeoutCtx, cancel = context.WithTimeout(ctx, dur)
	defer cancel()
	err = certVerifier.VerifyCertV2(timeoutCtx, eigenDACert)
	if err != nil {
		fmt.Printf("VerifyCertV2 has problem %w", err)
	}

	fmt.Printf("certVerifier.VerifyCertV2 onchain call verify correctly")
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
	fmt.Println("Retrieved NonSignerStakesAndSignature", "blobKey", hex.EncodeToString(blobKey[:]))
	//fmt.Println("Retrieved NonSignerStakesAndSignature", "nonSignerStakesAndSignature", nonSignerStakesAndSignature)

	eigenDACert, err := verification.BuildEigenDACert(blobStatusReply, nonSignerStakesAndSignature)
	if err != nil {
		return nil, fmt.Errorf("build eigen da cert: %w", err)
	}
	fmt.Println("Constructed EigenDACert", "blobKey", blobKey)

	return eigenDACert, nil
}
