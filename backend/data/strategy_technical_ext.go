package data

// strategy_technical_ext.go —— 每日选股引擎的 6 个单指标策略扩展。
//
// 全部基于 indicator 包纯函数指标；指标序列预热期为 math.NaN()，
// 取值一律走 indicator.LastValid / indicator.At 的 ok 判断，
// 数据不足 / 指标无效时统一返回 Score 0 + Signal「K线数据不足」，绝不 panic。
//
// 本文件只实现策略类型本身；注册到引擎由主线程负责。

import (
	"fmt"
	"math"

	"go-stock/backend/data/indicator"
)

// ---------------- 共享小工具（ext 前缀，避免与包内其他文件冲突） ----------------

// extInsufficient 数据不足/指标无效时的统一结果。
func extInsufficient() *StrategyResult {
	return &StrategyResult{Score: 0, Factors: map[string]float64{}, Signal: "K线数据不足"}
}

// extClamp01 截断到 [0,1]（NaN 归 0）。
func extClamp01(v float64) float64 {
	if math.IsNaN(v) || v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// extBool01 布尔转 0/1 子分数。
func extBool01(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// extOverride 读取 Overrides 中的正数覆盖值，缺失或非正返回默认值 def。
func extOverride(ctx *StrategyContext, key string, def float64) float64 {
	if ctx != nil && ctx.Overrides != nil {
		if v, ok := ctx.Overrides[key]; ok && v > 0 && !math.IsNaN(v) {
			return v
		}
	}
	return def
}

// extFlipAge 在 bull 序列中寻找最近一次 false→true 翻转，返回其距最新K线的偏移
// （0=最新一根）；window 根内未找到返回 ok=false。window 以内即「≤N 根内翻多」。
func extFlipAge(bull []bool, window int) (age int, ok bool) {
	n := len(bull)
	if n < 2 {
		return 0, false
	}
	if window > n-1 {
		window = n - 1
	}
	for off := 0; off <= window; off++ {
		i := n - 1 - off
		if i-1 < 0 {
			break
		}
		if bull[i] && !bull[i-1] {
			return off, true
		}
	}
	return 0, false
}

// extVolRatio 量比 = 当日成交量 / 前 20 根均量（不含当日，与 scoreVolumeFactor 口径一致）。
func extVolRatio(volume []float64) (float64, bool) {
	n := len(volume)
	if n < 21 {
		return 0, false
	}
	var sum float64
	for i := n - 21; i < n-1; i++ {
		sum += volume[i]
	}
	avg := sum / 20
	if math.IsNaN(avg) || avg <= 0 {
		return 0, false
	}
	return volume[n-1] / avg, true
}

// ---------------- 1. ADX趋势跟随 ----------------

// ADXTrendStrategy ADX(14)>阈值 且 +DI>−DI 且 ADX 较 3 根前上行时顺势给分；
// 分数按 ADX 强度 55-90。
type ADXTrendStrategy struct{}

func (s *ADXTrendStrategy) Name() string { return "ADX趋势跟随" }
func (s *ADXTrendStrategy) Code() string { return "adx_trend" }
func (s *ADXTrendStrategy) Description() string {
	return "基于ADX趋势强度与DI方向确认的顺势跟踪策略"
}

func (s *ADXTrendStrategy) Score(ctx *StrategyContext) *StrategyResult {
	if ctx == nil || len(ctx.CloseP) < 20 || len(ctx.HighP) < 20 || len(ctx.LowP) < 20 {
		return extInsufficient()
	}
	const period = 14
	threshold := extOverride(ctx, "adx_threshold", 25)

	adx, plusDI, minusDI := indicator.ADX(ctx.HighP, ctx.LowP, ctx.CloseP, period)
	adxNow, ok := indicator.LastValid(adx)
	if !ok {
		return extInsufficient()
	}
	plusNow, ok1 := indicator.At(plusDI, 0)
	minusNow, ok2 := indicator.At(minusDI, 0)
	adx3Ago, ok3 := indicator.At(adx, 3)
	if !ok1 || !ok2 {
		return extInsufficient()
	}

	adxStrength := extClamp01((adxNow - threshold) / math.Max(50-threshold, 1))
	diStrength := extClamp01((plusNow - minusNow) / 10)
	rising := ok3 && adxNow > adx3Ago

	factors := map[string]float64{
		"adxStrength": adxStrength,
		"diStrength":  diStrength,
		"trendRising": extBool01(rising),
	}

	// 方向成立的三条件：ADX 过阈值、+DI>−DI、ADX 3 日上行
	switch {
	case adxNow <= threshold:
		return &StrategyResult{Score: 0, Factors: factors,
			Signal: fmt.Sprintf("ADX %.1f≤%.0f·趋势强度不足", adxNow, threshold)}
	case plusNow <= minusNow:
		return &StrategyResult{Score: 0, Factors: factors,
			Signal: fmt.Sprintf("ADX %.1f·+DI %.1f≤−DI %.1f·空头占优", adxNow, plusNow, minusNow)}
	case !rising:
		return &StrategyResult{Score: 0, Factors: factors,
			Signal: fmt.Sprintf("ADX %.1f·+DI %.1f>−DI %.1f·较3根前未上行", adxNow, plusNow, minusNow)}
	}

	// 强度分：threshold → 55 分，50 → 90 分
	score := math.Round(55 + 35*adxStrength)
	return &StrategyResult{Score: score, Factors: factors,
		Signal: fmt.Sprintf("ADX %.1f·+DI %.1f>−DI %.1f·3日上行", adxNow, plusNow, minusNow)}
}

// ---------------- 2. 超级趋势翻多 ----------------

// SuperTrendFlipStrategy SuperTrend(10,3) bull 翻 true ≤3 根内且收盘>line；
// 分数 55-85 按翻转新鲜度，Signal 附止损参考（line 值）。
type SuperTrendFlipStrategy struct{}

func (s *SuperTrendFlipStrategy) Name() string { return "超级趋势翻多" }
func (s *SuperTrendFlipStrategy) Code() string { return "supertrend_flip" }
func (s *SuperTrendFlipStrategy) Description() string {
	return "基于SuperTrend翻多信号的短线趋势反转策略，附止损参考价"
}

func (s *SuperTrendFlipStrategy) Score(ctx *StrategyContext) *StrategyResult {
	if ctx == nil || len(ctx.CloseP) < 20 || len(ctx.HighP) < 20 || len(ctx.LowP) < 20 {
		return extInsufficient()
	}
	const atrPeriod = 10
	const mult = 3.0
	line, bull := indicator.SuperTrend(ctx.HighP, ctx.LowP, ctx.CloseP, atrPeriod, mult)
	lineNow, ok := indicator.LastValid(line)
	if !ok {
		return extInsufficient()
	}
	n := len(ctx.CloseP)
	closeNow := ctx.CloseP[n-1]
	aboveLine := closeNow > lineNow

	// 翻转新鲜度：age 0/1/2 → 1 / 0.5 / 0
	age, flipped := extFlipAge(bull, 3)
	freshness := 0.0
	if flipped {
		freshness = 1 - 0.5*float64(age)
	}
	factors := map[string]float64{
		"aboveLine":     extBool01(aboveLine),
		"flipFreshness": freshness,
	}

	if !flipped {
		bullNow := len(bull) > 0 && bull[len(bull)-1]
		if bullNow {
			return &StrategyResult{Score: 0, Factors: factors,
				Signal: fmt.Sprintf("超级趋势多头延续·线%.2f", lineNow)}
		}
		return &StrategyResult{Score: 0, Factors: factors,
			Signal: fmt.Sprintf("超级趋势空头·线%.2f", lineNow)}
	}
	if !aboveLine {
		return &StrategyResult{Score: 0, Factors: factors,
			Signal: fmt.Sprintf("超级趋势翻多%d根·收盘%.2f未站稳线%.2f", age+1, closeNow, lineNow)}
	}

	// 基础 55 + 新鲜度 0-30 → 55-85
	score := math.Round(55 + 30*freshness)
	return &StrategyResult{Score: score, Factors: factors,
		Signal: fmt.Sprintf("超级趋势翻多%d根·止损参考%.2f", age+1, lineNow)}
}

// ---------------- 3. SAR抛物线转向 ----------------

// SARTrendStrategy SAR(0.02,0.2) bull 翻 true ≤5 根内给分（越新鲜越高）；
// 近 5 根涨幅 >15% 视为拉伸过度降分（防追高）。
type SARTrendStrategy struct{}

func (s *SARTrendStrategy) Name() string { return "SAR抛物线转向" }
func (s *SARTrendStrategy) Code() string { return "sar_trend" }
func (s *SARTrendStrategy) Description() string {
	return "基于SAR抛物线转向翻多信号的顺势策略，拉伸过度时防追高降分"
}

func (s *SARTrendStrategy) Score(ctx *StrategyContext) *StrategyResult {
	if ctx == nil || len(ctx.CloseP) < 20 || len(ctx.HighP) < 20 || len(ctx.LowP) < 20 {
		return extInsufficient()
	}
	const step = 0.02
	const maxStep = 0.2
	sar, bull := indicator.PSAR(ctx.HighP, ctx.LowP, ctx.CloseP, step, maxStep)
	if _, ok := indicator.LastValid(sar); !ok {
		return extInsufficient()
	}
	n := len(ctx.CloseP)
	closeNow := ctx.CloseP[n-1]

	age, flipped := extFlipAge(bull, 5)
	factors := map[string]float64{
		"flipFreshness": extClamp01(1 - float64(age)/5),
	}
	if !flipped {
		bullNow := len(bull) > 0 && bull[len(bull)-1]
		if bullNow {
			return &StrategyResult{Score: 0, Factors: factors, Signal: "SAR多头延续·非新翻转"}
		}
		return &StrategyResult{Score: 0, Factors: factors, Signal: "SAR空头·未翻多"}
	}

	// 近 5 根涨幅（拉伸度）
	gain5 := 0.0
	if c5, ok := indicator.At(ctx.CloseP, 5); ok && c5 > 0 {
		gain5 = (closeNow/c5 - 1) * 100
	}
	stretch := extClamp01((gain5 - 15) / 10) // 15% → 0，25%+ → 1
	factors["stretch"] = stretch

	// 基础分按翻转新鲜度递减：age 0-5 → 75/70/62/56/50/45
	// （extFlipAge(bull,5) 返回 [0,5]，表必须 6 项，否则越界 panic）
	baseByAge := [6]float64{75, 70, 62, 56, 50, 45}
	score := baseByAge[age]
	signal := fmt.Sprintf("SAR翻多%d根·5日涨幅%.1f%%", age+1, gain5)
	if gain5 > 15 {
		score = math.Max(30, score-25) // 拉伸过度降分，防追高
		signal += "·拉伸过度防追高降分"
	}
	return &StrategyResult{Score: math.Round(score), Factors: factors, Signal: signal}
}

// ---------------- 4. MFI资金流 ----------------

// MFIFlowStrategy 两条做多路径：
//
//	A 低吸：MFI(period)<20 且较前一根回升 → Score 60-80；
//	B 突破：MFI 上穿 40 且 CMF(20)>0 → Score 55-75；
//	MFI>80 过热 → Score 0。
type MFIFlowStrategy struct{}

func (s *MFIFlowStrategy) Name() string { return "MFI资金流" }
func (s *MFIFlowStrategy) Code() string { return "mfi_flow" }
func (s *MFIFlowStrategy) Description() string {
	return "基于MFI超卖回升与CMF资金流确认的低吸策略，过热规避"
}

func (s *MFIFlowStrategy) Score(ctx *StrategyContext) *StrategyResult {
	if ctx == nil || len(ctx.CloseP) < 20 || len(ctx.HighP) < 20 || len(ctx.LowP) < 20 || len(ctx.Volume) < 20 {
		return extInsufficient()
	}
	period := int(extOverride(ctx, "mfi_period", 14))
	if period < 2 {
		period = 2
	}
	mfi := indicator.MFI(ctx.HighP, ctx.LowP, ctx.CloseP, ctx.Volume, period)
	mfiNow, ok := indicator.At(mfi, 0)
	if !ok {
		return extInsufficient()
	}
	if mfiNow > 80 {
		return &StrategyResult{Score: 0,
			Factors: map[string]float64{"mfiLevel": extClamp01((100 - mfiNow) / 20)},
			Signal:  fmt.Sprintf("MFI %.1f>80·超买过热", mfiNow)}
	}
	mfiPrev, okPrev := indicator.At(mfi, 1)
	if !okPrev {
		return extInsufficient()
	}

	// A. 超卖回升低吸
	if mfiNow < 20 && mfiNow > mfiPrev {
		oversold := extClamp01((20 - mfiNow) / 20)
		rebound := extClamp01((mfiNow - mfiPrev) / 10)
		score := math.Round(60 + 20*oversold)
		return &StrategyResult{Score: score,
			Factors: map[string]float64{"oversoldLevel": oversold, "rebound": rebound},
			Signal:  fmt.Sprintf("MFI %.1f<20低位回升·低吸", mfiNow)}
	}

	// B. MFI 上穿 40 且 CMF(20)>0
	if mfiNow > 40 && mfiPrev <= 40 {
		cmf := indicator.CMF(ctx.HighP, ctx.LowP, ctx.CloseP, ctx.Volume, 20)
		cmfNow, okC := indicator.LastValid(cmf)
		if !okC {
			return extInsufficient()
		}
		if cmfNow <= 0 {
			return &StrategyResult{Score: 0,
				Factors: map[string]float64{"crossStrength": extClamp01((mfiNow - 40) / 20)},
				Signal:  fmt.Sprintf("MFI %.1f上穿40·CMF %.2f≤0资金未流入", mfiNow, cmfNow)}
		}
		crossStrength := extClamp01((mfiNow - 40) / 20)
		cmfPos := extClamp01(cmfNow * 3)
		score := math.Round(55 + 20*crossStrength)
		return &StrategyResult{Score: score,
			Factors: map[string]float64{"crossStrength": crossStrength, "cmfPos": cmfPos},
			Signal:  fmt.Sprintf("MFI %.1f上穿40·CMF %.2f资金流入", mfiNow, cmfNow)}
	}

	// 中性区域
	return &StrategyResult{Score: 0,
		Factors: map[string]float64{"mfiLevel": extClamp01(mfiNow / 100)},
		Signal:  fmt.Sprintf("MFI %.1f·中性区域", mfiNow)}
}

// ---------------- 5. OBV量价背离 ----------------

// OBVDivergenceStrategy 近 20 根收盘创阶段新低而 OBV 不创新低 = 底背离（Score 65-85）；
// 价新高而 OBV 不新高 = 顶背离（Score 0 + Signal 注明顶背离）。
type OBVDivergenceStrategy struct{}

func (s *OBVDivergenceStrategy) Name() string { return "OBV量价背离" }
func (s *OBVDivergenceStrategy) Code() string { return "obv_divergence" }
func (s *OBVDivergenceStrategy) Description() string {
	return "基于OBV与价格顶底背离的反转预警与低吸策略"
}

func (s *OBVDivergenceStrategy) Score(ctx *StrategyContext) *StrategyResult {
	// 需要 20 根窗口 + 当前一根
	if ctx == nil || len(ctx.CloseP) < 21 || len(ctx.Volume) < 21 {
		return extInsufficient()
	}
	n := len(ctx.CloseP)
	obv := indicator.OBV(ctx.CloseP, ctx.Volume)
	obvNow, ok := indicator.At(obv, 0)
	if !ok {
		return extInsufficient()
	}
	closeNow := ctx.CloseP[n-1]
	eps := math.Abs(closeNow)*1e-9 + 1e-9

	low20, okL := indicator.LowestIn(ctx.CloseP, 20)
	high20, okH := indicator.HighestIn(ctx.CloseP, 20)
	obvLow20, okOL := indicator.LowestIn(obv, 20)
	obvHigh20, okOH := indicator.HighestIn(obv, 20)
	if !okL || !okH || !okOL || !okOH {
		return extInsufficient()
	}
	newLow := closeNow <= low20+eps
	newHigh := closeNow >= high20-eps

	// 顶背离：价新高而 OBV 未新高 → 风险信号
	if newHigh && obvNow < obvHigh20 {
		return &StrategyResult{Score: 0,
			Factors: map[string]float64{"topDivergence": 1},
			Signal:  fmt.Sprintf("顶背离·收盘%.2f创20日新高而OBV未新高·谨防回落", closeNow)}
	}
	// 底背离：价创新低而 OBV 未新低 → 低吸信号
	if newLow && obvNow > obvLow20 {
		rng := obvHigh20 - obvLow20
		pos := 0.5 // OBV 走平时取中位强度
		if rng > 0 {
			pos = extClamp01((obvNow - obvLow20) / rng)
		}
		score := math.Round(65 + 20*pos)
		return &StrategyResult{Score: score,
			Factors: map[string]float64{"bottomDivergence": 1, "obvPos": pos},
			Signal:  fmt.Sprintf("收盘%.2f创20日新低而OBV未新低·底背离", closeNow)}
	}

	return &StrategyResult{Score: 0,
		Factors: map[string]float64{},
		Signal:  "量价同步·20日内无背离"}
}

// ---------------- 6. 唐奇安突破 ----------------

// DonchianBreakoutStrategy 收盘上破 Donchian(period) 上轨：
// 量比（当日量/20日均量）≥ volume_min_ratio → Score 60-90；突破但量不足 → Score 40。
type DonchianBreakoutStrategy struct{}

func (s *DonchianBreakoutStrategy) Name() string { return "唐奇安突破" }
func (s *DonchianBreakoutStrategy) Code() string { return "donchian_breakout" }
func (s *DonchianBreakoutStrategy) Description() string {
	return "基于唐奇安通道上轨突破与放量确认的趋势突破策略"
}

func (s *DonchianBreakoutStrategy) Score(ctx *StrategyContext) *StrategyResult {
	if ctx == nil || len(ctx.CloseP) < 21 || len(ctx.HighP) < 21 || len(ctx.LowP) < 21 || len(ctx.Volume) < 21 {
		return extInsufficient()
	}
	period := int(extOverride(ctx, "donchian_period", 20))
	if period < 2 {
		period = 2
	}
	volMin := extOverride(ctx, "volume_min_ratio", 1.3)

	upper, _, _ := indicator.Donchian(ctx.HighP, ctx.LowP, period)
	// 突破基准取前一根的上轨（海龟式入场位）：无论实现是否把当前 bar 计入窗口，
	// 「今收 > 昨日通道上轨」均成立且不含未来数据。
	upperPrev, ok := indicator.At(upper, 1)
	if !ok {
		return extInsufficient()
	}
	ratio, okR := extVolRatio(ctx.Volume)
	if !okR {
		return extInsufficient()
	}
	closeNow := ctx.CloseP[len(ctx.CloseP)-1]

	if closeNow <= upperPrev {
		return &StrategyResult{Score: 0,
			Factors: map[string]float64{},
			Signal:  fmt.Sprintf("收盘%.2f未破%d日上轨%.2f", closeNow, period, upperPrev)}
	}

	// 放量确认：量比达标 60-90（volMin → 60，2.5 → 90）；不足 → 40
	volStrength := extClamp01((ratio - volMin) / math.Max(2.5-volMin, 0.1))
	if ratio >= volMin {
		score := math.Round(60 + 30*volStrength)
		return &StrategyResult{Score: score,
			Factors: map[string]float64{"breakout": 1, "volStrength": volStrength},
			Signal:  fmt.Sprintf("收盘%.2f突破%d日上轨%.2f·量比%.2f", closeNow, period, upperPrev, ratio)}
	}
	return &StrategyResult{Score: 40,
		Factors: map[string]float64{"breakout": 1, "volStrength": 0},
		Signal:  fmt.Sprintf("收盘%.2f突破%d日上轨%.2f·量比%.2f<%.1f放量不足", closeNow, period, upperPrev, ratio, volMin)}
}
