package data

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNormalizePythonExe 路径规整：目录补 python.exe、带引号去引号、不存在时报错
func TestNormalizePythonExe(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "python.exe")
	if err := os.WriteFile(exe, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	// 目录路径 → 自动补 python.exe
	got, err := normalizePythonExe(dir)
	if err != nil || got != exe {
		t.Errorf("dir input: got %q, err=%v, want %q", got, err, exe)
	}
	// 带引号
	got, err = normalizePythonExe(`"` + dir + `"`)
	if err != nil || got != exe {
		t.Errorf("quoted dir: got %q, err=%v, want %q", got, err, exe)
	}
	// 不存在的带分隔符路径 → 报错
	if _, err = normalizePythonExe(filepath.Join(dir, "nope", "python.exe")); err == nil {
		t.Error("不存在的路径应报错")
	}
}

// TestNormalizeKronosCode 前端 K 线代码格式应能规范为 K 线接口代码
func TestNormalizeKronosCode(t *testing.T) {
	cases := []struct{ in, want string }{
		{"1.600519", "sh600519"},
		{"0.000001", "sz000001"},
		{"0.430047", "bj430047"}, // 北交所与深市共用东财 market 0，按首码分流
		{"600519.SH", "sh600519"},
		{"000001.SZ", "sz000001"},
		{"600519", "sh600519"},
		{"105.AAPL", "usAAPL"},   // 东财美股市场号
		{"116.00700", "hk00700"}, // 东财港股市场号（旧实现会误判为 sz00700）
	}
	for _, c := range cases {
		if got := NormalizeKronosCode(c.in); got != c.want {
			t.Errorf("NormalizeKronosCode(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestApplyKronosDefaults 未配置时应补齐默认值
func TestApplyKronosDefaults(t *testing.T) {
	s := &Settings{}
	applyKronosDefaults(s)
	if s.KronosPort != 8765 || s.KronosModel != "NeoQuasar/Kronos-small" ||
		s.KronosDevice != "auto" || s.KronosT != 1.0 || s.KronosTopP != 0.9 ||
		s.KronosPredLen != 5 || s.KronosSampleCnt != 4 {
		t.Errorf("applyKronosDefaults got %+v", s)
	}
}

// TestBuildHoldingsDeepPromptWithKronos 预测数据应注入 prompt；无预测时不出现
func TestBuildHoldingsDeepPromptWithKronos(t *testing.T) {
	stocks := []*HoldingsDeepStock{
		{
			StockCode:    "sh600519",
			StockName:    "贵州茅台",
			CurrentPrice: 1500,
			Kronos: &KronosPrediction{
				Bars: []*KronosBar{{Date: "2026-09-22", Open: 1501, High: 1520, Low: 1495, Close: 1510}},
				Summary: &KronosForecast{
					Direction: "up", ChangePct: 0.67, PredEnd: 1510,
					PredHigh: 1520, PredLow: 1490, Confidence: 72, PredLen: 5,
				},
			},
		},
		{StockCode: "sz000001", StockName: "平安银行", CurrentPrice: 11},
	}
	prompt := BuildHoldingsDeepPrompt(stocks)
	if !strings.Contains(prompt, "Kronos模型K线预测") {
		t.Error("prompt 应包含 Kronos 预测段")
	}
	if !strings.Contains(prompt, "一致度 72/100") {
		t.Error("prompt 应包含置信度")
	}
	// 无预测的持仓不应出现占位预测段
	if strings.Count(prompt, "Kronos模型K线预测") != 1 {
		t.Errorf("预测段应只出现 1 次，实际 %d", strings.Count(prompt, "Kronos模型K线预测"))
	}

	// 全部无预测：不出现 Kronos 段
	noPred := BuildHoldingsDeepPrompt([]*HoldingsDeepStock{{StockCode: "sh600519", StockName: "贵州茅台"}})
	if strings.Contains(noPred, "Kronos模型K线预测") {
		t.Error("无预测数据时 prompt 不应包含 Kronos 段")
	}
}
