package risk

import "strings"

// 风控否决/降级状态机（方案 §8.1 D4）。
// 在合成阶段产出最终信号后，按风控输入对动作做强制覆盖：
//   - 否决(Veto)：buy → hold（不允许直接跳 sell，给用户留余地）；
//   - 降级(Downgrade)：downgrade_one（buy→hold 或 hold→sell）/ downgrade_two（buy→sell）；
//   - 合法转换仅 (buy,hold) (buy,sell) (hold,sell) 三种，不允许任何升级。
//
// 动作常量与 backend/internal/domain/analysis 的 D5 决策标尺取值一致
// （buy/hold/sell），此处不复用是为了避免 risk 包依赖带 gorm 模型的 domain 包。

// 信号动作（取值与 D5 DecisionBand.Action 一致）。
const (
	ActionBuy  = "buy"
	ActionHold = "hold"
	ActionSell = "sell"
)

// OverrideType 覆盖类型。
type OverrideType string

const (
	OverrideNone         OverrideType = ""              // 未触发
	OverrideVeto         OverrideType = "veto"          // 否决：buy → hold
	OverrideDowngradeOne OverrideType = "downgrade_one" // 一步降级：buy→hold 或 hold→sell
	OverrideDowngradeTwo OverrideType = "downgrade_two" // 两步降级：buy→sell
)

// OverrideTrigger 覆盖触发来源（方案 §8.1 D4，6 个触发来源）。
type OverrideTrigger string

const (
	TriggerSignalAdjustment OverrideTrigger = "signal_adjustment" // signal_adjustment=="veto"
	TriggerRiskAgentVeto    OverrideTrigger = "risk_agent_veto"   // 风控 Agent 分析意见明确否决
	TriggerRiskLevelHigh    OverrideTrigger = "risk_level_high"   // risk_level=="high"
	TriggerHighRiskFlag     OverrideTrigger = "high_risk_flag"    // 任意 high 级 risk_flag
	TriggerRiskAgentData    OverrideTrigger = "risk_agent_data"   // risk_agent 原始数据显示高风险
	TriggerDataUnavailable  OverrideTrigger = "data_unavailable"  // 关键数据缺失（日线获取失败等）
)

// OverrideInput 风控覆盖输入（由调用方从 D3 评估结果与合成信号装配）。
type OverrideInput struct {
	Action           string    // 合成阶段产出的当前动作 buy/hold/sell
	SignalAdjustment string    // 信号调整指令（"veto" 触发否决）
	RiskLevel        RiskLevel // D3 风险等级
	HighRiskFlags    []string  // 已筛出的 high 级风险标记
	RiskAgentVeto    bool      // 风控 Agent 明确否决
	RiskAgentHigh    bool      // risk_agent 原始数据显示高风险
	DataUnavailable  bool      // 关键数据缺失
}

// OverrideResult 风控覆盖结果。
type OverrideResult struct {
	OriginalAction string
	FinalAction    string
	Type           OverrideType
	Triggered      bool
	Triggers       []OverrideTrigger
	Reason         string // 中文说明（可直接作为 D5 护栏的 guardrail_reason）
}

// IsLegalTransition 状态机合法转换校验：仅允许 (buy,hold) (buy,sell) (hold,sell)。
func IsLegalTransition(from, to string) bool {
	switch from {
	case ActionBuy:
		return to == ActionHold || to == ActionSell
	case ActionHold:
		return to == ActionSell
	default:
		return false
	}
}

