package datasource

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"
)

// Router manages data source providers per data type, routing to the highest-priority
// available provider and falling back to lower-priority providers on failure.
type Router struct {
	mu                   sync.RWMutex
	quoteProviders       []QuoteProvider
	klineProviders       []KLineProvider
	newsProviders        []NewsProvider
	fundamentalProviders []FundamentalProvider
	sectorProviders      []SectorProvider
	cache                *CacheLayer
}

var (
	globalRouter     *Router
	globalKLineStore *KLineStore
	routerOnce       sync.Once
)

// GetRouter returns the singleton router instance.
func GetRouter() *Router {
	routerOnce.Do(func() {
		globalRouter = &Router{}
		globalKLineStore = NewKLineStore()
	})
	return globalRouter
}

// SetCache attaches a cache layer to the router.
func (r *Router) SetCache(c *CacheLayer) {
	r.cache = c
}

// RegisterQuoteProvider registers a quote data source provider.
func (r *Router) RegisterQuoteProvider(p QuoteProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.quoteProviders = append(r.quoteProviders, p)
	r.sortProviders()
}

// RegisterKLineProvider registers a K-line data source provider.
func (r *Router) RegisterKLineProvider(p KLineProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.klineProviders = append(r.klineProviders, p)
	r.sortProviders()
}

// RegisterNewsProvider registers a news data source provider.
func (r *Router) RegisterNewsProvider(p NewsProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.newsProviders = append(r.newsProviders, p)
	r.sortProviders()
}

// RegisterFundamentalProvider registers a fundamental data source provider.
func (r *Router) RegisterFundamentalProvider(p FundamentalProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fundamentalProviders = append(r.fundamentalProviders, p)
	r.sortProviders()
}

// RegisterSectorProvider registers a sector data source provider.
func (r *Router) RegisterSectorProvider(p SectorProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sectorProviders = append(r.sectorProviders, p)
	r.sortProviders()
}

// GetQuote gets quote data with automatic fallback through all registered providers.
func (r *Router) GetQuote(ctx context.Context, code string) (*QuoteData, error) {
	r.mu.RLock()
	providers := r.quoteProviders
	r.mu.RUnlock()

	// Check cache first
	if r.cache != nil {
		key := CacheKey(DataTypeQuote, code)
		if cached, ok := r.cache.Get(ctx, key); ok {
			if d, ok2 := cached.(*QuoteData); ok2 {
				return d, nil
			}
		}
	}

	for _, p := range providers {
		if !p.Available(ctx) {
			logger.SugaredLogger.Debugf("datasource: %s unavailable, skipping", p.Name())
			continue
		}
		data, err := p.GetQuote(ctx, code)
		if err == nil {
			logger.SugaredLogger.Infof("datasource: quote %s from %s", code, p.Name())
			// Write to cache
			if r.cache != nil {
				key := CacheKey(DataTypeQuote, code)
				_ = r.cache.Set(ctx, key, string(DataTypeQuote), data, 60*time.Second)
			}
			return data, nil
		}
		logger.SugaredLogger.Warnf("datasource: quote %s from %s failed: %v, trying next", code, p.Name(), err)
	}
	return nil, fmt.Errorf("GetQuote(%s): %w", code, ErrAllSourcesFailed)
}

// GetKLine gets K-line data with automatic fallback.
func (r *Router) GetKLine(ctx context.Context, code string, period string, count int) (*KLineData, error) {
	r.mu.RLock()
	providers := r.klineProviders
	r.mu.RUnlock()

	if r.cache != nil {
		key := CacheKey(DataTypeKLine, code, period, fmt.Sprintf("%d", count))
		if cached, ok := r.cache.Get(ctx, key); ok {
			if d, ok2 := cached.(*KLineData); ok2 {
				return d, nil
			}
		}
	}

	var triedProviders []string
	for _, p := range providers {
		if !p.Available(ctx) {
			triedProviders = append(triedProviders, fmt.Sprintf("%s(unavailable)", p.Name()))
			continue
		}
		triedProviders = append(triedProviders, p.Name())
		data, err := p.GetKLine(ctx, code, period, count)
		if err == nil {
			if r.cache != nil {
				key := CacheKey(DataTypeKLine, code, period, fmt.Sprintf("%d", count))
				_ = r.cache.Set(ctx, key, string(DataTypeKLine), data, 300*time.Second)
			}
			// Persist to SQLite via KLineStore (async, non-blocking)
			if globalKLineStore != nil && data != nil && len(data.Bars) > 0 {
				bars := BarsFromKLineData(code, period, p.Name(), true, data)
				if len(bars) > 0 {
					go func() {
						if db.Dao == nil {
							return
						}
						if err := globalKLineStore.UpsertKLines(context.Background(), bars); err != nil {
							logger.SugaredLogger.Warnf("failed to persist klines: stock=%s error=%v", code, err)
						}
					}()
				}
			}
			return data, nil
		}
		logger.SugaredLogger.Warnf("datasource: kline %s from %s failed: %v, trying next", code, p.Name(), err)
	}
	logger.SugaredLogger.Errorf("datasource: K线数据全部失败 %s — 尝试了 %d 个源: %v", code, len(triedProviders), triedProviders)
	return nil, fmt.Errorf("GetKLine(%s): %w", code, ErrAllSourcesFailed)
}

