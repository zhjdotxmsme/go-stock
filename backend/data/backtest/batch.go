package backtest

import (
	"context"
	"math"
	"sort"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

type BatchResult struct {
	TotalTrades    int
	WinCount       int
	LossCount      int
	WinRate        float64
	AvgReturn      float64
	TotalReturn    float64
	AvgHoldingDays float64
	MaxDrawdown    float64
	SharpeRatio    float64
	Results        []*Result
}

func generateSignalDates(ctx context.Context, stockCode, period, startDate, endDate string, adjusted bool) ([]string, error) {
	var dates []string
	err := db.Dao.WithContext(ctx).
		Model(&models.KLineBar{}).
		Where("stock_code = ? AND period = ? AND adjusted = ? AND trade_date BETWEEN ? AND ?",
			stockCode, period, adjusted, startDate, endDate).
		Pluck("DISTINCT trade_date", &dates).Error
	if err != nil {
		return nil, err
	}
	sort.Strings(dates)
	return dates, nil
}

func RunBatchBacktest(ctx context.Context, stockCode, startDate, endDate, period string, adjusted bool, entryPrice float64, holdingDays int, stopLoss, stopProfit float64) (*BatchResult, error) {
	dates, err := generateSignalDates(ctx, stockCode, period, startDate, endDate, adjusted)
	if err != nil {
		return nil, err
	}
	if len(dates) == 0 {
		return &BatchResult{}, nil
	}

	engine := NewEngine()
	var results []*Result

	for _, signalDate := range dates {
		in := Input{
			StockCode:   stockCode,
			SignalDate:  signalDate,
			EntryPrice:  entryPrice,
			HoldingDays: holdingDays,
			StopLoss:    stopLoss,
			StopProfit:  stopProfit,
			Adjusted:    adjusted,
		}
		r, err := engine.Run(ctx, in)
		if err != nil {
			continue
		}
		results = append(results, r)

		db.Dao.WithContext(ctx).Create(&models.AiRecommendBacktest{
			StockCode:   stockCode,
			SignalDate:  signalDate,
			EntryPrice:  r.EntryPrice,
			ExitPrice:   r.ExitPrice,
			ExitDate:    r.ExitDate,
			HoldingDays: r.HoldingDays,
			TotalReturn: r.TotalReturn,
			MaxDrawdown: r.MaxDrawdown,
			Alpha:       r.Alpha,
			Win:         r.Win,
			Source:      "batch",
		})
	}

	if len(results) == 0 {
		return &BatchResult{}, nil
	}

	return aggregateResults(results), nil
}

func aggregateResults(results []*Result) *BatchResult {
	total := len(results)
	winCount := 0
	var sumReturn, sumHoldingDays float64
	cumulative := 1.0
	var equityCurve []float64

	for _, r := range results {
		if r.Win {
			winCount++
		}
		sumReturn += r.TotalReturn
		sumHoldingDays += float64(r.HoldingDays)
		cumulative *= (1 + r.TotalReturn)
		equityCurve = append(equityCurve, cumulative)
	}

	lossCount := total - winCount
	winRate := float64(winCount) / float64(total)
	avgReturn := sumReturn / float64(total)
	avgHoldingDays := sumHoldingDays / float64(total)
	totalReturn := cumulative - 1

	peak := equityCurve[0]
	maxDD := 0.0
	for _, v := range equityCurve {
		if v > peak {
			peak = v
		}
		dd := (peak - v) / peak
		if dd > maxDD {
			maxDD = dd
		}
	}

	sharpe := 0.0
	if total > 1 && avgHoldingDays > 0 {
		mean := avgReturn
		var sumSq float64
		for _, r := range results {
			d := r.TotalReturn - mean
			sumSq += d * d
		}
		std := math.Sqrt(sumSq / float64(total-1))
		if std > 0 {
			sharpe = (mean / std) * math.Sqrt(252.0/avgHoldingDays)
		}
	}

	return &BatchResult{
		TotalTrades:    total,
		WinCount:       winCount,
		LossCount:      lossCount,
		WinRate:        winRate,
		AvgReturn:      avgReturn,
		TotalReturn:    totalReturn,
		AvgHoldingDays: avgHoldingDays,
		MaxDrawdown:    maxDD,
		SharpeRatio:    sharpe,
		Results:        results,
	}
}
