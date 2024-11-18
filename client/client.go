package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
)

// TODO: Add support for custom http client option
type Config struct {
	URL string
}

// SimpleCommitmentClient implements a simple client for the eigenda-proxy
// that can put/get simple commitment data and query the health endpoint.
// It is meant to be used by arbitrum nitro integrations.
// Optimism has its own client: https://github.com/ethereum-optimism/optimism/blob/develop/op-alt-da/daclient.go
// so clients wanting to send op commitment mode data should use that client.
type SimpleCommitmentClient struct {
	cfg        *Config
	httpClient *http.Client
}

func New(cfg *Config) *SimpleCommitmentClient {
	return &SimpleCommitmentClient{
		cfg,
		http.DefaultClient,
	}
}

// Health indicates if the server is operational; useful for event based awaits
// when integration testing
func (c *SimpleCommitmentClient) Health() error {
	url := c.cfg.URL + "/health"
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received bad status code: %d", resp.StatusCode)
	}

	return nil
}

// GetData fetches blob data associated with a DA certificate
func (c *SimpleCommitmentClient) GetData(ctx context.Context, comm []byte) ([]byte, error) {
	url := fmt.Sprintf("%s/get/0x%x?commitment_mode=simple", c.cfg.URL, comm)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to construct http request: %w", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received error response, code=%d, msg = %s", resp.StatusCode, string(b))
	}

	return b, nil
}

// SetData writes raw byte data to DA and returns the respective certificate
func (c *SimpleCommitmentClient) SetData(ctx context.Context, b []byte) ([]byte, error) {
	url := fmt.Sprintf("%s/put?commitment_mode=simple", c.cfg.URL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	b, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to store data: %v, err = %s", resp.StatusCode, string(b))
	}

	if len(b) == 0 {
		return nil, fmt.Errorf("read certificate is empty")
	}

	return b, err
}
