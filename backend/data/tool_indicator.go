package data

import (
	"context"
	"fmt"
	"go-stock/backend/data/datasource"
	"go-stock/backend/data/indicator"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"math"
	"sort"
)

// IndicatorResult holds technical indicator calculation results.
type IndicatorResult struct {
	MACD map[string]float64 `json:"macd,omitempty"`
	RSI  map[string]float64 `json:"rsi,omitempty"`
	KDJ  map[string]float64 `json:"kdj,omitempty"`
	BOLL map[string]float64 `json:"boll,omitempty"`
	MA   map[string]float64 `json:"ma,omitempty"`
	SMA  float64            `json:"sma,omitempty"`
	ATR  float64            `json:"atr,omitempty"`
	OBV  float64            `json:"obv,omitempty"`
	CCI  float64            `json:"cci,omitempty"`
	WR   float64            `json:"wr,omitempty"`
	BIAS float64            `json:"bias,omitempty"`
	GAP  *GapAnalysis       `json:"gap,omitempty"`
}

// GapAnalysis 跳空缺口分析（基于检测窗口内的日线）。
// 缺口定义：当日最低价高于前一日最高价（向上跳空）或当日最高价低于前一日最低价（向下跳空）。
// 回补定义：向上缺口被后续最低价触及缺口下沿（完全回补）；向下缺口被后续最高价触及缺口上沿。
type GapAnalysis struct {
	Window        int       `json:"window"`        // 检测窗口（K线根数）
	UnfilledCount int       `json:"unfilledCount"` // 窗口内未回补缺口数
	Recent        []GapInfo `json:"recent"`        // 窗口内缺口（新→旧，最多 recentLimit 个）
	Status        string    `json:"status"`        // 面向展示的一句话状态
}

// GapInfo 单个跳空缺口
type GapInfo struct {
	Direction string  `json:"direction"` // up / down
	Date      string  `json:"date"`      // 缺口发生日 yyyy-MM-dd
	GapLow    float64 `json:"gapLow"`    // 缺口区间下沿
	GapHigh   float64 `json:"gapHigh"`   // 缺口区间上沿
	WidthPct  float64 `json:"widthPct"`  // 缺口宽度相对缺口发生日前收盘的 %
	Filled    bool    `json:"filled"`    // 是否已（完全）回补
	FillDate  string  `json:"fillDate"`  // 回补日，未回补为空
	AgeDays   int     `json:"ageDays"`   // 距最新的交易日数（0=最新一根K线）
}

// gapWindowDays 缺口检测窗口（K线根数）；gapRecentLimit 最多保留的缺口明细条数；
// gapMinWidthPct 过滤噪音微型缺口的最低宽度（相对前收盘 %）。
const (
	gapWindowDays  = 60
	gapRecentLimit = 5
	gapMinWidthPct = 0.2
)

// IndicatorSummary is a human-readable summary of key technical signals.
type IndicatorSummary struct {
	Trend      string  `json:"trend"`      // 多头 / 空头 / 震荡
	MACDSignal string  `json:"macdSignal"` // 金叉 / 死叉 / 零轴上方 / 零轴下方
	RSIValue   float64 `json:"rsiValue"`
	RSIStatus  string  `json:"rsiStatus"`  // 超买 / 超卖 / 正常
	KDJSignal  string  `json:"kdjSignal"`  // 金叉 / 死叉
	BollStatus string  `json:"bollStatus"` // 上轨 / 中轨 / 下轨
	GapStatus  string  `json:"gapStatus"`  // 跳空缺口状态一句话
	Summary    string  `json:"summary"`
}

