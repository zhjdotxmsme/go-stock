package data

import (
	"encoding/json"
	"fmt"
	"go-stock/backend/data/datasource"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ── 输出结构体 ──

type CommodityTechnicalOutput struct {
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	Trend         string  `json:"trend"`         // 多头/空头/震荡
	SupportPrice  float64 `json:"supportPrice"`  // 支撑位
	Resistance    float64 `json:"resistance"`    // 压力位
	MacdSignal    string  `json:"macdSignal"`    // MACD 信号
	Rsi           float64 `json:"rsi"`
	RiskLevel     string  `json:"riskLevel"`     // 低/中/高
	Summary       string  `json:"summary"`       // 综合判断
}

type CommodityFundamentalOutput struct {
	Code            string  `json:"code"`
	Name            string  `json:"name"`
	SupplyDemand    string  `json:"supplyDemand"`   // 供需格局描述
	DollarIndex     float64 `json:"dollarIndex"`    // 美元指数
	MacroEvents     string  `json:"macroEvents"`     // 宏观事件影响
	CftcSentiment   string  `json:"cftcSentiment"`   // CFTC 持仓情绪
	Summary         string  `json:"summary"`
}

type CorrelationPair struct {
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	PearsonR       float64 `json:"pearsonR"`      // 基于对数收益率
	Interpretation string  `json:"interpretation"`
}

type CorrelationOutput struct {
	PrimaryCode     string            `json:"primaryCode"`
	PrimaryName     string            `json:"primaryName"`
	Pairs           []CorrelationPair `json:"pairs"`
	RatioCurrent    float64           `json:"ratioCurrent"`
	RatioPercentile float64           `json:"ratioPercentile"`
	RatioDesc       string            `json:"ratioDesc"`
}

type CommodityReportOutput struct {
	Summary         string `json:"summary"`
	MarketReview    string `json:"marketReview"`
	TechnicalView   string `json:"technicalView"`
	FundamentalView string `json:"fundamentalView"`
	CorrelationView string `json:"correlationView"`
	Outlook         string `json:"outlook"`
	RiskWarning     string `json:"riskWarning"`
}

func init() {
	registerToolHandler("GetCommodityTechnicals", handleCommodityTechnicals)
	registerToolHandler("GetCommodityFundamentals", handleCommodityFundamentals)
	registerToolHandler("GetCorrelationAnalysis", handleCorrelationAnalysis)
	registerToolHandler("GetCommodityReport", handleCommodityReport)
}

func handleCommodityTechnicals(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	parms := map[string]any{}
	if err := json.Unmarshal([]byte(funcArguments), &parms); err != nil {
		return err
	}
	code, _ := parms["code"].(string)
	period, _ := parms["period"].(string)

	ctx.Ch <- toolStartMessage(ctx, "GetCommodityTechnicals")
	output, err := GetCommodityTechnicalsOutput(code, period)
	content := commodityResultToString(output, err)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, content)
	return nil
}

func handleCommodityFundamentals(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	parms := map[string]any{}
	if err := json.Unmarshal([]byte(funcArguments), &parms); err != nil {
		return err
	}
	code, _ := parms["code"].(string)

	ctx.Ch <- toolStartMessage(ctx, "GetCommodityFundamentals")
	output, err := GetCommodityFundamentalsOutput(code)
	content := commodityResultToString(output, err)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, content)
	return nil
}

func handleCorrelationAnalysis(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	parms := map[string]any{}
	if err := json.Unmarshal([]byte(funcArguments), &parms); err != nil {
		return err
	}
	primaryCode, _ := parms["primaryCode"].(string)
	secondaryCodesStr, _ := parms["secondaryCodes"].(string)

	ctx.Ch <- toolStartMessage(ctx, "GetCorrelationAnalysis")
	secondaryCodes := []string{}
	if secondaryCodesStr != "" {
		for _, s := range strings.Split(secondaryCodesStr, ",") {
			secondaryCodes = append(secondaryCodes, strings.TrimSpace(s))
		}
	}
	output, err := GetCorrelationOutput(primaryCode, secondaryCodes)
	content := commodityResultToString(output, err)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, content)
	return nil
}

