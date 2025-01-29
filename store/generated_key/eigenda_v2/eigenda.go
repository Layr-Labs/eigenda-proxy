package eigenda_v2

import (
	"context"
	"fmt"
	"time"

	"github.com/Layr-Labs/eigenda-proxy/common"

	eigenda_common "github.com/Layr-Labs/eigenda/common"
	eth_utils "github.com/Layr-Labs/eigenda/core/eth"
	"github.com/ethereum/go-ethereum/log"
)

type V2StoreConfig struct {
	// address of service manager - used for loading chain state
	ServiceManagerAddr string

	MaxBlobSizeBytes uint64

	// total duration time that client waits for blob to confirm
	StatusQueryTimeout time.Duration

	// number of times to retry eigenda blob dispersals
	PutRetries uint
}

// Store does storage interactions and verifications for blobs with the
// EigenDA V2 protocol.
type Store struct {
	// TODO: disperserClient, retrieverClient usage
	cfg      *V2StoreConfig
	log      log.Logger

	// id --> public endpoint
	relays map[uint32]string
}

var _ common.GeneratedKeyStore = (*Store)(nil)

func NewStore(log log.Logger, cfg *V2StoreConfig, ethClient eigenda_common.EthClient) (*Store, error) {
	// create relay mapping
	// TODO: remove nil in favor of real logging - this is insecure rn
	reader, err := eth_utils.NewReader(nil, ethClient, "0x0", cfg.ServiceManagerAddr)
	if err != nil {
		return nil, err
	}

	relays, err := reader.GetRelayURLs(context.Background())
	if err != nil {
		return nil, err
	}

	
	return &Store{
		log:      log,
		cfg:      cfg,
		relays: relays,

	}, nil
}

// Get fetches a blob from DA using certificate fields and verifies blob
// against commitment to ensure data is valid and non-tampered.
func (e Store) Get(ctx context.Context, key []byte) ([]byte, error) {
	return nil, fmt.Errorf("unimplemented")
}

// Put disperses a blob for some pre-image and returns the associated RLP encoded certificate commit.
// TODO: Client polling for different status codes
//
//	Mapping status codes to 503 failover
func (e Store) Put(ctx context.Context, value []byte) ([]byte, error) {
	return nil, fmt.Errorf("unimplemented")
}

// Backend returns the backend type for EigenDA Store
func (e Store) BackendType() common.BackendType {
	return common.EigenDAV2BackendType
}

// Key is used to recover certificate fields and that verifies blob
// against commitment to ensure data is valid and non-tampered.
// TODO: develop segmented routes for read/write;
// commitment generation should only happen again when reading and not writing
func (e Store) Verify(ctx context.Context, key []byte, value []byte) error {
	return fmt.Errorf("unimplemented")
}
