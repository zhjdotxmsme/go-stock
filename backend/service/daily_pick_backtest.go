package service

import (
	"context"
	"sort"

	"go-stock/backend/data"
	"go-stock/backend/data/backtest"
	"go-stock/backend/logger"
)

// DailyPickBacktestResult holds one stock's pick info + its backtest result.
type DailyPickBacktestResult struct {
	StockCode    string  `json:"stockCode"`
	StockName    string  `json:"stockName"`
	TradeDate    string  `json:"tradeDate"`
	Score        float64 `json:"score"`
	StrategyCode string  `json:"strategyCode"`
	StrategyName string  `json:"strategyName"`
	Reason       string  `json:"reason"`

	TotalReturn     float64 `json:"totalReturn"`
	Win             bool    `json:"win"`
	HoldingDays     int     `json:"holdingDays"`
	MaxDrawdown     float64 `json:"maxDrawdown"`
	EntryPrice      float64 `json:"entryPrice"`
	ExitPrice       float64 `json:"exitPrice"`
	SlippageWarning string  `json:"slippageWarning"`
}

// DailyPickBacktestService bridges DailyPickEngine filtering with backtest execution.
type DailyPickBacktestService struct {
	engine *data.DailyPickEngine
}

// NewDailyPickBacktestService creates a new service instance.
func NewDailyPickBacktestService() *DailyPickBacktestService {
	return &DailyPickBacktestService{
		engine: data.NewDailyPickEngine(),
	}
}

// RunBacktestForDailyPicks runs DailyPickEngine to screen stocks by strategy,
// then backtests each pick on its trade date. Returns comparative results
// sorted by total return descending.
func (s *DailyPickBacktestService) RunBacktestForDailyPicks(tradeDate string, topN int, holdingDays int, stopLoss, stopProfit float64, adjusted bool) []DailyPickBacktestResult {
	ctx := context.Background()

	picks, err := s.engine.RunDailyPick(ctx, tradeDate, topN)
	if err != nil || len(picks) == 0 {
		logger.SugaredLogger.Errorf("daily_pick backtest: RunDailyPick failed for %s: %v", tradeDate, err)
		return nil
	}

	bt := backtest.NewEngine()
	var results []DailyPickBacktestResult

	for _, p := range picks {
		in := backtest.Input{
			StockCode:   p.StockCode,
			SignalDate:  p.TradeDate,
			EntryPrice:  0,
			HoldingDays: holdingDays,
			StopLoss:    stopLoss,
			StopProfit:  stopProfit,
			Adjusted:    adjusted,
		}
		r, err := bt.Run(ctx, in)
		if err != nil {
			logger.SugaredLogger.Debugf("daily_pick backtest: skip %s on %s: %v", p.StockCode, p.TradeDate, err)
			continue
		}

		results = append(results, DailyPickBacktestResult{
			StockCode:       p.StockCode,
			StockName:       p.StockName,
			TradeDate:       p.TradeDate,
			Score:           p.Score,
			StrategyCode:    p.StrategyCode,
			StrategyName:    p.StrategyName,
			Reason:          p.Reason,
			TotalReturn:     r.TotalReturn,
			Win:             r.Win,
			HoldingDays:     r.HoldingDays,
			MaxDrawdown:     r.MaxDrawdown,
			EntryPrice:      r.EntryPrice,
			ExitPrice:       r.ExitPrice,
			SlippageWarning: r.SlippageWarning,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].TotalReturn > results[j].TotalReturn
	})

	return results
}
