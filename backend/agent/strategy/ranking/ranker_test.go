package ranking

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
)

func almostEqual(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.001 {
		t.Errorf("%s: got %.4f, want %.4f", name, got, want)
	}
}

// ---------- Formatter ----------

func TestFormatCandidates(t *testing.T) {
	pool := FormatCandidates([]Candidate{{
		Code: "600519", Name: "贵州茅台", ScreenScore: 66.8,
		Price: 1700, ChangePercent: 1.2, Amount: 4.52e9, TurnoverRate: 0.35,
		VolumeRatio: 1.1, TotalCap: 2.135e12, PE: 28.5, PB: 9.8,
		Industry: "白酒", Concepts: []string{"消费", "白酒"}, IndustryRank: 1, IndustryChangePct: 0.8,
		HeatLatest: 72, HeatTrend: 3, HeatPersistenceDays: 5, HeatWatchCount: 120, HeatState: "升温",
		ChangePct60: 8.5, SignalScore: 65, MACDState: "bullish", RSIState: "neutral",
		PullbackMA20: true, Volatility: 18, MaxDrawdown: -5.2, ATR: 25.3,
		FactorScores:        map[string]float64{"value": 67, "momentum": 59.2},
		NewsTitles:          []string{"茅台三季报超预期"},
		FundamentalsCovered: true,
	}})

	for _, want := range []string{
		"候选 1: 600519 贵州茅台（综合分 66.80）",
		"价格 1700.00", "涨跌幅 +1.20%", "成交额 45.2亿", "换手率 0.35%", "量比 1.10",
		"PE 28.5", "PB 9.8",
		"行业 白酒(行业排名 1, 行业涨跌 +0.80%)", "概念 消费、白酒",
		"板块热度: 最新 72", "趋势 +3", "持续 5 天", "观察 120", "状态 升温",
		"60日涨跌 +8.5%", "信号分 65", "MACD bullish", "RSI neutral", "回踩MA20", "回撤 -5.2%",
		"因子评分: value 67 momentum 59",
		"新闻: 1) 茅台三季报超预期",
		"基本面: 已覆盖",
	} {
		if !strings.Contains(pool, want) {
			t.Errorf("格式化输出缺少 %q", want)
		}
	}

	// 零值可选字段不输出
	minimal := FormatCandidates([]Candidate{{Code: "000001", ScreenScore: 50}})
	for _, absent := range []string{"行情:", "板块热度", "技术面", "因子评分", "新闻", "基本面"} {
		if strings.Contains(minimal, absent) {
			t.Errorf("零值候选不应输出 %q", absent)
		}
	}
}

func TestBuildRankPrompt(t *testing.T) {
	prompt := BuildRankPrompt("候选 1: 600519 ...")
	for _, want := range []string{
		"market_view", "selection_logic", "portfolio_risk",
		"llm_score", "confidence", "thesis", "risk_flags", "invalidators",
		"候选 1: 600519 ...",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("Prompt 缺少 %q", want)
		}
	}
}

// ---------- RepairJSON ----------

