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
