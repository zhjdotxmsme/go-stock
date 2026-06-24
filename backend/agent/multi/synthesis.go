package multi

import (
	"context"
	"fmt"
)

// RunSynthesis combines all analyst reports and debate results into the final comprehensive report.
func RunSynthesis(ctx context.Context, ac *AgentContext) (*FinalReport, error) {
	report := &FinalReport{
		OverallRating:    "hold",
		InvestmentThesis: "",
		Strengths:        []string{},
		RiskFactors:      []string{},
		Catalysts:        []string{},
		MultiTimeframeView: map[string]string{
			"短期": "待分析",
			"中期": "待分析",
			"长期": "待分析",
		},
		Conclusion: "",
	}
	
	// Collect all analyst ratings
	for _, r := range ac.Reports {
		if r.Error != "" {
			continue
		}
		report.Strengths = append(report.Strengths, fmt.Sprintf("%s: %s", r.Role, r.Summary))
	}
	
	// Include debate consensus
	if ac.Debate != nil {
		for _, item := range ac.Debate.ConsensusItems {
			report.Catalysts = append(report.Catalysts, item)
		}
		for _, item := range ac.Debate.Disagreements {
			report.RiskFactors = append(report.RiskFactors, item)
		}
	}
	
	return report, nil
}
