package data

// Kronos 因子与滚动回测的纯函数单测

import (
	"testing"
	"time"

	"go-stock/backend/data/datasource"
)

func TestKronosFactorScore(t *testing.T) {
	mk := func(chg, conf float64) *KronosPrediction {
		return &KronosPrediction{Summary: &KronosForecast{ChangePct: chg, Confidence: conf}}
	}
	cases := []struct {
		name string
		pred *KronosPrediction
		want float64
	}{
		{"nil", nil, 0},
		{"涨5% 一致度100", mk(5, 100), 75},   // base=75, conf=1 → 75
		{"跌5% 一致度100", mk(-5, 100), 25},  // base=25
		{"涨5% 一致度0", mk(5, 0), 50},       // 全部收缩到中性
		{"涨5% 一致度50", mk(5, 50), 62.5},   // 75*0.5+50*0.5
		{"超涨截断", mk(30, 100), 100},       // chg 截断到 10 → base=100
		{"超跌截断", mk(-30, 100), 0},
	}
	for _, c := range cases {
		if got := kronosFactorScore(c.pred); got != c.want {
			t.Errorf("%s: kronosFactorScore=%v, want %v", c.name, got, c.want)
		}
	}
}

func mkKronosTestBars(closes []float64) []datasource.KLineBar {
	bars := make([]datasource.KLineBar, len(closes))
	for i, c := range closes {
		bars[i] = datasource.KLineBar{
			Time:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i),
			Close: c, High: c * 1.02, Low: c * 0.98, Open: c,
		}
	}
	return bars
}

func TestSimulateTrade(t *testing.T) {
	// 入场 100，5 根：101,102,103,104,105（low≈0.98*c ≥ 100*0.92=92，无止损触发）
	bars := mkKronosTestBars([]float64{100, 101, 102, 103, 104, 105})
	got := simulateTrade(bars, 0, 5, 0.08, 0.20)
	if got != 5 {
		t.Errorf("持有到期: got %v want 5", got)
	}
	// 止盈 3%：第1根 high=101*1.02=103.02 ≥ 103 → 触发
	got = simulateTrade(bars, 0, 5, 0.08, 0.03)
	if got != 3 {
		t.Errorf("止盈触发: got %v want 3", got)
	}
	// 止损：下跌序列 100 → 90（low=88.2 ≤ 92 触发 -8%）
	bars2 := mkKronosTestBars([]float64{100, 90, 91, 92, 93, 94})
	got = simulateTrade(bars2, 0, 5, 0.08, 0)
	if got != -8 {
		t.Errorf("止损触发: got %v want -8", got)
	}
	// 无止损：到期收益 = 94/100-1 = -6%
	got = simulateTrade(bars2, 0, 5, 0, 0)
	if got != -6 {
		t.Errorf("无止损持有到期: got %v want -6", got)
	}
}
