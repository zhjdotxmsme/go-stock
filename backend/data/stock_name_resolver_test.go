package data

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

func setupResolverTestDB(t *testing.T) {
	t.Helper()
	old := db.Dao
	d, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	db.Dao = d
	t.Cleanup(func() {
		if sqlDB, err := d.DB(); err == nil {
			_ = sqlDB.Close()
		}
		db.Dao = old
		stockNameCache = sync.Map{}
	})
	_ = db.Dao.AutoMigrate(&models.AllStockInfo{}, &StockBasic{})
}

func TestResolveStockName_FromAllStockInfo(t *testing.T) {
	setupResolverTestDB(t)
	_ = db.Dao.Create(&models.AllStockInfo{
		SECUCODE:         "600519.SH",
		SECURITYCODE:     "sh600519",
		SECURITYNAMEABBR: "贵州茅台",
	}).Error
	if got := ResolveStockName("600519.SH"); got != "贵州茅台" {
		t.Fatalf("600519.SH: got %q", got)
	}
	if got := ResolveStockName("sh600519"); got != "贵州茅台" {
		t.Fatalf("sh600519: got %q", got)
	}
}

func TestResolveStockNames_Batch(t *testing.T) {
	setupResolverTestDB(t)
	_ = db.Dao.Create(&models.AllStockInfo{SECUCODE: "000001.SZ", SECURITYCODE: "sz000001", SECURITYNAMEABBR: "平安银行"}).Error
	_ = db.Dao.Create(&models.AllStockInfo{SECUCODE: "600519.SH", SECURITYCODE: "sh600519", SECURITYNAMEABBR: "贵州茅台"}).Error
	m := ResolveStockNames([]string{"sz000001", "600519.SH"})
	if m["sz000001"] != "平安银行" || m["sh600519"] != "贵州茅台" {
		t.Fatalf("batch: %+v", m)
	}
}
