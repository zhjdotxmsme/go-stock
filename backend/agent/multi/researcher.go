package multi

import (
	"context"
	"fmt"
	"go-stock/backend/logger"
)

// RunDebate executes the bull/bear debate loop.
// numRounds: number of debate rounds (default 2, max 3)
// Each round: Bull states argument → Bear states counter-argument
// Round 2+ includes rebuttals of the other side's prior points.
func RunDebate(ctx context.Context, ac *AgentContext, numRounds int) (*DebateResult, error) {
	if numRounds < 1 || numRounds > 3 {
		numRounds = 2
	}
	
	// Build analyst context summary from all available reports
	var analystSummary string
	for _, r := range ac.Reports {
		if r.Error != "" {
			analystSummary += fmt.Sprintf("【%s】数据不可用\n", r.Role)
			continue
		}
		analystSummary += fmt.Sprintf("===== %s 分析报告 =====\n%s\n\n", r.Role, r.Content)
	}
	
	result := &DebateResult{}
	
	for round := 1; round <= numRounds; round++ {
		logger.SugaredLogger.Infof("debate round %d/%d", round, numRounds)
		
		// Bull researcher speaks
		bullArg := fmt.Sprintf("[第%d轮 看多方] 基于以下分析:\n%s\n", round, analystSummary)
		if len(result.Rounds) > 0 {
			last := result.Rounds[len(result.Rounds)-1]
			bullArg += fmt.Sprintf("\n对方观点: %s\n请针对上述观点进行反驳。", last.BearArgument)
		}
		
		// Bear researcher speaks
		bearArg := fmt.Sprintf("[第%d轮 看空方] 基于以下分析:\n%s\n", round, analystSummary)
		bearArg += fmt.Sprintf("\n对方观点: %s\n请针对上述观点提出风险警示。", bullArg)
		
		result.Rounds = append(result.Rounds, DebateRound{
			RoundNum:     round,
			BullArgument: bullArg,
			BearArgument: bearArg,
		})
	}
	
	// Extract consensus and disagreements (placeholder — will use LLM in full impl)
	if len(result.Rounds) > 0 {
		last := result.Rounds[len(result.Rounds)-1]
		result.BullFinalArg = last.BullArgument
		result.BearFinalArg = last.BearArgument
		result.ConsensusItems = []string{"待LLM提取共识点"}
		result.Disagreements = []string{"待LLM提取分歧点"}
	}
	
	return result, nil
}
