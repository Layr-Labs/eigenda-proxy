package store

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/log"
)

type WVMClient struct {
	ethUrl string
	log    log.Logger
}

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

func NewWVMClient(url string, log log.Logger) *WVMClient {
	return &WVMClient{ethUrl: url, log: log}
}

func (wvm *WVMClient) Store(ctx context.Context, data []byte) error {
	_, err := wvmEncode(data)
	if err != nil {
		return fmt.Errorf("failed to store data in wvm: %w", err)
	}
	// check that the data is lower than 100kb

	return nil
}

func wvmEncode(data []byte) ([]byte, error) {

	return nil, nil
}
