package store

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Layr-Labs/eigenda-proxy/verify"
	"github.com/Layr-Labs/eigenda/api/clients"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/patrickmn/go-cache"
)

type EigenDAStoreConfig struct {
	MaxBlobSizeBytes uint64
	// the # of Ethereum blocks to wait after the EigenDA L1BlockReference # before attempting to verify
	// & accredit a blob
	EthConfirmationDepth uint64

	// total duration time that client waits for blob to confirm
	StatusQueryTimeout time.Duration
}

// EigenDAStore does storage interactions and verifications for blobs with DA.
type EigenDAStore struct {
	client    *clients.EigenDAClient
	verifier  *verify.Verifier
	cfg       *EigenDAStoreConfig
	log       log.Logger
	wvmClient *WVMClient
	wvmCache  *cache.Cache
}

var _ KeyGeneratedStore = (*EigenDAStore)(nil)

func NewEigenDAStore(client *clients.EigenDAClient,
	v *verify.Verifier, log log.Logger, cfg *EigenDAStoreConfig, wvmClient *WVMClient) (*EigenDAStore, error) {
	return &EigenDAStore{
		client:    client,
		verifier:  v,
		log:       log,
		cfg:       cfg,
		wvmClient: wvmClient,
		wvmCache:  cache.New(24*15*time.Hour, 24*time.Hour),
	}, nil
}

// Get fetches a blob from DA using certificate fields and verifies blob
// against commitment to ensure data is valid and non-tampered.
func (e EigenDAStore) Get(ctx context.Context, key []byte) ([]byte, error) {
	var cert verify.Certificate
	err := rlp.DecodeBytes(key, &cert)
	if err != nil {
		return nil, fmt.Errorf("failed to decode DA cert to RLP format: %w", err)
	}

	decodedBlob, err := e.client.GetBlob(ctx, cert.BlobVerificationProof.BatchMetadata.BatchHeaderHash, cert.BlobVerificationProof.BlobIndex)
	if err != nil {
		return nil, fmt.Errorf("EigenDA client failed to retrieve decoded blob: %w", err)
	}

	return decodedBlob, nil
}

// GetWvmTxHashByCommitment uses commitment to get wvm tx hash from the internal map(temprorary hack)
// and returns it to the caller
func (e EigenDAStore) GetWvmTxHashByCommitment(ctx context.Context, key []byte) (string, error) {
	e.log.Info("trying to get wvm tx hash using provided commitment")
	var cert verify.Certificate
	err := rlp.DecodeBytes(key, &cert)
	if err != nil {
		return "", fmt.Errorf("failed to decode DA cert to RLP format: %w", err)
	}

	wvmTxHash, found := e.wvmCache.Get(commitmentKey(cert.BlobVerificationProof.BatchId, cert.BlobVerificationProof.BlobIndex))
	if !found {
		e.log.Info("wvm tx hash using provided commitment NOT FOUND", "provided key", commitmentKey(cert.BlobVerificationProof.BatchId, cert.BlobVerificationProof.BlobIndex))
		return "", fmt.Errorf("wvm tx hash for provided commitment not found")
	}

	e.log.Info("wvm tx hash using provided commitment FOUND", "provided key", commitmentKey(cert.BlobVerificationProof.BatchId, cert.BlobVerificationProof.BlobIndex))

	return wvmTxHash.(string), nil
}

func commitmentKey(batchID, blobIndex uint32) string {
	return fmt.Sprintf("%d:%d", batchID, blobIndex)
}

func (e EigenDAStore) GetBlobFromWvm(ctx context.Context, key []byte) ([]byte, error) {
	var cert verify.Certificate
	err := rlp.DecodeBytes(key, &cert)
	if err != nil {
		return nil, fmt.Errorf("failed to decode DA cert to RLP format: %w", err)
	}

	wvmTxHash, found := e.wvmCache.Get(commitmentKey(cert.BlobVerificationProof.BatchId, cert.BlobVerificationProof.BlobIndex))
	if !found {
		e.log.Info("wvm tx hash using provided commitment NOT FOUND", "provided key", commitmentKey(cert.BlobVerificationProof.BatchId, cert.BlobVerificationProof.BlobIndex))
		return nil, fmt.Errorf("wvm tx hash for provided commitment not found")
	}

	e.log.Info("wvm tx hash using provided commitment FOUND", "provided key", commitmentKey(cert.BlobVerificationProof.BatchId, cert.BlobVerificationProof.BlobIndex))

	r, err := http.Get(fmt.Sprintf("https://wvm-data-retriever.shuttleapp.rs/calldata/%s", wvmTxHash))
	if err != nil {
		return nil, fmt.Errorf("failed to call wvm-data-retriever")
	}

	type WvmResponse struct {
		ArweaveBlockHash   string `json:"arweave_block_hash"`
		Calldata           string `json:"calldata"`
		WarDecodedCalldata string `json:"war_decoded_calldata"`
		WvmBlockHash       string `json:"wvm_block_hash"`
	}

	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var wvmData WvmResponse
	err = json.Unmarshal(body, &wvmData)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	e.log.Info("Received data from WVM", "arweave_block_hash", wvmData.ArweaveBlockHash, "wvm_block_hash", wvmData.WvmBlockHash)

	b, err := hex.DecodeString(wvmData.Calldata[2:])
	if err != nil {
		return nil, err
	}

	wvmDecodedBlob, err := e.wvmClient.WvmDecode(b)
	if err != nil {
		return nil, fmt.Errorf("failed to decode calldata to eigen decoded blob: %w", err)
	}

	if len(wvmDecodedBlob) == 0 {
		return nil, fmt.Errorf("blob has length zero")
	}

	decodedData, err := e.client.Codec.DecodeBlob(wvmDecodedBlob)
	if err != nil {
		return nil, fmt.Errorf("error decoding eigen blob: %w", err)
	}

	return decodedData, nil
}

