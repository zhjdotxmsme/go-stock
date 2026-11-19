package khunter

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
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

func TestReplaceRunResultsDeduplicatesReruns(t *testing.T) {
	setupTestDB(t)
	r := NewRepo()
	sig := models.KhunterSignal{Code: "600519", Name: "贵州茅台", Strategy: "仙人指路",
		SignalDate: "2026-09-24", Close: 1700.5, VolumeRatio: 2.1, Reasons: `["冲高8%","反包确认"]`}
	if err := r.ReplaceSignalsForDate("2026-09-24", []models.KhunterSignal{sig}); err != nil {
		t.Fatal(err)
	}
	if err := r.ReplaceSignalsForDate("2026-09-24", []models.KhunterSignal{sig}); err != nil {
		t.Fatal(err)
	}
	sigs, err := r.GetSignalsByDate("2026-09-24")
	if err != nil {
		t.Fatal(err)
	}
	if len(sigs) != 1 {
		t.Fatalf("同日重跑应替换而非追加，got %d", len(sigs))
	}

	if err := r.ReplaceScoresForDate("2026-09-24", nil); err != nil {
		t.Fatal(err)
	}
	scores, err := r.GetScoresByDate("2026-09-24")
	if err != nil {
		t.Fatal(err)
	}
	if len(scores) != 0 {
		t.Fatalf("无候选重跑应清空当日评分，got %d", len(scores))
	}
}

func TestRunSummaryUpsertsByTradeDate(t *testing.T) {
	setupTestDB(t)
	r := NewRepo()
	row := &models.KhunterRunSummary{TradeDate: "2026-11-11", StockTotal: 10, FreshStocks: 9, Status: "success"}
	if err := r.UpsertRunSummary(row); err != nil {
		t.Fatal(err)
	}
	row = &models.KhunterRunSummary{TradeDate: "2026-11-11", StockTotal: 10, FreshStocks: 8,
		SignalCount: 1, ScoredCount: 1, HuntedCount: 1, Status: "success", DurationMs: 120}
	if err := r.UpsertRunSummary(row); err != nil {
		t.Fatal(err)
	}
	rows, err := r.GetRunSummaries(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].FreshStocks != 8 || rows[0].SignalCount != 1 || rows[0].DurationMs != 120 {
		t.Fatalf("汇总应按交易日覆盖更新, got %+v", rows)
	}
}

