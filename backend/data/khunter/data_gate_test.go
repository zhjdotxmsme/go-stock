package khunter

import (
	"testing"

	"go-stock/backend/models"
)

// validBar 构造一根单位齐全、质量合格的日 K。
func validBar() models.KLineBar {
	return models.KLineBar{
		TradeDate: "2026-11-11", Open: 10, High: 11, Low: 9, Close: 10,
		Volume: 10000, Amount: 100000, VolumeUnit: "share", AmountUnit: "CNY",
	}
}

func TestNormalizeVolumeUsesExplicitSourceUnits(t *testing.T) {
	tests := []struct {
		source    string
		raw, want int64
		unit      string
	}{
		{"eastmoney_kline", 120, 12000, "share"},
		{"mootdx_kline", 12000, 12000, "share"},
		{"freestockdb", 12000, 12000, "share"},
		{"unknown", 120, 120, "unknown"},
	}
	for _, tt := range tests {
		got, unit := NormalizeVolume(tt.source, tt.raw)
		if got != tt.want || unit != tt.unit {
			t.Fatalf("%s: got=(%d,%s)", tt.source, got, unit)
		}
	}
}

func TestDataGateRejectsInvalidBarsAndIntraday(t *testing.T) {
	valid := validBar()
	zeroAmount := valid
	zeroAmount.Amount = 0
	if d := EvaluateDataGate([]models.KLineBar{zeroAmount}, "2026-11-11", true); d.AllowConfirm {
		t.Fatal("零成交额不得确认")
	}
	badOHLC := valid
	badOHLC.High = 8
	if d := EvaluateDataGate([]models.KLineBar{badOHLC}, "2026-11-11", true); d.AllowConfirm {
		t.Fatal("OHLC 异常不得确认")
	}
	d := EvaluateDataGate([]models.KLineBar{valid}, "2026-11-11", false)
	if !d.AllowObservation || d.AllowConfirm || d.Status != "intraday" {
		t.Fatalf("盘中闸门错误: %+v", d)
	}
}

// 合格序列 + 同步完成：观察/确认/回测全部放行。
func TestDataGateAllowsValidSeriesWhenSyncCompleted(t *testing.T) {
	d := EvaluateDataGate([]models.KLineBar{validBar()}, "2026-11-11", true)
	if !d.AllowObservation || !d.AllowConfirm || !d.AllowBacktest || d.Status != "valid" || len(d.Reasons) != 0 {
		t.Fatalf("合格序列应全部放行: %+v", d)
	}
}

// 旧记录单位为空：允许展示（观察），禁止确认与回测。
func TestDataGateAllowsDisplayForLegacyEmptyUnits(t *testing.T) {
	b := validBar()
	b.VolumeUnit = ""
	b.AmountUnit = ""
	d := EvaluateDataGate([]models.KLineBar{b}, "2026-11-11", true)
	if !d.AllowObservation || d.AllowConfirm || d.AllowBacktest || d.Status != "invalid" {
		t.Fatalf("单位为空的旧记录闸门错误: %+v", d)
	}
}

func TestDataGateRejectsZeroVolumeAndUnknownUnit(t *testing.T) {
	zeroVol := validBar()
	zeroVol.Volume = 0
	if d := EvaluateDataGate([]models.KLineBar{zeroVol}, "2026-11-11", true); d.AllowConfirm {
		t.Fatal("零成交量不得确认")
	}
	unknownUnit := validBar()
	unknownUnit.VolumeUnit = "unknown"
	if d := EvaluateDataGate([]models.KLineBar{unknownUnit}, "2026-11-11", true); d.AllowConfirm {
		t.Fatal("未知成交量单位不得确认")
	}
	unknownAmount := validBar()
	unknownAmount.AmountUnit = "unknown"
	if d := EvaluateDataGate([]models.KLineBar{unknownAmount}, "2026-11-11", true); d.AllowConfirm {
		t.Fatal("未知成交额单位不得确认")
	}
	inverted := validBar()
	inverted.Low = 12 // Low > High，OHLC 关系异常
	if d := EvaluateDataGate([]models.KLineBar{inverted}, "2026-11-11", true); d.AllowConfirm {
		t.Fatal("Low>High 不得确认")
	}
}

// 目标日当天没有 K 线（序列停在前一交易日或为空）时不得确认。
func TestDataGateRequiresBarOnAsOfDate(t *testing.T) {
	stale := validBar()
	stale.TradeDate = "2026-11-10"
	d := EvaluateDataGate([]models.KLineBar{stale}, "2026-11-11", true)
	if d.AllowConfirm || d.Status == "valid" {
		t.Fatalf("目标日无K线不得确认: %+v", d)
	}
	empty := EvaluateDataGate(nil, "2026-11-11", true)
	if empty.AllowConfirm || empty.AllowObservation || empty.Status == "valid" {
		t.Fatalf("空序列闸门错误: %+v", empty)
	}
}

// 序列包含晚于目标日的未来 K 线：时点数据被污染，整体拒绝。
func TestDataGateRejectsFutureBars(t *testing.T) {
	future := validBar()
	future.TradeDate = "2026-11-12"
	d := EvaluateDataGate([]models.KLineBar{validBar(), future}, "2026-11-11", true)
	if d.AllowObservation || d.AllowConfirm || d.Status == "valid" {
		t.Fatalf("序列含未来K线必须整体拒绝: %+v", d)
	}
}
