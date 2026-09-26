package strategy

import (
	"context"
	"time"

	"go-stock/backend/data/datasource"
	"go-stock/backend/models"
)

// LoadDailyBars 从本地 kline_bars 读前复权日K（正序），不足 minBars 返回 nil。
// lookback 固定 400 自然日（≈270 个交易日），覆盖所有策略的最长窗口（120 根）。
func LoadDailyBars(ctx context.Context, code string, minBars int) ([]models.KLineBar, error) {
	end := time.Now().Format("2006-01-02")
	start := time.Now().AddDate(0, 0, -400).Format("2006-01-02")
	bars, err := datasource.NewKLineStore().QueryKLines(ctx, code, "day", start, end, true)
	if err != nil {
		return nil, err
	}
	if len(bars) < minBars {
		return nil, nil
	}
	return bars, nil
}
