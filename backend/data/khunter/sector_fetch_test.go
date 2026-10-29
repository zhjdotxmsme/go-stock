package khunter

import (
	"strings"
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

// 回归：fetchSectorInputs 入参是内部格式（sz300780），而 all_stock_info.sec_uri_tycode
// 存 6 位裸代码——修复前按内部格式查询永远查空，板块维度恒退化 50 分。
func TestFetchSectorInputsCodeFormat(t *testing.T) {
	setupTestDB(t)
	if err := db.Dao.AutoMigrate(&models.AllStockInfo{}, &models.BKFundFlow{}); err != nil {
		t.Fatalf("migrate all_stock_info/bk_fund_flow: %v", err)
	}
	info := models.AllStockInfo{
		SECUCODE: "300780.SZ", SECURITYCODE: "300780", SECURITYNAMEABBR: "德恩精工",
		INDUSTRY: "通用设备", CONCEPT: `["工业母机","机器人概念"]`,
	}
	if err := db.Dao.Create(&info).Error; err != nil {
		t.Fatalf("seed all_stock_info: %v", err)
	}
	bk := models.BKFundFlow{Name: "通用设备", SnapTime: time.Now().Format("2006-01-02 15:04:05"), NetInflow: 2e8}
	if err := db.Dao.Create(&bk).Error; err != nil {
		t.Fatalf("seed bk_fund_flow: %v", err)
	}

	secs := fetchSectorInputs("sz300780")
	if len(secs) != 3 {
		t.Fatalf("sectors = %d 项，期望 3 项（行业+2概念，概念按 JSON 解析）", len(secs))
	}
	for _, s := range secs {
		if strings.ContainsAny(s.Name, `["`) || strings.Contains(s.Name, "]") {
			t.Fatalf("板块名含 JSON 括号/引号残留: %q", s.Name)
		}
	}
	if secs[0].Name != "通用设备" {
		t.Fatalf("sector name = %q", secs[0].Name)
	}
	if secs[0].NetInflow != 2e8 {
		t.Fatalf("net_inflow = %v，期望 2e8", secs[0].NetInflow)
	}
}
