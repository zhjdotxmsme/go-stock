package strategy

import (
	"testing"

	"go-stock/backend/models"
)

// 130 根：从 30 跌到 ~16（>45% 深跌，先高后低），锚点（倒数第4根）放量长阳 +8.5%，
// 锚点前 20 日分两半：后半段 low 更低但下跌放缓（MACD 柱抬高 → 底背离）
func mkBottomInflectionBars() []models.KLineBar {
	n := 130
	dates := genDates(n)
	o := mkConst(n, 0)
	h := mkConst(n, 0)
	l := mkConst(n, 0)
	c := mkConst(n, 0)
	v := mkVol(n, 1000)
	// idx 0~85：从 30 匀速跌到 20（窗口最高在头部）
	for i := 0; i <= 85; i++ {
		c[i] = 30 - (30-20)*float64(i)/85
		o[i] = c[i] + 0.15
		h[i] = c[i] + 0.35
		l[i] = c[i] - 0.35
	}
	// idx 86~105（锚点前 20 日的前半段，106~115 正式计入）：先陡跌
	// 简化：86~115 线性从 20 → 16.3（后半段放缓），116~125 在 16.05~16.8 间走平略降
	for i := 86; i <= 115; i++ {
		c[i] = 20 - (20-16.4)*float64(i-86)/29
		o[i] = c[i] + 0.12
		h[i] = c[i] + 0.3
		l[i] = c[i] - 0.3
	}
	for i := 116; i <= 125; i++ {
		c[i] = 16.4 - 0.03*float64(i-116) // 极缓下跌 → MACD 柱抬高
		o[i] = c[i] + 0.08
		h[i] = c[i] + 0.2
		l[i] = c[i] - 0.25
	}
	l[124] = 16.05 // 后半段最低价（< 前半段最低 ~16.1），价格创新低
	l[114] = 16.10 // 前半段最低价
	// 锚点 idx=126（倒数第4根，off=3）：+8.5% 放量长阳
	anchor := 126
	prev := c[anchor-1]
	o[anchor] = prev * 1.01
	c[anchor] = prev * 1.085
	h[anchor] = c[anchor] * 1.01
	l[anchor] = prev * 0.99
	v[anchor] = 3000 // 量比(前10日均量) 3 ≥ 2.5
	// 锚点 close ≈ 17.3，距 120 日最低 16.05：(17.3-16.05)/16.05 ≈ 7.8% ≤ 15% ✓
	// idx 127~129：回调不破锚点开盘价
	for i := 127; i <= 129; i++ {
		c[i] = c[anchor] - 0.15*float64(i-anchor)
		o[i] = c[i] + 0.1
		h[i] = c[i] + 0.15
		l[i] = c[i] - 0.2
		if l[i] < o[anchor] { // 收盘不破开盘价即可，low 允许下探
			l[i] = o[anchor] - 0.3
		}
	}
	return mkBars(dates, o, h, l, c, v)
}

func TestBottomInflectionHit(t *testing.T) {
	bars := mkBottomInflectionBars()
	sig := BottomInflection{}.Select(bars, "测试股份", bars[len(bars)-1].TradeDate)
	if sig == nil {
		t.Fatal("应命中底部趋势拐点")
	}
	if sig.KeyDateType != "放量长阳日" {
		t.Fatalf("KeyDateType: %+v", sig)
	}
}

func TestBottomInflectionNoDeepDecline(t *testing.T) {
	bars := mkBottomInflectionBars()
	// 把前 86 根压成 15~17 的浅跌（< 45%）→ 不命中
	for i := 0; i <= 85; i++ {
		bars[i].Close = 17 - 1.0*float64(i)/85
		bars[i].Open = bars[i].Close + 0.1
		bars[i].High = bars[i].Close + 0.3
		bars[i].Low = bars[i].Close - 0.3
	}
	if sig := (BottomInflection{}).Select(bars, "测试股份", bars[len(bars)-1].TradeDate); sig != nil {
		t.Fatal("深跌不足不应命中")
	}
}

func TestBottomInflectionAnchorBroken(t *testing.T) {
	bars := mkBottomInflectionBars()
	// 锚点之后某日收盘跌破锚点开盘价 → 不命中
	bars[128].Close = bars[126].Open * 0.97
	if sig := (BottomInflection{}).Select(bars, "测试股份", bars[len(bars)-1].TradeDate); sig != nil {
		t.Fatal("回调破锚点开盘价不应命中")
	}
}
