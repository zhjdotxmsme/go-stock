package postanalysis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ScorecardConfig 本地评分卡配置（方案 §8.1 D10：文档称 18 参数，
// 此处把每个条件的阈值与分值全部外提，共 21 项）。JSON 序列化，惯例同 D1/D3。
//
// 6 个加分条件：价值质量+2.4、资金确认+1.8、控制反转+1.2、高置信度+0.8、
// 催化剂+0.5/条（封顶）、风控低风险+0.6（文档只列 5 类，第 6 类按风格补充）。
// 3 个减分条件：热钱不稳-2.5、量比异常-1.2、低置信度-1.0。总调整 ±MaxDelta 封顶。
type ScorecardConfig struct {
	// 价值质量：PE/PB 双低
	ValueBonus float64 `json:"valueBonus"` // 默认 2.4
	ValuePEMax float64 `json:"valuePeMax"` // 默认 30（PE>0 且 ≤ 该值）
	ValuePBMax float64 `json:"valuePbMax"` // 默认 3
	// 资金确认：成交额达标且量比在健康区间（放量但非异常）
	FundBonus     float64 `json:"fundBonus"`     // 默认 1.8
	FundAmountMin float64 `json:"fundAmountMin"` // 默认 2e8（元）
	FundVRMin     float64 `json:"fundVrMin"`     // 默认 1.2
	FundVRMax     float64 `json:"fundVrMax"`     // 默认 5
	// 控制反转：可控回调（跌幅在 [dropMin, -1] 区间）且信号分达标
	ReversalBonus     float64 `json:"reversalBonus"`     // 默认 1.2
	ReversalDropMin   float64 `json:"reversalDropMin"`   // 默认 -5（%）
	ReversalSignalMin float64 `json:"reversalSignalMin"` // 默认 60
	// 高置信度
	ConfidenceHighBonus     float64 `json:"confidenceHighBonus"`     // 默认 0.8
	ConfidenceHighThreshold float64 `json:"confidenceHighThreshold"` // 默认 0.75
	// 催化剂：每条加分，封顶
	CatalystPerItem float64 `json:"catalystPerItem"` // 默认 0.5
	CatalystCap     float64 `json:"catalystCap"`     // 默认 2.0（最多计 4 条）
	// 风控低风险
	LowRiskBonus float64 `json:"lowRiskBonus"` // 默认 0.6
	// 减分：热钱不稳
	HotMoneyPenalty float64 `json:"hotMoneyPenalty"` // 默认 2.5（以正数存储扣分幅度）
	// 减分：量比异常
	VRAbnormalPenalty   float64 `json:"vrAbnormalPenalty"`   // 默认 1.2
	VRAbnormalThreshold float64 `json:"vrAbnormalThreshold"` // 默认 6
	// 减分：低置信度
	ConfidenceLowPenalty   float64 `json:"confidenceLowPenalty"`   // 默认 1.0
	ConfidenceLowThreshold float64 `json:"confidenceLowThreshold"` // 默认 0.35
	// 总调整上限
	MaxDelta float64 `json:"maxDelta"` // 默认 8.0
}

// DefaultScorecardConfig 返回方案 §8.1 D10 规格的默认参数。
func DefaultScorecardConfig() ScorecardConfig {
	return ScorecardConfig{
		ValueBonus: 2.4, ValuePEMax: 30, ValuePBMax: 3,
		FundBonus: 1.8, FundAmountMin: 2e8, FundVRMin: 1.2, FundVRMax: 5,
		ReversalBonus: 1.2, ReversalDropMin: -5, ReversalSignalMin: 60,
		ConfidenceHighBonus: 0.8, ConfidenceHighThreshold: 0.75,
		CatalystPerItem: 0.5, CatalystCap: 2.0,
		LowRiskBonus:      0.6,
		HotMoneyPenalty:   2.5,
		VRAbnormalPenalty: 1.2, VRAbnormalThreshold: 6,
		ConfidenceLowPenalty: 1.0, ConfidenceLowThreshold: 0.35,
		MaxDelta: 8.0,
	}
}

