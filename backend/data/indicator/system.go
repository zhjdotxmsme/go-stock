// 系统类指标：Ichimoku / Alligator / PSAR / SATS / SMC 结构事件 / ElderRay。
//
// 移植基准：frontend/src/components/kline/calc.ts（同名函数、相同默认参数、相同运算顺序）。
// 约定：返回序列与输入等长，预热期（JS null）填 math.NaN()；bool 序列无信号期为 false。

package indicator

import "math"

// periodHL 对应 ichimokuValues 内部函数 periodHL（calc.ts L340）：
// 窗口 p 内 (最高高+最低低)/2；前 p-1 位 NaN。
func periodHL(h, l []float64, p int) []float64 {
	n := len(h)
	out := nanSlice(n)
	for i := p - 1; i < n; i++ {
		hi, lo := math.Inf(-1), math.Inf(1)
		for j := 0; j < p; j++ {
			if h[i-j] > hi {
				hi = h[i-j]
			}
			if l[i-j] < lo {
				lo = l[i-j]
			}
		}
		out[i] = (hi + lo) / 2
	}
	return out
}

// Ichimoku 一目均衡表，移植自 JS ichimokuValues（calc.ts L338，
// 默认 tenkanP=9, kijunP=26, senkouBP=52）。
//
// 与 JS 返回结构的映射（本契约只导出 4 条序列）：
//   - tenkan / kijun ↔ JS tenkan / kijun；
//   - spanA ↔ JS spanA = (tenkan+kijun)/2；
//   - spanB ↔ JS senkouB = periodHL(high, low, senkouBP)。
//
// 下标语义：JS 的 spanA/senkouB 数组本身就按“当前 K 线下标 i”存放数值
// （绘图时前移 kijunP 根属于绘图层位移），Go 亦不位移、下标对齐当前 K 线。
// JS 的 chikou（chikou[i]=closes[i+kijunP] 的后视位移序列）不在本契约内，未导出。
func Ichimoku(high, low, close []float64, tenkanP, kijunP, senkouBP int) (tenkan, kijun, spanA, spanB []float64) {
	tenkan = periodHL(high, low, tenkanP)
	kijun = periodHL(high, low, kijunP)
	spanB = periodHL(high, low, senkouBP)
	n := len(close)
	spanA = nanSlice(n)
	for i := 0; i < n; i++ {
		if !math.IsNaN(tenkan[i]) && !math.IsNaN(kijun[i]) {
			spanA[i] = (tenkan[i] + kijun[i]) / 2
		}
	}
	return tenkan, kijun, spanA, spanB
}

// Alligator 鳄鱼线，移植自 JS alligatorValues（calc.ts L882，
// 默认 jawLen=13, teethLen=8, lipsLen=5, jawOffset=8, teethOffset=5, lipsOffset=3）。
// 三线均为 mid=(high+low)/2 的 SMA，再按各自 offset 向未来位移：
// Go 下标 i 的值 = SMA(mid, len)[i-off]（与 JS jaw/teeth/lips 完全一致）；
// i<off 或源头为预热期时为 NaN。
func Alligator(high, low, close []float64, jawLen, teethLen, lipsLen, jawOff, teethOff, lipsOff int) (jaw, teeth, lips []float64) {
	n := len(close)
	mid := make([]float64, n)
	for i := 0; i < n; i++ {
		mid[i] = (high[i] + low[i]) / 2
	}
	jawRaw := smaNullAsZero(mid, jawLen)
	teethRaw := smaNullAsZero(mid, teethLen)
	lipsRaw := smaNullAsZero(mid, lipsLen)
	jaw, teeth, lips = nanSlice(n), nanSlice(n), nanSlice(n)
	for i := jawOff; i < n; i++ {
		if !math.IsNaN(jawRaw[i-jawOff]) {
			jaw[i] = jawRaw[i-jawOff]
		}
	}
	for i := teethOff; i < n; i++ {
		if !math.IsNaN(teethRaw[i-teethOff]) {
			teeth[i] = teethRaw[i-teethOff]
		}
	}
	for i := lipsOff; i < n; i++ {
		if !math.IsNaN(lipsRaw[i-lipsOff]) {
			lips[i] = lipsRaw[i-lipsOff]
		}
	}
	return jaw, teeth, lips
}

