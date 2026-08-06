package postanalysis

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func almostEqual(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.001 {
		t.Errorf("%s: got %.4f, want %.4f", name, got, want)
	}
}

// baseCandidate 构造不触发任何条件的基线候选。
func baseCandidate() CandidateInput {
	return CandidateInput{
		Code: "600519", Price: 100, ChangePercent: 2,
		Amount: 1e8, VolumeRatio: 1.0, TurnoverRate: 3,
		PE: 50, PB: 5, SignalScore: 40, LLMConfidence: 0.5, RiskLevel: "medium",
	}
}

// TestScorecardBonuses 6 个加分条件逐一触发。
func TestScorecardBonuses(t *testing.T) {
	s := NewScorecard(DefaultScorecardConfig())
	cases := []struct {
		name   string
		mutate func(in *CandidateInput)
		delta  float64
		detail string
	}{
		{"价值质量", func(in *CandidateInput) { in.PE, in.PB = 20, 2 }, 2.4, "价值质量"},
		{"资金确认", func(in *CandidateInput) { in.Amount, in.VolumeRatio = 3e8, 2.0 }, 1.8, "资金确认"},
		{"控制反转", func(in *CandidateInput) { in.ChangePercent, in.SignalScore = -3, 65 }, 1.2, "控制反转"},
		{"高置信度", func(in *CandidateInput) { in.LLMConfidence = 0.8 }, 0.8, "高置信度"},
		{"催化剂2条", func(in *CandidateInput) { in.Catalysts = []string{"a", "b"} }, 1.0, "催化剂×2"},
		{"催化剂封顶", func(in *CandidateInput) { in.Catalysts = []string{"a", "b", "c", "d", "e"} }, 2.0, "催化剂×5"},
		{"风控低风险", func(in *CandidateInput) { in.RiskLevel = "low" }, 0.6, "风控低风险"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := baseCandidate()
			tc.mutate(&in)
			outcomes, err := s.Analyze(context.Background(), []CandidateInput{in})
			if err != nil {
				t.Fatalf("Analyze 失败: %v", err)
			}
			almostEqual(t, "delta", outcomes[0].Delta, tc.delta)
			if !strings.Contains(outcomes[0].Detail, tc.detail) {
				t.Errorf("明细应含 %q: %q", tc.detail, outcomes[0].Detail)
			}
		})
	}
	// 基线不触发任何条件
	outcomes, _ := s.Analyze(context.Background(), []CandidateInput{baseCandidate()})
	almostEqual(t, "基线 delta", outcomes[0].Delta, 0)
}

// TestScorecardPenalties 3 个减分条件触发 + 加分减分叠加。
func TestScorecardPenalties(t *testing.T) {
	s := NewScorecard(DefaultScorecardConfig())

	in := baseCandidate()
	in.HotMoneyUnstable = true
	out, _ := s.Analyze(context.Background(), []CandidateInput{in})
	almostEqual(t, "热钱不稳", out[0].Delta, -2.5)

	in = baseCandidate()
	in.VolumeRatio = 7
	out, _ = s.Analyze(context.Background(), []CandidateInput{in})
	almostEqual(t, "量比异常", out[0].Delta, -1.2)

	in = baseCandidate()
	in.LLMConfidence = 0.3
	out, _ = s.Analyze(context.Background(), []CandidateInput{in})
	almostEqual(t, "低置信度", out[0].Delta, -1.0)

	// 叠加：资金确认量比区间 1.2-5，量比 7 不触发资金确认而触发量比异常
	in = baseCandidate()
	in.Amount, in.VolumeRatio = 3e8, 2.0 // +1.8
	in.HotMoneyUnstable = true           // -2.5
	out, _ = s.Analyze(context.Background(), []CandidateInput{in})
	almostEqual(t, "加减叠加", out[0].Delta, 1.8-2.5)
}

