package data

// strategy_combos.go 实现 10 个「组合策略」，对应前端 K 线组合指标预设
// frontend/src/components/kline/indicators/combos.ts（经典入门/趋势波段/震荡低吸/
// 量价确认/TTM挤压突破/一目均衡趋势/鳄鱼动量/Elder三重过滤/自适应趋势/聪明钱结构）。
//
// 统一计分模式（共振计分法）：
//   - Score = 40（基础分，仅当核心方向成立）+ Σ(命中规则分)，封顶 100，四舍五入；
//   - 反向/离场信号命中 → Score 归 0，Signal 注明「离场信号」；
//   - 数据不足的规则跳过（该规则记 0 分），必要时 Signal 注「数据不足」，绝不 panic；
//   - 可调参数从 ctx.Overrides 读取（comboOverride），参数名与 combo 语义对应，
//     如 "adx_threshold"（默认 25）、"ema_fast"/"ema_slow"、"squeeze_min" 等。
//
// 指标来源：backend/data/indicator（序列型，预热期为 NaN，LastValid/At/CrossOver 判定）
// 与 data 包既有最新值函数（calcSMA/calcBOLL/calcMACD/calcKDJ/calcATR）。

import (
	"fmt"
	"math"
	"strconv"

	"go-stock/backend/data/indicator"
)

// ---------------------------------------------------------------------------
// 通用小工具（combo 前缀，避免与包内既有符号冲突）
// ---------------------------------------------------------------------------

// comboOverride 读取可调参数；未配置或非法（<=0）时返回默认值。
func comboOverride(ctx *StrategyContext, key string, def float64) float64 {
	if ctx != nil && ctx.Overrides != nil {
		if v, ok := ctx.Overrides[key]; ok && !math.IsNaN(v) && v > 0 {
			return v
		}
	}
	return def
}

// comboIntOverride 读取整型参数并保证下限。
func comboIntOverride(ctx *StrategyContext, key string, def, min int) int {
	v := int(comboOverride(ctx, key, float64(def)))
	if v < min {
		return def
	}
	return v
}

