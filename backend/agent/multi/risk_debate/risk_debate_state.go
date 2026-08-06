// Package risk_debate 实现三方风控辩论 + 风控裁判（方案 §8.1 T1）。
// 在 Trader 决策之后，Risky → Safe → Neutral 三方循环辩论（每方看到其他两方
// 最新回复并必须直接回应），最多 3 轮后强制进入 Risk Judge 裁决（BUY/SELL/HOLD，
// 可否决 Trader 决策）。
// LLM 调用通过函数注入（同 D2 ranking 的 LLMCallFunc 模式），本包不直连任何
// LLM 配置与数据源，保持纯逻辑可测试；不接入引擎管线（接线属后续任务）。
package risk_debate

import (
	"fmt"
	"strings"
)

// Role 风控辩论角色。
type Role string

const (
	RoleRisky   Role = "risky"   // 激进分析师：主张高收益，挑战保守
	RoleSafe    Role = "safe"    // 保守分析师：保护资产，质疑风险
	RoleNeutral Role = "neutral" // 中立分析师：权衡双方，挑战极端
)

// debateOrder 每轮发言顺序（方案：Risky → Safe → Neutral 循环）。
var debateOrder = []Role{RoleRisky, RoleSafe, RoleNeutral}

// Speech 一方在某一轮的发言。
type Speech struct {
	Role    Role   `json:"role"`
	Round   int    `json:"round"`
	Content string `json:"content"`
}

// VetoRecord 风控裁判对 Trader 决策的否决记录。
type VetoRecord struct {
	TraderDecision string `json:"traderDecision"` // Trader 原始决策（原文）
	JudgeDecision  string `json:"judgeDecision"`  // 裁判裁决 BUY/SELL/HOLD
	Vetoed         bool   `json:"vetoed"`         // 是否否决了 Trader
	ParseFailed    bool   `json:"parseFailed"`    // 裁决解析失败（已默认 HOLD）
}

// State 三方辩论状态：发言历史 + 轮次 + 否决记录。
type State struct {
	MaxRounds int         `json:"maxRounds"`
	Speeches  []Speech    `json:"speeches"`
	Veto      *VetoRecord `json:"veto,omitempty"`
}

// NewState 构造辩论状态。
func NewState(maxRounds int) *State {
	return &State{MaxRounds: maxRounds}
}

// AddSpeech 记录一方发言（轮次按发言序号自动推进，每 3 方发言为一轮）。
func (s *State) AddSpeech(role Role, content string) {
	s.Speeches = append(s.Speeches, Speech{
		Role:    role,
		Round:   len(s.Speeches)/len(debateOrder) + 1,
		Content: content,
	})
}

// Rounds 已完成的辩论轮次。
func (s *State) Rounds() int {
	return len(s.Speeches) / len(debateOrder)
}

// LatestByRole 某角色最近一次发言内容（无则返回空串）。
func (s *State) LatestByRole(role Role) string {
	for i := len(s.Speeches) - 1; i >= 0; i-- {
		if s.Speeches[i].Role == role {
			return s.Speeches[i].Content
		}
	}
	return ""
}

// HistoryText 格式化全部发言历史（裁判 Prompt 用）。
func (s *State) HistoryText() string {
	var sb strings.Builder
	for _, sp := range s.Speeches {
		fmt.Fprintf(&sb, "【%s 第%d轮】\n%s\n\n", roleNameZH(sp.Role), sp.Round, sp.Content)
	}
	return sb.String()
}

// roleNameZH 角色中文名。
func roleNameZH(r Role) string {
	switch r {
	case RoleRisky:
		return "激进分析师"
	case RoleSafe:
		return "保守分析师"
	case RoleNeutral:
		return "中立分析师"
	default:
		return string(r)
	}
}
