package strategy

import (
	"testing"

	"go-stock/backend/models"
)

// 70 根：0~20 从 20 跌到 10（深跌 50%）→ idx=32 L1=10 → idx=45 颈线 H=12 →
// idx=58 L2=10.1（间隔 26>10，价差 1%≤3%）→ 回升 → idx=66（倒数第4根，含在最后5日内）
// +8.5% 放量日收盘 12.2（≥12*1.01=12.12，前一日 11.2<12）→ 其后收盘 ≥ 12*0.98=11.76
func mkWBottomBars() []models.KLineBar {
	n := 70
	dates := genDates(n)
	o := mkConst(n, 0)
	h := mkConst(n, 0)
	l := mkConst(n, 0)
	c := mkConst(n, 0)
	v := mkVol(n, 1000)
	// idx 0~30：从 20 跌到 10.3（low 探至 10.0，保证 L1 前 30 日内有 >12 的高点）
	for i := 0; i <= 30; i++ {
		c[i] = 20 - (20-10.3)*float64(i)/30
		o[i] = c[i] + 0.1
		h[i] = c[i] + 0.3
		l[i] = c[i] - 0.3
	}
	// idx 31~44：L1=10 在 idx=32，然后反弹（过渡段高点压到 11 以下，颈线仅靠 idx=41 spike）
	for i := 31; i <= 44; i++ {
		switch {
		case i == 32:
			c[i], l[i], o[i], h[i] = 10.4, 10.0, 10.5, 10.6 // L1 低点 low=10
		case i <= 38:
			c[i] = 10.4 + (10.8-10.4)*float64(i-32)/6
			o[i], h[i], l[i] = c[i]-0.1, c[i]+0.15, c[i]-0.2
		default:
			c[i] = 10.8 - (10.8-10.5)*float64(i-38)/6
			o[i], h[i], l[i] = c[i]+0.1, c[i]+0.15, c[i]-0.2
		}
	}
	// idx 45 附近颈线最高 high=12（idx=40 已经是高点区），在 idx=41 放颈线 spike
	h[41] = 12.0
	c[41] = 11.85
	// idx 45~57：回落到 L2（单调缓降，不产生额外局部低点）
	for i := 45; i <= 57; i++ {
		c[i] = 10.5 - (10.5-10.31)*float64(i-45)/12
		o[i], h[i], l[i] = c[i]+0.1, c[i]+0.15, c[i]-0.2
	}
	// idx=58：L2=10.1
	c[58], l[58], o[58], h[58] = 10.5, 10.1, 10.6, 10.7
	// idx 59~65：缓升到 11.2
	for i := 59; i <= 65; i++ {
		c[i] = 10.5 + (11.2-10.5)*float64(i-58)/7
		o[i], h[i], l[i] = c[i]-0.05, c[i]+0.1, c[i]-0.15
	}
	// idx=66：放量确认日，前收 11.2，+8.93% → 12.2，量比 1.5
	c[66], o[66], h[66], l[66] = 12.2, 11.3, 12.3, 11.25
	v[66] = 1500
	// idx 67~69：站在颈线上方（≥11.76），保持上行（MA10>MA30）
	for i := 67; i <= 69; i++ {
		c[i] = 12.2 + 0.1*float64(i-66)
		o[i], h[i], l[i] = c[i]-0.1, c[i]+0.1, c[i]-0.2
	}
	return mkBars(dates, o, h, l, c, v)
}

func TestWBottomHit(t *testing.T) {
	bars := mkWBottomBars()
	sig := WBottom{}.Select(bars, "测试股份", bars[len(bars)-1].TradeDate)
	if sig == nil {
		t.Fatal("应命中W底")
	}
	if sig.KeyDateType != "放量确认日" {
		t.Fatalf("KeyDateType: %+v", sig)
	}
}

func TestWBottomFakeFilter(t *testing.T) {
	bars := mkWBottomBars()
	// 破坏"L1 之前 30 日内有 >L1×1.2 的高点"：把前 30 根全部压平到 10~11
	for i := 0; i <= 30; i++ {
		bars[i].Open, bars[i].Close = 10.5, 10.5
		bars[i].High, bars[i].Low = 10.9, 10.1
	}
	if sig := (WBottom{}).Select(bars, "测试股份", bars[len(bars)-1].TradeDate); sig != nil {
		t.Fatal("无前置深跌不应命中（假W底过滤）")
	}
}

func TestWBottomNecklineSpace(t *testing.T) {
	bars := mkWBottomBars()
	// 颈线空间不足：h[41] 压到 10.9（< 10×1.1=11）→ 不命中
	bars[41].High = 10.9
	bars[41].Close = 10.8
	if sig := (WBottom{}).Select(bars, "测试股份", bars[len(bars)-1].TradeDate); sig != nil {
		t.Fatal("颈线空间不足不应命中")
	}
}
