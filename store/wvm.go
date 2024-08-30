package store

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/near/borsh-go"
)

type WVMClient struct {
	client  *ethclient.Client
	log     log.Logger
	privKey string
}

const (
	wvmRpcUrl  = "https://testnet-rpc.wvm.dev" // wvm rpc url
	wvmChainId = 9496                          // wvm chain id

	wvmArchiverAddress    = "0xF8a5a479f04d1565b925A67D088b8fC3f8f0b7eF" // we use it as a "from" address
	wvmArchivePoolAddress = "0x606dc1BE30A5966FcF3C10D907d1B76A7B1Bbbd9" // we use it as a "to" address
	data                  = "Hello WVM!"

	gasLimit = uint64(21500) // adjust this if necessary
	wei      = uint64(0)     // 0 Wei
)

/*
	1. client
	2. wallet
	3. compressing, brotli, borsch
	4. put to calldata
	5. sign tx
	6. send
	7. wait for confirmation, get tx id, return in log than return in response
	8. done
*/

func NewWVMClient(log log.Logger) *WVMClient {
	client, err := ethclient.Dial(wvmRpcUrl)
	if err != nil {
		panic(fmt.Sprintf("failed to connect to the WVM client: %v", err))
	}
	privKey := os.Getenv("WVM_PRIV_KEY")
	if privKey == "" {
		panic("wvm archiver signer key is empty")
	}

	return &WVMClient{client: client, log: log}
}

func (wvm *WVMClient) Store(ctx context.Context, eigenBlobData []byte) error {
	wvmData, err := wvm.wvmEncode(eigenBlobData)
	if err != nil {
		return fmt.Errorf("failed to store data in wvm: %w", err)
	}

	err = wvm.getSuggestedGasPrice(ctx)
	if err != nil {
		return fmt.Errorf("failed to store data in wvm: %w", err)
	}

	gas, err := wvm.estimateGas(ctx, wvmArchiverAddress, wvmArchivePoolAddress, wvmData)
	if err != nil {
		return fmt.Errorf("failed to store data in wvm: %w", err)
	}

	wvmRawTx, err := wvm.createRawTransaction(ctx, wvmArchivePoolAddress, string(wvmData), gas)
	if err != nil {
		return fmt.Errorf("failed to store data in wvm: %w", err)
	}

	err = wvm.sendRawTransaction(ctx, wvmRawTx)
	if err != nil {
		return fmt.Errorf("failed to store data in wvm: %w", err)
	}

	return nil
}

func (wvm *WVMClient) wvmEncode(eigenBlob []byte) ([]byte, error) {
	// borsch

	borshEncoded, err := borsh.Serialize(eigenBlob)
	wvm.log.Info("wvm: eigen blob serialized using borsh")
	if err != nil {
		return nil, err
	}

	// brotli
	brotliOut := bytes.Buffer{}
	writer := brotli.NewWriterOptions(&brotliOut, brotli.WriterOptions{Quality: 6})
	in := bytes.NewReader(borshEncoded)
	n, err := io.Copy(writer, in)
	if err != nil {
		panic(err)
	}
	if int(n) != len(borshEncoded) {
		panic("wvm: size mismatch during brotli compression")
	}
	if err := writer.Close(); err != nil {
		panic(fmt.Errorf("wvm: brotli writer close fail: %w", err))
	}

	return brotliOut.Bytes(), nil
}

// HELPERS

// getSuggestedGasPrice connects to an Ethereum node via RPC and retrieves the current suggested gas price.
func (wvm *WVMClient) getSuggestedGasPrice(ctx context.Context) error {
	gasPrice, err := wvm.client.SuggestGasPrice(ctx)
	if err != nil {
		return fmt.Errorf("failed to suggest gas price: %w", err)
	}

	// Print the suggested gas price to the terminal.
	wvm.log.Info("WVM suggested Gas Price:", gasPrice.String())

	return nil
}

// estimateGas tries estimates the suggested amount of gas that required to execute a given transaction.
func (wvm *WVMClient) estimateGas(ctx context.Context, from, to string, data []byte) (uint64, error) {
	var (
		fromAddr  = common.HexToAddress(from)
		toAddr    = common.HexToAddress(to)
		bytesData []byte
		err       error
	)

	var encoded string
	if string(data) != "" {
		if ok := strings.HasPrefix(string(data), "0x"); !ok {
			encoded = hexutil.Encode(data)
		}

		bytesData, err = hexutil.Decode(encoded)
		if err != nil {
			return 0, err
		}
	}

	msg := ethereum.CallMsg{
		From: fromAddr,
		To:   &toAddr,
		Gas:  0x00,
		Data: bytesData,
	}

	gas, err := wvm.client.EstimateGas(ctx, msg)
	if err != nil {
		return 0, err
	}

	wvm.log.Info("WVM estimated Gas Price:", gas)

	return gas, nil
}

