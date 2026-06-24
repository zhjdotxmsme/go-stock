package multi

// No imports needed — AgentContext no longer stores context.Context.
// Node functions receive ctx as their first parameter.

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

// FinalReport is the output of the synthesis node
type FinalReport struct {
	OverallRating      string // strong_buy / buy / hold / sell / strong_sell
	InvestmentThesis   string
	Strengths          []string
	RiskFactors        []string
	Catalysts          []string
	MultiTimeframeView map[string]string // short/medium/long term views
	Conclusion         string
}

// AgentContext carries shared state through the Graph.
// context.Context is NOT stored here — it is passed as the first parameter to each node function.
type AgentContext struct {
	StockCode  string
	StockName  string
	Market     string // A / HK / US
	UserQuery  string // the user's original question
	AIConfigID int
	Reports    []AgentReport
	Debate     *DebateResult
	FinalReport *FinalReport
}
