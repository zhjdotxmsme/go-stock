package khunter

import (
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

// 阶段 2 复盘字段必须随 EnsureMigrate 自动建列（GORM AutoMigrate 幂等）。
func TestEnsureMigrateAddsReviewColumns(t *testing.T) {
	setupTestDB(t)
	for _, col := range []string{"ReturnT1", "Return3D", "Return5D", "Reviewed"} {
		if !db.Dao.Migrator().HasColumn(&models.KhunterHunting{}, col) {
			t.Fatalf("KhunterHunting 缺列 %s", col)
		}
	}
	for _, col := range []string{"Hit", "ActualPct"} {
		if !db.Dao.Migrator().HasColumn(&models.KhunterKronosPrediction{}, col) {
			t.Fatalf("KhunterKronosPrediction 缺列 %s", col)
		}
	}
}
