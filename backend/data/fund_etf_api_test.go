package data

import (
	"testing"
)

func TestIsOnExchangeFund(t *testing.T) {
	yes := []string{
		"510300", // 沪ETF
		"588000", // 科创ETF
		"561560", // 沪ETF（56 前缀为本轮补充）
		"581000", // 沪基金（58 前缀为本轮补充）
		"518880", // 黄金ETF
		"159915", // 深ETF
		"161725", // LOF
		"500001", // 老封闭基金
	}
	for _, code := range yes {
		if !IsOnExchangeFund(code) {
			t.Errorf("IsOnExchangeFund(%q) = false, want true", code)
		}
	}

	no := []string{
		"600519", // 贵州茅台
		"000001", // 平安银行
		"300750", // 宁德时代
		"688981", // 中芯国际
		"430047", // 北交所
		"",
		"5",
	}
	for _, code := range no {
		if IsOnExchangeFund(code) {
			t.Errorf("IsOnExchangeFund(%q) = true, want false", code)
		}
	}
}

func TestPureFundCode(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"sh510300", "510300"},
		{"sz159915", "159915"},
		{"510300", "510300"},
		{" 510300 ", "510300"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := PureFundCode(tt.input); got != tt.want {
			t.Errorf("PureFundCode(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsOnExchangeFundCode(t *testing.T) {
	if !IsOnExchangeFundCode("sh510300") {
		t.Error("sh510300 should be on-exchange fund")
	}
	if !IsOnExchangeFundCode("sz159915") {
		t.Error("sz159915 should be on-exchange fund")
	}
	if IsOnExchangeFundCode("sh600519") {
		t.Error("sh600519 should not be on-exchange fund")
	}
}

func TestInstrumentKindOf(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"sh510300", InstrumentKindETF},
		{"sz159915", InstrumentKindETF},
		{"588000", InstrumentKindETF},
		{"sh600519", InstrumentKindStock},
		{"sz000001", InstrumentKindStock},
		{"hk00700", InstrumentKindStock},
		{"usAAPL", InstrumentKindStock},
	}
	for _, tt := range tests {
		if got := InstrumentKindOf(tt.input); got != tt.want {
			t.Errorf("InstrumentKindOf(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCalcEtfPremiumRate(t *testing.T) {
	tests := []struct {
		name  string
		price float64
		nav   float64
		want  *float64
	}{
		{"平价", 4.579, 4.579, ptrFloat(0)},
		{"溢价", 2.201, 1.9873, ptrFloat(10.75)},
		{"折价", 4.500, 4.6177, ptrFloat(-2.55)},
		{"价格非法", 0, 4.5, nil},
		{"净值非法", 4.5, 0, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calcEtfPremiumRate(tt.price, tt.nav)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("calcEtfPremiumRate(%v,%v) = %v, want nil", tt.price, tt.nav, *got)
				}
				return
			}
			if got == nil {
				t.Fatalf("calcEtfPremiumRate(%v,%v) = nil, want %v", tt.price, tt.nav, *tt.want)
			}
			if *got != *tt.want {
				t.Errorf("calcEtfPremiumRate(%v,%v) = %v, want %v", tt.price, tt.nav, *got, *tt.want)
			}
		})
	}
}

func ptrFloat(v float64) *float64 {
	return &v
}
