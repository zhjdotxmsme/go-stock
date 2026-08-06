package multi

import (
	"context"
	"testing"
	"time"
)

// TestParseAgentMode 模式解析：有效值、大小写、未知/空回退 standard。
func TestParseAgentMode(t *testing.T) {
	cases := map[string]AgentMode{
		"quick": ModeQuick, "QUICK": ModeQuick, " Quick ": ModeQuick,
		"standard": ModeStandard, "full": ModeFull, "specialist": ModeSpecialist,
		"": ModeStandard, "unknown": ModeStandard, "快速": ModeStandard,
	}
	for in, want := range cases {
		if got := ParseAgentMode(in); got != want {
			t.Errorf("ParseAgentMode(%q): got %s, want %s", in, got, want)
		}
	}
}

// TestDefaultConfig 默认配置即 standard 模式（现有行为不变）。
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultEngineConfig()
	if cfg.Mode != ModeStandard {
		t.Errorf("默认模式应为 standard, got %s", cfg.Mode)
	}
	if cfg.StageMinBudget != 15*time.Second {
		t.Errorf("默认阶段最小预算应为 15s, got %v", cfg.StageMinBudget)
	}
	// 零值 normalize 回退 standard
	zero := EngineConfig{}.normalize()
	if zero.Mode != ModeStandard || zero.StageMinBudget != 15*time.Second {
		t.Errorf("零值 normalize: %+v", zero)
	}
	// quick 默认分析师子集
	quick := EngineConfig{Mode: ModeQuick}.normalize()
	if len(quick.QuickAnalysts) != 1 || quick.QuickAnalysts[0] != "technical" {
		t.Errorf("quick 默认子集: %v", quick.QuickAnalysts)
	}
	// 引擎默认配置 = standard
	e := NewMultiAgentEngine(1)
	if e.config.Mode != ModeStandard {
		t.Errorf("引擎默认模式: %s", e.config.Mode)
	}
	// WithConfig 覆盖
	e = e.WithConfig(EngineConfig{Mode: ModeFull})
	if e.config.Mode != ModeFull || e.config.StageMinBudget != 15*time.Second {
		t.Errorf("WithConfig: %+v", e.config)
	}
}

// TestPlanStages 各模式阶段计划。
func TestPlanStages(t *testing.T) {
	// standard：不经模式编排
	if got := planStages(EngineConfig{Mode: ModeStandard}); got != nil {
		t.Errorf("standard 应返回 nil 计划, got %v", got)
	}

	// quick：分析师 → 合成，无辩论/风控/技能
	quick := planStages(EngineConfig{Mode: ModeQuick})
	if len(quick) != 2 || quick[0].id != stageAnalysts || quick[1].id != stageSynthesis {
		t.Errorf("quick 计划: %v", quick)
	}
	for _, st := range quick {
		if !st.required {
			t.Errorf("quick 全部阶段必需: %v", st)
		}
	}

	// full：分析师 → 辩论 → 风控辩论 → 合成；辩论/风控可跳过，合成必需
	full := planStages(EngineConfig{Mode: ModeFull})
	wantIDs := []string{stageAnalysts, stageDebate, stageRiskDebate, stageSynthesis}
	if len(full) != len(wantIDs) {
		t.Fatalf("full 计划长度: %d", len(full))
	}
	for i, id := range wantIDs {
		if full[i].id != id {
			t.Errorf("full[%d]: got %s, want %s", i, full[i].id, id)
		}
	}
	if full[3].required != true || full[1].required != false || full[2].required != false {
		t.Errorf("full required 标记: %v", full)
	}

	// specialist：full + 技能阶段（在合成之前）
	spec := planStages(EngineConfig{Mode: ModeSpecialist})
	wantIDs = []string{stageAnalysts, stageDebate, stageRiskDebate, stageSkills, stageSynthesis}
	if len(spec) != len(wantIDs) {
		t.Fatalf("specialist 计划长度: %d", len(spec))
	}
	for i, id := range wantIDs {
		if spec[i].id != id {
			t.Errorf("specialist[%d]: got %s, want %s", i, spec[i].id, id)
		}
	}
	if spec[3].required {
		t.Error("技能阶段应可被预算跳过")
	}
}

