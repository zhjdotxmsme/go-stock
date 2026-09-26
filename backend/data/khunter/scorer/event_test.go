package scorer

import (
	"testing"

	"go-stock/backend/models"
)

func TestEventScore(t *testing.T) {
	events := []models.KhunterEvent{
		{EventType: "业绩预增", Score: 20},
		{EventType: "股东减持", Score: -30},
	}
	d := ScoreEvent(events, "测试股份")
	if d.Score != 40 || d.Veto { // 50+20-30
		t.Fatalf("expect 40, got %+v", d)
	}
}

func TestEventVeto(t *testing.T) {
	// ST 名称否决
	d := ScoreEvent(nil, "ST中安")
	if !d.Veto || d.Score != -100 {
		t.Fatalf("ST 应否决, got %+v", d)
	}
	// 大股东减持否决
	d = ScoreEvent([]models.KhunterEvent{{EventType: "大股东减持", Score: -30}}, "测试股份")
	if !d.Veto {
		t.Fatalf("大股东减持应否决, got %+v", d)
	}
	// 业绩暴雷否决
	d = ScoreEvent([]models.KhunterEvent{{EventType: "业绩暴雷", Score: -20}}, "测试股份")
	if !d.Veto {
		t.Fatalf("业绩暴雷应否决, got %+v", d)
	}
}