// comboClamp01 将因子得分限制在 [0,1]。
func comboClamp01(x float64) float64 {
	if math.IsNaN(x) || x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

// comboCap100 汇总分并封顶 100、四舍五入。
func comboCap100(score float64) float64 {
	if math.IsNaN(score) || score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return math.Round(score)
}

// comboJoinSignal 将命中片段用「·」拼成单行 Signal。
func comboJoinSignal(parts ...string) string {
	out := ""
	for _, p := range parts {
		if p == "" {
			continue
		}
		if out != "" {
			out += "·"
		}
		out += p
	}
	return out
}

// comboMACDSeries 以 indicator.EMA 级联构造 MACD 序列（DIF/DEA/HIST），
// 与 data 包 calcMACD 的 fast/slow/signal 默认参数一致；NaN 按算术传播。
func comboMACDSeries(close []float64, fast, slow, signal int) (dif, dea, hist []float64) {
	ef := indicator.EMA(close, fast)
	es := indicator.EMA(close, slow)
	n := len(close)
	dif = make([]float64, n)
	for i := 0; i < n; i++ {
		dif[i] = ef[i] - es[i]
	}
	dea = indicator.EMA(dif, signal)
	hist = make([]float64, n)
	for i := 0; i < n; i++ {
		hist[i] = dif[i] - dea[i]
	}
	return dif, dea, hist
}

// comboSMASeries 简单移动平均序列（indicator 包未导出 SMA，本地补一个供 BOLL 使用）。
func comboSMASeries(values []float64, period int) []float64 {
	n := len(values)
	out := make([]float64, n)
	for i := range out {
		out[i] = math.NaN()
	}
	if period <= 0 {
		return out
	}
	var sum float64
	for i := 0; i < n; i++ {
		sum += values[i]
		if i >= period {
			sum -= values[i-period]
		}
		if i >= period-1 {
			out[i] = sum / float64(period)
		}
	}
	return out
}

// comboBOLLSeries 布林带序列（总体标准差），与前端 bollingerBands 口径一致；
// 用于需要带宽序列（收口/开口判定）的场景。
func comboBOLLSeries(close []float64, period int, mult float64) (mid, up, down []float64) {
	n := len(close)
	mid = comboSMASeries(close, period)
	up = make([]float64, n)
	down = make([]float64, n)
	for i := range up {
		up[i] = math.NaN()
		down[i] = math.NaN()
	}
	for i := period - 1; i < n; i++ {
		if math.IsNaN(mid[i]) {
			continue
		}
		var sq float64
		for j := i - period + 1; j <= i; j++ {
			d := close[j] - mid[i]
			sq += d * d
		}
		sd := math.Sqrt(sq / float64(period))
		up[i] = mid[i] + mult*sd
		down[i] = mid[i] - mult*sd
	}
	return mid, up, down
}

// comboRSISeries Cutler 式 RSI 序列（SMA 平滑），用于「拐头」等需要序列的判定。
func comboRSISeries(close []float64, period int) []float64 {
	n := len(close)
	out := make([]float64, n)
	for i := range out {
		out[i] = math.NaN()
	}
	if period <= 0 || n <= period {
		return out
	}
	for i := period; i < n; i++ {
		var up, dn float64
		for j := i - period + 1; j <= i; j++ {
			d := close[j] - close[j-1]
			if d > 0 {
				up += d
			} else {
				dn -= d
			}
		}
		if up+dn > 0 {
			out[i] = up / (up + dn) * 100
		}
	}
	return out
}

// comboVolRatio20 最新成交量相对 20 日均量的比值（量比），不足 21 根返回 false。
func comboVolRatio20(volume []float64) (float64, bool) {
	n := len(volume)
	if n < 21 {
		return 0, false
	}
	var sum float64
	for i := n - 21; i < n-1; i++ {
		sum += volume[i]
	}
	avg := sum / 20
	if avg <= 0 {
		return 0, false
	}
	return volume[n-1] / avg, true
}

// comboLastLevel 取「最近一个已确认」的价位（如分形/结构位），排除最后 tail 根观察窗。
func comboLastLevel(levels []float64, tail int) (float64, bool) {
	end := len(levels) - tail
	if end > len(levels) {
		end = len(levels)
	}
	for i := end - 1; i >= 0; i-- {
		if !math.IsNaN(levels[i]) {
			return levels[i], true
		}
	}
	return 0, false
}

// ---------------------------------------------------------------------------
// 1. ComboClassicStrategy 经典入门组合（MA + BOLL + MACD + KDJ）
// ---------------------------------------------------------------------------

// ComboClassicStrategy 四指标共振：MA20 定方向（核心）、BOLL 定位置、
// MACD 零轴上金叉定买卖、KDJ 低位金叉找时点；四指标同向最强。
type ComboClassicStrategy struct{}

func (s *ComboClassicStrategy) Name() string { return "经典入门组合" }
func (s *ComboClassicStrategy) Code() string { return "combo_classic" }
func (s *ComboClassicStrategy) Description() string {
	return "MA20方向+BOLL位置+MACD零轴上金叉+KDJ低位金叉四指标共振打分"
}

func (s *ComboClassicStrategy) Score(ctx *StrategyContext) *StrategyResult {
	n := len(ctx.CloseP)
	maPeriod := comboIntOverride(ctx, "ma_period", 20, 5)
	if n < 20 || n < maPeriod {
		return &StrategyResult{Score: 0, Factors: map[string]float64{}, Signal: "K线数据不足"}
	}
	kdjOversold := comboOverride(ctx, "kdj_oversold", 20)

	close := ctx.CloseP[n-1]
	maN := calcSMA(ctx.CloseP, maPeriod)
	core := close > maN

	// 离场：MACD 零轴下死叉（tips：零轴下死叉卖）。
	// 用 indicator.EMA 级联序列取状态，避免 calcMACD round2 舍入在缓变行情下把 DIF/DEA 抹成相等。
	difS, deaS, _ := comboMACDSeries(ctx.CloseP, 12, 26, 9)
	dif, difOk := indicator.LastValid(difS)
	dea, deaOk := indicator.LastValid(deaS)
	if difOk && deaOk && dif < dea && dif < 0 {
		return &StrategyResult{Score: 0, Factors: map[string]float64{"macdGolden": 0}, Signal: "离场信号：MACD零轴下死叉"}
	}

	// 规则A：BOLL 位置——中上轨间理想，触上轨不追、触下轨不杀
	bollF := 0.0
	bollPos := ""
	if b := calcBOLL(ctx.CloseP, 20, 2.0); b["Mid"] > 0 {
		switch {
		case close >= b["Mid"] && close < b["Up"]:
			bollF, bollPos = 1.0, "中上轨间"
		case close > b["Down"] && close < b["Mid"]:
			bollF, bollPos = 0.3, "中轨下方"
		case close >= b["Up"]:
			bollF, bollPos = 0.2, "触上轨不追"
		default:
			bollF, bollPos = 0.2, "触下轨不杀"
		}
	}

	// 规则B：MACD——零轴上金叉状态
	macdF, macdTxt := 0.0, ""
	switch {
	case difOk && deaOk && dif > dea && dif > 0:
		macdF, macdTxt = 1.0, "零轴上金叉"
	case difOk && deaOk && dif > dea:
		macdF, macdTxt = 0.5, "金叉(零轴下)"
	}

	// 规则C：KDJ——低位金叉比高位金叉可靠
	kdjF, kdjTxt := 0.0, ""
	if k := calcKDJ(ctx.HighP, ctx.LowP, ctx.CloseP, 9, 3); k["D"] != 0 {
		if k["K"] > k["D"] {
			switch {
			case k["K"] < kdjOversold:
				kdjF = 1.0
			case k["K"] < 50:
				kdjF = 0.6
			default:
				kdjF = 0.3
			}
			kdjTxt = fmt.Sprintf("KDJ%.1f金叉", k["K"])
		} else {
			kdjTxt = fmt.Sprintf("KDJ%.1f未金叉", k["K"])
		}
	}

	// 四指标同向加成
	bonus := 0.0
	if core && bollF >= 1.0 && macdF >= 1.0 && kdjF >= 0.6 {
		bonus = 10
	}

	maTxt := fmt.Sprintf("MA%d下方", maPeriod)
	if core {
		maTxt = fmt.Sprintf("站上MA%d", maPeriod)
	}
	score := 40*boolScore(core) + 15*bollF + 20*macdF + 15*kdjF + bonus
	signal := comboJoinSignal(maTxt, bollPos, macdTxt, kdjTxt)
	if !core {
		signal = comboJoinSignal("方向未确立："+signal, "仅规则分")
	}
	return &StrategyResult{
		Score: comboCap100(score),
		Factors: map[string]float64{
			"maAbove": boolScore(core), "bollPos": bollF, "macdGolden": macdF, "kdjGolden": kdjF, "alignedBonus": bonus / 10,
		},
		Signal: signal,
	}
}

func boolScore(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// ---------------------------------------------------------------------------
// 2. ComboTrendSwingStrategy 趋势波段组合（EMA + MACD + ADX）
// ---------------------------------------------------------------------------

// ComboTrendSwingStrategy 日线上升趋势波段：EMA12>EMA21 只做多（核心），
// ADX>25 才有趋势、MACD 柱放大=加速；EMA 死叉离场。
type ComboTrendSwingStrategy struct{}

func (s *ComboTrendSwingStrategy) Name() string { return "趋势波段组合" }
func (s *ComboTrendSwingStrategy) Code() string { return "combo_trend_swing" }
func (s *ComboTrendSwingStrategy) Description() string {
	return "EMA12>EMA21趋势向上+ADX>25确认趋势+MACD柱放大加速，做日线上升波段"
}

func (s *ComboTrendSwingStrategy) Score(ctx *StrategyContext) *StrategyResult {
	emaFast := comboIntOverride(ctx, "ema_fast", 12, 2)
	emaSlow := comboIntOverride(ctx, "ema_slow", 21, 3)
	if emaFast >= emaSlow {
		emaFast, emaSlow = 12, 21
	}
	if len(ctx.CloseP) < emaSlow+5 {
		return &StrategyResult{Score: 0, Factors: map[string]float64{}, Signal: "K线数据不足"}
	}
	adxTh := comboOverride(ctx, "adx_threshold", 25)

	emaF := indicator.EMA(ctx.CloseP, emaFast)
	emaS := indicator.EMA(ctx.CloseP, emaSlow)
	eF0, eFok := indicator.LastValid(emaF)
	eS0, eSok := indicator.LastValid(emaS)
	if !eFok || !eSok {
		return &StrategyResult{Score: 0, Factors: map[string]float64{}, Signal: "K线数据不足"}
	}

	// 离场：EMA 快线下穿慢线
	if indicator.CrossUnder(emaF, emaS) {
		return &StrategyResult{
			Score: 0, Factors: map[string]float64{"emaBull": 0},
			Signal: fmt.Sprintf("离场信号：EMA%d下穿EMA%d", emaFast, emaSlow),
		}
	}

	core := eF0 > eS0

	// 规则A：ADX 趋势强度——<20 观望不做
	adxScore, adxTxt := 0.0, ""
	if a, ok := indicator.LastValid(adxOf(ctx)); ok {
		switch {
		case a >= adxTh:
			adxScore, adxTxt = 1.0, fmt.Sprintf("ADX%.1f", a)
		case a >= 20:
			adxScore, adxTxt = 0.4, fmt.Sprintf("ADX%.1f偏弱", a)
		default:
			adxTxt = fmt.Sprintf("ADX%.1f观望", a)
		}
	}

	// 规则B：MACD 柱放大=加速，持续缩小=减速警惕
	_, _, hist := comboMACDSeries(ctx.CloseP, emaFast, emaSlow, 9)
	histScore, histTxt := 0.0, ""
	if h0, ok0 := indicator.LastValid(hist); ok0 {
		if h1, ok1 := indicator.At(hist, 1); ok1 {
			switch {
			case h0 > 0 && h0 > h1:
				histScore, histTxt = 1.0, "MACD柱放大加速"
			case h0 > 0:
				histScore, histTxt = 0.4, "MACD柱缩小减速"
			}
		}
	}

	// 规则C：MACD 零轴上金叉=最佳波段买点
	dif, dea, _ := comboMACDSeries(ctx.CloseP, emaFast, emaSlow, 9)
	crossScore, crossTxt := 0.0, ""
	if d0, ok0 := indicator.LastValid(dif); ok0 {
		if s0, oks := indicator.LastValid(dea); oks {
			if indicator.CrossOver(dif, dea) && d0 > 0 {
				crossScore, crossTxt = 1.0, "零轴上金叉"
			} else if d0 > s0 && d0 > 0 {
				crossScore, crossTxt = 0.5, "零轴上多头"
			}
		}
	}

	// 规则D：收盘站上快线
	aboveScore := 0.0
	if ctx.CloseP[len(ctx.CloseP)-1] > eF0 {
		aboveScore = 1.0
	}

	coreTxt := "EMA空头排列观望"
	if core {
		coreTxt = fmt.Sprintf("EMA%d>EMA%d", emaFast, emaSlow)
	}
	score := 40*boolScore(core) + 20*adxScore + 20*histScore + 15*crossScore + 5*aboveScore
	return &StrategyResult{
		Score: comboCap100(score),
		Factors: map[string]float64{
			"emaBull": boolScore(core), "adxStrong": adxScore, "histExpand": histScore,
			"macdCross": crossScore, "closeAboveEma": aboveScore,
		},
		Signal: comboJoinSignal(coreTxt, adxTxt, histTxt, crossTxt),
	}
}

// adxOf 计算 14 周期 ADX 序列（多个 combo 复用）。
func adxOf(ctx *StrategyContext) []float64 {
	adx, _, _ := indicator.ADX(ctx.HighP, ctx.LowP, ctx.CloseP, 14)
	return adx
}

// ---------------------------------------------------------------------------
// 3. ComboRangeDipStrategy 震荡低吸组合（BOLL + KDJ + RSI）
// ---------------------------------------------------------------------------

// ComboRangeDipStrategy 专做横盘震荡：先确认 BOLL 收口（开口即停用），
// 触下轨 + KDJ 低位金叉低吸；触上轨 + RSI 超买拐头高抛（离场）。
type ComboRangeDipStrategy struct{}

func (s *ComboRangeDipStrategy) Name() string { return "震荡低吸组合" }
func (s *ComboRangeDipStrategy) Code() string { return "combo_range_dip" }
func (s *ComboRangeDipStrategy) Description() string {
	return "BOLL收口确认震荡+触下轨+KDJ低位金叉低吸；BOLL开口自动停用"
}

func (s *ComboRangeDipStrategy) Score(ctx *StrategyContext) *StrategyResult {
	n := len(ctx.CloseP)
	if n < 30 {
		return &StrategyResult{Score: 0, Factors: map[string]float64{}, Signal: "K线数据不足"}
	}
	expandRatio := comboOverride(ctx, "bandwidth_expand", 1.3)
	kdjOversold := comboOverride(ctx, "kdj_oversold", 20)
	rsiOversold := comboOverride(ctx, "rsi_oversold", 30)
	rsiOverbought := comboOverride(ctx, "rsi_overbought", 70)

	mid, up, down := comboBOLLSeries(ctx.CloseP, 20, 2.0)
	bw := make([]float64, n)
	for i := 0; i < n; i++ {
		bw[i] = math.NaN()
		if !math.IsNaN(mid[i]) && mid[i] > 0 {
			bw[i] = (up[i] - down[i]) / mid[i]
		}
	}
	bw0, bw0ok := indicator.LastValid(bw)
	bw5, bw5ok := indicator.At(bw, 5)
	mid0, mid0ok := indicator.LastValid(mid)
	up0, up0ok := indicator.LastValid(up)
	down0, down0ok := indicator.LastValid(down)
	close := ctx.CloseP[n-1]

	// BOLL 开口 → 本组合停用
	if bw0ok && bw5ok && bw0 > bw5*expandRatio {
		return &StrategyResult{Score: 0, Factors: map[string]float64{"squeeze": 0}, Signal: "BOLL开口，本组合停用"}
	}

	// 单边下跌保护（tips：单边下跌中用会接飞刀）：中轨明显下移即非横盘 → 停用。
	// lookback 取 10 根覆盖一个完整锯齿/震荡周期，避免相位噪声误判。
	flatTh := comboOverride(ctx, "range_flat", 0.015)
	mid10, mid10ok := indicator.At(mid, 10)
	flat := !mid0ok || !mid10ok || mid10 <= 0 || math.Abs(mid0-mid10)/mid10 <= flatTh
	if mid0ok && mid10ok && mid10 > 0 && !flat && mid0 < mid10 {
		return &StrategyResult{Score: 0, Factors: map[string]float64{"rangeConfirmed": 0}, Signal: "单边下跌，本组合停用（防接飞刀）"}
	}

	rsi := comboRSISeries(ctx.CloseP, 14)
	r0, r0ok := indicator.LastValid(rsi)
	r1, r1ok := indicator.At(rsi, 1)

	// 离场：触上轨 + RSI 超买拐头 = 高抛
	if up0ok && r0ok && r1ok && close >= up0 && r0 > rsiOverbought && r0 < r1 {
		return &StrategyResult{Score: 0, Factors: map[string]float64{"dipPos": 0}, Signal: "离场信号：触上轨+RSI超买拐头（高抛）"}
	}

	// 核心：震荡确认（带宽未开口 + 中轨横盘）且价格处于中轨下方低位区
	core := bw0ok && mid0ok && flat && close < mid0

	// 规则A：触下轨=低吸点
	dipScore, dipTxt := 0.0, ""
	if down0ok {
		low0 := ctx.LowP[n-1]
		if low0 <= down0 || close <= down0*1.005 {
			dipScore, dipTxt = 1.0, "触下轨"
		} else if core {
			dipScore = 0.4
		}
	}

	// 规则B：KDJ 低位金叉
	kdjScore, kdjTxt := 0.0, ""
	if k := calcKDJ(ctx.HighP, ctx.LowP, ctx.CloseP, 9, 3); k["D"] != 0 {
		if k["K"] > k["D"] {
			switch {
			case k["K"] < kdjOversold:
				kdjScore = 1.0
				kdjTxt = fmt.Sprintf("KDJ%.1f低位金叉", k["K"])
			case k["K"] < 30:
				kdjScore, kdjTxt = 0.6, fmt.Sprintf("KDJ%.1f金叉", k["K"])
			default:
				kdjScore, kdjTxt = 0.3, fmt.Sprintf("KDJ%.1f金叉", k["K"])
			}
		}
	}

	// 规则C：RSI 超卖区低吸更安全
	rsiScore, rsiTxt := 0.0, ""
	if r0ok {
		switch {
		case r0 < rsiOversold:
			rsiScore, rsiTxt = 1.0, fmt.Sprintf("RSI%.1f超卖", r0)
		case r0 < rsiOversold+15:
			rsiScore, rsiTxt = 0.5, fmt.Sprintf("RSI%.1f", r0)
		}
	}

	score := 40*boolScore(core) + 25*dipScore + 25*kdjScore + 10*rsiScore
	coreTxt := "震荡未确立"
	if core {
		coreTxt = "BOLL收口震荡"
	}
	if bw0ok && bw5ok {
		coreTxt = fmt.Sprintf("%s·带宽%.3f", coreTxt, bw0)
	}
	return &StrategyResult{
		Score: comboCap100(score),
		Factors: map[string]float64{
			"rangeConfirmed": boolScore(core), "dipPos": dipScore, "kdjGolden": kdjScore, "rsiOversold": rsiScore,
		},
		Signal: comboJoinSignal(coreTxt, dipTxt, kdjTxt, rsiTxt),
	}
}

// ---------------------------------------------------------------------------
// 4. ComboVolPriceStrategy 量价确认组合（OBV + MFI + CMF）
// ---------------------------------------------------------------------------

// ComboVolPriceStrategy 用资金与成交量验证价格：上涨须放量、OBV 同步上行、
// CMF>0 净流入、MFI 不过热；三个指标至少两个配合才算有效突破；背离减分。
type ComboVolPriceStrategy struct{}

func (s *ComboVolPriceStrategy) Name() string { return "量价确认组合" }
func (s *ComboVolPriceStrategy) Code() string { return "combo_vol_price" }
func (s *ComboVolPriceStrategy) Description() string {
	return "OBV同步上行+CMF>0净流入+MFI不过热，量价配合确认突破有效"
}

func (s *ComboVolPriceStrategy) Score(ctx *StrategyContext) *StrategyResult {
	n := len(ctx.CloseP)
	if n < 22 {
		return &StrategyResult{Score: 0, Factors: map[string]float64{}, Signal: "K线数据不足"}
	}
	lookback := comboIntOverride(ctx, "obv_lookback", 5, 1)
	volRatioMin := comboOverride(ctx, "vol_ratio_min", 1.2)

	priceUp := ctx.CloseP[n-1] > ctx.CloseP[n-2]

	// 规则A：OBV 同步上行（突破可信）
	obv := indicator.OBV(ctx.CloseP, ctx.Volume)
	o0, o0ok := indicator.LastValid(obv)
	oRef, oRefok := indicator.At(obv, lookback)
	o1, o1ok := indicator.At(obv, 1)
	obvUp := o0ok && oRefok && o0 > oRef
	divergence := priceUp && o0ok && o1ok && o0 <= o1 // 价涨但 OBV 走平/下降 = 背离

	// 规则B：CMF>0 资金净流入
	cmfScore, cmfTxt := 0.0, ""
	if c0, ok := indicator.LastValid(indicator.CMF(ctx.HighP, ctx.LowP, ctx.CloseP, ctx.Volume, 20)); ok {
		switch {
		case c0 > 0:
			cmfScore, cmfTxt = 1.0, fmt.Sprintf("CMF%+.3f流入", c0)
		case c0 > -0.05:
			cmfScore, cmfTxt = 0.4, fmt.Sprintf("CMF%+.3f", c0)
		default:
			cmfTxt = fmt.Sprintf("CMF%+.3f流出", c0)
		}
	}

	// 规则C：MFI 不在 80 以上过热（<20 资金冰点看反弹）
	mfiScore, mfiTxt := 0.0, ""
	mfiOk := false
	if m0, ok := indicator.LastValid(indicator.MFI(ctx.HighP, ctx.LowP, ctx.CloseP, ctx.Volume, 14)); ok {
		switch {
		case m0 > 80:
			mfiTxt = fmt.Sprintf("MFI%.1f过热", m0)
		case m0 < 20:
			mfiScore, mfiTxt, mfiOk = 0.6, fmt.Sprintf("MFI%.1f冰点", m0), true
		default:
			mfiScore, mfiTxt, mfiOk = 1.0, fmt.Sprintf("MFI%.1f", m0), true
		}
	}

	// 规则D：放量
	volScore, volTxt := 0.0, ""
	if ratio, ok := comboVolRatio20(ctx.Volume); ok {
		switch {
		case ratio >= 2:
			volScore = 1.0
		case ratio >= volRatioMin:
			volScore = 0.7
		case ratio >= 1:
			volScore = 0.4
		}
		volTxt = fmt.Sprintf("量比%.2f", ratio)
	} else {
		volTxt = "量比数据不足"
	}

	// 至少两条配合才算有效突破（OBV上行 / CMF>0 / MFI 不过热）
	confirm := 0
	if obvUp {
		confirm++
	}
	if cmfScore >= 1.0 {
		confirm++
	}
	if mfiOk {
		confirm++
	}
	base := priceUp && confirm >= 2

	obvTxt := "OBV未同步"
	if divergence {
		obvTxt = "OBV背离上涨乏力"
	} else if obvUp {
		obvTxt = "OBV同步上行"
	}
	priceTxt := ""
	if !priceUp {
		priceTxt = "价未涨"
	}
	score := 40*boolScore(base) + 20*boolScore(obvUp) + 20*cmfScore + 15*mfiScore + 5*volScore
	return &StrategyResult{
		Score: comboCap100(score),
		Factors: map[string]float64{
			"validBreakout": boolScore(base), "obvUp": boolScore(obvUp), "cmfInflow": cmfScore,
			"mfiHealthy": mfiScore, "volumeUp": volScore,
		},
		Signal: comboJoinSignal(priceTxt, obvTxt, cmfTxt, mfiTxt, volTxt),
	}
}

// ---------------------------------------------------------------------------
// 5. ComboTTMBreakStrategy TTM挤压突破组合（TTM Squeeze + BOLL + Keltner）
// ---------------------------------------------------------------------------

// ComboTTMBreakStrategy 捕捉横盘蓄力后的爆发：BOLL 收入 Keltner 内（挤压）≥5 根，
// 挤压释放（黄转绿）+ 动量柱>0 + 放量为最佳入场；向下释放回避。
type ComboTTMBreakStrategy struct{}

func (s *ComboTTMBreakStrategy) Name() string { return "TTM挤压突破组合" }
func (s *ComboTTMBreakStrategy) Code() string { return "combo_ttm_break" }
func (s *ComboTTMBreakStrategy) Description() string {
	return "BOLL收入Keltner(挤压)后释放+动量柱>0+放量，捕捉蓄力后的爆发突破"
}

func (s *ComboTTMBreakStrategy) Score(ctx *StrategyContext) *StrategyResult {
	n := len(ctx.CloseP)
	if n < 30 {
		return &StrategyResult{Score: 0, Factors: map[string]float64{}, Signal: "K线数据不足"}
	}
	squeezeMin := comboIntOverride(ctx, "squeeze_min", 5, 2)
	volRatioMin := comboOverride(ctx, "vol_ratio_min", 1.5)

	sq, mom := indicator.TTMSqueeze(ctx.HighP, ctx.LowP, ctx.CloseP, 20, 20, 10, 2.0, 1.5)

	// 挤压持续根数（优先统计释放前的一段，其次统计进行中的一段）
	runLen := 0
	for i := n - 2; i >= 0 && sq[i]; i-- {
		runLen++
	}
	if runLen == 0 && n >= 1 && sq[n-1] {
		for i := n - 1; i >= 0 && sq[i]; i-- {
			runLen++
		}
	}
	releasedToday := n >= 2 && sq[n-2] && !sq[n-1]
	releasedRecently := releasedToday
	if !releasedRecently && n >= 4 {
		for i := n - 3; i <= n-2; i++ {
			if sq[i-1] && !sq[i] {
				releasedRecently = true
			}
		}
	}

	m0, m0ok := indicator.LastValid(mom)
	m1, m1ok := indicator.At(mom, 1)

	// 离场：挤压向下释放（动量柱 < 0）
	if releasedRecently && m0ok && m0 < 0 {
		return &StrategyResult{Score: 0, Factors: map[string]float64{"squeezeRelease": 0}, Signal: "离场信号：挤压向下释放，回避"}
	}

	// 核心：挤压后向上释放且动量为正
	core := releasedRecently && m0ok && m0 > 0

	// 规则A：挤压酝酿时长
	sqScore, sqTxt := 0.0, ""
	switch {
	case runLen >= squeezeMin:
		sqScore, sqTxt = 1.0, fmt.Sprintf("挤压%d根", runLen)
	case runLen >= 2:
		sqScore, sqTxt = 0.5, fmt.Sprintf("挤压%d根", runLen)
	}

	// 规则B：挤压释放（黄转绿）
	relScore, relTxt := 0.0, ""
	switch {
	case releasedToday:
		relScore, relTxt = 1.0, "黄转绿释放"
	case releasedRecently:
		relScore, relTxt = 0.6, "近日释放"
	case n >= 1 && sq[n-1]:
		relScore, relTxt = 0.2, "挤压中盯守"
	}

	// 规则C：动量柱>0 且放大
	momScore, momTxt := 0.0, ""
	if m0ok {
		momTxt = fmt.Sprintf("动量%+.2f", m0)
		switch {
		case m0 > 0 && m1ok && m0 > m1:
			momScore = 1.0
		case m0 > 0:
			momScore = 0.6
		}
	}

	// 规则D：放量确认
	volScore, volTxt := 0.0, ""
	if ratio, ok := comboVolRatio20(ctx.Volume); ok {
		switch {
		case ratio >= 2:
			volScore = 1.0
		case ratio >= volRatioMin:
			volScore = 0.7
		case ratio >= 1.2:
			volScore = 0.4
		}
		volTxt = fmt.Sprintf("量比%.2f", ratio)
	}

	score := 40*boolScore(core) + 15*sqScore + 25*relScore + 10*momScore + 10*volScore
	return &StrategyResult{
		Score: comboCap100(score),
		Factors: map[string]float64{
			"squeezeRelease": boolScore(core), "squeezeLen": sqScore, "release": relScore,
			"momentumUp": momScore, "volumeUp": volScore,
		},
		Signal: comboJoinSignal(sqTxt, relTxt, momTxt, volTxt),
	}
}

// ---------------------------------------------------------------------------
// 6. ComboIchimokuStrategy 一目均衡趋势组合（Ichimoku + ADX）
// ---------------------------------------------------------------------------

// ComboIchimokuStrategy 日本机构经典趋势系统：价格在云上方只做多（核心），
// 转换线上穿基准线 + ADX>25 过滤假突破；收盘跌回云层内止损离场。
// 云层按位移 26 根取值，需 ≥ 52+26 根数据，不足时核心不成立。
type ComboIchimokuStrategy struct{}

func (s *ComboIchimokuStrategy) Name() string { return "一目均衡趋势组合" }
func (s *ComboIchimokuStrategy) Code() string { return "combo_ichimoku" }
func (s *ComboIchimokuStrategy) Description() string {
	return "价格在云层上方+转换线上穿基准线+ADX>25过滤假突破，只做单边趋势"
}

func (s *ComboIchimokuStrategy) Score(ctx *StrategyContext) *StrategyResult {
	const tenkanP, kijunP, senkouBP, displacement = 9, 26, 52, 26
	n := len(ctx.CloseP)
	if n < senkouBP+displacement+2 {
		return &StrategyResult{Score: 0, Factors: map[string]float64{}, Signal: "K线数据不足"}
	}
	adxTh := comboOverride(ctx, "adx_threshold", 25)

	tenkan, kijun, spanA, spanB := indicator.Ichimoku(ctx.HighP, ctx.LowP, ctx.CloseP, tenkanP, kijunP, senkouBP)
	// 当前 K 线所对应的云层 = 26 根前计算出的先行跨度（绘图时前移 26）
	a, aok := indicator.At(spanA, displacement)
	b, bok := indicator.At(spanB, displacement)
	if !aok || !bok {
		return &StrategyResult{Score: 0, Factors: map[string]float64{}, Signal: "K线数据不足（云层未成形）"}
	}
	cloudTop := math.Max(a, b)
	cloudBot := math.Min(a, b)
	close := ctx.CloseP[n-1]

	// 离场/观望：跌回云内止损，云下方观望
	if close <= cloudBot {
		return &StrategyResult{Score: 0, Factors: map[string]float64{"aboveCloud": 0}, Signal: "价格在云层下方，观望"}
	}
	if close <= cloudTop {
		return &StrategyResult{Score: 0, Factors: map[string]float64{"aboveCloud": 0}, Signal: "离场信号：收盘跌回云层内"}
	}

	// 核心：价格在云上方=多头市场
	core := close > cloudTop

	// 规则A：转换线上穿基准线（强买入信号）
	tkScore, tkTxt := 0.0, ""
	t0, t0ok := indicator.LastValid(tenkan)
	k0, k0ok := indicator.LastValid(kijun)
	if t0ok && k0ok {
		if indicator.CrossOver(tenkan, kijun) {
			tkScore, tkTxt = 1.0, "转换上穿基准"
		} else if t0 > k0 {
			tkScore, tkTxt = 0.4, "转换在基准上方"
		}
	}

	// 规则B：ADX>25 确认趋势，过滤云层附近的假突破
	adxScore, adxTxt := 0.0, ""
	if v, ok := indicator.LastValid(adxOf(ctx)); ok {
		switch {
		case v >= adxTh:
			adxScore, adxTxt = 1.0, fmt.Sprintf("ADX%.1f", v)
		case v >= 20:
			adxScore, adxTxt = 0.4, fmt.Sprintf("ADX%.1f偏弱", v)
		}
	}

	// 规则C：云层厚度=支撑强度
	thickScore := 0.0
	thickness := (cloudTop - cloudBot) / close
	switch {
	case thickness >= 0.01:
		thickScore = 1.0
	case thickness >= 0.005:
		thickScore = 0.5
	}

	score := 40*boolScore(core) + 25*tkScore + 20*adxScore + 15*thickScore
	return &StrategyResult{
		Score: comboCap100(score),
		Factors: map[string]float64{
			"aboveCloud": boolScore(core), "tkCross": tkScore, "adxStrong": adxScore, "cloudThick": thickScore,
		},
		Signal: comboJoinSignal("云上方", fmt.Sprintf("云顶%.2f", cloudTop), tkTxt, adxTxt),
	}
}

// ---------------------------------------------------------------------------
// 7. ComboAlligatorStrategy 鳄鱼动量组合（Alligator + AO + Fractal）
// ---------------------------------------------------------------------------

// ComboAlligatorStrategy Bill Williams 系统：三线纠缠（沉睡期）坚决不操作（Score 0），
// 张嘴发散=趋势启动顺势入场，AO 同向放大动量确认，突破最近顶分形精确入场。
type ComboAlligatorStrategy struct{}

func (s *ComboAlligatorStrategy) Name() string { return "鳄鱼动量组合" }
func (s *ComboAlligatorStrategy) Code() string { return "combo_alligator" }
func (s *ComboAlligatorStrategy) Description() string {
	return "鳄鱼三线发散张嘴+AO动量同向放大+突破最近顶分形，等趋势张嘴再进场"
}

func (s *ComboAlligatorStrategy) Score(ctx *StrategyContext) *StrategyResult {
	n := len(ctx.CloseP)
	if n < 40 {
		return &StrategyResult{Score: 0, Factors: map[string]float64{}, Signal: "K线数据不足"}
	}
	entangleTh := comboOverride(ctx, "entangle_threshold", 0.015)

	jawS, teethS, lipsS := indicator.Alligator(ctx.HighP, ctx.LowP, ctx.CloseP, 13, 8, 5, 8, 5, 3)
	jaw, jok := indicator.LastValid(jawS)
	teeth, tok := indicator.LastValid(teethS)
	lips, lok := indicator.LastValid(lipsS)
	if !jok || !tok || !lok {
		return &StrategyResult{Score: 0, Factors: map[string]float64{}, Signal: "K线数据不足"}
	}
	close := ctx.CloseP[n-1]
	hi := math.Max(jaw, math.Max(teeth, lips))
	lo := math.Min(jaw, math.Min(teeth, lips))
	spread := (hi - lo) / close

	// 不操作：三线纠缠=沉睡期
	if spread < entangleTh {
		return &StrategyResult{Score: 0, Factors: map[string]float64{"openMouth": 0}, Signal: "鳄鱼三线纠缠（沉睡期），不操作"}
	}
	// 观望：三线向下发散（只做多系统）
	if lips < teeth && teeth < jaw {
		return &StrategyResult{Score: 0, Factors: map[string]float64{"openMouth": 0}, Signal: "鳄鱼三线向下发散，观望"}
	}

	// 核心：张嘴多头排列（唇>牙>颚）
	core := lips > teeth && teeth > jaw

	// 规则A：张嘴发散扩大=趋势启动初期
	widenScore, widenTxt := 0.0, ""
	j3, j3ok := indicator.At(jawS, 3)
	t3, t3ok := indicator.At(teethS, 3)
	l3, l3ok := indicator.At(lipsS, 3)
	if j3ok && t3ok && l3ok {
		hi3 := math.Max(j3, math.Max(t3, l3))
		lo3 := math.Min(j3, math.Min(t3, l3))
		spreadPrev := (hi3 - lo3) / close
		if spread > spreadPrev {
			widenScore, widenTxt = 1.0, "张嘴扩大"
		} else if core {
			widenScore, widenTxt = 0.5, "张嘴持稳"
		}
	}

	// 规则B：AO 同向放大=动量确认
	aoScore, aoTxt := 0.0, ""
	ao := indicator.AO(ctx.HighP, ctx.LowP, 5, 34)
	if a0, ok0 := indicator.LastValid(ao); ok0 {
		aoTxt = fmt.Sprintf("AO%+.2f", a0)
		if a1, ok1 := indicator.At(ao, 1); ok1 {
			switch {
			case a0 > 0 && a0 > a1:
				aoScore = 1.0
			case a0 > 0:
				aoScore = 0.5
			}
		}
	}

	// 规则C：突破最近顶分形=精确入场点
	frScore, frTxt := 0.0, ""
	upFr, _ := indicator.FractalLevels(ctx.HighP, ctx.LowP, 2)
	if lvl, ok := comboLastLevel(upFr, 2); ok {
		if close > lvl && ctx.CloseP[n-2] <= lvl {
			frScore, frTxt = 1.0, fmt.Sprintf("突破顶分形%.2f", lvl)
		} else if close > lvl {
			frScore, frTxt = 0.4, fmt.Sprintf("站上顶分形%.2f", lvl)
		}
	}

	score := 40*boolScore(core) + 15*widenScore + 20*aoScore + 25*frScore
	coreTxt := "三线方向未明"
	if core {
		coreTxt = "张嘴多头排列"
	}
	return &StrategyResult{
		Score: comboCap100(score),
		Factors: map[string]float64{
			"openMouth": boolScore(core), "widening": widenScore, "aoConfirm": aoScore, "fractalBreak": frScore,
		},
		Signal: comboJoinSignal(coreTxt, widenTxt, aoTxt, frTxt, fmt.Sprintf("开口%.2f%%", spread*100)),
	}
}

// ---------------------------------------------------------------------------
// 8. ComboElderTripleStrategy Elder三重过滤组合（EMA + ForceIndex + ElderRay）
// ---------------------------------------------------------------------------

// ComboElderTripleStrategy 三重滤网：EMA 方向定趋势只顺不逆（核心），
// 上升趋势中 ForceIndex 回落=回调到位，BearPower<0 且收窄回升=空头衰竭买入。
type ComboElderTripleStrategy struct{}

func (s *ComboElderTripleStrategy) Name() string { return "Elder三重过滤组合" }
func (s *ComboElderTripleStrategy) Code() string { return "combo_elder_triple" }
func (s *ComboElderTripleStrategy) Description() string {
	return "EMA方向定趋势+ForceIndex回落确认回调+BearPower收窄确认空头衰竭"
}

func (s *ComboElderTripleStrategy) Score(ctx *StrategyContext) *StrategyResult {
	n := len(ctx.CloseP)
	if n < 35 {
		return &StrategyResult{Score: 0, Factors: map[string]float64{}, Signal: "K线数据不足"}
	}
	emaPeriod := comboIntOverride(ctx, "ema_period", 13, 5)
	fiPeriod := comboIntOverride(ctx, "fi_period", 13, 2)

	ema := indicator.EMA(ctx.CloseP, emaPeriod)
	e0, e0ok := indicator.LastValid(ema)
	e1, e1ok := indicator.At(ema, 1)
	e2, e2ok := indicator.At(ema, 2)
	if !e0ok || !e1ok || !e2ok {
		return &StrategyResult{Score: 0, Factors: map[string]float64{}, Signal: "K线数据不足"}
	}

	// 离场：EMA 拐头向下立即止损
	if e0 < e1 && e1 < e2 {
		return &StrategyResult{Score: 0, Factors: map[string]float64{"emaUp": 0}, Signal: "离场信号：EMA拐头向下"}
	}

	// 第一重：EMA 方向定趋势，只顺不逆
	core := e0 > e2

	// 第二重：上升趋势中 ForceIndex 回落=回调到位（负值区最佳）
	fiScore, fiTxt := 0.0, ""
	fi := indicator.ForceIndex(ctx.CloseP, ctx.Volume, fiPeriod)
	if f0, ok0 := indicator.LastValid(fi); ok0 {
		if f1, ok1 := indicator.At(fi, 1); ok1 {
			switch {
			case f0 < 0:
				fiScore, fiTxt = 1.0, "ForceIndex回调到位"
			case f0 < f1:
				fiScore, fiTxt = 0.5, "ForceIndex回落"
			}
		}
	}

	// 第三重：BearPower<0 且收窄回升=空头衰竭
	bearScore, bearTxt := 0.0, ""
	bullP, bearP := indicator.ElderRay(ctx.HighP, ctx.LowP, ctx.CloseP, emaPeriod)
	bullScore := 0.0
	if b0, ok0 := indicator.LastValid(bearP); ok0 {
		bearTxt = fmt.Sprintf("BearPower%.2f", b0)
		if b1, ok1 := indicator.At(bearP, 1); ok1 {
			switch {
			case b0 < 0 && b0 > b1:
				bearScore = 1.0
			case b0 > b1:
				bearScore = 0.5
			}
		}
	}
	if p0, ok := indicator.LastValid(bullP); ok && p0 > 0 {
		bullScore = 1.0
	}

	score := 40*boolScore(core) + 20*fiScore + 25*bearScore + 15*bullScore
	coreTxt := "EMA未向上，等待"
	if core {
		coreTxt = fmt.Sprintf("EMA%d向上", emaPeriod)
	}
	return &StrategyResult{
		Score: comboCap100(score),
		Factors: map[string]float64{
			"emaUp": boolScore(core), "fiPullback": fiScore, "bearExhaust": bearScore, "bullPower": bullScore,
		},
		Signal: comboJoinSignal(coreTxt, fiTxt, bearTxt),
	}
}

// ---------------------------------------------------------------------------
// 9. ComboSATSStrategy 自适应趋势组合（SATS + ATR + ADX）
// ---------------------------------------------------------------------------

// ComboSATSStrategy 自适应趋势系统：SATS 红线（bull）持股（核心），TQI 趋势质量
// 加分、ADX>25 确认；转绿无条件离场（Score 0）。
type ComboSATSStrategy struct{}

func (s *ComboSATSStrategy) Name() string { return "自适应趋势组合" }
func (s *ComboSATSStrategy) Code() string { return "combo_sats" }
func (s *ComboSATSStrategy) Description() string {
	return "SATS红线持股+TQI趋势质量+ADX走强确认，转绿无条件离场，2倍ATR移动止损"
}

func (s *ComboSATSStrategy) Score(ctx *StrategyContext) *StrategyResult {
	n := len(ctx.CloseP)
	if n < 30 {
		return &StrategyResult{Score: 0, Factors: map[string]float64{}, Signal: "K线数据不足"}
	}
	adxTh := comboOverride(ctx, "adx_threshold", 25)

	line, bull, tqi := indicator.SATS(ctx.HighP, ctx.LowP, ctx.CloseP, ctx.Volume)
	line0, lok := indicator.LastValid(line)
	if !lok {
		return &StrategyResult{Score: 0, Factors: map[string]float64{}, Signal: "K线数据不足（SATS未成形）"}
	}

	// 离场：SATS 转绿，无条件离场
	if !bull[n-1] {
		return &StrategyResult{Score: 0, Factors: map[string]float64{"satsBull": 0}, Signal: "离场信号：SATS转绿，无条件离场"}
	}

	// 核心：SATS 红线持股看多（转绿已在上一步离场，走到这里 core 恒成立）

	// 规则A：TQI 趋势质量（0~1，越高趋势质量越好）
	tqiScore, tqiTxt := 0.0, ""
	if q, ok := indicator.LastValid(tqi); ok {
		tqiScore = comboClamp01(q)
		tqiTxt = fmt.Sprintf("TQI%.2f", tqiScore)
	} else {
		tqiTxt = "TQI数据不足"
	}

	// 规则B：ADX>25 确认趋势成立，震荡期不追信号
	adxScore, adxTxt := 0.0, ""
	if v, ok := indicator.LastValid(adxOf(ctx)); ok {
		switch {
		case v >= adxTh:
			adxScore, adxTxt = 1.0, fmt.Sprintf("ADX%.1f", v)
		case v >= 20:
			adxScore, adxTxt = 0.4, fmt.Sprintf("ADX%.1f偏弱", v)
		}
	}

	// 规则C：价格仍在 SATS 线上方（通道内持股有效）
	aboveScore := 0.0
	if ctx.CloseP[n-1] > line0 {
		aboveScore = 1.0
	}

	// 规则D：ATR 波动可控（止损=入场价-2×ATR，波动小空间更从容）
	atrScore, atrTxt := 0.0, ""
	atr := calcATR(ctx.HighP, ctx.LowP, ctx.CloseP, 14)
	if atr > 0 && ctx.CloseP[n-1] > 0 {
		atrPct := atr / ctx.CloseP[n-1]
		atrTxt = fmt.Sprintf("ATR%.1f%%", atrPct*100)
		switch {
		case atrPct <= 0.05:
			atrScore = 1.0
		case atrPct <= 0.09:
			atrScore = 0.5
		}
	}

	score := 40 + 20*tqiScore + 20*adxScore + 10*aboveScore + 10*atrScore
	return &StrategyResult{
		Score: comboCap100(score),
		Factors: map[string]float64{
			"satsBull": 1, "tqi": tqiScore, "adxStrong": adxScore, "aboveLine": aboveScore, "atrCalm": atrScore,
		},
		Signal: comboJoinSignal("SATS红线持股", tqiTxt, adxTxt, atrTxt),
	}
}

// ---------------------------------------------------------------------------
// 10. ComboSMCStrategy 聪明钱结构组合（SMC + MACD + 信号比）
// ---------------------------------------------------------------------------

// ComboSMCStrategy 跟踪机构资金的市场结构：最近事件为多头 BOS 顺势（核心）、
// CHoCH 后回踩结构入场、MACD 同向配合、净信号>0 共振；跌破最近摆动低点止损。
type ComboSMCStrategy struct{}

func (s *ComboSMCStrategy) Name() string { return "聪明钱结构组合" }
func (s *ComboSMCStrategy) Code() string { return "combo_smc" }
func (s *ComboSMCStrategy) Description() string {
	return "SMC结构突破BOS顺势+CHoCH后回踩+MACD同向，跌破结构低点止损"
}

func (s *ComboSMCStrategy) Score(ctx *StrategyContext) *StrategyResult {
	n := len(ctx.CloseP)
	internalLen := comboIntOverride(ctx, "internal_len", 5, 2)
	swingLen := comboIntOverride(ctx, "swing_len", 50, 5)
	if n < swingLen+10 {
		return &StrategyResult{Score: 0, Factors: map[string]float64{}, Signal: "K线数据不足（结构未成形）"}
	}

	// SMCEvents 需要 open 序列（KLineData.Open 为字符串，逐根解析，失败回退收盘价）
	openP := make([]float64, n)
	for i := 0; i < n; i++ {
		if i < len(ctx.KLines) {
			if v, err := strconv.ParseFloat(ctx.KLines[i].Open, 64); err == nil && v > 0 {
				openP[i] = v
				continue
			}
		}
		openP[i] = ctx.CloseP[i]
	}

	// 离场：跌破最近摆动低点（结构低点）
	_, downFr := indicator.FractalLevels(ctx.HighP, ctx.LowP, 2)
	if lvl, ok := comboLastLevel(downFr, 2); ok && ctx.CloseP[n-1] < lvl {
		return &StrategyResult{Score: 0, Factors: map[string]float64{"structure": 0}, Signal: fmt.Sprintf("离场信号：跌破结构低点%.2f", lvl)}
	}

	events := indicator.SMCEvents(openP, ctx.HighP, ctx.LowP, ctx.CloseP, internalLen, swingLen)
	if len(events) == 0 {
		return &StrategyResult{Score: 0, Factors: map[string]float64{"structure": 0}, Signal: "暂无SMC结构事件"}
	}
	last := events[len(events)-1]

	// 空头结构：观望（只做多系统）
	if !last.Bullish {
		txt := "空头结构突破（BOS向下），观望"
		if last.Type == "CHoCH" {
			txt = "CHoCH向下反转预警，观望"
		}
		return &StrategyResult{Score: 0, Factors: map[string]float64{"structure": 0}, Signal: txt}
	}

	// 核心：最近结构事件多头（BOS 顺势 / CHoCH 反转确立）；空头事件已在上一步观望

	// 规则A：结构强度——BOS 确认顺势，CHoCH 待回踩
	structScore, structTxt := 0.0, ""
	if last.Type == "BOS" {
		structScore, structTxt = 1.0, "BOS向上突破"
	} else {
		structScore, structTxt = 0.6, "CHoCH多头转变"
	}

	// 规则B：回踩结构位附近（突破位 ±3%）
	pullScore, pullTxt := 0.0, ""
	if last.Level > 0 {
		dev := (ctx.CloseP[n-1] - last.Level) / last.Level
		switch {
		case math.Abs(dev) <= 0.03:
			pullScore, pullTxt = 1.0, "回踩结构位"
		case dev > 0.03:
			pullScore, pullTxt = 0.3, "已远离结构位"
		}
		structTxt = fmt.Sprintf("%s@%.2f", structTxt, last.Level)
	}

	// 规则C：MACD 同向配合（金叉状态加分，fresh 金叉更佳）
	dif, dea, _ := comboMACDSeries(ctx.CloseP, 12, 26, 9)
	macdScore, macdTxt := 0.0, ""
	if d0, ok0 := indicator.LastValid(dif); ok0 {
		if s0, oks := indicator.LastValid(dea); oks {
			if indicator.CrossOver(dif, dea) {
				macdScore, macdTxt = 1.0, "MACD金叉"
			} else if d0 > s0 {
				macdScore, macdTxt = 0.75, "MACD多头"
			}
		}
	}

	// 规则D：信号比——近期净信号>0 且偏多（多事件共振）
	netScore, netTxt := 0.0, ""
	k := len(events)
	if k > 10 {
		k = 10
	}
	bullN, bearN := 0, 0
	for _, e := range events[len(events)-k:] {
		if e.Bullish {
			bullN++
		} else {
			bearN++
		}
	}
	net := bullN - bearN
	switch {
	case net > 0:
		netScore, netTxt = 1.0, fmt.Sprintf("净信号+%d", net)
	case net == 0:
		netScore, netTxt = 0.5, "净信号持平"
	}

	score := 40 + 20*structScore + 10*pullScore + 15*macdScore + 15*netScore
	return &StrategyResult{
		Score: comboCap100(score),
		Factors: map[string]float64{
			"structure": 1, "structStrength": structScore, "pullback": pullScore,
			"macdAlign": macdScore, "netSignal": netScore,
		},
		Signal: comboJoinSignal(structTxt, pullTxt, macdTxt, netTxt),
	}
}
