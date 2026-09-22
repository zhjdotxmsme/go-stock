package signal

// engine.go 信号判定入口。

import (
	"go-stock/backend/data/datasource"
)

// MinBars 判定的最小K线数（MACD 预热需要，低于此值返回空）。
const MinBars = 35

// Evaluate 判定 bars 在 asOfIdx 时点命中的全部信号。
// asOfIdx < 0 表示最新一根；bars 按时间升序。
// 数据不足或 asOfIdx 越界时返回空列表（不报错，降级语义）。
func (e *Engine) Evaluate(code string, bars []datasource.KLineBar, asOfIdx int) []SignalMatch {
	n := len(bars)
	if asOfIdx < 0 {
		asOfIdx = n - 1
	}
	if asOfIdx >= n {
		asOfIdx = n - 1
	}
	if asOfIdx+1 < MinBars {
		return nil
	}
	// 截断到时点（历史时点判定不应看到未来数据）
	used := bars[:asOfIdx+1]

	ctx := &evalCtx{bars: used, idx: asOfIdx}
	m := asOfIdx + 1
	ctx.open = make([]float64, m)
	ctx.high = make([]float64, m)
	ctx.low = make([]float64, m)
	ctx.close = make([]float64, m)
	ctx.vol = make([]float64, m)
	for i, b := range used {
		ctx.open[i] = b.Open
		ctx.high[i] = b.High
		ctx.low[i] = b.Low
		ctx.close[i] = b.Close
		ctx.vol[i] = float64(b.Volume)
	}
	ctx.ma5 = smaSeries(ctx.close, 5)
	ctx.ma10 = smaSeries(ctx.close, 10)
	ctx.ma20 = smaSeries(ctx.close, 20)
	ctx.ma60 = smaSeries(ctx.close, 60)
	ctx.dif, ctx.dea = macdSeries(ctx.close)
	ctx.k, ctx.d = kdjSeries(ctx.high, ctx.low, ctx.close, 9)

	// 资金流（仅注册了 NeedFunds 信号时才拉取）
	if e.Funds != nil {
		ctx.fundsInflow, ctx.fundsOK = e.Funds(code)
	}

	matches := make([]SignalMatch, 0, 4)
	for _, def := range registry {
		if def.NeedFunds && !ctx.fundsOK {
			continue // 资金流不可用，降级跳过
		}
		if def.detect(ctx) {
			matches = append(matches, SignalMatch{
				Key:       def.Key,
				Name:      def.Name,
				Category:  def.Category,
				Direction: def.Direction,
				Tip:       def.Tip,
			})
		}
	}
	return matches
}

// EvaluateLatest 判定最新时点的信号（Evaluate 的便捷封装）。
func (e *Engine) EvaluateLatest(code string, bars []datasource.KLineBar) []SignalMatch {
	return e.Evaluate(code, bars, -1)
}
