package skill_analysis

import (
	"strings"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

type TrackContext struct {
	Query     string
	SessionID string
	SkillIDs  []uint
}

// GetMatchedSkillIDs returns IDs of enabled skills whose trigger keywords match the query.
func GetMatchedSkillIDs(query string) []uint {
	if db.Dao == nil {
		return nil // DB not initialized
	}
	var skills []models.Skill
	db.Dao.Where("enable = ?", true).Find(&skills)
	var ids []uint
	lower := strings.ToLower(query)
	for _, s := range skills {
		if s.TriggerKeywords == "" {
			ids = append(ids, s.ID)
			continue
		}
		for _, k := range strings.Split(s.TriggerKeywords, ",") {
			if strings.Contains(lower, strings.TrimSpace(strings.ToLower(k))) {
				ids = append(ids, s.ID)
				break
			}
		}
	}
	return ids
}
