package risk

import (
	"math"
	"testing"
)

// cleanInput 构造一个不触发任何检查的基线输入。
func cleanInput() *RiskInput {
	return &RiskInput{
		Code:             "600000",
		ChangePercent:    1,
		VolumeRatio:      1.5,
		TurnoverRate:     5,
		PE:               15,
		PB:               2,
		SignalScore:      60,
		MACDState:        MACDBullish,
		RSIState:         RSINeutral,
		LLMConfidence:    0.8,
		KLineQuality:     90,
		HasSignalScore:   true,
		HasLLMConfidence: true,
		HasKLineQuality:  true,
	}
}

func findCheck(t *testing.T, r RiskResult, name string) RiskCheck {
	t.Helper()
	for _, c := range r.Checks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("未触发检查项: %s (points=%.2f, checks=%v)", name, r.Points, r.Checks)
	return RiskCheck{}
}

func almostEqual(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.001 {
		t.Errorf("%s: got %.4f, want %.4f", name, got, want)
	}
}

func TestRiskOverlayCleanInput(t *testing.T) {
	o := NewRiskOverlay(DefaultRiskProfile())
	r := o.Evaluate(cleanInput())
	almostEqual(t, "基线不扣分", r.Points, 0)
	if r.Level != RiskLow {
		t.Errorf("基线应为 low, got %s", r.Level)
	}
	if len(r.Checks) != 0 {
		t.Errorf("基线不应有触发项, got %v", r.Checks)
	}
}

// TestRiskOverlayTriggers 17 项检查逐项触发验证。
func TestRiskOverlayTriggers(t *testing.T) {
	p := DefaultRiskProfile()
	cases := []struct {
		name      string
		mutate    func(in *RiskInput)
		checkName string
		points    float64
	}{
		{"单日追高", func(in *RiskInput) { in.ChangePercent = 8 }, "chase_high", 4.0},
		{"单日破位", func(in *RiskInput) { in.ChangePercent = -7 }, "break_down", 3.5},
		{"异常量比", func(in *RiskInput) { in.VolumeRatio = 6 }, "abnormal_volume_ratio", 3.0},
		{"高换手", func(in *RiskInput) { in.TurnoverRate = 15 }, "high_turnover", 3.0},
		{"无效PE", func(in *RiskInput) { in.PE = 0 }, "invalid_pe", 3.0},
		{"亏损PE", func(in *RiskInput) { in.PE = -5 }, "invalid_pe", 3.0},
		{"高PB", func(in *RiskInput) { in.PB = 8 }, "high_pb", 2.0},
		{"弱日线信号", func(in *RiskInput) { in.SignalScore = 44 }, "weak_signal", 2.5},
		{"MACD空头", func(in *RiskInput) { in.MACDState = MACDBearish }, "macd_bearish", 2.0},
		{"RSI超买", func(in *RiskInput) { in.RSIState = RSIOverbought }, "rsi_overbought", 1.5},
		{"低LLM置信度", func(in *RiskInput) { in.LLMConfidence = 0.34 }, "low_llm_confidence", 1.5},
		{"LLM风险标记", func(in *RiskInput) { in.LLMRiskFlags = []string{"a", "b"} }, "llm_risk_flags", 2.4},
		{"深度分析风险", func(in *RiskInput) { in.DeepRiskFlags = []string{"a", "b"} }, "deep_analysis_risks", 3.0},
		{"低日线质量", func(in *RiskInput) { in.KLineQuality = 69 }, "low_kline_quality", 2.0},
		{"日线获取失败", func(in *RiskInput) { in.KLineFetchFailed = true }, "kline_fetch_failed", 6.0},
		{"缓存过期", func(in *RiskInput) { in.StaleCache = true }, "stale_cache", 2.5},
		{"数据源降级", func(in *RiskInput) { in.FallbackErrors = true }, "fallback_errors", 1.5},
		{"异常数据标记", func(in *RiskInput) { in.InvalidDataFlags = []string{"invalid_ohlc"} }, "invalid_data", 3.0},
	}

	o := NewRiskOverlay(p)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := cleanInput()
			tc.mutate(in)
			r := o.Evaluate(in)
			c := findCheck(t, r, tc.checkName)
			almostEqual(t, "扣分", c.Points, tc.points)
		})
	}
}

