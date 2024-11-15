package wvm

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Layr-Labs/eigenda-proxy/common"
	rpc "github.com/Layr-Labs/eigenda-proxy/store/precomputed_key/wvm/rpc"
	signer "github.com/Layr-Labs/eigenda-proxy/store/precomputed_key/wvm/signer"
	wvmtypes "github.com/Layr-Labs/eigenda-proxy/store/precomputed_key/wvm/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	cache "github.com/patrickmn/go-cache"
)

type WVM interface {
	SendTransaction(ctx context.Context, to string, data []byte) (string, error)
}

// Store...wraps wvm client, ethclient and concurrent internal cache
type Store struct {
	wvmClient WVM
	log       log.Logger
	txCache   *cache.Cache
	cfg       *wvmtypes.Config
}

func NewStore(cfg *wvmtypes.Config, log log.Logger) (*Store, error) {
	store := &Store{cfg: cfg, log: log, txCache: cache.New(24*15*time.Hour, 24*time.Hour)}

	if cfg.Web3SignerEndpoint != "" {
		web3signer, err := signer.NewWeb3SignerClient(cfg, log)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize web3signer client: %w", err)
		}
		wvmClient, err := rpc.NewWvmRPCClient(log, cfg, web3signer)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize rpc client for wvm chain: %w", err)
		}
		store.wvmClient = wvmClient
		return store, nil
	}

	// Us PrivateKey signer
	privateKey := os.Getenv("WVM_PRIV_KEY")
	if privateKey == "" {
		return nil, fmt.Errorf("wvm archiver private key is empty and wvm web3 signer is empty")
	}
	privateKeySigner := signer.NewPrivateKeySigner(privateKey, log, cfg.ChainID)
	wvmClient, err := rpc.NewWvmRPCClient(log, cfg, privateKeySigner)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize rpc client for wvm chain: %w", err)
	}

	store.wvmClient = wvmClient

	return store, nil
}

func (wvm *Store) BackendType() common.BackendType {
	return common.WVMBackendType
}

func (wvm *Store) Verify(_ context.Context, key []byte, value []byte) error {
	h := crypto.Keccak256Hash(value)
	if !bytes.Equal(h[:], key) {
		return fmt.Errorf("key does not match value, expected: %s got: %s", hex.EncodeToString(key), h.Hex())
	}

	return nil
}

func (wvm *Store) Put(ctx context.Context, key []byte, value []byte) error {
	ctx, cancel := context.WithTimeout(ctx, wvm.cfg.Timeout)
	defer cancel()

	wvmTxHash, err := wvm.wvmClient.SendTransaction(ctx, wvmtypes.ArchivePoolAddress, value)
	if err != nil {
		return fmt.Errorf("failed to send wvm transaction: %w", err)
	}

	wvm.txCache.Set(string(key), wvmTxHash, cache.DefaultExpiration)

	wvm.log.Info("wvm backend: save wvm tx hash - batch_id:blob_index in internal storage",
		"tx hash", wvmTxHash, "provided key", string(key))

	return nil
}

func (wvm *Store) Get(ctx context.Context, key []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, wvm.cfg.Timeout)
	defer cancel()

	wvmTxHash, err := wvm.getWvmTxHashByCommitment(key)
	if err != nil {
		return nil, err
	}

	wvm.log.Info("wvm backend: found wvm tx hash using provided commitment key", "provided key", string(key))
	data, err := wvm.getFromGateway(ctx, wvmTxHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get eigenda blob from wvm: %w", err)
	}

	return data, nil
}

// GetWvmTxHashByCommitment uses commitment to get wvm tx hash from the internal map(temprorary hack)
// and returns it to the caller
func (wvm *Store) getWvmTxHashByCommitment(key []byte) (string, error) {
	wvmTxHash, found := wvm.txCache.Get(string(key))
	if !found {
		wvm.log.Info("wvm backend: tx hash using provided commitment NOT FOUND", "provided key", string(key))
		return "", fmt.Errorf("wvm backend: tx hash for provided commitment not found")
	}

	wvm.log.Info("wvm backned: tx hash using provided commitment FOUND", "provided key", string(key))

	return wvmTxHash.(string), nil
}

const wvmGatewayURL = "https://gateway.wvm.dev/calldata/%s"

// Modified get function with improved error handling
func (wvm *Store) getFromGateway(ctx context.Context, wvmTxHash string) ([]byte, error) {
	type WvmRetrieverResponse struct {
		ArweaveBlockHash   string `json:"arweave_block_hash"`
		Calldata           string `json:"calldata"`
		WarDecodedCalldata string `json:"war_decoded_calldata"`
		WvmBlockHash       string `json:"wvm_block_hash"`
	}

	r, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf(wvmGatewayURL,
			wvmTxHash), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	r.Header.Set("Accept", "application/json")
	client := &http.Client{
		Timeout: wvm.cfg.Timeout,
	}

	wvm.log.Info("sending request to WVM data retriever",
		"url", r.URL.String(),
		"headers", r.Header)

	resp, err := client.Do(r)
	if err != nil {
		return nil, fmt.Errorf("failed to call wvm-data-retriever: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if err := validateResponse(resp, body); err != nil {
		wvm.log.Error("invalid response from WVM data retriever",
			"status", resp.Status,
			"content_type", resp.Header.Get("Content-Type"),
			"body", string(body))
		return nil, fmt.Errorf("invalid response: %w", err)
	}

	var wvmData WvmRetrieverResponse
	if err = json.Unmarshal(body, &wvmData); err != nil {
		wvm.log.Error("failed to unmarshal response",
			"error", err,
			"body", string(body),
			"content_type", resp.Header.Get("Content-Type"))
		return nil, fmt.Errorf("failed to unmarshal response: %w, body: %s", err, string(body))
	}

	wvm.log.Info("wvm backend: get data from wvm",
		"arweave_block_hash", wvmData.ArweaveBlockHash,
		"wvm_block_hash", wvmData.WvmBlockHash,
		"calldata_length", len(wvmData.Calldata))

	calldataBlob, err := hexutil.Decode(wvmData.Calldata)
	if err != nil {
		return nil, fmt.Errorf("failed to decode calldata: %w", err)
	}

	if len(calldataBlob) == 0 {
		return nil, fmt.Errorf("decoded blob has length zero")
	}

	return calldataBlob, nil
}

func validateResponse(resp *http.Response, body []byte) error {
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		return fmt.Errorf("unexpected content type: %s, body: %s", contentType, string(body))
	}

	return nil
}
