package models

import (
	"time"

	"gorm.io/gorm"
)

// DailyPick 短线荐股 — 每日收盘后技术面选股推荐
type DailyPick struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	StockCode string `json:"stockCode" gorm:"uniqueIndex:idx_daily_pick_date_code;size:20"`
	StockName string `json:"stockName" gorm:"size:50"`
	TradeDate string `json:"tradeDate" gorm:"uniqueIndex:idx_daily_pick_date_code;size:10"` // YYYY-MM-DD 推荐基准日

	// 评分
	Score float64 `json:"score" gorm:"index"` // 综合评分 0-100
	Rank  int     `json:"rank"`               // 当日排名

	StrategyCode string `json:"strategyCode" gorm:"size:50;index"`
	StrategyName string `json:"strategyName" gorm:"size:50"`

	// 因子得分明细
	VolumeFactor   float64 `json:"volumeFactor"`   // 成交量放大因子 (0-1)
	MaFactor       float64 `json:"maFactor"`       // 均线形态因子 (0-1)
	RsiFactor      float64 `json:"rsiFactor"`      // RSI 因子 (0-1)
	MacdFactor     float64 `json:"macdFactor"`     // MACD 因子 (0-1)
	PriceFactor    float64 `json:"priceFactor"`    // 价格位置因子 (0-1)
	TurnoverFactor float64 `json:"turnoverFactor"` // 换手率因子 (0-1)
	IndustryScore  float64 `json:"industryScore"`  // 行业强度 (0-1)
	ResearchScore  float64 `json:"researchScore"`  // 研报热度 (原始研报数量)
	MacroScore     float64 `json:"macroScore"`     // 宏观环境 (0-1)

	// 推荐日行情快照
	ClosePrice    float64 `json:"closePrice"`
	OpenPrice     float64 `json:"openPrice"`
	HighPrice     float64 `json:"highPrice"`
	LowPrice      float64 `json:"lowPrice"`
	Volume        int64   `json:"volume"`
	TurnoverRate  float64 `json:"turnoverRate"`
	ChangePercent float64 `json:"changePercent"` // 当日涨跌幅

	// 技术指标快照
	Ma5        float64 `json:"ma5"`
	Ma10       float64 `json:"ma10"`
	Ma20       float64 `json:"ma20"`
	Ma60       float64 `json:"ma60"`
	Macd       float64 `json:"macd"`
	MacdSignal float64 `json:"macdSignal"`
	Rsi14      float64 `json:"rsi14"`
	KdjK       float64 `json:"kdjK"`
	KdjD       float64 `json:"kdjD"`
	KdjJ       float64 `json:"kdjJ"`
	BollMid    float64 `json:"bollMid"`
	BollUp     float64 `json:"bollUp"`
	BollDown   float64 `json:"bollDown"`

	// 次日复盘（复盘时填充）
	NextOpen        float64 `json:"nextOpen"`
	NextHigh        float64 `json:"nextHigh"`
	NextLow         float64 `json:"nextLow"`
	NextClose       float64 `json:"nextClose"`
	NextReturn      float64 `json:"nextReturn"`                          // 次日收益率（开盘买入→收盘卖出）
	NextMaxReturn   float64 `json:"nextMaxReturn"`                       // 次日最大收益率
	NextMaxDrawdown float64 `json:"nextMaxDrawdown"`                     // 次日最大回撤
	Reviewed        bool    `json:"reviewed" gorm:"default:false;index"` // 是否已复盘

	// ===== D12 扩展：选股→排名→风控→后分析生命周期（方案 §8.1 D12，纯新增）=====

	// 核心（rank/stockCode/stockName 沿用既有字段）
	FinalScore    float64 `json:"finalScore"`                    // 混合最终分（screen×0.6+llm×0.4，D2）
	ScreenScore   float64 `json:"screenScore"`                   // 量化初筛综合分（D1 scoring 输出）
	LlmScore      float64 `json:"llmScore"`                      // LLM 二次排序分（D2）
	RankingReason string  `json:"rankingReason" gorm:"size:500"` // 排名理由

	// 价格/量（closePrice/changePercent/turnoverRate 沿用既有字段）
	Amount      float64 `json:"amount"`      // 当日成交额（元）
	TotalMV     float64 `json:"totalMv"`     // 总市值（元）
	VolumeRatio float64 `json:"volumeRatio"` // 量比

	// 基本面
	PeRatio float64 `json:"peRatio"` // 市盈率
	PbRatio float64 `json:"pbRatio"` // 市净率

	// 行业/主题
	Industry             string  `json:"industry" gorm:"size:50"`
	Concepts             string  `json:"concepts" gorm:"type:text"`        // 概念列表，JSON 数组
	IndustryRank         int     `json:"industryRank"`                     // 行业内排名
	BoardHeatLatest      float64 `json:"boardHeatLatest"`                  // 板块热度：最新
	BoardHeatTrend       float64 `json:"boardHeatTrend"`                   // 板块热度：趋势
	BoardHeatPersistence int     `json:"boardHeatPersistence"`             // 板块热度：持续天数
	BoardHeatCooling     float64 `json:"boardHeatCooling"`                 // 板块热度：降温幅度
	BoardHeatWatchCount  int     `json:"boardHeatWatchCount"`              // 板块热度：观察数
	BoardHeatSummary     string  `json:"boardHeatSummary" gorm:"size:500"` // 板块热度：摘要
	BoardHeatState       string  `json:"boardHeatState" gorm:"size:20"`    // 板块热度状态（升温/降温等）

	// 技术面（指标状态为字符串枚举，由调用方预计算）
	Change60dPct      float64 `json:"change60dPct"`              // 60 日涨跌幅 %
	SignalScore       float64 `json:"signalScore"`               // 日线信号分 0-100
	MacdStatus        string  `json:"macdStatus" gorm:"size:20"` // bullish/bearish/neutral
	RsiStatus         string  `json:"rsiStatus" gorm:"size:20"`  // overbought/oversold/neutral
	Breakout20dPct    float64 `json:"breakout20dPct"`            // 20 日突破幅度 %
	AmplitudePct      float64 `json:"amplitudePct"`              // 振幅 %
	VolumeRatio20d    float64 `json:"volumeRatio20d"`            // 20 日量比
	BodyPct           float64 `json:"bodyPct"`                   // K 线实体 %
	PullbackMa20      bool    `json:"pullbackMa20"`              // 是否回踩 MA20
	ConsolidationDays int     `json:"consolidationDays"`         // 盘整天数
	Volatility20dPct  float64 `json:"volatility20dPct"`          // 20 日年化波动率 %
	MaxDrawdownPct    float64 `json:"maxDrawdownPct"`            // 最大回撤 %
	Atr14             float64 `json:"atr14"`                     // ATR(14)

	// 日线质量
	DailyQualityScore float64 `json:"dailyQualityScore"`
	DailyQualityFlags string  `json:"dailyQualityFlags" gorm:"type:text"` // 质量标记，JSON 数组
	DailySource       string  `json:"dailySource" gorm:"size:50"`         // 日线数据源

	// 因子评分（D1 九因子，map[string]float64 序列化为 JSON 字符串）
	FactorScores string `json:"factorScores" gorm:"type:text"`

	// LLM 丰富字段（D2 ranked 输出；列表字段为 JSON 数组字符串）
	LlmConfidence   float64 `json:"llmConfidence"`
	LlmSector       string  `json:"llmSector" gorm:"size:50"`
	LlmTheme        string  `json:"llmTheme" gorm:"size:100"`
	LlmThesis       string  `json:"llmThesis" gorm:"type:text"`
	LlmReason       string  `json:"llmReason" gorm:"type:text"`
	LlmRisk         string  `json:"llmRisk" gorm:"type:text"`
	LlmCatalysts    string  `json:"llmCatalysts" gorm:"type:text"`
	LlmRiskFlags    string  `json:"llmRiskFlags" gorm:"type:text"`
	LlmTags         string  `json:"llmTags" gorm:"type:text"`
	LlmStyleFit     string  `json:"llmStyleFit" gorm:"size:100"`
	LlmWatchItems   string  `json:"llmWatchItems" gorm:"type:text"`
	LlmInvalidators string  `json:"llmInvalidators" gorm:"type:text"`

	// 风控（D3）
	RiskScore      float64 `json:"riskScore"`
	RiskLevel      string  `json:"riskLevel" gorm:"size:20"` // high/medium/low
	RiskPenalty    float64 `json:"riskPenalty"`
	RiskFlags      string  `json:"riskFlags" gorm:"type:text"`          // 风险标记，JSON 数组
	RiskChecks     string  `json:"riskChecks" gorm:"type:text"`         // 触发的检查项，JSON 数组
	ExcludedByRisk bool    `json:"excludedByRisk" gorm:"default:false"` // 是否被风控排除

	// 组合
	PortfolioPenalty float64 `json:"portfolioPenalty"`                // 板块集中度罚分
	PortfolioFlags   string  `json:"portfolioFlags" gorm:"type:text"` // 组合标记（所属板块桶等），JSON 数组

	// 后分析（D10）
	PostAnalysisStatus      string  `json:"postAnalysisStatus" gorm:"size:20"` // pending/completed/failed
	PostAnalysisCompleted   bool    `json:"postAnalysisCompleted" gorm:"default:false"`
	PostAnalysisScoreDelta  float64 `json:"postAnalysisScoreDelta"`              // 总分调整
	PostAnalysisLocalDelta  float64 `json:"postAnalysisLocalDelta"`              // 本地评分卡调整
	PostAnalysisRemoteDelta float64 `json:"postAnalysisRemoteDelta"`             // 远程分析器调整
	PostAnalysisDetail      string  `json:"postAnalysisDetail" gorm:"type:text"` // 后分析明细 JSON

	// 备注
	Reason  string `json:"reason" gorm:"size:500"`
	Remarks string `json:"remarks" gorm:"size:500"`
}

