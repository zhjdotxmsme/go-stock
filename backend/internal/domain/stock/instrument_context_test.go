package stock

import (
	"strings"
	"testing"
)

func TestDetectInstrumentContext_Markets(t *testing.T) {
	tests := []struct {
		code       string
		wantMarket string
	}{
		{"sh600519", "A股(沪深)"},
		{"sz000001", "A股(沪深)"},
		{"bj430047", "A股(北交所)"},
		{"hk00700", "港股"},
		{"usAAPL", "美股"},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			ctx := DetectInstrumentContext(tt.code)
			if ctx.Market != tt.wantMarket {
				t.Errorf("DetectInstrumentContext(%q).Market = %q, want %q", tt.code, ctx.Market, tt.wantMarket)
			}
			if ctx.PromptBlock() == "" {
				t.Errorf("DetectInstrumentContext(%q).PromptBlock() should not be empty", tt.code)
			}
		})
	}
}

func TestDetectInstrumentContext_UnknownPassesThrough(t *testing.T) {
	for _, code := range []string{"", "600519", "x", "sh", "SH600519"} {
		// "sh600519" 大写也应识别（规范化为小写前缀）
		ctx := DetectInstrumentContext(code)
		if code == "SH600519" {
			if ctx.Market != "A股(沪深)" {
				t.Errorf("uppercase prefix should normalize: got %q", ctx.Market)
			}
			continue
		}
		if ctx.Market != "" {
			t.Errorf("DetectInstrumentContext(%q).Market = %q, want empty", code, ctx.Market)
		}
		if ctx.PromptBlock() != "" {
			t.Errorf("unknown market PromptBlock should be empty, got %q", ctx.PromptBlock())
		}
	}
}

func TestPromptBlock_ContainsKeyRules(t *testing.T) {
	a := DetectInstrumentContext("sh600519").PromptBlock()
	for _, want := range []string{"±10%", "T+1", "100股", "人民币"} {
		if !strings.Contains(a, want) {
			t.Errorf("A股 PromptBlock missing %q", want)
		}
	}
	hk := DetectInstrumentContext("hk00700").PromptBlock()
	if !strings.Contains(hk, "无涨跌幅限制") || !strings.Contains(hk, "T+2") {
		t.Error("港股 PromptBlock missing no-limit / T+2 rules")
	}
	us := DetectInstrumentContext("usAAPL").PromptBlock()
	if !strings.Contains(us, "1 股起买") || !strings.Contains(us, "美元") {
		t.Error("美股 PromptBlock missing fractional-share / USD rules")
	}
}
