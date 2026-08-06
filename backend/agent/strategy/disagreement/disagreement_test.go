package disagreement

import "testing"

// TestClassify11Classes 11 类分歧各至少一个用例。
func TestClassify11Classes(t *testing.T) {
	cases := []struct {
		name     string
		opinions []AgentOpinion
		want     Class
	}{
		{"无意见", nil, ClassInsufficientOpinions},
		{"风控触发", []AgentOpinion{
			{Role: "technical", Signal: OpinionBuy},
			{Role: "risk", Signal: OpinionSell},
		}, ClassRiskOverride},
		{"多空混杂", []AgentOpinion{
			{Role: "technical", Signal: OpinionBuy},
			{Role: "fundamental", Signal: OpinionSell},
		}, ClassMixedDirectional},
		{"多空混杂优先于降级", []AgentOpinion{
			{Role: "technical", Signal: OpinionBuy},
			{Role: "fundamental", Signal: OpinionSell},
			{Role: "news", Signal: OpinionDegraded},
		}, ClassMixedDirectional},
		{"仅降级输入", []AgentOpinion{
			{Role: "technical", Signal: OpinionDegraded},
			{Role: "news", Signal: OpinionDegraded},
		}, ClassDegradedOnly},
		{"部分看多+降级", []AgentOpinion{
			{Role: "technical", Signal: OpinionBuy},
			{Role: "news", Signal: OpinionDegraded},
		}, ClassPartialBullishWithDegraded},
		{"部分看空+降级", []AgentOpinion{
			{Role: "fundamental", Signal: OpinionSell},
			{Role: "news", Signal: OpinionDegraded},
		}, ClassPartialBearishWithDegraded},
		{"全部看多", []AgentOpinion{
			{Role: "technical", Signal: OpinionBuy},
			{Role: "fundamental", Signal: OpinionBuy},
		}, ClassAlignedBullish},
		{"看多+中性", []AgentOpinion{
			{Role: "technical", Signal: OpinionBuy},
			{Role: "fundamental", Signal: OpinionHold},
		}, ClassBullishWithNeutral},
		{"全部看空", []AgentOpinion{
			{Role: "technical", Signal: OpinionSell},
			{Role: "fundamental", Signal: OpinionSell},
		}, ClassAlignedBearish},
		{"看空+中性", []AgentOpinion{
			{Role: "technical", Signal: OpinionSell},
			{Role: "fundamental", Signal: OpinionHold},
		}, ClassBearishWithNeutral},
		{"全部中性", []AgentOpinion{
			{Role: "technical", Signal: OpinionHold},
			{Role: "fundamental", Signal: OpinionHold},
		}, ClassAlignedNeutral},
		{"中性伴随降级视为整体中性", []AgentOpinion{
			{Role: "technical", Signal: OpinionHold},
			{Role: "news", Signal: OpinionDegraded},
		}, ClassAlignedNeutral},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Classify(tc.opinions); got != tc.want {
				t.Errorf("Classify: got %s, want %s", got, tc.want)
			}
		})
	}
}

// TestRiskBuyForcedToHold 风控 Agent 看多信号强制转为 hold 后再分类。
func TestRiskBuyForcedToHold(t *testing.T) {
	// 风控看多 + 一名看多：若不强制转换会是 aligned_bullish，
	// 转换后风控记为 hold → bullish_with_neutral
	got := Classify([]AgentOpinion{
		{Role: "risk", Signal: OpinionBuy},
		{Role: "technical", Signal: OpinionBuy},
	})
	if got != ClassBullishWithNeutral {
		t.Errorf("风控看多应转 hold: got %s, want %s", got, ClassBullishWithNeutral)
	}

	// 中文角色名同样识别为风控
	got = Classify([]AgentOpinion{
		{Role: "风控Agent", Signal: OpinionBuy},
		{Role: "technical", Signal: OpinionHold},
	})
	if got != ClassAlignedNeutral {
		t.Errorf("中文风控角色看多应转 hold: got %s, want %s", got, ClassAlignedNeutral)
	}

	// 风控看空直接 risk_override（即使有其他看多意见）
	got = Classify([]AgentOpinion{
		{Role: "risk_agent", Signal: OpinionSell},
		{Role: "technical", Signal: OpinionBuy},
		{Role: "fundamental", Signal: OpinionBuy},
	})
	if got != ClassRiskOverride {
		t.Errorf("风控看空应归类 risk_override: got %s", got)
	}
}

// TestHintsAndGuidance 11 类均有决策路径提示与合成引导文本。
func TestHintsAndGuidance(t *testing.T) {
	classes := []Class{
		ClassRiskOverride, ClassMixedDirectional, ClassDegradedOnly,
		ClassPartialBullishWithDegraded, ClassPartialBearishWithDegraded,
		ClassAlignedBullish, ClassBullishWithNeutral,
		ClassAlignedBearish, ClassBearishWithNeutral,
		ClassAlignedNeutral, ClassInsufficientOpinions,
	}
	for _, c := range classes {
		if c.DecisionHint() == "" {
			t.Errorf("%s 缺少决策路径提示", c)
		}
		if SynthesisGuidance(c) == "" {
			t.Errorf("%s 缺少合成引导文本", c)
		}
	}
	// 提示与文档表格一致（抽查）
	if ClassAlignedBullish.DecisionHint() != "可确认看多" {
		t.Errorf("aligned_bullish 提示: %q", ClassAlignedBullish.DecisionHint())
	}
	if ClassMixedDirectional.DecisionHint() != "综合评估后降级信号" {
		t.Errorf("mixed_directional 提示: %q", ClassMixedDirectional.DecisionHint())
	}
}

// TestNormalizeSignal 外部评级词汇归一化。
func TestNormalizeSignal(t *testing.T) {
	cases := map[string]OpinionSignal{
		"buy": OpinionBuy, "bullish": OpinionBuy, "看多": OpinionBuy,
		"hold": OpinionHold, "neutral": OpinionHold, "中性": OpinionHold,
		"sell": OpinionSell, "bearish": OpinionSell, "看空": OpinionSell,
		"": OpinionDegraded, "unknown": OpinionDegraded, "degraded": OpinionDegraded,
	}
	for in, want := range cases {
		if got := NormalizeSignal(in); got != want {
			t.Errorf("NormalizeSignal(%q): got %s, want %s", in, got, want)
		}
	}
}
