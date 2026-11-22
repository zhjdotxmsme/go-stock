package khunter

import (
	"context"
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
		&models.KhunterObservation{}, &models.KhunterKronosPrediction{},
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

// GetTradeDates 返回 [start, end] 区间内真实出现过的交易日（按 sh000300 日K），升序。
func (r *Repo) GetTradeDates(ctx context.Context, start, end string) ([]string, error) {
	var dates []string
	err := db.Dao.Model(&models.KLineBar{}).
		Where("stock_code = ? AND period = ? AND adjusted = ? AND trade_date BETWEEN ? AND ?",
			"sh000300", "day", false, start, end).
		Distinct("trade_date").
		Order("trade_date ASC").
		Pluck("trade_date", &dates).Error
	if err != nil {
		return nil, err
	}
	if len(dates) > 0 {
		return dates, nil
	}
	// 回退：本地库未同步 sh000300（如只同步了 sh000001/sz399006）时，
	// 用市场日 K 覆盖度重建交易日序列（与 datasource.LatestMarketTradeDate 同一口径：
	// 计数达峰值 90% 且不少于 500 只，避免停牌/脏日期污染日历）。
	return r.marketTradeDates(start, end)
}

// marketTradeDates 由市场日 K 覆盖度推导 [start, end] 的真实交易日序列。
func (r *Repo) marketTradeDates(start, end string) ([]string, error) {
	type row struct {
		TradeDate string
		C         int64
	}
	var rows []row
	err := db.Dao.Raw(`
SELECT substr(trade_date, 1, 10) AS trade_date, COUNT(DISTINCT stock_code) AS c
FROM kline_bars
WHERE period IN ('day','101') AND adjusted = 1
  AND substr(trade_date, 1, 10) BETWEEN ? AND ?
  AND substr(trade_date, 1, 10) >= '1990-01-01'
GROUP BY 1`, start, end).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	var peak int64
	for _, row := range rows {
		if row.C > peak {
			peak = row.C
		}
	}
	need := int64(500)
	if floor := peak * 9 / 10; floor > need {
		need = floor
	}
	dates := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.TradeDate != "" && row.C >= need {
			dates = append(dates, row.TradeDate)
		}
	}
	return dates, nil
}

// GetHoldingExposure 返回所有持仓中交易的 entry_price * shares 之和。
func (r *Repo) GetHoldingExposure() (float64, error) {
	var exposure float64
	err := db.Dao.Model(&models.KhunterHunting{}).
		Where("status = ?", HuntingStatusHolding).
		Select("COALESCE(SUM(entry_price * shares), 0)").
		Scan(&exposure).Error
	return exposure, err
}

func (r *Repo) GetHuntingList(status string) ([]models.KhunterHunting, error) {
	var rows []models.KhunterHunting
	q := db.Dao.Order("enter_date DESC")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	return rows, q.Find(&rows).Error
}

// UpdateHoldingProtectAndTrack 幂等地更新持仓保护位与已持有交易日数：
// 仅当新 track_days 大于当前值时才写入，避免同日重跑导致重复累加。
func (r *Repo) UpdateHoldingProtectAndTrack(id uint, protectPrice float64, trackDays int) error {
	return db.Dao.Model(&models.KhunterHunting{}).
		Where("id = ? AND status = ? AND track_days < ?", id, HuntingStatusHolding, trackDays).
		Updates(map[string]any{
			"protect_price": protectPrice,
			"track_days":    trackDays,
		}).Error
}

// GetObservationBySignal 按 (code, signal_date) 读取观察记录；不存在返回 nil。
func (r *Repo) GetObservationBySignal(code, signalDate string) (*models.KhunterObservation, error) {
	var row models.KhunterObservation
	err := db.Dao.Where("code = ? AND signal_date = ?", code, signalDate).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &row, err
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

// SaveKronosPredictions 按 (code, signal_date) 幂等保存批量预测明细。
func (r *Repo) SaveKronosPredictions(rows []models.KhunterKronosPrediction) error {
	if len(rows) == 0 {
		return nil
	}
	return db.Dao.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "code"}, {Name: "signal_date"}},
		UpdateAll: true,
	}).Create(&rows).Error
}