// TestRiskOverlayNoTriggerBelowThreshold 阈值之下不触发。
func TestRiskOverlayNoTriggerBelowThreshold(t *testing.T) {
	in := cleanInput()
	in.ChangePercent = 7.9  // < 8
	in.VolumeRatio = 5.9    // < 6
	in.TurnoverRate = 14.9  // < 15
	in.PB = 7.9             // < 8
	in.SignalScore = 45     // 不低于阈值
	in.LLMConfidence = 0.35 // 不低于阈值
	in.KLineQuality = 70    // 不低于阈值
	r := NewRiskOverlay(DefaultRiskProfile()).Evaluate(in)
	almostEqual(t, "阈值之下不扣分", r.Points, 0)
	if len(r.Checks) != 0 {
		t.Errorf("阈值之下不应有触发项, got %v", r.Checks)
	}
}

// TestRiskOverlayFlagCaps 风险标记扣分封顶。
func TestRiskOverlayFlagCaps(t *testing.T) {
	o := NewRiskOverlay(DefaultRiskProfile())

	in := cleanInput()
	in.LLMRiskFlags = []string{"a", "b", "c", "d", "e"} // 5×1.2=6.0 → 封顶 4.0
	r := o.Evaluate(in)
	almostEqual(t, "LLM标记封顶", findCheck(t, r, "llm_risk_flags").Points, 4.0)

	in = cleanInput()
	in.DeepRiskFlags = []string{"a", "b", "c", "d"} // 4×1.5=6.0 → 封顶 4.5
	r = o.Evaluate(in)
	almostEqual(t, "深度风险封顶", findCheck(t, r, "deep_analysis_risks").Points, 4.5)
}

// TestRiskOverlayAbsentData 无数据标记时不做对应检查（区分零值与无数据）。
func TestRiskOverlayAbsentData(t *testing.T) {
	in := cleanInput()
	in.SignalScore = 10 // 远低于阈值，但标记为无数据
	in.LLMConfidence = 0.1
	in.KLineQuality = 10
	in.HasSignalScore = false
	in.HasLLMConfidence = false
	in.HasKLineQuality = false
	r := NewRiskOverlay(DefaultRiskProfile()).Evaluate(in)
	almostEqual(t, "无数据不检查", r.Points, 0)
}

// TestRiskLevelGrading 风险分级边界（基于 max_penalty=12.0：7.92/3.96）。
func TestRiskLevelGrading(t *testing.T) {
	p := DefaultRiskProfile()
	cases := []struct {
		points float64
		want   RiskLevel
	}{
		{0, RiskLow},
		{3.95, RiskLow},
		{3.96, RiskMedium},
		{7.91, RiskMedium},
		{7.92, RiskHigh},
		{12, RiskHigh},
	}
	for _, tc := range cases {
		if got := p.LevelForPoints(tc.points); got != tc.want {
			t.Errorf("LevelForPoints(%.2f): got %s, want %s", tc.points, got, tc.want)
		}
	}

	// 集成验证：获取失败 6.0 → medium；叠加追高 4.0 → high
	o := NewRiskOverlay(p)
	in := cleanInput()
	in.KLineFetchFailed = true
	if got := o.Evaluate(in).Level; got != RiskMedium {
		t.Errorf("获取失败单项应为 medium, got %s", got)
	}
	in.ChangePercent = 8
	if got := o.Evaluate(in).Level; got != RiskHigh {
		t.Errorf("叠加追高应为 high, got %s", got)
	}
}

