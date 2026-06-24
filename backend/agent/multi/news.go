package multi

import "context"

// RunNewsAnalyst monitors macro news, industry dynamics, and company announcements.
// Uses tools: news, calendar, company_announcement
func RunNewsAnalyst(ctx context.Context, ac *AgentContext) (*AgentReport, error) {
	return &AgentReport{
		Role:    "news",
		Content: "",
		Summary: "",
		Rating:  "neutral",
	}, nil
}
