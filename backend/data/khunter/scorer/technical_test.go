package scorer

import "testing"

func TestTechnicalScoreSum(t *testing.T) {
	s := TechnicalScorer{}
	d := s.Score([]Hit{{Name: "仙人指路", Weight: 70}, {Name: "W底", Weight: 50}})
	if d.Score != 120 || d.Veto {
		t.Fatalf("expect 120 无否决, got %+v", d)
	}
	// 空命中
	d = s.Score(nil)
	if d.Score != 0 {
		t.Fatalf("expect 0, got %+v", d)
	}
}

func TestTechnicalOverride(t *testing.T) {
	s := TechnicalScorer{Overrides: map[string]int{"W底": 30}}
	d := s.Score([]Hit{{Name: "W底", Weight: 50}})
	if d.Score != 30 {
		t.Fatalf("override 应生效为 30, got %+v", d)
	}
}

func TestTechnicalVeto(t *testing.T) {
	s := TechnicalScorer{}
	d := s.Score([]Hit{{Name: "M头策略", Weight: -80}, {Name: "多死叉共振策略", Weight: -50}})
	if !d.Veto || d.Score != -100 {
		t.Fatalf("M头+多死叉应否决为 -100, got %+v", d)
	}
	// 单独命中 M头 不否决
	d = s.Score([]Hit{{Name: "M头策略", Weight: -80}})
	if d.Veto || d.Score != -80 {
		t.Fatalf("单独M头应为 -80 不否决, got %+v", d)
	}
}