// TestPortfolioBucketFor 板块桶匹配。
func TestPortfolioBucketFor(t *testing.T) {
	d := NewPortfolioDiversity()
	cases := []struct {
		industry string
		concepts []string
		want     string
	}{
		{"券商", nil, "金融"},
		{"房地产开发", nil, "地产链"},
		{"光伏设备", nil, "新能源"},
		{"通信设备", []string{"数据中心"}, "AI算力"},
		{"白酒", nil, "消费"},
		{"创新药", nil, "医药"},
		{"半导体", nil, "半导体"},
		{"化学原料", nil, ""},            // 无匹配
		{"银行", []string{"芯片"}, "金融"}, // 按桶顺序取第一个命中
	}
	for _, tc := range cases {
		if got := d.BucketFor(tc.industry, tc.concepts); got != tc.want {
			t.Errorf("BucketFor(%q, %v): got %q, want %q", tc.industry, tc.concepts, got, tc.want)
		}
	}
}

// TestPortfolioDiversityPenalty 板块集中度惩罚。
func TestPortfolioDiversityPenalty(t *testing.T) {
	d := NewPortfolioDiversity()

	// 同桶 2 只 → 超额 1 只扣 4.0
	r := d.Evaluate([]Holding{
		{Code: "601398", Industry: "银行"},
		{Code: "600030", Industry: "券商"},
	})
	almostEqual(t, "同桶2只", r.Penalty, 4.0)
	if len(r.Excess["金融"]) != 1 || r.Excess["金融"][0] != "600030" {
		t.Errorf("超额应为第二只 600030, got %v", r.Excess["金融"])
	}

	// 同桶 3 只 → 超额 2 只扣 8.0
	r = d.Evaluate([]Holding{
		{Code: "601398", Industry: "银行"},
		{Code: "600030", Industry: "券商"},
		{Code: "601318", Industry: "保险"},
	})
	almostEqual(t, "同桶3只", r.Penalty, 8.0)

	// 分属不同桶 → 不扣
	r = d.Evaluate([]Holding{
		{Code: "601398", Industry: "银行"},
		{Code: "600519", Industry: "白酒"},
		{Code: "688981", Industry: "半导体"},
	})
	almostEqual(t, "分散持仓", r.Penalty, 0)

	// 无匹配桶的标的不计入
	r = d.Evaluate([]Holding{
		{Code: "600000", Industry: "化学原料"},
		{Code: "600001", Industry: "钢铁"},
	})
	almostEqual(t, "无匹配桶", r.Penalty, 0)
	if len(r.Assignments) != 0 {
		t.Errorf("无匹配桶不应有归属, got %v", r.Assignments)
	}

	// 自定义同桶上限 2：2 只不扣，3 只扣 4.0
	d.MaxPerBucket = 2
	r = d.Evaluate([]Holding{
		{Code: "601398", Industry: "银行"},
		{Code: "600030", Industry: "券商"},
		{Code: "601318", Industry: "保险"},
	})
	almostEqual(t, "上限2只时3只", r.Penalty, 4.0)
}

// TestLoadRiskProfileJSON 配置加载。
func TestLoadRiskProfileJSON(t *testing.T) {
	// 空配置保留全部默认值
	p, err := LoadRiskProfileJSON([]byte(`{}`))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	almostEqual(t, "默认追高阈值", p.ChaseThreshold, 8)
	almostEqual(t, "默认分级基准", p.MaxPenalty, 12)

	// 部分覆盖，其余保留默认
	p, err = LoadRiskProfileJSON([]byte(`{"chaseThreshold": 10, "chasePenalty": 5}`))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	almostEqual(t, "覆盖追高阈值", p.ChaseThreshold, 10)
	almostEqual(t, "覆盖追高扣分", p.ChasePenalty, 5)
	almostEqual(t, "其余保留默认", p.BreakThreshold, -7)

	// 非法 JSON
	if _, err = LoadRiskProfileJSON([]byte(`{invalid`)); err == nil {
		t.Error("非法 JSON 应返回错误")
	}
}
