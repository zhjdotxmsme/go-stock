package multi

import (
	"context"
	"fmt"
	"strings"

	"go-stock/backend/agent/multi/risk_debate"
	"go-stock/backend/agent/strategy/disagreement"
	"go-stock/backend/agent/strategy/risk"
	"go-stock/backend/logger"

	"github.com/cloudwego/eino/schema"
)

// A3 增强（方案 §8.1）：D6 分歧分类 → 合成引导；T1 风控辩论 → D4 风控否决。
// 本文件只被模式管线（quick/full/specialist，engine_mode.go）调用；
// standard 管线（engine.go）不经过这里，行为与历史版本完全一致。

// ===== D6 分歧分类 =====

// classifyDisagreement 在分析师完成后对报告做 11 类分歧分类：
//   - Rating（bullish/bearish/neutral）经 NormalizeSignal 映射为 buy/sell/hold；
//   - 分析失败（Error 非空）的报告视为降级输入；
//   - 本阶段没有风控角色报告，Classify 内部的风控强制转 hold 规则自然不触发。
//
// 分类结果写入 AgentContext 并透出 SSE 事件；引导文本注入合成 Prompt（可开关）。
func (e *MultiAgentEngine) classifyDisagreement(ctx context.Context, ac *AgentContext, ch chan *schema.Message, guidanceEnabled bool) {
	opinions := make([]disagreement.AgentOpinion, 0, len(ac.Reports))
	for _, r := range ac.Reports {
		if r.Error != "" {
			opinions = append(opinions, disagreement.AgentOpinion{Role: r.Role, Signal: disagreement.OpinionDegraded})
			continue
		}
		opinions = append(opinions, disagreement.AgentOpinion{Role: r.Role, Signal: disagreement.NormalizeSignal(r.Rating)})
	}

	cls := disagreement.Classify(opinions)
	hint := cls.DecisionHint()
	ac.DisagreementClass = string(cls)
	ac.DecisionHint = hint
	if guidanceEnabled {
		ac.SynthesisGuidance = disagreement.SynthesisGuidance(cls)
	}

	logger.SugaredLogger.Infof("disagreement classification: class=%s hint=%s", cls, hint)
	emitEvent(ctx, ch, "agent:phase", map[string]string{
		"phase": "disagreement", "status": "end",
		"label": fmt.Sprintf("分析师分歧分类：%s", hint),
		"class": string(cls), "hint": hint,
	})
}

// ===== T1 风控辩论（默认接线）=====

// makeTierLLMCall 把 multi 包现有的分层 LLM 调用（LLMTierQuick/Deep）适配为
// risk_debate.LLMCallFunc：model 参数取 "quick"/"deep" 层级关键字。
func makeTierLLMCall(aiConfigID int) risk_debate.LLMCallFunc {
	return func(ctx context.Context, model, prompt string) (string, error) {
		tier, role := LLMTierQuick, "risk_debate"
		if model == "deep" {
			tier, role = LLMTierDeep, "risk_judge"
		}
		chatModel, err := GetChatModelWithTier(ctx, role, tier, aiConfigID)
		if err != nil {
			return "", err
		}
		msg, err := chatModel.Generate(ctx, []*schema.Message{{Role: schema.User, Content: prompt}})
		if err != nil {
			return "", err
		}
		return msg.Content, nil
	}
}

// defaultRiskDebateHook T1 真实接线：三方风控辩论（Risky/Safe/Neutral，quick 层）
// + 风控裁判（deep 层）+ D4 风控否决。发言与裁决经 SSE 事件透出。
// 任何失败返回 error（调用方记日志继续：不否决、不阻塞结果）。
func (e *MultiAgentEngine) defaultRiskDebateHook() RiskDebateHook {
	return func(ctx context.Context, ac *AgentContext) error {
		if ac.FinalReport == nil {
			logger.SugaredLogger.Warn("risk debate skipped: no final report (synthesis failed)")
			return nil
		}

		dc := risk_debate.DebateContext{
			StockCode:      ac.StockCode,
			StockName:      ac.StockName,
			TraderDecision: mapRatingToDecision(ac.FinalReport.OverallRating),
			TraderPlan:     truncateSummary(ac.FinalReport.Conclusion, 500),
			BullBearDebate: buildDebateSummary(ac),
			AnalystReports: buildAnalystSummaries(ac),
		}

		call := e.config.normalize().RiskDebateCall
		if call == nil {
			call = makeTierLLMCall(ac.AIConfigID)
		}
		result, err := risk_debate.NewEngine(3, "quick", "deep", call, nil).Run(ctx, dc)
		if err != nil {
			return err
		}
		ac.RiskDebate = result

		// SSE 透出：三方发言（复用 agent:debate 事件模式）与裁判裁决
		for _, sp := range result.Speeches {
			emitDebate(ac, sp.Round, "risk_"+string(sp.Role), sp.Content)
		}
		emitACEvent(ctx, ac, "agent:phase", map[string]string{
			"phase": "risk_judge", "status": "end",
			"label":    fmt.Sprintf("风控裁判裁决：%s", result.Decision),
			"decision": result.Decision,
			"vetoed":   fmt.Sprintf("%v", result.VetoedTrader),
		})

		// D4 风控否决/降级（裁决与合成信号不一致时按状态机调整）
		applyGuardrailOverride(ctx, ac, result)
		return nil
	}
}

