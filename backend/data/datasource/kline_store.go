package datasource

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/stockcode"

	"github.com/go-resty/resty/v2"
	"gorm.io/gorm/clause"
)

// DateRange represents a missing date interval.
type DateRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// KLineStore provides SQLite-backed K-line persistence.
type KLineStore struct{}

// NewKLineStore creates a new KLineStore instance.
func NewKLineStore() *KLineStore {
	return &KLineStore{}
}

// QueryKLines returns bars for (code, period, adjusted) in [start, end].
// Uses StockCodeCandidates to handle historical data with mixed code formats.
func (s *KLineStore) QueryKLines(ctx context.Context, stockCode, period, startDate, endDate string, adjusted bool) ([]models.KLineBar, error) {
	candidates := stockcode.StockCodeCandidates(stockCode)
	var bars []models.KLineBar
	err := db.Dao.WithContext(ctx).
		Where("stock_code IN ? AND period = ? AND adjusted = ? AND trade_date BETWEEN ? AND ?",
			candidates, period, adjusted, startDate, endDate).
		Order("trade_date ASC").
		Find(&bars).Error
	return bars, err
}

// UpsertKLines inserts or updates bars in batches with deduplication.
func (s *KLineStore) UpsertKLines(ctx context.Context, bars []models.KLineBar) error {
	if len(bars) == 0 {
		return nil
	}

	const defaultBatchSize = 1000
	batchSize := defaultBatchSize

	bars = deduplicateBars(bars)

	for i := 0; i < len(bars); i += batchSize {
		end := i + batchSize
		if end > len(bars) {
			end = len(bars)
		}
		batch := bars[i:end]
		if err := db.Dao.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "stock_code"}, {Name: "period"}, {Name: "trade_date"}, {Name: "adjusted"}},
			UpdateAll: true,
		}).Create(&batch).Error; err != nil {
			return fmt.Errorf("upsert klines batch %d-%d: %w", i, end, err)
		}
	}
	return nil
}

// deduplicateBars removes duplicate bars based on stock_code, period, trade_date, adjusted.
func deduplicateBars(bars []models.KLineBar) []models.KLineBar {
	seen := make(map[string]struct{}, len(bars))
	deduped := make([]models.KLineBar, 0, len(bars))
	
	for _, bar := range bars {
		key := fmt.Sprintf("%s|%s|%s|%t", bar.StockCode, bar.Period, bar.TradeDate, bar.Adjusted)
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			deduped = append(deduped, bar)
		}
	}
	return deduped
}

// FindMissingDateRanges computes missing date intervals given [start, end] and existing bars.
// Takes T+1 trading calendar into account by checking weekends and holidays.
func (s *KLineStore) FindMissingDateRanges(ctx context.Context, stockCode, period, startDate, endDate string, adjusted bool) ([]DateRange, error) {
	bars, err := s.QueryKLines(ctx, stockCode, period, startDate, endDate, adjusted)
	if err != nil {
		return nil, fmt.Errorf("query existing bars: %w", err)
	}

	existing := make(map[string]struct{}, len(bars))
	for _, b := range bars {
		existing[b.TradeDate] = struct{}{}
	}

	startT, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, fmt.Errorf("parse start date: %w", err)
	}
	endT, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return nil, fmt.Errorf("parse end date: %w", err)
	}

	// Bulk-load holiday flags for the range first: one batched HTTP request
	// per 50 dates instead of one request per trading day (N+1 avoidance).
	var rangeDates []time.Time
	for d := startT; !d.After(endT); d = d.AddDate(0, 0, 1) {
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			continue
		}
		rangeDates = append(rangeDates, d)
	}
	prefetchHolidays(rangeDates)

	var missing []DateRange
	var rangeStart *time.Time

	for d := startT; !d.After(endT); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")

		// Skip non-trading days (weekends)
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			continue
		}

		// Skip holidays (check if it's a trading day)
		if isHoliday(d) {
			continue
		}

		_, exists := existing[dateStr]
		if !exists {
			if rangeStart == nil {
				ts := d
				rangeStart = &ts
			}
		} else if rangeStart != nil {
			prev := d.AddDate(0, 0, -1)
			missing = append(missing, DateRange{
				Start: rangeStart.Format("2006-01-02"),
				End:   prev.Format("2006-01-02"),
			})
			rangeStart = nil
		}
	}

	if rangeStart != nil {
		missing = append(missing, DateRange{
			Start: rangeStart.Format("2006-01-02"),
			End:   endT.Format("2006-01-02"),
		})
	}

	return missing, nil
}

