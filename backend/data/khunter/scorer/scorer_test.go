package scorer

import (
	"testing"

	"go-stock/backend/models"
)

func TestScoreWeighted(t *testing.T) {
	in := FiveDimInput{
		Hits:      []Hit{{Name: "仙人指路", Weight: 70}},       // 技术 70
		MoneyFlow: []models.KhunterMoneyFlowDaily{},            // 不足5天 → degraded 30
		Finance:   FundamentalInput{Available: false},          // 50
		Sectors:   nil,                                          // 50
		Events:    nil,                                          // 50
		StockName: "测试股份",
	}
	r := Score(in)
	// total = 70*.35 + 30*.35 + 50*.1*3 = 24.5+10.5+15 = 50
	if r.Total != 50 || r.Level != "中性" || !r.Degraded {
		t.Fatalf("expect total=50 中性 degraded, got %+v", r)
	}
}

func TestScoreVetoDominates(t *testing.T) {
	in := FiveDimInput{
		Hits:      []Hit{{Name: "W底", Weight: 50}},
		Finance:   FundamentalInput{Available: true, HasYoy: true, NetProfitYoy: -60}, // 否决
		StockName: "测试股份",
	}
	r := Score(in)
	if r.Total != -100 || r.Level != "淘汰" || r.VetoReason == "" {
		t.Fatalf("否决应直接 -100 淘汰, got %+v", r)
	}
}

func TestLevelOf(t *testing.T) {
	cases := map[float64]string{85: "强烈推荐", 60: "推荐", 40: "中性", 20: "谨慎", 10: "回避", -100: "淘汰"}
	for score, want := range cases {
		if got := LevelOf(score); got != want {
			t.Fatalf("LevelOf(%v)=%s, want %s", score, got, want)
		}
	}
}
