package skill_analysis

import (
	"go-stock/backend/db"
	"go-stock/backend/models"
)

// RecalculateSkillScores recalculates and updates the AvgScore for every skill.
func RecalculateSkillScores() error {
	var skills []models.Skill
	if err := db.Dao.Find(&skills).Error; err != nil {
		return err
	}
	for _, s := range skills {
		score, err := calculateSkillScore(s.ID)
		if err != nil {
			continue
		}
		db.Dao.Model(&models.Skill{}).Where("id = ?", s.ID).Update("avg_score", score)
	}
	return nil
}

func calculateSkillScore(skillID uint) (float64, error) {
	var records []models.SkillUsageRecord
	db.Dao.Where("skill_id = ?", skillID).Find(&records)
	if len(records) == 0 {
		return 0, nil
	}
	var totalOutput, totalRating, mcpOK, mcpTotal, tokenTotal float64
	for _, r := range records {
		totalOutput += r.OutputScore
		totalRating += float64(r.UserRating)
		if r.MCPUsed {
			mcpTotal++
			if r.ErrorMsg == "" {
				mcpOK++
			}
		}
		tokenTotal += float64(r.TokenCost)
	}
	n := float64(len(records))
	outputNorm := totalOutput / (n * 100)
	ratingNorm := (totalRating/n + 1) / 2
	mcpRate := 0.0
	if mcpTotal > 0 {
		mcpRate = mcpOK / mcpTotal
	}
	tokenEff := 0.0
	if tokenTotal > 0 {
		tokenEff = totalOutput / tokenTotal
	}
	score := 0.30*outputNorm + 0.25*ratingNorm + 0.10*mcpRate + 0.10*tokenEff
	return score, nil
}
