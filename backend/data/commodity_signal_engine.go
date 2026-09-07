package data

// 商品策略信号引擎（纯函数，零网络依赖，可确定性单测）。
// 设计规格: .openteams/specs/2026-09-07-commodity-optimization-design.html 第 3 节
//
// 5 个信号基于日 K + 实时盘面计算：
//   趋势(MA20/MA60 交叉) · 动量(20/60 日收益) · 突破(唐奇安20日) ·
//   Carry(近远月价差) · 持仓四象限(OI)
//
// 综合分 = 加权和（缺失信号按归一化权重剔除），取值 [-1, +1]。

import (
	"fmt"
	"math"
)

// ---------- 信号等级 ----------

type TrendLevel string

const (
	TrendBull       TrendLevel = "bull"       // 多头
	TrendBear       TrendLevel = "bear"       // 空头
	TrendTransition TrendLevel = "transition" // 过渡（交叉与价格位置不一致）
)

type MomentumLevel string

const (
	MomentumStrongBull MomentumLevel = "strong_bull" // 强多头
	MomentumBull       MomentumLevel = "bull"        // 多头
	MomentumNeutral    MomentumLevel = "neutral"     // 中性
	MomentumBear       MomentumLevel = "bear"        // 空头
	MomentumStrongBear MomentumLevel = "strong_bear" // 强空头
)

type BreakoutLevel string

const (
	BreakoutUp    BreakoutLevel = "up"    // 多头突破（近 20 日高点区）
	BreakoutRange BreakoutLevel = "range" // 区间
	BreakoutDown  BreakoutLevel = "down"  // 空头突破（近 20 日低点区）
)

type CarryLevel string

const (
	CarryBackwardation CarryLevel = "backwardation" // 贴水（远月 < 近月*0.99）
	CarryNeutral       CarryLevel = "neutral"       // 期限结构中性
	CarryContango      CarryLevel = "contango"      // 升水（远月 > 近月*1.01）
)

type OIQuadrant string

const (
	OIQLongAttack  OIQuadrant = "long_attack"  // 涨+仓增：多头进攻
	OIQShortCover  OIQuadrant = "short_cover"  // 涨+仓减：空头回补
	OIQShortAttack OIQuadrant = "short_attack" // 跌+仓增：空头进攻
	OIQLongExit    OIQuadrant = "long_exit"    // 跌+仓减：多头离场
)

// ---------- 计算参数（阈值，便于统一调整与测试） ----------

const (
	momentum20dThreshold = 3.0 // 20 日收益 ±3% 视为有效方向
	momentum60dThreshold = 6.0 // 60 日收益 ±6% 视为有效方向
	donchianWindow       = 20  // 唐奇安通道窗口（日）
	donchianTolerance    = 0.01 // 突破判定容差 ±1%
	carryTolerance       = 1.0  // carry 判定阈值 ±1%
)

// HighLowClose 是引擎所需的最小 K 线切片（避免引擎依赖 datasource.KLineBar）。
type HighLowClose struct {
	High  float64
	Low   float64
	Close float64
}

// ---------- 纯信号计算 ----------

// ma 返回最后 n 个值的均值。
func ma(values []float64, n int) (float64, bool) {
	if len(values) < n || n <= 0 {
		return 0, false
	}
	window := values[len(values)-n:]
	sum := 0.0
	for _, v := range window {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, false
		}
		sum += v
	}
	return sum / float64(n), true
}

// ComputeTrend 趋势信号：MA20 vs MA60 交叉方向 + 收盘价相对 MA20 的位置。
// 规则：MA20>MA60 且 收盘≥MA20 → 多头；MA20<MA60 且 收盘≤MA20 → 空头；其余 → 过渡。
// 需要至少 60 根日 K。ok=false 表示数据不足。
func ComputeTrend(closes []float64) (TrendLevel, bool) {
	ma20, ok20 := ma(closes, 20)
	ma60, ok60 := ma(closes, 60)
	if !ok20 || !ok60 {
		return "", false
	}
	last := closes[len(closes)-1]
	switch {
	case ma20 > ma60 && last >= ma20:
		return TrendBull, true
	case ma20 < ma60 && last <= ma20:
		return TrendBear, true
	default:
		return TrendTransition, true
	}
}

