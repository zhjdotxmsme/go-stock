package khunter

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"go-stock/backend/data/indicator"
	"go-stock/backend/db"
	"go-stock/backend/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 观察与交易状态机。
// 观察：观察中 → 已确认 / 已过期（终态只能由对应事务迁移，同日重跑不得重置）。
// 交易：持仓中 → 已退出；旧版本记录统一迁移为 历史待核。
const (
	ObsStatusPending   = "观察中"
	ObsStatusConfirmed = "已确认"
	ObsStatusExpired   = "已过期"

	HuntingStatusHolding = "持仓中"
	HuntingStatusExited  = "已退出"
	HuntingStatusLegacy  = "历史待核"
)

var (
	// ErrObservationNotPending 观察不在"观察中"状态：已确认/已过期只能由对应事务迁移，
	// 不能被重复确认产生第二笔交易。
	ErrObservationNotPending = errors.New("观察不在待确认状态")
	// ErrTradeAlreadyExists 同一股票同一确认日已存在入池记录。
	ErrTradeAlreadyExists = errors.New("该股当日已有入池记录")
)

// SaveObservations 批量保存观察（逐条幂等 upsert）。
func (r *Repo) SaveObservations(rows []models.KhunterObservation) error {
	for i := range rows {
		if err := r.UpsertObservation(&rows[i]); err != nil {
			return err
		}
	}
	return nil
}

// UpsertObservation 按 (code, signal_date) 幂等保存观察。
// 故意不覆盖已有 status：已确认/已过期只能由对应事务迁移，不能被同日重跑重置为观察中。
func (r *Repo) UpsertObservation(row *models.KhunterObservation) error {
	return db.Dao.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "code"}, {Name: "signal_date"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"name", "data_status", "trigger_price", "signal_low", "atr14",
			"confirm_expire", "score", "signal_risk_json", "risk_history_json",
			"reject_reason", "updated_at",
		}),
	}).Create(row).Error
}

// GetPendingObservations 返回确认窗口未关闭的观察中记录（供确认引擎按交易日推进）：
// 状态必须是观察中；确认有效期为空视为窗口未定（仍待确认），非空时要求 >= asOfDate。
func (r *Repo) GetPendingObservations(asOfDate string) ([]models.KhunterObservation, error) {
	var rows []models.KhunterObservation
	err := db.Dao.
		Where("status = ? AND (confirm_expire = '' OR confirm_expire IS NULL OR confirm_expire >= ?)",
			ObsStatusPending, asOfDate).
		Order("score DESC, code").Find(&rows).Error
	return rows, err
}

// ConfirmHunting 事务化确认：校验观察处于观察中 → 同股同确认日无既有入池记录 →
// 创建交易（持仓中，带真实信号日）→ 观察改已确认。任一步失败整体回滚，
// 同一观察重复确认返回 ErrObservationNotPending，不产生第二笔交易。
// 研究模式与盘中运行不得调用本方法（确认只放行给数据闸门 AllowConfirm 的生产路径）。
func (r *Repo) ConfirmHunting(observationID uint, trade *models.KhunterHunting) error {
	if trade == nil {
		return fmt.Errorf("确认入池缺少交易参数")
	}
	return db.Dao.Transaction(func(tx *gorm.DB) error {
		var obs models.KhunterObservation
		err := tx.Where("id = ?", observationID).First(&obs).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: 观察 %d 不存在", ErrObservationNotPending, observationID)
		}
		if err != nil {
			return err
		}
		if obs.Status != ObsStatusPending {
			return fmt.Errorf("%w: 观察 %d 当前状态 %s", ErrObservationNotPending, observationID, obs.Status)
		}
		if trade.Code != obs.Code {
			return fmt.Errorf("交易代码 %s 与观察代码 %s 不一致", trade.Code, obs.Code)
		}
		var n int64
		if err := tx.Model(&models.KhunterHunting{}).
			Where("code = ? AND enter_date = ?", trade.Code, trade.EnterDate).
			Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			return fmt.Errorf("%w: %s @ %s", ErrTradeAlreadyExists, trade.Code, trade.EnterDate)
		}
		trade.SignalDate = obs.SignalDate
		trade.Status = HuntingStatusHolding
		if err := tx.Create(trade).Error; err != nil {
			return err
		}
		return tx.Model(&models.KhunterObservation{}).Where("id = ?", observationID).
			Update("status", ObsStatusConfirmed).Error
	})
}

// ExitHunting 持仓退出：WHERE id=? AND status='持仓中' 防止重复退出覆盖既有结果；
// 目标不是持仓中（已退出/历史待核/不存在）时安全跳过（幂等空操作）。
func (r *Repo) ExitHunting(id uint, exitDate string, exitPrice float64, exitReason string) error {
	return db.Dao.Model(&models.KhunterHunting{}).
		Where("id = ? AND status = ?", id, HuntingStatusHolding).
		Updates(map[string]any{
			"status":      HuntingStatusExited,
			"exit_date":   exitDate,
			"exit_price":  exitPrice,
			"exit_reason": exitReason,
		}).Error
}

// BuildObservation 由评分结果、信号日 K 线和数据闸门结论构造观察记录（不入池）。
// 触发价取信号日最高价 H（T+1~T+3 触及可确认），信号低点取信号日最低价 L，
// ATR14 为信号日 Wilder 均幅；价格口径与扫描加载的本地日 K 一致，
// 确认时由确认引擎转换为不复权真实价。
func BuildObservation(sc models.KhunterScore, bars []models.KLineBar, decision DataDecision, signalDate string) models.KhunterObservation {
	last := bars[len(bars)-1]
	return models.KhunterObservation{
		Code:         sc.Code,
		Name:         sc.Name,
		SignalDate:   signalDate,
		Status:       ObsStatusPending,
		DataStatus:   decision.Status,
		TriggerPrice: last.High,
		SignalLow:    last.Low,
		ATR14:        ATR14(bars),
		Score:        sc.Total,
		RejectReason: strings.Join(decision.Reasons, "；"),
	}
}

// ATR14 信号日 Wilder 口径 14 日均幅；不足 14 根返回 0（确认引擎按无效风险距离处理）。
func ATR14(bars []models.KLineBar) float64 {
	if len(bars) < 14 {
		return 0
	}
	n := len(bars)
	high := make([]float64, n)
	low := make([]float64, n)
	close := make([]float64, n)
	for i, b := range bars {
		high[i], low[i], close[i] = b.High, b.Low, b.Close
	}
	atr := indicator.ATR(high, low, close, 14)
	v := atr[n-1]
	if math.IsNaN(v) || v < 0 {
		return 0
	}
	return v
}

// GetObservations 观察列表（status 为空返回全部），按信号日倒序、分数倒序。
// 盘中运行产出的 intraday 观察也包含在内——观察不是买入推荐。
func (r *Repo) GetObservations(status string) ([]models.KhunterObservation, error) {
	q := db.Dao.Model(&models.KhunterObservation{}).Order("signal_date DESC, score DESC, code")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var rows []models.KhunterObservation
	err := q.Find(&rows).Error
	return rows, err
}

// 同步完成前提已由 readiness.go 的真实同步日志证据取代（Task 11）：
// ComputeDataReadiness 从 kline_sync_log 的 pending/running/failed、LastSyncDate
// 断点覆盖与当日 K 线覆盖三方面判定；15:00/15:30/17:00 等时点只作触发，不作证据。
