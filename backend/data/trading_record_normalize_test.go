package data

import (
	"path/filepath"
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

// TestCanonicalizeTradingRecordStock 交易日志代码/名称应规范成与"搜索股票"（全量股票库）一致。
func TestCanonicalizeTradingRecordStock(t *testing.T) {
	db.Init(filepath.Join(t.TempDir(), "test.db"))
	t.Cleanup(func() {
		if sqlDB, err := db.Dao.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.Dao.AutoMigrate(&models.AllStockInfo{}); err != nil {
		t.Fatalf("AutoMigrate error = %v", err)
	}
	if err := db.Dao.Create(&models.AllStockInfo{
		SECUCODE:         "600519.SH",
		SECURITYCODE:     "600519",
		SECURITYNAMEABBR: "贵州茅台",
	}).Error; err != nil {
		t.Fatalf("seed error = %v", err)
	}

	api := StockDataApi{}

	// 裸代码 + 手动乱填的名称：都以库为准
	rec := &TradingRecord{StockCode: "600519", StockName: "手动随便填"}
	api.canonicalizeTradingRecordStock(rec)
	if rec.StockCode != "600519.SH" || rec.StockName != "贵州茅台" {
		t.Fatalf("bare code = %+v", rec)
	}

	// 带市场前缀
	rec = &TradingRecord{StockCode: "sh600519"}
	api.canonicalizeTradingRecordStock(rec)
	if rec.StockCode != "600519.SH" || rec.StockName != "贵州茅台" {
		t.Fatalf("prefixed code = %+v", rec)
	}

	// 已是 SECUCODE
	rec = &TradingRecord{StockCode: "600519.SH", StockName: "贵州茅台"}
	api.canonicalizeTradingRecordStock(rec)
	if rec.StockCode != "600519.SH" || rec.StockName != "贵州茅台" {
		t.Fatalf("secucode = %+v", rec)
	}

	// 库里查不到（如港股）且名称也对不上：保持原样不阻塞
	rec = &TradingRecord{StockCode: "hk00700", StockName: "腾讯控股"}
	api.canonicalizeTradingRecordStock(rec)
	if rec.StockCode != "hk00700" || rec.StockName != "腾讯控股" {
		t.Fatalf("missing in lib should keep = %+v", rec)
	}

	// 代码查不到但名称精确命中：反查补全代码
	rec = &TradingRecord{StockCode: "999999", StockName: "贵州茅台"}
	api.canonicalizeTradingRecordStock(rec)
	if rec.StockCode != "600519.SH" || rec.StockName != "贵州茅台" {
		t.Fatalf("name fallback = %+v", rec)
	}

	// 空代码：直接跳过
	rec = &TradingRecord{StockCode: "", StockName: "贵州茅台"}
	api.canonicalizeTradingRecordStock(rec)
	if rec.StockCode != "" || rec.StockName != "贵州茅台" {
		t.Fatalf("empty code = %+v", rec)
	}
}
