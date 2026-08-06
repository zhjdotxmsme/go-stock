package risk_debate

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func testContext() DebateContext {
	return DebateContext{
		StockCode: "600519", StockName: "贵州茅台",
		StockInfo:      "现价 1700，PE 28.5",
		TraderDecision: "BUY",
		TraderPlan:     "建仓 20%，止损 -8%",
		BullBearDebate: "多方认为消费复苏，空方认为估值偏高",
		AnalystReports: "技术面：信号分 65；基本面：稳健",
	}
}

// scriptDebateCall 按发言顺序返回预置内容。
func scriptDebateCall(contents []string) (LLMCallFunc, *int) {
	idx := new(int)
	return func(ctx context.Context, model, prompt string) (string, error) {
		if *idx >= len(contents) {
			return "", fmt.Errorf("unexpected call %d", *idx)
		}
		content := contents[*idx]
		*idx++
		return content, nil
	}, idx
}

// TestFullDebateFlow 完整 3 方 × 3 轮流程：发言顺序、轮次、裁判裁决、否决判定。
func TestFullDebateFlow(t *testing.T) {
	debateContents := []string{}
	for round := 1; round <= 3; round++ {
		for _, role := range []string{"激进", "保守", "中立"} {
			debateContents = append(debateContents, fmt.Sprintf("%s方第%d轮观点", role, round))
		}
	}
	debateCall, _ := scriptDebateCall(debateContents)
	judgeCalls := 0
	judgeCall := func(ctx context.Context, model, prompt string) (string, error) {
		judgeCalls++
		if model != "deep-model" {
			t.Errorf("裁判应使用 JudgeModel, got %q", model)
		}
		// 裁判 Prompt 必须含辩论记录与关键指示
		if !strings.Contains(prompt, "保守方第3轮观点") {
			t.Error("裁判 Prompt 缺少辩论记录")
		}
		if !strings.Contains(prompt, "只有在有具体论据强烈支持时才选择持有") {
			t.Error("裁判 Prompt 缺少关键指示")
		}
		return "三方论据各有道理，但估值风险被低估。\n\n最终裁决: SELL", nil
	}

	e := NewEngine(3, "fast-model", "deep-model", debateCall, judgeCall)
	r, err := e.Run(context.Background(), testContext())
	if err != nil {
		t.Fatalf("Run 失败: %v", err)
	}

	if r.Rounds != 3 || len(r.Speeches) != 9 {
		t.Errorf("应为 3 轮 9 次发言: rounds=%d speeches=%d", r.Rounds, len(r.Speeches))
	}
	// 发言顺序 Risky → Safe → Neutral 循环
	wantOrder := []Role{RoleRisky, RoleSafe, RoleNeutral}
	for i, sp := range r.Speeches {
		if sp.Role != wantOrder[i%3] {
			t.Errorf("发言 %d 角色: got %s, want %s", i, sp.Role, wantOrder[i%3])
		}
		if sp.Round != i/3+1 {
			t.Errorf("发言 %d 轮次: got %d, want %d", i, sp.Round, i/3+1)
		}
	}
	if r.Decision != DecisionSell {
		t.Errorf("裁决: got %s, want SELL", r.Decision)
	}
	// Trader=BUY，裁判=SELL → 否决
	if !r.VetoedTrader {
		t.Error("裁判 SELL 与否决 Trader BUY 应成立")
	}
	if r.ParseFailed || r.DebateDegraded {
		t.Errorf("不应有解析失败/降级: %+v", r)
	}
	if judgeCalls != 1 {
		t.Errorf("裁判应调用 1 次, got %d", judgeCalls)
	}
	if r.JudgeReason == "" {
		t.Error("应保留裁判理由原文")
	}
}

// TestRoundLimitForcesJudge 轮次上限：2 轮后强制进入裁判，不再发言。
func TestRoundLimitForcesJudge(t *testing.T) {
	debateCalls := 0
	debateCall := func(ctx context.Context, model, prompt string) (string, error) {
		debateCalls++
		return "观点", nil
	}
	judgeCall := func(ctx context.Context, model, prompt string) (string, error) {
		return "最终裁决: HOLD", nil
	}
	e := NewEngine(2, "m", "m", debateCall, judgeCall)
	r, err := e.Run(context.Background(), testContext())
	if err != nil {
		t.Fatalf("Run 失败: %v", err)
	}
	if debateCalls != 6 || r.Rounds != 2 {
		t.Errorf("2 轮上限应发言 6 次: calls=%d rounds=%d", debateCalls, r.Rounds)
	}
	if r.Decision != DecisionHold {
		t.Errorf("裁决: got %s", r.Decision)
	}
	// Trader BUY vs 裁判 HOLD → 否决
	if !r.VetoedTrader {
		t.Error("HOLD 对 BUY 应构成否决")
	}
}

