package skill_analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"go-stock/backend/models"
	"io"
	"net/http"
	"time"
)

// LLMClient interface for generating skill drafts from URL content.
type LLMClient interface {
	Complete(ctx context.Context, prompt string) (string, error)
}

func GenerateSkillFromURL(ctx context.Context, url string, llm LLMClient) (*models.Skill, float64, error) {
	content, err := fetchURLContent(url)
	if err != nil {
		return nil, 0, err
	}
	prompt := fmt.Sprintf(generateSkillPrompt, content)
	resp, err := llm.Complete(ctx, prompt)
	if err != nil {
		return nil, 0, err
	}
	var draft struct {
		Name            string  `json:"name"`
		Category        string  `json:"category"`
		Description     string  `json:"description"`
		SystemPrompt    string  `json:"systemPrompt"`
		Examples        string  `json:"examples"`
		TriggerKeywords string  `json:"triggerKeywords"`
		Confidence      float64 `json:"confidence"`
	}
	if err := json.Unmarshal([]byte(resp), &draft); err != nil {
		return nil, 0, err
	}
	return &models.Skill{
		Name:            draft.Name,
		Category:        draft.Category,
		Description:     draft.Description,
		SystemPrompt:    draft.SystemPrompt,
		Examples:        draft.Examples,
		TriggerKeywords: draft.TriggerKeywords,
		Source:          "generated",
		Confidence:      draft.Confidence,
		Enable:          false,
	}, draft.Confidence, nil
}

func fetchURLContent(url string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	return string(body), nil
}