// GetTechnicalIndicators computes technical indicators from K-line data and returns them.
// Uses the existing datasource to fetch K-line data, then computes indicators locally.
// Also checks whether the stock-sdk MCP server is running (status == "available"/"running") as a
// readiness signal for future integration with MCP-based indicator calls.
func GetTechnicalIndicators(ctx context.Context, code string, period string, count int) (*IndicatorResult, error) {
	logger.SugaredLogger.Infof("indicators requested for %s period=%s count=%d", code, period, count)

	// Check if stock-sdk MCP server is running
	var mcp models.MCPServer
	err := db.Dao.Where("name = ? AND enable = ?", "stock-sdk", true).First(&mcp).Error
	if err == nil && mcp.ID > 0 {
		if mcp.Status == "available" || mcp.Status == "running" {
			logger.SugaredLogger.Infof("stock-sdk MCP server is %s, ready for indicator calls", mcp.Status)
		} else {
			logger.SugaredLogger.Debugf("stock-sdk MCP server status: %s (not running)", mcp.Status)
		}
	}

	// Compute indicators from K-line data (fetched via the datasource router)
	return computeIndicatorsFromKLine(ctx, code, period, count)
}

// computeIndicatorsFromKLine fetches K-line data via the datasource router and computes indicators locally.
func computeIndicatorsFromKLine(ctx context.Context, code string, period string, count int) (*IndicatorResult, error) {
	klineData, err := datasource.GetRouter().GetKLine(ctx, code, period, count)
	if err != nil || klineData == nil || len(klineData.Bars) == 0 {
		logger.SugaredLogger.Warnf("indicators: no kline data for %s: %v", code, err)
		return &IndicatorResult{}, nil
	}

	bars := klineData.Bars
	n := len(bars)
	if n < 5 {
		return &IndicatorResult{}, nil
	}

	// KLineBar fields are already float64, no parseFloat64 needed
	close := make([]float64, n)
	high := make([]float64, n)
	low := make([]float64, n)
	volume := make([]float64, n)
	for i, k := range bars {
		close[i] = k.Close
		high[i] = k.High
		low[i] = k.Low
		volume[i] = float64(k.Volume)
	}

	result := &IndicatorResult{}

	// MA (5, 10, 20, 60)
	maPeriods := []int{5, 10, 20, 60}
	result.MA = make(map[string]float64)
	for _, p := range maPeriods {
		if n >= p {
			result.MA[fmt.Sprintf("MA%d", p)] = calcSMA(close, p)
		}
	}
	result.SMA = result.MA["MA5"]

	// MACD (12, 26, 9)
	if n >= 26 {
		macd := calcMACD(close, 12, 26, 9)
		result.MACD = macd
	}

	// RSI (14)
	if n >= 14 {
		rsiVal := calcRSI(close, 14)
		result.RSI = map[string]float64{"RSI14": rsiVal}
	}

	// KDJ (9, 3, 3)
	if n >= 9 {
		kdj := calcKDJ(high, low, close, 9, 3)
		result.KDJ = kdj
	}

	// BOLL (20, 2)
	if n >= 20 {
		boll := calcBOLL(close, 20, 2.0)
		result.BOLL = boll
	}

	// WR (14)
	if n >= 14 {
		wr := calcWR(high, low, close, 14)
		result.WR = wr
	}

	// CCI (20)
	if n >= 20 {
		cci := calcCCI(high, low, close, 20)
		result.CCI = cci
	}

	// ATR (14)
	if n >= 14 {
		atr := calcATR(high, low, close, 14)
		result.ATR = atr
	}

	// OBV
	obv := calcOBV(close, volume)
	result.OBV = obv

	// BIAS (5)
	if n >= 5 {
		bias := calcBIAS(close, 5)
		result.BIAS = bias
	}

	// 跳空缺口（向上/向下跳空与回补状态）
	result.GAP = calcGapAnalysis(bars, gapWindowDays)

	logger.SugaredLogger.Infof("indicators computed for %s: MA=%.2f MACD=%.2f RSI=%.2f KDJ_K=%.2f",
		code, result.MA["MA5"], result.MACD["MACD"], result.RSI["RSI14"], result.KDJ["K"])

	return result, nil
}

