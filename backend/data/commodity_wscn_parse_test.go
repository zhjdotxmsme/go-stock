package data

import (
	"testing"
)

// TestParseWSCNKLineToBars 覆盖 WSCN K 线解析（根因B 的回归护）。
// 用 WSCN 实际返回的规范列序样本 [open, close, high, low, volume, tick_at] 验证
// 解析结果不再出现"时间戳读成开盘价、OHLC 整体错位"的 bug。
func TestParseWSCNKLineToBars(t *testing.T) {
	fields := []string{"open_px", "close_px", "high_px", "low_px", "turnover_volume", "tick_at"}
	lines := [][]float64{
		{4654.98, 4658.65, 4696.33, 4605.66, 0, 1787616000},  // 合法
		{4656.16, 4594.68, 4673.43, 4583.96, 12345, 1787702400}, // 合法，带量
		{100, 101, 102, 99, 0, 0},                                  // 时间戳=0，应被过滤
	}
	bars := parseWSCNKLineToBars(lines, fields)

	if len(bars) != 2 {
		t.Fatalf("expected 2 valid bars (zero-ts filtered), got %d", len(bars))
	}

	b := bars[0]
	if b.Open != 4654.98 {
		t.Errorf("Open = %v, want 4654.98", b.Open)
	}
	if b.Close != 4658.65 {
		t.Errorf("Close = %v, want 4658.65", b.Close)
	}
	if b.High != 4696.33 {
		t.Errorf("High = %v, want 4696.33", b.High)
	}
	if b.Low != 4605.66 {
		t.Errorf("Low = %v, want 4605.66", b.Low)
	}
	if b.Volume != 0 {
		t.Errorf("Volume = %v, want 0", b.Volume)
	}
	if b.Time.IsZero() || b.Time.Year() < 2020 || b.Time.Year() > 2100 {
		t.Errorf("Time invalid=%v (bug回归：时间戳被读错)", b.Time)
	}

	if bars[1].Volume != 12345 {
		t.Errorf("Volume(bar1) = %v, want 12345", bars[1].Volume)
	}
}

// TestParseWSCNKLineToBars_EmptyFieldsFallback：fields 元数据缺失时，
// 回退到规范序仍能正确解析（不因缺 fields 而崩）。
func TestParseWSCNKLineToBars_EmptyFieldsFallback(t *testing.T) {
	lines := [][]float64{{10, 11, 12, 9, 5, 1787616000}}
	bars := parseWSCNKLineToBars(lines, nil)
	if len(bars) != 1 {
		t.Fatalf("expected 1 bar, got %d", len(bars))
	}
	if bars[0].Open != 10 || bars[0].Close != 11 || bars[0].High != 12 || bars[0].Low != 9 {
		t.Errorf("fallback OHLC wrong: %+v", bars[0])
	}
	if bars[0].Time.Year() < 2020 {
		t.Errorf("fallback Time invalid=%v", bars[0].Time)
	}
}

// TestParseWSCNKLineToBars_NonCanonicalFieldsOrder：fields 描述了另一种列序（tick_at 在前）时，
// 按 fields 映射仍然正确 —— 证明"以 fields 元数据为准"是通用且稳健的。
func TestParseWSCNKLineToBars_NonCanonicalFieldsOrder(t *testing.T) {
	fields := []string{"tick_at", "open_px", "close_px", "high_px", "low_px", "turnover_volume"}
	lines := [][]float64{{1787616000, 10, 11, 12, 9, 5}}
	bars := parseWSCNKLineToBars(lines, fields)
	if len(bars) != 1 {
		t.Fatalf("expected 1 bar, got %d", len(bars))
	}
	if bars[0].Open != 10 || bars[0].Close != 11 || bars[0].High != 12 || bars[0].Low != 9 {
		t.Errorf("OHLC wrong under reordered fields: %+v", bars[0])
	}
	if bars[0].Volume != 5 {
		t.Errorf("Volume = %v, want 5", bars[0].Volume)
	}
	if bars[0].Time.Year() < 2020 {
		t.Errorf("Time invalid=%v", bars[0].Time)
	}
}
