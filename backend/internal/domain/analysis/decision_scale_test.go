package analysis

import "testing"

// TestSignalForScore 5 档标尺分数映射与边界。
func TestSignalForScore(t *testing.T) {
	cases := []struct {
		score  float64
		signal DecisionSignal
		action string
		label  string
	}{
		{100, SignalStrongBuy, ActionBuy, "强烈买入"},
		{80, SignalStrongBuy, ActionBuy, "强烈买入"},
		{79.9, SignalBuy, ActionBuy, "买入"},
		{60, SignalBuy, ActionBuy, "买入"},
		{59.9, SignalWatch, ActionHold, "观望"},
		{40, SignalWatch, ActionHold, "观望"},
		{39.9, SignalReduce, ActionSell, "减仓"},
		{20, SignalReduce, ActionSell, "减仓"},
		{19.9, SignalSell, ActionSell, "卖出"},
		{0, SignalSell, ActionSell, "卖出"},
		// 越界钳位
		{120, SignalStrongBuy, ActionBuy, "强烈买入"},
		{-5, SignalSell, ActionSell, "卖出"},
	}
	for _, tc := range cases {
		band := SignalForScore(tc.score)
		if band.Signal != tc.signal || band.Action != tc.action || band.LabelZH != tc.label {
			t.Errorf("SignalForScore(%.1f): got (%s, %s, %s), want (%s, %s, %s)",
				tc.score, band.Signal, band.Action, band.LabelZH, tc.signal, tc.action, tc.label)
		}
	}
}

// TestNeedsGuardrail 护栏校验：action 与分数档位标准动作不一致时需要 guardrail_reason。
func TestNeedsGuardrail(t *testing.T) {
	cases := []struct {
		score  float64
		action string
		want   bool
	}{
		{85, ActionBuy, false},  // 强烈买入档 + buy，一致
		{85, ActionHold, true},  // 高分却是 hold，需说明
		{85, ActionSell, true},  // 高分却是 sell，需说明
		{50, ActionHold, false}, // 观望档 + hold，一致
		{50, ActionBuy, true},   // 观望档却是 buy，需说明
		{30, ActionSell, false}, // 减仓档 + sell，一致
		{10, ActionSell, false}, // 卖出档 + sell，一致
		{10, ActionHold, true},  // 卖出档却是 hold，需说明
	}
	for _, tc := range cases {
		if got := NeedsGuardrail(tc.score, tc.action); got != tc.want {
			t.Errorf("NeedsGuardrail(%.1f, %q): got %v, want %v", tc.score, tc.action, got, tc.want)
		}
	}
}
