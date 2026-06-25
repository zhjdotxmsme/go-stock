package datasource

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/logger"
)

var (
	globalRouter     *Router
	globalKLineStore *KLineStore
	routerOnce       sync.Once
)

// KLineData represents K-line data in datasource package format.
type KLineData struct {
	Bars   []KLineBar
	Source string
}

// KLineBar represents a single K-line bar in datasource package format.
type KLineBar struct {
	Time   time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume int64
}

// Router provides K-line data with provider fallback and persistence.
type Router struct {
	// cache could be added here in the future for L1 caching
}

// GetRouter returns the singleton Router instance with KLineStore initialized.
func GetRouter() *Router {
	routerOnce.Do(func() {
		globalRouter = &Router{}
		globalKLineStore = NewKLineStore()
	})
	return globalRouter
}

// GetKLine fetches K-line data with provider fallback and persists to SQLite.
// Implements the priority order: tdx-mac → eastmoney → sina → tencent → tdx
func (r *Router) GetKLine(ctx context.Context, stockCode, period string, count int) (*KLineData, error) {
	// Map period codes to klt format used by providers
	var klt string
	switch period {
	case "day":
		klt = "101"
	case "week":
		klt = "102"
	case "month":
		klt = "103"
	default:
		klt = "101"
	}

	// Use existing fallback logic from sina_kline_api.FetchKLineWithFallback
	result := data.FetchKLineWithFallback(stockCode, "", klt, count, "")

	if result == nil || result.Data == nil || len(*result.Data) == 0 {
		return nil, fmt.Errorf("no kline data available for stock=%s period=%s", stockCode, period)
	}

	// Convert data.KLineData to datasource.KLineData format
	data := convertToKLineData(result.Data, result.Source)

	// Persist to kline_bars table
	if globalKLineStore != nil && data != nil && len(data.Bars) > 0 {
		bars := BarsFromKLineData(stockCode, period, result.Source, true, data)
		if len(bars) > 0 {
			// Persist asynchronously to avoid blocking the response
			go func() {
				if err := globalKLineStore.UpsertKLines(context.Background(), bars); err != nil {
					logger.SugaredLogger.Warnf("failed to persist klines to cache: stock=%s period=%s source=%s error=%v",
						stockCode, period, result.Source, err)
				} else {
					logger.SugaredLogger.Debugf("persisted klines to cache: stock=%s period=%s source=%s count=%d",
						stockCode, period, result.Source, len(bars))
				}
			}()
		}
	}

	return data, nil
}

// GetStockKLineDayData fetches daily K-line data with provider fallback and persistence.
func (r *Router) GetStockKLineDayData(ctx context.Context, stockCode string, count int) (*KLineData, error) {
	return r.GetKLine(ctx, stockCode, "day", count)
}

// GetStockKLineWeekData fetches weekly K-line data with provider fallback and persistence.
func (r *Router) GetStockKLineWeekData(ctx context.Context, stockCode string, count int) (*KLineData, error) {
	return r.GetKLine(ctx, stockCode, "week", count)
}

// GetStockKLineMonthData fetches monthly K-line data with provider fallback and persistence.
func (r *Router) GetStockKLineMonthData(ctx context.Context, stockCode string, count int) (*KLineData, error) {
	return r.GetKLine(ctx, stockCode, "month", count)
}

// convertToKLineData converts data.KLineData to datasource.KLineData format.
func convertToKLineData(data *[]data.KLineData, source string) *KLineData {
	if data == nil || len(*data) == 0 {
		return nil
	}

	bars := make([]KLineBar, 0, len(*data))
	for _, k := range *data {
		// Parse time from various formats
		var tradeTime time.Time
		var parseErr error

		// Try common date formats
		formats := []string{
			"2006-01-02",
			"2006-01-02 15:04:05",
			"20060102",
		}

		for _, format := range formats {
			tradeTime, parseErr = time.Parse(format, k.Day)
			if parseErr == nil {
				break
			}
		}

		if parseErr != nil {
			logger.SugaredLogger.Warnf("failed to parse trade date: %s source=%s", k.Day, source)
			continue
		}

		// Parse numeric fields
		open, _ := parseFloat(k.Open)
		high, _ := parseFloat(k.High)
		low, _ := parseFloat(k.Low)
		close, _ := parseFloat(k.Close)
		volume, _ := parseInt64(k.Volume)

		bars = append(bars, KLineBar{
			Time:   tradeTime,
			Open:   open,
			High:   high,
			Low:    low,
			Close:  close,
			Volume: volume,
		})
	}

	return &KLineData{
		Bars:   bars,
		Source: source,
	}
}

// parseFloat safely parses a string to float64.
func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

// parseInt64 safely parses a string to int64.
func parseInt64(s string) (int64, error) {
	var i int64
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}