// PSAR 抛物线 SAR，移植自 JS sarValues（calc.ts L405，默认 step=0.02, maxStep=0.2）。
// 返回 sar（下标 0 及 len<2 时为 NaN）与 bull（JS direction==1 → true，0 → false）。
// 翻转/加速因子的更新顺序（先 curSar=ep 再重置 ep、af）按 JS 原样移植。
func PSAR(high, low, close []float64, step, maxStep float64) (sar []float64, bull []bool) {
	n := len(close)
	sar = nanSlice(n)
	bull = make([]bool, n)
	if n < 2 {
		return sar, bull
	}
	isLong := close[1] > close[0]
	af := step
	var ep float64
	if isLong {
		ep = high[1]
	} else {
		ep = low[1]
	}
	var prevSar float64
	if isLong {
		prevSar = low[0]
	} else {
		prevSar = high[0]
	}
	sar[1] = prevSar
	bull[1] = isLong
	for i := 2; i < n; i++ {
		curSar := prevSar + af*(ep-prevSar)
		if isLong {
			curSar = math.Min(curSar, math.Min(low[i-1], low[i-2]))
			if low[i] < curSar {
				isLong = false
				curSar = ep
				ep = low[i]
				af = step
			} else if high[i] > ep {
				ep = high[i]
				af = math.Min(af+step, maxStep)
			}
		} else {
			curSar = math.Max(curSar, math.Max(high[i-1], high[i-2]))
			if high[i] > curSar {
				isLong = true
				curSar = ep
				ep = high[i]
				af = step
			} else if low[i] < ep {
				ep = low[i]
				af = math.Min(af+step, maxStep)
			}
		}
		sar[i] = curSar
		bull[i] = isLong
		prevSar = curSar
	}
	return sar, bull
}

// clamp01 对应 JS Math.max(0, Math.min(1, x))；x 为 NaN 时返回 NaN（与 JS 一致）。
func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

// satsOpts 为 JS satsValues（calc.ts L721）解构参数对象的 Go 对应物，
// 字段与默认值逐项对齐 calc.ts。
type satsOpts struct {
	atrLen          int
	baseMult        float64
	erLen           int
	adaptStrength   float64
	atrBaselineLen  int
	useAdaptive     bool
	useTqi          bool
	qualityStrength float64
	qualityCurve    float64
	smoothMult      bool
	useAsymBands    bool
	asymStrength    float64
	useEffAtr       bool
	useCharFlip     bool
	charFlipMinAge  int
	charFlipHigh    float64
	charFlipLow     float64
	tqiWeightEr     float64
	tqiWeightVol    float64
	tqiWeightStruct float64
	tqiWeightMom    float64
	tqiStructLen    int
	tqiMomLen       int
	volLen          int
	multSmoothAlpha float64
}

// defaultSatsOpts 返回 calc.ts satsValues 的默认参数。
func defaultSatsOpts() satsOpts {
	return satsOpts{
		atrLen:          14,
		baseMult:        2.0,
		erLen:           20,
		adaptStrength:   0.5,
		atrBaselineLen:  100,
		useAdaptive:     true,
		useTqi:          true,
		qualityStrength: 0.4,
		qualityCurve:    1.5,
		smoothMult:      true,
		useAsymBands:    true,
		asymStrength:    0.5,
		useEffAtr:       true,
		useCharFlip:     true,
		charFlipMinAge:  5,
		charFlipHigh:    0.55,
		charFlipLow:     0.25,
		tqiWeightEr:     0.35,
		tqiWeightVol:    0.20,
		tqiWeightStruct: 0.25,
		tqiWeightMom:    0.20,
		tqiStructLen:    20,
		tqiMomLen:       10,
		volLen:          20,
		multSmoothAlpha: 0.15,
	}
}

