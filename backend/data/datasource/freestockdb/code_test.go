package freestockdb

import (
	"testing"
	"time"
)

func TestEngineCode(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"sh600519", "600519", true},
		{"600519.SH", "600519", true},
		{"sz000001", "000001", true},
		{"600633", "600633", true},
		{"hk00700", "", false},
		{"usAAPL", "", false},
	}
	for _, c := range cases {
		got, err := EngineCode(c.in)
		if c.ok && err != nil {
			t.Errorf("EngineCode(%q) err=%v", c.in, err)
			continue
		}
		if !c.ok && err == nil {
			t.Errorf("EngineCode(%q) want error", c.in)
			continue
		}
		if got != c.want {
			t.Errorf("EngineCode(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestInternalCode(t *testing.T) {
	if got := InternalCode("sh600519", "600519"); got != "sh600519" {
		t.Errorf("InternalCode = %q", got)
	}
	if got := InternalCode("sz000001", "000001"); got != "sz000001" {
		t.Errorf("InternalCode = %q", got)
	}
}

func TestBarTime(t *testing.T) {
	d := barTime(20260625)
	if d.Format("2006-01-02") != "2026-06-25" {
		t.Errorf("day = %v", d)
	}
	if d.Location() != time.Local {
		t.Errorf("want local tz")
	}
	m := barTime(20260625145200)
	if m.Format("2006-01-02 15:04:05") != "2026-06-25 14:52:00" {
		t.Errorf("minute = %v", m)
	}
}
