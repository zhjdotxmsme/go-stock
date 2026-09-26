package strategy

import (
	"testing"

	"go-stock/backend/models"
)

// 25 根：前面平稳，idx=18 涨停(+10%, 3倍量)，idx=19~23 横盘缩量，今日(idx=24) 涨 3% 放量
func mkPullbackBars() []models.KLineBar {
	n := 25
	dates := genDates(n)
	o := mkConst(n, 10)
	h := mkConst(n, 10.1)
	l := mkConst(n, 9.9)
	c := mkConst(n, 10)
	v := mkVol(n, 1000)
	lu := 18
	o[lu], c[lu] = 10, 11.0 // +10%
	h[lu], l[lu] = 11.0, 10.0
	v[lu] = 3000 // 量比 3 ≥ 2.2
	// 回调期 19~23：收盘 ∈ [10.45, 11.55]，有收盘 < 11.0，有缩量日
	pullCloses := []float64{10.8, 10.6, 10.7, 10.55, 10.6}
	for k, pc := range pullCloses {
		i := lu + 1 + k
		o[i], c[i] = pc, pc
		h[i], l[i] = pc+0.05, pc-0.05
		v[i] = 1200
	}
	v[lu+2] = 1400 // ≤ 3000*0.5 缩量日
	// 今日：+3%（10.6→10.918），量 ≥ 昨日×1.5，close > MA5
	t := n - 1
	o[t] = 10.7
	c[t] = c[t-1] * 1.03
	h[t], l[t] = c[t]+0.05, o[t]-0.05
	v[t] = 2500
	return mkBars(dates, o, h, l, c, v)
}

func TestLimitUpPullbackHit(t *testing.T) {
	bars := mkPullbackBars()
	sig := LimitUpPullback{}.Select(bars, "测试股份", bars[len(bars)-1].TradeDate)
	if sig == nil {
		t.Fatal("应命中涨停回马枪")
	}
	if sig.KeyDateType != "涨停日" {
		t.Fatalf("KeyDateType 应为涨停日: %+v", sig)
	}
}

func TestLimitUpPullbackNoShrink(t *testing.T) {
	bars := mkPullbackBars()
	// 回调期全部不缩量（1600 > 3000*0.5=1500）→ 不命中
	for i := 19; i <= 23; i++ {
		bars[i].Volume = 1600
	}
	if sig := (LimitUpPullback{}).Select(bars, "测试股份", bars[len(bars)-1].TradeDate); sig != nil {
		t.Fatal("无缩量不应命中")
	}
}

func TestLimitUpPullbackTodayWeak(t *testing.T) {
	bars := mkPullbackBars()
	// 今日涨幅压到 1% → 不命中
	n := len(bars)
	bars[n-1].Close = bars[n-2].Close * 1.01
	if sig := (LimitUpPullback{}).Select(bars, "测试股份", bars[n-1].TradeDate); sig != nil {
		t.Fatal("今日涨幅不足不应命中")
	}
}
