package khunter

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"go-stock/backend/data/khunter/risk"
	"go-stock/backend/models"
)

// riskTestBar 构造一根单位齐全、质量合格的日 K（与 data_gate_test.validBar 同口径）。
func riskTestBar() models.KLineBar {
	return models.KLineBar{
		TradeDate: "2026-11-11", Open: 10, High: 11, Low: 9, Close: 10,
		Volume: 10000, Amount: 2e8, VolumeUnit: "share", AmountUnit: "CNY",
	}
}

// 四类风险阈值任一越界都必须否决确认并给出原因。
func TestEvaluateRiskThresholds(t *testing.T) {
	cfg := DefaultRiskControlConfig()
	base := RiskMetrics{FiveDayReturn: 0.10, MA20Deviation: 0.05, ATRRatio: 0.03, MedianAmount20: 2e8}
	tests := []struct {
		name   string
		mutate func(*RiskMetrics)
	}{
		{"近5日过热", func(m *RiskMetrics) { m.FiveDayReturn = 0.2001 }},
		{"MA20乖离过大", func(m *RiskMetrics) { m.MA20Deviation = 0.1501 }},
		{"ATR过高", func(m *RiskMetrics) { m.ATRRatio = 0.0801 }},
		{"流动性不足", func(m *RiskMetrics) { m.MedianAmount20 = 99999999 }},
	}
	for _, tt := range tests {
		m := base
		tt.mutate(&m)
		d := EvaluateRisk(m, DataDecision{AllowConfirm: true}, risk.LevelOf(-0.01), cfg)
		if d.AllowConfirm || len(d.Reasons) == 0 {
			t.Fatalf("%s 应拒绝确认: %+v", tt.name, d)
		}
	}
}

// 崩溃档（PositionLimit=0）与数据降级都不得新增风险。
func TestEvaluateRiskBlocksCrashAndDegradedData(t *testing.T) {
	cfg := DefaultRiskControlConfig()
	m := RiskMetrics{MedianAmount20: 2e8}
	if d := EvaluateRisk(m, DataDecision{AllowConfirm: true}, risk.LevelOf(-0.09), cfg); d.AllowConfirm {
		t.Fatal("崩溃档不得新增风险")
	}
	if d := EvaluateRisk(m, DataDecision{AllowConfirm: false}, risk.LevelOf(-0.01), cfg); d.AllowConfirm {
		t.Fatal("数据降级不得确认")
	}
}

// 干净指标 + 合格数据 + 正常市场档：放行确认且无否决原因。
func TestEvaluateRiskAllowsCleanMetrics(t *testing.T) {
	d := EvaluateRisk(
		RiskMetrics{FiveDayReturn: 0.10, MA20Deviation: 0.05, ATRRatio: 0.03, MedianAmount20: 2e8},
		DataDecision{AllowConfirm: true}, risk.LevelOf(-0.01), DefaultRiskControlConfig())
	if !d.AllowConfirm || len(d.Reasons) != 0 {
		t.Fatalf("干净指标应放行确认: %+v", d)
	}
}

// 指标口径：T close/T-5 close-1、close/MA20-1、指标包 ATR(14)、最近 20 根有效 K 线成交额中位数。
func TestBuildRiskMetricsComputesIndicatorInputs(t *testing.T) {
	bars := make([]models.KLineBar, 25)
	for i := range bars {
		bars[i] = riskTestBar()
		bars[i].TradeDate = "2026-10-01"
	}
	m, err := BuildRiskMetrics(bars, DefaultRiskControlConfig())
	if err != nil {
		t.Fatal(err)
	}
	if m.FiveDayReturn != 0 || m.MA20Deviation != 0 {
		t.Fatalf("恒定序列涨幅/乖离应为 0: %+v", m)
	}
	if math.Abs(m.ATR14-2) > 1e-9 || math.Abs(m.ATRRatio-0.2) > 1e-9 {
		t.Fatalf("ATR 口径错误（恒定 TR=2，收盘 10）: %+v", m)
	}
	if m.MedianAmount20 != 2e8 {
		t.Fatalf("成交额中位数应为 2e8: %v", m.MedianAmount20)
	}
}

// 近5日涨幅按 T 收盘 / T-5 收盘 - 1 计算（T 为最后一根有效 K 线）。
func TestBuildRiskMetricsFiveDayReturn(t *testing.T) {
	bars := make([]models.KLineBar, 30)
	for i := range bars {
		bars[i] = riskTestBar()
		bars[i].High, bars[i].Close, bars[i].Open = 10, 10, 10
		bars[i].Low = 10
	}
	bars[24].Close = 10 // T-5
	bars[29].Close = 12 // T
	bars[29].High = 12
	m, err := BuildRiskMetrics(bars, DefaultRiskControlConfig())
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(m.FiveDayReturn-0.2) > 1e-9 {
		t.Fatalf("近5日涨幅应为 12/10-1=0.2, got %v", m.FiveDayReturn)
	}
}

