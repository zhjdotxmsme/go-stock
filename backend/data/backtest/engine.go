package backtest

import (
	"context"
	"fmt"
	"math"

	"go-stock/backend/data/datasource"
	"go-stock/backend/data/trading"
	"go-stock/backend/models"
)

type Input struct {
	StockCode    string
	SignalDate   string
	SignalRating string
	EntryPrice   float64
	HoldingDays  int
	StopLoss     float64
	StopProfit   float64
	Adjusted     bool
	Benchmark    string
	Shares       int  // 持仓股数（手数 = Shares / 100），默认 100
	IsST         bool // 是否为 ST 股票，默认 false
}

type Result struct {
	StockCode       string
	SignalDate      string
	EntryPrice      float64
	ExitPrice       float64
	ExitDate        string
	HoldingDays     int
	TotalReturn     float64
	MaxDrawdown     float64
	BenchmarkReturn float64
	Alpha           float64
	Win             bool
	SlippageWarning string
	DailyValues     []float64 `json:"dailyValues"`
	BenchmarkValues []float64 `json:"benchmarkValues"`
}

type Engine struct {
	store *datasource.KLineStore
}

func NewEngine() *Engine {
	return &Engine{store: datasource.NewKLineStore()}
}

// queryCacheKLines 按候选存储格式依次做精确等值查询，命中第一种有数据的格式即返回。
// 历史数据可能同时存在裸码（600519）与前缀格式（sh600519）两种存量记录，
// 候选格式均为等值条件，仍命中 kline_bars 的复合索引。
func (e *Engine) queryCacheKLines(ctx context.Context, code, period, startDate, endDate string, adjusted bool) ([]models.KLineBar, error) {
	var lastErr error
	for _, candidate := range StockCodeCandidates(code) {
		bars, err := e.store.QueryKLines(ctx, candidate, period, startDate, endDate, adjusted)
		if err != nil {
			lastErr = err
			continue
		}
		if len(bars) > 0 {
			return bars, nil
		}
	}
	return nil, lastErr
}

// signalPriceLimit 返回信号日按板块规则的真实涨跌停价（与 trading.DailyPriceLimit 同一口径）。
// 代码无法识别（非 6 位数字，如测试 mock）时按主板近似：ST 5%、其余 10%，
// 仍返回 ok=true 以维持旧行为；prevClose <= 0 时 ok=false，由调用方跳过判定。
func (e *Engine) signalPriceLimit(in Input, prevClose float64) (upPrice, downPrice float64, ok bool) {
	if prevClose <= 0 {
		return 0, 0, false
	}
	if pl, ok := trading.DailyPriceLimit(in.StockCode, prevClose, in.IsST); ok {
		return pl.UpPrice, pl.DownPrice, true
	}
	rate := 0.10
	if in.IsST {
		rate = 0.05
	}
	return math.Round(prevClose*(1+rate)*100) / 100,
		math.Round(prevClose*(1-rate)*100) / 100, true
}

