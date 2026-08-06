package multi

import (
	"go-stock/backend/internal/domain/analysis"
	"go-stock/backend/logger"
)

// A4 增强（方案 §8.1 D5）：决策标尺接入。
// standard 管线（engine.go）与模式管线（engine_mode.go）都在合成完成、
// 结果落库前调用 applyDecisionScale；纯确定性计算，不触碰 Score/Rating 等既有字段。

// applyDecisionScale 计算 D5 决策标尺档位并写入 FinalReport。
// Score 为 1-10 分制，换算到 0-100 后查档；Score<=0（未评估）时不填。
// D4 风控否决已调整 OverallRating 时（GuardrailReason 非空），若分数档位的
// 标准动作与调整后动作不一致，按调整后动作钳位档位（hold→观望档、sell→减仓档、
// buy 不升级），保证 DecisionAction 与最终动作一致。
func applyDecisionScale(ac *AgentContext) {
	if ac == nil || ac.FinalReport == nil || ac.FinalReport.Score <= 0 {
		return
	}
	band := analysis.SignalForScore(ac.FinalReport.Score * 10)

	if ac.FinalReport.GuardrailReason != "" {
		finalAction := mapRatingToAction(ac.FinalReport.OverallRating)
		if band.Action != finalAction {
			band = bandForAction(finalAction)
		}
	}

	ac.FinalReport.DecisionSignal = string(band.Signal)
	ac.FinalReport.DecisionAction = band.Action
	ac.FinalReport.DecisionLabel = band.LabelZH
	logger.SugaredLogger.Infof("decision scale: score=%.1f signal=%s action=%s label=%s",
		ac.FinalReport.Score, band.Signal, band.Action, band.LabelZH)
}

// bandForAction D4 钳位用档位：hold→观望、sell→减仓、buy→买入（不升级到强烈买入）。
func bandForAction(action string) analysis.DecisionBand {
	target := analysis.SignalWatch
	switch action {
	case analysis.ActionSell:
		target = analysis.SignalReduce
	case analysis.ActionBuy:
		target = analysis.SignalBuy
	}
	for _, b := range analysis.DecisionScale {
		if b.Signal == target {
			return b
		}
	}
	return analysis.DecisionScale[len(analysis.DecisionScale)-1]
}
