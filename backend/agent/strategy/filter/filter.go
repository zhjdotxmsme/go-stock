// Package filter 实现 36 项硬过滤器 + 瀑布诊断（方案 §8.1 D7）。
// 快照级规则（9 类 15 参数）与日线级规则（15 类 23 参数）按序执行，
// 每层记录淘汰数量、样本行，单层淘汰率 >90% 时给出告警建议。
// 纯函数：输入由调用方装配（日线派生字段可用 D1 scoring 的指标函数预计算），
// 本包不直连任何数据源。
package filter

import "fmt"

// FilterInput 过滤输入（快照字段 + 日线派生字段，风格同 D1 scoring.FactorInput）。
// 日线派生字段由调用方预计算（MA 排列/信号分/突破幅度/波动率等）。
type FilterInput struct {
	Code string `json:"code"`
	Name string `json:"name"`

	// 快照字段
	Price         float64 `json:"price"`
	ChangePercent float64 `json:"changePercent"` // 当日涨跌幅 %
	Amount        float64 `json:"amount"`        // 成交额（元）
	TotalMV       float64 `json:"totalMv"`       // 总市值（元）
	PE            float64 `json:"pe"`
	PB            float64 `json:"pb"`
	VolumeRatio   float64 `json:"volumeRatio"`
	TurnoverRate  float64 `json:"turnoverRate"` // 换手率 %

	// 日线派生字段（HasDailyData=false 时视为无日线数据）
	HasDailyData      bool    `json:"hasDailyData"`
	ChangePct60       float64 `json:"changePct60"`       // 60 日涨跌幅 %
	MABullAlign       bool    `json:"maBullAlign"`       // MA 多头排列
	AboveMA20         bool    `json:"aboveMa20"`         // 价格站上 MA20
	MA20DeviationPct  float64 `json:"ma20DeviationPct"`  // 价格相对 MA20 偏离 %（回踩幅度）
	SignalScore       float64 `json:"signalScore"`       // 日线信号分 0-100
	MACDState         string  `json:"macdState"`         // bullish/bearish/neutral
	RSIState          string  `json:"rsiState"`          // overbought/oversold/neutral
	Breakout20dPct    float64 `json:"breakout20dPct"`    // 20 日突破幅度 %
	AmplitudePct      float64 `json:"amplitudePct"`      // 振幅 %
	VolumeRatio20d    float64 `json:"volumeRatio20d"`    // 20 日量比
	BodyPct           float64 `json:"bodyPct"`           // K 线实体比例（0-1 或 %，按调用方约定）
	ConsolidationDays int     `json:"consolidationDays"` // 盘整天数
	VolatilityPct     float64 `json:"volatilityPct"`     // 年化波动率 %
	MaxDrawdownPct    float64 `json:"maxDrawdownPct"`    // 最大回撤 %（负数）
	ATRPct            float64 `json:"atrPct"`            // ATR/价格 %
}

// Rule 单条硬过滤规则：Reject 返回淘汰原因，空串表示通过。
type Rule struct {
	Name   string // 规则名（诊断输出用）
	Desc   string // 规则中文描述
	Reject func(in *FilterInput) string
}

// Pipeline 过滤管道：规则按序执行，逐层淘汰。
type Pipeline struct {
	Rules []Rule
}

// NewPipeline 按配置构建完整管道（快照级规则在前，日线级规则在后）。
func NewPipeline(cfg *HardFilterConfig) *Pipeline {
	rules := SnapshotRules(cfg)
	rules = append(rules, DailyRules(cfg)...)
	return &Pipeline{Rules: rules}
}

// Apply 对候选池执行管道过滤，返回瀑布诊断报告。
func (p *Pipeline) Apply(candidates []FilterInput) FilterReport {
	report := FilterReport{TotalInput: len(candidates)}
	survivors := candidates
	for _, rule := range p.Rules {
		stat := StageStat{Rule: rule.Name, Desc: rule.Desc, Entering: len(survivors)}
		next := make([]FilterInput, 0, len(survivors))
		for i := range survivors {
			if reason := rule.Reject(&survivors[i]); reason != "" {
				stat.Rejected++
				if len(stat.Samples) < sampleLimit {
					stat.Samples = append(stat.Samples, RejectionSample{
						Code: survivors[i].Code, Name: survivors[i].Name, Reason: reason,
					})
				}
			} else {
				next = append(next, survivors[i])
			}
		}
		if stat.Rejected > 0 {
			report.Stages = append(report.Stages, stat)
			if warning := stat.warnIfOverwhelming(); warning != "" {
				report.Warnings = append(report.Warnings, warning)
			}
		}
		survivors = next
	}
	report.Passed = survivors
	report.TotalPassed = len(survivors)
	return report
}

// 边界判定辅助：value < min（min>0 时）或 value > max（max>0 时）返回淘汰原因。
func rangeReject(name string, value, min, max float64, format string) string {
	if min > 0 && value < min {
		return fmt.Sprintf("%s %.2f 低于下限 %.2f", name, value, min)
	}
	if max > 0 && value > max {
		return fmt.Sprintf("%s %.2f 高于上限 %.2f", name, value, max)
	}
	_ = format
	return ""
}

// inWhitelist 白名单判定：空白名单不约束。
func inWhitelist(whitelist []string, value string) bool {
	if len(whitelist) == 0 {
		return true
	}
	for _, w := range whitelist {
		if w == value {
			return true
		}
	}
	return false
}
