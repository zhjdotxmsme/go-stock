package datasource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"time"

	"github.com/coocood/freecache"
	"gorm.io/gorm"
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

// Get retrieves data from cache (L1 → L2) as a generic decoded value.
// Prefer GetInto for typed reads; this method cannot satisfy concrete-type
// assertions because JSON decoding into interface{} yields maps/slices.
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
	if !c.getL2(key, &entry) {
		return nil, false
	}

	// Promote to L1
	var data interface{}
	if err := json.Unmarshal([]byte(entry.Data), &data); err != nil {
		return nil, false
	}
	_ = c.l1Cache.Set([]byte(key), []byte(entry.Data), c.l1TTLFor(&entry))

	logger.SugaredLogger.Debugf("cache L2 hit: %s", key)
	return data, true
}

// GetInto retrieves data from cache into target, which must be a pointer to
// the concrete type stored by Set (e.g. *QuoteData or *[]NewsItem).
// Returns true on cache hit.
func (c *CacheLayer) GetInto(ctx context.Context, key string, target interface{}) bool {
	// L1: try memory
	if val, err := c.l1Cache.Get([]byte(key)); err == nil {
		if err := json.Unmarshal(val, target); err == nil {
			logger.SugaredLogger.Debugf("cache L1 hit: %s", key)
			return true
		}
	}

	// L2: try SQLite
	var entry CacheEntry
	if !c.getL2(key, &entry) {
		return false
	}

	if err := json.Unmarshal([]byte(entry.Data), target); err != nil {
		return false
	}
	_ = c.l1Cache.Set([]byte(key), []byte(entry.Data), c.l1TTLFor(&entry))

	logger.SugaredLogger.Debugf("cache L2 hit: %s", key)
	return true
}

// getL2 loads a non-expired entry from SQLite. Returns false on miss or when
// the DB is not initialized; unexpected read failures are logged as errors.
func (c *CacheLayer) getL2(key string, entry *CacheEntry) bool {
	if db.Dao == nil {
		return false
	}
	err := db.Dao.Where("cache_key = ? AND expires_at > ?", key, time.Now()).First(entry).Error
	if err == nil {
		return true
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.SugaredLogger.Errorf("cache L2 read failed for %s: %v", key, err)
	}
	return false
}

// l1TTLFor computes the remaining L1 TTL when promoting an L2 entry, clamped
// to [1, 300] seconds so promoted entries can never outlive the stored expiry.
func (c *CacheLayer) l1TTLFor(entry *CacheEntry) int {
	remaining := int(time.Until(entry.ExpiresAt).Seconds())
	if remaining > 300 {
		remaining = 300
	}
	if remaining < 1 {
		remaining = 1
	}
	return remaining
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
	if db.Dao == nil {
		return nil
	}
	entry := CacheEntry{
		CacheKey:  key,
		DataType:  dataType,
		Data:      string(raw),
		ExpiresAt: time.Now().Add(ttl),
	}
	if err := db.Dao.Where("cache_key = ?", key).Assign(entry).FirstOrCreate(&entry).Error; err != nil {
		logger.SugaredLogger.Errorf("cache L2 write failed for %s: %v", key, err)
	}

	return nil
}

// Invalidate removes all cache entries for a data type.
func (c *CacheLayer) Invalidate(dataType string) {
	if db.Dao == nil {
		c.l1Cache.Clear()
		return
	}
	if err := db.Dao.Where("data_type = ?", dataType).Delete(&CacheEntry{}).Error; err != nil {
		logger.SugaredLogger.Errorf("cache invalidate failed for %s: %v", dataType, err)
	}
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