func TestRepoSignalAndScoreRoundTrip(t *testing.T) {
	setupTestDB(t)
	r := NewRepo()

	sig := models.KhunterSignal{Code: "600519", Name: "贵州茅台", Strategy: "仙人指路",
		SignalDate: "2026-09-24", Close: 1700.5, VolumeRatio: 2.1, Reasons: `["冲高8%","反包确认"]`}
	if err := r.ReplaceSignalsForDate("2026-09-24", []models.KhunterSignal{sig}); err != nil {
		t.Fatalf("ReplaceSignalsForDate: %v", err)
	}
	sigs, err := r.GetSignalsByDate("2026-09-24")
	if err != nil || len(sigs) != 1 || sigs[0].Strategy != "仙人指路" {
		t.Fatalf("GetSignalsByDate: %v %+v", err, sigs)
	}

	sc := models.KhunterScore{Code: "600519", ScoreDate: "2026-09-24", Technical: 70, Moneyflow: 40,
		Fundamental: 60, Sector: 50, Event: 50, Total: 52.5, Level: "中性"}
	if err := r.ReplaceScoresForDate("2026-09-24", []models.KhunterScore{sc}); err != nil {
		t.Fatalf("ReplaceScoresForDate: %v", err)
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

func TestRepoHuntingTracking(t *testing.T) {
	setupTestDB(t)
	r := NewRepo()

	h := models.KhunterHunting{Code: "600519", Name: "贵州茅台", SignalDate: "2026-09-23",
		EnterDate: "2026-09-24", EnterScore: 70, SupportPrice: 1650,
		EntryPrice: 1660, InitialStopPrice: 1500, ProtectPrice: 1500, InitialRisk: 160,
		Status: HuntingStatusHolding}
	if err := r.SaveHunting(&h); err != nil {
		t.Fatalf("SaveHunting: %v", err)
	}
	if h.ID == 0 {
		t.Fatal("SaveHunting 未回填 ID")
	}
	if got, err := r.GetHuntingList(HuntingStatusHolding); err != nil || len(got) != 1 || got[0].EntryPrice != 1660 {
		t.Fatalf("持仓中列表: %v %+v", err, got)
	}

	// 退出：状态迁移到已退出，不再出现在持仓中
	if err := r.ExitHunting(h.ID, "2026-09-30", 1700, "失效位退出"); err != nil {
		t.Fatalf("ExitHunting: %v", err)
	}
	if got, err := r.GetHuntingList(HuntingStatusExited); err != nil || len(got) != 1 ||
		got[0].ExitDate != "2026-09-30" || got[0].ExitPrice != 1700 || got[0].ExitReason != "失效位退出" {
		t.Fatalf("已退出列表: %v %+v", err, got)
	}
	if got, err := r.GetHuntingList(HuntingStatusHolding); err != nil || len(got) != 0 {
		t.Fatalf("退出后持仓中应为空: %v %+v", err, got)
	}

	// 旧版记录迁移为历史待核：可按状态过滤，不混入持仓/退出列表
	legacy := models.KhunterHunting{Code: "000001", Name: "旧记录", EnterDate: "2026-08-01",
		Status: HuntingStatusLegacy}
	if err := r.SaveHunting(&legacy); err != nil {
		t.Fatalf("SaveHunting: %v", err)
	}
	if got, err := r.GetHuntingList(HuntingStatusLegacy); err != nil || len(got) != 1 {
		t.Fatalf("历史待核列表: %v %+v", err, got)
	}
}

// TestHuntingMigrationDeduplicatesLegacyRows 旧库迁移：还原旧版表结构（无 signal_date、
// 状态为 追踪中/已移除、同股同日重复行），EnsureMigrate 必须幂等地
//  1. 补齐 signal_date = enter_date；
//  2. 旧状态改为 历史待核（旧记录不是真实持仓，不得伪装成可执行仓位）；
//  3. 同组重复行保留最新 ID 删除其余，再建 (code,signal_date)/(code,enter_date) 唯一索引；
//  4. 重复迁移与新增的 持仓中 交易互不影响。
func TestHuntingMigrationDeduplicatesLegacyRows(t *testing.T) {
	setupTestDB(t)
	if err := db.Dao.Migrator().DropTable("khunter_hunting"); err != nil {
		t.Fatal(err)
	}
	legacyDDL := `CREATE TABLE khunter_hunting (
		id integer PRIMARY KEY AUTOINCREMENT,
		code text, name text, enter_date text, enter_score numeric,
		support_price numeric, status text, track_days numeric,
		created_at datetime, updated_at datetime)`
	if err := db.Dao.Exec(legacyDDL).Error; err != nil {
		t.Fatal(err)
	}
	seed := []string{
		`(1,'600000','旧记录1','2026-11-10',70,9.5,'追踪中',3,'2026-11-10 15:00:00','2026-11-10 15:00:00')`,
		`(2,'600000','旧记录2','2026-11-10',72,9.4,'追踪中',3,'2026-11-10 15:00:00','2026-11-10 15:00:00')`,
		`(3,'600000','旧记录3','2026-11-03',65,9.2,'已移除',5,'2026-11-03 15:00:00','2026-11-03 15:00:00')`,
	}
	for _, v := range seed {
		sql := `INSERT INTO khunter_hunting
			(id, code, name, enter_date, enter_score, support_price, status, track_days, created_at, updated_at)
			VALUES ` + v
		if err := db.Dao.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := EnsureMigrate(); err != nil {
		t.Fatalf("迁移旧表失败: %v", err)
	}
	var rows []models.KhunterHunting
	if err := db.Dao.Order("id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].ID != 2 || rows[1].ID != 3 {
		t.Fatalf("同组重复应保留最新 ID: %+v", rows)
	}
	for _, row := range rows {
		if row.Status != HuntingStatusLegacy {
			t.Fatalf("旧记录应显示为历史待核，got %s", row.Status)
		}
	}
	if rows[0].SignalDate != "2026-11-10" || rows[1].SignalDate != "2026-11-03" {
		t.Fatalf("旧 signal_date 应用 enter_date 补齐: %+v", rows)
	}
	var idxNames []string
	if err := db.Dao.Raw(
		"SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='khunter_hunting'",
	).Scan(&idxNames).Error; err != nil {
		t.Fatal(err)
	}
	hasIdx := func(name string) bool {
		for _, n := range idxNames {
			if n == name {
				return true
			}
		}
		return false
	}
	if !hasIdx("idx_khh_code_signal") || !hasIdx("idx_khh_code_enter") {
		t.Fatalf("唯一索引未创建: %v", idxNames)
	}

	// 唯一约束生效：同股同信号日 / 同股同确认日都只能一行
	dupSignal := models.KhunterHunting{Code: "600000", SignalDate: "2026-11-10",
		EnterDate: "2026-11-12", Status: HuntingStatusHolding}
	if err := db.Dao.Create(&dupSignal).Error; err == nil {
		t.Fatal("同股同信号日重复应被唯一索引拒绝")
	}
	dupEnter := models.KhunterHunting{Code: "600000", SignalDate: "2026-11-17",
		EnterDate: "2026-11-10", Status: HuntingStatusHolding}
	if err := db.Dao.Create(&dupEnter).Error; err == nil {
		t.Fatal("同股同确认日重复应被唯一索引拒绝")
	}

	// 重复迁移幂等，且不得改动新交易的持仓状态
	holding := models.KhunterHunting{Code: "688322", SignalDate: "2026-11-11",
		EnterDate: "2026-11-12", Status: HuntingStatusHolding}
	if err := db.Dao.Create(&holding).Error; err != nil {
		t.Fatal(err)
	}
	if err := EnsureMigrate(); err != nil {
		t.Fatalf("重复迁移应幂等: %v", err)
	}
	var got models.KhunterHunting
	if err := db.Dao.Where("code=?", "688322").First(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != HuntingStatusHolding {
		t.Fatalf("重复迁移不得改动持仓中交易，got %s", got.Status)
	}
	var total int64
	if err := db.Dao.Model(&models.KhunterHunting{}).Count(&total).Error; err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Fatalf("重复迁移不得增删数据，got %d 行", total)
	}
}

// seedDayBar 造一根指定 adjusted 口径的日 K（GetTradeDates 覆盖度测试用）。
func seedDayBar(code, date string, adjusted bool) models.KLineBar {
	return models.KLineBar{
		StockCode: code, Period: "day", TradeDate: date, Adjusted: adjusted,
		Open: 10, High: 10.2, Low: 9.8, Close: 10, PrevClose: 9.9,
		Volume: 100, VolumeUnit: "share", Amount: 1000, AmountUnit: "CNY", Source: "seed",
	}
}

// GetTradeDates 优先 sh000300；本地未同步沪深300时按市场日 K 覆盖度重建交易日序列，
// 剔除覆盖度远低于峰值的脏日期，避免确认/退出窗口落在非交易日上。
func TestGetTradeDatesFallsBackToMarketCoverage(t *testing.T) {
	setupTestDB(t)
	if err := db.Dao.AutoMigrate(&models.KLineBar{}); err != nil {
		t.Fatal(err)
	}
	// 600 只股票：d1/d2 全覆盖、d3 部分（550/600 ≥ 90% 峰值）、d4 脏日期（仅 50 只）
	codes := make([]string, 0, 600)
	for i := 0; i < 600; i++ {
		codes = append(codes, fmt.Sprintf("sh%06d", 600000+i))
	}
	rows := make([]models.KLineBar, 0, 600*3+60)
	for i, c := range codes {
		rows = append(rows, seedDayBar(c, "2026-01-05", true))
		rows = append(rows, seedDayBar(c, "2026-01-06", true))
		if i < 550 {
			rows = append(rows, seedDayBar(c, "2026-01-07", true))
		}
		if i < 50 {
			rows = append(rows, seedDayBar(c, "2026-01-08", true))
		}
	}
	if err := db.Dao.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}

	r := NewRepo()
	dates, err := r.GetTradeDates(context.Background(), "2026-01-01", "2026-01-31")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"2026-01-05", "2026-01-06", "2026-01-07"}
	if strings.Join(dates, ",") != strings.Join(want, ",") {
		t.Fatalf("覆盖度回退交易日错误: got %v want %v", dates, want)
	}

	// 主路径：一旦存在 sh000300，直接使用其日期，不做覆盖度回退
	idxBar := seedDayBar("sh000300", "2026-01-05", false)
	if err := db.Dao.Create(&idxBar).Error; err != nil {
		t.Fatal(err)
	}
	dates, err = r.GetTradeDates(context.Background(), "2026-01-01", "2026-01-31")
	if err != nil {
		t.Fatal(err)
	}
	if len(dates) != 1 || dates[0] != "2026-01-05" {
		t.Fatalf("sh000300 主路径应精确返回其交易日: %v", dates)
	}
}