func TestRepairJSON(t *testing.T) {
	cases := []struct {
		name  string
		input string
		check func(t *testing.T, out string)
	}{
		{
			name:  "尾逗号对象",
			input: `{"market_view": "震荡", "ranked": [],}`,
			check: func(t *testing.T, out string) {
				var v map[string]any
				if err := json.Unmarshal([]byte(out), &v); err != nil {
					t.Errorf("尾逗号修复后仍无法解析: %v", err)
				}
			},
		},
		{
			name:  "尾逗号数组",
			input: `{"ranked": [{"code": "600519",},],}`,
			check: func(t *testing.T, out string) {
				if !json.Valid([]byte(out)) {
					t.Errorf("数组尾逗号修复失败: %s", out)
				}
			},
		},
		{
			name:  "未闭合括号",
			input: `{"market_view": "x", "ranked": [{"code": "600519", "llm_score": 82}]`,
			check: func(t *testing.T, out string) {
				if !json.Valid([]byte(out)) {
					t.Errorf("括号闭合失败: %s", out)
				}
			},
		},
		{
			name:  "多对象只保留第一个",
			input: `{"a": 1}{"b": 2}`,
			check: func(t *testing.T, out string) {
				if out != `{"a": 1}` {
					t.Errorf("多对象恢复: got %q", out)
				}
			},
		},
		{
			name:  "部分恢复丢弃截断元素",
			input: `{"market_view": "x", "ranked": [{"code": "600519", "llm_score": 82}, {"code": "000001", "llm_s`,
			check: func(t *testing.T, out string) {
				var v struct {
					Ranked []struct {
						Code string `json:"code"`
					} `json:"ranked"`
				}
				if err := json.Unmarshal([]byte(out), &v); err != nil {
					t.Fatalf("部分恢复后无法解析: %v (out=%s)", err, out)
				}
				if len(v.Ranked) != 1 || v.Ranked[0].Code != "600519" {
					t.Errorf("部分恢复应只保留完整元素 600519, got %+v", v.Ranked)
				}
			},
		},
		{
			name:  "markdown 围栏与前导文字",
			input: "好的，分析如下：\n```json\n{\"a\": 1}\n```",
			check: func(t *testing.T, out string) {
				if out != `{"a": 1}` {
					t.Errorf("围栏剥离: got %q", out)
				}
			},
		},
		{
			name: "尾部杂散文本",
			input: `{"a": 1}
以上是分析结果。`,
			check: func(t *testing.T, out string) {
				if out != `{"a": 1}` {
					t.Errorf("尾部文本剥离: got %q", out)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, RepairJSON(tc.input))
		})
	}
}

// ---------- Ranker ----------

// okResponse 构造覆盖全部 code 的合法 LLM 输出。
func okResponse(scores map[string]float64) string {
	type item struct {
		Code       string  `json:"code"`
		LLMScore   float64 `json:"llm_score"`
		Confidence float64 `json:"confidence"`
		Thesis     string  `json:"thesis"`
		Risk       string  `json:"risk"`
	}
	items := []item{}
	for code, s := range scores {
		items = append(items, item{Code: code, LLMScore: s, Confidence: 0.8, Thesis: "论点", Risk: "风险"})
	}
	b, _ := json.Marshal(map[string]any{
		"market_view": "整体震荡", "selection_logic": "优选低估值", "portfolio_risk": "行业集中",
		"ranked": items,
	})
	return string(b)
}

func threeCandidates() []Candidate {
	return []Candidate{
		{Code: "600519", Name: "贵州茅台", ScreenScore: 80},
		{Code: "000001", Name: "平安银行", ScreenScore: 70},
		{Code: "300750", Name: "宁德时代", ScreenScore: 60},
	}
}

// TestRankNormal 正常排序 + 分数混合。
func TestRankNormal(t *testing.T) {
	mock := func(ctx context.Context, model, prompt string) (string, error) {
		return okResponse(map[string]float64{"600519": 90, "000001": 60, "300750": 75}), nil
	}
	r := NewRanker(DefaultRankerConfig()).Rank(context.Background(), threeCandidates(), []string{"main"}, mock)

	if r.Degraded {
		t.Fatal("不应退化")
	}
	if r.Model != "main" {
		t.Errorf("生效模型: got %q", r.Model)
	}
	almostEqual(t, "覆盖率", r.Coverage, 1.0)
	if r.MarketView != "整体震荡" || r.SelectionLogic != "优选低估值" || r.PortfolioRisk != "行业集中" {
		t.Errorf("顶层字段缺失: %+v", r)
	}
	if len(r.Stocks) != 3 {
		t.Fatalf("应返回 3 只股票, got %d", len(r.Stocks))
	}

	// 分数混合: final = screen×0.6 + llm×0.4
	scores := map[string]RankedStock{}
	for _, s := range r.Stocks {
		scores[s.Code] = s
	}
	almostEqual(t, "600519 混合分", scores["600519"].FinalScore, 80*0.6+90*0.4) // 84
	almostEqual(t, "000001 混合分", scores["000001"].FinalScore, 70*0.6+60*0.4) // 66
	almostEqual(t, "300750 混合分", scores["300750"].FinalScore, 60*0.6+75*0.4) // 66
	if !scores["600519"].Covered || scores["600519"].Thesis != "论点" {
		t.Errorf("LLM 字段未对齐: %+v", scores["600519"])
	}

	// 排序：84 第一；66 并列时稳定序（000001 在前）
	if r.Stocks[0].Code != "600519" || r.Stocks[1].Code != "000001" || r.Stocks[2].Code != "300750" {
		t.Errorf("排序结果: %v", []string{r.Stocks[0].Code, r.Stocks[1].Code, r.Stocks[2].Code})
	}
}