// holidayCache caches holiday lookups per date ("2006-01-02" → bool) for the
// process lifetime so repeated range scans don't re-query the holiday API.
var holidayCache sync.Map

// prefetchHolidays bulk-loads holiday flags via the timor.tech batch endpoint
// (50 dates per request). On batch failure the affected dates fall back to
// per-date queries inside isHoliday; only successful determinations are cached.
func prefetchHolidays(dates []time.Time) {
	var need []string
	for _, d := range dates {
		s := d.Format("2006-01-02")
		if _, ok := holidayCache.Load(s); !ok {
			need = append(need, s)
		}
	}
	if len(need) == 0 {
		return
	}

	client := resty.New().SetTimeout(15 * time.Second)
	var result struct {
		Code int `json:"code"`
		Data map[string]struct {
			Holiday bool `json:"holiday"`
		} `json:"data"`
	}

	for len(need) > 0 {
		chunk := need
		if len(chunk) > 50 {
			chunk = chunk[:50]
		}
		need = need[len(chunk):]

		result.Code = -1
		result.Data = nil
		url := "https://timor.tech/api/holiday/batch?d=" + strings.Join(chunk, "|")
		resp, err := client.R().SetResult(&result).Get(url)
		if err != nil || resp.StatusCode() != 200 || result.Code != 0 || result.Data == nil {
			continue // per-date fallback inside isHoliday handles these
		}
		for dateStr, info := range result.Data {
			holidayCache.Store(dateStr, info.Holiday)
		}
	}
}

// isHoliday checks if a given date is a holiday using the holiday tool API.
func isHoliday(date time.Time) bool {
	dateStr := date.Format("2006-01-02")

	if v, ok := holidayCache.Load(dateStr); ok {
		return v.(bool)
	}

	apiURL := fmt.Sprintf("https://timor.tech/api/holiday/info/%s", dateStr)
	var result struct {
		Code    int `json:"code"`
		Holiday struct {
			Holiday bool `json:"holiday"`
		} `json:"holiday"`
	}

	resp, err := resty.New().SetTimeout(10*time.Second).R().
		SetResult(&result).
		Get(apiURL)

	if err != nil || resp.StatusCode() != 200 || result.Code != 0 {
		return false
	}

	holidayCache.Store(dateStr, result.Holiday.Holiday)
	return result.Holiday.Holiday
}

// BarsFromKLineData converts datasource.KLineData to []models.KLineBar.
func BarsFromKLineData(code, period, source string, adjusted bool, data *KLineData) []models.KLineBar {
	if data == nil {
		return nil
	}
	bars := make([]models.KLineBar, 0, len(data.Bars))
	for _, b := range data.Bars {
		bars = append(bars, models.KLineBar{
			StockCode: code,
			Period:    period,
			TradeDate: b.Time.Format("2006-01-02"),
			Adjusted:  adjusted,
			Open:      b.Open,
			High:      b.High,
			Low:       b.Low,
			Close:     b.Close,
			PrevClose: b.PrevClose, // 透传数据源提供的 PrevClose
			Volume:    b.Volume,
			Amount:    b.Amount,
			Source:    source,
		})
	}
	sort.Slice(bars, func(i, j int) bool { return bars[i].TradeDate < bars[j].TradeDate })

	// 对未填充 PrevClose 的 bar 按序列填充（后一条的 PrevClose = 前一条的 Close）
	for i := 1; i < len(bars); i++ {
		if bars[i].PrevClose == 0 && bars[i-1].Close > 0 {
			bars[i].PrevClose = bars[i-1].Close
		}
	}

	return bars
}