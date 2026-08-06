package data

import (
	"encoding/json"
	"fmt"
	"testing"

	"go-stock/backend/models"
)

// makeTestKLines 生成 n 根缓慢上行的测试 K 线（收盘价 10 → 10+n*0.03）。
func makeTestKLines(n int) []KLineData {
	klines := make([]KLineData, n)
	for i := 0; i < n; i++ {
		close := 10 + float64(i)*0.03
		k := KLineData{
			Day:    fmt.Sprintf("2026-01-%02d", i+1),
			Open:   fmt.Sprintf("%.2f", close-0.02),
			Close:  fmt.Sprintf("%.2f", close),
			High:   fmt.Sprintf("%.2f", close+0.1),
			Low:    fmt.Sprintf("%.2f", close-0.12),
			Volume: "1000000",
			Amount: "100000000",
		}
		if i == n-1 {
			k.Amplitude = "3.5"
			k.TurnoverRate = "5.0"
			k.VolumeRatio = "1.5"
			k.ChangePercent = "2.0"
		}
		klines[i] = k
	}
	return klines
}

// makeTestTech 从 K 线构造 stockTechData（与 scoreStockTech 的装配一致）。
func makeTestTech(klines []KLineData) *stockTechData {
	n := len(klines)
	tech := &stockTechData{
		KLines: klines,
		CloseP: make([]float64, n),
		HighP:  make([]float64, n),
		LowP:   make([]float64, n),
		Volume: make([]float64, n),
	}
	for i, k := range klines {
		tech.CloseP[i] = parseFloat64(k.Close)
		tech.HighP[i] = parseFloat64(k.High)
		tech.LowP[i] = parseFloat64(k.Low)
		tech.Volume[i] = parseFloat64(k.Volume)
	}
	return tech
}

// makeScoredPick 构造一条 stage-2 评分成功结果。
func makeScoredPick(code string, tech *stockTechData) scored {
	last := tech.KLines[len(tech.KLines)-1]
	return scored{
		pick: models.DailyPick{
			StockCode:     code,
			StockName:     "测试" + code,
			Score:         60,
			ClosePrice:    parseFloat64(last.Close),
			OpenPrice:     parseFloat64(last.Open),
			ChangePercent: parseFloat64(last.ChangePercent),
			TurnoverRate:  parseFloat64(last.TurnoverRate),
			VolumeRatio:   parseFloat64(last.VolumeRatio),
			Amount:        parseFloat64(last.Amount),
			Ma20:          11.0,
			Macd:          0.05,
			MacdSignal:    0.03,
			Rsi14:         55,
		},
		tech: tech,
	}
}

func TestDefaultPickEnhanceConfig(t *testing.T) {
	cfg := DefaultPickEnhanceConfig()
	if !cfg.EnableFilter || !cfg.EnableScoring || !cfg.EnableRisk {
		t.Error("默认应三个增强步骤全开")
	}
	if cfg.RiskExcludeEnabled {
		t.Error("风控排除默认应关闭（只标记不剔除）")
	}
	// PE/PB/总市值数据装配不出来，对应规则必须禁用，否则全零值会被误杀
	if cfg.Filter.MinPE != 0 || cfg.Filter.MaxPE != 0 || cfg.Filter.MinPB != 0 ||
		cfg.Filter.MaxPB != 0 || cfg.Filter.MinTotalMV != 0 || cfg.Filter.MaxTotalMV != 0 {
		t.Error("PE/PB/总市值过滤规则默认应禁用")
	}
	if cfg.Filter.RejectMissingDaily {
		t.Error("候选池阶段无 K 线，RejectMissingDaily 应为 false")
	}
	if cfg.RiskProfile.InvalidPEPenalty != 0 {
		t.Error("PE 数据缺失，InvalidPEPenalty 应为 0 避免误扣分")
	}
	// 只启用 K 线可装配的因子
	for _, name := range []string{"momentum", "activity", "liquidity", "stability"} {
		if cfg.Scorer.Weights[name] <= 0 {
			t.Errorf("因子 %s 应启用", name)
		}
	}
	if cfg.Scorer.Weights["value"] != 0 {
		t.Error("value 因子依赖 PE/PB，默认不应启用")
	}
}

func TestPickEnhanceConfigNormalize(t *testing.T) {
	// 零值 → 默认配置
	var zero PickEnhanceConfig
	if got := zero.normalize(); !got.EnableFilter || !got.EnableScoring || !got.EnableRisk {
		t.Error("零值配置应回落默认（全开）")
	}
	// 显式配置保留
	explicit := DefaultPickEnhanceConfig()
	explicit.EnableRisk = false
	if got := explicit.normalize(); got.EnableRisk {
		t.Error("显式关闭 EnableRisk 不应被 normalize 覆盖")
	}
}