// TestScorecardClamp ±8 封顶。
func TestScorecardClamp(t *testing.T) {
	s := NewScorecard(DefaultScorecardConfig())

	// 全部 6 个加分：2.4+1.8+1.2+0.8+2.0+0.6 = 8.8 → 封顶 8.0
	in := CandidateInput{
		Code: "A", ChangePercent: -3, Amount: 3e8, VolumeRatio: 2.0,
		PE: 20, PB: 2, SignalScore: 65, LLMConfidence: 0.8,
		Catalysts: []string{"a", "b", "c", "d"}, RiskLevel: "low",
	}
	out, _ := s.Analyze(context.Background(), []CandidateInput{in})
	almostEqual(t, "加分封顶", out[0].Delta, 8.0)

	// 减分侧：热钱-2.5 + 量比-1.2 + 低置信-1.0 = -4.7，调低 MaxDelta 验证对称封顶
	cfg := DefaultScorecardConfig()
	cfg.MaxDelta = 4.0
	s2 := NewScorecard(cfg)
	in2 := baseCandidate()
	in2.HotMoneyUnstable, in2.VolumeRatio, in2.LLMConfidence = true, 7, 0.3
	out2, _ := s2.Analyze(context.Background(), []CandidateInput{in2})
	almostEqual(t, "减分封顶", out2[0].Delta, -4.0)
}

