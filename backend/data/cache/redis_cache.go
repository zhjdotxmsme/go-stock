// backend/data/cache/redis_cache.go
package cache

import (
	"context"
	"sync"
	"time"
)

// RedisCache implements L2 Redis cache
type RedisCache struct {
	client RedisClient
	ttl    time.Duration
}

// RedisClient defines the interface for Redis operations
type RedisClient interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value any, expiration time.Duration) error
	Del(ctx context.Context, keys ...string) error
	FlushDB(ctx context.Context) error
}

// MockRedisClient provides a mock implementation for testing
type MockRedisClient struct {
	mu   sync.RWMutex
	data map[string]mockEntry
}

func NewMockRedisClient() *MockRedisClient {
	return &MockRedisClient{
		data: make(map[string]mockEntry),
	}
}

// mockEntry 记录写入时间与过期时间，让 Mock 语义与真实 Redis 一致：
// 旧行为忽略 TTL，导致 L1 过期后 L2 仍返回陈旧值（MultiLevelCache_Expiration 场景）。
type mockEntry struct {
	value     any
	expiresAt time.Time
}

func (m *MockRedisClient) Get(ctx context.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if e, exists := m.data[key]; exists {
		if time.Now().After(e.expiresAt) {
			return "", &CacheNotFoundError{}
		}
		if str, ok := e.value.(string); ok {
			return str, nil
		}
		return "", &CacheNotFoundError{}
	}
	return "", &CacheNotFoundError{}
}

func (m *MockRedisClient) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = mockEntry{value: value, expiresAt: time.Now().Add(expiration)}
	return nil
}

func (m *MockRedisClient) Del(ctx context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, key := range keys {
		delete(m.data, key)
	}
	return nil
}

func (m *MockRedisClient) FlushDB(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string]mockEntry)
	return nil
}

// NewRedisCache creates a new Redis cache
func NewRedisCache(addr string, ttl time.Duration) *RedisCache {
	return &RedisCache{
		client: NewMockRedisClient(),
		ttl:    ttl,
	}
}

func (r *RedisCache) Get(ctx context.Context, key string) (any, error) {
	val, err := r.client.Get(ctx, key)
	if err != nil {
		return nil, &CacheNotFoundError{}
	}
	return val, nil
}

func (r *RedisCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	expiration := ttl
	if ttl == 0 {
		expiration = r.ttl
	}

	return r.client.Set(ctx, key, value, expiration)
}

func (r *RedisCache) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key)
}

func (r *RedisCache) Clear(ctx context.Context) error {
	return r.client.FlushDB(ctx)
}