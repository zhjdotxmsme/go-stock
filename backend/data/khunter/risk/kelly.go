package risk

import (
	"encoding/json"
	"math"
	"os"
)

const kellyMaxFraction = 0.15 // 半凯利上限（决策 D6）

// KellyFraction 半凯利：f*=(p×b−q)/b，取半，clamp [0, 0.15]
func KellyFraction(winRate, profitLossRatio float64) float64 {
	if profitLossRatio <= 0 || winRate < 0 || winRate > 1 {
		return 0
	}
	q := 1 - winRate
	f := (winRate*profitLossRatio - q) / profitLossRatio
	f = f / 2 // 半凯利
	if f < 1e-12 { // 浮点残渣（如 0.4*1.5-0.6≈3.7e-17）视为 0
		return 0
	}
	return math.Min(kellyMaxFraction, f)
}

// KellyShares 按凯利仓位计算股数（100 股整数倍，不足一手返回 0）
func KellyShares(capital, price, fraction float64) int {
	if price <= 0 || fraction <= 0 {
		return 0
	}
	amount := capital * fraction
	shares := int(amount/price) / 100 * 100
	if shares < 100 {
		return 0
	}
	return shares
}

// LoadKellyConfig 加载每策略 [胜率, 盈亏比] 配置；缺失返回 nil
func LoadKellyConfig(path string) map[string][2]float64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cfg map[string][2]float64
	if json.Unmarshal(data, &cfg) != nil {
		return nil
	}
	return cfg
}
