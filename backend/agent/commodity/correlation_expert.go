package commodity

import (
	"context"
	"errors"
	"fmt"
	"go-stock/backend/agent/multi"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"io"
	"strconv"
	"strings"

	"github.com/cloudwego/eino/schema"
)

func init() {
	RegisterExpert(&CorrelationExpert{})
}

type CorrelationExpert struct{}

func (e *CorrelationExpert) Role() string { return "correlation" }

var correlationCodes = []struct{ Code, Name string }{
	{"XAUUSD", "现货黄金"},
	{"XAGUSD", "现货白银"},
	{"USCL", "WTI原油"},
}

func (e *CorrelationExpert) Run(ctx context.Context, cc *CommodityContext) (*ExpertReport, error) {
	commodityApi := data.NewCommodityApi()

	var dataStr strings.Builder
	dataStr.WriteString(fmt.Sprintf("品种: %s(%s)\n\n## 跨品种价格数据\n", cc.Name, cc.Code))

	for _, c := range correlationCodes {
		quote, err := commodityApi.GetQuote(c.Code)
		if err != nil {
			dataStr.WriteString(fmt.Sprintf("%s: 数据获取失败\n", c.Name))
			continue
		}
		dataStr.WriteString(fmt.Sprintf("%s(%s): 最新价 %.2f  涨跌 %+.2f (%.2f%%)\n",
			c.Name, c.Code, quote.Price, quote.Change, quote.ChangePct))
	}

	// Calculate gold-silver ratio if both XAUUSD and XAGUSD available
	dataStr.WriteString("\n## 比价分析\n")
	auQuote, auErr := commodityApi.GetQuote("XAUUSD")
	agQuote, agErr := commodityApi.GetQuote("XAGUSD")
	if auErr == nil && agErr == nil && agQuote.Price > 0 {
		ratio := auQuote.Price / agQuote.Price
		ratioSignal := ""
		switch {
		case ratio > 90:
			ratioSignal = "金银比处于极高水平（>90），白银相对黄金被严重低估，或黄金避险需求极端化"
		case ratio > 80:
			ratioSignal = "金银比处于偏高水平（80-90），白银相对黄金被低估"
		case ratio < 65:
			ratioSignal = "金银比处于偏低水平（<65），工业需求强劲或黄金避险需求减弱"
		default:
			ratioSignal = "金银比处于正常区间（65-80）"
		}
		dataStr.WriteString(fmt.Sprintf("金银比(XAUUSD/XAGUSD): %.2f — %s\n", ratio, ratioSignal))
	}

	clQuote, clErr := commodityApi.GetQuote("USCL")
	if auErr == nil && clErr == nil && clQuote.Price > 0 {
		oilRatio := auQuote.Price / clQuote.Price
		dataStr.WriteString(fmt.Sprintf("金油比(XAUUSD/USCL): %.2f\n", oilRatio))
	}

	// Get DXY for correlation context via WallStreetCN direct
	wsApi := data.WallstreetcnApi{}
	dxyResult := wsApi.GetMarketReal([]string{"DXY.OTC"}, nil)
	if dxyResult != nil && dxyResult.Code == 20000 {
		if values, ok := dxyResult.Data.Snapshot["DXY.OTC"]; ok && len(values) >= 4 {
			dxyPx, _ := parseFloatFromAny(values[1])
			dxyChg, _ := parseFloatFromAny(values[2])
			dxyChgPct, _ := parseFloatFromAny(values[3])
			dataStr.WriteString(fmt.Sprintf("\n美元指数(DXY): %.2f  涨跌 %+.2f (%.2f%%)\n",
				dxyPx, dxyChg, dxyChgPct))
		}
	}

	chatModel, err := multi.GetChatModelWithTier(ctx, "correlation", multi.LLMTierQuick, cc.AIConfigID)
	if err != nil {
		return &ExpertReport{Role: "correlation", Content: "", Summary: "模型加载失败", Rating: "neutral", Error: err.Error()}, nil
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: GetRolePrompt("commodity_correlation", CorrelationExpertPrompt)},
		{Role: schema.User, Content: fmt.Sprintf("请分析商品 %s(%s) 的跨品种关联\n\n数据:\n%s\n\n用户问题: %s", cc.Name, cc.Code, dataStr.String(), cc.UserQuery)},
	}

	streamResult, err := chatModel.Stream(ctx, messages)
	if err != nil {
		logger.SugaredLogger.Errorf("correlation expert LLM error: %v", err)
		return &ExpertReport{Role: "correlation", Content: "", Summary: "分析失败", Rating: "neutral", Error: err.Error()}, nil
	}

	var content string
	for {
		chunk, err := streamResult.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			logger.SugaredLogger.Errorf("correlation expert stream error: %v", err)
			break
		}
		if chunk != nil {
			content += chunk.Content
			emitToken(cc, "correlation", chunk.Content)
		}
	}

	return &ExpertReport{
		Role:    "correlation",
		Content: content,
		Summary: truncateSummary(content, 100),
		Rating:  extractRating(content),
	}, nil
}

func parseFloatFromAny(v interface{}) (float64, error) {
	return strconv.ParseFloat(fmt.Sprintf("%v", v), 64)
}
