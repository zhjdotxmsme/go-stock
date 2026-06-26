package multi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go-stock/backend/logger"
	"io"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// RunSynthesis combines all analyst reports and debate results into the final comprehensive report.
// Calls the LLM with the SynthesisPrompt to generate a structured final analysis.
func RunSynthesis(ctx context.Context, ac *AgentContext) (*FinalReport, error) {
	// Build the full context string
	var contextStr string
	contextStr += "## 各维度分析报告\n\n"
	for _, r := range ac.Reports {
		if r.Error != "" {
			contextStr += fmt.Sprintf("【%s】数据不可用\n\n", r.Role)
			continue
		}
		contextStr += fmt.Sprintf("### %s (评级: %s)\n%s\n\n", r.Role, r.Rating, r.Summary)
	}

	if ac.Debate != nil {
		contextStr += "## 多空研究员辩论\n\n"
		for _, round := range ac.Debate.Rounds {
			contextStr += fmt.Sprintf("### 第%d轮\n", round.RoundNum)
			contextStr += fmt.Sprintf("**看多方**: %s\n\n", truncateSummary(round.BullArgument, 200))
			contextStr += fmt.Sprintf("**看空方**: %s\n\n", truncateSummary(round.BearArgument, 200))
		}
		if len(ac.Debate.ConsensusItems) > 0 {
			contextStr += "**共识点**:\n"
			for _, item := range ac.Debate.ConsensusItems {
				contextStr += fmt.Sprintf("- %s\n", item)
			}
		}
		if len(ac.Debate.Disagreements) > 0 {
			contextStr += "**分歧点**:\n"
			for _, item := range ac.Debate.Disagreements {
				contextStr += fmt.Sprintf("- %s\n", item)
			}
		}
		contextStr += "\n"
	}

	report := &FinalReport{
		OverallRating:    "hold",
		InvestmentThesis: "",
		Strengths:        []string{},
		RiskFactors:      []string{},
		Catalysts:        []string{},
		MultiTimeframeView: map[string]string{
			"短期": "待分析",
			"中期": "待分析",
			"长期": "待分析",
		},
		Conclusion: "",
	}

	// Try to call LLM for structured synthesis
	chatModel, err := GetChatModelWithTier(ctx, "synthesis", LLMTierDeep, ac.AIConfigID)
	if err != nil {
		logger.SugaredLogger.Warnf("synthesis LLM unavailable, using basic aggregation: %v", err)
		return basicSynthesis(report, ac)
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: SynthesisPrompt},
		{Role: schema.User, Content: fmt.Sprintf("请基于以下分析数据生成最终投资分析报告:\n\n%s", contextStr)},
	}

	streamResult, err := chatModel.Stream(ctx, messages)
	if err != nil {
		logger.SugaredLogger.Warnf("synthesis LLM error, using basic aggregation: %v", err)
		return basicSynthesis(report, ac)
	}

	var conclusion string
	for {
		chunk, err := streamResult.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			logger.SugaredLogger.Warnf("synthesis stream error: %v", err)
			break
		}
		if chunk != nil {
			conclusion += chunk.Content
			emitToken(ac, "synthesis", chunk.Content)
		}
	}

	if conclusion != "" {
		report.Conclusion = conclusion
		report.OverallRating = aggregateRatings(ac.Reports)
	}

	// Also populate basic fields from reports
	for _, r := range ac.Reports {
		if r.Error != "" {
			continue
		}
		report.Strengths = append(report.Strengths, fmt.Sprintf("%s: %s", r.Role, r.Summary))
	}

	if ac.Debate != nil {
		for _, item := range ac.Debate.ConsensusItems {
			report.Catalysts = append(report.Catalysts, item)
		}
		for _, item := range ac.Debate.Disagreements {
			report.RiskFactors = append(report.RiskFactors, item)
		}
	}

	// 结构化提取：从结论文本中提取结构化字段（轻量 LLM 调用）
	extractStructuredFields(ctx, ac, report)

	return report, nil
}

// basicSynthesis creates a minimal report without LLM (fallback).
func basicSynthesis(report *FinalReport, ac *AgentContext) (*FinalReport, error) {
	for _, r := range ac.Reports {
		if r.Error != "" {
			continue
		}
		report.Strengths = append(report.Strengths, fmt.Sprintf("%s: %s", r.Role, r.Summary))
		report.Conclusion += fmt.Sprintf("【%s】%s\n", r.Role, r.Summary)
	}
	if ac.Debate != nil {
		for _, item := range ac.Debate.ConsensusItems {
			report.Catalysts = append(report.Catalysts, item)
		}
		for _, item := range ac.Debate.Disagreements {
			report.RiskFactors = append(report.RiskFactors, item)
		}
	}
	report.OverallRating = aggregateRatings(ac.Reports)
	return report, nil
}

// aggregateRatings computes an overall rating from individual analyst ratings.
func aggregateRatings(reports []AgentReport) string {
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

// extractStructuredFields 从已生成的 Conclusion 文本中提取结构化字段。
// 使用轻量 LLM 调用，失败不影响主流程（降级使用默认值）。
func extractStructuredFields(ctx context.Context, ac *AgentContext, report *FinalReport) {
	chatModel, err := GetChatModelWithTier(ctx, "struct_extract", LLMTierQuick, ac.AIConfigID)
	if err != nil {
		logger.SugaredLogger.Warnf("struct extract LLM unavailable, skipping: %v", err)
		return
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: StructExtractPrompt},
		{Role: schema.User, Content: report.Conclusion},
	}

	result, err := chatModel.Generate(ctx, messages)
	if err != nil {
		logger.SugaredLogger.Warnf("struct extract LLM error, skipping: %v", err)
		return
	}

	content := result.Content
	if content == "" {
		return
	}

	// Try to extract JSON from the response (the LLM should output pure JSON)
	// Handle the case where the LLM wraps JSON in markdown codeblocks
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
		logger.SugaredLogger.Warnf("struct extract JSON parse error: %v", err)
		return
	}

	// Apply extracted values (validate ranges)
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

	logger.SugaredLogger.Infof("struct extract successful: score=%.1f trend=%s risk=%s items=%d",
		report.Score, report.Trend, report.RiskLevel, len(report.Checklist))
}
