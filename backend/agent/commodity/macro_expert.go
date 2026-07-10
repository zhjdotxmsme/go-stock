package commodity

import (
	"context"
	"errors"
	"fmt"
	"go-stock/backend/agent/multi"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"io"

	"github.com/cloudwego/eino/schema"
)

func init() {
	RegisterExpert(&MacroExpert{})
}

type MacroExpert struct{}

func (e *MacroExpert) Role() string { return "macro" }

func (e *MacroExpert) Run(ctx context.Context, cc *CommodityContext) (*ExpertReport, error) {
	commodityApi := data.NewCommodityApi()
	fredApi := data.NewFredApi()

	macro, err := commodityApi.GetMacroIndicators()
	dataStr := fmt.Sprintf("品种: %s(%s)\n\n## 宏观指标\n", cc.Name, cc.Code)
	if err != nil {
		dataStr += fmt.Sprintf("宏观指标获取失败: %v\n\n将基于有限信息进行分析。\n", err)
	} else {
		dataStr += fmt.Sprintf("美元指数(DXY): %.2f\n", macro.DXY)
		dataStr += fmt.Sprintf("美国2年期国债收益率: %.4f%%\n", macro.US2YR)
		dataStr += fmt.Sprintf("美国10年期国债收益率: %.4f%%\n", macro.US10YR)
		dataStr += fmt.Sprintf("美国30年期国债收益率: %.4f%%\n", macro.US30YR)
		dataStr += fmt.Sprintf("收益率曲线形态: %s\n", macro.YieldCurve)
		if macro.US10YR > 0 && macro.US2YR > 0 {
			spread := macro.US10YR - macro.US2YR
			dataStr += fmt.Sprintf("2s10s利差: %.4f%%\n", spread)
		}
	}

	// Fetch TIPS from FRED
	tipsRate, tipsErr := fredApi.GetTIPSRate()
	if tipsErr == nil {
		realRate := data.CalculateRealRate(macro.US10YR, tipsRate)
		dataStr += fmt.Sprintf("\n## 实际利率（TIPS）\n")
		dataStr += fmt.Sprintf("10年TIPS收益率: %.4f%%\n", tipsRate)
		dataStr += fmt.Sprintf("10年实际利率: %.4f%%\n", realRate)
		
		// TIPS interpretation
		tipsSignal := ""
		switch {
		case realRate > 2:
			tipsSignal = "实际利率处于高位（>2%），持有黄金的机会成本较高，对黄金形成压力"
		case realRate < 0:
			tipsSignal = "实际利率为负，持有黄金相对现金有优势，对黄金形成支撑"
		default:
			tipsSignal = "实际利率处于正常区间"
		}
		dataStr += fmt.Sprintf("实际利率解读: %s\n", tipsSignal)
	} else {
		dataStr += fmt.Sprintf("\n## 实际利率（TIPS）\n")
		dataStr += fmt.Sprintf("TIPS数据获取失败: %v\n\n注：实际利率是影响黄金价格的关键宏观指标\n", tipsErr)
	}

	// Fetch break-even inflation
	beInflation, beErr := fredApi.GetBreakEvenInflation()
	if beErr == nil {
		dataStr += fmt.Sprintf("\n## 通胀预期\n")
		dataStr += fmt.Sprintf("5年盈亏平衡通胀: %.4f%%\n", beInflation)
		beSignal := ""
		switch {
		case beInflation > 2.5:
			beSignal = "通胀预期较高（>2.5%），对黄金白银形成支撑"
		case beInflation < 2:
			beSignal = "通胀预期较低（<2%），可能抑制贵金属需求"
		default:
			beSignal = "通胀预期处于正常区间"
		}
		dataStr += fmt.Sprintf("通胀预期解读: %s\n", beSignal)
	}

	chatModel, err := multi.GetChatModelWithTier(ctx, "macro", multi.LLMTierQuick, cc.AIConfigID)
	if err != nil {
		return &ExpertReport{Role: "macro", Content: "", Summary: "模型加载失败", Rating: "neutral", Error: err.Error()}, nil
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: GetRolePrompt("commodity_macro", MacroExpertPrompt)},
		{Role: schema.User, Content: fmt.Sprintf("请分析商品 %s(%s) 的宏观环境\n\n数据:\n%s\n\n用户问题: %s", cc.Name, cc.Code, dataStr, cc.UserQuery)},
	}

	streamResult, err := chatModel.Stream(ctx, messages)
	if err != nil {
		logger.SugaredLogger.Errorf("macro expert LLM error: %v", err)
		return &ExpertReport{Role: "macro", Content: "", Summary: "分析失败", Rating: "neutral", Error: err.Error()}, nil
	}

	var content string
	for {
		chunk, err := streamResult.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			logger.SugaredLogger.Errorf("macro expert stream error: %v", err)
			break
		}
		if chunk != nil {
			content += chunk.Content
			emitToken(cc, "macro", chunk.Content)
		}
	}

	return &ExpertReport{
		Role:    "macro",
		Content: content,
		Summary: truncateSummary(content, 100),
		Rating:  extractRating(content),
	}, nil
}
