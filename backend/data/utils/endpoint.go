// backend/data/utils/endpoint.go
package utils

import (
	"time"
)

// Endpoint represents a single data source endpoint
type Endpoint struct {
	Name      string
	URL       string
	Method    string
	Headers   map[string]string
	Timeout   time.Duration
	RateLimit RateLimitConfig
	Parser    DataParser
}

// RateLimitConfig defines rate limiting for endpoints
type RateLimitConfig struct {
	RequestsPerSecond int
	BurstSize         int
}

// DataParser defines the interface for parsing responses
type DataParser interface {
	Parse(data []byte) (interface{}, error)
}