// EvaluateOverride 评估风控覆盖：收集触发来源，按优先级（veto > downgrade_two > downgrade_one）
// 确定覆盖类型并调整动作。未触发时动作原样返回。
func EvaluateOverride(in OverrideInput) OverrideResult {
	result := OverrideResult{
		OriginalAction: in.Action,
		FinalAction:    in.Action,
		Type:           OverrideNone,
	}

	var vetoTriggers, twoTriggers, oneTriggers []OverrideTrigger
	if in.SignalAdjustment == string(OverrideVeto) {
		vetoTriggers = append(vetoTriggers, TriggerSignalAdjustment)
	}
	if in.RiskAgentVeto {
		vetoTriggers = append(vetoTriggers, TriggerRiskAgentVeto)
	}
	if in.RiskLevel == RiskHigh {
		twoTriggers = append(twoTriggers, TriggerRiskLevelHigh)
	}
	if len(in.HighRiskFlags) > 0 {
		twoTriggers = append(twoTriggers, TriggerHighRiskFlag)
	}
	if in.RiskAgentHigh {
		twoTriggers = append(twoTriggers, TriggerRiskAgentData)
	}
	if in.DataUnavailable {
		oneTriggers = append(oneTriggers, TriggerDataUnavailable)
	}

	switch {
	case len(vetoTriggers) > 0:
		result.Type = OverrideVeto
		result.Triggers = vetoTriggers
		// 否决只针对买入信号：buy → hold，不允许直接跳 sell
		if in.Action == ActionBuy {
			result.FinalAction = ActionHold
		}
	case len(twoTriggers) > 0:
		result.Type = OverrideDowngradeTwo
		result.Triggers = append(twoTriggers, oneTriggers...)
		result.FinalAction = applyDowngrade(in.Action, 2)
	case len(oneTriggers) > 0:
		result.Type = OverrideDowngradeOne
		result.Triggers = oneTriggers
		result.FinalAction = applyDowngrade(in.Action, 1)
	default:
		return result
	}

	result.Triggered = result.FinalAction != in.Action
	if result.Triggered {
		// 状态机约束：覆盖结果必须是合法转换（不允许升级、不允许 sell 起跳）
		if !IsLegalTransition(in.Action, result.FinalAction) {
			result.FinalAction = in.Action
			result.Triggered = false
			result.Type = OverrideNone
			result.Triggers = nil
			return result
		}
		result.Reason = buildReason(in.Action, result.FinalAction, result.Type, result.Triggers)
	}
	return result
}

// applyDowngrade 按步数降级：一步 buy→hold/hold→sell，两步 buy→sell。
func applyDowngrade(action string, steps int) string {
	for i := 0; i < steps; i++ {
		switch action {
		case ActionBuy:
			action = ActionHold
		case ActionHold:
			action = ActionSell
		}
	}
	return action
}

// buildReason 生成中文覆盖说明。
func buildReason(from, to string, t OverrideType, triggers []OverrideTrigger) string {
	names := make([]string, 0, len(triggers))
	for _, tr := range triggers {
		names = append(names, triggerNameZH(tr))
	}
	var typeZH string
	switch t {
	case OverrideVeto:
		typeZH = "风控否决"
	case OverrideDowngradeOne:
		typeZH = "风控降级（一步）"
	case OverrideDowngradeTwo:
		typeZH = "风控降级（两步）"
	}
	return "触发" + typeZH + "（" + strings.Join(names, "、") + "）：" + from + " → " + to
}

// triggerNameZH 触发来源中文名。
func triggerNameZH(t OverrideTrigger) string {
	switch t {
	case TriggerSignalAdjustment:
		return "信号调整指令否决"
	case TriggerRiskAgentVeto:
		return "风控Agent否决"
	case TriggerRiskLevelHigh:
		return "风险等级high"
	case TriggerHighRiskFlag:
		return "high级风险标记"
	case TriggerRiskAgentData:
		return "风控Agent原始数据高风险"
	case TriggerDataUnavailable:
		return "关键数据缺失"
	default:
		return string(t)
	}
}

// sanitizedFlagZH D3 检查项名 → 低敏感度中文描述（方案 §8.1 D4：
// 给 LLM 的风控标记使用简化描述，避免 LLM 对"崩盘/对倒/被套"等词过度反应）。
var sanitizedFlagZH = map[string]string{
	"chase_high":            "短期涨幅偏大",
	"break_down":            "短期波动偏大",
	"abnormal_volume_ratio": "成交活跃度异常",
	"high_turnover":         "换手率偏高",
	"invalid_pe":            "盈利状况待改善",
	"high_pb":               "估值水平偏高",
	"weak_signal":           "技术面偏弱",
	"macd_bearish":          "中期趋势偏弱",
	"rsi_overbought":        "短期超买",
	"low_llm_confidence":    "分析置信度有限",
	"llm_risk_flags":        "模型提示注意风险",
	"deep_analysis_risks":   "深度分析提示注意风险",
	"low_kline_quality":     "数据质量一般",
	"kline_fetch_failed":    "行情数据不完整",
	"stale_cache":           "数据时效性一般",
	"fallback_errors":       "数据可靠性一般",
	"invalid_data":          "数据完整性待核实",
}

// SanitizeFlags 将 D3 风险检查项名转换为低敏感度中文描述，用于注入 LLM 上下文。
// 未知名目统一返回"存在风险提示"。
func SanitizeFlags(flags []string) []string {
	out := make([]string, 0, len(flags))
	for _, f := range flags {
		if desc, ok := sanitizedFlagZH[f]; ok {
			out = append(out, desc)
		} else {
			out = append(out, "存在风险提示")
		}
	}
	return out
}
