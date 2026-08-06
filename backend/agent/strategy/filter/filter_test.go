package filter

import (
	"strings"
	"testing"
)

// baseCandidate 构造一只通过默认配置全部规则的候选。
func baseCandidate() FilterInput {
	return FilterInput{
		Code: "600519", Name: "贵州茅台",
		Price: 100, ChangePercent: 2, Amount: 1e8, TotalMV: 5e9,
		PE: 20, PB: 2, VolumeRatio: 1.5, TurnoverRate: 3,
		HasDailyData: true,
		ChangePct60:  10, MABullAlign: true, AboveMA20: true, MA20DeviationPct: 2,
		SignalScore: 60, MACDState: "bullish", RSIState: "neutral",
		Breakout20dPct: 5, AmplitudePct: 5, VolumeRatio20d: 1.5, BodyPct: 0.5,
		ConsolidationDays: 3, VolatilityPct: 30, MaxDrawdownPct: -10, ATRPct: 3,
	}
}

// rejectedBy 用默认配置管道过滤单个候选，返回淘汰它的规则名（通过则空串）。
func rejectedBy(in FilterInput) string {
	cfg := DefaultHardFilterConfig()
	report := NewPipeline(&cfg).Apply([]FilterInput{in})
	if report.TotalPassed == 1 {
		return ""
	}
	return report.Stages[len(report.Stages)-1].Rule
}

// TestSnapshotRules 快照级 9 类规则触发/通过用例。
func TestSnapshotRules(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(in *FilterInput)
		rule   string
	}{
		{"ST名称", func(in *FilterInput) { in.Name = "STXX科技" }, "exclude_st"},
		{"退市名称", func(in *FilterInput) { in.Name = "退市XX" }, "exclude_st"},
		{"成交额过低", func(in *FilterInput) { in.Amount = 1e6 }, "amount_range"},
		{"价格过低", func(in *FilterInput) { in.Price = 1.5 }, "price_range"},
		{"价格过高", func(in *FilterInput) { in.Price = 400 }, "price_range"},
		{"PE过高", func(in *FilterInput) { in.PE = 200 }, "pe_range"},
		{"亏损PE", func(in *FilterInput) { in.PE = -5 }, "pe_range"}, // MinPE=0 边界：PE<=0 拒绝
		{"PB过高", func(in *FilterInput) { in.PB = 20 }, "pb_range"},
		{"量比过低", func(in *FilterInput) { in.VolumeRatio = 0.5 }, "volume_ratio_min"},
		{"换手率过低", func(in *FilterInput) { in.TurnoverRate = 0.5 }, "turnover_min"},
		{"接近涨停", func(in *FilterInput) { in.ChangePercent = 9.8 }, "change_pct_range"},
		{"接近跌停", func(in *FilterInput) { in.ChangePercent = -9.8 }, "change_pct_range"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := baseCandidate()
			tc.mutate(&in)
			if got := rejectedBy(in); got != tc.rule {
				t.Errorf("应被 %s 淘汰, got %q", tc.rule, got)
			}
		})
	}
	// 基线通过
	if got := rejectedBy(baseCandidate()); got != "" {
		t.Errorf("基线候选应全部通过, got %q", got)
	}
}

// TestDailyRules 日线级 15 类规则触发/通过用例。
func TestDailyRules(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(in *FilterInput)
		rule   string
	}{
		{"60日跌幅过深", func(in *FilterInput) { in.ChangePct60 = -20 }, "change_pct60_range"},
		{"60日涨幅过高", func(in *FilterInput) { in.ChangePct60 = 50 }, "change_pct60_range"},
		{"未站上MA20", func(in *FilterInput) { in.AboveMA20 = false }, "above_ma20"},
		{"信号分过低", func(in *FilterInput) { in.SignalScore = 40 }, "signal_score_min"},
		{"MACD空头", func(in *FilterInput) { in.MACDState = "bearish" }, "macd_whitelist"},
		{"RSI超买", func(in *FilterInput) { in.RSIState = "overbought" }, "rsi_whitelist"},
		{"突破幅度过高", func(in *FilterInput) { in.Breakout20dPct = 25 }, "breakout_range"},
		{"振幅过高", func(in *FilterInput) { in.AmplitudePct = 15 }, "amplitude_range"},
		{"20日量比过高", func(in *FilterInput) { in.VolumeRatio20d = 8 }, "volume_ratio20d_range"},
		{"实体比例过低", func(in *FilterInput) { in.BodyPct = 0.1 }, "body_pct_min"},
		{"MA20偏离过远", func(in *FilterInput) { in.MA20DeviationPct = 10 }, "ma20_deviation_range"},
		{"波动率过高", func(in *FilterInput) { in.VolatilityPct = 60 }, "volatility_max"},
		{"回撤过深", func(in *FilterInput) { in.MaxDrawdownPct = -30 }, "drawdown_min"},
		{"ATR过高", func(in *FilterInput) { in.ATRPct = 8 }, "atr_max"},
		{"无日线数据", func(in *FilterInput) { in.HasDailyData = false }, "change_pct60_range"}, // 第一条日线规则的 guard
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := baseCandidate()
			tc.mutate(&in)
			if got := rejectedBy(in); got != tc.rule {
				t.Errorf("应被 %s 淘汰, got %q", tc.rule, got)
			}
		})
	}
}

