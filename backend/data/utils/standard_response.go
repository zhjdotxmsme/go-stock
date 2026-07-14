// backend/data/utils/standard_response.go
package utils

import "time"

// StandardizedResponse is the unified response format for all data layers
type StandardizedResponse struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    interface{}    `json:"data"`
	Meta    ResponseMeta   `json:"meta"`
	Error   *ErrorResponse `json:"error,omitempty"`
}

// ResponseMeta contains metadata about the response
type ResponseMeta struct {
	Source             string    `json:"source"`
	FallbackUsed       bool      `json:"fallback_used"`
	Latency            int64     `json:"latency_ms"`
	Cached             bool      `json:"cached"`
	Timestamp          time.Time `json:"timestamp"`
	Version            string    `json:"api_version"`
	RateLimitRemaining int       `json:"rate_limit_remaining"`
}

// ErrorResponse contains error details
type ErrorResponse struct {
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Details   string    `json:"details,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	TraceID   string    `json:"trace_id"`
}