func handleCommodityReport(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	parms := map[string]any{}
	if err := json.Unmarshal([]byte(funcArguments), &parms); err != nil {
		return err
	}
	codes, _ := parms["codes"].(string)
	reportType, _ := parms["reportType"].(string)

	ctx.Ch <- toolStartMessage(ctx, "GetCommodityReport")
	output, err := GetCommodityReportOutput(codes, reportType)
	content := commodityResultToString(output, err)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, content)
	return nil
}

func toolStartMessage(ctx *ToolContext, name string) map[string]any {
	return map[string]any{
		"code":              1,
		"question":          ctx.Question,
		"chatId":            ctx.StreamResponseID,
		"model":             ctx.Model,
		"reasoning_content": fmt.Sprintf("\r\n```\r\n🔧 开始调用工具：%s\r\n```\r\n", name),
		"time":              ctxCurrentTime(),
	}
}

func commodityResultToString(output any, err error) string {
	if err != nil {
		return fmt.Sprintf("工具执行失败：%v", err)
	}
	b, _ := json.Marshal(output)
	return string(b)
}

func ctxCurrentTime() string {
	return time.Now().Format(time.DateTime)
}

// ── 核心逻辑函数（同时被 OpenAI handler 与 Eino wrapper 调用） ──

func GetCommodityTechnicalsOutput(code string, period string) (*CommodityTechnicalOutput, error) {
	if period == "" {
		period = "day"
	}

	asset := FindCommodityByCode(code)
	if asset == nil {
		return nil, fmt.Errorf("未找到品种: %s", code)
	}

	api := NewCommodityApi()
	klines, err := api.GetKLine(code, period, 120)
	if err != nil || len(klines) == 0 {
		return nil, fmt.Errorf("获取 %s K 线失败", asset.Name)
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

	macdResult := calcMACD(closes, 12, 26, 9)
	lastMacd := macdResult["MACD"]
	lastSignal := macdResult["Signal"]
	rsi := calcRSI(closes, 14)

	trend := "震荡"
	if len(closes) > 0 {
		lastClose := closes[len(closes)-1]
		if ma5 > ma10 && ma10 > ma20 && lastClose > ma5 {
			trend = "多头"
		} else if ma5 < ma10 && ma10 < ma20 && lastClose < ma5 {
			trend = "空头"
		}
	}

	support := minFloat(lows[max(0, len(lows)-20):])
	resistance := maxFloat(highs[max(0, len(highs)-20):])

	macdSignal := "中性"
	if lastMacd > lastSignal && lastMacd > 0 {
		macdSignal = "金叉偏多"
	} else if lastMacd < lastSignal && lastMacd < 0 {
		macdSignal = "死叉偏空"
	}

	riskLevel := "低"
	if rsi > 70 || rsi < 30 {
		riskLevel = "高"
	} else if rsi > 60 || rsi < 40 {
		riskLevel = "中"
	}

	lastClose := closes[len(closes)-1]
	summary := fmt.Sprintf("%s(%.2f) 趋势:%s, MACD:%s, RSI:%.1f, 支撑:%.2f, 压力:%.2f, 风险:%s",
		asset.Name, lastClose, trend, macdSignal, rsi, support, resistance, riskLevel)

	return &CommodityTechnicalOutput{
		Code:         code,
		Name:         asset.Name,
		Trend:        trend,
		SupportPrice: support,
		Resistance:   resistance,
		MacdSignal:   macdSignal,
		Rsi:          rsi,
		RiskLevel:    riskLevel,
		Summary:      summary,
	}, nil
}

func GetCommodityFundamentalsOutput(code string) (*CommodityFundamentalOutput, error) {
	asset := FindCommodityByCode(code)
	if asset == nil {
		return nil, fmt.Errorf("未找到品种: %s", code)
	}

	api := NewCommodityApi()
	quote, _ := api.GetQuote(code)
	price := 0.0
	if quote != nil {
		price = quote.Price
	}

	wsApi := WallstreetcnApi{}
	dxy := 0.0
	dxyReal := wsApi.GetMarketReal([]string{"DXY.OTC"}, nil)
	if dxyReal != nil && dxyReal.Code == 20000 {
		if values, ok := dxyReal.Data.Snapshot["DXY.OTC"]; ok && len(values) > 1 {
			dxy, _ = strconv.ParseFloat(fmt.Sprintf("%v", values[1]), 64)
		}
	}

	channel := "goldc-channel"
	if code == "USCL" {
		channel = "oil-channel"
	} else if code == "XAGUSD" {
		channel = "commodity-channel"
	}

	var newsText strings.Builder
	lives := wsApi.GetLives(channel, 20, "")
	if lives != nil && len(lives.Data.Items) > 0 {
		for _, l := range lives.Data.Items {
			newsText.WriteString(l.Title + " " + l.Content + "\n")
		}
	}

	summary := fmt.Sprintf("%s 当前价格:%.2f, 美元指数:%.2f, 近期新闻:%s",
		asset.Name, price, dxy, truncateStr(newsText.String(), 200))

	return &CommodityFundamentalOutput{
		Code:          code,
		Name:          asset.Name,
		SupplyDemand:  "参考新闻资讯",
		DollarIndex:   dxy,
		MacroEvents:   truncateStr(newsText.String(), 500),
		CftcSentiment: "暂不支持 CFTC 数据",
		Summary:       summary,
	}, nil
}

func GetCorrelationOutput(primaryCode string, secondaryCodes []string) (*CorrelationOutput, error) {
	primary := FindCommodityByCode(primaryCode)
	if primary == nil {
		return nil, fmt.Errorf("未找到主品种: %s", primaryCode)
	}

	if len(secondaryCodes) == 0 {
		return nil, fmt.Errorf("请提供关联品种代码")
	}

	api := NewCommodityApi()
	primaryKlines, _ := api.GetKLine(primaryCode, "day", 60)
	if len(primaryKlines) < 20 {
		return nil, fmt.Errorf("%s 数据不足", primary.Name)
	}
	primaryReturns := calcLogReturns(primaryKlines)

	var pairs []CorrelationPair
	for _, secCode := range secondaryCodes {
		secCode = strings.TrimSpace(secCode)
		if secCode == "" {
			continue
		}
		secAsset := FindCommodityByCode(secCode)
		if secAsset == nil {
			continue
		}

		secKlines, _ := api.GetKLine(secCode, "day", 60)
		if len(secKlines) < 20 {
			continue
		}
		secReturns := calcLogReturns(secKlines)

		n := minInt(len(primaryReturns), len(secReturns))
		if n < 5 {
			continue
		}
		r := pearsonCorrelation(primaryReturns[:n], secReturns[:n])

		interp := interpretCorrelation(r)
		pairs = append(pairs, CorrelationPair{
			Code:           secCode,
			Name:           secAsset.Name,
			PearsonR:       math.Round(r*100) / 100,
			Interpretation: interp,
		})
	}

	var ratioCurrent, ratioPercentile float64
	var ratioDesc string
	if primaryCode == "XAUUSD" && containsCode(secondaryCodes, "XAGUSD") {
		ratioCurrent, ratioPercentile, ratioDesc = calcGoldSilverRatio(api)
	}

	if len(pairs) == 0 {
		return nil, fmt.Errorf("关联数据不足，无法计算")
	}

	return &CorrelationOutput{
		PrimaryCode:     primaryCode,
		PrimaryName:     primary.Name,
		Pairs:           pairs,
		RatioCurrent:    ratioCurrent,
		RatioPercentile: ratioPercentile,
		RatioDesc:       ratioDesc,
	}, nil
}

func GetCommodityReportOutput(codes string, reportType string) (*CommodityReportOutput, error) {
	if reportType == "" {
		reportType = "周报"
	}

	codeList := []string{}
	for _, c := range strings.Split(codes, ",") {
		c = strings.TrimSpace(c)
		if c != "" {
			codeList = append(codeList, c)
		}
	}

	if len(codeList) == 0 {
		return nil, fmt.Errorf("请提供品种代码")
	}

	var technicalParts []string
	var fundamentalParts []string
	for _, code := range codeList {
		tech, err := GetCommodityTechnicalsOutput(code, "day")
		if err == nil && tech != nil {
			b, _ := json.Marshal(tech)
			technicalParts = append(technicalParts, string(b))
		}

		fund, err := GetCommodityFundamentalsOutput(code)
		if err == nil && fund != nil {
			b, _ := json.Marshal(fund)
			fundamentalParts = append(fundamentalParts, string(b))
		}
	}

	return &CommodityReportOutput{
		Summary:         fmt.Sprintf("%s %s 商品分析报告", reportType, codes),
		MarketReview:    fmt.Sprintf("共分析 %d 个品种", len(codeList)),
		TechnicalView:   strings.Join(technicalParts, "\n"),
		FundamentalView: strings.Join(fundamentalParts, "\n"),
		CorrelationView: "关联分析: 待补充",
		Outlook:         "展望: 综合技术面与基本面判断",
		RiskWarning:     "风险提示: 大宗商品价格波动剧烈，投资需谨慎",
	}, nil
}

func calcGoldSilverRatio(api *CommodityApi) (float64, float64, string) {
	goldKlines, _ := api.GetKLine("XAUUSD", "day", 2)
	silverKlines, _ := api.GetKLine("XAGUSD", "day", 2)
	if len(goldKlines) == 0 || len(silverKlines) == 0 || silverKlines[len(silverKlines)-1].Close == 0 {
		return 0, 0, "金银比计算失败"
	}
	ratioCurrent := goldKlines[len(goldKlines)-1].Close / silverKlines[len(silverKlines)-1].Close

	goldHist, _ := api.GetKLine("XAUUSD", "day", 365)
	silverHist, _ := api.GetKLine("XAGUSD", "day", 365)
	if len(goldHist) > 0 && len(silverHist) > 0 {
		minLen := minInt(len(goldHist), len(silverHist))
		ratios := make([]float64, 0, minLen)
		for i := 0; i < minLen; i++ {
			if silverHist[i].Close > 0 {
				ratios = append(ratios, goldHist[i].Close/silverHist[i].Close)
			}
		}
		pctl := percentile(ratios, ratioCurrent)
		return ratioCurrent, pctl, fmt.Sprintf("金银比当前 %.1f，处于历史 %.0f%% 分位", ratioCurrent, pctl*100)
	}
	return ratioCurrent, 0, fmt.Sprintf("金银比当前 %.1f", ratioCurrent)
}

func interpretCorrelation(r float64) string {
	absR := math.Abs(r)
	if absR > 0.7 {
		if r > 0 {
			return "强正相关"
		}
		return "强负相关"
	}
	if absR > 0.4 {
		if r > 0 {
			return "中等正相关"
		}
		return "中等负相关"
	}
	if absR > 0.2 {
		if r > 0 {
			return "弱正相关"
		}
		return "弱负相关"
	}
	return "弱相关"
}

func calcLogReturns(klines []datasource.KLineBar) []float64 {
	if len(klines) < 2 {
		return nil
	}
	returns := make([]float64, len(klines)-1)
	for i := 1; i < len(klines); i++ {
		if klines[i-1].Close > 0 {
			returns[i-1] = math.Log(klines[i].Close / klines[i-1].Close)
		}
	}
	return returns
}

func pearsonCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) < 3 {
		return 0
	}
	n := len(x)
	sumX, sumY, sumXY, sumX2, sumY2 := 0.0, 0.0, 0.0, 0.0, 0.0
	for i := 0; i < n; i++ {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
		sumY2 += y[i] * y[i]
	}
	denom := math.Sqrt((float64(n)*sumX2 - sumX*sumX) * (float64(n)*sumY2 - sumY*sumY))
	if denom == 0 {
		return 0
	}
	return (float64(n)*sumXY - sumX*sumY) / denom
}

func percentile(sortedData []float64, value float64) float64 {
	if len(sortedData) == 0 {
		return 0
	}
	sorted := make([]float64, len(sortedData))
	copy(sorted, sortedData)
	sort.Float64s(sorted)
	count := 0
	for _, v := range sorted {
		if v <= value {
			count++
		}
	}
	return float64(count) / float64(len(sorted))
}

func minFloat(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	m := data[0]
	for _, v := range data {
		if v < m {
			m = v
		}
	}
	return m
}

func maxFloat(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	m := data[0]
	for _, v := range data {
		if v > m {
			m = v
		}
	}
	return m
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func containsCode(list []string, target string) bool {
	for _, s := range list {
		if strings.TrimSpace(s) == target {
			return true
		}
	}
	return false
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
