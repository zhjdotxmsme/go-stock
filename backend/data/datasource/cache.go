package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"time"

	"github.com/coocood/freecache"
)

// CacheEntry represents a cached data row in SQLite.
type CacheEntry struct {
	ID        uint      `gorm:"primarykey"`
	CacheKey  string    `gorm:"uniqueIndex;size:255"`
	DataType  string    `gorm:"size:50;index"`
	Data      string    `gorm:"type:text"`
	ExpiresAt time.Time `gorm:"index"`
	CreatedAt time.Time
}

func (CacheEntry) TableName() string {
	return "datasource_cache"
}

// CacheLayer provides two-level caching: L1 = memory (freecache), L2 = SQLite.
type CacheLayer struct {
	l1Cache *freecache.Cache
}

// NewCacheLayer creates a new cache with the given memory size in MB.
func NewCacheLayer(memSizeMB int) *CacheLayer {
	return &CacheLayer{
		l1Cache: freecache.NewCache(memSizeMB * 1024 * 1024),
	}
}

// Get retrieves data from cache (L1 → L2).
func (c *CacheLayer) Get(ctx context.Context, key string) (interface{}, bool) {
	// L1: try memory
	if val, err := c.l1Cache.Get([]byte(key)); err == nil {
		var data interface{}
		if err := json.Unmarshal(val, &data); err == nil {
			logger.SugaredLogger.Debugf("cache L1 hit: %s", key)
			return data, true
		}
	}

	// L2: try SQLite
	var entry CacheEntry
	err := db.Dao.Where("cache_key = ? AND expires_at > ?", key, time.Now()).First(&entry).Error
	if err != nil {
		return nil, false
	}

	// Promote to L1
	var data interface{}
	if err := json.Unmarshal([]byte(entry.Data), &data); err != nil {
		return nil, false
	}
	_ = c.l1Cache.Set([]byte(key), []byte(entry.Data), 300) // 5 min TTL in L1

	logger.SugaredLogger.Debugf("cache L2 hit: %s", key)
	return data, true
}

// Set stores data in cache (L1 + L2).
func (c *CacheLayer) Set(ctx context.Context, key string, dataType string, data interface{}, ttl time.Duration) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("cache marshal error: %w", err)
	}

	// L1: memory
	_ = c.l1Cache.Set([]byte(key), raw, int(ttl.Seconds()))

	// L2: SQLite (upsert)
	entry := CacheEntry{
		CacheKey:  key,
		DataType:  dataType,
		Data:      string(raw),
		ExpiresAt: time.Now().Add(ttl),
	}
	db.Dao.Where("cache_key = ?", key).Assign(entry).FirstOrCreate(&entry)

	return nil
}

// Invalidate removes all cache entries for a data type.
func (c *CacheLayer) Invalidate(dataType string) {
	db.Dao.Where("data_type = ?", dataType).Delete(&CacheEntry{})
	c.l1Cache.Clear()
	logger.SugaredLogger.Infof("cache invalidated for data type: %s", dataType)
}

// CacheKey generates a standardized cache key for a data type and code.
func CacheKey(dataType DataType, code string, params ...string) string {
	key := fmt.Sprintf("%s:%s", dataType, code)
	for _, p := range params {
		key += ":" + p
	}
	return key
}
