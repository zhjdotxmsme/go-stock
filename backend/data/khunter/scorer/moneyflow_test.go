package scorer

import (
	"testing"

	"go-stock/backend/models"
)

func mfFlow(mainNet, lgRatio, smRatio float64) models.KhunterMoneyFlowDaily {
	return models.KhunterMoneyFlowDaily{MainNet: mainNet, LgNetRatio: lgRatio, SmNetRatio: smRatio}
}

func TestMoneyFlowStrong(t *testing.T) {
	// 5日主力净额 2 亿 → 主力分 100；大单占比每日 6% → 5×40=200
	flows := make([]models.KhunterMoneyFlowDaily, 5)
	for i := range flows {
		flows[i] = mfFlow(4e7, 6, -2) // 大单>0 小单<0 → 方向 100（吸筹）
	}
	d := ScoreMoneyFlow(flows)
	// total = 100*0.55 + 200*0.10 + 50*0.10 + 100*0.25 = 55+20+5+25 = 105 → clamp 100
	if d.Veto || d.Score != 100 {
		t.Fatalf("expect 100 无否决, got %+v", d)
	}
}

func TestMoneyFlowVetoMainOutflow(t *testing.T) {
	// 5日主力净额 < -1亿 且大单占比均值 < -5% → 否决
	flows := make([]models.KhunterMoneyFlowDaily, 5)
	for i := range flows {
		flows[i] = mfFlow(-3e7, -6, 2)
	}
	d := ScoreMoneyFlow(flows)
	if !d.Veto || d.Score != -100 {
		t.Fatalf("应否决 -100, got %+v", d)
	}
}

func TestMoneyFlowVetoDistribution(t *testing.T) {
	// 出货信号：大单占比均值 < -1% 且小单占比均值 > +1%（主力净额未达 -1亿 阈值）
	flows := make([]models.KhunterMoneyFlowDaily, 5)
	for i := range flows {
		flows[i] = mfFlow(-1e7, -2, 2)
	}
	d := ScoreMoneyFlow(flows)
	if !d.Veto {
		t.Fatalf("出货信号应否决, got %+v", d)
	}
}

func TestMoneyFlowInsufficientData(t *testing.T) {
	d := ScoreMoneyFlow([]models.KhunterMoneyFlowDaily{mfFlow(1e7, 2, -1)})
	if !d.Degraded || d.Score != 30 {
		t.Fatalf("数据不足应 degraded 中性 30, got %+v", d)
	}
}
