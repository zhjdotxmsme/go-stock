package multi

import (
	"context"
	"strings"
	"testing"

	"go-stock/backend/agent/multi/risk_debate"

	"github.com/cloudwego/eino/schema"
)

// drainEvents 读空缓冲通道中的事件 JSON。
func drainEvents(ch chan *schema.Message) []string {
	var out []string
	for {
		select {
		case msg := <-ch:
			out = append(out, msg.Content)
		default:
			return out
		}
	}
}

func TestClassifyDisagreement(t *testing.T) {
	e := NewMultiAgentEngine(0)
	ch := make(chan *schema.Message, 16)
	ac := &AgentContext{
		Reports: []AgentReport{
			{Role: "fundamental", Rating: "bullish"},
			{Role: "technical", Rating: "bearish"},
			{Role: "sentiment", Rating: "neutral"},
			{Role: "news", Error: "数据不可用"}, // 降级输入
		},
		StreamCh: ch,
	}

	e.classifyDisagreement(ac, ch, true)
	if ac.DisagreementClass != "mixed_directional" {
		t.Errorf("class: got %q, want mixed_directional", ac.DisagreementClass)
	}
	if ac.DecisionHint == "" {
		t.Error("DecisionHint 应填充")
	}
	if ac.SynthesisGuidance == "" {
		t.Error("引导开启时 SynthesisGuidance 应填充")
	}

	events := drainEvents(ch)
	if len(events) != 1 || !strings.Contains(events[0], "disagreement") {
		t.Errorf("应透出 disagreement 事件: %v", events)
	}

	// 引导关闭：分类/事件照常，引导文本为空
	ac2 := &AgentContext{Reports: ac.Reports, StreamCh: ch}
	e.classifyDisagreement(ac2, ch, false)
	if ac2.SynthesisGuidance != "" {
		t.Error("DisagreementGuidanceOff 时不应注入引导文本")
	}
	if ac2.DisagreementClass != "mixed_directional" {
		t.Error("分类不受引导开关影响")
	}
}

// mockRiskDebateCall 辩论发言返回角色套话，裁判 Prompt（含"风控裁判"）返回指定裁决。
func mockRiskDebateCall(decision string) risk_debate.LLMCallFunc {
	return func(ctx context.Context, model, prompt string) (string, error) {
		if strings.Contains(prompt, "风控裁判") {
			if model != "deep" {
				return "", nil // 裁判应走 deep 层（测试中通过返回值暴露）
			}
			return "综合三方论据，风险论据充分。\n\n最终裁决: " + decision, nil
		}
		return "我方观点：风险与收益需要权衡。", nil
	}
}

func fullModeEngine(call risk_debate.LLMCallFunc) *MultiAgentEngine {
	return NewMultiAgentEngine(0).WithConfig(EngineConfig{Mode: ModeFull, RiskDebateCall: call})
}

func TestDefaultRiskDebateHookVeto(t *testing.T) {
	e := fullModeEngine(mockRiskDebateCall("SELL"))
	hook := e.defaultRiskDebateHook()

	ch := make(chan *schema.Message, 64)
	ac := &AgentContext{
		StockCode: "600519.SH", StockName: "贵州茅台",
		Reports: []AgentReport{
			{Role: "technical", Rating: "bullish", Summary: "技术面强势"},
		},
		Debate:      &DebateResult{ConsensusItems: []string{"估值合理"}},
		FinalReport: &FinalReport{OverallRating: "buy", RiskLevel: "low", Conclusion: "建议买入"},
		StreamCh:    ch,
	}

	if err := hook(context.Background(), ac); err != nil {
		t.Fatalf("hook: %v", err)
	}

	// T1：辩论结果落库到上下文
	if ac.RiskDebate == nil {
		t.Fatal("RiskDebate 结果应写入 AgentContext")
	}
	if ac.RiskDebate.Decision != "SELL" || !ac.RiskDebate.VetoedTrader {
		t.Errorf("裁决: got %s veto=%v, want SELL/true", ac.RiskDebate.Decision, ac.RiskDebate.VetoedTrader)
	}
	if len(ac.RiskDebate.Speeches) != 9 { // 3 方 × 3 轮
		t.Errorf("发言数: got %d, want 9", len(ac.RiskDebate.Speeches))
	}

	// D4：裁判否决 Trader 的 BUY → veto 状态机 buy→hold（不允许直接跳 sell）
	if ac.FinalReport.OverallRating != "hold" {
		t.Errorf("D4 否决后评级: got %q, want hold", ac.FinalReport.OverallRating)
	}
	if ac.FinalReport.GuardrailReason == "" {
		t.Error("GuardrailReason 应填充")
	}
	if ac.FinalReport.RiskJudgeDecision != "SELL" {
		t.Errorf("RiskJudgeDecision: got %q, want SELL", ac.FinalReport.RiskJudgeDecision)
	}

	// SSE 透出：发言(9) + 裁判 + 否决事件
	events := drainEvents(ch)
	if len(events) != 11 {
		t.Fatalf("事件数: got %d, want 11（9 发言+裁判+否决）", len(events))
	}
	if !strings.Contains(events[9], "risk_judge") || !strings.Contains(events[10], "risk_override") {
		t.Errorf("尾部事件应为裁判与否决: %v", events[9:])
	}
}

