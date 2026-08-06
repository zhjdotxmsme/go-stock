package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"go-stock/backend/agent/strategy/ranking"
	"go-stock/backend/models"
)

// mockLLMResponse 构造合法的 LLM 排序响应 JSON。
func mockLLMResponse(scores map[string]float64) string {
	type stock struct {
		Code       string   `json:"code"`
		LLMScore   *float64 `json:"llm_score"`
		Confidence float64  `json:"confidence"`
		Sector     string   `json:"sector"`
		Theme      string   `json:"theme"`
		Thesis     string   `json:"thesis"`
		Reason     string   `json:"reason"`
		Risk       string   `json:"risk"`
		Catalysts  []string `json:"catalysts"`
		RiskFlags  []string `json:"risk_flags"`
		Tags       []string `json:"tags"`
	}
	resp := struct {
		MarketView     string  `json:"market_view"`
		SelectionLogic string  `json:"selection_logic"`
		PortfolioRisk  string  `json:"portfolio_risk"`
		Ranked         []stock `json:"ranked"`
	}{MarketView: "震荡市", SelectionLogic: "强者恒强", PortfolioRisk: "集中度可控"}
	for code, s := range scores {
		sc := s
		resp.Ranked = append(resp.Ranked, stock{
			Code: code, LLMScore: &sc, Confidence: 0.8,
			Sector: "半导体", Theme: "国产替代", Thesis: "逻辑", Reason: "推荐理由",
			Risk: "回撤风险", Catalysts: []string{"催化1"}, RiskFlags: []string{"风险1"},
			Tags: []string{"标签1"},
		})
	}
	data, _ := json.Marshal(resp)
	return string(data)
}

// makeRankedResult 构造带 D1 评分的成功结果（ScreenScore 已填）。
func makeRankedResult(code string, screenScore float64) scored {
	return scored{pick: models.DailyPick{
		StockCode: code, StockName: "测试" + code,
		Score: 60, ScreenScore: screenScore, FinalScore: screenScore,
		ClosePrice: 10, ChangePercent: 2, Amount: 3e8,
		TurnoverRate: 3, VolumeRatio: 2, SignalScore: 60,
	}}
}

func rankingTestEngine(call ranking.LLMCallFunc) *DailyPickEngine {
	cfg := DefaultPickEnhanceConfig()
	cfg.LLMCall = call
	cfg.ModelChain = []string{"mock-model"}
	return NewDailyPickEngine().WithEnhanceConfig(cfg)
}

func TestA2Defaults(t *testing.T) {
	cfg := DefaultPickEnhanceConfig()
	if !cfg.EnableLLMRanking {
		t.Error("D2 LLM 排序默认应开启（无 AI 配置时静默跳过）")
	}
	if !cfg.EnablePostAnalysis || !cfg.ApplyPostAnalysisDelta {
		t.Error("D10 后分析与 delta 计入默认应开启")
	}
	if cfg.EnableRotation {
		t.Error("D9 种子旋转默认应关闭")
	}
	if cfg.RemoteAnalyzerURL != "" {
		t.Error("远程分析器默认应不启用（URL 空）")
	}
	if cfg.LLMRankMaxCandidates != 30 {
		t.Errorf("LLM 排序候选上限: got %d, want 30", cfg.LLMRankMaxCandidates)
	}
	if cfg.Ranker.RankWeight != 0.40 {
		t.Errorf("RankWeight: got %v, want 0.40", cfg.Ranker.RankWeight)
	}
}

