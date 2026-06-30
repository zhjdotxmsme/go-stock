package models

import (
	"go-stock/backend/db"
	"testing"
)

func TestSkillUsageRecordMigrate(t *testing.T) {
	if db.Dao == nil {
		t.Skip("DB not initialized")
	}
	err := db.Dao.AutoMigrate(&SkillUsageRecord{})
	if err != nil {
		t.Fatalf("migrate SkillUsageRecord failed: %v", err)
	}
}
