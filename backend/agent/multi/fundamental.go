package multi

import (
	"context"
	"errors"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"io"

	"github.com/cloudwego/eino/schema"
)

// RunFundamentalAnalyst evaluates company financial health, valuation, and growth.
func RunFundamentalAnalyst(ctx context.Context, ac *AgentContext) (*AgentReport, error) {
	stockApi := data.NewStockDataApi()
	stockApi.GetStockBaseInfo()

	reports := data.GetFinancialReports(ac.StockCode, 30)

	dataStr := fmt.Sprintf("股票: %s(%s)\n", ac.StockName, ac.StockCode)
	if reports != nil && len(*reports) > 0 {
		for i, r := range *reports {
			if i < 5 {
				dataStr += r + "\n"
			}
		}
	} else {
		dataStr += "暂无详细财务数据\n"
	}

	chatModel, err := GetChatModelWithTier(ctx, "fundamental", LLMTierQuick, ac.AIConfigID)
	if err != nil {
		return &AgentReport{Role: "fundamental", Content: "", Summary: "模型加载失败", Rating: "neutral", Error: err.Error()}, nil
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: GetRolePrompt("multi_fundamental", FundamentalAnalystPrompt) + memoryInjection(ctx, ac, "fundamental")},
		{Role: schema.User, Content: fmt.Sprintf("请分析股票 %s(%s) 的基本面\n\n数据:\n%s", ac.StockName, ac.StockCode, dataStr)},
	}

	streamResult, err := chatModel.Stream(ctx, messages)
	if err != nil {
		logger.SugaredLogger.Errorf("fundamental analyst LLM error: %v", err)
		return &AgentReport{Role: "fundamental", Content: "", Summary: "分析失败", Rating: "neutral", Error: err.Error()}, nil
	}

	var content string
	for {
		chunk, err := streamResult.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			logger.SugaredLogger.Errorf("fundamental analyst stream error: %v", err)
			break
		}
		if chunk != nil {
			content += chunk.Content
			emitToken(ac, "fundamental", chunk.Content)
		}
	}

	return &AgentReport{
		Role:    "fundamental",
		Content: content,
		Summary: truncateSummary(content, 100),
		Rating:  extractRating(content),
	}, nil
}
