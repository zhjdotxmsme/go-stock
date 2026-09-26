package strategy

import (
	"testing"

	"go-stock/backend/models"
)

// 110 根：0~44 平盘 10 元 → 45~59 拉升（L=10 在 idx 45 前 45 日内）→ idx=60 H=14（+40%）
// → 61~79 回撤到 P=11.2（-20%，≥18% 且 < 40%-8%）→ idx=88 R=12.5（swing high < H）
// → 其后高点单调不增 → 今日 idx=109 阳线实体 5.8% 贴紧突破 R（+1.6%≤2%）放量 2 倍
func mkUptrendDipBars() []models.KLineBar {
	n := 110
	dates := genDates(n)
	o := mkConst(n, 10)
	h := mkConst(n, 10.15)
	l := mkConst(n, 9.9)
	c := mkConst(n, 10)
	v := mkVol(n, 1000)
	// idx 45~60：拉升到 14
	for i := 45; i <= 60; i++ {
		c[i] = 10 + (14-10)*float64(i-45)/15
		o[i] = c[i] - 0.1
		h[i] = c[i] + 0.12
		l[i] = c[i] - 0.15
	}
	l[45] = 10.0 // L=10（45 日窗口最低）
	// idx 61~79：回撤到 11.2
	for i := 61; i <= 79; i++ {
		c[i] = 13.9 - (13.9-11.2)*float64(i-61)/18
		o[i] = c[i] + 0.1
		h[i] = c[i] + 0.12
		l[i] = c[i] - 0.12
	}
	l[79] = 11.2 // P=11.2
	// idx 80~95：反弹到 R=12.5（idx 88），随后高点递减
	for i := 80; i <= 88; i++ {
		c[i] = 11.3 + (12.4-11.3)*float64(i-80)/8
		o[i] = c[i] - 0.08
		h[i] = c[i] + 0.1
		l[i] = c[i] - 0.12
	}
	h[88] = 12.5 // R
	c[88] = 12.4
	for i := 89; i <= 108; i++ {
		c[i] = 12.3 - 0.015*float64(i-89)
		o[i] = c[i] + 0.05
		h[i] = c[i] + 0.08 // 高点递减，不产生新 swing high
		l[i] = c[i] - 0.1
	}
	// 今日 idx=109：开 12.0 收 12.7（实体 5.8%），突破 R=12.5（+1.6%），量 2 倍
	t := n - 1
	o[t], c[t] = 12.0, 12.7
	h[t], l[t] = 12.75, 11.95
	v[t] = 2000
	return mkBars(dates, o, h, l, c, v)
}

func TestUptrendDipBuyHit(t *testing.T) {
	bars := mkUptrendDipBars()
	sig := UptrendDipBuy{}.Select(bars, "测试股份", bars[len(bars)-1].TradeDate)
	if sig == nil {
		t.Fatal("应命中主升低吸")
	}
	if sig.KeyDateType != "一拉高点" {
		t.Fatalf("KeyDateType: %+v", sig)
	}
}

func TestUptrendDipBuyChaseHigh(t *testing.T) {
	bars := mkUptrendDipBars()
	// 今日收盘拉到 13.0（突破 R 4% > 2%，追高）→ 不命中
	n := len(bars)
	bars[n-1].Close = 13.0
	bars[n-1].High = 13.05
	if sig := (UptrendDipBuy{}).Select(bars, "测试股份", bars[n-1].TradeDate); sig != nil {
		t.Fatal("贴紧突破幅度超限不应命中")
	}
}

func TestUptrendDipBuyPullbackTooDeep(t *testing.T) {
	bars := mkUptrendDipBars()
	// 二调跌破起涨点 L（l[45]=10.0 区域的前低 9.9）→ 一拉被全额回吐，不命中
	bars[70].Low = 9.85
	if sig := (UptrendDipBuy{}).Select(bars, "测试股份", bars[len(bars)-1].TradeDate); sig != nil {
		t.Fatal("回撤跌破起涨点不应命中")
	}
}
