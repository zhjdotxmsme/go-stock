package scorer

import "testing"

func TestSectorScore(t *testing.T) {
	// 板块A：排名前10(+50) 净流入2亿(+50) → 150；板块B：无排名 净流出2亿(-50) → 0
	// 个股取 MAX → 150
	d := ScoreSector([]SectorInput{
		{Name: "半导体", PctRank: 10, NetInflow: 2e8},
		{Name: "chip概念", PctRank: 0, NetInflow: -2e8},
	})
	if d.Score != 150 {
		t.Fatalf("expect 150, got %+v", d)
	}
	if d.Detail["best_sector"] != "半导体" {
		t.Fatalf("expect best_sector=半导体, got %+v", d)
	}
}

func TestSectorEmpty(t *testing.T) {
	d := ScoreSector(nil)
	if !d.Degraded || d.Score != 50 {
		t.Fatalf("无板块应 degraded 50, got %+v", d)
	}
	if d.Reason != "无板块归属数据" {
		t.Fatalf("expect reason 无板块归属数据, got %+v", d)
	}
}