func TestDefaultRiskDebateHookNoOverride(t *testing.T) {
	e := fullModeEngine(mockRiskDebateCall("HOLD"))
	hook := e.defaultRiskDebateHook()

	ac := &AgentContext{
		FinalReport: &FinalReport{OverallRating: "hold", RiskLevel: "low"},
		StreamCh:    make(chan *schema.Message, 64),
	}
	if err := hook(context.Background(), ac); err != nil {
		t.Fatalf("hook: %v", err)
	}
	if ac.FinalReport.OverallRating != "hold" || ac.FinalReport.GuardrailReason != "" {
		t.Errorf("裁决与信号一致不应触发否决: %+v", ac.FinalReport)
	}
}

func TestDefaultRiskDebateHookDegraded(t *testing.T) {
	// 裁判解析失败：不否决（降级原则）
	e := fullModeEngine(func(ctx context.Context, model, prompt string) (string, error) {
		if strings.Contains(prompt, "风控裁判") {
			return "嗯……我觉得吧，这个股票嘛……", nil // 无方向词
		}
		return "观点", nil
	})
	hook := e.defaultRiskDebateHook()

	ac := &AgentContext{
		FinalReport: &FinalReport{OverallRating: "buy", RiskLevel: "low"},
		StreamCh:    make(chan *schema.Message, 64),
	}
	if err := hook(context.Background(), ac); err != nil {
		t.Fatalf("hook: %v", err)
	}
	if !ac.RiskDebate.ParseFailed {
		t.Error("无法解析的裁判输出应标记 ParseFailed")
	}
	if ac.FinalReport.OverallRating != "buy" {
		t.Error("解析失败不应否决，评级保持 buy")
	}

	// 合成失败（无 FinalReport）：静默跳过
	ac2 := &AgentContext{StreamCh: make(chan *schema.Message, 8)}
	if err := hook(context.Background(), ac2); err != nil {
		t.Errorf("无 FinalReport 应跳过而非报错: %v", err)
	}
	if ac2.RiskDebate != nil {
		t.Error("无 FinalReport 不应执行辩论")
	}
}

func TestApplyGuardrailOverrideLevels(t *testing.T) {
	// 裁判 SELL（高风险数据）且报告 RiskLevel=high：downgrade_two buy→sell，
	// 但 veto 优先级更高——VetoedTrader=false 时走 downgrade_two
	ac := &AgentContext{FinalReport: &FinalReport{OverallRating: "buy", RiskLevel: "high"}}
	res := &risk_debate.RiskDebateResult{Decision: "SELL", VetoedTrader: false}
	applyGuardrailOverride(ac, res)
	if ac.FinalReport.OverallRating != "sell" {
		t.Errorf("downgrade_two: got %q, want sell", ac.FinalReport.OverallRating)
	}
	if ac.FinalReport.GuardrailReason == "" {
		t.Error("降级理由应填充")
	}

	// 裁判 HOLD、Trader SELL：VetoedTrader=true 但动作是 sell，veto 只针对 buy → 不动
	ac2 := &AgentContext{FinalReport: &FinalReport{OverallRating: "sell", RiskLevel: "low"}}
	res2 := &risk_debate.RiskDebateResult{Decision: "HOLD", VetoedTrader: true}
	applyGuardrailOverride(ac2, res2)
	if ac2.FinalReport.OverallRating != "sell" {
		t.Errorf("sell 信号不应被否决改动: got %q", ac2.FinalReport.OverallRating)
	}
}

func TestRatingActionMappings(t *testing.T) {
	cases := map[string]string{
		"strong_buy": "buy", "buy": "buy", "BUY": "buy",
		"strong_sell": "sell", "sell": "sell",
		"hold": "hold", "neutral": "hold", "": "hold",
	}
	for rating, want := range cases {
		if got := mapRatingToAction(rating); got != want {
			t.Errorf("mapRatingToAction(%q): got %q, want %q", rating, got, want)
		}
	}
	if mapActionToRating("buy") != "buy" || mapActionToRating("hold") != "hold" || mapActionToRating("sell") != "sell" {
		t.Error("mapActionToRating 映射错误")
	}
	if mapRatingToDecision("strong_buy") != "BUY" || mapRatingToDecision("hold") != "HOLD" || mapRatingToDecision("strong_sell") != "SELL" {
		t.Error("mapRatingToDecision 映射错误")
	}
}
