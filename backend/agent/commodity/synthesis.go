package commodity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go-stock/backend/agent/multi"
	"go-stock/backend/logger"
	"io"
	"strings"

	"github.com/cloudwego/eino/schema"
)

func RunSynthesis(ctx context.Context, cc *CommodityContext) (*CommodityReport, error) {
	var contextStr string
	contextStr += fmt.Sprintf("品种: %s(%s)  类别: %s\n\n", cc.Name, cc.Code, cc.Category)
	contextStr += "## 各维度分析报告\n\n"
	for _, r := range cc.Reports {
		if r.Error != "" {
			contextStr += fmt.Sprintf("【%s】数据不可用\n\n", r.Role)
			continue
		}
		contextStr += fmt.Sprintf("### %s (评级: %s)\n%s\n\n", r.Role, r.Rating, r.Summary)
	}

	if cc.Debate != nil {
		contextStr += "## 多空研究员辩论\n\n"
		for _, round := range cc.Debate.Rounds {
			contextStr += fmt.Sprintf("### 第%d轮\n", round.RoundNum)
			contextStr += fmt.Sprintf("**看多方**: %s\n\n", truncateSummary(round.BullArgument, 200))
			contextStr += fmt.Sprintf("**看空方**: %s\n\n", truncateSummary(round.BearArgument, 200))
		}
		if len(cc.Debate.ConsensusItems) > 0 {
			contextStr += "**共识点**:\n"
			for _, item := range cc.Debate.ConsensusItems {
				contextStr += fmt.Sprintf("- %s\n", item)
			}
		}
		if len(cc.Debate.Disagreements) > 0 {
			contextStr += "**分歧点**:\n"
			for _, item := range cc.Debate.Disagreements {
				contextStr += fmt.Sprintf("- %s\n", item)
			}
		}
		contextStr += "\n"
	}

	report := &CommodityReport{
		OverallRating: "hold",
		Strengths:     []string{},
		RiskFactors:   []string{},
		Catalysts:     []string{},
		Conclusion:    "",
	}

	chatModel, err := multi.GetChatModelWithTier(ctx, "synthesis", multi.LLMTierDeep, cc.AIConfigID)
	if err != nil {
		logger.SugaredLogger.Warnf("commodity synthesis LLM unavailable, using basic aggregation: %v", err)
		return basicSynthesis(report, cc)
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: GetRolePrompt("commodity_synthesis", SynthesisPrompt)},
		{Role: schema.User, Content: fmt.Sprintf("请基于以下分析数据生成最终大宗商品投资分析报告:\n\n%s", contextStr)},
	}

	streamResult, err := chatModel.Stream(ctx, messages)
	if err != nil {
		logger.SugaredLogger.Warnf("commodity synthesis LLM error, using basic aggregation: %v", err)
		return basicSynthesis(report, cc)
	}

	var conclusion string
	for {
		chunk, err := streamResult.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			logger.SugaredLogger.Warnf("commodity synthesis stream error: %v", err)
			break
		}
		if chunk != nil {
			conclusion += chunk.Content
			emitToken(cc, "synthesis", chunk.Content)
		}
	}

	if conclusion != "" {
		report.Conclusion = conclusion
		report.OverallRating = aggregateRatings(cc.Reports)
	}

	for _, r := range cc.Reports {
		if r.Error != "" {
			continue
		}
		report.Strengths = append(report.Strengths, fmt.Sprintf("%s: %s", r.Role, r.Summary))
	}

	if cc.Debate != nil {
		for _, item := range cc.Debate.ConsensusItems {
			report.Catalysts = append(report.Catalysts, item)
		}
		for _, item := range cc.Debate.Disagreements {
			report.RiskFactors = append(report.RiskFactors, item)
		}
	}

	extractStructuredFields(ctx, cc, report)

	return report, nil
}

