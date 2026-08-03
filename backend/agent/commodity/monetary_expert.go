package commodity

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"go-stock/backend/agent/multi"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"

	"github.com/cloudwego/eino/schema"
)

func init() {
	RegisterCategoryExpert(&MonetaryExpert{})
}

type MonetaryExpert struct{}

func (e *MonetaryExpert) Role() string { return "monetary" }

func (e *MonetaryExpert) Categories() []models.CommodityCategory {
	return []models.CommodityCategory{models.CategoryPreciousMetal}
}

func (e *MonetaryExpert) Run(ctx context.Context, cc *CommodityContext) (*ExpertReport, error) {
	commodityApi := data.NewCommodityApi()
	fredApi := data.NewFredApi()
	wsApi := data.WallstreetcnApi{}
	cotApi := data.NewCotApi()
	etfApi := data.NewEtfFlowApi()

	var dataStr strings.Builder
	dataStr.WriteString(fmt.Sprintf("品种: %s(%s)\n\n", cc.Name, cc.Code))

	// 1. Enhanced macro indicators (TIPS multi-tenor, break-even, TLT/TIP)
	dataStr.WriteString("## 实际利率多期限分析\n")
	macro, macroErr := commodityApi.GetMacroIndicatorsEnhanced()
	if macroErr == nil && macro != nil {
		dataStr.WriteString(fmt.Sprintf("美元指数(DXY): %.2f\n", macro.DXY))
		dataStr.WriteString(fmt.Sprintf("美国国债收益率: 2Y=%.2f%% 5Y=%.2f%% 7Y=%.2f%% 10Y=%.2f%% 30Y=%.2f%%\n",
			macro.US2YR, macro.US5YR, macro.US7YR, macro.US10YR, macro.US30YR))
		dataStr.WriteString(fmt.Sprintf("收益率曲线形态: %s\n", macro.YieldCurve))
		if macro.US10YR > 0 && macro.US2YR > 0 {
			dataStr.WriteString(fmt.Sprintf("2s10s利差: %.2f%%\n", macro.US10YR-macro.US2YR))
		}

		// TIPS multi-tenor
		dataStr.WriteString("\n### TIPS收益率（实际利率代理）\n")
		dataStr.WriteString(fmt.Sprintf("5年TIPS: %.2f%%\n", macro.TIPS5Y))
		dataStr.WriteString(fmt.Sprintf("10年TIPS: %.2f%%\n", macro.TIPS10Y))
		dataStr.WriteString(fmt.Sprintf("20年TIPS: %.2f%%\n", macro.TIPS20Y))
		dataStr.WriteString(fmt.Sprintf("30年TIPS: %.2f%%\n", macro.TIPS30Y))

		// Real rates at multiple tenors
		if macro.US10YR > 0 && macro.TIPS10Y > 0 {
			real10Y := data.CalculateRealRate(macro.US10YR, macro.TIPS10Y)
			dataStr.WriteString(fmt.Sprintf("\n10年实际利率: %.2f%% — ", real10Y))
			switch {
			case real10Y > 2:
				dataStr.WriteString("高位，贵金属承压明显")
			case real10Y > 1:
				dataStr.WriteString("中偏高，贵金属温和承压")
			case real10Y > 0:
				dataStr.WriteString("正但偏低，贵金属中性偏多")
			default:
				dataStr.WriteString("负利率区间，贵金属强支撑")
			}
			dataStr.WriteString("\n")
		}
		if macro.US5YR > 0 && macro.TIPS5Y > 0 {
			dataStr.WriteString(fmt.Sprintf("5年实际利率: %.2f%%\n", data.CalculateRealRate(macro.US5YR, macro.TIPS5Y)))
		}
		if macro.US30YR > 0 && macro.TIPS30Y > 0 {
			dataStr.WriteString(fmt.Sprintf("30年实际利率: %.2f%%\n", data.CalculateRealRate(macro.US30YR, macro.TIPS30Y)))
		}

		// Break-even inflation
		dataStr.WriteString("\n### 盈亏平衡通胀率\n")
		dataStr.WriteString(fmt.Sprintf("5年盈亏平衡通胀: %.2f%%\n", macro.BreakEven5Y))
		dataStr.WriteString(fmt.Sprintf("10年盈亏平衡通胀: %.2f%%\n", macro.BreakEven10Y))
		beSignal := ""
		switch {
		case macro.BreakEven10Y > 2.5:
			beSignal = "高通胀预期（>2.5%），支撑贵金属"
		case macro.BreakEven10Y < 2:
			beSignal = "低通胀预期（<2%），抑制贵金属需求"
		default:
			beSignal = "通胀预期处于正常区间"
		}
		dataStr.WriteString(fmt.Sprintf("通胀预期解读: %s\n", beSignal))

		// TLT/TIP ETF
		dataStr.WriteString("\n### 债券ETF价格\n")
		if macro.TLTPrice > 0 {
			dataStr.WriteString(fmt.Sprintf("TLT(20+年美债ETF): $%.2f\n", macro.TLTPrice))
		}
		if macro.TIPPrice > 0 {
			dataStr.WriteString(fmt.Sprintf("TIP(TIPS ETF): $%.2f\n", macro.TIPPrice))
		}
	} else {
		dataStr.WriteString(fmt.Sprintf("宏观指标获取失败: %v\n", macroErr))
		// Fallback: fetch basic TIPS
		tipsRate, tipsErr := fredApi.GetTIPSRate()
		if tipsErr == nil {
			dataStr.WriteString(fmt.Sprintf("10年TIPS(后备): %.2f%%\n", tipsRate))
		}
	}

	// 2. ETF fund flows (GLD/SLV)
	dataStr.WriteString("\n## ETF资金流\n")
	etfSymbol := data.GetCommodityEtfMapping(cc.Code)
	if etfSymbol != "" {
		etfData, etfErr := etfApi.GetEtfFlow(etfSymbol)
		if etfErr == nil && etfData != nil {
			dataStr.WriteString(fmt.Sprintf("%s 总资产: $%s\n", etfSymbol, data.FormatFlow(etfData.TotalAssets)))
			if etfData.Nav > 0 {
				dataStr.WriteString(fmt.Sprintf("%s NAV: $%.2f\n", etfSymbol, etfData.Nav))
			}
		}
	}

	// 3. CFTC COT data
	dataStr.WriteString("\n## CFTC持仓数据\n")
	cotCode := map[string]string{
		"XAUUSD": "GC", "XAGUSD": "SI", "XAU": "GC", "XAG": "SI",
		"AU": "GC", "AG": "SI",
	}[cc.Code]

	if cotCode != "" {
		latestCot, cotErr := cotApi.GetLatestCotData(cotCode)
		if cotErr == nil {
			dataStr.WriteString(fmt.Sprintf("报告日期: %s\n", latestCot.Date.Format("2006-01-02")))
			dataStr.WriteString(fmt.Sprintf("未平仓合约: %s\n", formatNumber(int64(latestCot.OpenInterest))))
			assetMgrNet := latestCot.AssetManagerLong - latestCot.AssetManagerShort
			dataStr.WriteString(fmt.Sprintf("管理基金净持仓: %s (多头:%s 空头:%s)\n",
				formatNumber(assetMgrNet), formatNumber(latestCot.AssetManagerLong), formatNumber(latestCot.AssetManagerShort)))
			dealerNet := latestCot.DealerLong - latestCot.DealerShort
			dataStr.WriteString(fmt.Sprintf("经销商净持仓: %s\n", formatNumber(dealerNet)))
			leveragedNet := latestCot.LeveragedLong - latestCot.LeveragedShort
			dataStr.WriteString(fmt.Sprintf("杠杆基金净持仓: %s\n", formatNumber(leveragedNet)))
		}

		summary, sumErr := cotApi.GetPositionSummary(cotCode)
		if sumErr == nil {
			dataStr.WriteString(fmt.Sprintf("\n持仓情绪: z-score=%.2f 拥挤度=%s 历史分位=%.1f%%\n",
				summary.ZScore, summary.CrowdedLevel, summary.HistoricalPercentile))
		}
	} else {
		dataStr.WriteString("COT数据暂不可用\n")
	}

	// 4. Gold-silver ratio
	dataStr.WriteString("\n## 金银比\n")
	auQuote, auErr := commodityApi.GetQuote("XAUUSD")
	agQuote, agErr := commodityApi.GetQuote("XAGUSD")
	if auErr == nil && agErr == nil && agQuote.Price > 0 {
		ratio := auQuote.Price / agQuote.Price
		ratioSignal := ""
		switch {
		case ratio > 90:
			ratioSignal = "极高水平，白银严重低估或避险极端化"
		case ratio > 80:
			ratioSignal = "偏高水平，白银相对低估"
		case ratio < 65:
			ratioSignal = "偏低水平，白银相对高估或工业需求强劲"
		default:
			ratioSignal = "正常区间"
		}
		dataStr.WriteString(fmt.Sprintf("金银比(XAUUSD/XAGUSD): %.2f — %s\n", ratio, ratioSignal))
	}

	// 5. News for central bank / ETF clues
	dataStr.WriteString("\n## 新闻线索（央行购金/ETF/供需）\n")
	channel := "goldc-channel"
	newsStr := wsApi.GetLivesReadable(channel, 20)
	if strings.TrimSpace(newsStr) == "" || newsStr == "暂无快讯数据" {
		newsStr = wsApi.GetLivesReadable("commodity-channel", 20)
	}
	dataStr.WriteString(newsStr)

	// Call LLM
	chatModel, err := multi.GetChatModelWithTier(ctx, "monetary", multi.LLMTierQuick, cc.AIConfigID)
	if err != nil {
		return &ExpertReport{Role: "monetary", Content: "", Summary: "模型加载失败", Rating: "neutral", Error: err.Error()}, nil
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: GetRolePrompt("commodity_monetary", MonetaryExpertPrompt)},
		{Role: schema.User, Content: fmt.Sprintf("请分析 %s(%s) 的货币属性\n\n数据:\n%s\n\n用户问题: %s", cc.Name, cc.Code, dataStr.String(), cc.UserQuery)},
	}

	streamResult, err := chatModel.Stream(ctx, messages)
	if err != nil {
		logger.SugaredLogger.Errorf("monetary expert LLM error: %v", err)
		return &ExpertReport{Role: "monetary", Content: "", Summary: "分析失败", Rating: "neutral", Error: err.Error()}, nil
	}

	var content string
	for {
		chunk, err := streamResult.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			logger.SugaredLogger.Errorf("monetary expert stream error: %v", err)
			break
		}
		if chunk != nil {
			content += chunk.Content
			emitToken(cc, "monetary", chunk.Content)
		}
	}

	return &ExpertReport{
		Role:    "monetary",
		Content: content,
		Summary: truncateSummary(content, 100),
		Rating:  extractRating(content),
	}, nil
}

// formatNumber formats large numbers with thousands separators (re-exported for use by other experts)
func formatNumber(n int64) string {
	in := strconv.FormatInt(n, 10)
	if len(in) <= 3 {
		return in
	}
	var out []byte
	for i := 0; i < len(in); i++ {
		if i > 0 && (len(in)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, in[i])
	}
	return string(out)
}

// parseFloatFromAny converts an interface{} (usually string from JSON) to float64
func parseFloatFromAny(v interface{}) (float64, error) {
	return strconv.ParseFloat(fmt.Sprintf("%v", v), 64)
}
