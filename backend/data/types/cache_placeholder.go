// backend/data/types/cache_placeholder.go
package types

import (
	"time"
)

// MultiLevelCache implements multi-level caching
type MultiLevelCache struct{}

// NewMultiLevelCache creates a new multi-level cache
func NewMultiLevelCache() *MultiLevelCache {
	return &MultiLevelCache{}
}

func (m *MultiLevelCache) Get(key string) (any, error) {
	return nil, &CacheNotFoundError{}
}

func (m *MultiLevelCache) Set(key string, value any, ttl time.Duration) error {
	return nil
}

// CacheNotFoundError is returned when cache miss occurs
type CacheNotFoundError struct{}

func (e *CacheNotFoundError) Error() string {
	return "cache not found"
}
