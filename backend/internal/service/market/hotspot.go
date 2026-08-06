// Package market 热点生命周期追踪（方案 §8.1 D8）。
// 5 阶段生命周期分类（初次异动 → 确认扩散 → 加速主升 → 分歧放量 → 降温退潮）
// + 板块内股票角色分配（核心龙头/助攻/补涨/后排/掉队）。
// 纯函数实现，不直连数据源，板块热度与股票数据由调用方装配。
package market

import "strings"

// LifecycleStage 热点生命周期 5 阶段（方案 §8.1 D8）。
type LifecycleStage string

const (
	StageEmerging     LifecycleStage = "初次异动"
	StageSpreading    LifecycleStage = "确认扩散"
	StageAccelerating LifecycleStage = "加速主升"
	StageDiverging    LifecycleStage = "分歧放量"
	StageFading       LifecycleStage = "降温退潮"
)

// StockRole 热点中股票的角色（方案 §8.1 D8 角色表）。
type StockRole string

const (
	RoleLeader    StockRole = "核心龙头"
	RoleSupporter StockRole = "助攻"
	RoleLaggard   StockRole = "补涨"
	RoleBackrow   StockRole = "后排"
	RoleDropped   StockRole = "掉队"
)

// HotspotInput 板块热度数据（参照 D1 scoring.ThemeHeatInput 扩展）。
type HotspotInput struct {
	Name            string  // 板块名
	Latest          float64 // 最新热度分 0-100
	Trend           float64 // 趋势分（正=升温，负=降温）
	Cooling         float64 // 降温分（正数表示降温幅度）
	PersistenceDays int     // 持续天数
	WatchCount      int     // 观察次数
	State           string  // 状态文本（升温/降温/退潮/分歧等）
}

// HotStock 板块内一只股票的数据。
type HotStock struct {
	Code      string
	Name      string
	Rank      int     // 板块内排名（1 起）
	Score     float64 // 强度分
	ChangePct float64 // 涨幅 %
}

// RoleAssignment 股票角色分配结果。
type RoleAssignment struct {
	Code string
	Name string
	Role StockRole
}

// HotspotResult 热点分析结果。
type HotspotResult struct {
	Name        string
	Stage       LifecycleStage
	StageReason string
	Stocks      []RoleAssignment
}

// HotspotConfig 生命周期阶段与角色阈值（文档未给出阶段数值条件，
// 以下为设计的默认规则，均可配置）。
type HotspotConfig struct {
	// 降温退潮：降温分 ≥ 阈值，或（趋势 ≤ 阈值 且 最新分 < 阈值），或状态含 降温/退潮
	FadingCooling float64 `json:"fadingCooling"` // 默认 20
	FadingTrend   float64 `json:"fadingTrend"`   // 默认 -5
	FadingLatest  float64 `json:"fadingLatest"`  // 默认 50
	// 分歧放量：最新分仍高但降温分抬头，或状态含 分歧
	DivergingLatest  float64 `json:"divergingLatest"`  // 默认 65
	DivergingCooling float64 `json:"divergingCooling"` // 默认 8
	// 加速主升：趋势强 + 最新分高 + 持续达标
	AccelTrend       float64 `json:"accelTrend"`       // 默认 6
	AccelLatest      float64 `json:"accelLatest"`      // 默认 70
	AccelPersistDays int     `json:"accelPersistDays"` // 默认 3
	// 确认扩散：最新分/持续/观察次数达标且仍在升温
	SpreadLatest      float64 `json:"spreadLatest"`      // 默认 55
	SpreadPersistDays int     `json:"spreadPersistDays"` // 默认 2
	SpreadWatchMin    int     `json:"spreadWatchMin"`    // 默认 10
	// 初次异动：兜底阶段（趋势 > 0 的未达标者）；趋势 ≤ 0 且未达退潮阈值者归退潮
	EmergingLatest float64 `json:"emergingLatest"` // 默认 45（参考线，低于此且无趋势视为退潮）

	// 角色阈值（方案角色表数值，可配置）
	LeaderMinScore     float64 `json:"leaderMinScore"`     // 龙头：排名第1的分数下限，默认 70
	LeaderTop3MinScore float64 `json:"leaderTop3MinScore"` // 龙头：前3的分数下限基准，默认 68
	LeaderTop3TopDelta float64 `json:"leaderTop3TopDelta"` // 龙头：前3的分数 ≥ top-该值，默认 8
	LeaderTop3MinChg   float64 `json:"leaderTop3MinChg"`   // 龙头：前3的涨幅下限 %，默认 5
	SupporterMinScore  float64 `json:"supporterMinScore"`  // 助攻分数下限，默认 62
	SupporterMinChg    float64 `json:"supporterMinChg"`    // 助攻涨幅下限 %，默认 3
	LaggardMinScore    float64 `json:"laggardMinScore"`    // 补涨分数下限，默认 48
	LaggardMinChg      float64 `json:"laggardMinChg"`      // 补涨涨幅下限 %，默认 0
	BackrowMinScore    float64 `json:"backrowMinScore"`    // 后排分数下限，默认 38
}