// satsRun 为 JS satsValues 主循环的逐行移植。
// 返回 stLine/upper/lower（预热期 NaN）、tqi（预热期 0，与 JS outTqi 填 0 一致）、
// bull（JS direction==1 → true）。atrBase 用 smaNullAsZero 复现 JS 里
// smaValues 把 rawAtr 的 null 按 0 计入窗口和的算术语义。
func satsRun(high, low, close, volume []float64, o satsOpts) (stLine, upper, lower, tqi []float64, bull []bool) {
	n := len(close)
	rawAtr := atrValues(high, low, close, o.atrLen)
	atrBase := smaNullAsZero(rawAtr, o.atrBaselineLen)
	stLine, upper, lower = nanSlice(n), nanSlice(n), nanSlice(n)
	tqi = make([]float64, n)
	bull = make([]bool, n)

	var prevLowerBand, prevUpperBand float64
	haveBands := false
	prevDir := 0
	var prevActiveMultSm, prevPassiveMultSm float64
	haveMultSm := false
	trendStartBar := 0

	tqiWeightSum := o.tqiWeightEr + o.tqiWeightVol + o.tqiWeightStruct + o.tqiWeightMom
	tqiWeightDenom := tqiWeightSum
	if tqiWeightDenom <= 0 {
		tqiWeightDenom = 1
	}

	for i := 0; i < n; i++ {
		if math.IsNaN(rawAtr[i]) || math.IsNaN(atrBase[i]) {
			continue // 对应 JS：四个输出序列该位保持 null/0，状态不重置
		}
		atrVal := rawAtr[i]
		volRatio := 1.0
		if atrBase[i] != 0 {
			volRatio = atrVal / atrBase[i]
		}
		erValue := 0.0
		if i >= o.erLen {
			change := math.Abs(close[i] - close[i-o.erLen])
			volatility := 0.0
			for j := 0; j < o.erLen; j++ {
				volatility += math.Abs(close[i-j] - close[i-j-1])
			}
			if volatility != 0 {
				erValue = change / volatility
			}
		}
		effAtr := atrVal
		if o.useEffAtr {
			effAtr = atrVal * (0.5 + 0.5*erValue)
		}
		tqiEr := clamp01(erValue)
		tqiVol := 0.5
		if volume[i] > 0 && i >= o.volLen {
			vMean := 0.0
			for j := 0; j < o.volLen; j++ {
				vMean += volume[i-j]
			}
			vMean /= float64(o.volLen)
			vStdSq := 0.0
			for j := 0; j < o.volLen; j++ {
				d := volume[i-j] - vMean
				vStdSq += d * d
			}
			vStd := math.Sqrt(vStdSq / float64(o.volLen))
			volZ := 0.0
			if vStd != 0 {
				volZ = (volume[i] - vMean) / vStd
			}
			tqiVol = clamp01((volZ + 1) / 3) // JS: (volZ-(-1))/(2-(-1))
		} else {
			tqiVol = clamp01((volRatio - 0.6) / 1.2) // JS: (volRatio-0.6)/(1.8-0.6)
		}
		tqiStruct := 0.0
		if i >= o.tqiStructLen {
			structHi, structLo := math.Inf(-1), math.Inf(1)
			for j := 0; j < o.tqiStructLen; j++ {
				if high[i-j] > structHi {
					structHi = high[i-j]
				}
				if low[i-j] < structLo {
					structLo = low[i-j]
				}
			}
			structRange := structHi - structLo
			pricePos := 0.5
			if structRange != 0 {
				pricePos = (close[i] - structLo) / structRange
			}
			tqiStruct = clamp01(math.Abs(pricePos-0.5) * 2)
		}
		tqiMom := 0.0
		if i >= o.tqiMomLen {
			windowChange := close[i] - close[i-o.tqiMomLen]
			alignedBars := 0
			for j := 0; j < o.tqiMomLen; j++ {
				barChange := close[i-j] - close[i-j-1]
				if (windowChange > 0 && barChange > 0) || (windowChange < 0 && barChange < 0) {
					alignedBars++
				}
			}
			tqiMom = float64(alignedBars) / float64(o.tqiMomLen)
		}
		tqiRaw := 0.5
		if o.useTqi {
			tqiRaw = (tqiEr*o.tqiWeightEr + tqiVol*o.tqiWeightVol +
				tqiStruct*o.tqiWeightStruct + tqiMom*o.tqiWeightMom) / tqiWeightDenom
		}
		tqiVal := clamp01(tqiRaw)
		tqi[i] = tqiVal

		legacyAdaptFactor := 1.0
		if o.useAdaptive {
			legacyAdaptFactor = 1 + o.adaptStrength*(0.5-erValue)
		}
		qualityDeviation := 0.5
		if o.useTqi {
			qualityDeviation = math.Pow(1-tqiVal, o.qualityCurve)
		}
		tqiMult := 1 - o.qualityStrength + o.qualityStrength*(0.6+0.8*qualityDeviation)
		symMult := o.baseMult * legacyAdaptFactor * tqiMult
		activeMultRaw, passiveMultRaw := symMult, symMult
		if o.useTqi && o.useAsymBands {
			asymTighten := 1 - o.asymStrength*tqiVal*0.3
			asymWiden := 1 + o.asymStrength*tqiVal*0.4
			activeMultRaw = symMult * asymTighten
			passiveMultRaw = symMult * asymWiden
		}
		activeMultSm := activeMultRaw
		passiveMultSm := passiveMultRaw
		if haveMultSm && o.smoothMult {
			activeMultSm = prevActiveMultSm*(1-o.multSmoothAlpha) + activeMultRaw*o.multSmoothAlpha
			passiveMultSm = prevPassiveMultSm*(1-o.multSmoothAlpha) + passiveMultRaw*o.multSmoothAlpha
		}
		prevActiveMultSm = activeMultSm
		prevPassiveMultSm = passiveMultSm
		haveMultSm = true

		curPrevDir := prevDir
		if curPrevDir == 0 {
			curPrevDir = 1
		}
		lowerMult, upperMult := activeMultSm, passiveMultSm
		if curPrevDir != 1 {
			lowerMult, upperMult = passiveMultSm, activeMultSm
		}
		hl2 := (high[i] + low[i]) / 2
		lowerBandRaw := hl2 - lowerMult*effAtr
		upperBandRaw := hl2 + upperMult*effAtr
		lowerBand := lowerBandRaw
		if haveBands && close[i-1] > prevLowerBand {
			lowerBand = math.Max(lowerBandRaw, prevLowerBand)
		}
		upperBand := upperBandRaw
		if haveBands && close[i-1] < prevUpperBand {
			upperBand = math.Min(upperBandRaw, prevUpperBand)
		}

		priceFlipUp := prevDir == -1 && haveBands && close[i] > prevUpperBand
		priceFlipDown := prevDir == 1 && haveBands && close[i] < prevLowerBand
		trendAge := i - trendStartBar
		prevTqi := 0.5
		if i > 0 {
			prevTqi = tqi[i-1]
		}
		charFlipCondBase := o.useCharFlip && o.useTqi && prevTqi > o.charFlipHigh &&
			tqiVal < o.charFlipLow && trendAge >= o.charFlipMinAge
		charFlipDown := charFlipCondBase && curPrevDir == 1 && i > 0 && close[i] < close[i-1]
		charFlipUp := charFlipCondBase && curPrevDir == -1 && i > 0 && close[i] > close[i-1]
		finalFlipUp := priceFlipUp || charFlipUp
		finalFlipDown := priceFlipDown || charFlipDown

		dir := curPrevDir
		if prevDir == 0 {
			dir = 1
		} else if finalFlipUp {
			dir = 1
		} else if finalFlipDown {
			dir = -1
		}
		if dir != curPrevDir {
			trendStartBar = i
		}
		prevLowerBand = lowerBand
		prevUpperBand = upperBand
		prevDir = dir
		haveBands = true

		if dir == 1 {
			stLine[i] = lowerBand
		} else {
			stLine[i] = upperBand
		}
		upper[i] = upperBand
		lower[i] = lowerBand
		bull[i] = dir == 1
	}
	return stLine, upper, lower, tqi, bull
}

