package skill_analysis

import (
	"go-stock/backend/models"
	"testing"
)

func TestMatchSkillsEmptyQuery(t *testing.T) {
	got := matchSkills("", []models.Skill{{Name: "通用", TriggerKeywords: ""}})
	if len(got) != 1 {
		t.Fatalf("expected 1 match, got %d", len(got))
	}
}

func TestMatchSkillsKeywordHit(t *testing.T) {
	got := matchSkills("分析 MACD 指标", []models.Skill{
		{Name: "技术分析", TriggerKeywords: "MACD,KDJ,RSI"},
		{Name: "基本面", TriggerKeywords: "营收,利润,PE"},
	})
	if len(got) != 1 {
		t.Fatalf("expected 1 match for 'MACD', got %d", len(got))
	}
}

func TestMatchSkillsNoHit(t *testing.T) {
	got := matchSkills("天气怎么样", []models.Skill{
		{Name: "技术分析", TriggerKeywords: "MACD,KDJ,RSI"},
	})
	if len(got) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(got))
	}
}

func TestMatchSkillsCaseInsensitive(t *testing.T) {
	got := matchSkills("分析 macd 指标", []models.Skill{
		{Name: "技术分析", TriggerKeywords: "MACD,KDJ,RSI"},
	})
	if len(got) != 1 {
		t.Fatalf("expected 1 case-insensitive match, got %d", len(got))
	}
}

func TestMatchSkillsMultipleHits(t *testing.T) {
	got := matchSkills("分析 MACD 和 PE 指标", []models.Skill{
		{Name: "技术分析", TriggerKeywords: "MACD,KDJ,RSI"},
		{Name: "基本面", TriggerKeywords: "营收,利润,PE"},
	})
	if len(got) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(got))
	}
}
