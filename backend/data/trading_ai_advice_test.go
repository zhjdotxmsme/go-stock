package data

import "testing"

func TestBareStockCode(t *testing.T) {
	cases := []struct{ in, want string }{
		{"600519.SH", "600519"},
		{"sh600519", "600519"},
		{"SZ000001", "000001"},
		{"000001.SZ", "000001"},
		{"bj832000", "832000"},
		{"hk00700", "00700"},
		{"600519", "600519"},
		{"", ""},
	}
	for _, c := range cases {
		if got := bareStockCode(c.in); got != c.want {
			t.Errorf("bareStockCode(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseRangePrice(t *testing.T) {
	cases := []struct {
		in       string
		wantMin  float64
		wantMax  float64
	}{
		{"10.5-12.3", 10.5, 12.3},
		{"10.5", 10.5, 0},
		{" 9.8 ~ 10.2 ", 9.8, 0}, // 非法分隔符：只解析出首个数值
		{"", 0, 0},
		{"12.3-10.5", 12.3, 12.3}, // 上下限颠倒时纠正
	}
	for _, c := range cases {
		minV, maxV := parseRangePrice(c.in)
		if minV != c.wantMin || maxV != c.wantMax {
			t.Errorf("parseRangePrice(%q) = (%v, %v), want (%v, %v)", c.in, minV, maxV, c.wantMin, c.wantMax)
		}
	}
}
