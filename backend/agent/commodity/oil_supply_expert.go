package commodity

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"go-stock/backend/agent/multi"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"

	"github.com/cloudwego/eino/schema"
)

func init() {
	RegisterCategoryExpert(&OilSupplyExpert{})
}

type OilSupplyExpert struct{}

func (e *OilSupplyExpert) Role() string { return "oil_supply" }

func (e *OilSupplyExpert) Categories() []models.CommodityCategory {
	return []models.CommodityCategory{models.CategoryEnergy}
}

func (e *OilSupplyExpert) Run(ctx context.Context, cc *CommodityContext) (*ExpertReport, error) {
	commodityApi := data.NewCommodityApi()
	wsApi := data.WallstreetcnApi{}
	cotApi := data.NewCotApi()

	var dataStr strings.Builder
	dataStr.WriteString(fmt.Sprintf("品种: %s(%s)\n\n", cc.Name, cc.Code))

	// 1. WTI-Brent spread
	dataStr.WriteString("## WTI-Brent价差分析\n")
	usclQuote, usclErr := commodityApi.GetQuote("USCL")
	uscoQuote, uscoErr := commodityApi.GetQuote("USCO")
	if usclErr == nil && uscoErr == nil && uscoQuote.Price > 0 {
		spread := usclQuote.Price - uscoQuote.Price
		dataStr.WriteString(fmt.Sprintf("WTI(CL): $%.2f  Brent(BZ): $%.2f\n", usclQuote.Price, uscoQuote.Price))
		dataStr.WriteString(fmt.Sprintf("WTI-Brent价差: $%.2f\n", spread))
		spreadSignal := ""
		switch {
		case spread > 0:
			spreadSignal = "Brent > WTI，大西洋盆地供需偏紧（欧洲/非洲供应紧张），布伦特溢价"
		case spread < -3:
			spreadSignal = "WTI显著低于Brent，美国供给过剩或库欣库存压力"
		default:
			spreadSignal = "价差处于正常区间"
		}
		dataStr.WriteString(fmt.Sprintf("价差解读: %s\n", spreadSignal))
	} else {
		dataStr.WriteString("价差数据获取失败\n")
	}

	// 2. CFTC COT data for crude oil
	dataStr.WriteString("\n## CFTC持仓数据（原油）\n")
	cotCode := map[string]string{
		"USCL": "CL", "USCO": "CL", "SC": "CL",
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
			dataStr.WriteString(fmt.Sprintf("经销商(炼厂)净持仓: %s\n", formatNumber(dealerNet)))
			leveragedNet := latestCot.LeveragedLong - latestCot.LeveragedShort
			dataStr.WriteString(fmt.Sprintf("杠杆基金净持仓: %s\n", formatNumber(leveragedNet)))
		}

		summary, sumErr := cotApi.GetPositionSummary(cotCode)
		if sumErr == nil {
			dataStr.WriteString(fmt.Sprintf("\n持仓情绪: z-score=%.2f 拥挤度=%s 历史分位=%.1f%%\n",
				summary.ZScore, summary.CrowdedLevel, summary.HistoricalPercentile))
			crowdedSignal := ""
			switch summary.CrowdedLevel {
			case "crowded_top":
				crowdedSignal = "警告：管理基金多头极度拥挤（z-score > 2），顶部反转风险高"
			case "crowded_bottom":
				crowdedSignal = "信号：管理基金空头极度拥挤（z-score < -2），底部反弹机会"
			case "moderate_top":
				crowdedSignal = "注意：管理基金多头较多（z-score > 1），需警惕回调"
			case "moderate_bottom":
				crowdedSignal = "注意：管理基金空头较多（z-score < -1），需警惕反弹"
			default:
				crowdedSignal = "持仓处于正常区间"
			}
			dataStr.WriteString(fmt.Sprintf("拥挤度解读: %s\n", crowdedSignal))
		}
	} else {
		dataStr.WriteString("COT数据暂不可用\n")
	}

	// 3. DXY for oil context
	dataStr.WriteString("\n## 美元指数\n")
	dxyResult := wsApi.GetMarketReal([]string{"DXY.OTC"}, nil)
	if dxyResult != nil && dxyResult.Code == 20000 {
		if values, ok := dxyResult.Data.Snapshot["DXY.OTC"]; ok && len(values) >= 4 {
			dxyPx, _ := parseFloatFromAny(values[1])
			dxyChg, _ := parseFloatFromAny(values[2])
			dxyChgPct, _ := parseFloatFromAny(values[3])
			dataStr.WriteString(fmt.Sprintf("DXY: %.2f  涨跌 %+.2f (%.2f%%)\n",
				dxyPx, dxyChg, dxyChgPct))
		}
	}

	// 4. News for OPEC, inventory, supply-demand
	dataStr.WriteString("\n## 原油供需新闻\n")
	channel := "oil-channel"
	newsStr := wsApi.GetLivesReadable(channel, 30)
	if strings.TrimSpace(newsStr) == "" || newsStr == "暂无快讯数据" {
		newsStr = wsApi.GetLivesReadable("commodity-channel", 30)
	}
	dataStr.WriteString(newsStr)

	dataStr.WriteString("\n\n注意: 请从新闻中提取OPEC+政策、EIA库存数据、页岩油产量、需求预期等供需线索。")

	// Call LLM
	chatModel, err := multi.GetChatModelWithTier(ctx, "oil_supply", multi.LLMTierQuick, cc.AIConfigID)
	if err != nil {
		return &ExpertReport{Role: "oil_supply", Content: "", Summary: "模型加载失败", Rating: "neutral", Error: err.Error()}, nil
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: GetRolePrompt("commodity_oil_supply", OilSupplyExpertPrompt)},
		{Role: schema.User, Content: fmt.Sprintf("请分析 %s(%s) 的供需基本面\n\n数据:\n%s\n\n用户问题: %s", cc.Name, cc.Code, dataStr.String(), cc.UserQuery)},
	}

	streamResult, err := chatModel.Stream(ctx, messages)
	if err != nil {
		logger.SugaredLogger.Errorf("oil_supply expert LLM error: %v", err)
		return &ExpertReport{Role: "oil_supply", Content: "", Summary: "分析失败", Rating: "neutral", Error: err.Error()}, nil
	}

	var content string
	for {
		chunk, err := streamResult.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			logger.SugaredLogger.Errorf("oil_supply expert stream error: %v", err)
			break
		}
		if chunk != nil {
			content += chunk.Content
			emitToken(cc, "oil_supply", chunk.Content)
		}
	}

	return &ExpertReport{
		Role:    "oil_supply",
		Content: content,
		Summary: truncateSummary(content, 100),
		Rating:  extractRating(content),
	}, nil
}