func (DailyPick) TableName() string {
	return "daily_picks"
}

// DailyPickQuery 分页查询参数
type DailyPickQuery struct {
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
	TradeDate string `json:"tradeDate"` // 按日期筛选 YYYY-MM-DD
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	Reviewed  *bool  `json:"reviewed"` // nil=全部, true=已复盘, false=未复盘
}

// DailyPickPageData 分页响应
type DailyPickPageData struct {
	List       []DailyPick `json:"list"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"pageSize"`
	TotalPages int         `json:"totalPages"`
}

// DailyPickStats 统计汇总
type DailyPickStats struct {
	TotalPicks     int     `json:"totalPicks"`     // 总推荐次数
	ReviewedPicks  int     `json:"reviewedPicks"`  // 已复盘次数
	WinCount       int     `json:"winCount"`       // 盈利次数
	LossCount      int     `json:"lossCount"`      // 亏损次数
	WinRate        float64 `json:"winRate"`        // 胜率
	AvgReturn      float64 `json:"avgReturn"`      // 平均收益率
	TotalReturn    float64 `json:"totalReturn"`    // 累计收益率（复利）
	MaxReturn      float64 `json:"maxReturn"`      // 单日最高收益
	MaxDrawdown    float64 `json:"maxDrawdown"`    // 单日最大回撤
	AvgMaxReturn   float64 `json:"avgMaxReturn"`   // 平均最大收益
	AvgMaxDrawdown float64 `json:"avgMaxDrawdown"` // 平均最大回撤
}

// StrategyConfig AI 配置选股的策略配置
type StrategyConfig struct {
	EnabledStrategies []string           `json:"enabled_strategies"`         // 空=全部启用
	StrategyWeights   map[string]float64 `json:"strategy_weights,omitempty"` // 策略权重覆盖
	StrategyParams    map[string]float64 `json:"strategy_params,omitempty"`  // 参数覆盖
	Filters           []FilterCondition  `json:"filters,omitempty"`          // 后置过滤
	TopN              int                `json:"top_n"`                      // 返回数量
}

type FilterCondition struct {
	Field string  `json:"field"` // rsi14|score|price|volume|turnover|macd
	Op    string  `json:"op"`    // >|<|>=|<=|==
	Value float64 `json:"value"`
}
