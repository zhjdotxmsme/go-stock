package risk

import (
	"math"
	"testing"
)

func TestVaRHistorical(t *testing.T) {
	// 100 个收益率：-0.10, -0.098, ..., 0.088（步长 0.002），99% 置信度
	returns := make([]float64, 100)
	for i := range returns {
		returns[i] = -0.10 + 0.002*float64(i)
	}
	hist, _, _ := VaR(returns, 0.99)
	// 1% 分位数 ≈ -0.10（最小值附近）
	if hist > -0.095 || hist < -0.105 {
		t.Fatalf("hist VaR expect ~-0.10, got %v", hist)
	}
}

func TestVaRHybrid(t *testing.T) {
	returns := make([]float64, 200)
	for i := range returns {
		returns[i] = -0.05 + 0.0005*float64(i)
	}
	hist, ewma, hybrid := VaR(returns, 0.99)
	if math.Abs(hybrid-(0.4*hist+0.6*ewma)) > 1e-9 {
		t.Fatalf("hybrid 应为 0.4hist+0.6ewma: %v %v %v", hist, ewma, hybrid)
	}
}

func TestLevelOf(t *testing.T) {
	cases := []struct {
		var1d      float64
		name       string
		posLimit   float64
		scoreExtra float64
	}{
		{-0.02, "正常", 1.0, 0},
		{-0.04, "注意", 0.7, 5},
		{-0.06, "危险", 0.4, 15},
		{-0.09, "崩溃", 0, 999},
	}
	for _, c := range cases {
		lv := LevelOf(c.var1d)
		if lv.Name != c.name || lv.PositionLimit != c.posLimit || lv.ScoreExtra != c.scoreExtra {
			t.Fatalf("LevelOf(%v)=%+v, want %s", c.var1d, lv, c.name)
		}
	}
}

func TestMultiDay(t *testing.T) {
	if v := MultiDay(-0.03, 5); math.Abs(v-(-0.03*math.Sqrt(5))) > 1e-9 {
		t.Fatalf("√n 法则错误: %v", v)
	}
}
