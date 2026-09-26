package risk

import (
	"math"
	"sort"
)

const ewmaLambda = 0.94

// VaR 计算三种 VaR（99% 等置信度，返回负值表示损失）。
// returns 为日收益率序列（正序），需 ≥100 个点，|r|>20% 的点剔除。
// hybrid = 0.4×historical + 0.6×EWMA(λ=0.94)
func VaR(returns []float64, confidence float64) (hist, ewma, hybrid float64) {
	clean := make([]float64, 0, len(returns))
	for _, r := range returns {
		if math.IsNaN(r) || math.IsInf(r, 0) || math.Abs(r) > 0.20 {
			continue
		}
		clean = append(clean, r)
	}
	if len(clean) < 100 {
		return 0, 0, 0
	}
	sorted := make([]float64, len(clean))
	copy(sorted, clean)
	sort.Float64s(sorted)

	alpha := 1 - confidence
	// 历史分位（线性插值）
	pos := alpha * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	hist = sorted[lo]
	if hi > lo {
		hist = sorted[lo] + (sorted[hi]-sorted[lo])*(pos-float64(lo))
	}

	// EWMA：近期权重高（w_i = λ^(n-1-i)），加权分位 + 线性插值
	weights := make([]float64, len(clean))
	wSum := 0.0
	for i := range clean {
		weights[i] = math.Pow(ewmaLambda, float64(len(clean)-1-i))
		wSum += weights[i]
	}
	// 按收益率升序累加时间权重
	type pair struct {
		r float64
		w float64
	}
	pairs := make([]pair, len(clean))
	for i := range clean {
		pairs[i] = pair{clean[i], weights[i] / wSum}
	}
	sort.Slice(pairs, func(a, b int) bool { return pairs[a].r < pairs[b].r })
	cum := 0.0
	ewma = pairs[0].r
	for i, p := range pairs {
		cum += p.w
		if cum >= alpha {
			if i > 0 && p.w > 0 {
				// 在 i-1 与 i 之间按累积权重线性插值
				prevCum := cum - p.w
				frac := (alpha - prevCum) / p.w
				ewma = pairs[i-1].r + (p.r-pairs[i-1].r)*frac
			} else {
				ewma = p.r
			}
			break
		}
	}
	hybrid = 0.4*hist + 0.6*ewma
	return hist, ewma, hybrid
}

// MultiDay 多日 VaR：平方根法则
func MultiDay(var1d float64, days int) float64 {
	return var1d * math.Sqrt(float64(days))
}
