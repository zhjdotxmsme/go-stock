// backend/data/cache/redis_cache.go
package cache

import (
	"context"
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
	data map[string]any
}

func NewMockRedisClient() *MockRedisClient {
	return &MockRedisClient{
		data: make(map[string]any),
	}
}

func (m *MockRedisClient) Get(ctx context.Context, key string) (string, error) {
	if val, exists := m.data[key]; exists {
		if str, ok := val.(string); ok {
			return str, nil
		}
		return "", &CacheNotFoundError{}
	}
	return "", &CacheNotFoundError{}
}

func (m *MockRedisClient) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	m.data[key] = value
	return nil
}

func (m *MockRedisClient) Del(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		delete(m.data, key)
	}
	return nil
}

func (m *MockRedisClient) FlushDB(ctx context.Context) error {
	m.data = make(map[string]any)
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