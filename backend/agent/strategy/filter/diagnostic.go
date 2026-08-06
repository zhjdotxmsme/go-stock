package filter

import (
	"fmt"
	"strings"
)

// 瀑布诊断（方案 §8.1 D7）：顺序展示每层过滤淘汰了多少只，
// 附带样本行和自动建议（单层 >90% 被淘汰时告警）。

// sampleLimit 每层保留的淘汰样本行数。
const sampleLimit = 3

// warnRatio 单层淘汰率告警阈值（>90%）。
const warnRatio = 0.9

// RejectionSample 淘汰样本行。
type RejectionSample struct {
	Code   string
	Name   string
	Reason string
}

// StageStat 单层过滤统计。
type StageStat struct {
	Rule     string            // 规则名
	Desc     string            // 规则中文描述
	Entering int               // 进入本层的候选数
	Rejected int               // 本层淘汰数
	Samples  []RejectionSample // 淘汰样本（最多 sampleLimit 条）
}

// warnIfOverwhelming 单层淘汰率 >90% 时生成告警建议。
func (s StageStat) warnIfOverwhelming() string {
	if s.Entering <= 0 || s.Rejected <= 0 {
		return ""
	}
	ratio := float64(s.Rejected) / float64(s.Entering)
	if ratio > warnRatio {
		return fmt.Sprintf("规则 %s(%s) 淘汰率 %.1f%%(%d/%d)，超过 90%%，建议放宽相关参数或检查数据源",
			s.Rule, s.Desc, ratio*100, s.Rejected, s.Entering)
	}
	return ""
}

// FilterReport 瀑布诊断报告。
type FilterReport struct {
	TotalInput  int           // 输入候选数
	TotalPassed int           // 最终通过数
	Passed      []FilterInput // 通过的候选
	Stages      []StageStat   // 有淘汰的层（按执行顺序）
	Warnings    []string      // 告警建议
}

// Text 瀑布诊断文本输出（日志/报告用）。
func (r *FilterReport) Text() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "过滤瀑布: 输入 %d 只 → 通过 %d 只\n", r.TotalInput, r.TotalPassed)
	for _, st := range r.Stages {
		fmt.Fprintf(&sb, "  [%s] %s: 进入 %d，淘汰 %d\n", st.Rule, st.Desc, st.Entering, st.Rejected)
		for _, sample := range st.Samples {
			fmt.Fprintf(&sb, "    样本: %s %s — %s\n", sample.Code, sample.Name, sample.Reason)
		}
	}
	for _, w := range r.Warnings {
		fmt.Fprintf(&sb, "  ⚠ 告警: %s\n", w)
	}
	return sb.String()
}
