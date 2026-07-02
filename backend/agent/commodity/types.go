package commodity

import "github.com/cloudwego/eino/schema"

type CommodityContext struct {
	Code        string
	Name        string
	UserQuery   string
	AIConfigID  int
	Reports     []ExpertReport
	Debate      *DebateResult
	FinalReport *CommodityReport
	StreamCh    chan *schema.Message
}

type ExpertReport struct {
	Role    string
	Content string
	Summary string
	Rating  string
	Data    map[string]any
	Error   string
}

type DebateRound struct {
	RoundNum     int
	BullArgument string
	BearArgument string
}

type DebateResult struct {
	Rounds         []DebateRound
	BullFinalArg   string
	BearFinalArg   string
	ConsensusItems []string
	Disagreements  []string
}

type PriceZone struct {
	Low  float64 `json:"low"`
	High float64 `json:"high"`
}

type ChecklistItem struct {
	Action      string `json:"action"`
	Priority    string `json:"priority"`
	IsCompleted bool   `json:"is_completed"`
}

type CommodityReport struct {
	OverallRating    string          `json:"overallRating"`
	InvestmentThesis string          `json:"investmentThesis"`
	Strengths        []string        `json:"strengths"`
	RiskFactors      []string        `json:"riskFactors"`
	Catalysts        []string        `json:"catalysts"`
	Conclusion       string          `json:"conclusion"`
	Score            float64         `json:"score"`
	Trend            string          `json:"trend"`
	EntryZone        *PriceZone      `json:"entryZone"`
	ExitZone         *PriceZone      `json:"exitZone"`
	RiskLevel        string          `json:"riskLevel"`
	Checklist        []ChecklistItem `json:"checklist"`
}
