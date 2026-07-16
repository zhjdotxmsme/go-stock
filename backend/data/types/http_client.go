// backend/data/types/http_client.go
package types

import (
	"context"
	"net/http"
	"time"
)

// HTTPClient provides HTTP request functionality
type HTTPClient struct {
	client  *http.Client
	timeout time.Duration
}

// NewHTTPClient creates a new HTTP client
func NewHTTPClient() *HTTPClient {
	return &HTTPClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		timeout: 30 * time.Second,
	}
}

func (h *HTTPClient) Get(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
	return []byte("mock response"), nil
}