// ComputeMomentum 动量信号：20 日与 60 日收益率的符号与幅度分档。
// 规则（pct = 收益率百分比）：
//
//	r20 与 r60 同号且均超阈值（±3% / ±6%）→ 强方向
//	r20 与 r60 同号（未达强档）      → 方向
//	其余（含符号相反）              → 中性
//
// 需要至少 61 根日 K。ok=false 表示数据不足。
func ComputeMomentum(closes []float64) (MomentumLevel, bool) {
	n := len(closes)
	if n < 61 {
		return "", false
	}
	last := closes[n-1]
	anchor20 := closes[n-21]
	anchor60 := closes[n-61]
	if anchor20 <= 0 || anchor60 <= 0 {
		return "", false
	}
	r20 := (last/anchor20 - 1) * 100
	r60 := (last/anchor60 - 1) * 100

	if r20 >= momentum20dThreshold && r60 >= momentum60dThreshold {
		return MomentumStrongBull, true
	}
	if r20 <= -momentum20dThreshold && r60 <= -momentum60dThreshold {
		return MomentumStrongBear, true
	}
	switch {
	case r20 > 0 && r60 > 0:
		return MomentumBull, true
	case r20 < 0 && r60 < 0:
		return MomentumBear, true
	default:
		return MomentumNeutral, true
	}
}

// ComputeBreakout 突破信号（唐奇安 20 日）：以最近 20 根（不含当根）K 线的
// 最高/最低价为通道，收盘价进入 ±1% 容差带内即判定突破。
// 需要至少 21 根日 K。ok=false 表示数据不足。
func ComputeBreakout(bars []HighLowClose) (BreakoutLevel, bool) {
	n := len(bars)
	if n < donchianWindow+1 {
		return "", false
	}
	cur := bars[n-1]
	high := 0.0
	low := math.MaxFloat64
	for i := n - donchianWindow - 1; i <= n-2; i++ {
		if bars[i].High > high {
			high = bars[i].High
		}
		if bars[i].Low < low {
			low = bars[i].Low
		}
	}
	if high <= 0 || low <= 0 {
		return "", false
	}
	switch {
	case cur.Close >= high*(1-donchianTolerance):
		return BreakoutUp, true
	case cur.Close <= low*(1+donchianTolerance):
		return BreakoutDown, true
	default:
		return BreakoutRange, true
	}
}

// ComputeCarry 期限结构信号：近远月价差占近月比例的百分比。
// 贴水 ≤ -1% → backwardation；升水 ≥ +1% → contango；其间 → neutral。
// ok=false 表示无效报价（近月无值/非法）。
func ComputeCarry(nearMonth, farMonth float64) (CarryLevel, bool) {
	if nearMonth <= 0 || farMonth <= 0 {
		return "", false
	}
	spreadPct := (farMonth - nearMonth) / nearMonth * 100
	switch {
	case spreadPct <= -carryTolerance:
		return CarryBackwardation, true
	case spreadPct >= carryTolerance:
		return CarryContango, true
	default:
		return CarryNeutral, true
	}
}

// ComputeOIQuadrant 持仓四象限：当日价格涨跌 × OI 增减。
// 价格与 OI 变化均恰为 0 时视为无信号（返回 ok=false）。
func ComputeOIQuadrant(priceChangePct, oiChange float64) (OIQuadrant, bool) {
	if priceChangePct == 0 && oiChange == 0 {
		return "", false
	}
	up := priceChangePct >= 0
	oiUp := oiChange >= 0
	switch {
	case up && oiUp:
		return OIQLongAttack, true
	case up && !oiUp:
		return OIQShortCover, true
	case !up && oiUp:
		return OIQShortAttack, true
	default:
		return OIQLongExit, true
	}
}

