package multi

import (
	"context"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"time"

	"github.com/cloudwego/eino/schema"
)

func formatF10Records(resp *data.F10GenericResp, max int) string {
	if resp == nil || resp.Result == nil || len(resp.Result.Data) == 0 {
		return "暂无数据\n"
	}
	var s string
	for i, row := range resp.Result.Data {
		if i >= max {
			break
		}
		s += fmt.Sprintf("- 记录 %d:\n", i+1)
		for k, v := range row {
			if v == nil || v == "" {
				continue
			}
			s += fmt.Sprintf("  %s: %v\n", k, v)
		}
	}
	return s
}

func buildLockupData(ctx context.Context, ac *AgentContext) string {
	stockApi := data.NewStockDataApi()

	dataStr := fmt.Sprintf("股票: %s(%s)\n市场: %s\n分析时间: %s\n",
		ac.StockName, ac.StockCode, ac.Market, time.Now().Format("2006-01-02 15:04:05"))

	restricted, _ := stockApi.GetStockRestrictedShares(ac.StockCode)
	dataStr += "\n限售股解禁数据:\n"
	dataStr += formatF10Records(restricted, 10)

	reduction, _ := stockApi.GetStockHolderReduction(ac.StockCode)
	dataStr += "\n大股东减持计划数据:\n"
	dataStr += formatF10Records(reduction, 10)

	pledge, _ := stockApi.GetStockPledge(ac.StockCode)
	dataStr += "\n股权质押数据:\n"
	dataStr += formatF10Records(pledge, 10)

	holderNum := stockApi.GetStockHolderNum(ac.StockCode)
	if holderNum != nil && holderNum.Result.Data != nil && len(holderNum.Result.Data) > 0 {
		dataStr += "\n股东户数变化(最新):\n"
		first := holderNum.Result.Data[0]
		dataStr += fmt.Sprintf("- 日期: %s, 股东总户数: %v, 较上期变化: %v%%, 户均持股: %v\n",
			first.ENDDATE, first.HOLDERTOTALNUM, first.TOTALNUMRATIO, first.AVGFREESHARES)
	}

	return dataStr
}

// RunLockupAnalyst monitors unlocking events and shareholder reduction plans.
func RunLockupAnalyst(ctx context.Context, ac *AgentContext) (*AgentReport, error) {
	dataStr := buildLockupData(ctx, ac)

	chatModel, err := GetChatModelWithTier(ctx, "lockup", LLMTierQuick, ac.AIConfigID)
	if err != nil {
		return &AgentReport{Role: "lockup", Content: "", Summary: "模型加载失败", Rating: "neutral", Error: err.Error()}, nil
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: GetRolePrompt("multi_lockup", LockupAnalystPrompt)},
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
			emitToken(ac, "lockup", chunk.Content)
		}
	}

	return &AgentReport{
		Role:    "lockup",
		Content: content,
		Summary: truncateSummary(content, 100),
		Rating:  extractRating(content),
	}, nil
}
