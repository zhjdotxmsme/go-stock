package khunter

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

// setupTestDB 用临时文件库替换全局 db.Dao，测试后恢复
func setupTestDB(t *testing.T) {
	t.Helper()
	old := db.Dao
	d, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	db.Dao = d
	t.Cleanup(func() {
		if sqlDB, err := d.DB(); err == nil {
			sqlDB.Close()
		}
		db.Dao = old
	})
	if err := EnsureMigrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
}

func TestRepoSignalAndScoreRoundTrip(t *testing.T) {
	setupTestDB(t)
	r := NewRepo()

	sig := models.KhunterSignal{Code: "600519", Name: "贵州茅台", Strategy: "仙人指路",
		SignalDate: "2026-09-24", Close: 1700.5, VolumeRatio: 2.1, Reasons: `["冲高8%","反包确认"]`}
	if err := r.SaveSignals([]models.KhunterSignal{sig}); err != nil {
		t.Fatalf("SaveSignals: %v", err)
	}
	sigs, err := r.GetSignalsByDate("2026-09-24")
	if err != nil || len(sigs) != 1 || sigs[0].Strategy != "仙人指路" {
		t.Fatalf("GetSignalsByDate: %v %+v", err, sigs)
	}

	sc := models.KhunterScore{Code: "600519", ScoreDate: "2026-09-24", Technical: 70, Moneyflow: 40,
		Fundamental: 60, Sector: 50, Event: 50, Total: 52.5, Level: "中性"}
	if err := r.SaveScores([]models.KhunterScore{sc}); err != nil {
		t.Fatalf("SaveScores: %v", err)
	}
	scores, err := r.GetScoresByDate("2026-09-24")
	if err != nil || len(scores) != 1 || scores[0].Total != 52.5 {
		t.Fatalf("GetScoresByDate: %v %+v", err, scores)
	}
}

func TestRepoMoneyFlowUpsert(t *testing.T) {
	setupTestDB(t)
	r := NewRepo()
	rows := []models.KhunterMoneyFlowDaily{
		{Code: "600519", Date: "2026-09-23", MainNet: 1.2e8, LgNetRatio: 3.5},
		{Code: "600519", Date: "2026-09-23", MainNet: 2.0e8, LgNetRatio: 5.0}, // 重复键 → 覆盖
	}
	if err := r.UpsertMoneyFlow(rows); err != nil {
		t.Fatalf("UpsertMoneyFlow: %v", err)
	}
	got, err := r.GetMoneyFlow("600519", 5)
	if err != nil || len(got) != 1 || got[0].MainNet != 2.0e8 {
		t.Fatalf("GetMoneyFlow: %v %+v", err, got)
	}
}