func (e *Engine) Run(ctx context.Context, in Input) (*Result, error) {
	if in.HoldingDays <= 0 {
		in.HoldingDays = 20
	}
	if in.StopLoss <= 0 {
		in.StopLoss = 0.08
	}
	if in.StopProfit <= 0 {
		in.StopProfit = 0.20
	}
	if in.Benchmark == "" {
		in.Benchmark = "sh510300"
	}
	if in.Shares <= 0 {
		in.Shares = 100
	}

	// 最小交易单位校验（A 股 1 手 = 100 股）
	if in.Shares%100 != 0 {
		return nil, fmt.Errorf("invalid lot size: %d shares (must be multiple of 100)", in.Shares)
	}

	// endDate 传 "" 时 SQLite BETWEEN 'date' AND '' 恒为空集，这里用上界日期保证缓存可查
	bars, err := e.queryCacheKLines(ctx, in.StockCode, "day", in.SignalDate, "9999-12-31", in.Adjusted)
	if err != nil || len(bars) == 0 {
		router := datasource.GetRouter()
		data, fallbackErr := router.GetKLine(ctx, in.StockCode, "day", 500)
		if fallbackErr != nil {
			if err != nil {
				return nil, fmt.Errorf("cache miss for %s from %s and fallback failed: %w (cache: %w)", in.StockCode, in.SignalDate, fallbackErr, err)
			}
			return nil, fmt.Errorf("no kline data for %s from %s: %w", in.StockCode, in.SignalDate, fallbackErr)
		}
		if data == nil || len(data.Bars) == 0 {
			return nil, fmt.Errorf("no kline data for %s from %s", in.StockCode, in.SignalDate)
		}
		allBars := datasource.BarsFromKLineData(in.StockCode, "day", "backtest", in.Adjusted, data)
		if len(allBars) == 0 {
			return nil, fmt.Errorf("no kline data for %s from %s after conversion", in.StockCode, in.SignalDate)
		}
		// 回退路径获取了 500 条 bars（从最早期开始），需过滤到 signalDate 之后
		startIdx := 0
		for i, bar := range allBars {
			if bar.TradeDate >= in.SignalDate {
				startIdx = i
				break
			}
		}
		// 在过滤前保存前一条 bar 的收盘价，用于填充 signal bar 的 PrevClose
		if startIdx > 0 && allBars[startIdx].PrevClose == 0 && allBars[startIdx-1].Close > 0 {
			allBars[startIdx].PrevClose = allBars[startIdx-1].Close
		}
		bars = allBars[startIdx:]
		if len(bars) == 0 {
			return nil, fmt.Errorf("no kline data for %s from %s after filtering", in.StockCode, in.SignalDate)
		}
	}

	// DB 路径：如果第一条 bar 没有 PrevClose，尝试查询前一日记录补充
	if len(bars) > 0 && bars[0].PrevClose == 0 {
		prevBars, _ := e.queryCacheKLines(ctx, in.StockCode, "day", "1900-01-01", in.SignalDate, in.Adjusted)
		if len(prevBars) >= 2 {
			bars[0].PrevClose = prevBars[len(prevBars)-2].Close
		}
	}

	signalBar := bars[0]

	// 涨跌停约束检查：信号日收于板块真实涨停/跌停价（0.01 元取整）禁止买入
	if upPrice, downPrice, ok := e.signalPriceLimit(in, signalBar.PrevClose); ok {
		switch {
		case signalBar.Close >= upPrice-0.001:
			return nil, fmt.Errorf("price limit on signal date %s for %s: buy-day limit-up (close=%.2f prev=%.2f limit=%.2f)",
				in.SignalDate, in.StockCode, signalBar.Close, signalBar.PrevClose, upPrice)
		case signalBar.Close <= downPrice+0.001:
			return nil, fmt.Errorf("price limit on signal date %s for %s: buy-day limit-down (close=%.2f prev=%.2f limit=%.2f)",
				in.SignalDate, in.StockCode, signalBar.Close, signalBar.PrevClose, downPrice)
		}
	}
	// PrevClose 无效时跳过检查（旧数据无此字段），回测口径保守允许交易；
	// 狩猎场确认链路不得沿用该回退：缺前收必须拒绝确认（在后续确认任务接入）。

	entry := in.EntryPrice
	if entry <= 0 {
		entry = signalBar.Close
	}

	exitPrice := entry
	exitIdx := 0
	warning := ""

	// T+1 约束：bars[0] 为买入日（信号日），从 i=1 开始检查退出
	// 确保买入当日不可卖出
	var dailyValues []float64
	// path 是实际成交能走到的价格：止损/止盈日用成交价，其余用收盘。
	// 最大回撤按这条路径的峰谷计算，中途下跌后收复仍会计入。
	var path []float64
	for i := 1; i < len(bars) && i <= in.HoldingDays; i++ {
		bar := bars[i]
		dailyValues = append(dailyValues, (bar.Close-entry)/entry)

		if in.StopLoss > 0 && bar.Low <= entry*(1-in.StopLoss) {
			exitPrice = entry * (1 - in.StopLoss)
			exitIdx = i
			path = append(path, exitPrice)
			break
		}
		if in.StopProfit > 0 && bar.High >= entry*(1+in.StopProfit) {
			exitPrice = entry * (1 + in.StopProfit)
			exitIdx = i
			path = append(path, exitPrice)
			break
		}

		exitPrice = bar.Close
		exitIdx = i
		path = append(path, bar.Close)
	}

	if exitIdx == 0 {
		return nil, fmt.Errorf("no exit found for %s (signal=%s, holding=%d)", in.StockCode, in.SignalDate, in.HoldingDays)
	}

	if signalBar.Close >= signalBar.High*0.999 {
		warning = "buy-day limit-up"
	}
	if bars[exitIdx].Close <= bars[exitIdx].Low*1.001 {
		if warning != "" {
			warning += "; "
		}
		warning += "sell-day limit-down"
	}

	ret := (exitPrice - entry) / entry
	maxDD := pathMaxDrawdown(entry, path)

	benchRet := 0.0
	var benchValues []float64
	benchBars, _ := e.queryCacheKLines(ctx, in.Benchmark, "day", in.SignalDate, bars[exitIdx].TradeDate, in.Adjusted)
	if len(benchBars) >= 2 {
		benchRet = (benchBars[len(benchBars)-1].Close - benchBars[0].Close) / benchBars[0].Close
		for i := 1; i < len(benchBars) && i <= exitIdx; i++ {
			benchValues = append(benchValues, (benchBars[i].Close-benchBars[0].Close)/benchBars[0].Close)
		}
	}

	return &Result{
		StockCode:       in.StockCode,
		SignalDate:      in.SignalDate,
		EntryPrice:      entry,
		ExitPrice:       exitPrice,
		ExitDate:        bars[exitIdx].TradeDate,
		HoldingDays:     exitIdx,
		TotalReturn:     ret,
		MaxDrawdown:     maxDD,
		BenchmarkReturn: benchRet,
		Alpha:           ret - benchRet,
		Win:             ret > 0,
		SlippageWarning: warning,
		DailyValues:     dailyValues,
		BenchmarkValues: benchValues,
	}, nil
}

// pathMaxDrawdown 是持有期内权益曲线的最大回撤（相对买入价）。
// 只看「最高价到卖出价」会把中途下跌后收复的交易记成接近 0。
func pathMaxDrawdown(entry float64, prices []float64) float64 {
	if entry <= 0 {
		return 0
	}
	peak := 1.0
	maxDD := 0.0
	for _, px := range prices {
		if px <= 0 {
			continue
		}
		eq := px / entry
		if eq > peak {
			peak = eq
		}
		if dd := (peak - eq) / peak; dd > maxDD {
			maxDD = dd
		}
	}
	return maxDD
}