// GetStockKLineDayData fetches daily K-line data with provider fallback.
func (r *Router) GetStockKLineDayData(ctx context.Context, stockCode string, count int) (*KLineData, error) {
	return r.GetKLine(ctx, stockCode, "day", count)
}

// GetStockKLineWeekData fetches weekly K-line data with provider fallback.
func (r *Router) GetStockKLineWeekData(ctx context.Context, stockCode string, count int) (*KLineData, error) {
	return r.GetKLine(ctx, stockCode, "week", count)
}

// GetStockKLineMonthData fetches monthly K-line data with provider fallback.
func (r *Router) GetStockKLineMonthData(ctx context.Context, stockCode string, count int) (*KLineData, error) {
	return r.GetKLine(ctx, stockCode, "month", count)
}

// GetNews gets news data with automatic fallback.
func (r *Router) GetNews(ctx context.Context, code string, count int) ([]NewsItem, error) {
	r.mu.RLock()
	providers := r.newsProviders
	r.mu.RUnlock()

	if r.cache != nil {
		key := CacheKey(DataTypeNews, code, fmt.Sprintf("%d", count))
		if cached, ok := r.cache.Get(ctx, key); ok {
			if d, ok2 := cached.([]NewsItem); ok2 {
				return d, nil
			}
		}
	}

	for _, p := range providers {
		if !p.Available(ctx) {
			continue
		}
		data, err := p.GetNews(ctx, code, count)
		if err == nil {
			if r.cache != nil {
				key := CacheKey(DataTypeNews, code, fmt.Sprintf("%d", count))
				_ = r.cache.Set(ctx, key, string(DataTypeNews), data, 120*time.Second)
			}
			return data, nil
		}
		logger.SugaredLogger.Warnf("datasource: news %s from %s failed: %v, trying next", code, p.Name(), err)
	}
	return nil, fmt.Errorf("GetNews(%s): %w", code, ErrAllSourcesFailed)
}

// GetFundamental gets fundamental data with automatic fallback.
func (r *Router) GetFundamental(ctx context.Context, code string) (*FundamentalData, error) {
	r.mu.RLock()
	providers := r.fundamentalProviders
	r.mu.RUnlock()

	if r.cache != nil {
		key := CacheKey(DataTypeFundamental, code)
		if cached, ok := r.cache.Get(ctx, key); ok {
			if d, ok2 := cached.(*FundamentalData); ok2 {
				return d, nil
			}
		}
	}

	for _, p := range providers {
		if !p.Available(ctx) {
			continue
		}
		data, err := p.GetFundamental(ctx, code)
		if err == nil {
			if r.cache != nil {
				key := CacheKey(DataTypeFundamental, code)
				_ = r.cache.Set(ctx, key, string(DataTypeFundamental), data, 600*time.Second)
			}
			return data, nil
		}
		logger.SugaredLogger.Warnf("datasource: fundamental %s from %s failed: %v, trying next", code, p.Name(), err)
	}
	return nil, fmt.Errorf("GetFundamental(%s): %w", code, ErrAllSourcesFailed)
}

// GetSectorData gets sector data with automatic fallback.
func (r *Router) GetSectorData(ctx context.Context, code string) (*SectorData, error) {
	r.mu.RLock()
	providers := r.sectorProviders
	r.mu.RUnlock()

	if r.cache != nil {
		key := CacheKey(DataTypeSector, code)
		if cached, ok := r.cache.Get(ctx, key); ok {
			if d, ok2 := cached.(*SectorData); ok2 {
				return d, nil
			}
		}
	}

	for _, p := range providers {
		if !p.Available(ctx) {
			continue
		}
		data, err := p.GetSectorData(ctx, code)
		if err == nil {
			if r.cache != nil {
				key := CacheKey(DataTypeSector, code)
				_ = r.cache.Set(ctx, key, string(DataTypeSector), data, 300*time.Second)
			}
			return data, nil
		}
		logger.SugaredLogger.Warnf("datasource: sector %s from %s failed: %v, trying next", code, p.Name(), err)
	}
	return nil, fmt.Errorf("GetSectorData(%s): %w", code, ErrAllSourcesFailed)
}

// sortProviders sorts all provider slices by priority (ascending).
func (r *Router) sortProviders() {
	sort.Slice(r.quoteProviders, func(i, j int) bool { return r.quoteProviders[i].Priority() < r.quoteProviders[j].Priority() })
	sort.Slice(r.klineProviders, func(i, j int) bool { return r.klineProviders[i].Priority() < r.klineProviders[j].Priority() })
	sort.Slice(r.newsProviders, func(i, j int) bool { return r.newsProviders[i].Priority() < r.newsProviders[j].Priority() })
	sort.Slice(r.fundamentalProviders, func(i, j int) bool { return r.fundamentalProviders[i].Priority() < r.fundamentalProviders[j].Priority() })
	sort.Slice(r.sectorProviders, func(i, j int) bool { return r.sectorProviders[i].Priority() < r.sectorProviders[j].Priority() })
}
