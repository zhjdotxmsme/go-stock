package multi

import (
	"strings"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/models"
)

var checkNow = time.Date(2026, 9, 7, 10, 0, 0, 0, time.Local)

func klineBars(days ...string) *[]data.KLineData {
	bars := make([]data.KLineData, 0, len(days))
	for _, d := range days {
		bars = append(bars, data.KLineData{Day: d, Close: "10.5", Open: "10.0", High: "11", Low: "9.8"})
	}
	return &bars
}

func fullPack(kline *[]data.KLineData) *DataPack {
	return &DataPack{
		StockCode:           "sh600519",
		KLineDaily:          kline,
		TechnicalIndicators: &data.IndicatorResult{},
		FinancialReports:    &[]string{"report-1"},
		HistoryMoneyData:    []models.StockMoneyDataHis{{}},
	}
}

func TestCheckDataPack_NilPackPasses(t *testing.T) {
	rep := CheckDataPack(nil, checkNow)
	if !rep.Passed {
		t.Fatalf("nil pack should pass (analysts self-fetch), got fail: %s", rep.FailReason)
	}
}

func TestCheckDataPack_AllHealthy(t *testing.T) {
	pack := fullPack(klineBars("2026-09-04", "2026-09-07"))
	rep := CheckDataPack(pack, checkNow)
	if !rep.Passed {
		t.Fatalf("healthy pack should pass, got: %s", rep.Summary())
	}
	if rep.Completeness != 1.0 {
		t.Errorf("completeness = %v, want 1.0", rep.Completeness)
	}
}

func TestCheckDataPack_MissingKlineFails(t *testing.T) {
	pack := fullPack(nil)
	rep := CheckDataPack(pack, checkNow)
	if rep.Passed {
		t.Fatal("missing kline should hard-fail")
	}
	if !strings.Contains(rep.FailReason, "K线") {
		t.Errorf("fail reason should mention K线, got: %s", rep.FailReason)
	}
}

func TestCheckDataPack_UnparseableKlineFails(t *testing.T) {
	bars := &[]data.KLineData{{Day: "garbage", Close: "not-a-number"}}
	rep := CheckDataPack(fullPack(bars), checkNow)
	if rep.Passed {
		t.Fatal("all-unparseable kline should hard-fail")
	}
}

func TestCheckDataPack_StaleKlineFails(t *testing.T) {
	// 20 天前的数据：超过 12 天阈值（长假间隙之内才放行）
	rep := CheckDataPack(fullPack(klineBars("2026-08-18")), checkNow)
	if rep.Passed {
		t.Fatal("stale kline (>12d) should fail")
	}
	if !strings.Contains(rep.FailReason, "过期") {
		t.Errorf("fail reason should mention 过期, got: %s", rep.FailReason)
	}
}

func TestCheckDataPack_LongHolidayGapPasses(t *testing.T) {
	// 2024 春节式间隙：2/8 收盘 → 2/19 复市，间隔 11 天，应在阈值内
	rep := CheckDataPack(fullPack(klineBars("2024-02-08")), time.Date(2024, 2, 19, 9, 30, 0, 0, time.Local))
	if !rep.Passed {
		t.Fatalf("11-day holiday gap should pass, got: %s", rep.Summary())
	}
}

func TestCheckDataPack_CompletenessBelowHalfFails(t *testing.T) {
	// 只有 K 线可用（1/4 = 25% < 50%）：K 线本身新鲜，但整体完整率不足
	rep := CheckDataPack(&DataPack{KLineDaily: klineBars("2026-09-07")}, checkNow)
	if rep.Passed {
		t.Fatalf("completeness 25%% should fail, got: %s", rep.Summary())
	}
	if rep.Completeness != 0.25 {
		t.Errorf("completeness = %v, want 0.25", rep.Completeness)
	}
}

func TestCheckDataPack_ExactlyHalfCompletenessPasses(t *testing.T) {
	// K 线 + 技术指标可用（2/4 = 50%）：达到阈值应放行
	rep := CheckDataPack(&DataPack{
		KLineDaily:          klineBars("2026-09-07"),
		TechnicalIndicators: &data.IndicatorResult{},
	}, checkNow)
	if !rep.Passed {
		t.Fatalf("completeness 50%% should pass, got: %s", rep.Summary())
	}
}
