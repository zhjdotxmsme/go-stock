package multi

import (
	"context"
	"errors"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"io"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// RunTechnicalAnalyst analyzes K-line data and technical indicators.
// ETF/LOF 品种分流：追加 ETF 专属数据段（溢价率/净值/份额趋势）并切换到 ETF 分析框架。
func RunTechnicalAnalyst(ctx context.Context, ac *AgentContext) (*AgentReport, error) {
	isEtf := ac.InstrumentKind == data.InstrumentKindETF
	instrumentLabel := "股票"
	if isEtf {
		instrumentLabel = "ETF"
	}

	klineData := func() *[]data.KLineData {
		if ac.DataPack != nil {
			return ac.DataPack.KLineDaily
		}
		return data.NewStockDataApi().GetKLineData(ac.StockCode, "101", int64(60))
	}()

	dataStr := fmt.Sprintf("%s: %s(%s)\n", instrumentLabel, ac.StockName, ac.StockCode)
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

	// Technical indicators from stock-sdk MCP (shared via DataPack when set)
	var indicators *data.IndicatorResult
	if ac.DataPack != nil {
		indicators = ac.DataPack.TechnicalIndicators
	} else {
		indicators, _ = data.GetTechnicalIndicators(ctx, ac.StockCode, "101", 60)
	}
	if indicators != nil {
		dataStr += "\n技术指标:\n"
		if len(indicators.MA) > 0 {
			dataStr += fmt.Sprintf("MA: %v\n", indicators.MA)
		}
		if len(indicators.MACD) > 0 {
			dataStr += fmt.Sprintf("MACD: %v\n", indicators.MACD)
		}
		if len(indicators.RSI) > 0 {
			dataStr += fmt.Sprintf("RSI: %v\n", indicators.RSI)
		}
		if len(indicators.KDJ) > 0 {
			dataStr += fmt.Sprintf("KDJ: %v\n", indicators.KDJ)
		}
	}

	if isEtf && ac.DataPack != nil && ac.DataPack.EtfQuote != nil {
		dataStr += formatEtfQuoteBlock(ac.DataPack.EtfQuote)
	}

	chatModel, err := GetChatModelWithTier(ctx, "technical", LLMTierQuick, ac.AIConfigID)
	if err != nil {
		return &AgentReport{Role: "technical", Content: "", Summary: "模型加载失败", Rating: "neutral", Error: err.Error()}, nil
	}

	roleKey, framework := "multi_technical", TechnicalAnalystPrompt
	if isEtf {
		roleKey, framework = "multi_technical_etf", ETFTechnicalAnalystPrompt
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: GetRolePrompt(roleKey, framework) + instrumentContextBlock(ac.StockCode) + memoryInjection(ctx, ac, "technical")},
		{Role: schema.User, Content: fmt.Sprintf("请分析%s %s(%s) 的技术面\n\nK线数据(最近60个交易日):\n%s", instrumentLabel, ac.StockName, ac.StockCode, dataStr)},
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

// formatEtfQuoteBlock 将 ETF 专属快照（溢价率/净值/份额趋势）序列化为 Prompt 数据段。
func formatEtfQuoteBlock(q *data.EtfQuoteSnapshot) string {
	var b strings.Builder
	b.WriteString("\nETF专属数据:\n")
	if q.Price > 0 {
		b.WriteString(fmt.Sprintf("最新市价: %.3f\n", q.Price))
	}
	if q.Nav > 0 {
		b.WriteString(fmt.Sprintf("单位净值: %.4f (%s)\n", q.Nav, q.NavDate))
	}
	if q.PremiumRate != nil {
		b.WriteString(fmt.Sprintf("溢价率: %+.2f%%（正=溢价，负=折价）\n", *q.PremiumRate))
	}
	if q.Shares != nil {
		b.WriteString(fmt.Sprintf("总份额: %.2f亿份\n", *q.Shares/1e8))
	}
	if q.TurnoverRate > 0 {
		b.WriteString(fmt.Sprintf("换手率: %.2f%%\n", q.TurnoverRate))
	}
	if len(q.SharesTrend) > 1 {
		b.WriteString("近期份额快照(日期 份额亿份 溢价率%):\n")
		for _, row := range q.SharesTrend {
			prem := "-"
			if row.PremiumRate != nil {
				prem = fmt.Sprintf("%+.2f", *row.PremiumRate)
			}
			b.WriteString(fmt.Sprintf("%s %.2f %s\n", row.Date, row.Shares/1e8, prem))
		}
	}
	return b.String()
}
