package data

import "testing"

func TestSanitizeKLineData_DropsFutureIndexGarbage(t *testing.T) {
	in := []KLineData{
		{Day: "2026-10-17", Close: "3800"},
		{Day: "2099-02-11", Close: "-3.39"},
		{Day: "9999", Close: "1"},
		{Day: "99991231", Close: "1"},
	}
	out := SanitizeKLineData(in)
	if len(out) != 1 {
		t.Fatalf("want 1 bar, got %d: %+v", len(out), out)
	}
	if out[0].Day != "2026-10-17" {
		t.Fatalf("unexpected kept bar: %+v", out[0])
	}
}

func TestSanitizeKLineData_KeepsYearMonthPeriods(t *testing.T) {
	in := []KLineData{
		{Day: "2025", Close: "3000"},
		{Day: "2025-09", Close: "3100"},
	}
	out := SanitizeKLineData(in)
	if len(out) != 2 {
		t.Fatalf("want 2 bars, got %d", len(out))
	}
}
