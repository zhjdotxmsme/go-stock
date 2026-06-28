package strategy

import (
	"sort"
)

// Strategy represents a trading strategy with its analytical perspective and data requirements.
// The Prompt field is appended to the SynthesisPrompt in the multi-agent LLM synthesis stage.
type Strategy struct {
	Name        string   // 显示名称: "均线策略"
	Code        string   // 唯一标识: "moving_average"
	Description string   // 简短描述
	Category    string   // "technical" / "fundamental" / "sentiment" / "event"
	Prompt      string   // LLM 分析视角 prompt（追加到 SynthesisPrompt）
	DataNeeds   []string // 数据需求: "kline" / "news" / "fundamental" / "sentiment"
	Enabled     bool     // 默认 true
}

// registry holds all registered strategies, indexed by their unique code.
var registry = make(map[string]*Strategy)

// Register adds a strategy to the global registry.
// If the strategy is nil, it is ignored. The Enabled field is set to true by default.
func Register(s *Strategy) {
	if s == nil {
		return
	}
	s.Enabled = true
	registry[s.Code] = s
}

// GetByCode returns the strategy with the given code, or nil if not found.
func GetByCode(code string) *Strategy {
	return registry[code]
}

// GetAll returns a copy of all registered strategies, sorted by Code in ascending order.
func GetAll() []*Strategy {
	strategies := make([]*Strategy, 0, len(registry))
	for _, s := range registry {
		strategies = append(strategies, s)
	}

	sort.Slice(strategies, func(i, j int) bool {
		return strategies[i].Code < strategies[j].Code
	})

	return strategies
}
