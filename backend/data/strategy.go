package data

// ScoringStrategy defines a pluggable scoring approach for daily stock picks.
type ScoringStrategy interface {
	// Name returns the display name, e.g. "均线趋势"
	Name() string
	// Code returns the unique identifier, e.g. "ma_trend"
	Code() string
	// Description returns a brief description.
	Description() string
	// Score computes a multi-factor score (0-100) and returns factor details + signal summary.
	Score(ctx *StrategyContext) *StrategyResult
}

// StrategyContext holds pre-extracted K-line data and other scoring context.
type StrategyContext struct {
	KLines    []KLineData
	CloseP    []float64
	HighP     []float64
	LowP      []float64
	Volume    []float64
	StockCode string
	StockName string
	TradeDate string

	// Fundamental / industry context (pre-fetched, shared across all candidates)
	IndustryCode        string  // stock's industry name
	IndustryRankScore   float64 // industry strength score (0-1), from industry money-flow rank
	MacroScore          float64 // macro environment score (0-1), from PMI/CPI/GDP
	ResearchReportCount int     // number of research reports in last 30 days
}

// StrategyResult holds the scoring output.
type StrategyResult struct {
	Score   float64            // 0-100
	Factors map[string]float64 // per-factor breakdown
	Signal  string             // e.g. "MA5>MA10>MA20多头排列，BIAS+4.2%"
}
