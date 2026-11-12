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
		&models.KhunterRunSummary{},
	)
}

type Repo struct{}

func NewRepo() *Repo { return &Repo{} }

// ReplaceSignalsForDate 保存一次扫描结果；重跑同日先清旧信号，避免重复追加。
func (r *Repo) ReplaceSignalsForDate(tradeDate string, rows []models.KhunterSignal) error {
	return db.Dao.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("signal_date = ?", tradeDate).Delete(&models.KhunterSignal{}).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
}

func (r *Repo) GetSignalsByDate(date string) ([]models.KhunterSignal, error) {
	var rows []models.KhunterSignal
	err := db.Dao.Where("signal_date = ?", date).Order("strategy, code").Find(&rows).Error
	return rows, err
}

// ReplaceScoresForDate 用本次扫描完整替换当日评分；无候选时清空旧结果。
func (r *Repo) ReplaceScoresForDate(tradeDate string, rows []models.KhunterScore) error {
	return db.Dao.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("score_date = ?", tradeDate).Delete(&models.KhunterScore{}).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
}

func (r *Repo) GetScoresByDate(date string) ([]models.KhunterScore, error) {
	var rows []models.KhunterScore
	err := db.Dao.Where("score_date = ?", date).Order("total DESC").Find(&rows).Error
	return rows, err
}

func (r *Repo) GetLatestScoreDate() (string, error) {
	var d string
	err := db.Dao.Model(&models.KhunterScore{}).Select("MAX(score_date)").Scan(&d).Error
	return d, err
}

func (r *Repo) GetLatestSignalDate() (string, error) {
	var d string
	err := db.Dao.Model(&models.KhunterSignal{}).Select("MAX(signal_date)").Scan(&d).Error
	return d, err
}

func (r *Repo) UpsertRunSummary(row *models.KhunterRunSummary) error {
	return db.Dao.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "trade_date"}},
		UpdateAll: true,
	}).Create(row).Error
}

func (r *Repo) GetRunSummaries(limit int) ([]models.KhunterRunSummary, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	var rows []models.KhunterRunSummary
	err := db.Dao.Order("trade_date DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

func (r *Repo) GetLatestRunSummaryDate() (string, error) {
	var d string
	err := db.Dao.Model(&models.KhunterRunSummary{}).Select("MAX(trade_date)").Scan(&d).Error
	return d, err
}

func (r *Repo) GetRunSummary(tradeDate string) (*models.KhunterRunSummary, error) {
	var row models.KhunterRunSummary
	err := db.Dao.Where("trade_date = ?", tradeDate).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
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
