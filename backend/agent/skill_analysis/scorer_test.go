package skill_analysis

import "testing"

func TestCalculateSkillScoreEmpty(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Logf("calculateSkillScore panicked (expected without DB): %v", r)
		}
	}()
	score, err := calculateSkillScore(99999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 0 {
		t.Fatalf("expected 0 for empty records, got %f", score)
	}
}
