package risk_debate

import (
	"context"
	"fmt"
	"strings"
)

// LLMCallFunc LLM 调用函数注入（同 D2 ranking 模式）：
// 给定模型名与 prompt，返回模型文本输出。模型与调用实现由调用方装配。
type LLMCallFunc func(ctx context.Context, model string, prompt string) (string, error)

// 裁判裁决取值。
const (
	DecisionBuy  = "BUY"
	DecisionSell = "SELL"
	DecisionHold = "HOLD"
)

// DebateContext 辩论上下文（字符串字段为主，与调用方解耦）。
type DebateContext struct {
	StockCode      string // 股票代码
	StockName      string // 股票名称
	StockInfo      string // 股票行情/基本面摘要
	TraderDecision string // Trader 决策（如 "BUY"）
	TraderPlan     string // Trader 交易计划/理由
	BullBearDebate string // 多空辩论摘要
	AnalystReports string // 分析师报告摘要
}

// RiskDebateResult 风控辩论结果。
type RiskDebateResult struct {
	Speeches       []Speech `json:"speeches"`      // 三方各轮发言（按发言顺序）
	Rounds         int      `json:"rounds"`        // 实际辩论轮次
	Decision       string   `json:"decision"`      // 最终裁决 BUY/SELL/HOLD
	VetoedTrader   bool     `json:"vetoedTrader"`  // 是否否决了 Trader 决策
	JudgeReason    string   `json:"judgeReason"`   // 裁判理由（完整输出）
	ParseFailed    bool     `json:"parseFailed"`   // 裁决解析失败（Decision 默认 HOLD）
	DebateDegraded bool     `json:"debatDegraded"` // 辩论中途失败提前进入裁决
}

// Engine 风控辩论引擎。
type Engine struct {
	MaxRounds   int         // 最大辩论轮次，默认 3（之后强制进入裁判）
	DebateModel string      // 三方辩论使用的模型
	JudgeModel  string      // 裁判使用的模型（对应 Deep 层）
	DebateCall  LLMCallFunc // 辩论 LLM 调用
	JudgeCall   LLMCallFunc // 裁判 LLM 调用（nil 时用 DebateCall）
}

// NewEngine 构造引擎；maxRounds <= 0 时按 3 处理。
func NewEngine(maxRounds int, debateModel, judgeModel string, debateCall, judgeCall LLMCallFunc) *Engine {
	if maxRounds <= 0 {
		maxRounds = 3
	}
	if judgeCall == nil {
		judgeCall = debateCall
	}
	return &Engine{
		MaxRounds:   maxRounds,
		DebateModel: debateModel,
		JudgeModel:  judgeModel,
		DebateCall:  debateCall,
		JudgeCall:   judgeCall,
	}
}

// Run 执行三方循环辩论并裁决。
// 流程：每轮 Risky → Safe → Neutral 依次发言（首轮基于上下文立论，
// 后续轮次必须直接回应其他两方最新回复）；达到轮次上限后强制进入裁判。
// 某方发言调用失败时提前结束辩论（DebateDegraded=true），以已有发言进入裁决。
func (e *Engine) Run(ctx context.Context, dc DebateContext) (*RiskDebateResult, error) {
	state := NewState(e.MaxRounds)
	degraded := false

loop:
	for round := 1; round <= e.MaxRounds; round++ {
		for _, role := range debateOrder {
			prompt := buildRolePrompt(role, dc, state)
			content, err := e.DebateCall(ctx, e.DebateModel, prompt)
			if err != nil {
				degraded = true
				break loop
			}
			state.AddSpeech(role, strings.TrimSpace(content))
		}
	}

	// 强制进入裁判裁决
	judgePrompt := BuildJudgePrompt(dc, state)
	judgeOut, err := e.JudgeCall(ctx, e.JudgeModel, judgePrompt)
	if err != nil {
		return nil, fmt.Errorf("风控裁判调用失败: %w", err)
	}

	decision, parseOK := ParseDecision(judgeOut)
	veto := &VetoRecord{
		TraderDecision: dc.TraderDecision,
		JudgeDecision:  decision,
		Vetoed:         isVeto(dc.TraderDecision, decision),
		ParseFailed:    !parseOK,
	}
	state.Veto = veto

	return &RiskDebateResult{
		Speeches:       state.Speeches,
		Rounds:         state.Rounds(),
		Decision:       decision,
		VetoedTrader:   veto.Vetoed,
		JudgeReason:    strings.TrimSpace(judgeOut),
		ParseFailed:    !parseOK,
		DebateDegraded: degraded,
	}, nil
}

// isVeto 判定裁判是否否决了 Trader 决策：
// Trader 决策可解析且与裁判裁决不一致即为否决；Trader 决策无法解析时不判定否决。
func isVeto(traderDecision, judgeDecision string) bool {
	trader, ok := NormalizeDecision(traderDecision)
	if !ok {
		return false
	}
	return trader != judgeDecision
}

// buildRolePrompt 按角色构建发言 Prompt（首轮立论，后续轮直接回应其他两方最新回复）。
func buildRolePrompt(role Role, dc DebateContext, state *State) string {
	switch role {
	case RoleRisky:
		return BuildRiskyPrompt(dc, state)
	case RoleSafe:
		return BuildSafePrompt(dc, state)
	default:
		return BuildNeutralPrompt(dc, state)
	}
}

// debateContextText 公共上下文文本。
func debateContextText(dc DebateContext) string {
	var sb strings.Builder
	if dc.StockCode != "" || dc.StockName != "" {
		fmt.Fprintf(&sb, "标的: %s %s\n", dc.StockCode, dc.StockName)
	}
	if dc.StockInfo != "" {
		fmt.Fprintf(&sb, "股票信息:\n%s\n", dc.StockInfo)
	}
	if dc.AnalystReports != "" {
		fmt.Fprintf(&sb, "分析师报告摘要:\n%s\n", dc.AnalystReports)
	}
	if dc.BullBearDebate != "" {
		fmt.Fprintf(&sb, "多空辩论摘要:\n%s\n", dc.BullBearDebate)
	}
	if dc.TraderDecision != "" {
		fmt.Fprintf(&sb, "Trader 决策: %s\n", dc.TraderDecision)
	}
	if dc.TraderPlan != "" {
		fmt.Fprintf(&sb, "Trader 交易计划:\n%s\n", dc.TraderPlan)
	}
	return sb.String()
}

// othersLatestText 其他两方最新回复（供当前发言方直接回应）。
func othersLatestText(self Role, state *State) string {
	var sb strings.Builder
	for _, role := range debateOrder {
		if role == self {
			continue
		}
		latest := state.LatestByRole(role)
		if latest != "" {
			fmt.Fprintf(&sb, "【%s最新观点】\n%s\n\n", roleNameZH(role), latest)
		}
	}
	return sb.String()
}
