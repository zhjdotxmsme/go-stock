package data

import (
	"context"
	"fmt"
	"go-stock/backend/data/datasource"
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
}

// IndicatorSummary is a human-readable summary of key technical signals.
type IndicatorSummary struct {
	Trend      string // 多头 / 空头 / 震荡
	MACDSignal string // 金叉 / 死叉 / 零轴上方 / 零轴下方
	RSIValue   float64
	RSIStatus  string // 超买 / 超卖 / 正常
	KDJSignal  string // 金叉 / 死叉
	BollStatus string // 上轨 / 中轨 / 下轨
	Summary    string
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

	logger.SugaredLogger.Infof("indicators computed for %s: MA=%.2f MACD=%.2f RSI=%.2f KDJ_K=%.2f",
		code, result.MA["MA5"], result.MACD["MACD"], result.RSI["RSI14"], result.KDJ["K"])

	return result, nil
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

	s.Summary = fmt.Sprintf("趋势:%s MACD:%s RSI:%.0f(%s)",
		s.Trend, s.MACDSignal, s.RSIValue, s.RSIStatus)

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

func calcMACD(close []float64, fast, slow, signal int) map[string]float64 {
	n := len(close)
	emaFast := calcEMA(close, fast)
	emaSlow := calcEMA(close, slow)
	macdLine := emaFast - emaSlow

	// Build MACD line history for signal calculation
	macdHistory := make([]float64, n)
	emaF := calcEMAFirst(close, fast)
	emaS := calcEMAFirst(close, slow)
	for i := 0; i < n; i++ {
		if i == 0 {
			emaF = close[i]
			emaS = close[i]
		} else {
			emaF = emaF + (2.0/(float64(fast)+1))*(close[i]-emaF)
			emaS = emaS + (2.0/(float64(slow)+1))*(close[i]-emaS)
		}
		macdHistory[i] = emaF - emaS
	}

	signalLine := calcEMALast(macdHistory, signal)
	histogram := macdLine - signalLine

	return map[string]float64{
		"MACD":      round2(macdLine),
		"Signal":    round2(signalLine),
		"Histogram": round2(histogram),
	}
}

func calcEMAFirst(data []float64, period int) float64 {
	sum := 0.0
	for i := 0; i < period && i < len(data); i++ {
		sum += data[i]
	}
	return sum / float64(period)
}

func calcEMA(data []float64, period int) float64 {
	n := len(data)
	if n < period {
		return 0
	}
	multiplier := 2.0 / (float64(period) + 1)
	ema := calcSMA(data[:period], period)
	for i := period; i < n; i++ {
		ema = (data[i]-ema)*multiplier + ema
	}
	return ema
}

func calcEMALast(data []float64, period int) float64 {
	n := len(data)
	if n < period {
		return 0
	}
	multiplier := 2.0 / (float64(period) + 1)
	ema := calcSMA(data[:period], period)
	for i := period; i < n; i++ {
		ema = (data[i]-ema)*multiplier + ema
	}
	return ema
}

func calcRSI(data []float64, period int) float64 {
	n := len(data)
	if n < period+1 {
		return 50
	}
	gains, losses := 0.0, 0.0
	for i := n - period; i < n; i++ {
		diff := data[i] - data[i-1]
		if diff > 0 {
			gains += diff
		} else {
			losses -= diff
		}
	}
	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)
	if avgLoss == 0 {
		return 100
	}
	rs := avgGain / avgLoss
	return round2(100 - 100/(1+rs))
}

func calcKDJ(high, low, close []float64, n, k int) map[string]float64 {
	length := len(high)
	if length < n {
		return map[string]float64{"K": 50, "D": 50, "J": 50}
	}

	// Find highest high and lowest low in last n periods
	start := length - n
	hh := high[start]
	ll := low[start]
	for i := start; i < length; i++ {
		if high[i] > hh {
			hh = high[i]
		}
		if low[i] < ll {
			ll = low[i]
		}
	}

	lastClose := close[length-1]
	var rsv float64
	if hh != ll {
		rsv = (lastClose - ll) / (hh - ll) * 100
	} else {
		rsv = 50
	}

	// Simplified: use single-period calculation
	kVal := 2.0/3.0*50 + 1.0/3.0*rsv
	dVal := 2.0/3.0*50 + 1.0/3.0*kVal
	jVal := 3*kVal - 2*dVal

	return map[string]float64{
		"K": round2(kVal),
		"D": round2(dVal),
		"J": round2(jVal),
	}
}

func calcBOLL(close []float64, period int, multiplier float64) map[string]float64 {
	n := len(close)
	if n < period {
		return map[string]float64{"Mid": 0, "Up": 0, "Down": 0}
	}

	start := n - period
	mid := calcSMA(close, period)

	// Calculate standard deviation
	sumSq := 0.0
	for i := start; i < n; i++ {
		diff := close[i] - mid
		sumSq += diff * diff
	}
	std := math.Sqrt(sumSq / float64(period))

	up := mid + multiplier*std
	down := mid - multiplier*std

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

func calcATR(high, low, close []float64, period int) float64 {
	n := len(close)
	if n < period+1 {
		return 0
	}
	sum := 0.0
	for i := n - period; i < n; i++ {
		tr := math.Max(high[i]-low[i], math.Abs(high[i]-close[i-1]))
		tr = math.Max(tr, math.Abs(low[i]-close[i-1]))
		sum += tr
	}
	return round2(sum / float64(period))
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
