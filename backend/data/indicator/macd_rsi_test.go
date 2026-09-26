package indicator

import (
	"math"
	"testing"
)

// refEMA 独立参考实现：SMA 种子 + 标量递推，用于交叉验证 MACD/RSI 的转写正确性。
// 与包内 EMA 语义一致但实现路径不同（单值递推，无窗口重扫）。
func refEMA(values []float64, period int) []float64 {
	n := len(values)
	out := make([]float64, n)
	for i := range out {
		out[i] = math.NaN()
	}
	if n < period {
		return out
	}
	sum := 0.0
	for j := 0; j < period; j++ {
		sum += values[j]
	}
	ema := sum / float64(period)
	out[period-1] = ema
	k := 2.0 / float64(period+1)
	for i := period; i < n; i++ {
		ema = values[i]*k + ema*(1-k)
		out[i] = ema
	}
	return out
}

func almostEq(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestMACDAgainstReference(t *testing.T) {
	// 构造含上涨/回调/震荡的 60 点序列
	close := make([]float64, 60)
	for i := range close {
		close[i] = 10 + float64(i)*0.1 + 3*math.Sin(float64(i)/3)
	}
	fast, slow, signal := 12, 26, 9

	dif, dea, hist := MACD(close, fast, slow, signal)

	// 参考实现
	ef := refEMA(close, fast)
	es := refEMA(close, slow)
	refDif := make([]float64, 60)
	for i := range refDif {
		refDif[i] = ef[i] - es[i]
	}
	refDea := refEMA(refDif[slow-1:], signal) // 从首个有效 DIF 起种子
	_ = refDea

	for i := range close {
		if i < slow-1 {
			if !math.IsNaN(dif[i]) {
				t.Fatalf("dif[%d] 应为 NaN 预热期, got %v", i, dif[i])
			}
			continue
		}
		if !almostEq(dif[i], refDif[i]) {
			t.Fatalf("dif[%d]=%v, 参考值 %v", i, dif[i], refDif[i])
		}
	}
	// DEA 预热：首个有效值在 slow-1+signal-1 = 33
	if !math.IsNaN(dea[slow+signal-3]) {
		t.Fatalf("dea[%d] 应为 NaN 预热期", slow+signal-3)
	}
	if math.IsNaN(dea[slow+signal-2]) {
		t.Fatalf("dea[%d] 应为首个有效值", slow+signal-2)
	}
	// HIST = DIF - DEA
	for i := slow + signal - 2; i < 60; i++ {
		if !almostEq(hist[i], dif[i]-dea[i]) {
			t.Fatalf("hist[%d]=%v != dif-dea=%v", i, hist[i], dif[i]-dea[i])
		}
	}
	// 上涨段末端 DIF 应在零轴上方
	if dif[59] <= 0 {
		t.Errorf("上涨趋势末端 DIF 应为正, got %v", dif[59])
	}
}

func TestRSIBasicBehavior(t *testing.T) {
	period := 6
	// 单调上涨 → RSI=100
	up := make([]float64, 20)
	for i := range up {
		up[i] = float64(i)
	}
	rsi := RSI(up, period)
	if v, ok := LastValid(rsi); !ok || v != 100 {
		t.Errorf("单调上涨 RSI 应为 100, got %v ok=%v", v, ok)
	}
	// 单调下跌 → RSI=0
	down := make([]float64, 20)
	for i := range down {
		down[i] = float64(20 - i)
	}
	if v, _ := LastValid(RSI(down, period)); v != 0 {
		t.Errorf("单调下跌 RSI 应为 0, got %v", v)
	}
	// 全平 → 无涨跌，序列全 NaN
	flat := make([]float64, 20)
	for i := range flat {
		flat[i] = 5
	}
	if _, ok := LastValid(RSI(flat, period)); ok {
		t.Errorf("全平序列 RSI 应全 NaN")
	}
	// 预热期：首个有效值在 i=period
	if !math.IsNaN(RSI(up, period)[period-1]) {
		t.Errorf("rsi[%d] 应为 NaN 预热期", period-1)
	}
	if math.IsNaN(RSI(up, period)[period]) {
		t.Errorf("rsi[%d] 应为首个有效值", period)
	}
	// 数据不足 → 全 NaN
	if _, ok := LastValid(RSI(up[:period], period)); ok {
		t.Errorf("len<=period 应全 NaN")
	}
}

func TestRSIAgainstReference(t *testing.T) {
	// 交替涨跌序列，手算参考：窗口 [i-period+1, i] 的 Cutler RSI
	close := []float64{10, 11, 10.5, 12, 11, 12.5, 11.5, 13, 12, 13.5, 12.5, 14}
	period := 6
	rsi := RSI(close, period)
	i := len(close) - 1
	var up, dn float64
	for j := i - period + 1; j <= i; j++ {
		d := close[j] - close[j-1]
		if d > 0 {
			up += d
		} else {
			dn -= d
		}
	}
	want := up / (up + dn) * 100
	if !almostEq(rsi[i], want) {
		t.Errorf("rsi[%d]=%v, 手算参考 %v", i, rsi[i], want)
	}
}
