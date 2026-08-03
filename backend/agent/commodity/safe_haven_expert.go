package commodity

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"go-stock/backend/agent/multi"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"

	"github.com/cloudwego/eino/schema"
)

func init() {
	RegisterCategoryExpert(&SafeHavenExpert{})
}

type SafeHavenExpert struct{}

func (e *SafeHavenExpert) Role() string { return "safehaven" }

func (e *SafeHavenExpert) Categories() []models.CommodityCategory {
	return []models.CommodityCategory{models.CategoryPreciousMetal}
}

func (e *SafeHavenExpert) Run(ctx context.Context, cc *CommodityContext) (*ExpertReport, error) {
	fredApi := data.NewFredApi()
	wsApi := data.WallstreetcnApi{}

	var dataStr strings.Builder
	dataStr.WriteString(fmt.Sprintf("品种: %s(%s)\n\n", cc.Name, cc.Code))

	// 1. VIX (CBOE Volatility Index)
	dataStr.WriteString("## VIX恐慌指数\n")
	vix, vixErr := fredApi.GetVIX()
	if vixErr == nil {
		dataStr.WriteString(fmt.Sprintf("VIX最新值: %.2f\n", vix))
		vixSignal := ""
		switch {
		case vix > 30:
			vixSignal = "高恐慌区间（>30），避险需求强烈，利好贵金属"
		case vix > 20:
			vixSignal = "中等恐慌区间（20-30），避险需求适中"
		case vix > 15:
			vixSignal = "低恐慌区间（15-20），避险需求温和"
		default:
			vixSignal = "极低恐慌区间（<15），市场极度乐观，避险需求疲弱"
		}
		dataStr.WriteString(fmt.Sprintf("VIX解读: %s\n", vixSignal))
	} else {
		dataStr.WriteString(fmt.Sprintf("VIX数据获取失败: %v\n", vixErr))
	}

	// 2. Yield curve info for recession risk
	dataStr.WriteString("\n## 收益率曲线与衰退风险\n")
	macroApi := data.NewCommodityApi()
	macro, macroErr := macroApi.GetMacroIndicators()
	if macroErr == nil && macro != nil {
		dataStr.WriteString(fmt.Sprintf("收益率曲线形态: %s\n", macro.YieldCurve))
		if macro.US10YR > 0 && macro.US2YR > 0 {
			spread := macro.US10YR - macro.US2YR
			if spread < 0 {
				dataStr.WriteString(fmt.Sprintf("2s10s利差: %.2f%% (倒挂) — 衰退预警信号，利好贵金属避险需求\n", spread))
			} else {
				dataStr.WriteString(fmt.Sprintf("2s10s利差: %.2f%% (正常)\n", spread))
			}
		}
	}

	// 3. Economic calendar (next 7 days)
	dataStr.WriteString("\n## 经济日历（未来7天）\n")
	calendarStr := wsApi.GetCalendarReadable(7)
	if strings.TrimSpace(calendarStr) != "" {
		dataStr.WriteString(calendarStr)
	} else {
		dataStr.WriteString("暂无近期经济事件数据\n")
	}

	// 4. News for geopolitical/risk events
	dataStr.WriteString("\n## 避险相关新闻\n")
	channel := "goldc-channel"
	newsStr := wsApi.GetLivesReadable(channel, 15)
	if strings.TrimSpace(newsStr) == "" || newsStr == "暂无快讯数据" {
		newsStr = wsApi.GetLivesReadable("commodity-channel", 15)
	}
	dataStr.WriteString(newsStr)

	// 5. Global macro news for broader context
	dataStr.WriteString("\n## 全球宏观新闻\n")
	globalNews := wsApi.GetLivesReadable("global-channel", 10)
	dataStr.WriteString(globalNews)

	dataStr.WriteString(fmt.Sprintf("\n\n当前时间: %s\n", time.Now().Format("2006-01-02 15:04:05")))

	// Call LLM
	chatModel, err := multi.GetChatModelWithTier(ctx, "safehaven", multi.LLMTierQuick, cc.AIConfigID)
	if err != nil {
		return &ExpertReport{Role: "safehaven", Content: "", Summary: "模型加载失败", Rating: "neutral", Error: err.Error()}, nil
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: GetRolePrompt("commodity_safehaven", SafeHavenExpertPrompt)},
		{Role: schema.User, Content: fmt.Sprintf("请分析 %s(%s) 的避险情绪\n\n数据:\n%s\n\n用户问题: %s", cc.Name, cc.Code, dataStr.String(), cc.UserQuery)},
	}

	streamResult, err := chatModel.Stream(ctx, messages)
	if err != nil {
		logger.SugaredLogger.Errorf("safehaven expert LLM error: %v", err)
		return &ExpertReport{Role: "safehaven", Content: "", Summary: "分析失败", Rating: "neutral", Error: err.Error()}, nil
	}

	var content string
	for {
		chunk, err := streamResult.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			logger.SugaredLogger.Errorf("safehaven expert stream error: %v", err)
			break
		}
		if chunk != nil {
			content += chunk.Content
			emitToken(cc, "safehaven", chunk.Content)
		}
	}

	return &ExpertReport{
		Role:    "safehaven",
		Content: content,
		Summary: truncateSummary(content, 100),
		Rating:  extractRating(content),
	}, nil
}
