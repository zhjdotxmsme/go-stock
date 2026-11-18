package khunter

import (
	"errors"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// EnsureMigrate 懒迁移 khunter 全部表（幂等）。
// 旧版 khunter_hunting 数据先在事务内去重并迁移为 历史待核，再由 AutoMigrate
// 创建 (code,signal_date)/(code,enter_date) 唯一索引，避免存量重复行导致建索引失败。
func EnsureMigrate() error {
	if db.Dao == nil {
		return nil
	}
	if err := migrateHuntingLegacy(); err != nil {
		return err
	}
	return db.Dao.AutoMigrate(
		&models.KhunterSignal{}, &models.KhunterScore{}, &models.KhunterHunting{},
		&models.KhunterObservation{},
		&models.KhunterMoneyFlowDaily{}, &models.KhunterRiskLevel{}, &models.KhunterEvent{},
		&models.KhunterRunSummary{},
		&models.KhunterResearchRecord{}, &models.KhunterFundamentalSnapshot{}, &models.KhunterSectorSnapshot{},
	)
}

// migrateHuntingLegacy 迁移旧版 khunter_hunting 数据（幂等）：
//  1. 补齐 signal_date 列（AutoMigrate 建唯一索引前该列必须存在且已回填）；
//  2. 事务内：旧 signal_date 用 enter_date 补齐；旧状态（追踪中/已移除/空）改为
//     历史待核；同一 (code, signal_date) 与 (code, enter_date) 分组重复行保留最新
//     ID 并删除其余行。
//  3. 新状态的 持仓中/已退出 交易不受影响，重复迁移是空操作。
func migrateHuntingLegacy() error {
	if !db.Dao.Migrator().HasTable(&models.KhunterHunting{}) {
		return nil // 全新库：AutoMigrate 直接建表（含唯一索引）
	}
	if !db.Dao.Migrator().HasColumn(&models.KhunterHunting{}, "SignalDate") {
		if err := db.Dao.Migrator().AddColumn(&models.KhunterHunting{}, "SignalDate"); err != nil {
			return err
		}
	}
	return db.Dao.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`UPDATE khunter_hunting
			SET signal_date = enter_date
			WHERE signal_date IS NULL OR signal_date = ''`).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.KhunterHunting{}).
			Where("status IS NULL OR status = '' OR status IN ?", []string{"追踪中", "已移除"}).
			Update("status", HuntingStatusLegacy).Error; err != nil {
			return err
		}
		if err := tx.Exec(`DELETE FROM khunter_hunting WHERE id NOT IN
			(SELECT MAX(id) FROM khunter_hunting GROUP BY code, signal_date)`).Error; err != nil {
			return err
		}
		return tx.Exec(`DELETE FROM khunter_hunting WHERE id NOT IN
			(SELECT MAX(id) FROM khunter_hunting GROUP BY code, enter_date)`).Error
	})
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

// GetMoneyFlowAsOf 返回 asOfDate 及之前最近 days 天资金流，正序（日期升序）。
// 历史时点查询：绝不返回 date > asOfDate 的行。
func (r *Repo) GetMoneyFlowAsOf(code, asOfDate string, days int) ([]models.KhunterMoneyFlowDaily, error) {
	var rows []models.KhunterMoneyFlowDaily
	err := db.Dao.Where("code = ? AND date <= ?", code, asOfDate).
		Order("date DESC").Limit(days).Find(&rows).Error
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return rows, err
}

// GetMoneyFlow 兼容包装：按今天取最近 days 天资金流（其他调用方沿用旧入口）。
func (r *Repo) GetMoneyFlow(code string, days int) ([]models.KhunterMoneyFlowDaily, error) {
	return r.GetMoneyFlowAsOf(code, time.Now().Format("2006-01-02"), days)
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

// ErrSnapshotUnavailable 表示评分时点之前没有任何已落库的输入快照，历史结果不可回测。
var ErrSnapshotUnavailable = errors.New("point-in-time snapshot unavailable")

func (r *Repo) UpsertFundamentalSnapshot(row *models.KhunterFundamentalSnapshot) error {
	return db.Dao.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "code"}, {Name: "available_date"}},
		UpdateAll: true,
	}).Create(row).Error
}

func (r *Repo) UpsertSectorSnapshot(row *models.KhunterSectorSnapshot) error {
	return db.Dao.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "code"}, {Name: "available_date"}},
		UpdateAll: true,
	}).Create(row).Error
}

// GetFundamentalSnapshotAsOf 返回 asOfDate 当日已可用的最新基本面快照：
// 绝不返回 available_date > asOfDate 的行；没有可用快照时返回 ErrSnapshotUnavailable。
func (r *Repo) GetFundamentalSnapshotAsOf(code, asOfDate string) (models.KhunterFundamentalSnapshot, error) {
	var row models.KhunterFundamentalSnapshot
	err := db.Dao.Where("code=? AND available_date<=?", code, asOfDate).
		Order("available_date DESC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return row, ErrSnapshotUnavailable
	}
	return row, err
}

// GetSectorSnapshotAsOf 返回 asOfDate 当日已可用的最新板块快照，语义同 GetFundamentalSnapshotAsOf。
func (r *Repo) GetSectorSnapshotAsOf(code, asOfDate string) (models.KhunterSectorSnapshot, error) {
	var row models.KhunterSectorSnapshot
	err := db.Dao.Where("code=? AND available_date<=?", code, asOfDate).
		Order("available_date DESC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return row, ErrSnapshotUnavailable
	}
	return row, err
}
