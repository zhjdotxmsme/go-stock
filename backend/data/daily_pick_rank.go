package data

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"go-stock/backend/agent/strategy/postanalysis"
	"go-stock/backend/agent/strategy/ranking"
	"go-stock/backend/agent/strategy/risk"
	"go-stock/backend/agent/strategy/scoring"
	"go-stock/backend/logger"
	"go-stock/backend/models"

	"github.com/go-resty/resty/v2"
)

// ===== A2：D2 LLM 二次排序 + D3 风控（排序后）+ D10 后分析 + D9 种子旋转 =====
// 管线顺序（DSA）：D7 过滤 → D1 评分 → D2 LLM 排序 → D3 风控标记 → D10 后分析 → （可选 D9 旋转）

// parseJSONStringArray 解析 D12 type:text 字段中的 JSON 数组字符串；空/非法返回 nil。
func parseJSONStringArray(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil
	}
	return out
}

// ===== LLM 模型链装配（参考 CallLLMForConfig 的 AI 配置获取方式）=====

// llmModelChain 按设置中 AI 配置的排列顺序（首个=主模型）构建模型链。
// 无 AI 配置或查询失败时返回 nil（调用方静默跳过 LLM 排序）。
func (e *DailyPickEngine) llmModelChain() (chain []string, call ranking.LLMCallFunc) {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Warnf("daily_pick: build LLM model chain failed: %v", r)
			chain, call = nil, nil
		}
	}()
	cfg := GetSettingConfig()
	if cfg == nil || len(cfg.AiConfigs) == 0 {
		return nil, nil
	}
	for _, ac := range cfg.AiConfigs {
		if ac != nil && ac.ModelName != "" {
			chain = append(chain, ac.ModelName)
		}
	}
	if len(chain) == 0 {
		return nil, nil
	}
	return chain, buildLLMChatCall(cfg.AiConfigs)
}

// buildLLMChatCall 由 AI 配置列表装配 ranking.LLMCallFunc：
// 按模型名找到对应配置，走 OpenAI 兼容 /chat/completions（与 CallLLMForConfig 同一模式）。
func buildLLMChatCall(aiConfigs []*AIConfig) ranking.LLMCallFunc {
	byModel := make(map[string]*AIConfig, len(aiConfigs))
	for _, ac := range aiConfigs {
		if ac != nil && ac.ModelName != "" {
			byModel[ac.ModelName] = ac
		}
	}
	return func(ctx context.Context, model, prompt string) (string, error) {
		ac := byModel[model]
		if ac == nil {
			return "", fmt.Errorf("no AI config for model %q", model)
		}

		timeout := time.Duration(ac.TimeOut) * time.Second
		if timeout <= 0 {
			timeout = 30 * time.Second // 排序 prompt 较大，超时比配置生成（15s）宽
		}
		client := resty.New().SetTimeout(timeout)
		if ac.HttpProxyEnabled && ac.HttpProxy != "" {
			client.SetProxy(ac.HttpProxy)
		}

		body := map[string]any{
			"model": model,
			"messages": []map[string]string{
				{"role": "system", "content": "You are a stock ranking expert. Output ONLY valid JSON matching the specified schema. No explanation, no markdown."},
				{"role": "user", "content": prompt},
			},
			"temperature": 0.1,
			"max_tokens":  4096,
		}
		if supportsJSONMode(model) {
			body["response_format"] = map[string]string{"type": "json_object"}
		}

		resp, err := client.R().
			SetContext(ctx).
			SetHeader("Content-Type", "application/json").
			SetHeader("Authorization", "Bearer "+ac.ApiKey).
			SetBody(body).
			Post(strings.TrimRight(ac.BaseUrl, "/") + "/chat/completions")
		if err != nil {
			return "", fmt.Errorf("LLM request failed: %w", err)
		}
		if resp.StatusCode() != 200 {
			return "", fmt.Errorf("LLM returned status %d", resp.StatusCode())
		}
		var chatResp struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(resp.Body(), &chatResp); err != nil {
			return "", fmt.Errorf("parse LLM response: %w", err)
		}
		if len(chatResp.Choices) == 0 {
			return "", fmt.Errorf("LLM returned empty choices")
		}
		return chatResp.Choices[0].Message.Content, nil
	}
}

// ===== D2 LLM 二次排序 =====

