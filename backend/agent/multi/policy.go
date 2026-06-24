package multi

import (
	"context"
	"fmt"
	"go-stock/backend/logger"

	"github.com/cloudwego/eino/schema"
)

// RunPolicyAnalyst analyzes policy impacts on the stock.
func RunPolicyAnalyst(ctx context.Context, ac *AgentContext) (*AgentReport, error) {
	dataStr := fmt.Sprintf("股票: %s(%s)\n市场: %s\n政策面分析基于新闻和公告数据。",
		ac.StockName, ac.StockCode, ac.Market)

	chatModel, err := GetChatModel(ctx, "policy", ac.AIConfigID)
	if err != nil {
		return &AgentReport{Role: "policy", Content: "", Summary: "模型加载失败", Rating: "neutral", Error: err.Error()}, nil
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: PolicyAnalystPrompt},
		{Role: schema.User, Content: fmt.Sprintf("请分析股票 %s(%s) 的政策面\n\n数据:\n%s", ac.StockName, ac.StockCode, dataStr)},
	}

	result, err := chatModel.Stream(ctx, messages)
	if err != nil {
		logger.SugaredLogger.Errorf("policy analyst LLM error: %v", err)
		return &AgentReport{Role: "policy", Content: "", Summary: "分析失败", Rating: "neutral", Error: err.Error()}, nil
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
		Role:    "policy",
		Content: content,
		Summary: truncateSummary(content, 100),
		Rating:  extractRating(content),
	}, nil
}
