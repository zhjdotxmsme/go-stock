package risk

import "testing"

func TestKellyFraction(t *testing.T) {
	// p=0.4, b=1.5: f*=(0.4*1.5-0.6)/1.5=0 → 0
	if f := KellyFraction(0.4, 1.5); f != 0 {
		t.Fatalf("f*=0 期望 0, got %v", f)
	}
	// p=0.5, b=2: f*=(1-0.5)/2=0.25 → 半凯利 0.125
	if f := KellyFraction(0.5, 2); f < 0.124 || f > 0.126 {
		t.Fatalf("expect ~0.125, got %v", f)
	}
	// p=0.9, b=5: f*=0.88 → 半凯利 0.44 → clamp 0.15
	if f := KellyFraction(0.9, 5); f != 0.15 {
		t.Fatalf("expect clamp 0.15, got %v", f)
	}
	// 非法输入
	if f := KellyFraction(0.5, 0); f != 0 {
		t.Fatalf("b=0 应返回 0, got %v", f)
	}
}

func TestKellyShares(t *testing.T) {
	// 30万 × 0.125 = 37500，价 17 元 → 2205 股 → 取整 2200
	if s := KellyShares(300000, 17, 0.125); s != 2200 {
		t.Fatalf("expect 2200, got %v", s)
	}
	// 金额不足一手 → 0
	if s := KellyShares(300000, 1700, 0.001); s != 0 {
		t.Fatalf("expect 0, got %v", s)
	}
}
