package strategy

import (
	"testing"

	"go-stock/backend/models"
)

// 构造 40 根正序 K 线：缓升趋势 + 倒数第 2 根（信号日）冲高回落长上影 + 今日反包
func mkImmortalBars() []models.KLineBar {
	n := 40
	dates := genDates(n)
	o := mkConst(n, 0)
	h := mkConst(n, 0)
	l := mkConst(n, 0)
	c := mkConst(n, 0)
	v := mkVol(n, 1000)
	// 前 38 根缓升（保证 MA 多头 + R² 高）
	for i := 0; i < n-2; i++ {
		c[i] = 10 + 0.1*float64(i)
		o[i] = c[i] - 0.05
		h[i] = c[i] + 0.06
		l[i] = c[i] - 0.08
	}
	// 信号日 i = n-2：昨收 prev = c[n-3]，冲高 10%，收阳 +2%，上影 8%
	sig := n - 2
	prev := c[sig-1]
	o[sig] = prev
	c[sig] = prev * 1.02
	h[sig] = prev * 1.10 // surge = 10% ≥ 8%；us/h = 0.08/1.10 ≈ 7.3% ≥ 4%
	l[sig] = prev * 0.99
	v[sig] = 3000        // 量比 3 ≥ 1.5
	// 今日 i = n-1：反包（close ≥ (c[sig]+h[sig])/2 = prev*1.06），且 close ≥ MA5
	t := n - 1
	c[t] = prev * 1.07
	o[t] = prev * 1.05
	h[t] = prev * 1.08
	l[t] = prev * 1.04
	return mkBars(dates, o, h, l, c, v)
}

func TestImmortalGuidanceHit(t *testing.T) {
	bars := mkImmortalBars()
	sig := ImmortalGuidance{}.Select(bars, "测试股份", bars[len(bars)-1].TradeDate)
	if sig == nil {
		t.Fatal("应命中仙人指路")
	}
	if sig.StrategyName != "仙人指路" || sig.Close <= 0 || sig.VolumeRatio <= 0 {
		t.Fatalf("信号字段不完整: %+v", sig)
	}
}

func TestImmortalGuidanceNoSurge(t *testing.T) {
	bars := mkImmortalBars()
	// 把信号日冲高压到 5%（< 8%），不应命中
	n := len(bars)
	bars[n-2].High = bars[n-3].Close * 1.05
	if sig := (ImmortalGuidance{}).Select(bars, "测试股份", bars[n-1].TradeDate); sig != nil {
		t.Fatalf("冲高不足不应命中: %+v", sig)
	}
}

func TestImmortalGuidanceSuspended(t *testing.T) {
	bars := mkImmortalBars()
	// 选股日比最后一根晚一天 → 停牌过滤
	if sig := (ImmortalGuidance{}).Select(bars, "测试股份", "2999-01-01"); sig != nil {
		t.Fatal("停牌股不应命中")
	}
}

func TestImmortalGuidanceShortData(t *testing.T) {
	bars := mkImmortalBars()[:20] // < MinBars 30
	if sig := (ImmortalGuidance{}).Select(bars, "测试股份", ""); sig != nil {
		t.Fatal("数据不足不应命中")
	}
}