func TestApplyLLMRanking(t *testing.T) {
	call := func(ctx context.Context, model, prompt string) (string, error) {
		if model != "mock-model" {
			return "", fmt.Errorf("unexpected model %q", model)
		}
		return mockLLMResponse(map[string]float64{"600001.SH": 80, "600002.SH": 30}), nil
	}
	e := rankingTestEngine(call)

	// A 的 screen 分低但 LLM 分高：混合后 A 应排到 B 前
	results := []scored{makeRankedResult("600001.SH", 60), makeRankedResult("600002.SH", 70)}
	ranked := e.applyLLMRanking(context.Background(), results)
	if !ranked {
		t.Fatal("LLM 排序成功应返回 ranked=true")
	}

	// A: 60×0.6+80×0.4 = 68；B: 70×0.6+30×0.4 = 54
	a, b := results[0].pick, results[1].pick
	if a.StockCode != "600001.SH" {
		t.Errorf("重排后首位: got %s, want 600001.SH", a.StockCode)
	}
	if a.FinalScore != 68 {
		t.Errorf("A FinalScore: got %v, want 68", a.FinalScore)
	}
	if b.FinalScore != 54 {
		t.Errorf("B FinalScore: got %v, want 54", b.FinalScore)
	}

	// LLM 丰富字段写回
	if a.LlmScore != 80 || a.LlmConfidence != 0.8 || a.LlmSector != "半导体" ||
		a.LlmTheme != "国产替代" || a.LlmThesis != "逻辑" || a.LlmRisk != "回撤风险" ||
		a.RankingReason != "推荐理由" {
		t.Errorf("LLM 标量字段写回不完整: %+v", a)
	}
	var catalysts, riskFlags, tags []string
	if err := json.Unmarshal([]byte(a.LlmCatalysts), &catalysts); err != nil || len(catalysts) != 1 {
		t.Errorf("LlmCatalysts 应为 JSON 数组: %q", a.LlmCatalysts)
	}
	if err := json.Unmarshal([]byte(a.LlmRiskFlags), &riskFlags); err != nil || len(riskFlags) != 1 {
		t.Errorf("LlmRiskFlags 应为 JSON 数组: %q", a.LlmRiskFlags)
	}
	if err := json.Unmarshal([]byte(a.LlmTags), &tags); err != nil || len(tags) != 1 {
		t.Errorf("LlmTags 应为 JSON 数组: %q", a.LlmTags)
	}
}

func TestApplyLLMRankingDegraded(t *testing.T) {
	call := func(ctx context.Context, model, prompt string) (string, error) {
		return "", errors.New("mock LLM failure")
	}
	e := rankingTestEngine(call)

	results := []scored{makeRankedResult("600001.SH", 60), makeRankedResult("600002.SH", 70)}
	ranked := e.applyLLMRanking(context.Background(), results)
	if ranked {
		t.Error("LLM 全部失败应降级（ranked=false，保持 screen 序）")
	}
	if results[0].pick.StockCode != "600001.SH" {
		t.Error("降级时不应改变结果顺序")
	}
	if results[0].pick.FinalScore != results[0].pick.ScreenScore {
		t.Error("降级时 FinalScore 应保持等于 ScreenScore")
	}
	if results[0].pick.LlmScore != 0 {
		t.Error("降级时不应写入 LLM 字段")
	}
}

func TestApplyLLMRankingNoAIConfig(t *testing.T) {
	// 模型链来源注入为空（模拟无 AI 配置）：静默跳过，不是错误
	e := NewDailyPickEngine().WithModelChainFn(func() ([]string, ranking.LLMCallFunc) {
		return nil, nil
	})
	results := []scored{makeRankedResult("600001.SH", 60)}
	if e.applyLLMRanking(context.Background(), results) {
		t.Error("无 AI 配置时应静默跳过（ranked=false），不是错误")
	}

	// 开关关闭
	cfg := DefaultPickEnhanceConfig()
	cfg.EnableLLMRanking = false
	cfg.LLMCall = func(ctx context.Context, model, prompt string) (string, error) {
		t.Error("开关关闭时不应调用 LLM")
		return "", nil
	}
	cfg.ModelChain = []string{"m"}
	e2 := NewDailyPickEngine().WithEnhanceConfig(cfg)
	if e2.applyLLMRanking(context.Background(), results) {
		t.Error("EnableLLMRanking=false 应直接跳过")
	}
}

