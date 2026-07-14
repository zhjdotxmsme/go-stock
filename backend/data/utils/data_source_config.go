// backend/data/utils/data_source_config.go
package utils

import "time"

// DataSourceConfig manages primary and fallback data sources
type DataSourceConfig struct {
	Primary   Endpoint
	Fallbacks []Endpoint
	Strategy  FallbackStrategy
	Retry     RetryConfig
	Cache     CacheConfig
}

// FallbackStrategy defines how to choose between data sources
type FallbackStrategy string

const (
	FailoverStrategy   FallbackStrategy = "FAILOVER"    // Primary fails → use first fallback
	RoundRobinStrategy FallbackStrategy = "ROUND_ROBIN" // Rotate through sources
	RandomStrategy     FallbackStrategy = "RANDOM"      // Random selection
)

// RetryConfig defines retry behavior
type RetryConfig struct {
	MaxAttempts int
	Backoff     BackoffStrategy
	ShouldRetry func(error) bool
}

// BackoffStrategy defines delay between retries
type BackoffStrategy interface {
	Next(attempt int) time.Duration
}

// CacheConfig defines caching behavior
type CacheConfig struct {
	Enabled        bool
	TTL            time.Duration
	MaxSize        int64
	EvictionPolicy string // "LRU" | "LFU" | "FIFO"
}
