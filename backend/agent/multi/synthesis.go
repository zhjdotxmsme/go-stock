package multi

import (
	"context"
	"errors"
	"fmt"
	"go-stock/backend/agent/multi/signal"
	"go-stock/backend/agent/strategy"
	"go-stock/backend/logger"
	"io"

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

	synthesisContent := GetRolePrompt("multi_synthesis", SynthesisPrompt)
	if ac.StrategyCode != "" {
		if s := strategy.GetByCode(ac.StrategyCode); s != nil {
			synthesisContent += "\n\n【策略视角】\n" + s.Prompt
		}
	}
	// D6 分歧引导（A3）：仅模式管线设置 SynthesisGuidance，
	// standard 管线该字段恒为空，Prompt 与历史版本逐字节一致。
	if ac.SynthesisGuidance != "" {
		synthesisContent += "\n\n【分析师分歧引导】\n" + ac.SynthesisGuidance
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: synthesisContent},
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
// 使用多层降级策略：
// Tier 1: 轻量 LLM 调用提取 JSON
// Tier 2: 关键词提取
// Tier 3: 中文价格正则模式匹配
// Tier 4: 智能估算 (基于分析师评级)
func extractStructuredFields(ctx context.Context, ac *AgentContext, report *FinalReport) {
	// 首先尝试 LLM 提取 (Tier 1)
	chatModel, err := GetChatModelWithTier(ctx, "struct_extract", LLMTierQuick, ac.AIConfigID)
	if err == nil {
		messages := []*schema.Message{
			{Role: schema.System, Content: GetRolePrompt("multi_struct_extract", StructExtractPrompt)},
			{Role: schema.User, Content: report.Conclusion},
		}

		result, err := chatModel.Generate(ctx, messages)
		if err == nil && result.Content != "" {
			// 使用多层降级提取器解析 LLM 响应
			reportWriter := &reportWriterAdapter{report}
			provider := &analystReportProviderAdapter{ac}
			signal.ExtractStructured(result.Content, reportWriter, provider)
			logger.SugaredLogger.Infof("struct extract successful: score=%.1f trend=%s risk=%s",
				report.Score, report.Trend, report.RiskLevel)
			return
		}
	}

	// Tier 1 失败，降级到 Tier 2-4 (纯模式匹配)
	logger.SugaredLogger.Warnf("LLM struct extract failed, using pattern-based fallback")
	reportWriter := &reportWriterAdapter{report}
	provider := &analystReportProviderAdapter{ac}
	signal.ExtractStructured(report.Conclusion, reportWriter, provider)
}

// reportWriterAdapter 实现 signal.ReportWriter 接口 (避免循环依赖)
type reportWriterAdapter struct {
	report *FinalReport
}

func (a *reportWriterAdapter) SetScore(score float64) {
	a.report.Score = score
}

func (a *reportWriterAdapter) SetTrend(trend string) {
	a.report.Trend = trend
}

func (a *reportWriterAdapter) SetEntryZone(low, high float64) {
	a.report.EntryZone = &PriceZone{Low: low, High: high}
}

func (a *reportWriterAdapter) SetExitZone(low, high float64) {
	a.report.ExitZone = &PriceZone{Low: low, High: high}
}

func (a *reportWriterAdapter) SetRiskLevel(risk string) {
	a.report.RiskLevel = risk
}

// analystReportProviderAdapter 实现 signal.AnalystReportProvider 接口
type analystReportProviderAdapter struct {
	ac *AgentContext
}

func (a *analystReportProviderAdapter) GetReportCount() int {
	return len(a.ac.Reports)
}

func (a *analystReportProviderAdapter) GetReportRating(index int) string {
	if index < 0 || index >= len(a.ac.Reports) {
		return ""
	}
	return a.ac.Reports[index].Rating
}
