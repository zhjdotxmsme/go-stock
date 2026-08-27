package models_test

import (
	"go-stock/backend/db"
	"go-stock/backend/models"
	"testing"
)

func TestSkillUsageRecordMigrate(t *testing.T) {
	if db.Dao == nil {
		t.Skip("DB not initialized")
	}
	err := db.Dao.AutoMigrate(&models.SkillUsageRecord{})
	if err != nil {
		t.Fatalf("migrate SkillUsageRecord failed: %v", err)
	}
}
