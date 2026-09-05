// backend/data/cache/database_cache.go
package cache

import (
	"context"
	"encoding/json"
	"sync"
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

// ensureTableOnce CacheItem 表从未纳入 db.AutoMigrate（db 包反向依赖 cache 会造成循环导入），
// 这里在首个 DatabaseCache 实例创建时一次性建表；已存在时为 no-op。
var ensureTableOnce sync.Once

// NewDatabaseCache creates a new database cache
func NewDatabaseCache(ttl time.Duration) *DatabaseCache {
	d := &DatabaseCache{
		db:  db.Dao,
		ttl: ttl,
	}
	if d.db != nil {
		ensureTableOnce.Do(func() {
			_ = d.db.AutoMigrate(&CacheItem{})
		})
	}
	return d
}

func (d *DatabaseCache) Get(ctx context.Context, key string) (any, error) {
	if d.db == nil {
		return nil, &CacheNotFoundError{}
	}
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
	if d.db == nil {
		// DB 层未初始化（如部分测试环境）：L3 降级为 no-op，不影响 L1/L2
		return nil
	}
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
	if d.db == nil {
		return nil
	}
	return d.db.WithContext(ctx).Where("key = ?", key).Delete(&CacheItem{}).Error
}

func (d *DatabaseCache) Clear(ctx context.Context) error {
	if d.db == nil {
		return nil
	}
	// Clear 语义是清空全部缓存（L1/L2 均如此）；只删已过期项会导致
	// MultiLevelCache.Clear 后 L3 仍返回旧数据（缓存一致性 bug）
	return d.db.WithContext(ctx).Where("1 = 1").Delete(&CacheItem{}).Error
}

func (d *DatabaseCache) CleanupExpired(ctx context.Context) error {
	if d.db == nil {
		return nil
	}
	return d.db.WithContext(ctx).Where("expiration < ?", time.Now()).Delete(&CacheItem{}).Error
}