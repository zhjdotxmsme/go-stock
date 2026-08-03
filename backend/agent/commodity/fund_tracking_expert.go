package commodity

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"go-stock/backend/agent/multi"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"

	"github.com/cloudwego/eino/schema"
)

func init() {
	RegisterCategoryExpert(&FundTrackingExpert{})
}

type FundTrackingExpert struct{}

func (e *FundTrackingExpert) Role() string { return "fund_tracking" }

func (e *FundTrackingExpert) Categories() []models.CommodityCategory {
	return []models.CommodityCategory{models.CategoryFund}
}

func (e *FundTrackingExpert) Run(ctx context.Context, cc *CommodityContext) (*ExpertReport, error) {
	commodityApi := data.NewCommodityApi()
	wsApi := data.WallstreetcnApi{}

	var dataStr strings.Builder
	dataStr.WriteString(fmt.Sprintf("品种: %s(%s)\n\n", cc.Name, cc.Code))

	// 1. ETF quote
	etfQuote, etfErr := commodityApi.GetQuote(cc.Code)
	if etfErr == nil {
		dataStr.WriteString(fmt.Sprintf("## ETF实时数据\n"))
		dataStr.WriteString(fmt.Sprintf("ETF价格: %.4f\n", etfQuote.Price))
		dataStr.WriteString(fmt.Sprintf("涨跌: %+.4f (%.2f%%)\n", etfQuote.Change, etfQuote.ChangePct))
		if etfQuote.High > 0 {
			dataStr.WriteString(fmt.Sprintf("最高: %.4f  最低: %.4f\n", etfQuote.High, etfQuote.Low))
		}
	} else {
		dataStr.WriteString(fmt.Sprintf("ETF价格获取失败: %v\n", etfErr))
	}

	// 2. International reference price (e.g., 518880 -> GC=F)
	dataStr.WriteString("\n## 与国际参考价对比\n")
	asset := data.FindCommodityByCode(cc.Code)
	if asset != nil && asset.InternationalRef != "" {
		intlQuote, intlErr := commodityApi.GetQuoteIntl(cc.Code)
		if intlErr == nil {
			dataStr.WriteString(fmt.Sprintf("国际参考(%s): $%.2f\n", asset.InternationalRef, intlQuote.Price))
			dataStr.WriteString(fmt.Sprintf("涨跌: %+.2f (%.2f%%)\n", intlQuote.Change, intlQuote.ChangePct))
			// Calculate tracking difference
			if etfErr == nil && intlQuote.Price > 0 && etfQuote.Price > 0 {
				// Simple tracking comparison (ETF price vs international price)
				// Note: unit difference (ETF in CNY/gram vs international in USD/oz) makes direct comparison complex
				dataStr.WriteString(fmt.Sprintf("注意: ETF单位(元/份)与国际期货单位($/oz)不同，需要通过汇率和单位换算后对比跟踪误差。\n"))
			}
		} else {
			dataStr.WriteString(fmt.Sprintf("国际参考价获取失败: %v\n", intlErr))
		}
	} else {
		dataStr.WriteString("无国际参考价\n")
	}

	// 3. ETF K-line (60 days for trend analysis)
	dataStr.WriteString("\n## ETF近期K线(近10日)\n")
	klines, klineErr := commodityApi.GetKLine(cc.Code, "day", 60)
	if klineErr == nil && len(klines) > 0 {
		dataStr.WriteString("日期       收盘    涨跌%%\n")
		showCount := len(klines)
		if showCount > 10 {
			showCount = 10
		}
		for i := len(klines) - showCount; i < len(klines); i++ {
			k := klines[i]
			changePct := 0.0
			if k.PrevClose > 0 {
				changePct = (k.Close - k.PrevClose) / k.PrevClose * 100
			}
			dataStr.WriteString(fmt.Sprintf("%s  %.4f  %+.2f%%\n",
				k.Time.Format("01-02"), k.Close, changePct))
		}

		// Volume analysis
		totalVol := 0.0
		avgVol := 0.0
		for _, k := range klines {
			totalVol += float64(k.Volume)
		}
		if len(klines) > 0 {
			avgVol = totalVol / float64(len(klines))
		}
		if len(klines) > 0 && klines[len(klines)-1].Volume > 0 {
			lastVol := float64(klines[len(klines)-1].Volume)
			volRatio := lastVol / avgVol
			dataStr.WriteString(fmt.Sprintf("\n成交量分析: 最新成交量=%.0f, 20日均量=%.0f, 量比=%.2f\n",
				lastVol, avgVol, volRatio))
			if volRatio > 2 {
				dataStr.WriteString("成交量显著放大（量比>2），资金进出活跃\n")
			} else if volRatio < 0.5 {
				dataStr.WriteString("成交量显著萎缩（量比<0.5），市场关注度低\n")
			} else {
				dataStr.WriteString("成交量正常\n")
			}
		}
	} else {
		dataStr.WriteString(fmt.Sprintf("K线数据获取失败: %v\n", klineErr))
	}

	// 4. News for underlying asset
	dataStr.WriteString("\n## 底层资产相关新闻\n")
	channel := "goldc-channel" // Both gold ETF (518880) and silver LOF (161226) are precious metals
	newsStr := wsApi.GetLivesReadable(channel, 20)
	if strings.TrimSpace(newsStr) == "" || newsStr == "暂无快讯数据" {
		newsStr = wsApi.GetLivesReadable("commodity-channel", 20)
	}
	dataStr.WriteString(newsStr)

	// Call LLM
	chatModel, err := multi.GetChatModelWithTier(ctx, "fund_tracking", multi.LLMTierQuick, cc.AIConfigID)
	if err != nil {
		return &ExpertReport{Role: "fund_tracking", Content: "", Summary: "模型加载失败", Rating: "neutral", Error: err.Error()}, nil
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: GetRolePrompt("commodity_fund_tracking", FundTrackingExpertPrompt)},
		{Role: schema.User, Content: fmt.Sprintf("请分析 %s(%s) 的ETF跟踪情况\n\n数据:\n%s\n\n用户问题: %s", cc.Name, cc.Code, dataStr.String(), cc.UserQuery)},
	}

	streamResult, err := chatModel.Stream(ctx, messages)
	if err != nil {
		logger.SugaredLogger.Errorf("fund_tracking expert LLM error: %v", err)
		return &ExpertReport{Role: "fund_tracking", Content: "", Summary: "分析失败", Rating: "neutral", Error: err.Error()}, nil
	}

	var content string
	for {
		chunk, err := streamResult.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			logger.SugaredLogger.Errorf("fund_tracking expert stream error: %v", err)
			break
		}
		if chunk != nil {
			content += chunk.Content
			emitToken(cc, "fund_tracking", chunk.Content)
		}
	}

	return &ExpertReport{
		Role:    "fund_tracking",
		Content: content,
		Summary: truncateSummary(content, 100),
		Rating:  extractRating(content),
	}, nil
}
