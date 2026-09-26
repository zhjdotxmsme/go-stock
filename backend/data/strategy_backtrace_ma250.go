package data

import (
	"go-stock/backend/util/timeutil"
)

// strategy_backtrace_ma250.go —— 回踩年线策略（移植自 InStock backtrace_ma250）。
//
// 形态定义（窗口 = 最近 60 个交易日）：
//  1. 前段（区间最高收盘价之前）：股价由年线 MA250 以下向上突破；
//  2. 后段（最高收盘日及之后）：收盘价全部站在年线之上（回踩不破）；
//  3. 后段最低收盘日距最高收盘日 10~50 个自然日（InStock 用自然日口径）；
//  4. 回踩伴随缩量：最高日成交量 / 后段最低日成交量 > 2，且 后段最低价/最高价 < 0.8。
//
// 评分（满分 100）：突破年线 25 + 回踩不破 25 + 时间窗 20 + 缩量回踩 30。
// 需要至少 250 根 K 线计算 MA250，不足返回「K线数据不足」。
type BacktraceMA250Strategy struct{}

func (s *BacktraceMA250Strategy) Name() string { return "回踩年线" }
func (s *BacktraceMA250Strategy) Code() string { return "backtrace_ma250" }
func (s *BacktraceMA250Strategy) Description() string {
	return "放量突破年线 MA250 后缩量回踩不破，中线强势确认"
}

func (s *BacktraceMA250Strategy) Score(ctx *StrategyContext) *StrategyResult {
	fail := &StrategyResult{Score: 0, Factors: map[string]float64{}, Signal: ""}
	n := len(ctx.CloseP)
	if n < 260 || len(ctx.KLines) < n || len(ctx.Volume) < n {
		fail.Signal = "K线数据不足（需≥260根）"
		return fail
	}

	ma250 := comboSMASeries(ctx.CloseP, 250)
	winStart := n - 60

	// 区间最高/最低收盘（独立跟踪，修正 InStock if/elif 互斥漏记）
	hiIdx, loIdx := winStart, winStart
	for i := winStart; i < n; i++ {
		if ctx.CloseP[i] > ctx.CloseP[hiIdx] {
			hiIdx = i
		}
		if ctx.CloseP[i] < ctx.CloseP[loIdx] {
			loIdx = i
		}
	}
	if hiIdx == winStart {
		fail.Signal = "区间最高点在窗口首日，无前段突破过程"
		return fail
	}

	factors := map[string]float64{}

	// 条件1：前段由年线以下向上突破（前段首日收盘<年线，最高日前一日收盘>年线）
	if ctx.CloseP[winStart] < ma250[winStart] && ctx.CloseP[hiIdx-1] > ma250[hiIdx-1] {
		factors["cross_above_ma250"] = 25
	}

	// 条件2：后段全部收于年线之上，并找后段最低收盘日
	holdAbove := true
	lowBackIdx := hiIdx
	for i := hiIdx; i < n; i++ {
		if ctx.CloseP[i] < ma250[i] {
			holdAbove = false
			break
		}
		if ctx.CloseP[i] < ctx.CloseP[lowBackIdx] {
			lowBackIdx = i
		}
	}
	if holdAbove {
		factors["hold_above_ma250"] = 25
	}

	// 条件3：后段最低日与最高日相差 10~50 个自然日（沿用 InStock 自然日口径）
	hiDate, errHi := timeutil.ParseDate(ctx.KLines[hiIdx].Day)
	loDate, errLo := timeutil.ParseDate(ctx.KLines[lowBackIdx].Day)
	if errHi == nil && errLo == nil {
		days := loDate.Sub(hiDate).Hours() / 24
		if days >= 10 && days <= 50 {
			factors["pullback_window"] = 20
		}
	}

	// 条件4：缩量回踩（最高日量/最低日量 > 2 且 最低收盘/最高收盘 < 0.8）
	if ctx.Volume[lowBackIdx] > 0 && ctx.CloseP[hiIdx] > 0 {
		volRatio := ctx.Volume[hiIdx] / ctx.Volume[lowBackIdx]
		backRatio := ctx.CloseP[lowBackIdx] / ctx.CloseP[hiIdx]
		if volRatio > 2 && backRatio < 0.8 {
			factors["shrink_volume_pullback"] = 30
		}
	}

	score := 0.0
	for _, v := range factors {
		score += v
	}
	res := &StrategyResult{Score: score, Factors: factors}
	switch {
	case score >= 100:
		res.Signal = "突破年线后缩量回踩不破，回踩年线形态成立"
	case score >= 50:
		res.Signal = "年线附近运行，形态条件部分满足"
	default:
		res.Signal = "未形成回踩年线形态"
	}
	return res
}