// TestChainAccumulate 链式累加：评分卡 + 远程分析器 delta 相加。
func TestChainAccumulate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"deltas":[{"code":"600519","delta":1.5,"detail":"远程加分"},{"code":"000001","delta":-0.5}]}`))
	}))
	defer server.Close()

	cfg := DefaultScorecardConfig()
	chain := NewChain(
		NewScorecard(cfg),
		NewRemoteAnalyzer("remote", server.URL, 0, server.Client()),
	)
	candidates := []CandidateInput{
		{Code: "600519", PE: 20, PB: 2, Amount: 3e8, VolumeRatio: 2, LLMConfidence: 0.5, RiskLevel: "low"}, // 评分卡 2.4+1.8+0.6=4.8
		{Code: "000001", PE: 50, PB: 5, LLMConfidence: 0.5},                                                // 评分卡 0
	}
	results := chain.Analyze(context.Background(), candidates)
	if len(results) != 2 {
		t.Fatalf("结果数: %d", len(results))
	}
	almostEqual(t, "600519 评分卡", results[0].Deltas["local_scorecard"], 4.8)
	almostEqual(t, "600519 远程", results[0].Deltas["remote"], 1.5)
	almostEqual(t, "600519 总调整", results[0].TotalDelta, 6.3)
	almostEqual(t, "000001 总调整", results[1].TotalDelta, -0.5)
	if results[0].Status != StatusComplete || results[1].Status != StatusComplete {
		t.Errorf("状态: %s / %s", results[0].Status, results[1].Status)
	}
	if results[0].Details["remote"] != "远程加分" {
		t.Errorf("远程明细: %q", results[0].Details["remote"])
	}
}

// failAnalyzer 永远失败的分析器。
type failAnalyzer struct{ name string }

func (f failAnalyzer) Name() string { return f.name }
func (f failAnalyzer) Analyze(ctx context.Context, candidates []CandidateInput) ([]AnalyzerOutcome, error) {
	return nil, errors.New("boom")
}

// TestChainFailureContinues 单分析器失败不中断链：标记 failed，后续分析器继续。
func TestChainFailureContinues(t *testing.T) {
	cfg := DefaultScorecardConfig()
	chain := NewChain(
		failAnalyzer{name: "broken"},
		NewScorecard(cfg),
	)
	results := chain.Analyze(context.Background(), []CandidateInput{{Code: "600519", PE: 20, PB: 2, LLMConfidence: 0.5}})
	r := results[0]
	if r.Status != StatusPartial {
		t.Errorf("应标记 partial: %s", r.Status)
	}
	if r.Errors["broken"] == "" {
		t.Error("应记录失败原因")
	}
	// 后续分析器照常执行
	almostEqual(t, "后续照常执行", r.Deltas["local_scorecard"], 2.4)
	almostEqual(t, "总调整只含成功者", r.TotalDelta, 2.4)
}

// TestChainResultCountMismatch 分析器返回数量不一致视为失败。
func TestChainResultCountMismatch(t *testing.T) {
	bad := &stubAnalyzer{outcomes: []AnalyzerOutcome{{Delta: 1}}} // 1 个结果 vs 2 个候选
	chain := NewChain(bad)
	results := chain.Analyze(context.Background(), []CandidateInput{{Code: "A"}, {Code: "B"}})
	if results[0].Status != StatusPartial || results[1].Status != StatusPartial {
		t.Error("数量不一致应标记失败")
	}
}

type stubAnalyzer struct{ outcomes []AnalyzerOutcome }

func (s *stubAnalyzer) Name() string { return "stub" }
func (s *stubAnalyzer) Analyze(ctx context.Context, candidates []CandidateInput) ([]AnalyzerOutcome, error) {
	return s.outcomes, nil
}

// TestRemoteAnalyzer HTTP 分析器：成功/非200/坏JSON/超时。
func TestRemoteAnalyzer(t *testing.T) {
	candidates := []CandidateInput{{Code: "600519"}, {Code: "000001"}}

	// 成功（含未覆盖候选：远程未返回 000001 → delta 0 不视为失败）
	t.Run("成功", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("应为 POST, got %s", r.Method)
			}
			w.Write([]byte(`{"deltas":[{"code":"600519","delta":2.0,"detail":"ok"}]}`))
		}))
		defer server.Close()
		a := NewRemoteAnalyzer("remote", server.URL, 0, server.Client())
		out, err := a.Analyze(context.Background(), candidates)
		if err != nil {
			t.Fatalf("Analyze 失败: %v", err)
		}
		almostEqual(t, "覆盖候选", out[0].Delta, 2.0)
		almostEqual(t, "未覆盖候选", out[1].Delta, 0)
	})

	// 非 200
	t.Run("非200", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()
		a := NewRemoteAnalyzer("remote", server.URL, 0, server.Client())
		if _, err := a.Analyze(context.Background(), candidates); err == nil ||
			!strings.Contains(err.Error(), "非 200") {
			t.Errorf("应返回非200错误: %v", err)
		}
	})

	// 坏 JSON
	t.Run("坏JSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`not json`))
		}))
		defer server.Close()
		a := NewRemoteAnalyzer("remote", server.URL, 0, server.Client())
		if _, err := a.Analyze(context.Background(), candidates); err == nil ||
			!strings.Contains(err.Error(), "解析失败") {
			t.Errorf("应返回解析错误: %v", err)
		}
	})

	// 超时（服务端睡眠超过客户端超时）
	t.Run("超时", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(200 * time.Millisecond)
			w.Write([]byte(`{"deltas":[]}`))
		}))
		defer server.Close()
		client := &http.Client{Timeout: 50 * time.Millisecond}
		a := NewRemoteAnalyzer("remote", server.URL, 0, client)
		if _, err := a.Analyze(context.Background(), candidates); err == nil {
			t.Error("应超时失败")
		}
	})
}

// TestScorecardConfigJSON 配置加载。
func TestScorecardConfigJSON(t *testing.T) {
	cfg, err := LoadScorecardConfigJSON([]byte(`{}`))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	almostEqual(t, "默认价值加分", cfg.ValueBonus, 2.4)
	almostEqual(t, "默认上限", cfg.MaxDelta, 8.0)

	cfg, err = LoadScorecardConfigJSON([]byte(`{"maxDelta": 5, "valueBonus": 3}`))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	almostEqual(t, "覆盖上限", cfg.MaxDelta, 5)
	almostEqual(t, "覆盖加分", cfg.ValueBonus, 3)
	almostEqual(t, "其余保留默认", cfg.FundBonus, 1.8)

	if _, err = LoadScorecardConfigJSON([]byte(`{invalid`)); err == nil {
		t.Error("非法 JSON 应返回错误")
	}
}