// calcGapAnalysis 在最近 window 根 K 线内检测跳空缺口与回补状态。
// bars 按时间升序；窗口不足时按实际数据计算。缺口明细按新→旧排列，最多保留 gapRecentLimit 条。
func calcGapAnalysis(bars []datasource.KLineBar, window int) *GapAnalysis {
	n := len(bars)
	if window <= 0 {
		window = gapWindowDays
	}
	if n < 2 {
		return &GapAnalysis{Window: window, Status: "数据不足，无法检测缺口"}
	}
	start := 0
	if n > window {
		start = n - window
	}

	var gaps []GapInfo
	for i := start + 1; i < n; i++ {
		prev, cur := bars[i-1], bars[i]
		if prev.High <= 0 || prev.Low <= 0 {
			continue
		}
		gap := GapInfo{Date: cur.Time.Format("2006-01-02"), AgeDays: n - 1 - i}
		switch {
		case cur.Low > prev.High:
			// 向上跳空：缺口区间 [前高, 当日低]
			gap.Direction = "up"
			gap.GapLow, gap.GapHigh = prev.High, cur.Low
		case cur.High < prev.Low:
			// 向下跳空：缺口区间 [当日高, 前低]
			gap.Direction = "down"
			gap.GapLow, gap.GapHigh = cur.High, prev.Low
		default:
			continue
		}
		width := gap.GapHigh - gap.GapLow
		if prev.Close > 0 {
			gap.WidthPct = round2(width / prev.Close * 100)
		}
		if gap.WidthPct < gapMinWidthPct {
			continue // 过滤噪音微型缺口
		}
		// 回补检测：向上缺口看后续最低价是否触及缺口下沿；向下缺口看后续最高价是否触及缺口上沿
		for j := i + 1; j < n; j++ {
			if gap.Direction == "up" && bars[j].Low <= gap.GapLow {
				gap.Filled = true
				gap.FillDate = bars[j].Time.Format("2006-01-02")
				break
			}
			if gap.Direction == "down" && bars[j].High >= gap.GapHigh {
				gap.Filled = true
				gap.FillDate = bars[j].Time.Format("2006-01-02")
				break
			}
		}
		gaps = append(gaps, gap)
	}

	analysis := &GapAnalysis{Window: window}
	// 新→旧排列，最多保留 recentLimit 条明细
	for i := len(gaps) - 1; i >= 0 && len(analysis.Recent) < gapRecentLimit; i-- {
		analysis.Recent = append(analysis.Recent, gaps[i])
	}
	for _, g := range analysis.Recent {
		if !g.Filled {
			analysis.UnfilledCount++
		}
	}
	analysis.Status = gapStatusText(analysis)
	return analysis
}

// gapStatusText 生成面向展示的缺口状态一句话。
func gapStatusText(a *GapAnalysis) string {
	if a == nil || len(a.Recent) == 0 {
		return fmt.Sprintf("近%d根K线无跳空缺口", a.Window)
	}
	var latest *GapInfo
	for i := range a.Recent {
		if !a.Recent[i].Filled {
			latest = &a.Recent[i]
			break
		}
	}
	unfilled := a.UnfilledCount
	if latest == nil {
		return fmt.Sprintf("近%d根K线共%d个缺口，均已回补", a.Window, len(a.Recent))
	}
	dir := "向上"
	role := "支撑"
	if latest.Direction == "down" {
		dir = "向下"
		role = "压力"
	}
	text := fmt.Sprintf("最近未回补缺口：%s跳空 %s（%.2f-%.2f，第%d个交易日前），构成%s",
		dir, latest.Date, latest.GapLow, latest.GapHigh, latest.AgeDays+1, role)
	if unfilled > 1 {
		text += fmt.Sprintf("；近%d根K线还有%d个未回补缺口", a.Window, unfilled)
	}
	return text
}

