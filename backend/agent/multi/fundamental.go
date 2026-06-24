package multi

import "context"

// RunFundamentalAnalyst evaluates company financial health, valuation, and growth.
// Uses tools: stock_info, financial_report, research_report
func RunFundamentalAnalyst(ctx context.Context, ac *AgentContext) (*AgentReport, error) {
	// LLM will be called with FundamentalAnalystPrompt + data tools
	// For now, return a basic report structure
	return &AgentReport{
		Role:    "fundamental",
		Content: "",
		Summary: "",
		Rating:  "neutral",
	}, nil
}
