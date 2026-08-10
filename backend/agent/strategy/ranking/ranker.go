// Package ranking 实现 LLM 二次排序（方案 §8.1 D2）。
// 候选池（screen_score 初排）经 LLM 跨股票对比打分后按混合分重排：
//
//	final_score = screen_score × (1 - rank_weight) + llm_score × rank_weight
//
// 模型链降级：主模型→备选（去重），每模型最多重试 1 次，
// 覆盖率（成功解析出 llm_score 的候选占比）≥ 阈值算成功，全部失败退化为原始 screen_score 排序。
// LLM 调用通过 LLMCallFunc 注入，本包不直连任何数据源/AI 配置，保持纯逻辑可测试。
package ranking

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
)

// LLMCallFunc LLM 调用函数注入：给定模型名与 prompt，返回模型文本输出。
// 模型链与调用实现由调用方装配（如从设置中的 AI 配置构建），本包不感知。
type LLMCallFunc func(ctx context.Context, model string, prompt string) (string, error)

// Candidate 候选股票输入（30+ 字段，方案 §8.1 D2 字段清单）。
// 风格与 D1 scoring.FactorInput / D3 risk.RiskInput 一致：纯数据，由调用方装配，
// 可选字段标注 omitempty（零值/空值不参与 Prompt 格式化）。
type Candidate struct {
	Code        string  `json:"code"`
	Name        string  `json:"name,omitempty"`
	ScreenScore float64 `json:"screenScore"` // 初排综合分（D1 scoring 输出），必填

	// 行情与估值
	Price         float64 `json:"price,omitempty"`
	ChangePercent float64 `json:"changePercent,omitempty"` // 当日涨跌幅 %
	Amount        float64 `json:"amount,omitempty"`        // 成交额（元）
	TurnoverRate  float64 `json:"turnoverRate,omitempty"`  // 换手率 %
	VolumeRatio   float64 `json:"volumeRatio,omitempty"`   // 量比
	TotalCap      float64 `json:"totalCap,omitempty"`      // 总市值（元）
	PE            float64 `json:"pe,omitempty"`
	PB            float64 `json:"pb,omitempty"`

	// 行业与板块
	Industry          string   `json:"industry,omitempty"`
	Concepts          []string `json:"concepts,omitempty"`
	IndustryRank      int      `json:"industryRank,omitempty"`      // 行业内排名
	IndustryChangePct float64  `json:"industryChangePct,omitempty"` // 行业当日涨跌 %

	// 板块热度 6 维评分
	HeatLatest          float64 `json:"heatLatest,omitempty"`
	HeatTrend           float64 `json:"heatTrend,omitempty"`
	HeatPersistenceDays int     `json:"heatPersistenceDays,omitempty"`
	HeatCooling         float64 `json:"heatCooling,omitempty"`
	HeatWatchCount      int     `json:"heatWatchCount,omitempty"`
	HeatState           string  `json:"heatState,omitempty"`   // 升温/降温等状态
	HeatSummary         string  `json:"heatSummary,omitempty"` // 热度摘要

	// 技术面（60 日视角，指标状态由调用方预计算后以字符串传入）
	ChangePct60       float64 `json:"changePct60,omitempty"`       // 60 日涨跌幅 %
	SignalScore       float64 `json:"signalScore,omitempty"`       // 日线信号分 0-100
	MACDState         string  `json:"macdState,omitempty"`         // bullish/bearish/neutral
	RSIState          string  `json:"rsiState,omitempty"`          // overbought/oversold/neutral
	BreakoutPct       float64 `json:"breakoutPct,omitempty"`       // 突破幅度 %
	AmplitudePct      float64 `json:"amplitudePct,omitempty"`      // 振幅 %
	VolumeRatio20     float64 `json:"volumeRatio20,omitempty"`     // 20 日量比
	BodyPct           float64 `json:"bodyPct,omitempty"`           // K 线实体 %
	PullbackMA20      bool    `json:"pullbackMA20,omitempty"`      // 是否回踩 MA20
	ConsolidationDays int     `json:"consolidationDays,omitempty"` // 盘整天数
	Volatility        float64 `json:"volatility,omitempty"`        // 年化波动率 %
	MaxDrawdown       float64 `json:"maxDrawdown,omitempty"`       // 最大回撤 %
	ATR               float64 `json:"atr,omitempty"`               // ATR(14)

	// D1 九因子评分（因子名 -> 得分）
	FactorScores map[string]float64 `json:"factorScores,omitempty"`

	// 资讯与基本面
	NewsTitles          []string `json:"newsTitles,omitempty"`          // 近期新闻标题
	FundamentalsCovered bool     `json:"fundamentalsCovered,omitempty"` // 基本面数据是否覆盖
}