// GetIndicatorSummary generates a human-readable summary of technical indicators.
func GetIndicatorSummary(result *IndicatorResult) *IndicatorSummary {
	if result == nil {
		return &IndicatorSummary{Trend: "数据不足", Summary: "无技术指标数据"}
	}

	s := &IndicatorSummary{}

	// Trend judgment based on MA alignment
	ma5 := result.MA["MA5"]
	ma10 := result.MA["MA10"]
	ma20 := result.MA["MA20"]
	if ma5 > 0 && ma10 > 0 && ma20 > 0 {
		if ma5 > ma10 && ma10 > ma20 {
			s.Trend = "多头排列"
		} else if ma5 < ma10 && ma10 < ma20 {
			s.Trend = "空头排列"
		} else {
			s.Trend = "震荡"
		}
	} else {
		s.Trend = "数据不足"
	}

	// MACD signal
	if v, ok := result.MACD["MACD"]; ok {
		if signal, ok := result.MACD["Signal"]; ok {
			if v > signal && result.MACD["Histogram"] > 0 {
				s.MACDSignal = "金叉,零轴上方"
			} else if v > signal && result.MACD["Histogram"] < 0 {
				s.MACDSignal = "金叉,零轴下方"
			} else if v < signal && result.MACD["Histogram"] > 0 {
				s.MACDSignal = "死叉,零轴上方"
			} else {
				s.MACDSignal = "死叉,零轴下方"
			}
		}
	}

	// RSI
	if rsi, ok := result.RSI["RSI14"]; ok {
		s.RSIValue = rsi
		if rsi > 70 {
			s.RSIStatus = "超买"
		} else if rsi < 30 {
			s.RSIStatus = "超卖"
		} else {
			s.RSIStatus = "正常"
		}
	}

	// KDJ signal
	if k, ok := result.KDJ["K"]; ok {
		if d, ok := result.KDJ["D"]; ok {
			if k > d {
				s.KDJSignal = "金叉"
			} else {
				s.KDJSignal = "死叉"
			}
		}
	}

	// BOLL status
	if mid, ok := result.BOLL["Mid"]; ok {
		if up, ok := result.BOLL["Up"]; ok {
			if result.SMA > up {
				s.BollStatus = "上轨上方"
			} else if result.SMA > mid {
				s.BollStatus = "中轨上方"
			} else {
				s.BollStatus = "下轨附近"
			}
			_ = up // reference
			_ = mid
		}
	}

	// 跳空缺口状态
	if result.GAP != nil {
		s.GapStatus = result.GAP.Status
	}

	s.Summary = fmt.Sprintf("趋势:%s MACD:%s RSI:%.0f(%s)",
		s.Trend, s.MACDSignal, s.RSIValue, s.RSIStatus)
	if s.GapStatus != "" {
		s.Summary += "；" + s.GapStatus
	}

	return s
}

// --- Technical Indicator Calculations ---

func parseFloat64(s string) float64 {
	var v float64
	fmt.Sscanf(s, "%f", &v)
	return v
}

func calcSMA(data []float64, period int) float64 {
	if len(data) < period {
		return 0
	}
	start := len(data) - period
	sum := 0.0
	for i := start; i < len(data); i++ {
		sum += data[i]
	}
	return round2(sum / float64(period))
}

// calcMACD 统一委托 indicator.MACD（SMA 种子 EMA，talib/同花顺口径），取最新有效值。
// 注意：DIF/DEA 预热需 slow+signal-1≈34 根 K 线，不足返回全 0（旧实现
// 混用 SMA 种子与首值种子两种 EMA 口径，DIF 与 DEA 不可比，已修复）。
func calcMACD(close []float64, fast, slow, signal int) map[string]float64 {
	dif, dea, hist := indicator.MACD(close, fast, slow, signal)
	d, dok := indicator.LastValid(dif)
	s, sok := indicator.LastValid(dea)
	h, hok := indicator.LastValid(hist)
	if !dok || !sok || !hok {
		return map[string]float64{"MACD": 0, "Signal": 0, "Histogram": 0}
	}
	return map[string]float64{
		"MACD":      round2(d),
		"Signal":    round2(s),
		"Histogram": round2(h),
	}
}

// calcRSI 统一委托 indicator.RSI（Cutler 式 SMA 平滑），取最新有效值。
// 数据不足或全平（无涨跌）时返回中性值 50。
func calcRSI(data []float64, period int) float64 {
	if v, ok := indicator.LastValid(indicator.RSI(data, period)); ok {
		return round2(v)
	}
	return 50
}

