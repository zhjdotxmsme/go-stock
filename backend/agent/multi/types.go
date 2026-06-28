package multi

import "github.com/cloudwego/eino/schema"

// AgentReport is the output of each analyst node
type AgentReport struct {
	Role    string // fundamental / technical / sentiment / news
	Content string // full markdown analysis text
	Summary string // one-paragraph summary
	Rating  string // bullish / bearish / neutral
	Error   string // empty if successful
}

// DebateRound represents one round of bull/bear debate
type DebateRound struct {
	RoundNum     int
	BullArgument string
	BearArgument string
}

// DebateResult is the output of the researcher node
type DebateResult struct {
	Rounds         []DebateRound
	BullFinalArg   string
	BearFinalArg   string
	ConsensusItems []string
	Disagreements  []string
}

// PriceZone 表示价格区间
type PriceZone struct {
	Low  float64 `json:"low"`
	High float64 `json:"high"`
}

// ChecklistItem 表示操作检查项
type ChecklistItem struct {
	Action      string `json:"action"`       // 操作描述
	Priority    string `json:"priority"`     // high / medium / low
	IsCompleted bool   `json:"is_completed"` // 默认 false
}

// FinalReport is the output of the synthesis node
type FinalReport struct {
	OverallRating      string // strong_buy / buy / hold / sell / strong_sell
	InvestmentThesis   string
	Strengths          []string
	RiskFactors        []string
	Catalysts          []string
	MultiTimeframeView map[string]string // short/medium/long term views
	Conclusion         string

	// 新增结构化字段
	Score     float64         `json:"score"`     // 1-10 综合评分，0=未评估
	Trend     string          `json:"trend"`     // up / down / sideways
	EntryZone *PriceZone      `json:"entryZone"` // 买入区间，nil=未提供
	ExitZone  *PriceZone      `json:"exitZone"`  // 卖出区间，nil=未提供
	RiskLevel string          `json:"riskLevel"` // low / medium / high
	Checklist []ChecklistItem `json:"checklist"` // 操作检查清单
}

// PolicyReport is the output of the policy analyst
type PolicyReport struct {
	PolicyEvents   []string // recent policy events
	IndustryImpact string   // impact on the industry
	RegulatoryRisk []string // regulatory risks
	Summary        string
}

// HotMoneyReport is the output of the hot money tracker
type HotMoneyReport struct {
	DragonTiger []string // dragon tiger list data
	CapitalFlow string   // capital flow summary
	MajorOrders []string // major order流向
	Summary     string
}

// LockupReport is the output of the lockup watcher
type LockupReport struct {
	UnlockEvents   []string // upcoming unlock events
	ReductionPlans []string // major shareholder reduction plans
	PledgeRatio    string   // pledge ratio
	Summary        string
}

// AgentContext carries shared state through the Graph.
// context.Context is NOT stored here — it is passed as the first parameter to each node function.
type AgentContext struct {
	StockCode    string
	StockName    string
	Market       string // A / HK / US
	UserQuery    string // the user's original question
	StrategyCode string // 空=全分析模式, 非空=策略Code（如 "moving_average"）
	AIConfigID   int
	Reports      []AgentReport
	Debate       *DebateResult
	FinalReport  *FinalReport
	// StreamCh is an optional channel for streaming token-level output to the frontend.
	// When set, each node pushes streaming events as it processes.
	StreamCh chan *schema.Message
}
