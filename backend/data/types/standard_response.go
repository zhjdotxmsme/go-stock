// backend/data/types/standard_response.go
package types

import "time"

// StandardizedResponse is the unified response format for all data layers.
type StandardizedResponse struct {
	// Code is the operation result code (0 typically means success).
	Code int `json:"code"`
	// Message is a human-readable description of the result.
	Message string `json:"message"`
	// Data holds the actual response payload.
	Data any `json:"data"`
	// Meta contains response metadata.
	Meta ResponseMeta `json:"meta"`
	// Error provides error details when the operation fails.
	Error *ErrorResponse `json:"error,omitempty"`
}

// ResponseMeta contains metadata about the response.
type ResponseMeta struct {
	// Source identifies the data layer that produced the response.
	Source string `json:"source"`
	// FallbackUsed indicates whether a fallback endpoint was used.
	FallbackUsed bool `json:"fallback_used"`
	// Latency is the request duration in milliseconds.
	Latency int64 `json:"latency_ms"`
	// Cached indicates whether the response came from cache.
	Cached bool `json:"cached"`
	// Timestamp is when the response was generated.
	Timestamp time.Time `json:"timestamp"`
	// Version is the API version of the source.
	Version string `json:"api_version"`
	// RateLimitRemaining is the number of requests remaining in the current quota.
	RateLimitRemaining int `json:"rate_limit_remaining"`
}

// ErrorResponse contains error details.
type ErrorResponse struct {
	// Type is the error category.
	Type string `json:"type"`
	// Message is a human-readable error description.
	Message string `json:"message"`
	// Details provides additional error context.
	Details string `json:"details,omitempty"`
	// Timestamp is when the error occurred.
	Timestamp time.Time `json:"timestamp"`
	// TraceID is a unique identifier for tracing the error.
	TraceID string `json:"trace_id"`
}
