package khunter

import (
	"context"
	"errors"
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

func TestResearchRunDoesNotMutateCurrentHunting(t *testing.T) {
	setupTestDB(t)
	old := models.KhunterHunting{Code: "600000", EnterDate: "2026-11-10", Status: "追踪中"}
	if err := db.Dao.Create(&old).Error; err != nil {
		t.Fatal(err)
	}
	_, err := RunPipelineWithOptions(context.Background(), PipelineOptions{
		TradeDate: "2026-11-11", Mode: RunModeResearch,
	})
	if err != nil {
		t.Fatal(err)
	}
	var huntingCount, researchCount int64
	if err := db.Dao.Model(&models.KhunterHunting{}).Count(&huntingCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Dao.Model(&models.KhunterResearchRecord{}).Count(&researchCount).Error; err != nil {
		t.Fatal(err)
	}
	if huntingCount != 1 || researchCount != 1 {
		t.Fatalf("研究运行污染当前池: hunting=%d research=%d", huntingCount, researchCount)
	}
	// 研究运行只写 KhunterResearchRecord：生产信号/评分/风险档/运行汇总必须保持为空
	for _, m := range []any{&models.KhunterSignal{}, &models.KhunterScore{}, &models.KhunterRiskLevel{}, &models.KhunterRunSummary{}} {
		var n int64
		if err := db.Dao.Model(m).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatalf("研究运行写入生产表 %T: %d 行", m, n)
		}
	}
}

func TestFundamentalSnapshotAsOf(t *testing.T) {
	setupTestDB(t)
	rows := []models.KhunterFundamentalSnapshot{
		{Code: "600000", AvailableDate: "2026-11-01", ReportPeriod: "2026-09-30", Payload: "old"},
		{Code: "600000", AvailableDate: "2026-11-15", ReportPeriod: "2026-12-31", Payload: "future"},
	}
	if err := db.Dao.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	got, err := NewRepo().GetFundamentalSnapshotAsOf("600000", "2026-11-11")
	if err != nil || got.Payload != "old" {
		t.Fatalf("读取到未来快照: %+v err=%v", got, err)
	}
	_, err = NewRepo().GetFundamentalSnapshotAsOf("600000", "2026-10-31")
	if !errors.Is(err, ErrSnapshotUnavailable) {
		t.Fatalf("缺快照必须返回不可回测错误: %v", err)
	}
}

func TestResearchRecordUpsertsByTradeDate(t *testing.T) {
	setupTestDB(t)
	r := NewRepo()
	row := &models.KhunterResearchRecord{TradeDate: "2026-11-11", Status: "success", ResultJSON: `{"v":1}`}
	if err := r.UpsertResearchRecord(row); err != nil {
		t.Fatal(err)
	}
	row = &models.KhunterResearchRecord{TradeDate: "2026-11-11", Status: "success", ResultJSON: `{"v":2}`}
	if err := r.UpsertResearchRecord(row); err != nil {
		t.Fatal(err)
	}
	rows, err := r.GetResearchRecords(10)
	if err != nil || len(rows) != 1 || rows[0].ResultJSON != `{"v":2}` {
		t.Fatalf("研究记录应按交易日覆盖更新: %+v err=%v", rows, err)
	}
	got, err := r.GetResearchRecord("2026-11-11")
	if err != nil || got == nil || got.ResultJSON != `{"v":2}` {
		t.Fatalf("GetResearchRecord: %+v err=%v", got, err)
	}
	if missing, err := r.GetResearchRecord("2026-11-12"); err != nil || missing != nil {
		t.Fatalf("缺记录应返回 nil 而非错误: %+v err=%v", missing, err)
	}
}

func TestSectorSnapshotAsOf(t *testing.T) {
	setupTestDB(t)
	rows := []models.KhunterSectorSnapshot{
		{Code: "600000", AvailableDate: "2026-11-01", Payload: `[{"Name":"银行","NetInflow":100}]`},
		{Code: "600000", AvailableDate: "2026-11-12", Payload: `[{"Name":"未来板块","NetInflow":999}]`},
	}
	if err := db.Dao.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	got, err := NewRepo().GetSectorSnapshotAsOf("600000", "2026-11-11")
	if err != nil || got.Payload != `[{"Name":"银行","NetInflow":100}]` {
		t.Fatalf("板块快照越过评分日: %+v err=%v", got, err)
	}
	if _, err = NewRepo().GetSectorSnapshotAsOf("600000", "2026-10-31"); !errors.Is(err, ErrSnapshotUnavailable) {
		t.Fatalf("缺板块快照必须返回不可回测错误: %v", err)
	}
}
