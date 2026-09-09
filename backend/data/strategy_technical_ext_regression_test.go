package data

// strategy_technical_ext_regression_test.go
// 回归：SARTrendStrategy 在翻转恰好发生在 5 根前时曾越界 panic
// （baseByAge 仅 5 项而 extFlipAge(bull,5) 返回 [0,5]），见提交修复。
// 测试：① extFlipAge 返回域上界=窗口；② 多种回撤→反转形态下全策略不 panic。

import (
	"math"
	"testing"
)

// TestExtFlipAgeUpperBound extFlipAge 在窗口内任意偏移（含=window）都返回 age<=window。
func TestExtFlipAgeUpperBound(t *testing.T) {
	for _, window := range []int{3, 5, 8} {
		n := 30
		for flipOff := 0; flipOff <= window; flipOff++ {
			bull := make([]bool, n)
			bull[n-1-flipOff] = true // 仅在距末根 flipOff 处出现一次 false→true
			age, ok := extFlipAge(bull, window)
			if !ok {
				t.Fatalf("window=%d flipOff=%d: expected flip found", window, flipOff)
			}
			if age != flipOff {
				t.Fatalf("window=%d flipOff=%d: age=%d, want %d", window, flipOff, age, flipOff)
			}
		}
	}
}

// TestExtStrategiesNoPanicOnLateFlip 多种回撤深度×反转位置下，全部 6 个扩展策略
// 必须安全返回（回归 SAR age=5 越界）。
func TestExtStrategiesNoPanicOnLateFlip(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("strategy panicked: %v", r)
		}
	}()

	for _, dropDays := range []int{3, 5, 6, 8, 10, 12} {
		pcts := make([]float64, 0, 40)
		for i := 0; i < dropDays; i++ {
			pcts = append(pcts, -0.8)
		}
		for i := 0; i < 40-dropDays; i++ {
			pcts = append(pcts, 0.9)
		}
		ctx := extCtx(extKLines(extCloses(10, pcts), 1.5))
		for _, s := range extAllExtStrategies() {
			r := extSafeScore(t, s.Code(), s, ctx)
			if r == nil {
				t.Fatalf("%s(drop=%d) nil result", s.Code(), dropDays)
			}
			if r.Score < 0 || r.Score > 100 || math.IsNaN(r.Score) {
				t.Fatalf("%s(drop=%d) Score=%v out of [0,100]", s.Code(), dropDays, r.Score)
			}
		}
	}
}