// LoadScorecardConfigJSON 从 JSON 字节流加载配置；缺省字段保留默认值。
func LoadScorecardConfigJSON(data []byte) (ScorecardConfig, error) {
	cfg := DefaultScorecardConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("解析评分卡配置失败: %w", err)
	}
	return cfg, nil
}

// Scorecard 本地评分卡分析器。
type Scorecard struct {
	Config ScorecardConfig
}

// NewScorecard 按配置构造评分卡。
func NewScorecard(cfg ScorecardConfig) *Scorecard {
	return &Scorecard{Config: cfg}
}

func (s *Scorecard) Name() string { return "local_scorecard" }

// Analyze 对每只候选独立评估加/减分条件，总调整 ±MaxDelta 封顶。
func (s *Scorecard) Analyze(ctx context.Context, candidates []CandidateInput) ([]AnalyzerOutcome, error) {
	outcomes := make([]AnalyzerOutcome, len(candidates))
	for i := range candidates {
		outcomes[i] = s.scoreOne(&candidates[i])
	}
	return outcomes, nil
}

// scoreOne 评估单只候选。
func (s *Scorecard) scoreOne(in *CandidateInput) AnalyzerOutcome {
	c := s.Config
	delta := 0.0
	var hits []string

	// ===== 加分条件 =====
	// 1. 价值质量：PE>0 且 PE/PB 双低
	if in.PE > 0 && in.PE <= c.ValuePEMax && in.PB > 0 && in.PB <= c.ValuePBMax {
		delta += c.ValueBonus
		hits = append(hits, "价值质量")
	}
	// 2. 资金确认：成交额达标且量比在健康区间
	if in.Amount >= c.FundAmountMin && in.VolumeRatio >= c.FundVRMin && in.VolumeRatio <= c.FundVRMax {
		delta += c.FundBonus
		hits = append(hits, "资金确认")
	}
	// 3. 控制反转：可控回调且信号分达标
	if in.ChangePercent >= c.ReversalDropMin && in.ChangePercent <= -1 && in.SignalScore >= c.ReversalSignalMin {
		delta += c.ReversalBonus
		hits = append(hits, "控制反转")
	}
	// 4. 高置信度
	if in.LLMConfidence >= c.ConfidenceHighThreshold {
		delta += c.ConfidenceHighBonus
		hits = append(hits, "高置信度")
	}
	// 5. 催化剂：每条加分，封顶
	if n := len(in.Catalysts); n > 0 {
		bonus := float64(n) * c.CatalystPerItem
		if bonus > c.CatalystCap {
			bonus = c.CatalystCap
		}
		delta += bonus
		hits = append(hits, fmt.Sprintf("催化剂×%d", n))
	}
	// 6. 风控低风险
	if in.RiskLevel == "low" {
		delta += c.LowRiskBonus
		hits = append(hits, "风控低风险")
	}

	// ===== 减分条件 =====
	// 1. 热钱不稳
	if in.HotMoneyUnstable {
		delta -= c.HotMoneyPenalty
		hits = append(hits, "热钱不稳")
	}
	// 2. 量比异常
	if in.VolumeRatio >= c.VRAbnormalThreshold {
		delta -= c.VRAbnormalPenalty
		hits = append(hits, "量比异常")
	}
	// 3. 低置信度
	if in.LLMConfidence > 0 && in.LLMConfidence < c.ConfidenceLowThreshold {
		delta -= c.ConfidenceLowPenalty
		hits = append(hits, "低置信度")
	}

	// ±MaxDelta 封顶
	if c.MaxDelta > 0 {
		if delta > c.MaxDelta {
			delta = c.MaxDelta
		} else if delta < -c.MaxDelta {
			delta = -c.MaxDelta
		}
	}
	return AnalyzerOutcome{Delta: delta, Detail: strings.Join(hits, "、")}
}
