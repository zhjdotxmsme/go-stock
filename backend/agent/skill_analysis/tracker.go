package skill_analysis

import (
	"go-stock/backend/db"
	"go-stock/backend/models"
)

func RecordMatch(query, sessionID string, skillIDs []uint) error {
	if db.Dao == nil {
		return nil // DB not initialized, skip silently
	}
	for _, sid := range skillIDs {
		rec := models.SkillUsageRecord{
			SkillID:   sid,
			Query:     query,
			SessionID: sessionID,
			Matched:   true,
			Triggered: true,
		}
		if err := db.Dao.Create(&rec).Error; err != nil {
			return err
		}
	}
	return nil
}

func UpdateResult(sessionID string, outputScore float64, mcpUsed bool, errMsg string) error {
	if db.Dao == nil {
		return nil // DB not initialized, skip silently
	}
	return db.Dao.Model(&models.SkillUsageRecord{}).
		Where("session_id = ?", sessionID).
		Updates(map[string]any{
			"output_score": outputScore,
			"mcp_used":     mcpUsed,
			"error_msg":    errMsg,
		}).Error
}
