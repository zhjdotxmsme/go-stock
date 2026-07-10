package commodity

import (
	"context"
	"errors"
	"fmt"
	"go-stock/backend/agent/multi"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"io"
	"strconv"
	"strings"

	"github.com/cloudwego/eino/schema"
)

func init() {
	RegisterExpert(&SupplyExpert{})
}

type SupplyExpert struct{}

func (e *SupplyExpert) Role() string { return "supply" }

var supplyToChannel = map[string]string{
	"XAUUSD": "goldc-channel",
	"XAGUSD": "goldc-channel",
	"USCL":   "oil-channel",
	"AU":     "goldc-channel",
	"AG":     "goldc-channel",
	"SC":     "oil-channel",
}

func (e *SupplyExpert) Run(ctx context.Context, cc *CommodityContext) (*ExpertReport, error) {
	wsApi := data.WallstreetcnApi{}
	cotApi := data.NewCotApi()

	channel := supplyToChannel[cc.Code]
	if channel == "" {
		channel = "commodity-channel"
	}

	newsStr := wsApi.GetLivesReadable(channel, 30)
	if strings.TrimSpace(newsStr) == "" || newsStr == "暂无快讯数据" {
		newsStr = wsApi.GetLivesReadable("commodity-channel", 30)
	}

	var dataStr strings.Builder
	dataStr.WriteString(fmt.Sprintf("品种: %s(%s)\n\n", cc.Name, cc.Code))

	// Add COT data if available
	dataStr.WriteString("## CFTC 持仓数据\n")
	cotCode := map[string]string{
		"XAUUSD": "GC",
		"XAGUSD": "SI",
		"USCL":   "CL",
	}[cc.Code]

	if cotCode != "" {
		latestCot, cotErr := cotApi.GetLatestCotData(cotCode)
		if cotErr == nil {
			dataStr.WriteString(fmt.Sprintf("报告日期: %s\n", latestCot.Date.Format("2006-01-02")))
			dataStr.WriteString(fmt.Sprintf("未平仓合约: %s\n", formatNumber(int64(latestCot.OpenInterest))))

			// Asset Manager positions (managed money)
			assetMgrNet := latestCot.AssetManagerLong - latestCot.AssetManagerShort
			dataStr.WriteString(fmt.Sprintf("资产管理人净持仓: %s\n", formatNumber(assetMgrNet)))
			dataStr.WriteString(fmt.Sprintf("  多头: %s\n", formatNumber(latestCot.AssetManagerLong)))
			dataStr.WriteString(fmt.Sprintf("  空头: %s\n", formatNumber(latestCot.AssetManagerShort)))

			// Dealer positions (commercial hedgers)
			dealerNet := latestCot.DealerLong - latestCot.DealerShort
			dataStr.WriteString(fmt.Sprintf("经销商净持仓: %s\n", formatNumber(dealerNet)))

			// Leveraged funds
			leveragedNet := latestCot.LeveragedLong - latestCot.LeveragedShort
			dataStr.WriteString(fmt.Sprintf("杠杆基金净持仓: %s\n", formatNumber(leveragedNet)))
		} else {
			dataStr.WriteString(fmt.Sprintf("COT数据获取失败: %v\n\n注：CFTC持仓是判断机构情绪的关键指标\n", cotErr))
		}

		// Get position summary with z-score
		summary, sumErr := cotApi.GetPositionSummary(cotCode)
		if sumErr == nil {
			dataStr.WriteString(fmt.Sprintf("\n## 持仓情绪分析\n"))
			dataStr.WriteString(fmt.Sprintf("净持仓: %s\n", formatNumber(summary.NetPosition)))
			dataStr.WriteString(fmt.Sprintf("z-score: %.2f\n", summary.ZScore))

			crowdedSignal := ""
			switch summary.CrowdedLevel {
			case "crowded_top":
				crowdedSignal = "警告：管理基金多头极其拥挤（z-score > 2），历史数据显示这是顶部信号"
			case "crowded_bottom":
				crowdedSignal = "信号：管理基金空头极其拥挤（z-score < -2），历史数据显示这是底部信号"
			case "moderate_top":
				crowdedSignal = "注意：管理基金多头较多（z-score > 1），需警惕反向风险"
			case "moderate_bottom":
				crowdedSignal = "注意：管理基金空头较多（z-score < -1），需警惕反向风险"
			default:
				crowdedSignal = "持仓处于正常区间"
			}
			dataStr.WriteString(fmt.Sprintf("拥挤度: %s\n", summary.CrowdedLevel))
			dataStr.WriteString(fmt.Sprintf("拥挤度解读: %s\n", crowdedSignal))
			dataStr.WriteString(fmt.Sprintf("历史分位数: %.1f%%\n", summary.HistoricalPercentile))
		} else {
			dataStr.WriteString(fmt.Sprintf("持仓情绪分析失败: %v\n", sumErr))
		}
	} else {
		dataStr.WriteString(fmt.Sprintf("COT数据暂不可用（%s 非 CFTC 交易品种）\n", cc.Code))
	}

	dataStr.WriteString("\n## 新闻数据（用于提取供需线索）\n")
	dataStr.WriteString(newsStr)
	dataStr.WriteString("\n\n注意: 供需分析结合 COT 持仓数据和新闻推断，请标注结论的可信度。\n")

	chatModel, err := multi.GetChatModelWithTier(ctx, "supply", multi.LLMTierQuick, cc.AIConfigID)
	if err != nil {
		return &ExpertReport{Role: "supply", Content: "", Summary: "模型加载失败", Rating: "neutral", Error: err.Error()}, nil
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: GetRolePrompt("commodity_supply", SupplyExpertPrompt)},
		{Role: schema.User, Content: fmt.Sprintf("请分析商品 %s(%s) 的供需基本面\n\n数据:\n%s\n\n用户问题: %s", cc.Name, cc.Code, dataStr.String(), cc.UserQuery)},
	}

	streamResult, err := chatModel.Stream(ctx, messages)
	if err != nil {
		logger.SugaredLogger.Errorf("supply expert LLM error: %v", err)
		return &ExpertReport{Role: "supply", Content: "", Summary: "分析失败", Rating: "neutral", Error: err.Error()}, nil
	}

	var content string
	for {
		chunk, err := streamResult.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			logger.SugaredLogger.Errorf("supply expert stream error: %v", err)
			break
		}
		if chunk != nil {
			content += chunk.Content
			emitToken(cc, "supply", chunk.Content)
		}
	}

	return &ExpertReport{
		Role:    "supply",
		Content: content,
		Summary: truncateSummary(content, 100),
		Rating:  extractRating(content),
	}, nil
}

// formatNumber formats large numbers with thousands separators
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