// createRawTransaction creates a raw EIP-1559 transaction and returns it as a hex string.
func (wvm *WVMClient) createRawTransaction(ctx context.Context, to string, data string, gasLimit uint64) (string, error) {
	// Suggest the base fee for inclusion in a block.
	baseFee, err := wvm.client.SuggestGasPrice(ctx)
	if err != nil {
		return "", err
	}
	// priorityFee, err := wvm.client.SuggestGasTipCap(context.Background())
	// if err != nil {
	// 	return "", err
	// }

	// WVM: maybe we don't need this, but e.g.
	// increment := new(big.Int).Mul(big.NewInt(2), big.NewInt(params.GWei))
	// gasFeeCap := new(big.Int).Add(baseFee, increment)
	// gasFeeCap.Add(gasFeeCap, priorityFee)

	gasFeeCap := baseFee

	// address shenanigans
	// Decode the provided private key.
	privKey := os.Getenv("WVM_PRIV_KEY")
	if privKey == "" {
		panic("wvm archiver signer key is empty")
	}
	pKeyBytes, err := hexutil.Decode("0x" + privKey)
	if err != nil {
		return "", err
	}
	// Convert the private key bytes to an ECDSA private key.
	ecdsaPrivateKey, err := crypto.ToECDSA(pKeyBytes)
	if err != nil {
		return "", err
	}
	// Extract the public key from the ECDSA private key.
	publicKey := ecdsaPrivateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return "", fmt.Errorf("Error casting public key to ECDSA")
	}
	// Compute the Ethereum address of the signer from the public key.
	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	// Retrieve the nonce for the signer's account, representing the transaction count.
	nonce, err := wvm.client.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		return "", err
	}

	// Prepare data payload.
	var hexData string
	if strings.HasPrefix(data, "0x") {
		hexData = data
	} else {
		hexData = hexutil.Encode([]byte(data))
	}
	bytesData, err := hexutil.Decode(hexData)
	if err != nil {
		return "", err
	}

	// Set up the transaction fields, including the recipient address, value, and gas parameters.
	toAddr := common.HexToAddress(to)

	txData := types.DynamicFeeTx{
		ChainID:   big.NewInt(wvmChainId),
		Nonce:     nonce,
		GasTipCap: big.NewInt(0),
		GasFeeCap: gasFeeCap,
		Gas:       gasLimit,
		To:        &toAddr,
		Data:      bytesData,
	}

	tx := types.NewTx(&txData)
	signedTx, err := types.SignTx(tx, types.LatestSignerForChainID(big.NewInt(wvmChainId)), ecdsaPrivateKey)

	if err != nil {
		return "", err
	}

	// Encode the signed transaction into RLP (Recursive Length Prefix) format for transmission.
	var buf bytes.Buffer
	err = signedTx.EncodeRLP(&buf)

	if err != nil {
		return "", err
	}

	// Return the RLP-encoded transaction as a hexadecimal string.
	rawTxRLPHex := hex.EncodeToString(buf.Bytes())

	return rawTxRLPHex, nil
}

// Transaction represents the structure of the transaction JSON.
type Transaction struct {
	Type                 string   `json:"type"`
	ChainID              string   `json:"chainId"`
	Nonce                string   `json:"nonce"`
	To                   string   `json:"to"`
	Gas                  string   `json:"gas"`
	GasPrice             string   `json:"gasPrice,omitempty"`
	MaxPriorityFeePerGas string   `json:"maxPriorityFeePerGas"`
	MaxFeePerGas         string   `json:"maxFeePerGas"`
	Value                string   `json:"value"`
	Input                string   `json:"input"`
	AccessList           []string `json:"accessList"`
	V                    string   `json:"v"`
	R                    string   `json:"r"`
	S                    string   `json:"s"`
	YParity              string   `json:"yParity"`
	Hash                 string   `json:"hash"`
	TransactionTime      string   `json:"transactionTime,omitempty"`
	TransactionCost      string   `json:"transactionCost,omitempty"`
}

func (wvm *WVMClient) sendRawTransaction(ctx context.Context, rawTx string) error {
	rawTxBytes, err := hex.DecodeString(rawTx)
	if err != nil {
		return err
	}

	tx := new(types.Transaction)

	err = rlp.DecodeBytes(rawTxBytes, &tx)
	if err != nil {
		return err
	}

	err = wvm.client.SendTransaction(ctx, tx)
	if err != nil {
		return err
	}

	var txDetails Transaction
	txBytes, err := tx.MarshalJSON()
	if err != nil {
		return err
	}
	if err := json.Unmarshal(txBytes, &txDetails); err != nil {
		return err
	}

	txDetails.TransactionTime = tx.Time().Format(time.RFC822)
	txDetails.TransactionCost = tx.Cost().String()

	convertFields := []string{"Nonce", "MaxPriorityFeePerGas", "MaxFeePerGas", "Value", "Type", "Gas"}
	for _, field := range convertFields {
		if err := convertHexField(&txDetails, field); err != nil {
			return err
		}
	}

	txJSON, err := json.MarshalIndent(txDetails, "", "\t")
	if err != nil {
		return err
	}

	wvm.log.Info("WVM:Raw TX Receipt:", string(txJSON))

	return nil
}

// helper function
func convertHexField(tx *Transaction, field string) error {
	typeOfTx := reflect.TypeOf(*tx)

	// Get the value of the Transaction struct
	txValue := reflect.ValueOf(tx).Elem()

	// Parse the hexadecimal string as an integer
	hexStr := txValue.FieldByName(field).String()

	intValue, err := strconv.ParseUint(hexStr[2:], 16, 64)
	if err != nil {
		return err
	}

	// Convert the integer to a decimal string
	decimalStr := strconv.FormatUint(intValue, 10)

	// Check if the field exists
	_, ok := typeOfTx.FieldByName(field)
	if !ok {
		return fmt.Errorf("field %s does not exist in Transaction struct", field)
	}

	// Set the field value to the decimal string
	txValue.FieldByName(field).SetString(decimalStr)

	return nil
}
