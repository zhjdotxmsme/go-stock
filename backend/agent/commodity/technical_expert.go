package commodity

import (
	"context"
	"errors"
	"fmt"
	"go-stock/backend/agent/multi"
	"go-stock/backend/data"
	"go-stock/backend/data/datasource"
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
	for i, k := range klines {
		closes[i] = k.Close
	}

	ma5 := calcSMA(closes, 5)
	ma10 := calcSMA(closes, 10)
	ma20 := calcSMA(closes, 20)
	ma60 := calcSMA(closes, 60)

	rsi14 := calcRSI(closes, 14)

	lastClose := closes[len(closes)-1]

	highs := make([]float64, len(klines))
	lows := make([]float64, len(klines))
	for i, k := range klines {
		highs[i] = k.High
		lows[i] = k.Low
	}

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

	result += "\n### 支撑与压力\n"
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

func calcRSI(data []float64, period int) float64 {
	if len(data) < period+1 {
		return 0
	}
	var avgGain, avgLoss float64
	for i := len(data) - period; i < len(data); i++ {
		change := data[i] - data[i-1]
		if change > 0 {
			avgGain += change
		} else {
			avgLoss += -change
		}
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)
	if avgLoss == 0 {
		return 100
	}
	rs := avgGain / avgLoss
	return math.Round((100-(100/(1+rs)))*10) / 10
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
