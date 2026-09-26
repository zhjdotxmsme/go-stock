package commodity

import (
	"context"
	"errors"
	"fmt"
	"go-stock/backend/agent/multi"
	"go-stock/backend/data"
	"go-stock/backend/data/datasource"
	"go-stock/backend/data/indicator"
	"go-stock/backend/logger"
	"io"
	"math"
	"sort"

	"github.com/cloudwego/eino/schema"
)

func init() {
	RegisterExpert(&TechnicalExpert{})
}

type TechnicalExpert struct{}

func (e *TechnicalExpert) Role() string { return "technical" }

func (e *TechnicalExpert) Run(ctx context.Context, cc *CommodityContext) (*ExpertReport, error) {
	commodityApi := data.NewCommodityApi()
	klines, err := commodityApi.GetKLine(cc.Code, "day", 120)
	dataStr := fmt.Sprintf("品种: %s(%s)\n\n## K线数据（最近120日）\n", cc.Name, cc.Code)
	if err != nil || len(klines) == 0 {
		dataStr += "K线数据获取失败，无法进行技术分析\n"
	} else {
		dataStr += e.buildTechnicalIndicators(klines)
	}

	chatModel, err := multi.GetChatModelWithTier(ctx, "technical", multi.LLMTierQuick, cc.AIConfigID)
	if err != nil {
		return &ExpertReport{Role: "technical", Content: "", Summary: "模型加载失败", Rating: "neutral", Error: err.Error()}, nil
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: GetRolePrompt("commodity_technical", TechnicalExpertPrompt)},
		{Role: schema.User, Content: fmt.Sprintf("请分析商品 %s(%s) 的技术面\n\n数据:\n%s\n\n用户问题: %s", cc.Name, cc.Code, dataStr, cc.UserQuery)},
	}

	streamResult, err := chatModel.Stream(ctx, messages)
	if err != nil {
		logger.SugaredLogger.Errorf("technical expert LLM error: %v", err)
		return &ExpertReport{Role: "technical", Content: "", Summary: "分析失败", Rating: "neutral", Error: err.Error()}, nil
	}

	var content string
	for {
		chunk, err := streamResult.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			logger.SugaredLogger.Errorf("technical expert stream error: %v", err)
			break
		}
		if chunk != nil {
			content += chunk.Content
			emitToken(cc, "technical", chunk.Content)
		}
	}

	return &ExpertReport{
		Role:    "technical",
		Content: content,
		Summary: truncateSummary(content, 100),
		Rating:  extractRating(content),
	}, nil
}

func (e *TechnicalExpert) buildTechnicalIndicators(klines []datasource.KLineBar) string {
	if len(klines) == 0 {
		return "暂无K线数据\n"
	}

	closes := make([]float64, len(klines))
	highs := make([]float64, len(klines))
	lows := make([]float64, len(klines))
	for i, k := range klines {
		closes[i] = k.Close
		highs[i] = k.High
		lows[i] = k.Low
	}

	ma5 := calcSMA(closes, 5)
	ma10 := calcSMA(closes, 10)
	ma20 := calcSMA(closes, 20)
	ma60 := calcSMA(closes, 60)

	rsi14 := calcRSI(closes, 14)
	macd, macdSignal, macdHist := calcMACD(closes, 12, 26, 9)
	atr14 := calcATR(highs, lows, closes, 14)
	bbMiddle, bbUpper, bbLower, bbWidth := calcBollinger(closes, 20, 2)

	lastClose := closes[len(closes)-1]

	periodHigh := maxSlice(highs, len(highs)-20, len(highs))
	periodLow := minSlice(lows, len(lows)-20, len(lows))
	allHigh := maxSlice(highs, 0, len(highs))
	allLow := minSlice(lows, 0, len(lows))

	var result string
	result += fmt.Sprintf("当前价格: %.2f\n\n", lastClose)

	result += "### 均线系统\n"
	result += fmt.Sprintf("MA5:  %.2f  |  %s\n", ma5, maLabel(lastClose, ma5))
	result += fmt.Sprintf("MA10: %.2f  |  %s\n", ma10, maLabel(lastClose, ma10))
	result += fmt.Sprintf("MA20: %.2f  |  %s\n", ma20, maLabel(lastClose, ma20))
	result += fmt.Sprintf("MA60: %.2f  |  %s\n", ma60, maLabel(lastClose, ma60))

	maTrend := "缠绕/震荡"
	if ma5 > ma10 && ma10 > ma20 && ma20 > ma60 {
		maTrend = "多头排列（看涨）"
	} else if ma5 < ma10 && ma10 < ma20 && ma20 < ma60 {
		maTrend = "空头排列（看跌）"
	}
	result += fmt.Sprintf("均线排列: %s\n\n", maTrend)

	result += "### 技术指标\n"
	result += fmt.Sprintf("RSI(14): %.1f  |  %s\n", rsi14, rsiLabel(rsi14))
	result += fmt.Sprintf("MACD(12,26,9): %.2f | Signal: %.2f | Histogram: %.2f\n", macd, macdSignal, macdHist)
	result += fmt.Sprintf("MACD状态: %s\n", macdLabel(macd, macdSignal, macdHist))
	result += fmt.Sprintf("ATR(14): %.2f\n", atr14)
	result += fmt.Sprintf("Bollinger(20,2): 上轨 %.2f | 中轨 %.2f | 下轨 %.2f | 带宽 %.2f%%\n", bbUpper, bbMiddle, bbLower, bbWidth*100)
	result += fmt.Sprintf("布林带状态: %s\n\n", bollingerLabel(lastClose, bbUpper, bbMiddle, bbLower))

	result += "### 支撑与压力\n"
	result += fmt.Sprintf("近期压力位(20日高): %.2f\n", periodHigh)
	result += fmt.Sprintf("近期支撑位(20日低): %.2f\n", periodLow)
	result += fmt.Sprintf("历史最高: %.2f\n", allHigh)
	result += fmt.Sprintf("历史最低: %.2f\n", allLow)

	var klinePreview string
	start := len(klines) - 10
	if start < 0 {
		start = 0
	}
	klinePreview += "\n### 最近10日K线\n"
	klinePreview += "日期        | 开盘   | 收盘   | 最高   | 最低\n"
	for i := start; i < len(klines); i++ {
		k := klines[i]
		klinePreview += fmt.Sprintf("%s | %.2f | %.2f | %.2f | %.2f\n",
			k.Time.Format("01-02"), k.Open, k.Close, k.High, k.Low)
	}

	return result + klinePreview
}