// TestRankModelChainFallback 模型链降级：主模型失败 → 备选成功，链去重，重试次数受限。
func TestRankModelChainFallback(t *testing.T) {
	calls := map[string]int{}
	mock := func(ctx context.Context, model, prompt string) (string, error) {
		calls[model]++
		if model == "main" {
			return "", errors.New("429 rate limit")
		}
		return okResponse(map[string]float64{"600519": 90, "000001": 60, "300750": 75}), nil
	}
	// "main" 重复出现应被去重
	chain := []string{"main", "main", "backup"}
	r := NewRanker(DefaultRankerConfig()).Rank(context.Background(), threeCandidates(), chain, mock)

	if r.Degraded {
		t.Fatal("备选模型成功时不应退化")
	}
	if r.Model != "backup" {
		t.Errorf("应降级到 backup, got %q", r.Model)
	}
	// main: 去重后 1 个模型 × (1 次调用 + 1 次重试) = 2 次；backup 首次成功 = 1 次
	if calls["main"] != 2 {
		t.Errorf("主模型应调用 2 次(含重试), got %d", calls["main"])
	}
	if calls["backup"] != 1 {
		t.Errorf("备选模型应调用 1 次, got %d", calls["backup"])
	}
}

// TestRankRetryThenSuccess 首次输出非法 JSON，重试后成功。
func TestRankRetryThenSuccess(t *testing.T) {
	calls := 0
	mock := func(ctx context.Context, model, prompt string) (string, error) {
		calls++
		if calls == 1 {
			return "这不是 JSON", nil
		}
		return okResponse(map[string]float64{"600519": 90, "000001": 60, "300750": 75}), nil
	}
	r := NewRanker(DefaultRankerConfig()).Rank(context.Background(), threeCandidates(), []string{"main"}, mock)
	if r.Degraded {
		t.Fatal("重试成功后不应退化")
	}
	if calls != 2 {
		t.Errorf("应调用 2 次, got %d", calls)
	}
}

// TestRankLowCoverageDegrades 覆盖率不足 → 换模型/最终退化。
func TestRankLowCoverageDegrades(t *testing.T) {
	// 3 只候选只覆盖 1 只 → 覆盖率 0.33 < 0.60
	mock := func(ctx context.Context, model, prompt string) (string, error) {
		return okResponse(map[string]float64{"600519": 90}), nil
	}
	r := NewRanker(DefaultRankerConfig()).Rank(context.Background(), threeCandidates(), []string{"main"}, mock)
	if !r.Degraded {
		t.Error("覆盖率不足应退化")
	}
	almostEqual(t, "退化覆盖率", r.Coverage, 0)
	// 退化排序按 screen_score 降序
	if r.Stocks[0].Code != "600519" || r.Stocks[1].Code != "000001" || r.Stocks[2].Code != "300750" {
		t.Errorf("退化排序: %v", []string{r.Stocks[0].Code, r.Stocks[1].Code, r.Stocks[2].Code})
	}
	for _, s := range r.Stocks {
		almostEqual(t, "退化 final=screen", s.FinalScore, s.ScreenScore)
	}
}