// applyLLMRanking 对 D1 评分结果执行 LLM 二次排序：
// FinalScore = screen×0.6 + llm×0.4，llm 丰富字段写回 D12。
// 返回 true 表示结果已按 FinalScore 重排（调用方跳旧 Score 排序）；
// 无 AI 配置/LLM 失败/降级时返回 false，保持 screen 序（行为同旧版）。
func (e *DailyPickEngine) applyLLMRanking(ctx context.Context, results []scored) (ranked bool) {
	cfg := e.enhanceCfg.normalize()
	// 设置页开关（nil = 未设置，沿用 enhanceCfg 默认值）
	if sc := GetSettingConfig(); sc != nil && sc.EnableLLMRanking != nil {
		cfg.EnableLLMRanking = *sc.EnableLLMRanking
	}
	if !cfg.EnableLLMRanking {
		return false
	}
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("daily_pick: LLM ranking panic: %v, keep screen order", r)
			ranked = false
		}
	}()

	// 取 ScreenScore>0 的成功结果，按 ScreenScore 降序截取前 N 送入 LLM
	var idxs []int
	for i := range results {
		if results[i].err == nil && results[i].pick.ScreenScore > 0 {
			idxs = append(idxs, i)
		}
	}
	if len(idxs) == 0 {
		return false
	}
	sort.SliceStable(idxs, func(a, b int) bool {
		return results[idxs[a]].pick.ScreenScore > results[idxs[b]].pick.ScreenScore
	})
	maxN := cfg.LLMRankMaxCandidates
	if maxN <= 0 {
		maxN = 30
	}
	rankIdxs := idxs
	if len(rankIdxs) > maxN {
		rankIdxs = rankIdxs[:maxN]
	}

	call, chain := cfg.LLMCall, cfg.ModelChain
	if call == nil || len(chain) == 0 {
		if e.modelChainFn != nil {
			chain, call = e.modelChainFn()
		} else {
			chain, call = e.llmModelChain()
		}
	}
	if call == nil || len(chain) == 0 {
		logger.SugaredLogger.Info("daily_pick: no AI config available, skip LLM ranking (keep screen order)")
		return false
	}

	candidates := make([]ranking.Candidate, 0, len(rankIdxs))
	for _, i := range rankIdxs {
		candidates = append(candidates, buildRankCandidate(&results[i].pick))
	}

	// 排序 Prompt 支持在提示词管理页编辑：role_key=d2_ranking。
	// 首次读取时把默认模板写入 DB（Upsert 幂等），之后一直用页面上的版本。
	if tpl := GetPromptByRoleKey("d2_ranking"); tpl != "" {
		cfg.Ranker.PromptTemplate = tpl
	} else {
		UpsertPromptByRoleKey("d2_ranking", "每日选股LLM排序", ranking.DefaultRankPromptTemplate(), "multi_agent")
	}

	res := ranking.NewRanker(cfg.Ranker).Rank(ctx, candidates, chain, call)
	if res.Degraded {
		logger.SugaredLogger.Warn("daily_pick: LLM ranking degraded (all models failed), keep screen order")
		return false
	}
	logger.SugaredLogger.Infof("daily_pick: LLM ranking ok (model=%s coverage=%.0f%%): market_view=%s | selection_logic=%s | portfolio_risk=%s",
		res.Model, res.Coverage*100, res.MarketView, res.SelectionLogic, res.PortfolioRisk)

	// 写回 D12 字段
	byCode := make(map[string]ranking.RankedStock, len(res.Stocks))
	for _, rs := range res.Stocks {
		byCode[rs.Code] = rs
	}
	for _, i := range rankIdxs {
		rs, ok := byCode[results[i].pick.StockCode]
		if !ok {
			continue
		}
		writeRankedFields(&results[i].pick, &rs)
	}

	// 重排：参与排序的子集按 FinalScore 降序，未参与的保持原相对顺序
	sort.SliceStable(rankIdxs, func(a, b int) bool {
		return results[rankIdxs[a]].pick.FinalScore > results[rankIdxs[b]].pick.FinalScore
	})
	inRank := make(map[int]bool, len(rankIdxs))
	for _, i := range rankIdxs {
		inRank[i] = true
	}
	reordered := make([]scored, 0, len(results))
	for _, i := range rankIdxs {
		reordered = append(reordered, results[i])
	}
	for i := range results {
		if !inRank[i] {
			reordered = append(reordered, results[i])
		}
	}
	copy(results, reordered)
	return true
}

