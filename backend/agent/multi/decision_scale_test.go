package multi

import "testing"

func TestApplyDecisionScale(t *testing.T) {
	t.Run("按分数档位_无风控调整", func(t *testing.T) {
		ac := &AgentContext{FinalReport: &FinalReport{Score: 8.5, OverallRating: "buy"}}
		applyDecisionScale(ac)
		fr := ac.FinalReport
		if fr.DecisionSignal != "strong_buy" || fr.DecisionAction != "buy" || fr.DecisionLabel != "强烈买入" {
			t.Errorf("got %s/%s/%s, want strong_buy/buy/强烈买入",
				fr.DecisionSignal, fr.DecisionAction, fr.DecisionLabel)
		}
	})

	t.Run("D4钳位_高分但被否决为hold", func(t *testing.T) {
		ac := &AgentContext{FinalReport: &FinalReport{
			Score: 8.0, OverallRating: "hold", GuardrailReason: "风控裁判否决买入",
		}}
		applyDecisionScale(ac)
		fr := ac.FinalReport
		if fr.DecisionSignal != "watch" || fr.DecisionAction != "hold" || fr.DecisionLabel != "观望" {
			t.Errorf("got %s/%s/%s, want watch/hold/观望",
				fr.DecisionSignal, fr.DecisionAction, fr.DecisionLabel)
		}
	})

	t.Run("D4钳位_降级为sell", func(t *testing.T) {
		ac := &AgentContext{FinalReport: &FinalReport{
			Score: 7.0, OverallRating: "sell", GuardrailReason: "高风险降级",
		}}
		applyDecisionScale(ac)
		fr := ac.FinalReport
		if fr.DecisionSignal != "reduce" || fr.DecisionAction != "sell" {
			t.Errorf("got %s/%s, want reduce/sell", fr.DecisionSignal, fr.DecisionAction)
		}
	})

	t.Run("档位动作与最终动作一致时保留分数档", func(t *testing.T) {
		// score 3.0 → 30 → 减仓档（动作 sell），D4 调整后为 sell，动作一致不钳位
		ac := &AgentContext{FinalReport: &FinalReport{
			Score: 3.0, OverallRating: "sell", GuardrailReason: "高风险降级",
		}}
		applyDecisionScale(ac)
		if ac.FinalReport.DecisionSignal != "reduce" {
			t.Errorf("got %s, want reduce", ac.FinalReport.DecisionSignal)
		}
	})

	t.Run("GuardrailReason为空时不钳位", func(t *testing.T) {
		// 无 D4 触发（GuardrailReason 空）即使 rating 与分数档动作不一致也按分数档
		ac := &AgentContext{FinalReport: &FinalReport{Score: 8.0, OverallRating: "hold"}}
		applyDecisionScale(ac)
		if ac.FinalReport.DecisionSignal != "strong_buy" {
			t.Errorf("got %s, want strong_buy", ac.FinalReport.DecisionSignal)
		}
	})

	t.Run("Score未评估时不填", func(t *testing.T) {
		ac := &AgentContext{FinalReport: &FinalReport{Score: 0}}
		applyDecisionScale(ac)
		if ac.FinalReport.DecisionSignal != "" || ac.FinalReport.DecisionAction != "" {
			t.Error("Score<=0 时不应填充决策标尺字段")
		}
	})

	t.Run("nil安全", func(t *testing.T) {
		applyDecisionScale(nil)
		applyDecisionScale(&AgentContext{})
	})
}
