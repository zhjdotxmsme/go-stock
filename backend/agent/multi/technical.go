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

// RunTechnicalAnalyst analyzes K-line data and technical indicators.
func RunTechnicalAnalyst(ctx context.Context, ac *AgentContext) (*AgentReport, error) {
	stockApi := data.NewStockDataApi()
	klineData := stockApi.GetKLineData(ac.StockCode, "101", int64(60))

	dataStr := fmt.Sprintf("股票: %s(%s)\n", ac.StockName, ac.StockCode)
	if klineData != nil {
		for i, k := range *klineData {
			if i < 30 {
				dataStr += fmt.Sprintf("日期:%s 开:%s 高:%s 低:%s 收:%s 量:%s\n",
					k.Day, k.Open, k.High, k.Low, k.Close, k.Volume)
			}
		}
	} else {
		dataStr += "暂无K线数据\n"
	}

	chatModel, err := GetChatModel(ctx, "technical", ac.AIConfigID)
	if err != nil {
		return &AgentReport{Role: "technical", Content: "", Summary: "模型加载失败", Rating: "neutral", Error: err.Error()}, nil
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: TechnicalAnalystPrompt},
		{Role: schema.User, Content: fmt.Sprintf("请分析股票 %s(%s) 的技术面\n\nK线数据(最近60个交易日):\n%s", ac.StockName, ac.StockCode, dataStr)},
	}

	streamResult, err := chatModel.Stream(ctx, messages)
	if err != nil {
		logger.SugaredLogger.Errorf("technical analyst LLM error: %v", err)
		return &AgentReport{Role: "technical", Content: "", Summary: "分析失败", Rating: "neutral", Error: err.Error()}, nil
	}

	var content string
	for {
		chunk, err := streamResult.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			logger.SugaredLogger.Errorf("technical analyst stream error: %v", err)
			break
		}
		if chunk != nil {
			content += chunk.Content
			emitToken(ac, "technical", chunk.Content)
		}
	}

	return &AgentReport{
		Role:    "technical",
		Content: content,
		Summary: truncateSummary(content, 100),
		Rating:  extractRating(content),
	}, nil
}