// Put disperses a blob for some pre-image and returns the associated RLP encoded certificate commit.
func (e EigenDAStore) Put(ctx context.Context, value []byte) ([]byte, error) {
	encodedBlob, err := e.client.GetCodec().EncodeBlob(value)
	if err != nil {
		return nil, fmt.Errorf("EigenDA client failed to re-encode blob: %w", err)
	}
	// WVM: check that the data is lower than 100kb - Set it in configs via proxy config
	if uint64(len(encodedBlob)) > e.cfg.MaxBlobSizeBytes {
		return nil, fmt.Errorf("encoded blob is larger than max blob size: blob length %d, max blob size %d", len(value), e.cfg.MaxBlobSizeBytes)
	}

	dispersalStart := time.Now()
	blobInfo, err := e.client.PutBlob(ctx, value)
	if err != nil {
		return nil, err
	}
	cert := (*verify.Certificate)(blobInfo)

	err = e.verifier.VerifyCommitment(cert.BlobHeader.Commitment, encodedBlob)
	if err != nil {
		return nil, err
	}

	dispersalDuration := time.Since(dispersalStart)
	remainingTimeout := e.cfg.StatusQueryTimeout - dispersalDuration

	ticker := time.NewTicker(12 * time.Second) // avg. eth block time
	defer ticker.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), remainingTimeout)
	defer cancel()

	done := false
	for !done {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timed out when trying to verify the DA certificate for a blob batch after dispersal")
		case <-ticker.C:
			err = e.verifier.VerifyCert(cert)
			switch {
			case err == nil:
				done = true
			case errors.Is(err, verify.ErrBatchMetadataHashNotFound):
				e.log.Info("Blob confirmed, waiting for sufficient confirmation depth...", "targetDepth", e.cfg.EthConfirmationDepth)
			default:
				return nil, err
			}
		}
	}

	// WVM
	// TO-DO: here we store the blob in wvm!!!!
	e.log.Info("WVM: save BLOB in wvm chain", "batch id", blobInfo.BlobVerificationProof.BatchId, "blob index", blobInfo.BlobVerificationProof.BlobIndex)
	wvmTxHash, err := e.wvmClient.Store(ctx, encodedBlob)
	if err != nil {
		return nil, err
	}

	e.log.Info("WVM:TX Hash:", "tx hash", wvmTxHash)

	// store wvm txid and blob ??header?? to the key-value storage
	e.wvmCache.Set(commitmentKey(cert.BlobVerificationProof.BatchId, cert.BlobVerificationProof.BlobIndex), wvmTxHash, cache.DefaultExpiration)
	//
	e.log.Info("WVM:saved wvm Tx Hash by batch_id:blob_index",
		"tx hash", wvmTxHash,
		"key", commitmentKey(cert.BlobVerificationProof.BatchId, cert.BlobVerificationProof.BlobIndex))

	bytes, err := rlp.EncodeToBytes(cert)
	if err != nil {
		return nil, fmt.Errorf("failed to encode DA cert to RLP format: %w", err)
	}

	return bytes, nil
}

// Entries are a no-op for EigenDA Store
func (e EigenDAStore) Stats() *Stats {
	return nil
}

// Backend returns the backend type for EigenDA Store
func (e EigenDAStore) BackendType() BackendType {
	return EigenDA
}

// Key is used to recover certificate fields and that verifies blob
// against commitment to ensure data is valid and non-tampered.
func (e EigenDAStore) Verify(key []byte, value []byte) error {
	var cert verify.Certificate
	err := rlp.DecodeBytes(key, &cert)
	if err != nil {
		return fmt.Errorf("failed to decode DA cert to RLP format: %w", err)
	}

	// re-encode blob for verification
	encodedBlob, err := e.client.GetCodec().EncodeBlob(value)
	if err != nil {
		return fmt.Errorf("EigenDA client failed to re-encode blob: %w", err)
	}

	// verify kzg data commitment
	err = e.verifier.VerifyCommitment(cert.BlobHeader.Commitment, encodedBlob)
	if err != nil {
		return err
	}

	// verify DA certificate against on-chain
	return e.verifier.VerifyCert(&cert)
}
