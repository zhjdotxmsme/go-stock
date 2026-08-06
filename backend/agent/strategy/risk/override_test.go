package risk

import (
	"strings"
	"testing"
)

// TestIsLegalTransition 合法转换仅 (buy,hold) (buy,sell) (hold,sell)。
func TestIsLegalTransition(t *testing.T) {
	legal := [][2]string{{"buy", "hold"}, {"buy", "sell"}, {"hold", "sell"}}
	for _, tr := range legal {
		if !IsLegalTransition(tr[0], tr[1]) {
			t.Errorf("(%s,%s) 应合法", tr[0], tr[1])
		}
	}
	illegal := [][2]string{
		{"buy", "buy"}, {"hold", "buy"}, {"hold", "hold"},
		{"sell", "buy"}, {"sell", "hold"}, {"sell", "sell"},
		{"", "buy"}, {"buy", ""},
	}
	for _, tr := range illegal {
		if IsLegalTransition(tr[0], tr[1]) {
			t.Errorf("(%s,%s) 不应合法", tr[0], tr[1])
		}
	}
}

// TestVeto 否决：buy → hold，不直接跳 sell；非 buy 动作不受影响。
func TestVeto(t *testing.T) {
	r := EvaluateOverride(OverrideInput{Action: ActionBuy, SignalAdjustment: "veto"})
	if r.Type != OverrideVeto || !r.Triggered {
		t.Fatalf("应触发否决, got %+v", r)
	}
	if r.FinalAction != ActionHold {
		t.Errorf("否决应为 buy→hold, got %s", r.FinalAction)
	}
	if len(r.Triggers) != 1 || r.Triggers[0] != TriggerSignalAdjustment {
		t.Errorf("触发来源: %v", r.Triggers)
	}
	if r.Reason == "" || !strings.Contains(r.Reason, "否决") {
		t.Errorf("应有中文覆盖说明: %q", r.Reason)
	}

	// 风控 Agent 否决意见同样触发 veto
	r = EvaluateOverride(OverrideInput{Action: ActionBuy, RiskAgentVeto: true})
	if r.Type != OverrideVeto || r.FinalAction != ActionHold || r.Triggers[0] != TriggerRiskAgentVeto {
		t.Errorf("风控Agent否决: %+v", r)
	}

	// hold/sell 动作遇否决不变化
	for _, action := range []string{ActionHold, ActionSell} {
		r = EvaluateOverride(OverrideInput{Action: action, SignalAdjustment: "veto"})
		if r.FinalAction != action || r.Triggered {
			t.Errorf("否决不应影响 %s: %+v", action, r)
		}
	}
}

// TestDowngradeTwo 两步降级：buy→sell / hold→sell；触发来源 3 种。
func TestDowngradeTwo(t *testing.T) {
	cases := []struct {
		name    string
		input   OverrideInput
		trigger OverrideTrigger
	}{
		{"风险等级high", OverrideInput{Action: ActionBuy, RiskLevel: RiskHigh}, TriggerRiskLevelHigh},
		{"high级风险标记", OverrideInput{Action: ActionBuy, HighRiskFlags: []string{"chase_high"}}, TriggerHighRiskFlag},
		{"风控Agent原始数据", OverrideInput{Action: ActionBuy, RiskAgentHigh: true}, TriggerRiskAgentData},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := EvaluateOverride(tc.input)
			if r.Type != OverrideDowngradeTwo || r.FinalAction != ActionSell {
				t.Errorf("两步降级 buy→sell: %+v", r)
			}
			if r.Triggers[0] != tc.trigger {
				t.Errorf("触发来源: got %v, want %v", r.Triggers, tc.trigger)
			}
		})
	}
	// hold 两步降级 → sell
	r := EvaluateOverride(OverrideInput{Action: ActionHold, RiskLevel: RiskHigh})
	if r.FinalAction != ActionSell || !r.Triggered {
		t.Errorf("hold 两步降级应为 sell: %+v", r)
	}
	// sell 已是终态，动作不变
	r = EvaluateOverride(OverrideInput{Action: ActionSell, RiskLevel: RiskHigh})
	if r.FinalAction != ActionSell || r.Triggered {
		t.Errorf("sell 不应再变化: %+v", r)
	}
}

// TestDowngradeOne 一步降级：buy→hold / hold→sell（数据缺失触发）。
func TestDowngradeOne(t *testing.T) {
	r := EvaluateOverride(OverrideInput{Action: ActionBuy, DataUnavailable: true})
	if r.Type != OverrideDowngradeOne || r.FinalAction != ActionHold {
		t.Errorf("一步降级 buy→hold: %+v", r)
	}
	r = EvaluateOverride(OverrideInput{Action: ActionHold, DataUnavailable: true})
	if r.Type != OverrideDowngradeOne || r.FinalAction != ActionSell {
		t.Errorf("一步降级 hold→sell: %+v", r)
	}
}

// TestOverridePrecedence 优先级：veto > downgrade_two > downgrade_one。
func TestOverridePrecedence(t *testing.T) {
	// veto + risk high 同时存在 → 按否决处理（buy→hold 而非 buy→sell）
	r := EvaluateOverride(OverrideInput{
		Action: ActionBuy, SignalAdjustment: "veto", RiskLevel: RiskHigh,
	})
	if r.Type != OverrideVeto || r.FinalAction != ActionHold {
		t.Errorf("veto 应优先于两步降级: %+v", r)
	}
	// downgrade_two + downgrade_one → 两步，触发来源合并
	r = EvaluateOverride(OverrideInput{
		Action: ActionBuy, RiskLevel: RiskHigh, DataUnavailable: true,
	})
	if r.Type != OverrideDowngradeTwo || len(r.Triggers) != 2 {
		t.Errorf("两步降级应合并触发来源: %+v", r)
	}
}

// TestNoOverride 无触发时原样返回。
func TestNoOverride(t *testing.T) {
	r := EvaluateOverride(OverrideInput{Action: ActionBuy, RiskLevel: RiskLow})
	if r.Triggered || r.Type != OverrideNone || r.FinalAction != ActionBuy || r.Reason != "" {
		t.Errorf("无触发应原样返回: %+v", r)
	}
}

// TestSanitizeFlags 低敏感度描述：检查项名 → 简化描述，未知 → 通用提示，不含刺激性词汇。
func TestSanitizeFlags(t *testing.T) {
	out := SanitizeFlags([]string{"chase_high", "kline_fetch_failed", "unknown_xyz"})
	want := []string{"短期涨幅偏大", "行情数据不完整", "存在风险提示"}
	for i, w := range want {
		if out[i] != w {
			t.Errorf("SanitizeFlags[%d]: got %q, want %q", i, out[i], w)
		}
	}
	// 全部 17 个 D3 检查项都有映射
	for _, name := range []string{
		"chase_high", "break_down", "abnormal_volume_ratio", "high_turnover", "invalid_pe",
		"high_pb", "weak_signal", "macd_bearish", "rsi_overbought", "low_llm_confidence",
		"llm_risk_flags", "deep_analysis_risks", "low_kline_quality", "kline_fetch_failed",
		"stale_cache", "fallback_errors", "invalid_data",
	} {
		got := SanitizeFlags([]string{name})[0]
		if got == "存在风险提示" {
			t.Errorf("检查项 %s 缺少低敏描述", name)
		}
		// 低敏描述不应含刺激性词汇
		for _, scary := range []string{"崩盘", "对倒", "被套", "严重"} {
			if strings.Contains(got, scary) {
				t.Errorf("检查项 %s 的低敏描述 %q 含刺激性词汇 %q", name, got, scary)
			}
		}
	}
}
