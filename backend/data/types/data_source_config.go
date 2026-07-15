// backend/data/types/data_source_config.go
package types

import "time"

// DataSourceConfig manages primary and fallback data sources.
type DataSourceConfig struct {
	// Primary is the main endpoint used for data fetching.
	Primary Endpoint
	// Fallbacks are alternative endpoints used when primary fails.
	Fallbacks []Endpoint
	// Strategy determines how primary and fallback endpoints are selected.
	Strategy FallbackStrategy
	// Retry controls retry behavior for failed requests.
	Retry RetryConfig
	// Cache controls response caching behavior.
	Cache CacheConfig
}

// FallbackStrategy defines how to choose between data sources.
type FallbackStrategy string

const (
	// FailoverStrategy uses fallbacks only when the primary endpoint fails.
	FailoverStrategy FallbackStrategy = "FAILOVER"
	// RoundRobinStrategy rotates through all available endpoints.
	RoundRobinStrategy FallbackStrategy = "ROUND_ROBIN"
	// RandomStrategy selects endpoints at random.
	RandomStrategy FallbackStrategy = "RANDOM"
)

// RetryConfig defines retry behavior.
type RetryConfig struct {
	// MaxAttempts is the maximum number of request attempts.
	MaxAttempts int
	// Backoff determines the delay between retry attempts.
	Backoff BackoffStrategy
	// ShouldRetry evaluates whether a failed request should be retried.
	ShouldRetry func(error) bool
}

// BackoffStrategy defines delay between retries.
type BackoffStrategy interface {
	// Next returns the delay duration for the given attempt number.
	Next(attempt int) time.Duration
}

// CacheConfig defines caching behavior.
type CacheConfig struct {
	// Enabled toggles response caching.
	Enabled bool
	// TTL is the duration after which cached entries expire.
	TTL time.Duration
	// MaxSize is the maximum size of the cache in bytes.
	MaxSize int64
	// EvictionPolicy selects the cache eviction algorithm.
	EvictionPolicy string // "LRU" | "LFU" | "FIFO"
}
