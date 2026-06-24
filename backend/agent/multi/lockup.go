package multi

import (
	"context"
	"fmt"
	"go-stock/backend/logger"

	"github.com/cloudwego/eino/schema"
)

// RunLockupAnalyst monitors unlocking events and shareholder reduction plans.
func RunLockupAnalyst(ctx context.Context, ac *AgentContext) (*AgentReport, error) {
	dataStr := fmt.Sprintf("股票: %s(%s)\n市场: %s\n解禁分析基于限售股解禁和股东减持数据。",
		ac.StockName, ac.StockCode, ac.Market)

	chatModel, err := GetChatModel(ctx, "lockup", ac.AIConfigID)
	if err != nil {
		return &AgentReport{Role: "lockup", Content: "", Summary: "模型加载失败", Rating: "neutral", Error: err.Error()}, nil
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: LockupAnalystPrompt},
		{Role: schema.User, Content: fmt.Sprintf("请分析股票 %s(%s) 的解禁压力\n\n数据:\n%s", ac.StockName, ac.StockCode, dataStr)},
	}

	result, err := chatModel.Stream(ctx, messages)
	if err != nil {
		logger.SugaredLogger.Errorf("lockup analyst LLM error: %v", err)
		return &AgentReport{Role: "lockup", Content: "", Summary: "分析失败", Rating: "neutral", Error: err.Error()}, nil
	}

	var content string
	for {
		chunk, err := result.Recv()
		if err != nil {
			break
		}
		if chunk != nil {
			content += chunk.Content
		}
	}

	return &AgentReport{
		Role:    "lockup",
		Content: content,
		Summary: truncateSummary(content, 100),
		Rating:  extractRating(content),
	}, nil
}