// RankedStock 单只候选的重排序结果。
type RankedStock struct {
	Code        string  `json:"code"`
	ScreenScore float64 `json:"screenScore"`
	FinalScore  float64 `json:"finalScore"` // 混合分（未覆盖时等于 screenScore）

	// LLM 输出（未被 LLM 覆盖时 LLMScore=0 且 Covered=false，其余为零值）
	Covered      bool     `json:"covered"`
	LLMScore     float64  `json:"llmScore,omitempty"`
	Confidence   float64  `json:"confidence,omitempty"`
	Sector       string   `json:"sector,omitempty"`
	Theme        string   `json:"theme,omitempty"`
	Thesis       string   `json:"thesis,omitempty"`
	Reason       string   `json:"reason,omitempty"`
	Risk         string   `json:"risk,omitempty"`
	Catalysts    []string `json:"catalysts,omitempty"`
	RiskFlags    []string `json:"riskFlags,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	StyleFit     string   `json:"styleFit,omitempty"`
	WatchItems   []string `json:"watchItems,omitempty"`
	Invalidators []string `json:"invalidators,omitempty"`
}

// RankedResult LLM 二次排序结果。
type RankedResult struct {
	MarketView     string        `json:"marketView,omitempty"`     // 整体观点
	SelectionLogic string        `json:"selectionLogic,omitempty"` // 选股逻辑
	PortfolioRisk  string        `json:"portfolioRisk,omitempty"`  // 组合风险
	Stocks         []RankedStock `json:"stocks"`                   // 按 FinalScore 降序
	Coverage       float64       `json:"coverage"`                 // 成功解析出 llm_score 的候选占比
	Model          string        `json:"model,omitempty"`          // 实际生效的模型
	Degraded       bool          `json:"degraded"`                 // true = 全部模型失败，退化为 screen_score 排序
}

// RankerConfig 重排序器配置（JSON 序列化，遵循 D1/D3 的 JSON 配置惯例）。
type RankerConfig struct {
	RankWeight        float64 `json:"rankWeight"`        // llm_score 混合权重，默认 0.40
	CoverageThreshold float64 `json:"coverageThreshold"` // 覆盖率成功阈值，默认 0.60
	MaxRetries        int     `json:"maxRetries"`        // 每模型最大重试次数，默认 1（即最多调用 2 次）
}

// DefaultRankerConfig 返回方案 §8.1 D2 规格的默认配置。
func DefaultRankerConfig() RankerConfig {
	return RankerConfig{RankWeight: 0.40, CoverageThreshold: 0.60, MaxRetries: 1}
}

// LoadRankerConfigJSON 从 JSON 字节流加载配置；缺省字段保留默认值。
func LoadRankerConfigJSON(data []byte) (RankerConfig, error) {
	cfg := DefaultRankerConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("解析重排序配置失败: %w", err)
	}
	return cfg.normalize(), nil
}

// Ranker LLM 重排序器。
type Ranker struct {
	Config RankerConfig
}

// NewRanker 按配置构造重排序器。
func NewRanker(cfg RankerConfig) *Ranker {
	return &Ranker{Config: cfg.normalize()}
}

// normalize 收敛非法配置：rank_weight 夹到 [0,1]（越界会导致混合分失去意义）。
func (cfg RankerConfig) normalize() RankerConfig {
	if cfg.RankWeight < 0 {
		cfg.RankWeight = 0
	}
	if cfg.RankWeight > 1 {
		cfg.RankWeight = 1
	}
	if cfg.CoverageThreshold < 0 {
		cfg.CoverageThreshold = 0
	}
	if cfg.CoverageThreshold > 1 {
		cfg.CoverageThreshold = 1
	}
	return cfg
}

// llmWireStock LLM 输出的单只股票 JSON 结构（llm_score 用指针以区分"0 分"与"未给出"）。
type llmWireStock struct {
	Code         string   `json:"code"`
	LLMScore     *float64 `json:"llm_score"`
	Confidence   float64  `json:"confidence"`
	Sector       string   `json:"sector"`
	Theme        string   `json:"theme"`
	Thesis       string   `json:"thesis"`
	Reason       string   `json:"reason"`
	Risk         string   `json:"risk"`
	Catalysts    []string `json:"catalysts"`
	RiskFlags    []string `json:"risk_flags"`
	Tags         []string `json:"tags"`
	StyleFit     string   `json:"style_fit"`
	WatchItems   []string `json:"watch_items"`
	Invalidators []string `json:"invalidators"`
}

// llmWireResponse LLM 输出的顶层 JSON 结构。
type llmWireResponse struct {
	MarketView     string         `json:"market_view"`
	SelectionLogic string         `json:"selection_logic"`
	PortfolioRisk  string         `json:"portfolio_risk"`
	Ranked         []llmWireStock `json:"ranked"`
}

// Rank 对候选池执行 LLM 二次排序。
// 全部模型失败（调用报错/输出不可解析/覆盖率不足）时退化为原始 screen_score 排序（Degraded=true）。
func (r *Ranker) Rank(ctx context.Context, candidates []Candidate, modelChain []string, call LLMCallFunc) RankedResult {
	if len(candidates) == 0 {
		return RankedResult{Degraded: true}
	}
	prompt := BuildRankPrompt(FormatCandidates(candidates))

	for _, model := range dedupModels(modelChain) {
		attempts := 1 + r.Config.MaxRetries
		for i := 0; i < attempts; i++ {
			raw, err := call(ctx, model, prompt)
			if err != nil {
				continue
			}
			resp, err := parseLLMResponse(raw)
			if err != nil {
				continue
			}
			result := r.buildResult(candidates, resp)
			if result.Coverage < r.Config.CoverageThreshold {
				break // 覆盖率不足视为该模型失败，重试无意义，换下一模型
			}
			result.Model = model
			return result
		}
	}

	return fallbackResult(candidates)
}

// dedupModels 模型链去重（保留首次出现顺序，忽略空串）。
func dedupModels(chain []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(chain))
	for _, m := range chain {
		if m == "" || seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
	}
	return out
}

// parseLLMResponse 解析 LLM 输出（先直接解析，失败则经 JSON 修复后再解析）。
func parseLLMResponse(raw string) (*llmWireResponse, error) {
	var resp llmWireResponse
	if err := json.Unmarshal([]byte(raw), &resp); err == nil {
		return &resp, nil
	}
	repaired := RepairJSON(raw)
	if err := json.Unmarshal([]byte(repaired), &resp); err != nil {
		return nil, fmt.Errorf("LLM 输出解析失败: %w", err)
	}
	return &resp, nil
}

// buildResult 将 LLM 输出与候选池按 code 对齐，计算覆盖率与混合分，按 FinalScore 降序。
func (r *Ranker) buildResult(candidates []Candidate, resp *llmWireResponse) RankedResult {
	byCode := make(map[string]llmWireStock, len(resp.Ranked))
	for _, ws := range resp.Ranked {
		if ws.Code != "" {
			byCode[ws.Code] = ws
		}
	}

	result := RankedResult{
		MarketView:     resp.MarketView,
		SelectionLogic: resp.SelectionLogic,
		PortfolioRisk:  resp.PortfolioRisk,
		Stocks:         make([]RankedStock, 0, len(candidates)),
	}
	w := r.Config.RankWeight
	covered := 0
	for _, c := range candidates {
		rs := RankedStock{Code: c.Code, ScreenScore: c.ScreenScore, FinalScore: c.ScreenScore}
		if ws, ok := byCode[c.Code]; ok && ws.LLMScore != nil {
			rs.Covered = true
			rs.LLMScore = *ws.LLMScore
			rs.Confidence = ws.Confidence
			rs.Sector = ws.Sector
			rs.Theme = ws.Theme
			rs.Thesis = ws.Thesis
			rs.Reason = ws.Reason
			rs.Risk = ws.Risk
			rs.Catalysts = ws.Catalysts
			rs.RiskFlags = ws.RiskFlags
			rs.Tags = ws.Tags
			rs.StyleFit = ws.StyleFit
			rs.WatchItems = ws.WatchItems
			rs.Invalidators = ws.Invalidators
			rs.FinalScore = c.ScreenScore*(1-w) + rs.LLMScore*w
			covered++
		}
		result.Stocks = append(result.Stocks, rs)
	}
	result.Coverage = float64(covered) / float64(len(candidates))
	sort.SliceStable(result.Stocks, func(i, j int) bool {
		return result.Stocks[i].FinalScore > result.Stocks[j].FinalScore
	})
	return result
}

// fallbackResult 退化排序：保持原始 screen_score 降序，LLM 字段为空。
func fallbackResult(candidates []Candidate) RankedResult {
	result := RankedResult{
		Stocks:   make([]RankedStock, 0, len(candidates)),
		Degraded: true,
	}
	for _, c := range candidates {
		result.Stocks = append(result.Stocks, RankedStock{
			Code:        c.Code,
			ScreenScore: c.ScreenScore,
			FinalScore:  c.ScreenScore,
		})
	}
	sort.SliceStable(result.Stocks, func(i, j int) bool {
		return result.Stocks[i].FinalScore > result.Stocks[j].FinalScore
	})
	return result
}
