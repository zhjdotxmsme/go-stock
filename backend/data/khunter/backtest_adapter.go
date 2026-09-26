package khunter

import (
	"context"
	"sort"
	"sync"

	"go-stock/backend/data/backtest"
	"go-stock/backend/data/khunter/strategy"
	"go-stock/backend/logger"
)

// StrategyBacktestStat 单策略回测统计
type StrategyBacktestStat struct {
	Strategy        string
	Total           int
	Wins            int
	WinRate         float64
	AvgWin          float64
	AvgLoss         float64
	ProfitLossRatio float64
}

// aggregateReturns 聚合收益率序列为统计（纯函数）
func aggregateReturns(name string, returns []float64) StrategyBacktestStat {
	st := StrategyBacktestStat{Strategy: name, Total: len(returns)}
	var winSum, lossSum float64
	for _, r := range returns {
		if r > 0 {
			st.Wins++
			winSum += r
		} else if r < 0 {
			lossSum += -r
		}
	}
	if st.Total > 0 {
		st.WinRate = float64(st.Wins) / float64(st.Total)
	}
	if st.Wins > 0 {
		st.AvgWin = winSum / float64(st.Wins)
	}
	if losses := st.Total - st.Wins; losses > 0 {
		st.AvgLoss = lossSum / float64(losses)
	}
	if st.AvgLoss > 0 {
		st.ProfitLossRatio = st.AvgWin / st.AvgLoss
	}
	return st
}

// BacktestStrategies 信号级回测：对 codes 在 [startDate, endDate] 内逐交易日跑 5 策略，
// 信号日收盘价买入、持有 holdingDays 天，走现有 backtest.Engine 的涨跌停/基准规则。
// 注：这是重计算（全市场约百万次 Select），并发 8，进度经 onProgress 回调。
// 已知限制：strategy.LoadDailyBars 固定拉取距今 400 自然日的窗口，startDate 早于该窗口起点时
// 早期信号缺少 120 根指标预热数据，会被各策略 MinBars 门槛自然跳过。
func BacktestStrategies(ctx context.Context, codes []string, startDate, endDate string,
	holdingDays int, onProgress func(done, total int)) ([]StrategyBacktestStat, error) {

	engine := backtest.NewEngine()
	strategies := strategy.Registry()
	returns := map[string][]float64{}
	var mu sync.Mutex
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	done := 0

	for _, code := range codes {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		wg.Add(1)
		go func(code string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			defer func() {
				mu.Lock()
				done++
				if onProgress != nil {
					onProgress(done, len(codes))
				}
				mu.Unlock()
			}()
			// 需要 startDate 之前至少 120 根做指标预热：loader 固定拉距今 400 自然日
			bars, err := strategy.LoadDailyBars(ctx, code, 130)
			if err != nil || len(bars) == 0 {
				return
			}
			for i := range bars {
				d := bars[i].TradeDate
				if d < startDate || d > endDate {
					continue
				}
				window := bars[:i+1]
				for _, s := range strategies {
					if len(window) < s.MinBars() {
						continue
					}
					sig := s.Select(window, "", d)
					if sig == nil {
						continue
					}
					r, err := engine.Run(ctx, backtest.Input{
						StockCode: code, SignalDate: d, EntryPrice: sig.Close,
						HoldingDays: holdingDays, StopLoss: 0.08, Adjusted: true,
					})
					if err != nil || r == nil {
						continue
					}
					mu.Lock()
					returns[s.Name()] = append(returns[s.Name()], r.TotalReturn)
					mu.Unlock()
				}
			}
		}(code)
	}
	wg.Wait()

	names := make([]string, 0, len(returns))
	for name := range returns {
		names = append(names, name)
	}
	sort.Strings(names)
	stats := make([]StrategyBacktestStat, 0, len(names))
	for _, name := range names {
		stats = append(stats, aggregateReturns(name, returns[name]))
		logger.SugaredLogger.Infof("khunter 回测 %s: %d 笔 胜率 %.1f%% 盈亏比 %.2f",
			name, len(returns[name]), stats[len(stats)-1].WinRate*100, stats[len(stats)-1].ProfitLossRatio)
	}
	return stats, nil
}
