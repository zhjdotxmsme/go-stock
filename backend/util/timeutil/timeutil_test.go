package timeutil

import (
	"testing"
	"time"
)

func TestParseDate(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"2026-09-21", "2026-09-21"},
		{"2026/09/21", "2026-09-21"}, // sina 斜杠格式归一
		{"20260921", "2026-09-21"},   // tushare 紧凑格式
		{" 2026-09-21 ", "2026-09-21"},
	}
	for _, tt := range tests {
		got, err := ParseDate(tt.in)
		if err != nil {
			t.Errorf("ParseDate(%q) err: %v", tt.in, err)
			continue
		}
		if DateStr(got) != tt.want {
			t.Errorf("ParseDate(%q) = %s, want %s", tt.in, DateStr(got), tt.want)
		}
	}
	if _, err := ParseDate("not-a-date"); err == nil {
		t.Error("ParseDate(not-a-date) should fail")
	}
}

func TestParseDateTimeLocalNotUTC(t *testing.T) {
	// 关键回归：解析结果必须是本地时区而非 UTC（time.Parse 的默认行为）
	tm, err := ParseDateTime("2026-09-21 09:30:00")
	if err != nil {
		t.Fatal(err)
	}
	if tm.Location() != time.Local {
		t.Errorf("location = %v, want time.Local", tm.Location())
	}
	if tm.Hour() != 9 || tm.Minute() != 30 {
		t.Errorf("time = %v, want 09:30", tm)
	}
}

func TestParseDateTimeLayouts(t *testing.T) {
	tests := []string{
		"2026-09-21 09:30:00",
		"2026-09-21 09:30",
		"2026-09-21",
		"20260921",
		"2026/09/21 09:30:00",
		"2026-09-21T09:30:00+08:00", // RFC3339 自带时区
	}
	for _, in := range tests {
		if tm := MustParseDateTime(in); tm.IsZero() {
			t.Errorf("MustParseDateTime(%q) returned zero", in)
		}
	}
}

func TestUnixConversions(t *testing.T) {
	sec := int64(1790000000)
	if UnixToLocal(sec).Unix() != sec {
		t.Error("UnixToLocal round-trip failed")
	}
	ms := int64(1790000000123)
	if UnixMilliToLocal(ms).UnixMilli() != ms {
		t.Error("UnixMilliToLocal round-trip failed")
	}
}

func TestDateStrZero(t *testing.T) {
	// 零值不应 panic
	_ = DateStr(time.Time{})
	_ = DateTimeStr(time.Time{})
}
