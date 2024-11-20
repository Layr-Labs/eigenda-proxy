package types

import (
	"time"
)

const (
	ArchivePoolAddress = "0x0000000000000000000000000000000000000000" // the data settling address, a unified standard across WeaveVM archiving services
)

// Config...WeaveVM client configuration
type Config struct {
	Enabled bool
	// RPC endpoint of WeaveVM chain
	Endpoint string
	// WeaveVM chain id
	ChainID int64
	// Timeout on WeaveVM calls in seconds
	Timeout time.Duration

	// Web3Signer configuration
	Web3SignerEndpoint      string
	Web3SignerTLSCertFile   string
	Web3SignerTLSKeyFile    string
	Web3SignerTLSCACertFile string
}

type RetrieverResponse struct {
	ArweaveBlockHash   string `json:"arweave_block_hash"`
	Calldata           string `json:"calldata"`
	WarDecodedCalldata string `json:"war_decoded_calldata"`
	WvmBlockHash       string `json:"weaveVM_block_hash"`
}