// ---------- 综合分 ----------

// SignalValue 返回各信号等级对应的强度值（[-1,+1]），供综合分加权。
func SignalValue(kind string, level string) (float64, bool) {
	switch kind {
	case "trend":
		switch TrendLevel(level) {
		case TrendBull:
			return 1, true
		case TrendBear:
			return -1, true
		case TrendTransition:
			return 0, true
		}
	case "momentum":
		switch MomentumLevel(level) {
		case MomentumStrongBull:
			return 1, true
		case MomentumBull:
			return 0.5, true
		case MomentumNeutral:
			return 0, true
		case MomentumBear:
			return -0.5, true
		case MomentumStrongBear:
			return -1, true
		}
	case "breakout":
		switch BreakoutLevel(level) {
		case BreakoutUp:
			return 1, true
		case BreakoutRange:
			return 0, true
		case BreakoutDown:
			return -1, true
		}
	case "carry":
		switch CarryLevel(level) {
		case CarryBackwardation:
			return 0.5, true // 贴水偏多（结构性偏多，半权表达）
		case CarryNeutral:
			return 0, true
		case CarryContango:
			return -0.5, true // 升水偏空
		}
	case "oi":
		switch OIQuadrant(level) {
		case OIQLongAttack:
			return 1, true
		case OIQShortCover:
			return 0.5, true
		case OIQShortAttack:
			return -1, true
		case OIQLongExit:
			return -0.5, true
		}
	}
	return 0, false
}

// signalWeights 综合分权重（设计规格固定值，和为 1.0）。
var signalWeights = map[string]float64{
	"trend":    0.30,
	"momentum": 0.25,
	"breakout": 0.20,
	"carry":    0.15,
	"oi":       0.10,
}

// SignalParts 参与综合分的信号集合；nil 表示该信号缺失（数据不足/非期货标的）。
type SignalParts struct {
	Trend    *string
	Momentum *string
	Breakout *string
	Carry    *string
	OI       *string
}

// CompositeScore 按权重计算综合分。缺失信号自动从权重集中剔除并重归一化。
// 返回 (score, 参与计算信号数, error)；全部缺失时报错。score 钳制在 [-1, +1]。
func CompositeScore(parts SignalParts) (float64, int, error) {
	type item struct {
		key   string
		value float64
	}
	items := make([]item, 0, 5)
	add := func(key string, ptr *string) {
		if ptr == nil || *ptr == "" {
			return
		}
		v, ok := SignalValue(key, *ptr)
		if !ok {
			return
		}
		items = append(items, item{key: key, value: v})
	}
	add("trend", parts.Trend)
	add("momentum", parts.Momentum)
	add("breakout", parts.Breakout)
	add("carry", parts.Carry)
	add("oi", parts.OI)

	if len(items) == 0 {
		return 0, 0, fmt.Errorf("no signal available for composite score")
	}
	weighted := 0.0
	totalW := 0.0
	for _, it := range items {
		w, ok := signalWeights[it.key]
		if !ok {
			continue
		}
		weighted += w * it.value
		totalW += w
	}
	if totalW == 0 {
		return 0, 0, fmt.Errorf("zero weight in composite score")
	}
	score := weighted / totalW
	if score > 1 {
		score = 1
	}
	if score < -1 {
		score = -1
	}
	return score, len(items), nil
}

// verdictThreshold 综合分结论阈值：|score| ≥ 0.25 才算有方向。
const verdictThreshold = 0.25

// Verdict 由综合分得出结论标签。
func Verdict(score float64) string {
	switch {
	case score >= verdictThreshold:
		return "偏多"
	case score <= -verdictThreshold:
		return "偏空"
	default:
		return "中性"
	}
}
