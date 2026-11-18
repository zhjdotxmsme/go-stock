package khunter

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

func TestObservationUpsertIsIdempotent(t *testing.T) {
	setupTestDB(t)
	r := NewRepo()
	row := models.KhunterObservation{
		Code: "688322", SignalDate: "2026-11-11", Status: "观察中", DataStatus: "valid",
		TriggerPrice: 50, SignalLow: 45, ATR14: 2,
	}
	if err := r.UpsertObservation(&row); err != nil {
		t.Fatal(err)
	}
	row.ID = 0
	if err := r.UpsertObservation(&row); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Dao.Model(&models.KhunterObservation{}).
		Where("code=? AND signal_date=?", "688322", "2026-11-11").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("同股同信号日应只有一条观察，got %d", count)
	}
}

func TestPersistObservationsDoesNotCreateHunting(t *testing.T) {
	setupTestDB(t)
	r := NewRepo()
	rows := []models.KhunterObservation{{
		Code: "600000", SignalDate: "2026-11-11", Status: "观察中", DataStatus: "valid", Score: 80,
	}}
	if err := r.SaveObservations(rows); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Dao.Model(&models.KhunterHunting{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("候选直接写入狩猎池: %d", count)
	}
}

// 同日重跑不得把已确认观察重置为观察中；终态只能由对应事务迁移。
func TestUpsertObservationKeepsConfirmedStatus(t *testing.T) {
	setupTestDB(t)
	r := NewRepo()
	row := models.KhunterObservation{Code: "600000", SignalDate: "2026-11-11",
		Status: ObsStatusPending, DataStatus: "valid"}
	if err := r.UpsertObservation(&row); err != nil {
		t.Fatal(err)
	}
	if err := db.Dao.Model(&models.KhunterObservation{}).Where("id = ?", row.ID).
		Update("status", ObsStatusConfirmed).Error; err != nil {
		t.Fatal(err)
	}
	rerun := models.KhunterObservation{Code: "600000", SignalDate: "2026-11-11",
		Status: ObsStatusPending, DataStatus: "valid", Score: 88}
	if err := r.UpsertObservation(&rerun); err != nil {
		t.Fatal(err)
	}
	var got models.KhunterObservation
	if err := db.Dao.Where("code=? AND signal_date=?", "600000", "2026-11-11").First(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != ObsStatusConfirmed {
		t.Fatalf("重跑不得重置已确认状态，got %s", got.Status)
	}
	if got.Score != 88 {
		t.Fatalf("重跑应更新评分等展示字段，got %v", got.Score)
	}
}

// GetPendingObservations 只返回观察中且确认窗口未关闭的记录：
// 已确认/已过期状态排除；确认有效期早于 asOfDate 的排除；未设有效期的仍视为待确认。
func TestGetPendingObservationsAsOf(t *testing.T) {
	setupTestDB(t)
	r := NewRepo()
	rows := []models.KhunterObservation{
		{Code: "600000", SignalDate: "2026-11-09", Status: ObsStatusPending, ConfirmExpire: "2026-11-12"},
		{Code: "600001", SignalDate: "2026-11-06", Status: ObsStatusPending, ConfirmExpire: "2026-11-10"},
		{Code: "600002", SignalDate: "2026-11-09", Status: ObsStatusConfirmed, ConfirmExpire: "2026-11-12"},
		{Code: "600003", SignalDate: "2026-11-09", Status: ObsStatusExpired, ConfirmExpire: "2026-11-12"},
		{Code: "600004", SignalDate: "2026-11-09", Status: ObsStatusPending},
	}
	if err := r.SaveObservations(rows); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetPendingObservations("2026-11-11")
	if err != nil {
		t.Fatal(err)
	}
	codes := map[string]bool{}
	for _, o := range got {
		codes[o.Code] = true
	}
	if len(got) != 2 || !codes["600000"] || !codes["600004"] {
		t.Fatalf("待确认观察应只含窗口未关闭的观察中记录，got %+v", got)
	}
}

// ConfirmHunting 事务化：校验观察状态 → 创建交易 → 观察改已确认。
// 同一观察重复确认必须失败且不产生第二笔交易。
func TestConfirmHuntingTransitionsObservationOnce(t *testing.T) {
	setupTestDB(t)
	r := NewRepo()
	obs := models.KhunterObservation{Code: "688322", Name: "测试股", SignalDate: "2026-11-11",
		Status: ObsStatusPending, DataStatus: "valid", TriggerPrice: 50, SignalLow: 45, ATR14: 2}
	if err := r.UpsertObservation(&obs); err != nil {
		t.Fatal(err)
	}

	trade := models.KhunterHunting{Code: obs.Code, Name: obs.Name,
		EnterDate: "2026-11-12", EnterScore: 80,
		EntryPrice: 50, InitialStopPrice: 45, ProtectPrice: 45, InitialRisk: 5}
	if err := r.ConfirmHunting(obs.ID, &trade); err != nil {
		t.Fatal(err)
	}
	if trade.ID == 0 {
		t.Fatal("确认未创建交易")
	}
	if trade.Status != HuntingStatusHolding {
		t.Fatalf("新交易必须为持仓中，got %s", trade.Status)
	}
	if trade.SignalDate != "2026-11-11" {
		t.Fatalf("交易必须带真实信号日，got %q", trade.SignalDate)
	}
	var got models.KhunterObservation
	if err := db.Dao.First(&got, obs.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != ObsStatusConfirmed {
		t.Fatalf("确认后观察应为已确认，got %s", got.Status)
	}

	dup := models.KhunterHunting{Code: obs.Code, EnterDate: "2026-11-13",
		EntryPrice: 51, InitialStopPrice: 46, InitialRisk: 5}
	if err := r.ConfirmHunting(obs.ID, &dup); !errors.Is(err, ErrObservationNotPending) {
		t.Fatalf("重复确认必须拒绝并返回非观察中错误，got %v", err)
	}
	var n int64
	if err := db.Dao.Model(&models.KhunterHunting{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("同一观察只能生成一笔交易，got %d", n)
	}
}

// 同一股票同一确认日只保留一条入池记录：两条不同信号日的观察在同一天确认时，
// 第二条必须被拒绝（数据库唯一索引 + 事务内前置检查双保险）。
func TestConfirmHuntingRejectsSecondTradeSameEnterDate(t *testing.T) {
	setupTestDB(t)
	r := NewRepo()
	obsA := models.KhunterObservation{Code: "600000", SignalDate: "2026-11-10", Status: ObsStatusPending}
	obsB := models.KhunterObservation{Code: "600000", SignalDate: "2026-11-09", Status: ObsStatusPending}
	if err := r.SaveObservations([]models.KhunterObservation{obsA, obsB}); err != nil {
		t.Fatal(err)
	}
	var ids []uint
	if err := db.Dao.Model(&models.KhunterObservation{}).Order("id").Pluck("id", &ids).Error; err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatalf("观察应各自独立落库，got %d", len(ids))
	}

	tradeA := models.KhunterHunting{Code: "600000", EnterDate: "2026-11-11",
		EntryPrice: 10, InitialStopPrice: 9, InitialRisk: 1}
	if err := r.ConfirmHunting(ids[0], &tradeA); err != nil {
		t.Fatal(err)
	}
	tradeB := models.KhunterHunting{Code: "600000", EnterDate: "2026-11-11",
		EntryPrice: 10.5, InitialStopPrice: 9.5, InitialRisk: 1}
	if err := r.ConfirmHunting(ids[1], &tradeB); !errors.Is(err, ErrTradeAlreadyExists) {
		t.Fatalf("同股同确认日第二次入池必须拒绝，got %v", err)
	}
	var n int64
	if err := db.Dao.Model(&models.KhunterHunting{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("同股同确认日只保留一条入池记录，got %d", n)
	}
}

// ExitHunting 只迁移持仓中 → 已退出；重复退出与不存在/非持仓 ID 都是安全空操作，
// 不得覆盖首次退出结果。
func TestExitHuntingIsIdempotentOnExitedPosition(t *testing.T) {
	setupTestDB(t)
	r := NewRepo()
	obs := models.KhunterObservation{Code: "600000", SignalDate: "2026-11-11", Status: ObsStatusPending}
	if err := r.UpsertObservation(&obs); err != nil {
		t.Fatal(err)
	}
	trade := models.KhunterHunting{Code: obs.Code, EnterDate: "2026-11-12",
		EntryPrice: 10, InitialStopPrice: 9, ProtectPrice: 9, InitialRisk: 1}
	if err := r.ConfirmHunting(obs.ID, &trade); err != nil {
		t.Fatal(err)
	}

	if err := r.ExitHunting(trade.ID, "2026-11-14", 9.5, "失效位退出"); err != nil {
		t.Fatalf("ExitHunting: %v", err)
	}
	got, err := r.GetHuntingList(HuntingStatusExited)
	if err != nil || len(got) != 1 || got[0].ExitDate != "2026-11-14" ||
		got[0].ExitPrice != 9.5 || got[0].ExitReason != "失效位退出" {
		t.Fatalf("退出字段未落库: %v %+v", err, got)
	}

	if err := r.ExitHunting(trade.ID, "2026-11-20", 1, "重复退出"); err != nil {
		t.Fatalf("重复退出应安全跳过: %v", err)
	}
	got, err = r.GetHuntingList(HuntingStatusExited)
	if err != nil || len(got) != 1 || got[0].ExitDate != "2026-11-14" || got[0].ExitReason != "失效位退出" {
		t.Fatalf("重复退出不得覆盖首次退出结果: %v %+v", err, got)
	}
	if err := r.ExitHunting(99999, "2026-11-20", 1, "不存在的ID"); err != nil {
		t.Fatalf("不存在的 ID 应安全跳过: %v", err)
	}
}

// BuildObservation 捕获信号日触发价 H / 信号日低点 L / ATR14 和数据闸门结论。
func TestBuildObservationCapturesSignalLevels(t *testing.T) {
	bars := make([]models.KLineBar, 20)
	for i := range bars {
		bars[i] = models.KLineBar{
			TradeDate: "2026-11-11", Open: 10, High: 11, Low: 9, Close: 10,
			Volume: 10000, Amount: 100000, VolumeUnit: "share", AmountUnit: "CNY",
		}
	}
	obs := BuildObservation(models.KhunterScore{Code: "688322", Name: "测试股", Total: 77},
		bars, DataDecision{AllowObservation: true, Status: "valid"}, "2026-11-11")
	if obs.Code != "688322" || obs.SignalDate != "2026-11-11" || obs.Status != ObsStatusPending {
		t.Fatalf("观察基础字段错误: %+v", obs)
	}
	if obs.TriggerPrice != 11 || obs.SignalLow != 9 {
		t.Fatalf("触发价/信号低点错误: %+v", obs)
	}
	if obs.ATR14 != 2 {
		t.Fatalf("ATR14 应为常数 TR=2，got %v", obs.ATR14)
	}
	if obs.Score != 77 || obs.DataStatus != "valid" || obs.RejectReason != "" {
		t.Fatalf("评分/数据状态透传错误: %+v", obs)
	}

	// 盘中决策：允许观察、禁止确认，拒绝原因必须透传到观察记录
	d := EvaluateDataGate(bars, "2026-11-11", false)
	if !d.AllowObservation || d.AllowConfirm || d.Status != "intraday" {
		t.Fatalf("盘中闸门错误: %+v", d)
	}
	obs2 := BuildObservation(models.KhunterScore{Code: "600000", Total: 80}, bars, d, "2026-11-11")
	if obs2.DataStatus != "intraday" || !strings.Contains(obs2.RejectReason, "盘中") {
		t.Fatalf("盘中观察必须带数据状态与拒绝原因: %+v", obs2)
	}
}

// 实时流水线无新信号时不得清空或重置旧观察，也不得创建狩猎行：
// 无信号日必须继续处理旧观察和已有持仓。
func TestPipelineKeepsPendingObservationsWithoutNewSignals(t *testing.T) {
	setupTestDB(t)
	if err := db.Dao.AutoMigrate(&models.AllStockInfo{}, &models.KLineBar{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Dao.Create(&models.AllStockInfo{
		SECUCODE: "600000.SH", SECURITYCODE: "600000", SECURITYNAMEABBR: "平稳股",
	}).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 30; i++ {
		date := time.Date(2026, 10, 20, 0, 0, 0, 0, time.Local).AddDate(0, 0, i)
		if err := db.Dao.Create(&models.KLineBar{
			StockCode: "sh600000", Period: "day", TradeDate: date.Format("2006-01-02"), Adjusted: true,
			Open: 10, High: 10.2, Low: 9.8, Close: 10,
			Volume: 10000, VolumeUnit: "share", Amount: 100000, AmountUnit: "CNY",
		}).Error; err != nil {
			t.Fatal(err)
		}
	}

	r := NewRepo()
	if err := r.SaveObservations([]models.KhunterObservation{{
		Code: "600000", SignalDate: "2026-11-05", Status: ObsStatusPending, DataStatus: "valid",
	}}); err != nil {
		t.Fatal(err)
	}

	res, err := RunPipeline(context.Background(), "2026-11-06")
	if err != nil {
		t.Fatal(err)
	}
	if res.Hunted != 0 {
		t.Fatalf("实时运行不得直接入池，hunted=%d", res.Hunted)
	}
	var huntingCount int64
	if err := db.Dao.Model(&models.KhunterHunting{}).Count(&huntingCount).Error; err != nil {
		t.Fatal(err)
	}
	if huntingCount != 0 {
		t.Fatalf("无信号重跑不得创建狩猎行: %d", huntingCount)
	}
	var got models.KhunterObservation
	if err := db.Dao.Where("code=? AND signal_date=?", "600000", "2026-11-05").First(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != ObsStatusPending {
		t.Fatalf("无信号重跑不得重置旧观察状态，got %s", got.Status)
	}
}
