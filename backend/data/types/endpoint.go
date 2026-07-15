// backend/data/types/endpoint.go
package types

import "time"

// HTTPMethod represents an HTTP request method.
type HTTPMethod string

const (
	// MethodGet is the HTTP GET method.
	MethodGet HTTPMethod = "GET"
	// MethodPost is the HTTP POST method.
	MethodPost HTTPMethod = "POST"
	// MethodPut is the HTTP PUT method.
	MethodPut HTTPMethod = "PUT"
	// MethodDelete is the HTTP DELETE method.
	MethodDelete HTTPMethod = "DELETE"
	// MethodPatch is the HTTP PATCH method.
	MethodPatch HTTPMethod = "PATCH"
)

// Endpoint represents a single data source endpoint.
type Endpoint struct {
	// Name is the unique identifier for this endpoint.
	Name string
	// URL is the endpoint address.
	URL string
	// Method is the HTTP method used for requests.
	Method HTTPMethod
	// Headers are optional HTTP headers to include with requests.
	Headers map[string]string
	// Timeout is the maximum duration to wait for a response.
	Timeout time.Duration
	// RateLimit configures request throttling for this endpoint.
	RateLimit RateLimitConfig
	// Parser converts raw response data into a typed value.
	Parser DataParser
}

// RateLimitConfig defines rate limiting for endpoints.
type RateLimitConfig struct {
	// RequestsPerSecond is the sustained rate limit.
	RequestsPerSecond int
	// BurstSize is the maximum number of requests allowed in a single burst.
	BurstSize int
}

// DataParser defines the interface for parsing responses.
type DataParser interface {
	// Parse converts raw response bytes into a typed value.
	Parse(data []byte) (any, error)
}
