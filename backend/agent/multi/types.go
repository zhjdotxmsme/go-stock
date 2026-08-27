package multi

import (
	"go-stock/backend/agent/multi/risk_debate"

	"github.com/cloudwego/eino/schema"
)

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

	// A3 增强字段（D6 分歧分类 / T1 风控辩论 / D4 风控否决，仅 full/specialist/quick 模式填充；
	// standard 模式不填，零值经 omitempty 不出现在事件 JSON 中，前端无感知）
	DisagreementClass string `json:"disagreementClass,omitempty"` // D6 分歧分类
	DecisionHint      string `json:"decisionHint,omitempty"`      // D6 决策路径提示
	RiskJudgeDecision string `json:"riskJudgeDecision,omitempty"` // T1 风控裁判裁决 BUY/SELL/HOLD
	GuardrailReason   string `json:"guardrailReason,omitempty"`   // D4 风控否决/降级理由

	// A4 增强字段（D5 决策标尺，standard 与模式管线均填充；Score<=0 时不填，
	// 零值经 omitempty 不出现在事件 JSON 中）
	DecisionSignal string `json:"decisionSignal,omitempty"` // D5 信号 strong_buy/buy/watch/reduce/sell
	DecisionAction string `json:"decisionAction,omitempty"` // D5 标准动作 buy/hold/sell（与最终动作一致）
	DecisionLabel  string `json:"decisionLabel,omitempty"`  // D5 中文标签 强烈买入/买入/观望/减仓/卖出
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

	// A3 增强状态（仅模式管线填充；standard 管线不触碰，保持行为不变）
	DisagreementClass string                        // D6 分歧分类结果
	DecisionHint      string                        // D6 决策路径提示
	SynthesisGuidance string                        // D6 注入合成 Prompt 的引导文本
	RiskDebate        *risk_debate.RiskDebateResult // T1 风控辩论结果

	// StreamCh is an optional channel for streaming token-level output to the frontend.
	// When set, each node pushes streaming events as it processes.
	StreamCh chan *schema.Message

	// DataPack 是引擎预取的共享数据包（A2）：分析师优先读取，为 nil 时
	// 各自回退到自己的抓取路径（兼容测试与自定义管线）。
	DataPack *DataPack

	// A5 增强状态（T2 反思记忆注入；由引擎从 EngineConfig 同步，分析师节点读取）
	MemoryInjectionOff bool // 关闭反思记忆注入 Prompt
	// MemoryRetrieve 记忆检索注入点（测试/自定义用）；nil = 默认 SQLite 记忆库。
	// 返回注入用的经验文本列表（空 = 无记忆，不注入）。
	MemoryRetrieve func(role, situation string) []string
}
