package multi

import (
	"context"
	"errors"
	"fmt"
	"go-stock/backend/logger"
	"io"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// RunDebate executes the bull/bear debate loop.
// numRounds: number of debate rounds (default 2, max 3)
// Each round: Bull LLM states argument → Bear LLM states counter-argument
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
		analystSummary += fmt.Sprintf("【%s】评级:%s\n摘要:%s\n\n", r.Role, r.Rating, r.Summary)
	}

	result := &DebateResult{}

	for round := 1; round <= numRounds; round++ {
		logger.SugaredLogger.Infof("debate round %d/%d", round, numRounds)

		// Bull researcher speaks
		bullPrompt := fmt.Sprintf("基于以下分析报告，请从看多角度进行分析:\n\n%s", analystSummary)
		if len(result.Rounds) > 0 {
			last := result.Rounds[len(result.Rounds)-1]
			bullPrompt += fmt.Sprintf("\n\n空方观点:\n%s\n\n请针对上述空方观点进行反驳，提出看多的理由。", last.BearArgument)
		}

		bullArg, err := callResearcher(ctx, ac, "bull", bullPrompt)
		if err != nil {
			logger.SugaredLogger.Errorf("bull researcher round %d error: %v", round, err)
			bullArg = "看多分析暂不可用"
		}
		emitDebate(ac, round, "bull", sanitizeJSON(bullArg))

		// Bear researcher speaks
		bearPrompt := fmt.Sprintf("基于以下分析报告，请从看空/风险角度进行分析:\n\n%s", analystSummary)
		bearPrompt += fmt.Sprintf("\n\n多方观点:\n%s\n\n请针对上述多方观点提出风险警示和看空理由。", bullArg)

		bearArg, err := callResearcher(ctx, ac, "bear", bearPrompt)
		if err != nil {
			logger.SugaredLogger.Errorf("bear researcher round %d error: %v", round, err)
			bearArg = "看空分析暂不可用"
		}
		emitDebate(ac, round, "bear", sanitizeJSON(bearArg))

		result.Rounds = append(result.Rounds, DebateRound{
			RoundNum:     round,
			BullArgument: bullArg,
			BearArgument: bearArg,
		})
	}

	// Extract consensus and disagreements
	if len(result.Rounds) > 0 {
		last := result.Rounds[len(result.Rounds)-1]
		result.BullFinalArg = last.BullArgument
		result.BearFinalArg = last.BearArgument

		consensusPrompt := fmt.Sprintf("多方观点:\n%s\n\n空方观点:\n%s\n\n请列出双方达成共识的点和存在分歧的点。",
			result.BullFinalArg, result.BearFinalArg)

		consensusResult, err := callResearcher(ctx, ac, "synthesis", consensusPrompt)
		if err == nil {
			result.ConsensusItems = extractListItems(consensusResult, "共识")
			result.Disagreements = extractListItems(consensusResult, "分歧")
		}
		if len(result.ConsensusItems) == 0 {
			result.ConsensusItems = []string{"待进一步分析确认共识点"}
		}
		if len(result.Disagreements) == 0 {
			result.Disagreements = []string{"待进一步分析确认分歧点"}
		}
	}

	return result, nil
}

// callResearcher calls the LLM with streaming, accumulates the response, and returns it.
func callResearcher(ctx context.Context, ac *AgentContext, side string, userPrompt string) (string, error) {
	var sysPrompt string
	switch side {
	case "bull":
		sysPrompt = GetRolePrompt("multi_bull_researcher", BullResearcherPrompt)
	case "bear":
		sysPrompt = GetRolePrompt("multi_bear_researcher", BearResearcherPrompt)
	default:
		sysPrompt = "你是一个中立的分析助手，请客观列出观点。"
	}

	chatModel, err := GetChatModelWithTier(ctx, "researcher_"+side, LLMTierDeep, ac.AIConfigID)
	if err != nil {
		return "", fmt.Errorf("researcher model: %w", err)
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: sysPrompt},
		{Role: schema.User, Content: userPrompt},
	}

	streamResult, err := chatModel.Stream(ctx, messages)
	if err != nil {
		return "", err
	}

	var content string
	for {
		chunk, err := streamResult.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return content, err
		}
		if chunk != nil {
			content += chunk.Content
		}
	}
	return content, nil
}

// extractListItems attempts to extract bullet items from LLM response.
func extractListItems(text string, keyword string) []string {
	var items []string
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "•") {
			items = append(items, strings.TrimLeft(trimmed, "-*• "))
		}
	}
	if len(items) == 0 && len(text) > 0 {
		items = append(items, text)
	}
	return items
}
