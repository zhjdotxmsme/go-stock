package stockcode

import "testing"

func TestNormalize_AShare(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// Already normalized
		{"sh600519", "sh600519"},
		{"sz000001", "sz000001"},
		{"bj430047", "bj430047"},
		// Tushare format
		{"600519.SH", "sh600519"},
		{"000001.SZ", "sz000001"},
		{"430047.BJ", "bj430047"},
		{"688001.SH", "sh688001"},
		{"300001.SZ", "sz300001"},
		// EastMoney secid
		{"1.600519", "sh600519"},
		{"0.000001", "sz000001"},
		{"0.430047", "sz430047"}, // BJ uses SZ market in EM secid, but pure code heuristic would say bj
		{"128.00700", "hk00700"},
		// Pure digits (first-digit heuristic)
		{"600519", "sh600519"},
		{"000001", "sz000001"},
		{"300001", "sz300001"},
		{"430047", "bj430047"},
		{"688001", "sh688001"},
		{"900001", "bj900001"},
		// Uppercase prefix
		{"SH600519", "sh600519"},
		{"SZ000001", "sz000001"},
		// With whitespace
		{" sh600519 ", "sh600519"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Normalize(tt.input)
			if got != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalize_HK(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// Already normalized
		{"hk00700", "hk00700"},
		// Tushare format
		{"00700.HK", "hk00700"},
		{"09988.HK", "hk09988"},
		// EastMoney
		{"128.00700", "hk00700"},
		// Uppercase
		{"HK00700", "hk00700"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Normalize(tt.input)
			if got != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalize_US(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// Already normalized
		{"usAAPL", "usAAPL"},
		{"usTSLA", "usTSLA"},
		// Sina variant gb_
		{"gb_AAPL", "usAAPL"},
		{"gb_TSLA", "usTSLA"},
		{"GB_AAPL", "usAAPL"},
		// Tushare format
		{"AAPL.US", "usAAPL"},
		{"TSLA.US", "usTSLA"},
		// Uppercase
		{"USAAPL", "usAAPL"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Normalize(tt.input)
			if got != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalize_EdgeCases(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"   ", ""},
		{"AAPL", "usAAPL"}, // Pure letter = US ticker
		{"7", "7"},         // Single digit, too short
		{"12345", "12345"}, // 5 digits, cannot determine market, return as-is
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Normalize(tt.input)
			if got != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseStockCode(t *testing.T) {
	tests := []struct {
		input     string
		wantPre   string
		wantPure  string
	}{
		{"sh600519", "sh", "600519"},
		{"600519.SH", "sh", "600519"},
		{"1.600519", "sh", "600519"},
		{"sz000001", "sz", "000001"},
		{"000001.SZ", "sz", "000001"},
		{"0.000001", "sz", "000001"},
		{"bj430047", "bj", "430047"},
		{"430047.BJ", "bj", "430047"},
		{"hk00700", "hk", "00700"},
		{"00700.HK", "hk", "00700"},
		{"128.00700", "hk", "00700"},
		{"usAAPL", "us", "AAPL"},
		{"gb_AAPL", "us", "AAPL"},
		{"AAPL.US", "us", "AAPL"},
		{"SH600519", "sh", "600519"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			pre, pure := ParseStockCode(tt.input)
			if pre != tt.wantPre || pure != tt.wantPure {
				t.Errorf("ParseStockCode(%q) = (%q, %q), want (%q, %q)",
					tt.input, pre, pure, tt.wantPre, tt.wantPure)
			}
		})
	}
}

func TestToTushare(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"sh600519", "600519.SH"},
		{"sz000001", "000001.SZ"},
		{"bj430047", "430047.BJ"},
		{"hk00700", "00700.HK"},
		{"usAAPL", "AAPL"},
		{"gb_AAPL", "AAPL"},
		{"600519.SH", "600519.SH"},
		{"00700.HK", "00700.HK"},
		{"600519", "600519.SH"},
		{"AAPL", "AAPL"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToTushare(tt.input)
			if got != tt.want {
				t.Errorf("ToTushare(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestToEastMoney(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"sh600519", "1.600519"},
		{"sz000001", "0.000001"},
		{"bj430047", "0.430047"},
		{"hk00700", "128.00700"},
		{"usAAPL", ""}, // US not supported
		{"gb_AAPL", ""},
		{"600519.SH", "1.600519"},
		{"00700.HK", "128.00700"},
		{"600519", "1.600519"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToEastMoney(tt.input)
			if got != tt.want {
				t.Errorf("ToEastMoney(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestToTDX(t *testing.T) {
	tests := []struct {
		input     string
		wantMkt   uint8
		wantSym   string
	}{
		{"sh600519", 1, "600519"},
		{"sz000001", 0, "000001"},
		{"bj430047", 83, "430047"},
		{"hk00700", 3, "00700"},
		{"usAAPL", 2, "AAPL"},
		{"gb_AAPL", 2, "AAPL"},
		{"600519.SH", 1, "600519"},
		{"600519", 1, "600519"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			mkt, sym := ToTDX(tt.input)
			if mkt != tt.wantMkt || sym != tt.wantSym {
				t.Errorf("ToTDX(%q) = (%d, %q), want (%d, %q)",
					tt.input, mkt, sym, tt.wantMkt, tt.wantSym)
			}
		})
	}
}

func TestToTusharePure(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"sh600519", "600519"},
		{"usAAPL", "AAPL"},
		{"gb_AAPL", "AAPL"},
		{"600519.SH", "600519"},
		{"AAPL.US", "AAPL"},
		{"hk00700", "00700"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToTusharePure(tt.input)
			if got != tt.want {
				t.Errorf("ToTusharePure(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMarket(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"sh600519", "SH"},
		{"sz000001", "SZ"},
		{"bj430047", "BJ"},
		{"hk00700", "HK"},
		{"usAAPL", "US"},
		{"gb_AAPL", "US"},
		{"600519", "SH"},
		{"00700", ""},
		{"AAPL", "US"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Market(tt.input)
			if got != tt.want {
				t.Errorf("Market(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsA股(t *testing.T) {
	if !IsA股("sh600519") { t.Error("sh600519 should be A-share") }
	if !IsA股("sz000001") { t.Error("sz000001 should be A-share") }
	if !IsA股("bj430047") { t.Error("bj430047 should be A-share") }
	if IsA股("hk00700")   { t.Error("hk00700 should not be A-share") }
	if IsA股("usAAPL")    { t.Error("usAAPL should not be A-share") }
}

func TestIsHK(t *testing.T) {
	if !IsHK("hk00700")   { t.Error("hk00700 should be HK") }
	if IsHK("sh600519")  { t.Error("sh600519 should not be HK") }
}

func TestIsUS(t *testing.T) {
	if !IsUS("usAAPL")    { t.Error("usAAPL should be US") }
	if !IsUS("gb_AAPL")   { t.Error("gb_AAPL should be US") }
	if IsUS("sh600519")   { t.Error("sh600519 should not be US") }
}

func TestPureCode(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"sh600519", "600519"},
		{"hk00700", "00700"},
		{"usAAPL", "AAPL"},
		{"600519.SH", "600519"},
		{"1.600519", "600519"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := PureCode(tt.input)
			if got != tt.want {
				t.Errorf("PureCode(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStockCodeCandidates(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"sh600519", []string{"sh600519", "600519.SH"}},
		{"usAAPL", []string{"usAAPL", "AAPL", "gb_AAPL"}},
		{"hk00700", []string{"hk00700", "00700.HK"}},
		{"", nil},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := StockCodeCandidates(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("StockCodeCandidates(%q) len = %d, want %d", tt.input, len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("StockCodeCandidates(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}
