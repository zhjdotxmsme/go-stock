package commodity

import (
	"context"
	"errors"
	"fmt"
	"go-stock/backend/agent/multi"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"io"
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
	dataStr.WriteString("## 新闻数据（用于提取供需线索）\n")
	dataStr.WriteString(newsStr)
	dataStr.WriteString("\n\n注意: 当前供需分析主要基于新闻文本推断，请标注结论的可信度。\n")

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