// TestDebateDegraded 辩论中途调用失败：提前进入裁判，标记降级。
func TestDebateDegraded(t *testing.T) {
	calls := 0
	debateCall := func(ctx context.Context, model, prompt string) (string, error) {
		calls++
		if calls == 4 { // 第 2 轮第一位发言失败
			return "", errors.New("llm timeout")
		}
		return fmt.Sprintf("观点%d", calls), nil
	}
	judgeCall := func(ctx context.Context, model, prompt string) (string, error) {
		if !strings.Contains(prompt, "观点3") || strings.Contains(prompt, "观点4") {
			t.Error("裁判应只看到已收集的发言")
		}
		return "最终裁决: BUY", nil
	}
	e := NewEngine(3, "m", "m", debateCall, judgeCall)
	r, err := e.Run(context.Background(), testContext())
	if err != nil {
		t.Fatalf("Run 失败: %v", err)
	}
	if !r.DebateDegraded || r.Rounds != 1 || len(r.Speeches) != 3 {
		t.Errorf("降级: %+v", r)
	}
	if r.Decision != DecisionBuy || r.VetoedTrader {
		t.Errorf("裁判 BUY 与 Trader BUY 一致，不应否决: %+v", r)
	}
}

// TestJudgeCallFailure 裁判调用失败返回错误。
func TestJudgeCallFailure(t *testing.T) {
	debateCall := func(ctx context.Context, model, prompt string) (string, error) { return "观点", nil }
	judgeCall := func(ctx context.Context, model, prompt string) (string, error) {
		return "", errors.New("deep model down")
	}
	e := NewEngine(1, "m", "m", debateCall, judgeCall)
	if _, err := e.Run(context.Background(), testContext()); err == nil {
		t.Error("裁判失败应返回错误")
	}
}

// TestVetoRules 否决判定各分支。
func TestVetoRules(t *testing.T) {
	cases := []struct {
		trader, judge string
		want          bool
	}{
		{"BUY", DecisionSell, true},
		{"BUY", DecisionHold, true},
		{"buy", DecisionBuy, false}, // 大小写不敏感
		{"SELL", DecisionBuy, true},
		{"HOLD", DecisionHold, false},
		{"买入", DecisionSell, true},    // Trader 中文决策
		{"垃圾文本", DecisionHold, false}, // Trader 不可解析 → 不判定否决
	}
	for _, tc := range cases {
		if got := isVeto(tc.trader, tc.judge); got != tc.want {
			t.Errorf("isVeto(%q, %s): got %v, want %v", tc.trader, tc.judge, got, tc.want)
		}
	}
}

// TestParseDecision 裁决解析各分支。
func TestParseDecision(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
		ok   bool
	}{
		{"显式裁决行", "理由如下……\n最终裁决: BUY", DecisionBuy, true},
		{"裁决行中文", "理由……\n最终裁决: 持有", DecisionHold, true},
		{"英文关键词取最后", "BUY 论据不充分……因此选择 SELL", DecisionSell, true},
		{"中文关键词取最后", "有人建议买入，但我认为应当观望", DecisionHold, true},
		{"裁决行优先于正文", "正文提到 SELL\n最终裁决: HOLD", DecisionHold, true},
		{"解析失败默认HOLD", "没有任何方向性词汇的文本", DecisionHold, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseDecision(tc.text)
			if got != tc.want || ok != tc.ok {
				t.Errorf("ParseDecision: got (%s, %v), want (%s, %v)", got, ok, tc.want, tc.ok)
			}
		})
	}
}

// TestRolePrompts 三方 Prompt：首轮立论、次轮必须直接回应其他两方最新回复。
func TestRolePrompts(t *testing.T) {
	dc := testContext()
	state := NewState(3)

	// 首轮：无他方观点，为立论模式
	p := BuildRiskyPrompt(dc, state)
	if !strings.Contains(p, "激进风控分析师") || !strings.Contains(p, "第一位发言者") {
		t.Error("激进首轮 Prompt 应为立论模式")
	}
	for _, want := range []string{"600519", "多空辩论摘要", "Trader 决策"} {
		if !strings.Contains(p, want) {
			t.Errorf("Prompt 缺少上下文 %q", want)
		}
	}

	// 次轮：包含其他两方最新回复与"直接回应"要求
	state.AddSpeech(RoleRisky, "激进观点一")
	state.AddSpeech(RoleSafe, "保守观点一")
	p = BuildNeutralPrompt(dc, state)
	if !strings.Contains(p, "激进观点一") || !strings.Contains(p, "保守观点一") {
		t.Error("中立次轮 Prompt 应含其他两方最新回复")
	}
	if !strings.Contains(p, "直接回应") {
		t.Error("次轮 Prompt 应要求直接回应")
	}
	// 中立自己的历史不应出现在"其他两方"里
	state.AddSpeech(RoleNeutral, "中立观点一")
	p = BuildNeutralPrompt(dc, state)
	if strings.Count(p, "中立观点一") != 0 {
		t.Error("其他两方文本不应含自己的发言")
	}

	// 三个角色立场关键词
	if !strings.Contains(BuildSafePrompt(dc, state), "保护资产") {
		t.Error("保守 Prompt 缺少核心使命")
	}
	if !strings.Contains(BuildNeutralPrompt(dc, state), "挑战任何极端立场") {
		t.Error("中立 Prompt 缺少核心使命")
	}
}
