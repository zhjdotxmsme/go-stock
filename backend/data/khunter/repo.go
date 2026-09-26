package khunter

import (
	"errors"

	"go-stock/backend/db"
	"go-stock/backend/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// EnsureMigrate 懒迁移 khunter 全部表（幂等）
func EnsureMigrate() error {
	if db.Dao == nil {
		return nil
	}
	return db.Dao.AutoMigrate(
		&models.KhunterSignal{}, &models.KhunterScore{}, &models.KhunterHunting{},
		&models.KhunterMoneyFlowDaily{}, &models.KhunterRiskLevel{}, &models.KhunterEvent{},
	)
}

type Repo struct{}

func NewRepo() *Repo { return &Repo{} }

func (r *Repo) SaveSignals(rows []models.KhunterSignal) error {
	if len(rows) == 0 {
		return nil
	}
	return db.Dao.Create(&rows).Error
}

func (r *Repo) GetSignalsByDate(date string) ([]models.KhunterSignal, error) {
	var rows []models.KhunterSignal
	err := db.Dao.Where("signal_date = ?", date).Order("strategy, code").Find(&rows).Error
	return rows, err
}

func (r *Repo) SaveScores(rows []models.KhunterScore) error {
	if len(rows) == 0 {
		return nil
	}
	return db.Dao.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "code"}, {Name: "score_date"}},
		UpdateAll: true,
	}).Create(&rows).Error
}

func (r *Repo) GetScoresByDate(date string) ([]models.KhunterScore, error) {
	var rows []models.KhunterScore
	err := db.Dao.Where("score_date = ?", date).Order("total DESC").Find(&rows).Error
	return rows, err
}

func (r *Repo) UpsertMoneyFlow(rows []models.KhunterMoneyFlowDaily) error {
	if len(rows) == 0 {
		return nil
	}
	return db.Dao.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "code"}, {Name: "date"}},
		UpdateAll: true,
	}).Create(&rows).Error
}

// GetMoneyFlow 返回最近 days 天资金流，正序（日期升序）
func (r *Repo) GetMoneyFlow(code string, days int) ([]models.KhunterMoneyFlowDaily, error) {
	var rows []models.KhunterMoneyFlowDaily
	err := db.Dao.Where("code = ?", code).Order("date DESC").Limit(days).Find(&rows).Error
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return rows, err
}

func (r *Repo) SaveHunting(h *models.KhunterHunting) error {
	return db.Dao.Save(h).Error
}

func (r *Repo) GetHuntingList(status string) ([]models.KhunterHunting, error) {
	var rows []models.KhunterHunting
	q := db.Dao.Order("enter_date DESC")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	return rows, q.Find(&rows).Error
}

// UpdateHuntingStatus 更新狩猎场条目状态（追踪中 / 已移除）
func (r *Repo) UpdateHuntingStatus(id uint, status string) error {
	return db.Dao.Model(&models.KhunterHunting{}).Where("id = ?", id).
		Update("status", status).Error
}

// IncrTrackDays 追踪天数 +1
func (r *Repo) IncrTrackDays(id uint) error {
	return db.Dao.Model(&models.KhunterHunting{}).Where("id = ?", id).
		Update("track_days", gorm.Expr("track_days + 1")).Error
}

func (r *Repo) SaveRiskLevel(rl *models.KhunterRiskLevel) error {
	return db.Dao.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "date"}},
		UpdateAll: true,
	}).Create(rl).Error
}

func (r *Repo) GetLatestRiskLevel() (*models.KhunterRiskLevel, error) {
	var row models.KhunterRiskLevel
	err := db.Dao.Order("date DESC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *Repo) SaveEvents(rows []models.KhunterEvent) error {
	if len(rows) == 0 {
		return nil
	}
	return db.Dao.Create(&rows).Error
}

// GetActiveEvents 查询 code 在 date 当天仍在有效期内的事件
func (r *Repo) GetActiveEvents(code, date string) ([]models.KhunterEvent, error) {
	var rows []models.KhunterEvent
	err := db.Dao.Where("code = ? AND event_date <= ? AND expire_date >= ?", code, date, date).
		Order("event_date DESC").Find(&rows).Error
	return rows, err
}
