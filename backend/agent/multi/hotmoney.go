package multi

import (
	"context"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"time"

	"github.com/cloudwego/eino/schema"
)

func buildHotMoneyData(ctx context.Context, ac *AgentContext) string {
	stockApi := data.NewStockDataApi()
	newsApi := data.NewMarketNewsApi()

	dataStr := fmt.Sprintf("股票: %s(%s)\n市场: %s\n分析时间: %s\n",
		ac.StockName, ac.StockCode, ac.Market, time.Now().Format("2006-01-02 15:04:05"))

	date := time.Now().Format("2006-01-02")
	tigerRanks := newsApi.LongTiger(date)
	if tigerRanks != nil && len(*tigerRanks) > 0 {
		dataStr += "\n今日龙虎榜数据:\n"
		found := false
		for i, rank := range *tigerRanks {
			if i >= 20 {
				break
			}
			if rank.SECURITYCODE != ac.StockCode {
				continue
			}
			found = true
			dataStr += fmt.Sprintf("- 日期: %s, 原因: %s, 收盘价: %.2f, 涨跌幅: %.2f%%, 龙虎榜净额: %.2f, 买入额: %.2f, 卖出额: %.2f\n",
				rank.TRADEDATE, rank.EXPLANATION, rank.CLOSEPRICE, rank.CHANGERATE, rank.BILLBOARDNETAMT, rank.BILLBOARDBUYAMT, rank.BILLBOARDSELLAMT)
		}
		if !found {
			dataStr += "- 今日未上榜\n"
		}
	} else {
		dataStr += "\n今日龙虎榜数据: 暂无\n"
	}

	moneyHis := func() []models.StockMoneyDataHis {
		if ac.DataPack != nil {
			return ac.DataPack.HistoryMoneyData
		}
		return stockApi.GetStockHistoryMoneyData(ac.StockCode)
	}()
	if len(moneyHis) > 0 {
		dataStr += "\n近5日资金流向:\n"
		for i := len(moneyHis) - 1; i >= 0 && i >= len(moneyHis)-5; i-- {
			m := moneyHis[i]
			dataStr += fmt.Sprintf("- %s 主力净额: %s, 主力净占比: %s%%, 收盘价: %s, 涨跌幅: %s%%\n",
				m.Date, m.F62, m.F184, m.F2, m.F3)
		}
	} else {
		dataStr += "\n近5日资金流向: 暂无\n"
	}

	return dataStr
}

// RunHotMoneyAnalyst tracks capital flow and dragon tiger board data.
func RunHotMoneyAnalyst(ctx context.Context, ac *AgentContext) (*AgentReport, error) {
	dataStr := buildHotMoneyData(ctx, ac)

	chatModel, err := GetChatModelWithTier(ctx, "hotmoney", LLMTierQuick, ac.AIConfigID)
	if err != nil {
		return &AgentReport{Role: "hotmoney", Content: "", Summary: "模型加载失败", Rating: "neutral", Error: err.Error()}, nil
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: GetRolePrompt("multi_hot_money", HotMoneyAnalystPrompt) + memoryInjection(ctx, ac, "hotmoney")},
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
			emitToken(ac, "hotmoney", chunk.Content)
		}
	}

	return &AgentReport{
		Role:    "hotmoney",
		Content: content,
		Summary: truncateSummary(content, 100),
		Rating:  extractRating(content),
	}, nil
}