func TestApplyHardFilter(t *testing.T) {
	e := NewDailyPickEngine()
	candidates := []stockCandidate{
		{Code: "600001.SH", Name: "正常股", HasSnapshot: true, Price: 10, ChangePercent: 2, Amount: 1e8, VolumeRatio: 1.5, TurnoverRate: 3},
		{Code: "600002.SH", Name: "ST测试", HasSnapshot: true, Price: 10, ChangePercent: 2, Amount: 1e8, VolumeRatio: 1.5, TurnoverRate: 3},
		{Code: "600003.SH", Name: "低价股", HasSnapshot: true, Price: 1, ChangePercent: 2, Amount: 1e8, VolumeRatio: 1.5, TurnoverRate: 3},
		{Code: "600004.SH", Name: "无快照", HasSnapshot: false},
	}

	got := e.applyHardFilter(candidates)
	codes := map[string]bool{}
	for _, c := range got {
		codes[c.Code] = true
	}
	if !codes["600001.SH"] {
		t.Error("正常股应通过过滤")
	}
	if codes["600002.SH"] {
		t.Error("ST 股应被过滤")
	}
	if codes["600003.SH"] {
		t.Error("低价股（价格<2）应被过滤")
	}
	if !codes["600004.SH"] {
		t.Error("无快照数据的候选应放行")
	}

	// 开关关闭：全部放行
	off := NewDailyPickEngine().WithEnhanceConfig(PickEnhanceConfig{EnableFilter: false, EnableScoring: true, EnableRisk: true})
	if got := off.applyHardFilter(candidates); len(got) != len(candidates) {
		t.Errorf("EnableFilter=false 应全部放行, got %d/%d", len(got), len(candidates))
	}

	// 全部被淘汰：回落原始候选池（失败降级）
	allST := []stockCandidate{
		{Code: "600005.SH", Name: "ST甲", HasSnapshot: true, Price: 10, ChangePercent: 2, Amount: 1e8, VolumeRatio: 1.5, TurnoverRate: 3},
	}
	if got := e.applyHardFilter(allST); len(got) != 1 {
		t.Error("过滤结果为空时应回落原始候选池")
	}
}

func TestEnhanceResultsScoring(t *testing.T) {
	e := NewDailyPickEngine()
	e.stockIndustryMap = map[string]string{"600001.SH": "半导体"}

	tech := makeTestTech(makeTestKLines(60))
	results := []scored{makeScoredPick("600001.SH", tech)}

	baseScore := results[0].pick.Score
	got := e.enhanceResults(results)
	pick := got[0].pick

	if pick.Score != baseScore {
		t.Errorf("旧字段 Score 不应被修改: %v → %v", baseScore, pick.Score)
	}
	if pick.ScreenScore <= 0 {
		t.Error("ScreenScore 应 > 0")
	}
	if pick.FinalScore != pick.ScreenScore {
		t.Error("D2 未接入前 FinalScore 应等于 ScreenScore")
	}
	var factors map[string]float64
	if err := json.Unmarshal([]byte(pick.FactorScores), &factors); err != nil {
		t.Fatalf("FactorScores 应为 JSON: %v", err)
	}
	for _, name := range []string{"momentum", "activity", "liquidity", "stability"} {
		if _, ok := factors[name]; !ok {
			t.Errorf("FactorScores 缺少因子 %s", name)
		}
	}
	if len(factors) != 9 {
		t.Errorf("FactorScores 应含全部 9 个因子明细, got %d", len(factors))
	}

	// D12 技术面派生字段
	if pick.SignalScore <= 0 {
		t.Error("SignalScore 应已填充")
	}
	if pick.MacdStatus != "bullish" { // macd=0.05 > signal=0.03 且 >0
		t.Errorf("MacdStatus: got %q, want bullish", pick.MacdStatus)
	}
	if pick.RsiStatus != "neutral" { // rsi=55
		t.Errorf("RsiStatus: got %q, want neutral", pick.RsiStatus)
	}
	if pick.Change60dPct <= 0 {
		t.Error("Change60dPct 应已填充（上行 K 线为正）")
	}
	if pick.AmplitudePct != 3.5 {
		t.Errorf("AmplitudePct: got %v, want 3.5", pick.AmplitudePct)
	}
	if pick.VolumeRatio20d <= 0 {
		t.Error("VolumeRatio20d 应已填充")
	}
	if pick.Atr14 <= 0 {
		t.Error("Atr14 应已填充")
	}
	if pick.Industry != "半导体" {
		t.Errorf("Industry: got %q, want 半导体", pick.Industry)
	}
}

