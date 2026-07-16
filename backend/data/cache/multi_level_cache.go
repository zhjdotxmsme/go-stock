// backend/data/cache/multi_level_cache.go
package cache

import (
	"context"
	"time"
)

// MultiLevelCache implements multi-level caching strategy
type MultiLevelCache struct {
	l1 *MemoryCache
	l2 *RedisCache
	l3 *DatabaseCache
}

// NewMultiLevelCache creates a new multi-level cache
func NewMultiLevelCache() *MultiLevelCache {
	return &MultiLevelCache{
		l1: NewMemoryCache(100*1024*1024, time.Minute*5), // 100MB, 5min TTL
		l2: NewRedisCache("localhost:6379", time.Hour),   // 1 hour TTL
		l3: NewDatabaseCache(time.Hour * 24),            // 24 hour TTL
	}
}

func (m *MultiLevelCache) Get(key string) (any, error) {
	// L1: Memory cache (fastest)
	if value, err := m.l1.Get(key); err == nil {
		return value, nil
	}

	// L2: Redis cache (medium speed)
	if value, err := m.l2.Get(context.Background(), key); err == nil {
		// Backfill L1
		m.l1.Set(key, value, 0)
		return value, nil
	}

	// L3: Database cache (slowest)
	if value, err := m.l3.Get(context.Background(), key); err == nil {
		// Backfill L2 and L1
		m.l2.Set(context.Background(), key, value, 0)
		m.l1.Set(key, value, 0)
		return value, nil
	}

	return nil, &CacheNotFoundError{}
}

func (m *MultiLevelCache) Set(key string, value any, ttl time.Duration) error {
	// Set in all levels
	m.l1.Set(key, value, ttl)
	m.l2.Set(context.Background(), key, value, ttl)
	m.l3.Set(context.Background(), key, value, ttl)

	return nil
}

func (m *MultiLevelCache) Delete(key string) error {
	// Delete from all levels
	m.l1.Delete(key)
	m.l2.Delete(context.Background(), key)
	m.l3.Delete(context.Background(), key)

	return nil
}

func (m *MultiLevelCache) Clear() error {
	m.l1.Clear()
	m.l2.Clear(context.Background())
	m.l3.Clear(context.Background())

	return nil
}

// GetLevel accesses specific cache level (for debugging)
func (m *MultiLevelCache) GetLevel(level int, key string) (any, error) {
	switch level {
	case 1:
		return m.l1.Get(key)
	case 2:
		return m.l2.Get(context.Background(), key)
	case 3:
		return m.l3.Get(context.Background(), key)
	default:
		return nil, &CacheNotFoundError{}
	}
}

// WarmUp preloads cache with hot keys
func (m *MultiLevelCache) WarmUp(ctx context.Context, keys []string, dataProvider func(string) (any, error)) error {
	for _, key := range keys {
		value, err := dataProvider(key)
		if err != nil {
			continue // Skip failed data fetch
		}

		m.Set(key, value, 0)
	}

	return nil
}