// calcMACD 统一委托 indicator.MACD（SMA 种子 EMA，talib/同花顺口径），
// 与 data 包个股 MACD 同口径；预热期不足（< slow+signal-1 根）返回 0,0,0。
func calcMACD(data []float64, fastPeriod, slowPeriod, signalPeriod int) (macd, signal, hist float64) {
	difS, deaS, histS := indicator.MACD(data, fastPeriod, slowPeriod, signalPeriod)
	d, dok := indicator.LastValid(difS)
	s, sok := indicator.LastValid(deaS)
	h, hok := indicator.LastValid(histS)
	if !dok || !sok || !hok {
		return 0, 0, 0
	}
	macd = math.Round(d*100) / 100
	signal = math.Round(s*100) / 100
	hist = math.Round(h*100) / 100
	return macd, signal, hist
}

// calcATR 统一委托 indicator.ATR（Wilder 递推，同花顺/通达信口径），取最新有效值。
// 旧「最后 period 根 TR 简单平均」实现注释谎称 Wilder 且与软件数值不一致，已废弃。
func calcATR(high, low, close []float64, period int) float64 {
	if v, ok := indicator.LastValid(indicator.ATR(high, low, close, period)); ok {
		return math.Round(v*100) / 100
	}
	return 0
}

// calcBollinger 统一委托 indicator.BOLL（SMA + 总体标准差，同花顺口径），取最新有效值；
// width=(upper-lower)/middle 按未舍入值计算。
func calcBollinger(data []float64, period int, stdDevFactor float64) (middle, upper, lower, width float64) {
	if len(data) < period {
		return 0, 0, 0, 0
	}
	midS, upS, lowS := indicator.BOLL(data, period, stdDevFactor)
	m, mok := indicator.LastValid(midS)
	u, uok := indicator.LastValid(upS)
	l, lok := indicator.LastValid(lowS)
	if !mok || !uok || !lok || m == 0 {
		return 0, 0, 0, 0
	}
	middle = m
	upper = math.Round(u*100) / 100
	lower = math.Round(l*100) / 100
	width = math.Round((u-l)/m*1000) / 1000
	return
}

func macdLabel(macd, signal, hist float64) string {
	if hist == 0 {
		return "无数据"
	}
	if hist > 0 {
		if macd > 0 {
			return "MACD 在零轴上方，多头动能"
		}
		return "MACD 金叉向上，可能转多"
	}
	if macd < 0 {
		return "MACD 在零轴下方，空头动能"
	}
	return "MACD 死叉向下，可能转空"
}

func bollingerLabel(price, upper, middle, lower float64) string {
	if price == 0 || upper == 0 || lower == 0 {
		return "无数据"
	}
	if price > upper {
		return "突破上轨（超买）"
	}
	if price < lower {
		return "跌破下轨（超卖）"
	}
	bandRange := upper - lower
	if bandRange == 0 {
		return "无数据"
	}
	position := (price - lower) / bandRange
	if position > 0.8 {
		return "接近上轨（偏强）"
	}
	if position < 0.2 {
		return "接近下轨（偏弱）"
	}
	return "位于中轨附近（震荡）"
}

func calcSMA(data []float64, period int) float64 {
	if len(data) < period || period <= 0 {
		return 0
	}
	sum := 0.0
	for i := len(data) - period; i < len(data); i++ {
		sum += data[i]
	}
	return math.Round(sum/float64(period)*100) / 100
}

// calcRSI 统一委托 indicator.RSI（Cutler 式 SMA 平滑），与 data 包个股 RSI 同口径；
// 数据不足或全平时返回 0。
func calcRSI(data []float64, period int) float64 {
	if v, ok := indicator.LastValid(indicator.RSI(data, period)); ok {
		return math.Round(v*10) / 10
	}
	return 0
}

func maxSlice(data []float64, start, end int) float64 {
	if start < 0 {
		start = 0
	}
	if end > len(data) {
		end = len(data)
	}
	if start >= end {
		return 0
	}
	sorted := make([]float64, end-start)
	copy(sorted, data[start:end])
	sort.Float64s(sorted)
	return sorted[len(sorted)-1]
}

func minSlice(data []float64, start, end int) float64 {
	if start < 0 {
		start = 0
	}
	if end > len(data) {
		end = len(data)
	}
	if start >= end {
		return 0
	}
	sorted := make([]float64, end-start)
	copy(sorted, data[start:end])
	sort.Float64s(sorted)
	return sorted[0]
}

func maLabel(price, ma float64) string {
	if ma == 0 {
		return "无数据"
	}
	if price > ma {
		return "价格在均线上方 ↑"
	}
	return "价格在均线下方 ↓"
}

func rsiLabel(rsi float64) string {
	if rsi == 0 {
		return "无数据"
	}
	if rsi >= 70 {
		return "超买区域"
	}
	if rsi <= 30 {
		return "超卖区域"
	}
	return "中性区间"
}