func TestApplyRiskWithLLMFlags(t *testing.T) {
	e := NewDailyPickEngine()
	// D2 先写 LlmRiskFlags/LlmConfidence，风控（排序后）应消费它们
	pick := makeRankedResult("600050.SH", 60).pick
	pick.LlmRiskFlags = `["商誉减值","股权质押"]`
	pick.LlmConfidence = 0.2 // < 0.35 低置信度

	got := e.applyRiskToResults([]scored{{pick: pick}})
	p := got[0].pick

	// llm_risk_flags: 2×1.2=2.4；low_llm_confidence: 1.5 → 共 3.9
	if p.RiskScore != 3.9 {
		t.Errorf("RiskScore: got %v, want 3.9（LLM 标记+低置信度应计入）", p.RiskScore)
	}
	var flags []string
	if err := json.Unmarshal([]byte(p.RiskFlags), &flags); err != nil {
		t.Fatalf("RiskFlags 解析失败: %v", err)
	}
	joined := fmt.Sprintf("%v", flags)
	if !contains(joined, "llm_risk_flags") || !contains(joined, "low_llm_confidence") {
		t.Errorf("RiskFlags 应含 llm_risk_flags 与 low_llm_confidence: %v", flags)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestApplyPostAnalysis(t *testing.T) {
	e := NewDailyPickEngine()

	// 资金确认（Amount 3e8≥2e8 且量比 2∈[1.2,5]）+ 风控低风险（RiskLevel=low）
	// → delta = 1.8 + 0.6 = 2.4
	ok := makeRankedResult("600060.SH", 60)
	ok.pick.RiskLevel = "low"
	// clamp 场景：FinalScore 99 + 正 delta 应截断到 100
	high := makeRankedResult("600061.SH", 99)
	high.pick.RiskLevel = "low"
	// 评分失败：不动
	failed := scored{pick: models.DailyPick{StockCode: "600062.SH"}, err: errors.New("x")}

	got := e.applyPostAnalysis(context.Background(), []scored{ok, high, failed})

	p0 := got[0].pick
	if p0.PostAnalysisStatus != "completed" || !p0.PostAnalysisCompleted {
		t.Errorf("后分析状态: got %q/%v, want completed/true", p0.PostAnalysisStatus, p0.PostAnalysisCompleted)
	}
	if p0.PostAnalysisScoreDelta != 2.4 || p0.PostAnalysisLocalDelta != 2.4 {
		t.Errorf("delta: got total=%v local=%v, want 2.4/2.4", p0.PostAnalysisScoreDelta, p0.PostAnalysisLocalDelta)
	}
	if p0.PostAnalysisRemoteDelta != 0 {
		t.Error("远程分析器默认不启用，RemoteDelta 应为 0")
	}
	if p0.FinalScore != 62.4 {
		t.Errorf("FinalScore 应计入 delta: got %v, want 62.4", p0.FinalScore)
	}
	var detail map[string]any
	if err := json.Unmarshal([]byte(p0.PostAnalysisDetail), &detail); err != nil {
		t.Errorf("PostAnalysisDetail 应为 JSON: %v", err)
	}

	p1 := got[1].pick
	if p1.FinalScore != 100 {
		t.Errorf("FinalScore clamp: got %v, want 100", p1.FinalScore)
	}

	if got[2].pick.PostAnalysisStatus != "" {
		t.Error("err != nil 的结果不应做后分析")
	}
}

func TestApplyPostAnalysisDeltaOff(t *testing.T) {
	cfg := DefaultPickEnhanceConfig()
	cfg.ApplyPostAnalysisDelta = false
	e := NewDailyPickEngine().WithEnhanceConfig(cfg)

	ok := makeRankedResult("600070.SH", 60)
	ok.pick.RiskLevel = "low"
	got := e.applyPostAnalysis(context.Background(), []scored{ok})

	p := got[0].pick
	if p.PostAnalysisScoreDelta != 2.4 {
		t.Errorf("delta 字段仍应写入: got %v, want 2.4", p.PostAnalysisScoreDelta)
	}
	if p.FinalScore != 60 {
		t.Errorf("ApplyPostAnalysisDelta=false 时 FinalScore 不应变: got %v", p.FinalScore)
	}

	// 开关全关：完全不动
	cfg2 := DefaultPickEnhanceConfig()
	cfg2.EnablePostAnalysis = false
	e2 := NewDailyPickEngine().WithEnhanceConfig(cfg2)
	got2 := e2.applyPostAnalysis(context.Background(), []scored{makeRankedResult("600071.SH", 60)})
	if got2[0].pick.PostAnalysisStatus != "" {
		t.Error("EnablePostAnalysis=false 应完全跳过")
	}
}

func TestApplyRotation(t *testing.T) {
	// 默认关闭：原样返回
	e := NewDailyPickEngine()
	picks := []models.DailyPick{
		{StockCode: "A", FinalScore: 90, PostAnalysisCompleted: true},
		{StockCode: "B", FinalScore: 80, PostAnalysisCompleted: true},
	}
	pool := []scored{{pick: models.DailyPick{StockCode: "C", Score: 60, FinalScore: 79.5, PostAnalysisCompleted: true}}}
	if got := e.applyRotation(picks, pool, "2026-08-07"); got[1].StockCode != "B" {
		t.Error("D9 默认关闭，选中列表不应变化")
	}

	// 开启：长度不变、第一名稳定、成员来自 picks∪pool、按分数降序
	cfg := DefaultPickEnhanceConfig()
	cfg.EnableRotation = true
	e2 := NewDailyPickEngine().WithEnhanceConfig(cfg)
	picks2 := []models.DailyPick{
		{StockCode: "A", FinalScore: 90, PostAnalysisCompleted: true},
		{StockCode: "B", FinalScore: 80, PostAnalysisCompleted: true},
		{StockCode: "C", FinalScore: 79, PostAnalysisCompleted: true},
		{StockCode: "D", FinalScore: 78.5, PostAnalysisCompleted: true},
	}
	pool2 := []scored{
		{pick: models.DailyPick{StockCode: "E", Score: 60, FinalScore: 78, PostAnalysisCompleted: true}},
		{pick: models.DailyPick{StockCode: "F", Score: 60, FinalScore: 77.8, PostAnalysisCompleted: true}},
	}
	got := e2.applyRotation(picks2, pool2, "2026-08-07")
	if len(got) != len(picks2) {
		t.Fatalf("旋转后长度: got %d, want %d", len(got), len(picks2))
	}
	if got[0].StockCode != "A" {
		t.Error("第一名稳定，永不旋出")
	}
	valid := map[string]bool{"A": true, "B": true, "C": true, "D": true, "E": true, "F": true}
	for i, p := range got {
		if !valid[p.StockCode] {
			t.Errorf("旋入成员应来自 picks∪pool: %s", p.StockCode)
		}
		if i > 0 && got[i-1].FinalScore < p.FinalScore {
			t.Error("旋转后仍应按分数降序")
		}
	}
}

func TestBuildRankCandidate(t *testing.T) {
	pick := models.DailyPick{
		StockCode: "600080.SH", StockName: "测试", ScreenScore: 65,
		ClosePrice: 10, ChangePercent: 2, Amount: 1e8, TurnoverRate: 3, VolumeRatio: 1.5,
		Industry: "半导体", Concepts: `["芯片","国产替代"]`,
		Change60dPct: 12.5, SignalScore: 60, MacdStatus: "bullish", RsiStatus: "neutral",
		FactorScores: `{"momentum":70,"activity":55}`,
	}
	c := buildRankCandidate(&pick)
	if c.Code != "600080.SH" || c.ScreenScore != 65 || c.Industry != "半导体" {
		t.Errorf("基础字段装配错误: %+v", c)
	}
	if len(c.Concepts) != 2 || c.Concepts[0] != "芯片" {
		t.Errorf("Concepts 应解析 JSON 数组: %v", c.Concepts)
	}
	if c.MACDState != "bullish" || c.ChangePct60 != 12.5 {
		t.Errorf("技术面字段装配错误: %+v", c)
	}
	if c.FactorScores["momentum"] != 70 {
		t.Errorf("FactorScores 应解析: %v", c.FactorScores)
	}
}
