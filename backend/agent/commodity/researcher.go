package commodity

import (
	"context"
	"errors"
	"fmt"
	"go-stock/backend/agent/multi"
	"go-stock/backend/logger"
	"io"
	"strings"

	"github.com/cloudwego/eino/schema"
)

func RunDebate(ctx context.Context, cc *CommodityContext, numRounds int) (*DebateResult, error) {
	if numRounds < 1 || numRounds > 3 {
		numRounds = 2
	}

	var analystSummary string
	for _, r := range cc.Reports {
		if r.Error != "" {
			analystSummary += fmt.Sprintf("【%s】数据不可用\n", r.Role)
			continue
		}
		analystSummary += fmt.Sprintf("【%s】评级:%s\n摘要:%s\n\n", r.Role, r.Rating, r.Summary)
	}

	result := &DebateResult{}

	for round := 1; round <= numRounds; round++ {
		logger.SugaredLogger.Infof("commodity debate round %d/%d", round, numRounds)

		bullPrompt := fmt.Sprintf("基于以下大宗商品分析报告，请从看多角度进行分析:\n\n%s", analystSummary)
		if len(result.Rounds) > 0 {
			last := result.Rounds[len(result.Rounds)-1]
			bullPrompt += fmt.Sprintf("\n\n空方观点:\n%s\n\n请针对上述空方观点进行反驳，提出看多的理由。", last.BearArgument)
		}

		bullArg, err := callDebateLLM(ctx, cc, "bull", bullPrompt)
		if err != nil {
			logger.SugaredLogger.Errorf("bull researcher round %d error: %v", round, err)
			bullArg = "看多分析暂不可用"
		}
		emitDebate(cc, round, "bull", sanitizeJSON(bullArg))

		bearPrompt := fmt.Sprintf("基于以下大宗商品分析报告，请从看空/风险角度进行分析:\n\n%s", analystSummary)
		bearPrompt += fmt.Sprintf("\n\n多方观点:\n%s\n\n请针对上述多方观点提出风险警示和看空理由。", bullArg)

		bearArg, err := callDebateLLM(ctx, cc, "bear", bearPrompt)
		if err != nil {
			logger.SugaredLogger.Errorf("bear researcher round %d error: %v", round, err)
			bearArg = "看空分析暂不可用"
		}
		emitDebate(cc, round, "bear", sanitizeJSON(bearArg))

		result.Rounds = append(result.Rounds, DebateRound{
			RoundNum:     round,
			BullArgument: bullArg,
			BearArgument: bearArg,
		})
	}

	if len(result.Rounds) > 0 {
		last := result.Rounds[len(result.Rounds)-1]
		result.BullFinalArg = last.BullArgument
		result.BearFinalArg = last.BearArgument

		consensusPrompt := fmt.Sprintf("多方观点:\n%s\n\n空方观点:\n%s\n\n请列出双方达成共识的点和存在分歧的点。",
			result.BullFinalArg, result.BearFinalArg)

		consensusResult, err := callDebateLLM(ctx, cc, "synthesis", consensusPrompt)
		if err == nil {
			result.ConsensusItems = extractItems(consensusResult, "共识")
			result.Disagreements = extractItems(consensusResult, "分歧")
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

func callDebateLLM(ctx context.Context, cc *CommodityContext, side string, userPrompt string) (string, error) {
	var sysPrompt string
	switch side {
	case "bull":
		sysPrompt = GetRolePrompt("commodity_bull", BullResearcherPrompt)
	case "bear":
		sysPrompt = GetRolePrompt("commodity_bear", BearResearcherPrompt)
	default:
		sysPrompt = "你是一个中立的分析助手，请客观列出观点。"
	}

	chatModel, err := multi.GetChatModelWithTier(ctx, "researcher_"+side, multi.LLMTierDeep, cc.AIConfigID)
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

func extractItems(text string, keyword string) []string {
	var items []string
	lines := strings.Split(text, "\n")
	inSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, keyword) {
			inSection = true
			continue
		}
		if inSection && (strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "•")) {
			item := strings.TrimLeft(trimmed, "-*• ")
			if item != "" {
				items = append(items, item)
			}
		}
	}
	if len(items) == 0 && len(text) > 0 {
		lines := strings.Split(text, "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "•") {
				item := strings.TrimLeft(trimmed, "-*• ")
				if item != "" {
					items = append(items, item)
				}
			}
		}
	}
	if len(items) == 0 && len(text) > 0 {
		items = append(items, text)
	}
	return items
}
