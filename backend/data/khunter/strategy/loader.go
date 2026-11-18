package strategy

import (
	"context"
	"fmt"
	"time"

	"go-stock/backend/data/datasource"
	"go-stock/backend/models"
)

// LoadDailyBarsAsOf 从本地 kline_bars 读前复权日K（正序），只取 [asOfDate-400 自然日, asOfDate]
// 窗口内的数据，历史时点加载绝不包含 asOfDate 之后的未来 K 线；不足 minBars 返回 nil。
// 400 自然日（≈270 个交易日）覆盖所有策略的最长窗口（120 根）。
func LoadDailyBarsAsOf(
	ctx context.Context, code, asOfDate string, minBars int,
) ([]models.KLineBar, error) {
	end, err := time.Parse("2006-01-02", asOfDate)
	if err != nil {
		return nil, fmt.Errorf("invalid asOfDate %q: %w", asOfDate, err)
	}
	start := end.AddDate(0, 0, -400).Format("2006-01-02")
	bars, err := datasource.NewKLineStore().QueryKLinesPreferAdjusted(
		ctx, code, "day", start, asOfDate,
	)
	if err != nil {
		return nil, err
	}
	if len(bars) < minBars {
		return nil, nil
	}
	return bars, nil
}

// LoadDailyBars 兼容包装：按今天加载日K（其他调用方沿用旧入口）。
func LoadDailyBars(ctx context.Context, code string, minBars int) ([]models.KLineBar, error) {
	return LoadDailyBarsAsOf(ctx, code, time.Now().Format("2006-01-02"), minBars)
}