// calcKDJ 统一委托 indicator.KDJ（同花顺口径：RSV 全序列 + SMA(X,3,1) 递推，初值 50），
// 取最新值。数据不足 n 根时返回中性 50/50/50（旧单日近似口径已废弃，数值对齐软件）。
func calcKDJ(high, low, close []float64, n, k int) map[string]float64 {
	if len(close) < n {
		return map[string]float64{"K": 50, "D": 50, "J": 50}
	}
	kS, dS, jS := indicator.KDJ(high, low, close, n, k, k)
	kk, _ := indicator.LastValid(kS)
	dd, _ := indicator.LastValid(dS)
	jj, _ := indicator.LastValid(jS)
	return map[string]float64{
		"K": round2(kk),
		"D": round2(dd),
		"J": round2(jj),
	}
}

// calcBOLL 统一委托 indicator.BOLL（SMA + 总体标准差，同花顺口径），取最新有效值。
func calcBOLL(close []float64, period int, multiplier float64) map[string]float64 {
	if len(close) < period {
		return map[string]float64{"Mid": 0, "Up": 0, "Down": 0}
	}
	midS, upS, lowS := indicator.BOLL(close, period, multiplier)
	mid, mok := indicator.LastValid(midS)
	up, uok := indicator.LastValid(upS)
	down, dok := indicator.LastValid(lowS)
	if !mok || !uok || !dok {
		return map[string]float64{"Mid": 0, "Up": 0, "Down": 0}
	}
	return map[string]float64{
		"Mid":  round2(mid),
		"Up":   round2(up),
		"Down": round2(down),
	}
}

func calcWR(high, low, close []float64, period int) float64 {
	n := len(close)
	if n < period {
		return -50
	}
	start := n - period
	hh := high[start]
	ll := low[start]
	for i := start; i < n; i++ {
		if high[i] > hh {
			hh = high[i]
		}
		if low[i] < ll {
			ll = low[i]
		}
	}
	if hh == ll {
		return -50
	}
	return round2((hh - close[n-1]) / (hh - ll) * -100)
}

func calcCCI(high, low, close []float64, period int) float64 {
	n := len(close)
	if n < period {
		return 0
	}
	start := n - period
	tp := make([]float64, period)
	sum := 0.0
	for i := 0; i < period; i++ {
		idx := start + i
		tp[i] = (high[idx] + low[idx] + close[idx]) / 3
		sum += tp[i]
	}
	mean := sum / float64(period)

	md := 0.0
	for i := 0; i < period; i++ {
		md += math.Abs(tp[i] - mean)
	}
	md /= float64(period)

	if md == 0 {
		return 0
	}
	return round2((tp[period-1] - mean) / (0.015 * md))
}

// calcATR 统一委托 indicator.ATR（Wilder 递推，同花顺/通达信口径），取最新有效值。
// 旧「最后 period 根 TR 简单平均」口径与软件数值不一致，已废弃。
func calcATR(high, low, close []float64, period int) float64 {
	if v, ok := indicator.LastValid(indicator.ATR(high, low, close, period)); ok {
		return round2(v)
	}
	return 0
}

func calcOBV(close, volume []float64) float64 {
	if len(close) < 2 {
		return 0
	}
	obv := 0.0
	// Calculate from the beginning of the available data
	start := 0
	if len(close) > 30 {
		start = len(close) - 30
	}
	for i := start + 1; i < len(close); i++ {
		if close[i] > close[i-1] {
			obv += volume[i]
		} else if close[i] < close[i-1] {
			obv -= volume[i]
		}
	}
	return round2(obv)
}

func calcBIAS(close []float64, period int) float64 {
	n := len(close)
	if n < period {
		return 0
	}
	ma := calcSMA(close, period)
	if ma == 0 {
		return 0
	}
	return round2((close[n-1] - ma) / ma * 100)
}

// round2 rounds a float64 to 2 decimal places.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// SortMapKeys returns sorted keys of a map for deterministic iteration.
// Not used in computation, available for display purposes.
func SortMapKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
