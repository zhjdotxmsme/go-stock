package khunter

import (
	"os"
	"path/filepath"
	"testing"
)

// 初始阈值固定（spec 6.1 / 计划全局约束），不得漂移。
func TestRiskControlConfigDefaultsFixed(t *testing.T) {
	cfg := DefaultRiskControlConfig()
	checks := []struct {
		name      string
		got, want float64
	}{
		{"maxFiveDayReturn", cfg.MaxFiveDayReturn, 0.20},
		{"maxMA20Deviation", cfg.MaxMA20Deviation, 0.15},
		{"maxATRRatio", cfg.MaxATRRatio, 0.08},
		{"minMedianAmount20", cfg.MinMedianAmount20, 100000000},
		{"atrStopMultiple", cfg.ATRStopMultiple, 1.5},
		{"maxEntryGap", cfg.MaxEntryGap, 0.02},
		{"accountRiskFraction", cfg.AccountRiskFraction, 0.005},
		{"initialCash", cfg.InitialCash, 1000000},
		{"commissionRate", cfg.CommissionRate, 0.0003},
		{"minCommission", cfg.MinCommission, 5},
		{"stampDutyRate", cfg.StampDutyRate, 0.0005},
		{"transferFeeRate", cfg.TransferFeeRate, 0},
		{"slippageRate", cfg.SlippageRate, 0.0005},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Fatalf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
	if cfg.ATRPeriod != 14 || cfg.ConfirmTradingDays != 3 || cfg.MaxHoldingTradingDays != 5 {
		t.Fatalf("整型参数漂移: %+v", cfg)
	}
}

// 仓库配置文件必须与默认配置完全一致（初始值只有这一份口径）。
func TestRiskControlConfigFileMatchesDefaults(t *testing.T) {
	got := LoadRiskControlConfig(filepath.Join("..", "..", "..", "config", "khunter_risk_control.json"))
	if got != DefaultRiskControlConfig() {
		t.Fatalf("config/khunter_risk_control.json 与默认配置不一致: %+v", got)
	}
}

// 文件缺失/解析失败回退默认；部分字段覆盖时只替换出现的键。
func TestLoadRiskControlConfigFallbackAndOverride(t *testing.T) {
	missing := LoadRiskControlConfig(filepath.Join(t.TempDir(), "not-exist.json"))
	if missing != DefaultRiskControlConfig() {
		t.Fatalf("缺文件必须回退默认配置: %+v", missing)
	}
	broken := filepath.Join(t.TempDir(), "broken.json")
	if err := os.WriteFile(broken, []byte("{oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := LoadRiskControlConfig(broken); got != DefaultRiskControlConfig() {
		t.Fatalf("解析失败必须回退默认配置: %+v", got)
	}
	partial := filepath.Join(t.TempDir(), "partial.json")
	if err := os.WriteFile(partial, []byte(`{"maxFiveDayReturn":0.3}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := LoadRiskControlConfig(partial)
	if got.MaxFiveDayReturn != 0.3 || got.MaxMA20Deviation != 0.15 || got.MinMedianAmount20 != 100000000 {
		t.Fatalf("部分覆盖应只替换出现的键: %+v", got)
	}
}