// SATS 自适应趋势线（Squeeze-Adaptive Trend System），移植自 JS satsValues
// （calc.ts L721），使用 calc.ts 的全部默认参数（见 defaultSatsOpts）。
//
// 契约映射：line ↔ JS stLine（预热期 NaN）；bull ↔ JS direction==1（预热期 false）；
// tqi ↔ JS tqi（注意：JS outTqi 以 0 填充，预热期为 0 而非 NaN）。
// JS 的 upper/lower 通道带在 satsRun 内部同样计算并参与黄金值对照，契约未导出。
// TQI（Trend Quality Index）为四分量加权：ER 效率比(0.35) + 量能 Z 分数(0.20) +
// 结构位置(0.25) + 动量一致性(0.20)，逐分量 clamp01 后加权再 clamp01。
func SATS(high, low, close, volume []float64) (line []float64, bull []bool, tqi []float64) {
	st, _, _, tq, bullSeq := satsRun(high, low, close, volume, defaultSatsOpts())
	return st, bullSeq, tq
}

// ElderRay 爱尔德射线，移植自 JS elderRayValues（calc.ts L1030，默认 emaPeriod=13）。
// bullPower=high-EMA(close, emaPeriod)，bearPower=low-EMA(close, emaPeriod)；
// EMA 预热期为 NaN。
func ElderRay(high, low, close []float64, emaPeriod int) (bullPower, bearPower []float64) {
	ema := EMA(close, emaPeriod)
	n := len(close)
	bullPower, bearPower = nanSlice(n), nanSlice(n)
	for i := 0; i < n; i++ {
		if !math.IsNaN(ema[i]) {
			bullPower[i] = high[i] - ema[i]
			bearPower[i] = low[i] - ema[i]
		}
	}
	return bullPower, bearPower
}

