package multi

import (
	"context"
	"fmt"
	"go-stock/backend/logger"

	"github.com/cloudwego/eino/schema"
)

// RunHotMoneyAnalyst tracks capital flow and dragon tiger board data.
func RunHotMoneyAnalyst(ctx context.Context, ac *AgentContext) (*AgentReport, error) {
	dataStr := fmt.Sprintf("股票: %s(%s)\n市场: %s\n资金面分析基于龙虎榜和资金流向数据。",
		ac.StockName, ac.StockCode, ac.Market)

	chatModel, err := GetChatModel(ctx, "hotmoney", ac.AIConfigID)
	if err != nil {
		return &AgentReport{Role: "hotmoney", Content: "", Summary: "模型加载失败", Rating: "neutral", Error: err.Error()}, nil
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: HotMoneyAnalystPrompt},
		{Role: schema.User, Content: fmt.Sprintf("请分析股票 %s(%s) 的资金面\n\n数据:\n%s", ac.StockName, ac.StockCode, dataStr)},
	}

	result, err := chatModel.Stream(ctx, messages)
	if err != nil {
		logger.SugaredLogger.Errorf("hotmoney analyst LLM error: %v", err)
		return &AgentReport{Role: "hotmoney", Content: "", Summary: "分析失败", Rating: "neutral", Error: err.Error()}, nil
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
		Role:    "hotmoney",
		Content: content,
		Summary: truncateSummary(content, 100),
		Rating:  extractRating(content),
	}, nil
}
