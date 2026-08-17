package multi

import (
	"context"
	"math"
	"strings"
	"time"

	"go-stock/backend/agent/multi/risk_debate"
)

// 4 模式 Agent 编排（方案 §8.1 D11）。
// standard 为默认模式，行为与历史管线完全一致（走 engine.go 原有管线代码）；
// quick/full/specialist 经 runModePipeline 按阶段计划执行，带预算控制。

// AgentMode Agent 编排模式。
type AgentMode string

const (
	// ModeQuick 快速模式：精简分析师子集 → 合成（跳过辩论与风控）。
	ModeQuick AgentMode = "quick"
	// ModeStandard 标准模式：7 分析师并行 → 多空辩论 → 合成（默认）。
	ModeStandard AgentMode = "standard"
	// ModeFull 完整模式：standard + 合成后风控辩论（T1 已默认接线）+ D4 风控否决。
	ModeFull AgentMode = "full"
	// ModeSpecialist 专家模式：full + 技能 Agent（挂点，SkillRouter 预留）。
	ModeSpecialist AgentMode = "specialist"
)

// ParseAgentMode 解析模式字符串；空值/未知值回退 standard（保证默认行为不变）。
func ParseAgentMode(s string) AgentMode {
	switch AgentMode(strings.ToLower(strings.TrimSpace(s))) {
	case ModeQuick:
		return ModeQuick
	case ModeFull:
		return ModeFull
	case ModeSpecialist:
		return ModeSpecialist
	default:
		return ModeStandard
	}
}

// RiskDebateHook 风控辩论阶段挂点（full/specialist 模式）。
// T1 risk_debate 包（backend/agent/multi/risk_debate）已就绪，
// 接线时由调用方将引擎的 LLM 调用适配为 risk_debate.Engine 后挂入本钩子。
type RiskDebateHook func(ctx context.Context, ac *AgentContext) error

// SkillAgent 技能 Agent 占位接口（specialist 模式，SkillRouter 预留）。
type SkillAgent interface {
	Name() string
	Run(ctx context.Context, ac *AgentContext) error
}

// EngineConfig 引擎配置：模式 + 预算控制 + 挂点。
// 零值/默认配置即 standard 模式，现有调用方无感知。
type EngineConfig struct {
	Mode AgentMode // 编排模式，默认 standard

	TotalBudget    time.Duration // 管线总预算，0 = 不限
	StageMinBudget time.Duration // 剩余预算低于该值时跳过非必需阶段，默认 15s
	StageTimeout   time.Duration // 单阶段超时，0 = 不限

	QuickAnalysts []string      // quick 模式分析师子集，默认 ["technical"]
	RiskDebate    RiskDebateHook // full/specialist 风控辩论挂点（nil = 引擎默认 T1 接线）
	SkillAgents   []SkillAgent  // specialist 技能 Agent 列表（空 = 跳过该阶段）

	// RiskDebateCall 风控辩论 LLM 调用注入（测试/自定义模型用）；
	// nil 时默认接线按 LLMTierQuick/Deep 从 AI 配置装配。
	RiskDebateCall risk_debate.LLMCallFunc

	// DisagreementGuidanceOff 关闭 D6 分歧引导注入合成 Prompt（默认 false=开启）。
	// 分类与事件透出不受此开关影响。
	DisagreementGuidanceOff bool

	// MemoryInjectionOff 关闭 T2 反思记忆检索注入分析师 Prompt（默认 false=开启）。
	// 关闭时分析师 Prompt 与历史版本逐字节一致。
	MemoryInjectionOff bool

	// TestPanicHook 测试专用注入点：非 nil 时在 Run goroutine 内先执行；
	// panic 用于验证 recover 护栏。生产代码必须为 nil。
	TestPanicHook func()
}

// DefaultEngineConfig 返回默认配置（standard 模式，15s 阶段最小预算）。
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{Mode: ModeStandard, StageMinBudget: 15 * time.Second}
}

// normalize 填充默认值（空模式回退 standard，quick 默认子集等）。
func (c EngineConfig) normalize() EngineConfig {
	if c.Mode == "" {
		c.Mode = ModeStandard
	}
	if c.StageMinBudget <= 0 {
		c.StageMinBudget = 15 * time.Second
	}
	if c.Mode == ModeQuick && len(c.QuickAnalysts) == 0 {
		c.QuickAnalysts = []string{"technical"}
	}
	return c
}

// 阶段 ID（复用现有前端事件 phase 名，风控/技能为新增）。
const (
	stageAnalysts   = "analysts"
	stageDebate     = "debate"
	stageRiskDebate = "risk_debate"
	stageSkills     = "skills"
	stageSynthesis  = "synthesis"
)

// plannedStage 阶段计划项：required=true 的阶段不可被预算控制跳过
// （分析师与合成为产出必需；辩论/风控/技能可被跳过）。
type plannedStage struct {
	id       string
	required bool
	label    string
}

// planStages 按模式展开阶段计划。
// standard 返回 nil：走 engine.go 现有管线，不经模式编排（行为逐字节不变）。
func planStages(cfg EngineConfig) []plannedStage {
	switch cfg.Mode {
	case ModeQuick:
		return []plannedStage{
			{stageAnalysts, true, "技术分析师分析中..."},
			{stageSynthesis, true, "正在生成最终报告..."},
		}
	case ModeFull, ModeSpecialist:
		// A3 顺序（DSA）：分析师 → 多空辩论 → [技能] → 合成 → 风控辩论（T1+D4 在合成后，
		// 因为风控裁判需要 Trader 决策 = 合成最终信号，D4 否决在合成信号上调整）。
		stages := []plannedStage{
			{stageAnalysts, true, "各维度分析师并行分析中..."},
			{stageDebate, false, "多空研究员辩论中..."},
		}
		if cfg.Mode == ModeSpecialist {
			stages = append(stages, plannedStage{stageSkills, false, "技能 Agent 分析中..."})
		}
		stages = append(stages,
			plannedStage{stageSynthesis, true, "正在生成最终报告..."},
			plannedStage{stageRiskDebate, false, "风控辩论中..."},
		)
		return stages
	default:
		return nil
	}
}

// budgetTracker 预算追踪器（方案 §8.1 D11：
// 每阶段独立超时预算；剩余 < 15s 时跳过而非启动注定超时的阶段）。
type budgetTracker struct {
	deadline time.Time // 零值 = 不限预算
	minStage time.Duration
	now      func() time.Time // 可注入时钟，便于测试
}

// newBudgetTracker 构造预算追踪器；total<=0 表示不限预算。
func newBudgetTracker(total, minStage time.Duration, now func() time.Time) *budgetTracker {
	if now == nil {
		now = time.Now
	}
	bt := &budgetTracker{minStage: minStage, now: now}
	if total > 0 {
		bt.deadline = now().Add(total)
	}
	return bt
}

// limited 是否启用预算控制。
func (b *budgetTracker) limited() bool { return !b.deadline.IsZero() }

// remaining 剩余预算；不限预算时返回极大值。
func (b *budgetTracker) remaining() time.Duration {
	if !b.limited() {
		return time.Duration(math.MaxInt64)
	}
	return b.deadline.Sub(b.now())
}

// canStart 是否允许启动一个非必需阶段（剩余 < minStage 时拒绝）。
func (b *budgetTracker) canStart() bool {
	if !b.limited() {
		return true
	}
	return b.remaining() >= b.minStage
}
