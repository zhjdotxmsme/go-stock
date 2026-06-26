package datasource

import (
	"context"
	"fmt"
	"sort"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"

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
func (s *KLineStore) QueryKLines(ctx context.Context, stockCode, period, startDate, endDate string, adjusted bool) ([]models.KLineBar, error) {
	var bars []models.KLineBar
	err := db.Dao.WithContext(ctx).
		Where("stock_code = ? AND period = ? AND adjusted = ? AND trade_date BETWEEN ? AND ?",
			stockCode, period, adjusted, startDate, endDate).
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

// isHoliday checks if a given date is a holiday using the holiday tool API.
func isHoliday(date time.Time) bool {
	dateStr := date.Format("2006-01-02")
	
	apiURL := fmt.Sprintf("https://timor.tech/api/holiday/info/%s", dateStr)
	var result struct {
		Code    int `json:"code"`
		Holiday struct {
			Holiday bool `json:"holiday"`
		} `json:"holiday"`
	}
	
	resp, err := data.SharedHTTPClient.R().
		SetResult(&result).
		Get(apiURL)
	
	if err != nil || resp.StatusCode() != 200 || result.Code != 0 {
		return false
	}
	
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
			Volume:    b.Volume,
			Amount:    b.Amount,
			Source:    source,
		})
	}
	sort.Slice(bars, func(i, j int) bool { return bars[i].TradeDate < bars[j].TradeDate })
	return bars
}