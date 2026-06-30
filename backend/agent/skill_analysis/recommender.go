package skill_analysis

import (
	"strings"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

type Recommendation struct {
	Type         string `json:"type"` // enable / create / merge
	SkillID      uint   `json:"skillId,omitempty"`
	Name         string `json:"name,omitempty"`
	Reason       string `json:"reason"`
	SuggestedURL string `json:"suggestedUrl,omitempty"`
}

func GetRecommendations(query string) []Recommendation {
	var recs []Recommendation

	var enabled []models.Skill
	db.Dao.Where("enable = ?", true).Order("sort_order ASC, created_at DESC").Find(&enabled)

	var all []models.Skill
	db.Dao.Order("sort_order ASC, created_at DESC").Find(&all)

	matched := matchSkills(query, enabled)
	if len(matched) == 0 {
		recs = append(recs, Recommendation{
			Type:   "create",
			Reason: "当前 Query 未命中任何 Skill，建议根据 yfinance/QUANTAXIS/auto-research 创建新 Skill",
		})
	}
	for _, s := range all {
		if !s.Enable && s.AvgScore > 0.7 {
			recs = append(recs, Recommendation{
				Type:    "enable",
				SkillID: s.ID,
				Name:    s.Name,
				Reason:  "该 Skill 评分较高但尚未启用",
			})
		}
	}
	return recs
}

func matchSkills(query string, skills []models.Skill) []models.Skill {
	var matched []models.Skill
	lower := strings.ToLower(query)
	for _, s := range skills {
		if s.TriggerKeywords == "" {
			matched = append(matched, s)
			continue
		}
		for _, k := range strings.Split(s.TriggerKeywords, ",") {
			if strings.Contains(lower, strings.TrimSpace(strings.ToLower(k))) {
				matched = append(matched, s)
				break
			}
		}
	}
	return matched
}
