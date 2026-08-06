// Package disagreement 实现 11 类多 Agent 分歧分类（方案 §8.1 D6）。
// 在各分析师并行完成后、多空辩论/合成之前，对 Agent 意见做分歧归类，
// 并按类型生成注入合成 Prompt 的中文引导文本。
// 关键规则：风控 Agent 的看多信号强制转为 hold 后再分类，防止风控产生看多偏差。
// 纯函数，不依赖任何数据源。
package disagreement

import "strings"

// OpinionSignal Agent 意见信号。
type OpinionSignal string

const (
	OpinionBuy      OpinionSignal = "buy"
	OpinionHold     OpinionSignal = "hold"
	OpinionSell     OpinionSignal = "sell"
	OpinionDegraded OpinionSignal = "degraded" // 降级输入（分析失败/数据不足）
)

// NormalizeSignal 兼容外部评级词汇：bullish→buy、bearish→sell、neutral→hold，
// 未知/空值一律视为 degraded。
func NormalizeSignal(s string) OpinionSignal {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "buy", "bullish", "强烈买入", "买入", "看多":
		return OpinionBuy
	case "hold", "neutral", "观望", "中性":
		return OpinionHold
	case "sell", "bearish", "卖出", "减仓", "看空":
		return OpinionSell
	default:
		return OpinionDegraded
	}
}

// AgentOpinion 单个 Agent 的意见。
type AgentOpinion struct {
	Role   string        // 分析师角色（"risk"/含"风控" 的角色视为风控 Agent）
	Signal OpinionSignal // 可用 NormalizeSignal 从外部评级转换
}

// isRiskRole 判定风控 Agent 角色。
func isRiskRole(role string) bool {
	r := strings.ToLower(role)
	return strings.Contains(r, "risk") || strings.Contains(role, "风控")
}

// Class 分歧分类（方案 §8.1 D6，11 类）。
type Class string

const (
	ClassRiskOverride               Class = "risk_override"
	ClassMixedDirectional           Class = "mixed_directional"
	ClassDegradedOnly               Class = "degraded_only"
	ClassPartialBullishWithDegraded Class = "partial_bullish_with_degraded"
	ClassPartialBearishWithDegraded Class = "partial_bearish_with_degraded"
	ClassAlignedBullish             Class = "aligned_bullish"
	ClassBullishWithNeutral         Class = "bullish_with_neutral"
	ClassAlignedBearish             Class = "aligned_bearish"
	ClassBearishWithNeutral         Class = "bearish_with_neutral"
	ClassAlignedNeutral             Class = "aligned_neutral"
	ClassInsufficientOpinions       Class = "insufficient_opinions"
)

// DecisionHint 决策路径提示（方案 §8.1 D6 表格第三列）。
func (c Class) DecisionHint() string {
	switch c {
	case ClassRiskOverride:
		return "优先执行风控，限制买入信号"
	case ClassMixedDirectional:
		return "综合评估后降级信号"
	case ClassDegradedOnly:
		return "保守处理，建议观望"
	case ClassPartialBullishWithDegraded:
		return "谨慎看多"
	case ClassPartialBearishWithDegraded:
		return "谨慎看空"
	case ClassAlignedBullish:
		return "可确认看多"
	case ClassBullishWithNeutral:
		return "看多但需注意"
	case ClassAlignedBearish:
		return "可确认看空"
	case ClassBearishWithNeutral:
		return "看空但需注意"
	case ClassAlignedNeutral:
		return "观望"
	default:
		return "保守处理"
	}
}

// SynthesisGuidance 返回注入合成 Prompt 的中文引导文本。
func SynthesisGuidance(c Class) string {
	switch c {
	case ClassRiskOverride:
		return "风控已触发覆盖：请优先执行风控结论，限制任何买入倾向的信号强度，并在报告中明确说明风控理由。"
	case ClassMixedDirectional:
		return "分析师意见多空混杂、存在明显分歧：请综合评估双方论据后适当降级信号强度，并列出主要分歧点。"
	case ClassDegradedOnly:
		return "所有分析师输入均为降级状态（分析失败或数据不足）：请保守处理，建议观望，不要给出买入结论。"
	case ClassPartialBullishWithDegraded:
		return "部分分析师看多，但存在降级输入：可谨慎看多，需降低信号强度并说明降级输入带来的不确定性。"
	case ClassPartialBearishWithDegraded:
		return "部分分析师看空，但存在降级输入：可谨慎看空，需说明降级输入带来的不确定性。"
	case ClassAlignedBullish:
		return "全部有效分析师一致看多：可确认看多信号，但仍需列出主要风险点。"
	case ClassBullishWithNeutral:
		return "分析师整体看多但存在中性意见：可给出看多信号，但需注意中性分析师的顾虑并在报告中体现。"
	case ClassAlignedBearish:
		return "全部有效分析师一致看空：可确认看空信号，给出回避或减仓建议。"
	case ClassBearishWithNeutral:
		return "分析师整体看空但存在中性意见：可给出看空信号，但需说明中性分析师的不同视角。"
	case ClassAlignedNeutral:
		return "分析师意见整体中性：建议观望，不要给出方向性结论。"
	default:
		return "有效分析师意见不足：请保守处理，降低结论置信度。"
	}
}

// Classify 对一组 Agent 意见做 11 类分歧分类。
// 分类前先将风控 Agent 的看多信号强制转为 hold；
// 风控 Agent 给出看空信号视为风控触发，直接归类 risk_override。
func Classify(opinions []AgentOpinion) Class {
	if len(opinions) == 0 {
		return ClassInsufficientOpinions
	}

	buy, hold, sell, degraded := 0, 0, 0, 0
	for _, op := range opinions {
		signal := op.Signal
		if isRiskRole(op.Role) {
			// 关键规则：风控看多强制转 hold；风控看空视为风控触发
			switch signal {
			case OpinionBuy:
				signal = OpinionHold
			case OpinionSell:
				return ClassRiskOverride
			}
		}
		switch signal {
		case OpinionBuy:
			buy++
		case OpinionHold:
			hold++
		case OpinionSell:
			sell++
		default:
			degraded++
		}
	}

	if buy+hold+sell == 0 {
		return ClassDegradedOnly
	}

	// 多空同时存在 → 混杂（优先于部分方向性分类）
	if buy > 0 && sell > 0 {
		return ClassMixedDirectional
	}
	if buy > 0 {
		switch {
		case degraded > 0:
			return ClassPartialBullishWithDegraded
		case hold > 0:
			return ClassBullishWithNeutral
		default:
			return ClassAlignedBullish
		}
	}
	if sell > 0 {
		switch {
		case degraded > 0:
			return ClassPartialBearishWithDegraded
		case hold > 0:
			return ClassBearishWithNeutral
		default:
			return ClassAlignedBearish
		}
	}
	// 全部中性（允许伴随降级输入，视为整体中性）
	return ClassAlignedNeutral
}
