package scoring

import (
	"encoding/json"
	"fmt"
	"sort"
)

// ScorerConfig 评分器配置（JSON 序列化，遵循仓库 models.StrategyConfig 的 JSON 配置惯例，
// 仓库后端无 YAML 配置先例，故不引入 YAML 依赖）。
type ScorerConfig struct {
	// TechWeight 技术面权重（0~1），用于推导默认因子权重，默认 0.35。
	// 当 Weights 非空时以 Weights 为准，TechWeight 不再参与推导。
	TechWeight float64 `json:"techWeight"`
	// Weights 显式因子权重（因子名 -> 权重），非空时覆盖 TechWeight 推导结果。
	Weights map[string]float64 `json:"weights,omitempty"`
}

// DefaultScorerConfig 返回 tech_weight=0.35 的默认配置。
func DefaultScorerConfig() ScorerConfig {
	return ScorerConfig{TechWeight: 0.35}
}

// LoadScorerConfigJSON 从 JSON 字节流加载配置；TechWeight 缺省（<=0 且未显式设置 Weights）时按 0.35 处理。
func LoadScorerConfigJSON(data []byte) (ScorerConfig, error) {
	cfg := ScorerConfig{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("解析评分器配置失败: %w", err)
	}
	if len(cfg.Weights) == 0 && cfg.TechWeight <= 0 {
		cfg.TechWeight = 0.35
	}
	return cfg, nil
}

// DefaultWeights 按 tech_weight 推导默认因子权重（方案 §8.1 D1）：
//
//	value      = (1-tw) × 0.50   （价值面）
//	liquidity  = (1-tw) × 0.25   （价值面）
//	stability  = (1-tw) × 0.25   （价值面）
//	momentum   = tw × 0.55       （技术面）
//	activity   = tw × 0.45       （技术面）
//	reversal / size / theme_heat / topic_alignment = 0（默认不启用）
func DefaultWeights(techWeight float64) map[string]float64 {
	tw := clamp(techWeight, 0, 1)
	return map[string]float64{
		"value":           (1 - tw) * 0.50,
		"momentum":        tw * 0.55,
		"activity":        tw * 0.45,
		"liquidity":       (1 - tw) * 0.25,
		"stability":       (1 - tw) * 0.25,
		"reversal":        0,
		"size":            0,
		"theme_heat":      0,
		"topic_alignment": 0,
	}
}

// ScoreResult 一只股票的综合评分结果。
type ScoreResult struct {
	Code    string                  // 股票代码
	Total   float64                 // 加权综合分 0-100
	Factors map[string]FactorResult // 各因子得分明细
	Weights map[string]float64      // 实际生效的归一化权重
}

// Scorer 因子加权聚合评分器。
type Scorer struct {
	factors []Factor
	weights map[string]float64
}

// NewScorer 注册全部 9 个因子（默认参数），并按配置设置权重。
func NewScorer(cfg ScorerConfig) *Scorer {
	s := &Scorer{
		factors: []Factor{
			NewValueFactor(),
			NewLiquidityFactor(),
			NewMomentumFactor(),
			NewReversalFactor(),
			NewActivityFactor(),
			NewStabilityFactor(),
			NewSizeFactor(),
			NewThemeHeatFactor(),
			NewTopicAlignmentFactor(),
		},
	}
	if len(cfg.Weights) > 0 {
		s.weights = make(map[string]float64, len(cfg.Weights))
		for k, v := range cfg.Weights {
			s.weights[k] = v
		}
	} else {
		s.weights = DefaultWeights(cfg.TechWeight)
	}
	return s
}

// Weights 返回当前生效的权重表（拷贝）。
func (s *Scorer) Weights() map[string]float64 {
	out := make(map[string]float64, len(s.weights))
	for k, v := range s.weights {
		out[k] = v
	}
	return out
}

// normalizedWeights 将权重归一化（权重和为 0 时返回空表，调用方按全因子等权处理）。
func (s *Scorer) normalizedWeights() map[string]float64 {
	sum := 0.0
	for _, w := range s.weights {
		if w > 0 {
			sum += w
		}
	}
	out := make(map[string]float64, len(s.weights))
	if sum <= 0 {
		return out
	}
	for k, w := range s.weights {
		if w > 0 {
			out[k] = w / sum
		}
	}
	return out
}

// Score 计算一只股票的综合评分：各因子得分按归一化权重加权求和。
// 全部权重为 0 时退化为 9 因子等权平均。
func (s *Scorer) Score(input *FactorInput) ScoreResult {
	result := ScoreResult{
		Code:    input.Code,
		Factors: make(map[string]FactorResult, len(s.factors)),
		Weights: s.normalizedWeights(),
	}

	weights := result.Weights
	equalFallback := len(weights) == 0

	total := 0.0
	for _, f := range s.factors {
		fr := f.Score(input)
		result.Factors[f.Name()] = fr
		if equalFallback {
			total += fr.Score / float64(len(s.factors))
		} else {
			total += fr.Score * weights[f.Name()]
		}
	}
	result.Total = clamp100(total)
	return result
}

// FactorNames 返回已注册因子名列表（有序，便于日志与测试断言）。
func (s *Scorer) FactorNames() []string {
	names := make([]string, 0, len(s.factors))
	for _, f := range s.factors {
		names = append(names, f.Name())
	}
	sort.Strings(names)
	return names
}
