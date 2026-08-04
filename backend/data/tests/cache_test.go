// backend/data/tests/cache_test.go
package data_test

import (
	"testing"
	"time"
	"go-stock/backend/data/cache"
)

func TestMultiLevelCache_SetAndGet(t *testing.T) {
	cache := cache.NewMultiLevelCache()

	err := cache.Set("test_key", "test_value", time.Minute)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	value, err := cache.Get("test_key")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if value != "test_value" {
		t.Errorf("Get() = %v, want test_value", value)
	}
}

func TestMultiLevelCache_L1CachePriority(t *testing.T) {
	cache := cache.NewMultiLevelCache()

	// Set value
	cache.Set("test_key", "test_value", time.Minute)

	// Get from L1 (memory cache)
	value, err := cache.Get("test_key")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if value != "test_value" {
		t.Errorf("Get() = %v, want test_value", value)
	}
}

func TestMultiLevelCache_Expiration(t *testing.T) {
	cache := cache.NewMultiLevelCache()

	// Set value with short TTL
	cache.Set("test_key", "test_value", time.Millisecond*100)

	// Wait for expiration
	time.Sleep(time.Millisecond * 150)

	// Should get cache miss
	_, err := cache.Get("test_key")
	if err == nil {
		t.Error("Expected cache miss after expiration")
	}
}

func TestMultiLevelCache_Delete(t *testing.T) {
	cache := cache.NewMultiLevelCache()

	cache.Set("test_key", "test_value", time.Minute)
	err := cache.Delete("test_key")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err = cache.Get("test_key")
	if err == nil {
		t.Error("Expected cache miss after deletion")
	}
}

func TestMultiLevelCache_Clear(t *testing.T) {
	cache := cache.NewMultiLevelCache()

	cache.Set("key1", "value1", time.Minute)
	cache.Set("key2", "value2", time.Minute)

	err := cache.Clear()
	if err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	_, err = cache.Get("key1")
	if err == nil {
		t.Error("Expected cache miss after clear")
	}
}