func basicSynthesis(report *CommodityReport, cc *CommodityContext) (*CommodityReport, error) {
	for _, r := range cc.Reports {
		if r.Error != "" {
			continue
		}
		report.Strengths = append(report.Strengths, fmt.Sprintf("%s: %s", r.Role, r.Summary))
		report.Conclusion += fmt.Sprintf("【%s】%s\n", r.Role, r.Summary)
	}
	if cc.Debate != nil {
		for _, item := range cc.Debate.ConsensusItems {
			report.Catalysts = append(report.Catalysts, item)
		}
		for _, item := range cc.Debate.Disagreements {
			report.RiskFactors = append(report.RiskFactors, item)
		}
	}
	report.OverallRating = aggregateRatings(cc.Reports)
	return report, nil
}

func aggregateRatings(reports []ExpertReport) string {
	bullish := 0
	bearish := 0
	for _, r := range reports {
		if r.Rating == "strong_buy" || r.Rating == "bullish" {
			bullish++
		}
		if r.Rating == "strong_sell" || r.Rating == "bearish" {
			bearish++
		}
	}
	if bullish > bearish+1 {
		return "buy"
	}
	if bearish > bullish+1 {
		return "sell"
	}
	if bearish > bullish {
		return "hold"
	}
	if bullish > bearish {
		return "hold"
	}
	return "hold"
}

func extractStructuredFields(ctx context.Context, cc *CommodityContext, report *CommodityReport) {
	chatModel, err := multi.GetChatModelWithTier(ctx, "struct_extract", multi.LLMTierQuick, cc.AIConfigID)
	if err != nil {
		logger.SugaredLogger.Warnf("commodity struct extract LLM unavailable, skipping: %v", err)
		return
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: GetRolePrompt("commodity_struct_extract", StructExtractPrompt)},
		{Role: schema.User, Content: report.Conclusion},
	}

	result, err := chatModel.Generate(ctx, messages)
	if err != nil {
		logger.SugaredLogger.Warnf("commodity struct extract LLM error, skipping: %v", err)
		return
	}

	content := result.Content
	if content == "" {
		return
	}

	jsonStr := content
	if idx := strings.Index(content, "```json\n"); idx >= 0 {
		content = content[idx+8:]
		if end := strings.Index(content, "\n```"); end >= 0 {
			jsonStr = content[:end]
		}
	} else if idx := strings.Index(content, "```"); idx >= 0 {
		content = content[idx+3:]
		if end := strings.Index(content, "```"); end >= 0 {
			jsonStr = content[:end]
		}
	}

	var extracted struct {
		Score     float64         `json:"score"`
		Trend     string          `json:"trend"`
		EntryZone *PriceZone      `json:"entryZone"`
		ExitZone  *PriceZone      `json:"exitZone"`
		RiskLevel string          `json:"riskLevel"`
		Checklist []ChecklistItem `json:"checklist"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &extracted); err != nil {
		logger.SugaredLogger.Warnf("commodity struct extract JSON parse error: %v", err)
		return
	}

	if extracted.Score >= 1 && extracted.Score <= 10 {
		report.Score = extracted.Score
	}
	switch extracted.Trend {
	case "up", "down", "sideways":
		report.Trend = extracted.Trend
	}
	if extracted.EntryZone != nil && extracted.EntryZone.Low > 0 && extracted.EntryZone.High > 0 {
		report.EntryZone = extracted.EntryZone
	}
	if extracted.ExitZone != nil && extracted.ExitZone.Low > 0 && extracted.ExitZone.High > 0 {
		report.ExitZone = extracted.ExitZone
	}
	switch extracted.RiskLevel {
	case "low", "medium", "high":
		report.RiskLevel = extracted.RiskLevel
	}
	if len(extracted.Checklist) > 0 {
		report.Checklist = extracted.Checklist
	}

	logger.SugaredLogger.Infof("commodity struct extract successful: score=%.1f trend=%s risk=%s items=%d",
		report.Score, report.Trend, report.RiskLevel, len(report.Checklist))
}
