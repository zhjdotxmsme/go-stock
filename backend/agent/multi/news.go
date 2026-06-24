package multi

import (
	"context"
	"errors"
	"fmt"
	"go-stock/backend/logger"
	"io"
	"time"

	"github.com/cloudwego/eino/schema"
)

// RunNewsAnalyst monitors macro news, industry dynamics, and company announcements.
func RunNewsAnalyst(ctx context.Context, ac *AgentContext) (*AgentReport, error) {
	dataStr := fmt.Sprintf("股票: %s(%s)\n分析时间: %s\n暂无新闻数据",
		ac.StockName, ac.StockCode, time.Now().Format("2006-01-02 15:04:05"))

	chatModel, err := GetChatModel(ctx, "news", ac.AIConfigID)
	if err != nil {
		return &AgentReport{Role: "news", Content: "", Summary: "模型加载失败", Rating: "neutral", Error: err.Error()}, nil
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: NewsAnalystPrompt},
		{Role: schema.User, Content: fmt.Sprintf("请分析股票 %s(%s) 的相关新闻和事件影响\n\n数据:\n%s", ac.StockName, ac.StockCode, dataStr)},
	}

	streamResult, err := chatModel.Stream(ctx, messages)
	if err != nil {
		logger.SugaredLogger.Errorf("news analyst LLM error: %v", err)
		return &AgentReport{Role: "news", Content: "", Summary: "分析失败", Rating: "neutral", Error: err.Error()}, nil
	}

	var content string
	for {
		chunk, err := streamResult.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			logger.SugaredLogger.Errorf("news analyst stream error: %v", err)
			break
		}
		if chunk != nil {
			content += chunk.Content
			emitToken(ac, "news", chunk.Content)
		}
	}

	return &AgentReport{
		Role:    "news",
		Content: content,
		Summary: truncateSummary(content, 100),
		Rating:  extractRating(content),
	}, nil
}
