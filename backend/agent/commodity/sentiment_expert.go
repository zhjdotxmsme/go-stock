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
	RegisterExpert(&SentimentExpert{})
}

type SentimentExpert struct{}

func (e *SentimentExpert) Role() string { return "sentiment" }

var commodityToChannel = map[string]string{
	"XAUUSD": "goldc-channel",
	"XAGUSD": "goldc-channel",
	"USCL":   "oil-channel",
	"AU":     "goldc-channel",
	"AG":     "goldc-channel",
	"SC":     "oil-channel",
}

func (e *SentimentExpert) Run(ctx context.Context, cc *CommodityContext) (*ExpertReport, error) {
	wsApi := data.WallstreetcnApi{}

	channel := commodityToChannel[cc.Code]
	if channel == "" {
		channel = "commodity-channel"
	}

	newsStr := wsApi.GetLivesReadable(channel, 20)
	if strings.TrimSpace(newsStr) == "" || newsStr == "暂无快讯数据" {
		newsStr = wsApi.GetLivesReadable("commodity-channel", 20)
	}

	vipNewsStr := wsApi.GetLivesReadable("global-channel", 10)

	var dataStr strings.Builder
	dataStr.WriteString(fmt.Sprintf("品种: %s(%s)\n\n", cc.Name, cc.Code))
	dataStr.WriteString(newsStr)
	if vipNewsStr != "" && vipNewsStr != "暂无快讯数据" {
		dataStr.WriteString("\n---\n")
		dataStr.WriteString("### 全球宏观新闻参考\n")
		dataStr.WriteString(vipNewsStr)
	}

	chatModel, err := multi.GetChatModelWithTier(ctx, "sentiment", multi.LLMTierQuick, cc.AIConfigID)
	if err != nil {
		return &ExpertReport{Role: "sentiment", Content: "", Summary: "模型加载失败", Rating: "neutral", Error: err.Error()}, nil
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: GetRolePrompt("commodity_sentiment", SentimentExpertPrompt)},
		{Role: schema.User, Content: fmt.Sprintf("请分析商品 %s(%s) 的市场情绪\n\n新闻数据:\n%s\n\n用户问题: %s", cc.Name, cc.Code, dataStr.String(), cc.UserQuery)},
	}

	streamResult, err := chatModel.Stream(ctx, messages)
	if err != nil {
		logger.SugaredLogger.Errorf("sentiment expert LLM error: %v", err)
		return &ExpertReport{Role: "sentiment", Content: "", Summary: "分析失败", Rating: "neutral", Error: err.Error()}, nil
	}

	var content string
	for {
		chunk, err := streamResult.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			logger.SugaredLogger.Errorf("sentiment expert stream error: %v", err)
			break
		}
		if chunk != nil {
			content += chunk.Content
			emitToken(cc, "sentiment", chunk.Content)
		}
	}

	return &ExpertReport{
		Role:    "sentiment",
		Content: content,
		Summary: truncateSummary(content, 100),
		Rating:  extractRating(content),
	}, nil
}
