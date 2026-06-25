package multi

import (
	"context"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"time"

	"github.com/cloudwego/eino/schema"
)

func buildPolicyData(ctx context.Context, ac *AgentContext) string {
	newsApi := data.NewMarketNewsApi()

	dataStr := fmt.Sprintf("股票: %s(%s)\n市场: %s\n分析时间: %s\n",
		ac.StockName, ac.StockCode, ac.Market, time.Now().Format("2006-01-02 15:04:05"))

	notices := newsApi.StockNotice(ac.StockCode)
	if len(notices) > 0 {
		dataStr += "\n近期公告:\n"
		for i, n := range notices {
			if i >= 10 {
				break
			}
			if m, ok := n.(map[string]any); ok {
				title, _ := m["title"].(string)
				noticeDate, _ := m["notice_date"].(string)
				columnName, _ := m["column_name"].(string)
				dataStr += fmt.Sprintf("- [%s] %s (%s)\n", noticeDate, title, columnName)
			}
		}
	} else {
		dataStr += "\n近期公告: 暂无\n"
	}

	telegraphs := newsApi.GetNewsList("", 15)
	if telegraphs != nil && len(*telegraphs) > 0 {
		dataStr += "\n近期市场资讯:\n"
		for i, t := range *telegraphs {
			if i >= 10 {
				break
			}
			if t == nil {
				continue
			}
			dataStr += fmt.Sprintf("- [%s] %s\n", t.Source, t.Title)
			if t.Content != "" {
				dataStr += fmt.Sprintf("  %s\n", t.Content)
			}
		}
	} else {
		dataStr += "\n近期市场资讯: 暂无\n"
	}

	return dataStr
}

// RunPolicyAnalyst analyzes policy impacts on the stock.
func RunPolicyAnalyst(ctx context.Context, ac *AgentContext) (*AgentReport, error) {
	dataStr := buildPolicyData(ctx, ac)

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
			emitToken(ac, "policy", chunk.Content)
		}
	}

	return &AgentReport{
		Role:    "policy",
		Content: content,
		Summary: truncateSummary(content, 100),
		Rating:  extractRating(content),
	}, nil
}
