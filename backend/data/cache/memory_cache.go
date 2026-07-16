// backend/data/cache/memory_cache.go
package cache

import (
	"sync"
	"time"
)

// MemoryCache implements L1 memory cache
type MemoryCache struct {
	items      map[string]*cacheItem
	mu         sync.RWMutex
	maxSize    int64
	currentSize int64
	ttl        time.Duration
}

type cacheItem struct {
	value      any
	expiration time.Time
	size       int64
}

// NewMemoryCache creates a new memory cache
func NewMemoryCache(maxSize int64, ttl time.Duration) *MemoryCache {
	return &MemoryCache{
		items:   make(map[string]*cacheItem),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

func (m *MemoryCache) Get(key string) (any, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, exists := m.items[key]
	if !exists {
		return nil, &CacheNotFoundError{}
	}

	if time.Now().After(item.expiration) {
		return nil, &CacheNotFoundError{}
	}

	return item.value, nil
}

func (m *MemoryCache) Set(key string, value any, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if key already exists and update size
	if oldItem, exists := m.items[key]; exists {
		m.currentSize -= oldItem.size
	}

	// Calculate item size (rough estimation)
	size := int64(len(key) + 32)

	// Check if cache is full
	if m.currentSize+size > m.maxSize {
		m.evictLRU()
	}

	// Set expiration
	expiration := time.Now().Add(ttl)
	if ttl == 0 {
		expiration = time.Now().Add(m.ttl)
	}

	m.items[key] = &cacheItem{
		value:      value,
		expiration: expiration,
		size:       size,
	}
	m.currentSize += size

	return nil
}

func (m *MemoryCache) evictLRU() {
	// Simple LRU eviction: remove oldest item
	var oldestKey string
	var oldestTime time.Time

	for key, item := range m.items {
		if oldestKey == "" || item.expiration.Before(oldestTime) {
			oldestKey = key
			oldestTime = item.expiration
		}
	}

	if oldestKey != "" {
		if item, exists := m.items[oldestKey]; exists {
			m.currentSize -= item.size
		}
		delete(m.items, oldestKey)
	}
}

func (m *MemoryCache) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if item, exists := m.items[key]; exists {
		m.currentSize -= item.size
		delete(m.items, key)
	}

	return nil
}

func (m *MemoryCache) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.items = make(map[string]*cacheItem)
	m.currentSize = 0

	return nil
}

// CacheNotFoundError is returned when cache miss occurs
type CacheNotFoundError struct{}

func (e *CacheNotFoundError) Error() string {
	return "cache not found"
}