// TestDailyOptionalRules 默认关闭的日线规则：开启后生效。
func TestDailyOptionalRules(t *testing.T) {
	cfg := DefaultHardFilterConfig()
	cfg.RequireMABullAlign = true
	cfg.MinConsolidationDays = 2
	cfg.MaxConsolidationDays = 5

	in := baseCandidate()
	in.MABullAlign = false
	report := NewPipeline(&cfg).Apply([]FilterInput{in})
	if report.TotalPassed != 0 || report.Stages[0].Rule != "ma_bull_align" {
		t.Errorf("MA非多头排列应被淘汰: %+v", report.Stages)
	}

	in = baseCandidate()
	in.ConsolidationDays = 10
	report = NewPipeline(&cfg).Apply([]FilterInput{in})
	if report.TotalPassed != 0 || report.Stages[0].Rule != "consolidation_days_range" {
		t.Errorf("盘整天数超上限应被淘汰: %+v", report.Stages)
	}
}

// TestZeroConfigPassAll 零值配置全放行。
func TestZeroConfigPassAll(t *testing.T) {
	cfg := HardFilterConfig{}
	in := FilterInput{Code: "000001", Name: "ST平安", PE: -100, HasDailyData: false}
	report := NewPipeline(&cfg).Apply([]FilterInput{in})
	if report.TotalPassed != 1 {
		t.Errorf("零值配置应全放行: %+v", report)
	}
}

// TestMissingDailyAllowed RejectMissingDaily=false 时无日线数据跳过日线规则。
func TestMissingDailyAllowed(t *testing.T) {
	cfg := DefaultHardFilterConfig()
	cfg.RejectMissingDaily = false
	in := baseCandidate()
	in.HasDailyData = false
	report := NewPipeline(&cfg).Apply([]FilterInput{in})
	if report.TotalPassed != 1 {
		t.Errorf("无日线数据应跳过日线规则: %+v", report)
	}
}

// TestWaterfallDiagnostic 瀑布诊断：逐层淘汰、样本行、>90% 告警。
func TestWaterfallDiagnostic(t *testing.T) {
	cfg := DefaultHardFilterConfig()
	var candidates []FilterInput
	// 96 只量比过低（第一层数值规则淘汰率 >90% 告警）
	for i := 0; i < 96; i++ {
		c := baseCandidate()
		c.Code = strings.Repeat("6", 6)
		c.VolumeRatio = 0.1
		candidates = append(candidates, c)
	}
	// 4 只正常
	for i := 0; i < 4; i++ {
		candidates = append(candidates, baseCandidate())
	}
	// 1 只 ST（第一层就被淘汰）
	st := baseCandidate()
	st.Name = "ST测试"
	candidates = append(candidates, st)

	report := NewPipeline(&cfg).Apply(candidates)
	if report.TotalInput != 101 || report.TotalPassed != 4 {
		t.Errorf("瀑布计数: input=%d passed=%d", report.TotalInput, report.TotalPassed)
	}
	if len(report.Stages) != 2 {
		t.Fatalf("应有 2 层淘汰记录, got %d", len(report.Stages))
	}
	// 第一层 ST 淘汰 1 只，带样本
	if report.Stages[0].Rule != "exclude_st" || report.Stages[0].Rejected != 1 {
		t.Errorf("第一层: %+v", report.Stages[0])
	}
	if len(report.Stages[0].Samples) != 1 || report.Stages[0].Samples[0].Reason == "" {
		t.Errorf("第一层样本: %+v", report.Stages[0].Samples)
	}
	// 第三层（量比）淘汰 96/100 = 96% → 告警
	vrStage := report.Stages[1]
	if vrStage.Rule != "volume_ratio_min" || vrStage.Entering != 100 || vrStage.Rejected != 96 {
		t.Errorf("量比层: %+v", vrStage)
	}
	if len(vrStage.Samples) != 3 {
		t.Errorf("样本行应封顶 3 条, got %d", len(vrStage.Samples))
	}
	if len(report.Warnings) != 1 || !strings.Contains(report.Warnings[0], "volume_ratio_min") {
		t.Errorf("应有量比层 >90%% 告警: %v", report.Warnings)
	}
	// 文本输出包含关键段落
	text := report.Text()
	for _, want := range []string{"输入 101 只 → 通过 4 只", "exclude_st", "volume_ratio_min", "告警"} {
		if !strings.Contains(text, want) {
			t.Errorf("诊断文本缺少 %q", want)
		}
	}
}

// TestLoadHardFilterConfigJSON 配置加载。
func TestLoadHardFilterConfigJSON(t *testing.T) {
	cfg, err := LoadHardFilterConfigJSON([]byte(`{}`))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if !cfg.ExcludeST || cfg.MinSignalScore != 50 {
		t.Errorf("默认值: %+v", cfg)
	}

	cfg, err = LoadHardFilterConfigJSON([]byte(`{"minSignalScore": 65, "macdWhitelist": ["bullish"]}`))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if cfg.MinSignalScore != 65 || len(cfg.MACDWhitelist) != 1 || !cfg.ExcludeST {
		t.Errorf("部分覆盖: %+v", cfg)
	}

	if _, err = LoadHardFilterConfigJSON([]byte(`{invalid`)); err == nil {
		t.Error("非法 JSON 应返回错误")
	}
}
