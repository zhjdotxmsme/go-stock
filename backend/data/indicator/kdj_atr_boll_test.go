package indicator

import (
	"math"
	"testing"
)

// TestKDJReference 手算参考：n=2, m1=m2=3，K/D 初值 50 的 SMA(X,N,1) 递推。
func TestKDJReference(t *testing.T) {
	high := []float64{10, 12}
	low := []float64{8, 9}
	close := []float64{9, 11}
	k, d, j := KDJ(high, low, close, 2, 3, 3)

	// i=0: 窗口[0,0]，rsv=(9-8)/(10-8)*100=50，K0=D0=J0=50
	if !almostEq(k[0], 50) || !almostEq(d[0], 50) || !almostEq(j[0], 50) {
		t.Fatalf("i=0 应为 50/50/50, 得 %v/%v/%v", k[0], d[0], j[0])
	}
	// i=1: 窗口[0,1]，hh=12,ll=8,rsv=(11-8)/4*100=75
	// K1=(50*2+75)/3=58.333..., D1=(50*2+K1)/3=52.777..., J1=3K1-2D1
	k1 := (50.0*2 + 75.0) / 3
	d1 := (50.0*2 + k1) / 3
	j1 := 3*k1 - 2*d1
	if !almostEq(k[1], k1) || !almostEq(d[1], d1) || !almostEq(j[1], j1) {
		t.Fatalf("i=1 应为 %v/%v/%v, 得 %v/%v/%v", k1, d1, j1, k[1], d[1], j[1])
	}
}

// TestKDJFlatBar 一字板（hh==ll）时 RSV=50，K/D/J 保持。
func TestKDJFlatBar(t *testing.T) {
	flat := []float64{5, 5, 5}
	k, d, j := KDJ(flat, flat, flat, 9, 3, 3)
	for i := range flat {
		if !almostEq(k[i], 50) || !almostEq(d[i], 50) || !almostEq(j[i], 50) {
			t.Fatalf("一字板 i=%d 应为 50/50/50, 得 %v/%v/%v", i, k[i], d[i], j[i])
		}
	}
}

// TestKDJConvergesToRSV 持续上涨时 RSV 恒为 100，K/D 应单调收敛到 100。
func TestKDJConvergesToRSV(t *testing.T) {
	n := 30
	high := make([]float64, n)
	low := make([]float64, n)
	close := make([]float64, n)
	for i := range n {
		low[i] = float64(i)
		high[i] = float64(i) + 1
		close[i] = float64(i) + 1 // 收在最高 → RSV=100
	}
	k, d, _ := KDJ(high, low, close, 9, 3, 3)
	for i := 1; i < n; i++ {
		if k[i] <= k[i-1] || d[i] <= d[i-1] {
			t.Fatalf("RSV 恒 100 时 K/D 应单调递增: i=%d K=%v->%v D=%v->%v", i, k[i-1], k[i], d[i-1], d[i])
		}
	}
	if k[n-1] < 99 || d[n-1] < 95 {
		t.Fatalf("30 根后 K/D 应已收敛到 100 附近: K=%v D=%v", k[n-1], d[n-1])
	}
}

// TestATRWilder 手算 Wilder 递推：种子=前 period 根 TR 均值，之后 (prev*(N-1)+tr)/N。
func TestATRWilder(t *testing.T) {
	high := []float64{10, 12, 11, 15}
	low := []float64{8, 10, 9, 12}
	close := []float64{9, 11, 10, 14}
	// tr = [2, 3, 2, 5]；atr[1]=(2+3)/2=2.5；atr[2]=(2.5+2)/2=2.25；atr[3]=(2.25+5)/2=3.625
	atr := ATR(high, low, close, 2)
	if !math.IsNaN(atr[0]) {
		t.Fatalf("预热期 atr[0] 应为 NaN, 得 %v", atr[0])
	}
	want := []float64{2.5, 2.25, 3.625}
	for i, w := range want {
		if !almostEq(atr[i+1], w) {
			t.Fatalf("atr[%d] 应为 %v, 得 %v", i+1, w, atr[i+1])
		}
	}
}

// TestBOLLReference 手算参考：period=3 的 SMA + 总体标准差（除以 N）。
func TestBOLLReference(t *testing.T) {
	close := []float64{1, 2, 3, 4, 5}
	mid, up, low := BOLL(close, 3, 2)
	if !math.IsNaN(mid[0]) || !math.IsNaN(mid[1]) {
		t.Fatalf("预热期应为 NaN")
	}
	// i=2: mid=2, var=((1-2)²+0+(3-2)²)/3=2/3
	std := math.Sqrt(2.0 / 3.0)
	if !almostEq(mid[2], 2) || !almostEq(up[2], 2+2*std) || !almostEq(low[2], 2-2*std) {
		t.Fatalf("i=2 应为 2/%v/%v, 得 %v/%v/%v", 2+2*std, 2-2*std, mid[2], up[2], low[2])
	}
	// i=4: mid=4，同 std
	if !almostEq(mid[4], 4) || !almostEq(up[4], 4+2*std) || !almostEq(low[4], 4-2*std) {
		t.Fatalf("i=4 应为 4/%v/%v, 得 %v/%v/%v", 4+2*std, 4-2*std, mid[4], up[4], low[4])
	}
}
