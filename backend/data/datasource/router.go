package datasource

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/stockcode"

	"golang.org/x/sync/singleflight"
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
	snapshotProviders    []SnapshotProvider
	cache                *CacheLayer
	flights              singleflight.Group
}

// providerTimeouts bounds each provider call per data type so a hanging
// source cannot consume the whole fallback budget. The call runs in a
// goroutine with a buffered result channel: on deadline the Router moves on
// while the abandoned call still completes into the buffer (bounded by the
// provider's own HTTP client timeout) instead of leaking.
var providerTimeouts = map[DataType]time.Duration{
	DataTypeQuote:       8 * time.Second,
	DataTypeKLine:       25 * time.Second,
	DataTypeNews:        10 * time.Second,
	DataTypeFundamental: 12 * time.Second,
	DataTypeSector:      10 * time.Second,
	DataTypeSnapshot:    8 * time.Second,
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

// skipLog logs a provider failure at the appropriate level: expected
// unsupported requests are debug-level, real failures warn.
func skipLog(dataType, code, provider string, err error) {
	if errors.Is(err, ErrUnsupported) {
		logger.SugaredLogger.Debugf("datasource: %s %s skipped by %s: %v", dataType, code, provider, err)
		return
	}
	logger.SugaredLogger.Warnf("datasource: %s %s from %s failed: %v, trying next", dataType, code, provider, err)
}

// callWithProviderTimeout runs fn under the per-provider deadline for the
// given data type.
func callWithProviderTimeout[T any](ctx context.Context, dt DataType, fn func(ctx context.Context) (T, error)) (T, error) {
	timeout, ok := providerTimeouts[dt]
	if !ok {
		timeout = 10 * time.Second
	}
	pctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	type result struct {
		val T
		err error
	}
	ch := make(chan result, 1) // buffered: abandoned calls complete without blocking
	go func() {
		v, err := fn(pctx)
		ch <- result{v, err}
	}()

	select {
	case res := <-ch:
		return res.val, res.err
	case <-pctx.Done():
		var zero T
		return zero, fmt.Errorf("provider timeout after %s: %w", timeout, pctx.Err())
	}
}

// RegisterSnapshotProvider registers a rich-snapshot data source provider.
func (r *Router) RegisterSnapshotProvider(p SnapshotProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.snapshotProviders = append(r.snapshotProviders, p)
	r.sortProviders()
}

// sortSnapshotProviders is handled by sortProviders; add the slice there too.
// GetQuote gets quote data with automatic fallback through all registered providers.
func (r *Router) GetQuote(ctx context.Context, code string) (*QuoteData, error) {
	code = stockcode.Normalize(code)
	key := CacheKey(DataTypeQuote, code)

	v, err, _ := r.flights.Do(key, func() (interface{}, error) {
		return r.getQuoteInner(ctx, code, key)
	})
	if err != nil {
		return nil, err
	}
	return v.(*QuoteData), nil
}

func (r *Router) getQuoteInner(ctx context.Context, code, key string) (interface{}, error) {
	r.mu.RLock()
	providers := r.quoteProviders
	r.mu.RUnlock()

	// Check cache first
	if r.cache != nil {
		d := &QuoteData{}
		if r.cache.GetInto(ctx, key, d) {
			return d, nil
		}
	}

	for _, p := range providers {
		if !p.Available(ctx) {
			logger.SugaredLogger.Debugf("datasource: %s unavailable, skipping", p.Name())
			continue
		}
		data, err := callWithProviderTimeout(ctx, DataTypeQuote, func(c context.Context) (*QuoteData, error) {
			return p.GetQuote(c, code)
		})
		if err == nil {
			logger.SugaredLogger.Infof("datasource: quote %s from %s", code, p.Name())
			// Write to cache
			if r.cache != nil {
				_ = r.cache.Set(ctx, key, string(DataTypeQuote), data, 60*time.Second)
			}
			return data, nil
		}
		skipLog("quote", code, p.Name(), err)
	}
	return nil, fmt.Errorf("GetQuote(%s): %w", code, ErrAllSourcesFailed)
}

// GetKLine gets K-line data with automatic fallback.
func (r *Router) GetKLine(ctx context.Context, code string, period string, count int) (*KLineData, error) {
	code = stockcode.Normalize(code)
	key := CacheKey(DataTypeKLine, code, period, fmt.Sprintf("%d", count))

	v, err, _ := r.flights.Do(key, func() (interface{}, error) {
		return r.getKLineInner(ctx, code, period, count, key)
	})
	if err != nil {
		return nil, err
	}
	return v.(*KLineData), nil
}

func (r *Router) getKLineInner(ctx context.Context, code, period string, count int, key string) (interface{}, error) {
	r.mu.RLock()
	providers := r.klineProviders
	r.mu.RUnlock()

	if r.cache != nil {
		d := &KLineData{}
		if r.cache.GetInto(ctx, key, d) {
			return d, nil
		}
	}

	var triedProviders []string
	for _, p := range providers {
		if !p.Available(ctx) {
			triedProviders = append(triedProviders, fmt.Sprintf("%s(unavailable)", p.Name()))
			continue
		}
		triedProviders = append(triedProviders, p.Name())
		data, err := callWithProviderTimeout(ctx, DataTypeKLine, func(c context.Context) (*KLineData, error) {
			return p.GetKLine(c, code, period, count)
		})
		if err == nil {
			if r.cache != nil {
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
		skipLog("kline", code, p.Name(), err)
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
	code = stockcode.Normalize(code)
	key := CacheKey(DataTypeNews, code, fmt.Sprintf("%d", count))

	v, err, _ := r.flights.Do(key, func() (interface{}, error) {
		return r.getNewsInner(ctx, code, count, key)
	})
	if err != nil {
		return nil, err
	}
	return v.([]NewsItem), nil
}

func (r *Router) getNewsInner(ctx context.Context, code string, count int, key string) (interface{}, error) {
	r.mu.RLock()
	providers := r.newsProviders
	r.mu.RUnlock()

	if r.cache != nil {
		items := []NewsItem{}
		if r.cache.GetInto(ctx, key, &items) {
			return items, nil
		}
	}

	for _, p := range providers {
		if !p.Available(ctx) {
			continue
		}
		data, err := callWithProviderTimeout(ctx, DataTypeNews, func(c context.Context) ([]NewsItem, error) {
			return p.GetNews(c, code, count)
		})
		if err == nil {
			if r.cache != nil {
				_ = r.cache.Set(ctx, key, string(DataTypeNews), data, 120*time.Second)
			}
			return data, nil
		}
		skipLog("news", code, p.Name(), err)
	}
	return nil, fmt.Errorf("GetNews(%s): %w", code, ErrAllSourcesFailed)
}

// GetFundamental gets fundamental data with automatic fallback.
func (r *Router) GetFundamental(ctx context.Context, code string) (*FundamentalData, error) {
	code = stockcode.Normalize(code)
	key := CacheKey(DataTypeFundamental, code)

	v, err, _ := r.flights.Do(key, func() (interface{}, error) {
		return r.getFundamentalInner(ctx, code, key)
	})
	if err != nil {
		return nil, err
	}
	return v.(*FundamentalData), nil
}

func (r *Router) getFundamentalInner(ctx context.Context, code, key string) (interface{}, error) {
	r.mu.RLock()
	providers := r.fundamentalProviders
	r.mu.RUnlock()

	if r.cache != nil {
		d := &FundamentalData{}
		if r.cache.GetInto(ctx, key, d) {
			return d, nil
		}
	}

	for _, p := range providers {
		if !p.Available(ctx) {
			continue
		}
		data, err := callWithProviderTimeout(ctx, DataTypeFundamental, func(c context.Context) (*FundamentalData, error) {
			return p.GetFundamental(c, code)
		})
		if err == nil {
			if r.cache != nil {
				_ = r.cache.Set(ctx, key, string(DataTypeFundamental), data, 600*time.Second)
			}
			return data, nil
		}
		skipLog("fundamental", code, p.Name(), err)
	}
	return nil, fmt.Errorf("GetFundamental(%s): %w", code, ErrAllSourcesFailed)
}

// GetSectorData gets sector data with automatic fallback.
func (r *Router) GetSectorData(ctx context.Context, code string) (*SectorData, error) {
	code = stockcode.Normalize(code)
	key := CacheKey(DataTypeSector, code)

	v, err, _ := r.flights.Do(key, func() (interface{}, error) {
		return r.getSectorInner(ctx, code, key)
	})
	if err != nil {
		return nil, err
	}
	return v.(*SectorData), nil
}

func (r *Router) getSectorInner(ctx context.Context, code, key string) (interface{}, error) {
	r.mu.RLock()
	providers := r.sectorProviders
	r.mu.RUnlock()

	if r.cache != nil {
		d := &SectorData{}
		if r.cache.GetInto(ctx, key, d) {
			return d, nil
		}
	}

	for _, p := range providers {
		if !p.Available(ctx) {
			continue
		}
		data, err := callWithProviderTimeout(ctx, DataTypeSector, func(c context.Context) (*SectorData, error) {
			return p.GetSectorData(c, code)
		})
		if err == nil {
			if r.cache != nil {
				_ = r.cache.Set(ctx, key, string(DataTypeSector), data, 300*time.Second)
			}
			return data, nil
		}
		skipLog("sector", code, p.Name(), err)
	}
	return nil, fmt.Errorf("GetSectorData(%s): %w", code, ErrAllSourcesFailed)
}

// sortProviders sorts all provider slices by priority (ascending, stable so
// same-priority providers keep registration order deterministically).
func (r *Router) sortProviders() {
	sort.SliceStable(r.quoteProviders, func(i, j int) bool { return r.quoteProviders[i].Priority() < r.quoteProviders[j].Priority() })
	sort.SliceStable(r.klineProviders, func(i, j int) bool { return r.klineProviders[i].Priority() < r.klineProviders[j].Priority() })
	sort.SliceStable(r.newsProviders, func(i, j int) bool { return r.newsProviders[i].Priority() < r.newsProviders[j].Priority() })
	sort.SliceStable(r.fundamentalProviders, func(i, j int) bool { return r.fundamentalProviders[i].Priority() < r.fundamentalProviders[j].Priority() })
	sort.SliceStable(r.sectorProviders, func(i, j int) bool { return r.sectorProviders[i].Priority() < r.sectorProviders[j].Priority() })
	sort.SliceStable(r.snapshotProviders, func(i, j int) bool { return r.snapshotProviders[i].Priority() < r.snapshotProviders[j].Priority() })
}

// GetSnapshot gets a rich real-time snapshot with automatic fallback.
// Short cache TTL: snapshots back "real-time price" style reads, so they
// must stay fresher than plain quotes.
func (r *Router) GetSnapshot(ctx context.Context, code string) (*SnapshotData, error) {
	code = stockcode.Normalize(code)
	key := CacheKey(DataTypeSnapshot, code)

	v, err, _ := r.flights.Do(key, func() (interface{}, error) {
		return r.getSnapshotInner(ctx, code, key)
	})
	if err != nil {
		return nil, err
	}
	return v.(*SnapshotData), nil
}

func (r *Router) getSnapshotInner(ctx context.Context, code, key string) (interface{}, error) {
	r.mu.RLock()
	providers := r.snapshotProviders
	r.mu.RUnlock()

	if r.cache != nil {
		d := &SnapshotData{}
		if r.cache.GetInto(ctx, key, d) {
			return d, nil
		}
	}

	for _, p := range providers {
		if !p.Available(ctx) {
			logger.SugaredLogger.Debugf("datasource: %s unavailable, skipping", p.Name())
			continue
		}
		data, err := callWithProviderTimeout(ctx, DataTypeSnapshot, func(c context.Context) (*SnapshotData, error) {
			return p.GetSnapshot(c, code)
		})
		if err == nil {
			logger.SugaredLogger.Infof("datasource: snapshot %s from %s", code, p.Name())
			if r.cache != nil {
				_ = r.cache.Set(ctx, key, string(DataTypeSnapshot), data, 15*time.Second)
			}
			return data, nil
		}
		skipLog("snapshot", code, p.Name(), err)
	}
	return nil, fmt.Errorf("GetSnapshot(%s): %w", code, ErrAllSourcesFailed)
}
