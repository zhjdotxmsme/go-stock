package backtest

import (
	"context"
	"fmt"

	"go-stock/backend/data/datasource"
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
}

type Engine struct {
	store *datasource.KLineStore
}

func NewEngine() *Engine {
	return &Engine{store: datasource.NewKLineStore()}
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

	bars, err := e.store.QueryKLines(ctx, in.StockCode, "day", in.SignalDate, "", in.Adjusted)
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
		bars = datasource.BarsFromKLineData(in.StockCode, "day", "backtest", in.Adjusted, data)
		if len(bars) == 0 {
			return nil, fmt.Errorf("no kline data for %s from %s after conversion", in.StockCode, in.SignalDate)
		}
	}

	signalBar := bars[0]
	entry := in.EntryPrice
	if entry <= 0 {
		entry = signalBar.Close
	}

	maxPrice := entry
	exitPrice := entry
	exitIdx := 0
	warning := ""

	for i := 1; i < len(bars) && i <= in.HoldingDays; i++ {
		bar := bars[i]
		if bar.High > maxPrice {
			maxPrice = bar.High
		}

		if in.StopLoss > 0 && bar.Low <= entry*(1-in.StopLoss) {
			exitPrice = entry * (1 - in.StopLoss)
			exitIdx = i
			break
		}
		if in.StopProfit > 0 && bar.High >= entry*(1+in.StopProfit) {
			exitPrice = entry * (1 + in.StopProfit)
			exitIdx = i
			break
		}

		exitPrice = bar.Close
		exitIdx = i
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
	var maxDD float64
	if maxPrice > 0 {
		maxDD = (maxPrice - exitPrice) / maxPrice
	}
	if maxDD < 0 {
		maxDD = 0
	}

	benchRet := 0.0
	benchBars, _ := e.store.QueryKLines(ctx, in.Benchmark, "day", in.SignalDate, bars[exitIdx].TradeDate, in.Adjusted)
	if len(benchBars) >= 2 {
		benchRet = (benchBars[len(benchBars)-1].Close - benchBars[0].Close) / benchBars[0].Close
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
	}, nil
}
