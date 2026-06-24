package multi

import (
	"context"
	"fmt"
	"go-stock/backend/logger"
	"time"

	"github.com/cloudwego/eino/schema"
)

// RunSentimentAnalyst evaluates market sentiment and public opinion.
func RunSentimentAnalyst(ctx context.Context, ac *AgentContext) (*AgentReport, error) {
	dataStr := fmt.Sprintf("股票: %s(%s)\n分析时间: %s\n",
		ac.StockName, ac.StockCode, time.Now().Format("2006-01-02 15:04:05"))

	chatModel, err := GetChatModel(ctx, "sentiment", ac.AIConfigID)
	if err != nil {
		return &AgentReport{Role: "sentiment", Content: "", Summary: "模型加载失败", Rating: "neutral", Error: err.Error()}, nil
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: SentimentAnalystPrompt},
		{Role: schema.User, Content: fmt.Sprintf("请分析股票 %s(%s) 的市场情绪\n\n数据:\n%s", ac.StockName, ac.StockCode, dataStr)},
	}

	result, err := chatModel.Generate(ctx, messages)
	if err != nil {
		logger.SugaredLogger.Errorf("sentiment analyst LLM error: %v", err)
		return &AgentReport{Role: "sentiment", Content: "", Summary: "分析失败", Rating: "neutral", Error: err.Error()}, nil
	}

	content := ""
	if result != nil {
		content = result.Content
	}

	return &AgentReport{
		Role:    "sentiment",
		Content: content,
		Summary: truncateSummary(content, 100),
		Rating:  extractRating(content),
	}, nil
}