// SMCEvent 为 Smart Money Concepts 结构事件（BOS / CHoCH）。
type SMCEvent struct {
	Index   int     // 事件所在 K 线下标（JS 事件的 time，即摆动点下标 i）
	Type    string  // "BOS"（趋势延续的结构破坏）或 "CHoCH"（趋势转变）
	Bullish bool    // true=多头事件（向上突破/转多），false=空头事件
	Level   float64 // 被突破/转变的前一摆动点价位（JS 事件的 fromPrice）
}

// SMCEvents 结构事件，移植自 JS smcValues（calc.ts L1182）的内部结构状态机
// （对应 JS 返回的 bosLines + chochLines，基于 internalLen 内部摆动点）。
//
// 判定（与 JS 逐行一致）：
//   - 内部摆动点：strict fractal，i∈[internalLen, n-internalLen)，highs[i] 严格大于
//     前后各 internalLen 根为摆动高点（intHighs），低点对称（intLows）；
//   - 扫描到新高点 pivot 且 pivot > 上一个内部摆动高点值时：
//     趋势为空(-1) → 记 CHoCH(bullish=true) 并转多；已为多(1) → 记 BOS(bullish=true)；
//     未定(0) → 仅置多不记事件；低点对称（更低低 → CHoCH/BOS(bullish=false)）；
//   - lastHighVal/lastLowVal 无条件更新为最近摆动点值（即便未创新高/新低）。
//
// 输出顺序：整体按 i 升序；同一根 K 线先高点事件后低点事件（与 JS 扫描顺序一致，
// JS 的 bosLines/chochLines 两条数组在此合并为一条时间序事件流）。
//
// 参数说明：open 仅对齐 JS 签名（BOS/CHoCH 判定不使用 open，JS 中 open 只用于其
// 订单块识别部分）；swingLen 仅对齐 JS 签名——JS 另有按 swingLen 大级别摆动点推导的
// swingBosLines/swingChochLines（状态机逻辑与本函数完全相同），因契约只保留单一
// 事件流而未导出。level 取 JS 事件的 fromPrice（被突破价位），toPrice（新摆动点
// 价位）契约未含、未导出。
func SMCEvents(open, high, low, close []float64, internalLen, swingLen int) []SMCEvent {
	_ = open     // 见函数文档：事件判定不使用 open
	_ = swingLen // 见函数文档：大级别 swing 事件未导出
	n := len(close)

	intHighs, intLows := nanSlice(n), nanSlice(n)
	for i := internalLen; i < n-internalLen; i++ {
		isHigh, isLow := true, true
		for j := 1; j <= internalLen; j++ {
			if high[i] <= high[i-j] || high[i] <= high[i+j] {
				isHigh = false
			}
			if low[i] >= low[i-j] || low[i] >= low[i+j] {
				isLow = false
			}
			if !isHigh && !isLow {
				break
			}
		}
		if isHigh {
			intHighs[i] = high[i]
		}
		if isLow {
			intLows[i] = low[i]
		}
	}

	events := make([]SMCEvent, 0, 8)
	lastHighIdx, lastLowIdx := -1, -1
	lastHighVal, lastLowVal := math.Inf(-1), math.Inf(1)
	trend := 0
	for i := 0; i < n; i++ {
		if !math.IsNaN(intHighs[i]) {
			if lastHighIdx >= 0 && intHighs[i] > lastHighVal {
				switch {
				case trend == -1:
					events = append(events, SMCEvent{Index: i, Type: "CHoCH", Bullish: true, Level: lastHighVal})
					trend = 1
				case trend == 1:
					events = append(events, SMCEvent{Index: i, Type: "BOS", Bullish: true, Level: lastHighVal})
				}
				if trend == 0 {
					trend = 1
				}
			}
			lastHighIdx = i
			lastHighVal = intHighs[i]
		}
		if !math.IsNaN(intLows[i]) {
			if lastLowIdx >= 0 && intLows[i] < lastLowVal {
				switch {
				case trend == 1:
					events = append(events, SMCEvent{Index: i, Type: "CHoCH", Bullish: false, Level: lastLowVal})
					trend = -1
				case trend == -1:
					events = append(events, SMCEvent{Index: i, Type: "BOS", Bullish: false, Level: lastLowVal})
				}
				if trend == 0 {
					trend = -1
				}
			}
			lastLowIdx = i
			lastLowVal = intLows[i]
		}
	}
	return events
}
