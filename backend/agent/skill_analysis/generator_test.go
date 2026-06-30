package skill_analysis

import (
	"context"
	"testing"
)

type fakeLLM struct{}

func (f fakeLLM) Complete(ctx context.Context, prompt string) (string, error) {
	return `{"name":"测试","category":"通用","description":"测试","systemPrompt":"你是测试","examples":"","triggerKeywords":"测试","confidence":0.9}`, nil
}

func TestGenerateSkillFromURL(t *testing.T) {
	// This test will likely fail with a network error (fake URL), but it should reach the LLM step
	skill, conf, err := GenerateSkillFromURL(context.Background(), "http://example.com", fakeLLM{})
	if err != nil {
		t.Skipf("skipping: network-dependent test: %v", err)
	}
	if skill != nil {
		if skill.Name != "测试" || conf < 0 {
			t.Fatal("unexpected draft result")
		}
	}
}
