package skill_analysis

import (
	"testing"
)

func TestGetMatchedSkillIDs(t *testing.T) {
	// Depends on DB having data - just verify it doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("GetMatchedSkillIDs panicked: %v", r)
		}
	}()
	ids := GetMatchedSkillIDs("test query")
	_ = ids
}

func TestRecordMatchNoDB(t *testing.T) {
	// RecordMatch may fail if DB not initialized, but should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RecordMatch panicked: %v", r)
		}
	}()
	_ = RecordMatch("test", "sess-1", []uint{1, 2})
}