// TestBudgetTracker 预算控制：不限预算 / 剩余充足 / 剩余 < 15s 拒绝启动。
func TestBudgetTracker(t *testing.T) {
	// 不限预算：永远可启动
	unlimited := newBudgetTracker(0, 15*time.Second, nil)
	if unlimited.limited() || !unlimited.canStart() {
		t.Error("不限预算应永远可启动")
	}

	// 注入假时钟：总预算 60s（base 与 cur 必须分开，clock 是 cur 的别名）
	base := time.Now()
	cur := base
	clock := &cur
	bt := newBudgetTracker(60*time.Second, 15*time.Second, func() time.Time { return *clock })
	if !bt.limited() || !bt.canStart() {
		t.Error("剩余 60s 应可启动")
	}
	// 剩余恰好 15s：可启动（>=）
	*clock = base.Add(45 * time.Second)
	if !bt.canStart() {
		t.Error("剩余 15s 应可启动")
	}
	// 剩余 14s < 15s：拒绝（跳过而非启动注定超时的阶段）
	*clock = base.Add(46 * time.Second)
	if bt.canStart() {
		t.Error("剩余 14s 应拒绝启动")
	}
	if bt.remaining() != 14*time.Second {
		t.Errorf("剩余预算: %v", bt.remaining())
	}
	// 预算耗尽
	*clock = base.Add(61 * time.Second)
	if bt.canStart() {
		t.Error("预算耗尽应拒绝启动")
	}
}

// stubSkillAgent 技能 Agent 占位实现（接口编译与调用验证）。
type stubSkillAgent struct {
	name  string
	ran   bool
	runErr error
}

func (s *stubSkillAgent) Name() string { return s.name }
func (s *stubSkillAgent) Run(ctx context.Context, ac *AgentContext) error {
	s.ran = true
	return s.runErr
}

// TestSkillAgentHook 技能 Agent 挂点可注入并被调用。
func TestSkillAgentHook(t *testing.T) {
	var agents []SkillAgent
	stub := &stubSkillAgent{name: "chip_analysis"}
	agents = append(agents, stub)
	if agents[0].Name() != "chip_analysis" {
		t.Errorf("技能 Agent 名: %s", agents[0].Name())
	}
	if err := agents[0].Run(context.Background(), &AgentContext{}); err != nil {
		t.Errorf("技能 Agent 执行: %v", err)
	}
	if !stub.ran {
		t.Error("技能 Agent 未被调用")
	}
}

// TestRiskDebateHook 风控辩论挂点类型可注入。
func TestRiskDebateHook(t *testing.T) {
	called := false
	var hook RiskDebateHook = func(ctx context.Context, ac *AgentContext) error {
		called = true
		return nil
	}
	cfg := EngineConfig{Mode: ModeFull, RiskDebate: hook}.normalize()
	if cfg.RiskDebate == nil {
		t.Fatal("挂点应保留")
	}
	if err := cfg.RiskDebate(context.Background(), &AgentContext{}); err != nil || !called {
		t.Errorf("挂点调用: called=%v err=%v", called, err)
	}
}

// TestAnalystRunners quick 子集角色映射完整。
func TestAnalystRunners(t *testing.T) {
	for _, role := range []string{"fundamental", "technical", "sentiment", "news", "policy", "hotmoney", "lockup"} {
		if _, ok := analystRunners[role]; !ok {
			t.Errorf("缺少分析师角色 %q", role)
		}
	}
	if _, ok := analystRunners["nonexistent"]; ok {
		t.Error("未知角色不应存在")
	}
}
