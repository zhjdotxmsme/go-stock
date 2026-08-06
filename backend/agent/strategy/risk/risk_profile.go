package risk

import (
	"encoding/json"
	"fmt"
)

// RiskProfile 风控叠加层阈值配置（方案 §8.1 D3，27 项检查参数 + 分级基准）。
// JSON 序列化，遵循仓库 models.StrategyConfig 与 D1 scoring.ScorerConfig 的 JSON 配置惯例
// （方案文档原文为 YAML，仓库后端无 YAML 配置先例，故不引入 YAML 依赖）。
type RiskProfile struct {
	// 单日追高：涨幅 >= 阈值扣分
	ChaseThreshold float64 `json:"chaseThreshold"` // 默认 8（%）
	ChasePenalty   float64 `json:"chasePenalty"`   // 默认 4.0
	// 单日破位：跌幅 <= 阈值扣分
	BreakThreshold float64 `json:"breakThreshold"` // 默认 -7（%）
	BreakPenalty   float64 `json:"breakPenalty"`   // 默认 3.5
	// 异常量比：量比 >= 阈值扣分
	VolumeRatioThreshold float64 `json:"volumeRatioThreshold"` // 默认 6.0
	VolumeRatioPenalty   float64 `json:"volumeRatioPenalty"`   // 默认 3.0
	// 高换手：换手率 >= 阈值扣分
	TurnoverThreshold float64 `json:"turnoverThreshold"` // 默认 15（%）
	TurnoverPenalty   float64 `json:"turnoverPenalty"`   // 默认 3.0
	// 无效 PE：PE <= 0 扣分（亏损企业）
	InvalidPEPenalty float64 `json:"invalidPEPenalty"` // 默认 3.0
	// 高 PB：PB >= 阈值扣分
	PBThreshold float64 `json:"pbThreshold"` // 默认 8.0
	PBPenalty   float64 `json:"pbPenalty"`   // 默认 2.0
	// 弱日线信号：signal_score < 阈值扣分
	WeakSignalThreshold float64 `json:"weakSignalThreshold"` // 默认 45
	WeakSignalPenalty   float64 `json:"weakSignalPenalty"`   // 默认 2.5
	// MACD 空头扣分
	MACDBearishPenalty float64 `json:"macdBearishPenalty"` // 默认 2.0
	// RSI 超买扣分
	RSIOverboughtPenalty float64 `json:"rsiOverboughtPenalty"` // 默认 1.5
	// 低 LLM 置信度：confidence < 阈值扣分
	LLMConfidenceThreshold float64 `json:"llmConfidenceThreshold"` // 默认 0.35
	LLMConfidencePenalty   float64 `json:"llmConfidencePenalty"`   // 默认 1.5
	// LLM 风险标记：每条扣分，封顶
	LLMFlagPenalty float64 `json:"llmFlagPenalty"` // 默认 1.2
	LLMFlagCap     float64 `json:"llmFlagCap"`     // 默认 4.0
	// 深度分析风险：每条扣分，封顶
	DeepFlagPenalty float64 `json:"deepFlagPenalty"` // 默认 1.5
	DeepFlagCap     float64 `json:"deepFlagCap"`     // 默认 4.5
	// 低日线质量：quality < 阈值扣分
	LowQualityThreshold float64 `json:"lowQualityThreshold"` // 默认 70
	LowQualityPenalty   float64 `json:"lowQualityPenalty"`   // 默认 2.0
	// 日线获取失败扣分
	FetchFailedPenalty float64 `json:"fetchFailedPenalty"` // 默认 6.0
	// 日线缓存过期扣分
	StaleCachePenalty float64 `json:"staleCachePenalty"` // 默认 2.5
	// 数据源降级扣分
	FallbackPenalty float64 `json:"fallbackPenalty"` // 默认 1.5
	// 异常数据标记扣分（存在任意标记即扣一次）
	InvalidDataPenalty float64 `json:"invalidDataPenalty"` // 默认 3.0

	// MaxPenalty 风险分级基准分（方案 max_penalty=12.0）：
	// points >= MaxPenalty×0.66 → high，>= MaxPenalty×0.33 → medium，否则 low。
	MaxPenalty float64 `json:"maxPenalty"` // 默认 12.0
}

// DefaultRiskProfile 返回方案 §8.1 D3 规格的默认阈值。
func DefaultRiskProfile() RiskProfile {
	return RiskProfile{
		ChaseThreshold:         8,
		ChasePenalty:           4.0,
		BreakThreshold:         -7,
		BreakPenalty:           3.5,
		VolumeRatioThreshold:   6.0,
		VolumeRatioPenalty:     3.0,
		TurnoverThreshold:      15,
		TurnoverPenalty:        3.0,
		InvalidPEPenalty:       3.0,
		PBThreshold:            8.0,
		PBPenalty:              2.0,
		WeakSignalThreshold:    45,
		WeakSignalPenalty:      2.5,
		MACDBearishPenalty:     2.0,
		RSIOverboughtPenalty:   1.5,
		LLMConfidenceThreshold: 0.35,
		LLMConfidencePenalty:   1.5,
		LLMFlagPenalty:         1.2,
		LLMFlagCap:             4.0,
		DeepFlagPenalty:        1.5,
		DeepFlagCap:            4.5,
		LowQualityThreshold:    70,
		LowQualityPenalty:      2.0,
		FetchFailedPenalty:     6.0,
		StaleCachePenalty:      2.5,
		FallbackPenalty:        1.5,
		InvalidDataPenalty:     3.0,
		MaxPenalty:             12.0,
	}
}

// LoadRiskProfileJSON 从 JSON 字节流加载配置；缺省字段保留默认值。
func LoadRiskProfileJSON(data []byte) (RiskProfile, error) {
	p := DefaultRiskProfile()
	if err := json.Unmarshal(data, &p); err != nil {
		return p, fmt.Errorf("解析风控配置失败: %w", err)
	}
	return p, nil
}

// 分级比例（方案：points >= 7.92 → high，>= 3.96 → medium，基于 max_penalty=12.0）。
const (
	highRatio   = 0.66 // 12.0 × 0.66 = 7.92
	mediumRatio = 0.33 // 12.0 × 0.33 = 3.96
)

// LevelForPoints 按累计扣分判定风险等级。
func (p RiskProfile) LevelForPoints(points float64) RiskLevel {
	switch {
	case points >= p.MaxPenalty*highRatio:
		return RiskHigh
	case points >= p.MaxPenalty*mediumRatio:
		return RiskMedium
	default:
		return RiskLow
	}
}
