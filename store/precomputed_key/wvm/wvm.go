package weaveVM

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
	rpc "github.com/Layr-Labs/eigenda-proxy/store/precomputed_key/weaveVM/rpc"
	signer "github.com/Layr-Labs/eigenda-proxy/store/precomputed_key/weaveVM/signer"
	weaveVMtypes "github.com/Layr-Labs/eigenda-proxy/store/precomputed_key/weaveVM/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	cache "github.com/patrickmn/go-cache"
)

type WeaveVM interface {
	SendTransaction(ctx context.Context, to string, data []byte) (string, error)
}

// Store...wraps weaveVM client, ethclient and concurrent internal cache
type Store struct {
	weaveVMClient WeaveVM
	log           log.Logger
	txCache       *cache.Cache
	cfg           *weaveVMtypes.Config
}

func NewStore(cfg *weaveVMtypes.Config, log log.Logger) (*Store, error) {
	store := &Store{cfg: cfg, log: log, txCache: cache.New(24*15*time.Hour, 24*time.Hour)}

	if cfg.Web3SignerEndpoint != "" {
		web3signer, err := signer.NewWeb3SignerClient(cfg, log)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize web3signer client: %w", err)
		}
		weaveVMClient, err := rpc.NewWvmRPCClient(log, cfg, web3signer)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize rpc client for weaveVM chain: %w", err)
		}
		store.weaveVMClient = weaveVMClient
		return store, nil
	}

	// Us PrivateKey signer
	privateKey := os.Getenv("WeaveVM_PRIV_KEY")
	if privateKey == "" {
		return nil, fmt.Errorf("weaveVM archiver private key is empty and weaveVM web3 signer is empty")
	}
	privateKeySigner := signer.NewPrivateKeySigner(privateKey, log, cfg.ChainID)
	weaveVMClient, err := rpc.NewWvmRPCClient(log, cfg, privateKeySigner)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize rpc client for weaveVM chain: %w", err)
	}

	store.weaveVMClient = weaveVMClient

	return store, nil
}

func (weaveVM *Store) BackendType() common.BackendType {
	return common.WeaveVMBackendType
}

func (weaveVM *Store) Verify(_ context.Context, key []byte, value []byte) error {
	h := crypto.Keccak256Hash(value)
	if !bytes.Equal(h[:], key) {
		return fmt.Errorf("key does not match value, expected: %s got: %s", hex.EncodeToString(key), h.Hex())
	}

	return nil
}

func (weaveVM *Store) Put(ctx context.Context, key []byte, value []byte) error {
	ctx, cancel := context.WithTimeout(ctx, weaveVM.cfg.Timeout)
	defer cancel()

	weaveVMTxHash, err := weaveVM.weaveVMClient.SendTransaction(ctx, weaveVMtypes.ArchivePoolAddress, value)
	if err != nil {
		return fmt.Errorf("failed to send weaveVM transaction: %w", err)
	}

	weaveVM.txCache.Set(string(key), weaveVMTxHash, cache.DefaultExpiration)

	weaveVM.log.Info("weaveVM backend: save weaveVM tx hash - batch_id:blob_index in internal storage",
		"tx hash", weaveVMTxHash, "provided key", string(key))

	return nil
}

func (weaveVM *Store) Get(ctx context.Context, key []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, weaveVM.cfg.Timeout)
	defer cancel()

	weaveVMTxHash, err := weaveVM.getWvmTxHashByCommitment(key)
	if err != nil {
		return nil, err
	}

	weaveVM.log.Info("weaveVM backend: found weaveVM tx hash using provided commitment key", "provided key", string(key))
	data, err := weaveVM.getFromGateway(ctx, weaveVMTxHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get eigenda blob from weaveVM: %w", err)
	}

	return data, nil
}

// GetWvmTxHashByCommitment uses commitment to get weaveVM tx hash from the internal map(temprorary hack)
// and returns it to the caller
func (weaveVM *Store) getWvmTxHashByCommitment(key []byte) (string, error) {
	weaveVMTxHash, found := weaveVM.txCache.Get(string(key))
	if !found {
		weaveVM.log.Info("weaveVM backend: tx hash using provided commitment NOT FOUND", "provided key", string(key))
		return "", fmt.Errorf("weaveVM backend: tx hash for provided commitment not found")
	}

	weaveVM.log.Info("weaveVM backned: tx hash using provided commitment FOUND", "provided key", string(key))

	return weaveVMTxHash.(string), nil
}

const weaveVMGatewayURL = "https://gateway.weaveVM.dev/calldata/%s"

// Modified get function with improved error handling
func (weaveVM *Store) getFromGateway(ctx context.Context, weaveVMTxHash string) ([]byte, error) {
	type WvmRetrieverResponse struct {
		ArweaveBlockHash   string `json:"arweave_block_hash"`
		Calldata           string `json:"calldata"`
		WarDecodedCalldata string `json:"war_decoded_calldata"`
		WvmBlockHash       string `json:"weaveVM_block_hash"`
	}

	r, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf(weaveVMGatewayURL,
			weaveVMTxHash), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	r.Header.Set("Accept", "application/json")
	client := &http.Client{
		Timeout: weaveVM.cfg.Timeout,
	}

	weaveVM.log.Info("sending request to WeaveVM data retriever",
		"url", r.URL.String(),
		"headers", r.Header)

	resp, err := client.Do(r)
	if err != nil {
		return nil, fmt.Errorf("failed to call weaveVM-data-retriever: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if err := validateResponse(resp, body); err != nil {
		weaveVM.log.Error("invalid response from WeaveVM data retriever",
			"status", resp.Status,
			"content_type", resp.Header.Get("Content-Type"),
			"body", string(body))
		return nil, fmt.Errorf("invalid response: %w", err)
	}

	var weaveVMData WvmRetrieverResponse
	if err = json.Unmarshal(body, &weaveVMData); err != nil {
		weaveVM.log.Error("failed to unmarshal response",
			"error", err,
			"body", string(body),
			"content_type", resp.Header.Get("Content-Type"))
		return nil, fmt.Errorf("failed to unmarshal response: %w, body: %s", err, string(body))
	}

	weaveVM.log.Info("weaveVM backend: get data from weaveVM",
		"arweave_block_hash", weaveVMData.ArweaveBlockHash,
		"weaveVM_block_hash", weaveVMData.WvmBlockHash,
		"calldata_length", len(weaveVMData.Calldata))

	calldataBlob, err := hexutil.Decode(weaveVMData.Calldata)
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