// TestRankPartialCoverage 覆盖率达标但部分覆盖：未覆盖候选保持 screen_score。
func TestRankPartialCoverage(t *testing.T) {
	// 3 只覆盖 2 只 → 0.67 ≥ 0.60
	mock := func(ctx context.Context, model, prompt string) (string, error) {
		return okResponse(map[string]float64{"000001": 95, "300750": 50}), nil
	}
	r := NewRanker(DefaultRankerConfig()).Rank(context.Background(), threeCandidates(), []string{"main"}, mock)
	if r.Degraded {
		t.Fatal("覆盖率达标不应退化")
	}
	almostEqual(t, "覆盖率", r.Coverage, 2.0/3.0)
	byCode := map[string]RankedStock{}
	for _, s := range r.Stocks {
		byCode[s.Code] = s
	}
	if byCode["600519"].Covered {
		t.Error("600519 未被覆盖")
	}
	almostEqual(t, "未覆盖保持 screen", byCode["600519"].FinalScore, 80)
	almostEqual(t, "覆盖混合", byCode["000001"].FinalScore, 70*0.6+95*0.4) // 80
}

// TestRankAllModelsFail 全部模型失败 → 退化。
func TestRankAllModelsFail(t *testing.T) {
	mock := func(ctx context.Context, model, prompt string) (string, error) {
		return "", errors.New("upstream down")
	}
	r := NewRanker(DefaultRankerConfig()).Rank(context.Background(), threeCandidates(), []string{"a", "b"}, mock)
	if !r.Degraded {
		t.Error("全部失败应退化")
	}
	if r.Model != "" {
		t.Errorf("退化时 Model 应为空, got %q", r.Model)
	}
}

// TestRankEmptyCandidates 空候选池。
func TestRankEmptyCandidates(t *testing.T) {
	r := NewRanker(DefaultRankerConfig()).Rank(context.Background(), nil, []string{"main"},
		func(ctx context.Context, model, prompt string) (string, error) { return "{}", nil })
	if !r.Degraded || len(r.Stocks) != 0 {
		t.Errorf("空候选池应返回空退化结果, got %+v", r)
	}
}

// TestRankConfigJSON 配置加载。
func TestRankConfigJSON(t *testing.T) {
	cfg, err := LoadRankerConfigJSON([]byte(`{}`))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	almostEqual(t, "默认 rank_weight", cfg.RankWeight, 0.40)
	almostEqual(t, "默认覆盖率阈值", cfg.CoverageThreshold, 0.60)
	if cfg.MaxRetries != 1 {
		t.Errorf("默认重试次数: got %d", cfg.MaxRetries)
	}

	cfg, err = LoadRankerConfigJSON([]byte(`{"rankWeight": 0.5}`))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	almostEqual(t, "覆盖 rank_weight", cfg.RankWeight, 0.5)
	almostEqual(t, "其余保留默认", cfg.CoverageThreshold, 0.60)

	if _, err = LoadRankerConfigJSON([]byte(`{invalid`)); err == nil {
		t.Error("非法 JSON 应返回错误")
	}
}

// TestRankerConfigClamp 越界配置应被收敛到合法区间。
func TestRankerConfigClamp(t *testing.T) {
	cfg, err := LoadRankerConfigJSON([]byte(`{"rankWeight": 1.5, "coverageThreshold": -0.2}`))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	almostEqual(t, "rankWeight 上限收敛", cfg.RankWeight, 1.0)
	almostEqual(t, "coverageThreshold 下限收敛", cfg.CoverageThreshold, 0.0)

	cfg, err = LoadRankerConfigJSON([]byte(`{"rankWeight": -0.3}`))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	almostEqual(t, "rankWeight 下限收敛", cfg.RankWeight, 0.0)

	r := NewRanker(RankerConfig{RankWeight: 2.0})
	almostEqual(t, "NewRanker 收敛 rankWeight", r.Config.RankWeight, 1.0)
}