// 不足 20 根有效 K 线必须报错（禁止确认）；无效 K 线不计入。
func TestBuildRiskMetricsRequiresTwentyValidBars(t *testing.T) {
	short := make([]models.KLineBar, 19)
	for i := range short {
		short[i] = riskTestBar()
	}
	if _, err := BuildRiskMetrics(short, DefaultRiskControlConfig()); err == nil {
		t.Fatal("不足 20 根有效 K 线必须返回错误")
	}

	// 25 根中 6 根旧记录单位为空：有效仅 19 根 → 报错
	mixed := make([]models.KLineBar, 25)
	for i := range mixed {
		mixed[i] = riskTestBar()
		if i < 6 {
			mixed[i].VolumeUnit, mixed[i].AmountUnit = "", ""
		}
	}
	if _, err := BuildRiskMetrics(mixed, DefaultRiskControlConfig()); err == nil {
		t.Fatal("无效 K 线不得计入 20 根有效样本")
	}

	// 25 根中 5 根无效：有效 20 根 → 通过，且中位数只取有效 K 线
	ok := make([]models.KLineBar, 25)
	for i := range ok {
		ok[i] = riskTestBar()
		if i < 5 {
			ok[i].AmountUnit = ""
		} else {
			ok[i].Amount = 1e8 + float64(i)
		}
	}
	m, err := BuildRiskMetrics(ok, DefaultRiskControlConfig())
	if err != nil {
		t.Fatal(err)
	}
	if m.MedianAmount20 != (1e8+14+1e8+15)/2 {
		t.Fatalf("中位数应只取有效 K 线（20 根取中间两根均值）: %v", m.MedianAmount20)
	}
}

// 风险历史逐日追加：信号日失败 + 入场日恢复的结果都保留，同日重跑覆盖不重复。
func TestAppendRiskHistoryKeepsEveryDayOnce(t *testing.T) {
	cfg := DefaultRiskControlConfig()
	signalDay := EvaluateRisk(RiskMetrics{MedianAmount20: 2e8},
		DataDecision{AllowConfirm: false}, risk.LevelOf(-0.01), cfg)
	h := AppendRiskHistory("", "2026-11-11", signalDay)
	entryDay := EvaluateRisk(RiskMetrics{FiveDayReturn: 0.10, MA20Deviation: 0.05, ATRRatio: 0.03, MedianAmount20: 2e8},
		DataDecision{AllowConfirm: true}, risk.LevelOf(-0.01), cfg)
	h = AppendRiskHistory(h, "2026-11-12", entryDay)
	h = AppendRiskHistory(h, "2026-11-12", entryDay)

	var rows []RiskHistoryEntry
	if err := json.Unmarshal([]byte(h), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Date != "2026-11-11" || rows[1].Date != "2026-11-12" {
		t.Fatalf("风险历史应逐日追加且同日不重复: %s", h)
	}
	if rows[0].Decision.AllowConfirm || !rows[1].Decision.AllowConfirm {
		t.Fatalf("信号日失败与入场日恢复两份结论都应保留: %+v", rows)
	}
}

// 信号日风险结论落库：SignalRiskJSON 存结论、RiskHistoryJSON 追加当日记录、
// 否决原因并入 RejectReason；风险过滤不修改评分/形态字段。
func TestRiskMetricsPersistedOnObservation(t *testing.T) {
	bars := make([]models.KLineBar, 25)
	for i := range bars {
		bars[i] = riskTestBar()
	}
	dataReason := "盘中未收盘或同步未完成，只可观察不得确认"
	obs := models.KhunterObservation{Code: "600000", SignalDate: "2026-11-11", RejectReason: dataReason}
	d := AttachSignalRisk(&obs, bars,
		DataDecision{AllowObservation: true, Status: "intraday", Reasons: []string{dataReason}},
		risk.LevelOf(-0.01), DefaultRiskControlConfig())
	if d.AllowConfirm {
		t.Fatalf("盘中数据信号日不得确认: %+v", d)
	}

	var saved RiskDecision
	if err := json.Unmarshal([]byte(obs.SignalRiskJSON), &saved); err != nil {
		t.Fatalf("SignalRiskJSON 必须是合法 RiskDecision JSON: %v", err)
	}
	if saved.AllowConfirm || !strings.Contains(strings.Join(saved.Reasons, "；"), "数据不允许确认") {
		t.Fatalf("信号日结论未落库: %+v", saved)
	}
	if math.Abs(saved.Metrics.MedianAmount20-2e8) > 1e-9 {
		t.Fatalf("信号日指标未落库: %+v", saved.Metrics)
	}

	var rows []RiskHistoryEntry
	if err := json.Unmarshal([]byte(obs.RiskHistoryJSON), &rows); err != nil {
		t.Fatalf("RiskHistoryJSON 必须是合法数组: %v", err)
	}
	if len(rows) != 1 || rows[0].Date != "2026-11-11" || rows[0].Decision.AllowConfirm {
		t.Fatalf("信号日风险历史未追加: %+v", rows)
	}
	if !strings.Contains(obs.RejectReason, dataReason) || !strings.Contains(obs.RejectReason, "数据不允许确认") {
		t.Fatalf("拒绝原因应合并数据闸门与风险否决: %q", obs.RejectReason)
	}
}

// 有效 K 线不足时按错误否决确认并落库，阈值不再评估。
func TestRiskMetricsPersistedOnInsufficientHistory(t *testing.T) {
	bars := make([]models.KLineBar, 10)
	for i := range bars {
		bars[i] = riskTestBar()
	}
	obs := models.KhunterObservation{Code: "600000", SignalDate: "2026-11-11"}
	d := AttachSignalRisk(&obs, bars, DataDecision{AllowConfirm: true}, risk.LevelOf(-0.01), DefaultRiskControlConfig())
	if d.AllowConfirm || len(d.Reasons) == 0 || !strings.Contains(d.Reasons[0], "20") {
		t.Fatalf("历史不足必须否决并说明根数: %+v", d)
	}
	if obs.RejectReason == "" {
		t.Fatal("否决原因必须并入 RejectReason")
	}
}