// DefaultHotspotConfig 返回默认阈值。
func DefaultHotspotConfig() HotspotConfig {
	return HotspotConfig{
		FadingCooling: 20, FadingTrend: -5, FadingLatest: 50,
		DivergingLatest: 65, DivergingCooling: 8,
		AccelTrend: 6, AccelLatest: 70, AccelPersistDays: 3,
		SpreadLatest: 55, SpreadPersistDays: 2, SpreadWatchMin: 10,
		EmergingLatest: 45,
		LeaderMinScore: 70, LeaderTop3MinScore: 68, LeaderTop3TopDelta: 8, LeaderTop3MinChg: 5,
		SupporterMinScore: 62, SupporterMinChg: 3,
		LaggardMinScore: 48, LaggardMinChg: 0,
		BackrowMinScore: 38,
	}
}

// HotspotAnalyzer 热点生命周期分析器。
type HotspotAnalyzer struct {
	Config HotspotConfig
}

// NewHotspotAnalyzer 按配置构造分析器。
func NewHotspotAnalyzer(cfg HotspotConfig) *HotspotAnalyzer {
	return &HotspotAnalyzer{Config: cfg}
}

// ClassifyStage 5 阶段生命周期分类（判定优先级：退潮 > 分歧 > 主升 > 扩散 > 异动）。
func (a *HotspotAnalyzer) ClassifyStage(in *HotspotInput) (LifecycleStage, string) {
	c := a.Config

	// 降温退潮：状态明示 / 降温分高 / 趋势转负且热度低
	if strings.Contains(in.State, "降温") || strings.Contains(in.State, "退潮") {
		return StageFading, "状态文本明示降温/退潮"
	}
	if in.Cooling >= c.FadingCooling {
		return StageFading, "降温分过高"
	}
	if in.Trend <= c.FadingTrend && in.Latest < c.FadingLatest {
		return StageFading, "趋势转负且热度走低"
	}

	// 分歧放量：热度仍高但降温分抬头，或状态明示分歧
	if strings.Contains(in.State, "分歧") {
		return StageDiverging, "状态文本明示分歧"
	}
	if in.Latest >= c.DivergingLatest && in.Cooling >= c.DivergingCooling {
		return StageDiverging, "高热但降温分抬头，多空分歧"
	}

	// 加速主升：趋势强 + 高热 + 持续达标
	if in.Trend >= c.AccelTrend && in.Latest >= c.AccelLatest && in.PersistenceDays >= c.AccelPersistDays {
		return StageAccelerating, "趋势强劲且持续高热"
	}

	// 确认扩散：热度/持续/观察达标且仍在升温
	if in.Trend > 0 && in.Latest >= c.SpreadLatest &&
		in.PersistenceDays >= c.SpreadPersistDays && in.WatchCount >= c.SpreadWatchMin {
		return StageSpreading, "持续升温且关注度扩散"
	}

	// 初次异动：仍在升温的未达标者
	if in.Trend > 0 || in.Latest >= c.EmergingLatest {
		return StageEmerging, "新近升温，尚未确认扩散"
	}
	// 趋势走平/转弱但未达退潮阈值：按退潮处理
	return StageFading, "趋势走弱"
}

// AssignRole 股票角色分配（方案 §8.1 D8 角色表，按优先级判定）。
// topScore 为板块第 1 名的强度分。
func (a *HotspotAnalyzer) AssignRole(rank int, score, changePct, topScore float64) StockRole {
	c := a.Config
	// 核心龙头：排名第1 且分≥70，或排名前3 且分≥max(68, top-8) 且涨幅≥5%
	if rank == 1 && score >= c.LeaderMinScore {
		return RoleLeader
	}
	threshold := c.LeaderTop3MinScore
	if topScore-c.LeaderTop3TopDelta > threshold {
		threshold = topScore - c.LeaderTop3TopDelta
	}
	if rank >= 1 && rank <= 3 && score >= threshold && changePct >= c.LeaderTop3MinChg {
		return RoleLeader
	}
	// 助攻：分≥62 且涨幅≥3%
	if score >= c.SupporterMinScore && changePct >= c.SupporterMinChg {
		return RoleSupporter
	}
	// 补涨：分≥48 且涨幅≥0
	if score >= c.LaggardMinScore && changePct >= c.LaggardMinChg {
		return RoleLaggard
	}
	// 后排：分≥38
	if score >= c.BackrowMinScore {
		return RoleBackrow
	}
	// 掉队：其余
	return RoleDropped
}

// Analyze 对板块做生命周期分类并分配全部股票角色。
func (a *HotspotAnalyzer) Analyze(in *HotspotInput, stocks []HotStock) HotspotResult {
	stage, reason := a.ClassifyStage(in)
	topScore := 0.0
	for _, s := range stocks {
		if s.Rank == 1 {
			topScore = s.Score
			break
		}
	}
	if topScore == 0 && len(stocks) > 0 {
		topScore = stocks[0].Score
	}
	result := HotspotResult{
		Name: in.Name, Stage: stage, StageReason: reason,
		Stocks: make([]RoleAssignment, 0, len(stocks)),
	}
	for _, s := range stocks {
		result.Stocks = append(result.Stocks, RoleAssignment{
			Code: s.Code, Name: s.Name,
			Role: a.AssignRole(s.Rank, s.Score, s.ChangePct, topScore),
		})
	}
	return result
}