// buildRankCandidate 从 DailyPick 装配 ranking.Candidate（填不了的字段留零，
// omitempty 使其不参与 prompt 格式化）。
func buildRankCandidate(pick *models.DailyPick) ranking.Candidate {
	c := ranking.Candidate{
		Code:        pick.StockCode,
		Name:        pick.StockName,
		ScreenScore: pick.ScreenScore,

		Price:         pick.ClosePrice,
		ChangePercent: pick.ChangePercent,
		Amount:        pick.Amount,
		TurnoverRate:  pick.TurnoverRate,
		VolumeRatio:   pick.VolumeRatio,

		Industry: pick.Industry,
		Concepts: parseJSONStringArray(pick.Concepts),

		ChangePct60:       pick.Change60dPct,
		SignalScore:       pick.SignalScore,
		MACDState:         pick.MacdStatus,
		RSIState:          pick.RsiStatus,
		BreakoutPct:       pick.Breakout20dPct,
		AmplitudePct:      pick.AmplitudePct,
		VolumeRatio20:     pick.VolumeRatio20d,
		BodyPct:           pick.BodyPct,
		PullbackMA20:      pick.PullbackMa20,
		ConsolidationDays: pick.ConsolidationDays,
		Volatility:        pick.Volatility20dPct,
		MaxDrawdown:       pick.MaxDrawdownPct,
		ATR:               pick.Atr14,
	}
	if pick.FactorScores != "" {
		var fs map[string]float64
		if err := json.Unmarshal([]byte(pick.FactorScores), &fs); err == nil {
			c.FactorScores = fs
		}
	}
	return c
}

// writeRankedFields 把 LLM 排序结果写回 D12 字段（列表字段序列化为 JSON 字符串）。
func writeRankedFields(pick *models.DailyPick, rs *ranking.RankedStock) {
	pick.FinalScore = math.Round(rs.FinalScore*100) / 100
	if !rs.Covered {
		return
	}
	pick.LlmScore = math.Round(rs.LLMScore*100) / 100
	pick.LlmConfidence = rs.Confidence
	pick.LlmSector = rs.Sector
	pick.LlmTheme = rs.Theme
	pick.LlmThesis = rs.Thesis
	pick.LlmReason = rs.Reason
	pick.LlmRisk = rs.Risk
	pick.LlmStyleFit = rs.StyleFit
	if rs.Reason != "" {
		pick.RankingReason = rs.Reason
	}
	if data, err := json.Marshal(rs.Catalysts); err == nil && len(rs.Catalysts) > 0 {
		pick.LlmCatalysts = string(data)
	}
	if data, err := json.Marshal(rs.RiskFlags); err == nil && len(rs.RiskFlags) > 0 {
		pick.LlmRiskFlags = string(data)
	}
	if data, err := json.Marshal(rs.Tags); err == nil && len(rs.Tags) > 0 {
		pick.LlmTags = string(data)
	}
	if data, err := json.Marshal(rs.WatchItems); err == nil && len(rs.WatchItems) > 0 {
		pick.LlmWatchItems = string(data)
	}
	if data, err := json.Marshal(rs.Invalidators); err == nil && len(rs.Invalidators) > 0 {
		pick.LlmInvalidators = string(data)
	}
}

// ===== D3 风控叠加（DSA 顺序：排序之后，LLM 风险标记参与扣分）=====

// applyRiskToResults 对成功结果执行 D3 风控叠加（A1 逻辑，自 enhanceResults 拆出）。
// 默认只标记不剔除：高风险票 ExcludedByRisk=true 但仍在结果中。
func (e *DailyPickEngine) applyRiskToResults(results []scored) []scored {
	cfg := e.enhanceCfg.normalize()
	if !cfg.EnableRisk {
		return results
	}
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("daily_pick: risk overlay panic: %v", r)
		}
	}()

	overlay := risk.NewRiskOverlay(cfg.RiskProfile)
	for i := range results {
		if results[i].err != nil {
			continue
		}
		applyRiskOverlay(&results[i].pick, overlay)
	}
	return results
}

// ===== D10 后分析链 =====

