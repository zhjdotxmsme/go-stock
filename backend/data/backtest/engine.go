package backtest

import (
	"context"
	"fmt"
	"strings"

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

// limitUpDown 返回给定 Input 的涨跌停阈值系数。
// 返回 (upThreshold, downThreshold)，例如 (1.099, 0.901) 表示 10% 涨跌停。
func (e *Engine) limitUpDown(in Input) (upFactor, downFactor float64) {
	if in.IsST {
		return 1.049, 0.951 // ST 股 5%
	}
	// 根据代码前缀判断板块
	code := in.StockCode
	if strings.HasPrefix(code, "sh688") || strings.HasPrefix(code, "sz300") || strings.HasPrefix(code, "sz301") {
		return 1.199, 0.801 // 科创板/创业板 20%
	}
	if strings.HasPrefix(code, "sh4") || strings.HasPrefix(code, "sh8") || strings.HasPrefix(code, "sz8") || strings.HasPrefix(code, "bj8") {
		return 1.299, 0.701 // 北交所 30%
	}
	return 1.099, 0.901 // 主板 10%
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
		prevBars, _ := e.store.QueryKLines(ctx, in.StockCode, "day", "1900-01-01", in.SignalDate, in.Adjusted)
		if len(prevBars) >= 2 {
			bars[0].PrevClose = prevBars[len(prevBars)-2].Close
		}
	}

	signalBar := bars[0]

	// 涨跌停约束检查：信号日若涨停/跌停，禁止买入
	upFactor, downFactor := e.limitUpDown(in)

	if signalBar.PrevClose > 0 {
		switch {
		case signalBar.Close >= signalBar.PrevClose*upFactor:
			return nil, fmt.Errorf("price limit on signal date %s for %s: buy-day limit-up (close=%.2f >= prev=%.2f*%.4f)",
				in.SignalDate, in.StockCode, signalBar.Close, signalBar.PrevClose, upFactor)
		case signalBar.Close <= signalBar.PrevClose*downFactor:
			return nil, fmt.Errorf("price limit on signal date %s for %s: buy-day limit-down (close=%.2f <= prev=%.2f*%.4f)",
				in.SignalDate, in.StockCode, signalBar.Close, signalBar.PrevClose, downFactor)
		}
	}
	// PrevClose == 0 时跳过检查（旧数据无此字段），保守允许交易

	entry := in.EntryPrice
	if entry <= 0 {
		entry = signalBar.Close
	}

	maxPrice := entry
	exitPrice := entry
	exitIdx := 0
	warning := ""

	// T+1 约束：bars[0] 为买入日（信号日），从 i=1 开始检查退出
	// 确保买入当日不可卖出
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