// ===== D4 风控否决 =====

// applyGuardrailOverride 按 D4 状态机评估风控覆盖：
// 裁判否决（Veto: buy→hold）/ 裁判 SELL 或报告高风险（downgrade）。
// 调整理由写入 FinalReport.GuardrailReason 并透出 SSE 事件。
// 裁决解析失败时不否决（降级原则：宁可不动，不可误动）。
func applyGuardrailOverride(ctx context.Context, ac *AgentContext, res *risk_debate.RiskDebateResult) {
	if ac.FinalReport == nil {
		return
	}
	ac.FinalReport.RiskJudgeDecision = res.Decision
	if res.ParseFailed {
		logger.SugaredLogger.Warn("risk judge decision parse failed, no override applied")
		return
	}

	out := risk.EvaluateOverride(risk.OverrideInput{
		Action:        mapRatingToAction(ac.FinalReport.OverallRating),
		RiskLevel:     risk.RiskLevel(ac.FinalReport.RiskLevel),
		RiskAgentVeto: res.VetoedTrader,
		RiskAgentHigh: res.Decision == risk_debate.DecisionSell,
	})
	if !out.Triggered {
		return
	}

	original := ac.FinalReport.OverallRating
	ac.FinalReport.OverallRating = mapActionToRating(out.FinalAction)
	ac.FinalReport.GuardrailReason = out.Reason
	logger.SugaredLogger.Infof("guardrail override: %s → %s (%s)", original, out.FinalAction, out.Reason)
	emitACEvent(ctx, ac, "agent:phase", map[string]string{
		"phase": "risk_override", "status": "end",
		"label":    "风控调整：" + out.Reason,
		"original": original,
		"final":    ac.FinalReport.OverallRating,
		"reason":   out.Reason,
	})
}

// ===== 装配辅助 =====

// emitACEvent 向 AgentContext 事件通道发事件（通道为 nil 时静默跳过，避免死锁）。
func emitACEvent(ctx context.Context, ac *AgentContext, eventType string, data map[string]string) {
	if ac == nil || ac.StreamCh == nil {
		return
	}
	emitEvent(ctx, ac.StreamCh, eventType, data)
}

// mapRatingToDecision FinalReport.OverallRating → risk_debate 裁决词汇 BUY/SELL/HOLD。
func mapRatingToDecision(rating string) string {
	switch strings.ToLower(rating) {
	case "strong_buy", "buy":
		return risk_debate.DecisionBuy
	case "strong_sell", "sell":
		return risk_debate.DecisionSell
	default:
		return risk_debate.DecisionHold
	}
}

// mapRatingToAction FinalReport.OverallRating → D4 动作词汇 buy/hold/sell。
func mapRatingToAction(rating string) string {
	switch strings.ToLower(rating) {
	case "strong_buy", "buy":
		return risk.ActionBuy
	case "strong_sell", "sell":
		return risk.ActionSell
	default:
		return risk.ActionHold
	}
}

// mapActionToRating D4 动作 → FinalReport.OverallRating（只可能被降级，不会出现升级）。
func mapActionToRating(action string) string {
	switch action {
	case risk.ActionBuy:
		return "buy"
	case risk.ActionSell:
		return "sell"
	default:
		return "hold"
	}
}

// buildDebateSummary 多空辩论摘要（风控辩论上下文用）。
func buildDebateSummary(ac *AgentContext) string {
	if ac.Debate == nil {
		return ""
	}
	var sb strings.Builder
	for _, round := range ac.Debate.Rounds {
		fmt.Fprintf(&sb, "第%d轮 多方: %s\n第%d轮 空方: %s\n",
			round.RoundNum, truncateSummary(round.BullArgument, 200),
			round.RoundNum, truncateSummary(round.BearArgument, 200))
	}
	for _, item := range ac.Debate.ConsensusItems {
		fmt.Fprintf(&sb, "共识: %s\n", item)
	}
	for _, item := range ac.Debate.Disagreements {
		fmt.Fprintf(&sb, "分歧: %s\n", item)
	}
	return sb.String()
}

// buildAnalystSummaries 分析师报告摘要（风控辩论上下文用）。
func buildAnalystSummaries(ac *AgentContext) string {
	var sb strings.Builder
	for _, r := range ac.Reports {
		if r.Error != "" {
			fmt.Fprintf(&sb, "【%s】数据不可用\n", r.Role)
			continue
		}
		fmt.Fprintf(&sb, "【%s 评级:%s】%s\n", r.Role, r.Rating, truncateSummary(r.Summary, 150))
	}
	return sb.String()
}