func TestEnhanceResultsRisk(t *testing.T) {
	e := NewDailyPickEngine()
	tech := makeTestTech(makeTestKLines(60))

	// 追高场景：当日涨幅 9% ≥ 阈值 8%
	chased := makeScoredPick("600010.SH", tech)
	chased.pick.ChangePercent = 9
	// 无风险场景：正常数据，PE=0 不得触发 invalid_pe
	calm := makeScoredPick("600011.SH", makeTestTech(makeTestKLines(60)))

	// DSA 顺序：D1 评分 → D3 风控（A2 起风控为排序后的独立阶段）
	got := e.applyRiskToResults(e.enhanceResults([]scored{chased, calm}))

	p0 := got[0].pick
	if p0.RiskScore <= 0 {
		t.Error("追高票 RiskScore 应 > 0")
	}
	if p0.RiskLevel == "" || p0.RiskLevel == "low" {
		t.Errorf("追高票 RiskLevel 不应为 low, got %q", p0.RiskLevel)
	}
	var flags []string
	if err := json.Unmarshal([]byte(p0.RiskFlags), &flags); err != nil || len(flags) == 0 {
		t.Fatalf("RiskFlags 应为非空 JSON 数组: %v", err)
	}
	if flags[0] != "chase_high" {
		t.Errorf("首个风险标记: got %q, want chase_high", flags[0])
	}
	var checks []map[string]any
	if err := json.Unmarshal([]byte(p0.RiskChecks), &checks); err != nil || len(checks) == 0 {
		t.Fatalf("RiskChecks 应为非空 JSON 数组: %v", err)
	}

	p1 := got[1].pick
	if p1.RiskScore != 0 {
		t.Errorf("正常票 RiskScore 应为 0（PE=0 不得误扣）, got %v", p1.RiskScore)
	}
	if p1.RiskLevel != "low" {
		t.Errorf("正常票 RiskLevel: got %q, want low", p1.RiskLevel)
	}
	if p1.ExcludedByRisk {
		t.Error("正常票不应被标记 ExcludedByRisk")
	}
}

func TestEnhanceResultsDegradation(t *testing.T) {
	e := NewDailyPickEngine()
	tech := makeTestTech(makeTestKLines(60))

	// 评分失败的结果：完全不动
	failed := scored{pick: models.DailyPick{StockCode: "600020.SH", Score: 0}, err: fmt.Errorf("kline 不足")}
	// tech=nil：K 线缺失，不应 panic，评分/风控走中性数据
	noTech := scored{pick: models.DailyPick{
		StockCode: "600021.SH", Score: 50, ClosePrice: 10,
		ChangePercent: 1, TurnoverRate: 3, VolumeRatio: 1, Amount: 1e8,
	}}
	ok := makeScoredPick("600022.SH", tech)

	got := e.applyRiskToResults(e.enhanceResults([]scored{failed, noTech, ok}))

	if got[0].pick.ScreenScore != 0 || got[0].pick.RiskLevel != "" {
		t.Error("err != nil 的结果不应被增强")
	}
	if got[1].pick.ScreenScore <= 0 {
		t.Error("tech=nil 时评分应降级为中性分而非跳过")
	}
	if got[1].pick.RiskLevel == "" {
		t.Error("tech=nil 时风控仍应执行")
	}
	if got[2].pick.ScreenScore <= 0 {
		t.Error("正常结果应有 ScreenScore")
	}
}

func TestEnhanceResultsDisabled(t *testing.T) {
	cfg := DefaultPickEnhanceConfig()
	cfg.EnableScoring = false
	cfg.EnableRisk = false
	cfg.EnableFilter = false
	e := NewDailyPickEngine().WithEnhanceConfig(cfg)

	tech := makeTestTech(makeTestKLines(60))
	got := e.enhanceResults([]scored{makeScoredPick("600030.SH", tech)})
	pick := got[0].pick

	// 评分/风控全关：增强整体短路，所有 D12 增强字段留零（off 即全 off）
	if pick.ScreenScore != 0 || pick.FinalScore != 0 || pick.FactorScores != "" {
		t.Error("EnableScoring=false 时评分字段应留零")
	}
	if pick.RiskLevel != "" || pick.RiskFlags != "" {
		t.Error("EnableRisk=false 时风控字段应留零")
	}
	if pick.SignalScore != 0 {
		t.Error("增强全关时技术面派生字段应留零")
	}

	// 只开风控（A2 起风控为独立阶段 applyRiskToResults）
	cfg2 := DefaultPickEnhanceConfig()
	cfg2.EnableScoring = false
	e2 := NewDailyPickEngine().WithEnhanceConfig(cfg2)
	got2 := e2.applyRiskToResults(e2.enhanceResults([]scored{makeScoredPick("600031.SH", makeTestTech(makeTestKLines(60)))}))
	if got2[0].pick.ScreenScore != 0 {
		t.Error("EnableScoring=false 时 ScreenScore 应留零")
	}
	if got2[0].pick.RiskLevel == "" {
		t.Error("EnableRisk=true 时风控应执行")
	}
}

func TestRiskExcludedSwitch(t *testing.T) {
	pick := models.DailyPick{StockCode: "600040.SH", ExcludedByRisk: true}

	// 默认：只标记不剔除
	if NewDailyPickEngine().riskExcluded(pick) {
		t.Error("默认配置下高风险票应保留在结果中")
	}

	// 开启剔除
	cfg := DefaultPickEnhanceConfig()
	cfg.RiskExcludeEnabled = true
	e := NewDailyPickEngine().WithEnhanceConfig(cfg)
	if !e.riskExcluded(pick) {
		t.Error("RiskExcludeEnabled=true 时高风险票应被剔除")
	}
	pick.ExcludedByRisk = false
	if e.riskExcluded(pick) {
		t.Error("未被风控标记的票不应被剔除")
	}
}
