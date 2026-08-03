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
	RegisterCategoryExpert(&OilGeopoliticalExpert{})
}

type OilGeopoliticalExpert struct{}

func (e *OilGeopoliticalExpert) Role() string { return "oil_geo" }

func (e *OilGeopoliticalExpert) Categories() []models.CommodityCategory {
	return []models.CommodityCategory{models.CategoryEnergy}
}

func (e *OilGeopoliticalExpert) Run(ctx context.Context, cc *CommodityContext) (*ExpertReport, error) {
	wsApi := data.WallstreetcnApi{}

	var dataStr strings.Builder
	dataStr.WriteString(fmt.Sprintf("品种: %s(%s)\n\n", cc.Name, cc.Code))

	// 1. Seasonal context
	dataStr.WriteString("## 当前季节性背景\n")
	month := time.Now().Month()
	dataStr.WriteString(fmt.Sprintf("当前月份: %d月\n", month))
	seasonalContext := ""
	switch {
	case month >= 1 && month <= 3:
		seasonalContext = "Q1：冬季取暖需求尾声，通常季节性偏弱。中国春节假期影响需求。"
	case month >= 4 && month <= 6:
		seasonalContext = "Q2：检修季，需求平淡。炼厂春季维护降低原油加工量。"
	case month >= 7 && month <= 9:
		seasonalContext = "Q3：夏季驾驶季高峰，汽油需求强劲，通常季节性偏强。美国出行旺季。"
	case month >= 10 && month <= 12:
		seasonalContext = "Q4：冬季取暖需求启动，同时炼厂秋季检修结束。年底补库存。"
	}
	dataStr.WriteString(fmt.Sprintf("季节性特征: %s\n", seasonalContext))

	// 2. Economic calendar
	dataStr.WriteString("\n## 经济日历（未来7天）\n")
	calendarStr := wsApi.GetCalendarReadable(7)
	if strings.TrimSpace(calendarStr) != "" {
		dataStr.WriteString(calendarStr)
	} else {
		dataStr.WriteString("暂无近期经济事件数据\n")
	}

	// 3. DXY for oil correlation
	dataStr.WriteString("\n## 美元指数\n")
	dxyResult := wsApi.GetMarketReal([]string{"DXY.OTC"}, nil)
	if dxyResult != nil && dxyResult.Code == 20000 {
		if values, ok := dxyResult.Data.Snapshot["DXY.OTC"]; ok && len(values) >= 4 {
			dxyPx, _ := parseFloatFromAny(values[1])
			dxyChg, _ := parseFloatFromAny(values[2])
			dxyChgPct, _ := parseFloatFromAny(values[3])
			dataStr.WriteString(fmt.Sprintf("DXY: %.2f  涨跌 %+.2f (%.2f%%)\n",
				dxyPx, dxyChg, dxyChgPct))
			dataStr.WriteString("美元与原油关系: 美元↑ → 原油（美元计价）对非美元买家变贵 → 需求可能↓\n")
		}
	}

	// 4. News for geopolitical events
	dataStr.WriteString("\n## 地缘政治与市场新闻\n")
	// Oil-specific news
	oilNews := wsApi.GetLivesReadable("oil-channel", 20)
	if strings.TrimSpace(oilNews) == "" || oilNews == "暂无快讯数据" {
		oilNews = wsApi.GetLivesReadable("commodity-channel", 20)
	}
	dataStr.WriteString(oilNews)

	// Global macro news for broader geopolitical context
	dataStr.WriteString("\n## 全球宏观新闻（地缘事件）\n")
	globalNews := wsApi.GetLivesReadable("global-channel", 15)
	dataStr.WriteString(globalNews)

	dataStr.WriteString(fmt.Sprintf("\n\n当前时间: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	dataStr.WriteString("\n\n注意: 请重点关注中东局势、红海航运、俄乌冲突、OPEC+会议日程等地缘风险因素。")

	// Call LLM
	chatModel, err := multi.GetChatModelWithTier(ctx, "oil_geo", multi.LLMTierQuick, cc.AIConfigID)
	if err != nil {
		return &ExpertReport{Role: "oil_geo", Content: "", Summary: "模型加载失败", Rating: "neutral", Error: err.Error()}, nil
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: GetRolePrompt("commodity_oil_geo", OilGeopoliticalExpertPrompt)},
		{Role: schema.User, Content: fmt.Sprintf("请分析 %s(%s) 的地缘政治风险和季节性因素\n\n数据:\n%s\n\n用户问题: %s", cc.Name, cc.Code, dataStr.String(), cc.UserQuery)},
	}

	streamResult, err := chatModel.Stream(ctx, messages)
	if err != nil {
		logger.SugaredLogger.Errorf("oil_geo expert LLM error: %v", err)
		return &ExpertReport{Role: "oil_geo", Content: "", Summary: "分析失败", Rating: "neutral", Error: err.Error()}, nil
	}

	var content string
	for {
		chunk, err := streamResult.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			logger.SugaredLogger.Errorf("oil_geo expert stream error: %v", err)
			break
		}
		if chunk != nil {
			content += chunk.Content
			emitToken(cc, "oil_geo", chunk.Content)
		}
	}

	return &ExpertReport{
		Role:    "oil_geo",
		Content: content,
		Summary: truncateSummary(content, 100),
		Rating:  extractRating(content),
	}, nil
}
