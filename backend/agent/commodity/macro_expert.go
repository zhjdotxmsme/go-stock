package commodity

import (
	"context"
	"errors"
	"fmt"
	"io"

	"go-stock/backend/agent/multi"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"

	"github.com/cloudwego/eino/schema"
)

func init() {
	RegisterExpert(&MacroExpert{})
}

type MacroExpert struct{}

func (e *MacroExpert) Role() string { return "macro" }

func (e *MacroExpert) Run(ctx context.Context, cc *CommodityContext) (*ExpertReport, error) {
	commodityApi := data.NewCommodityApi()

	var dataStr strings.Builder
	dataStr.WriteString(fmt.Sprintf("品种: %s(%s)\n\n## 宏观指标\n", cc.Name, cc.Code))

	// Use enhanced macro indicators (TIPS multi-tenor, break-even, TLT/TIP)
	macro, err := commodityApi.GetMacroIndicatorsEnhanced()
	if err != nil {
		dataStr += fmt.Sprintf("宏观指标获取失败: %v\n\n将基于有限信息进行分析。\n", err)
	} else {
		dataStr.WriteString(fmt.Sprintf("美元指数(DXY): %.2f\n", macro.DXY))
		dataStr.WriteString(fmt.Sprintf("美国国债收益率:\n"))
		dataStr.WriteString(fmt.Sprintf("  2Y: %.2f%%\n", macro.US2YR))
		dataStr.WriteString(fmt.Sprintf("  5Y: %.2f%%\n", macro.US5YR))
		dataStr.WriteString(fmt.Sprintf("  7Y: %.2f%%\n", macro.US7YR))
		dataStr.WriteString(fmt.Sprintf("  10Y: %.2f%%\n", macro.US10YR))
		dataStr.WriteString(fmt.Sprintf("  30Y: %.2f%%\n", macro.US30YR))
		dataStr.WriteString(fmt.Sprintf("收益率曲线形态: %s\n", macro.YieldCurve))
		if macro.US10YR > 0 && macro.US2YR > 0 {
			spread := macro.US10YR - macro.US2YR
			dataStr.WriteString(fmt.Sprintf("2s10s利差: %.2f%%\n", spread))
			if spread < 0 {
				dataStr.WriteString("曲线倒挂 → 衰退预警\n")
			}
		}

		// TIPS multi-tenor
		dataStr.WriteString(fmt.Sprintf("\n## 实际利率（TIPS多期限）\n"))
		dataStr.WriteString(fmt.Sprintf("5年TIPS: %.2f%%\n", macro.TIPS5Y))
		dataStr.WriteString(fmt.Sprintf("10年TIPS: %.2f%%\n", macro.TIPS10Y))
		dataStr.WriteString(fmt.Sprintf("20年TIPS: %.2f%%\n", macro.TIPS20Y))
		dataStr.WriteString(fmt.Sprintf("30年TIPS: %.2f%%\n", macro.TIPS30Y))

		// Real rates
		if macro.US10YR > 0 && macro.TIPS10Y > 0 {
			realRate := data.CalculateRealRate(macro.US10YR, macro.TIPS10Y)
			dataStr.WriteString(fmt.Sprintf("\n10年实际利率: %.2f%%\n", realRate))
			switch {
			case realRate > 2:
				dataStr.WriteString("实际利率高位（>2%），持有商品机会成本较高\n")
			case realRate < 0:
				dataStr.WriteString("实际利率为负，持有无息资产有优势\n")
			default:
				dataStr.WriteString("实际利率处于正常区间\n")
			}
		}

		// Break-even inflation
		dataStr.WriteString(fmt.Sprintf("\n## 盈亏平衡通胀率\n"))
		dataStr.WriteString(fmt.Sprintf("5年: %.2f%%\n", macro.BreakEven5Y))
		dataStr.WriteString(fmt.Sprintf("10年: %.2f%%\n", macro.BreakEven10Y))
		switch {
		case macro.BreakEven10Y > 2.5:
			dataStr.WriteString("通胀预期较高（>2.5%），支撑抗通胀商品\n")
		case macro.BreakEven10Y < 2:
			dataStr.WriteString("通胀预期较低（<2%），抑制商品需求\n")
		default:
			dataStr.WriteString("通胀预期处于正常区间\n")
		}

		// TLT/TIP ETF
		dataStr.WriteString(fmt.Sprintf("\n## 债券ETF价格\n"))
		if macro.TLTPrice > 0 {
			dataStr.WriteString(fmt.Sprintf("TLT(20+年美债ETF): $%.2f\n", macro.TLTPrice))
		}
		if macro.TIPPrice > 0 {
			dataStr.WriteString(fmt.Sprintf("TIP(TIPS ETF): $%.2f\n", macro.TIPPrice))
		}
	}

	// Category-specific analysis hint
	switch cc.Category {
	case models.CategoryPreciousMetal:
		dataStr.WriteString("\n> 分析侧重: 实际利率和美元是贵金属定价的核心驱动\n")
	case models.CategoryEnergy:
		dataStr.WriteString("\n> 分析侧重: 美元强弱和全球需求预期（收益率曲线→衰退信号）对原油影响显著\n")
	case models.CategoryFund:
		dataStr.WriteString("\n> 分析侧重: 宏观环境对底层资产的传导效应\n")
	}

	chatModel, err := multi.GetChatModelWithTier(ctx, "macro", multi.LLMTierQuick, cc.AIConfigID)
	if err != nil {
		return &ExpertReport{Role: "macro", Content: "", Summary: "模型加载失败", Rating: "neutral", Error: err.Error()}, nil
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: GetRolePrompt("commodity_macro", MacroExpertPrompt)},
		{Role: schema.User, Content: fmt.Sprintf("请分析商品 %s(%s) 的宏观环境\n\n数据:\n%s\n\n用户问题: %s", cc.Name, cc.Code, dataStr.String(), cc.UserQuery)},
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