// applyPostAnalysis 执行 D10 后分析链（本地评分卡常驻；RemoteAnalyzerURL 非空时
// 追加远程分析器）。delta 写入 D12 后分析字段；ApplyPostAnalysisDelta=true 时
// FinalScore = clamp(FinalScore + TotalDelta, 0, 100)。
func (e *DailyPickEngine) applyPostAnalysis(ctx context.Context, results []scored) []scored {
	cfg := e.enhanceCfg.normalize()
	if !cfg.EnablePostAnalysis {
		return results
	}
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("daily_pick: post-analysis panic: %v", r)
		}
	}()

	var idxs []int
	inputs := make([]postanalysis.CandidateInput, 0, len(results))
	for i := range results {
		pick := &results[i].pick
		if results[i].err != nil || pick.Score <= 0 {
			continue
		}
		idxs = append(idxs, i)
		inputs = append(inputs, postanalysis.CandidateInput{
			Code:          pick.StockCode,
			Name:          pick.StockName,
			Price:         pick.ClosePrice,
			ChangePercent: pick.ChangePercent,
			Amount:        pick.Amount,
			VolumeRatio:   pick.VolumeRatio,
			TurnoverRate:  pick.TurnoverRate,
			SignalScore:   pick.SignalScore,
			LLMConfidence: pick.LlmConfidence,
			Catalysts:     parseJSONStringArray(pick.LlmCatalysts),
			RiskLevel:     pick.RiskLevel,
			// PE/PB/HotMoneyUnstable 数据缺失，留零（对应评分卡条件不触发）
		})
	}
	if len(idxs) == 0 {
		return results
	}

	analyzers := []postanalysis.PostAnalyzer{postanalysis.NewScorecard(cfg.Scorecard)}
	remoteName := ""
	if cfg.RemoteAnalyzerURL != "" {
		remote := postanalysis.NewRemoteAnalyzer("remote_dsa", cfg.RemoteAnalyzerURL, 0, nil)
		analyzers = append(analyzers, remote)
		remoteName = remote.Name()
	}

	paResults := postanalysis.NewChain(analyzers...).Analyze(ctx, inputs)
	for k, i := range idxs {
		r := paResults[k]
		pick := &results[i].pick
		pick.PostAnalysisStatus = r.Status
		pick.PostAnalysisCompleted = r.Status == postanalysis.StatusComplete
		pick.PostAnalysisScoreDelta = math.Round(r.TotalDelta*100) / 100
		pick.PostAnalysisLocalDelta = math.Round(r.Deltas["local_scorecard"]*100) / 100
		if remoteName != "" {
			pick.PostAnalysisRemoteDelta = math.Round(r.Deltas[remoteName]*100) / 100
		}
		if data, err := json.Marshal(r); err == nil {
			pick.PostAnalysisDetail = string(data)
		}
		if cfg.ApplyPostAnalysisDelta && pick.FinalScore > 0 && r.TotalDelta != 0 {
			pick.FinalScore = math.Round(clampFloat(pick.FinalScore+r.TotalDelta, 0, 100)*100) / 100
		}
	}
	return results
}

// ===== D9 种子旋转（默认关闭）=====

// applyRotation 对截断后的最终选中结果做种子化旋转（只换成员资格，不换相对排名）。
// pool 为落选的成功结果。默认关闭；开启后 seed=RotationSeed（默认 daily_pick），
// period=tradeDate。任何异常回落原选中列表。
func (e *DailyPickEngine) applyRotation(picks []models.DailyPick, results []scored, tradeDate string) []models.DailyPick {
	cfg := e.enhanceCfg.normalize()
	if !cfg.EnableRotation || len(picks) < 2 {
		return picks
	}
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("daily_pick: rotation panic: %v", r)
		}
	}()

	scoreOf := func(p *models.DailyPick) float64 {
		if p.FinalScore > 0 {
			return p.FinalScore
		}
		return p.Score
	}
	selected := make([]scoring.ScoredCandidate, 0, len(picks))
	inPicks := make(map[string]bool, len(picks))
	for i := range picks {
		selected = append(selected, scoring.ScoredCandidate{
			Code: picks[i].StockCode, Score: scoreOf(&picks[i]),
			PostAnalysisCompleted: picks[i].PostAnalysisCompleted,
		})
		inPicks[picks[i].StockCode] = true
	}
	var pool []scoring.ScoredCandidate
	for i := range results {
		p := &results[i].pick
		if results[i].err != nil || p.Score <= 0 || inPicks[p.StockCode] {
			continue
		}
		pool = append(pool, scoring.ScoredCandidate{
			Code: p.StockCode, Score: scoreOf(p),
			PostAnalysisCompleted: p.PostAnalysisCompleted,
		})
	}

	seed := cfg.RotationSeed
	if seed == "" {
		seed = "daily_pick"
	}
	rotated := scoring.RotateSelection(selected, pool, seed, tradeDate)

	byCode := make(map[string]*models.DailyPick, len(picks)+len(pool))
	for i := range picks {
		byCode[picks[i].StockCode] = &picks[i]
	}
	for i := range results {
		if results[i].err == nil {
			if _, ok := byCode[results[i].pick.StockCode]; !ok {
				byCode[results[i].pick.StockCode] = &results[i].pick
			}
		}
	}
	out := make([]models.DailyPick, 0, len(rotated))
	for _, c := range rotated {
		if p, ok := byCode[c.Code]; ok {
			out = append(out, *p)
		}
	}
	if len(out) != len(picks) {
		logger.SugaredLogger.Warn("daily_pick: rotation output mismatch, fallback to original picks")
		return picks
	}
	return out
}

// clampFloat 将 v 限制在 [lo, hi]。
func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
