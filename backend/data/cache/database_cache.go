// backend/data/cache/database_cache.go
package cache

import (
	"context"
	"encoding/json"
	"time"
	"go-stock/backend/db"
	"gorm.io/gorm"
)

// DatabaseCache implements L3 database cache
type DatabaseCache struct {
	db  *gorm.DB
	ttl time.Duration
}

// CacheItem represents a cached item in database
type CacheItem struct {
	Key        string    `gorm:"primaryKey"`
	Value      string    `gorm:"type:text"`
	Expiration time.Time `gorm:"index"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// NewDatabaseCache creates a new database cache
func NewDatabaseCache(ttl time.Duration) *DatabaseCache {
	return &DatabaseCache{
		db:  db.Dao,
		ttl: ttl,
	}
}

func (d *DatabaseCache) Get(ctx context.Context, key string) (any, error) {
	var item CacheItem
	err := d.db.WithContext(ctx).Where("key = ? AND expiration > ?", key, time.Now()).First(&item).Error
	if err == gorm.ErrRecordNotFound {
		return nil, &CacheNotFoundError{}
	}
	if err != nil {
		return nil, err
	}

	var value any
	if err := json.Unmarshal([]byte(item.Value), &value); err != nil {
		return nil, err
	}

	return value, nil
}

func (d *DatabaseCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	// Serialize value
	valueBytes, err := json.Marshal(value)
	if err != nil {
		return err
	}

	expiration := time.Now().Add(ttl)
	if ttl == 0 {
		expiration = time.Now().Add(d.ttl)
	}

	item := CacheItem{
		Key:        key,
		Value:      string(valueBytes),
		Expiration: expiration,
	}

	return d.db.WithContext(ctx).Save(&item).Error
}

func (d *DatabaseCache) Delete(ctx context.Context, key string) error {
	return d.db.WithContext(ctx).Where("key = ?", key).Delete(&CacheItem{}).Error
}

func (d *DatabaseCache) Clear(ctx context.Context) error {
	return d.db.WithContext(ctx).Where("expiration < ?", time.Now()).Delete(&CacheItem{}).Error
}

func (d *DatabaseCache) CleanupExpired(ctx context.Context) error {
	return d.db.WithContext(ctx).Where("expiration < ?", time.Now()).Delete(&CacheItem{}).Error
}