// Task 11：日K同步就绪前置条件。确认引擎只允许在日K同步真正完成后运行：
// 就绪证据来自 kline_sync_log 的 pending/running/failed 任务、LastSyncDate 断点
// 覆盖与当日实际 K 线覆盖；15:30（kline_sync 触发）与 17:00（流水线触发）
// 只是 cron 触发时间，从不作为就绪证据。
package khunter

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/data/history"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// DataReadiness 日K同步就绪结论（plan 共享类型）：确认引擎的前置条件。
type DataReadiness struct {
	TradeDate string `json:"tradeDate"`
	Completed bool   `json:"completed"`
	Pending   int    `json:"pending"`
	Running   int    `json:"running"`
	Failed    int    `json:"failed"`
	Reason    string `json:"reason"`
}

// ErrDataNotReady 日K同步未完成：禁止确认（观察不受影响，照常保存）。
var ErrDataNotReady = errors.New("kline data not ready")

// RequireConfirmationReadiness 确认前置条件：同步未完成时返回携带
// pending/running/failed 计数与诊断原因的错误；完成时放行。
func RequireConfirmationReadiness(r DataReadiness) error {
	if r.Completed {
		return nil
	}
	return fmt.Errorf("%w: %s (pending=%d running=%d failed=%d)", ErrDataNotReady, r.Reason, r.Pending, r.Running, r.Failed)
}

// ComputeDataReadiness 从日K同步日志计算 tradeDate 的就绪状态（now 供测试注入）。
// 三个证据缺一不可：
//   - day 周期同步任务无 pending/running/failed；
//   - 全部 day 任务 LastSyncDate 断点的最小值已覆盖 tradeDate；
//   - 本地日K实际存在 tradeDate 的K线（当日覆盖）。
func ComputeDataReadiness(ctx context.Context, tradeDate string, now time.Time) DataReadiness {
	if now.IsZero() {
		now = time.Now()
	}
	r := DataReadiness{TradeDate: tradeDate}
	if tradeDate == "" {
		tradeDate = resolveTradeDateAsOf("", now)
		r.TradeDate = tradeDate
	}
	if db.Dao == nil {
		r.Reason = "数据库未初始化，无法核实日K同步状态"
		return r
	}

	// 1) day 周期同步任务计数
	type statusCount struct {
		Status string
		C      int64
	}
	var counts []statusCount
	if err := db.Dao.WithContext(ctx).Model(&models.KLineSyncLog{}).
		Where("period = ?", "day").
		Select("status, COUNT(*) AS c").
		Group("status").Scan(&counts).Error; err != nil {
		logger.SugaredLogger.Warnf("khunter 日K同步任务统计失败: %v", err)
		r.Reason = "日K同步任务统计失败: " + err.Error()
		return r
	}
	for _, row := range counts {
		switch row.Status {
		case history.SyncStatusPending:
			r.Pending += int(row.C)
		case history.SyncStatusRunning:
			r.Running += int(row.C)
		case history.SyncStatusFailed:
			r.Failed += int(row.C)
		}
	}

	// 2) LastSyncDate 断点覆盖（非空断点的最小值必须达到 tradeDate）
	var minLastPtr *string
	if err := db.Dao.WithContext(ctx).Model(&models.KLineSyncLog{}).
		Where("period = ? AND IFNULL(last_sync_date, '') <> ''", "day").
		Select("MIN(last_sync_date)").Scan(&minLastPtr).Error; err != nil {
		logger.SugaredLogger.Warnf("khunter 日K同步断点统计失败: %v", err)
	}
	minLast := ""
	if minLastPtr != nil {
		minLast = *minLastPtr
	}

	// 3) 当日覆盖：本地日K存在 tradeDate 的K线
	var covered int64
	db.Dao.WithContext(ctx).Model(&models.KLineBar{}).
		Where("period IN ('day','101') AND trade_date = ?", tradeDate).
		Limit(1).Count(&covered)

	blockers := make([]string, 0, 3)
	if r.Pending > 0 || r.Running > 0 || r.Failed > 0 {
		blockers = append(blockers, fmt.Sprintf("存在未完成的日K同步任务(pending=%d running=%d failed=%d)", r.Pending, r.Running, r.Failed))
	}
	switch {
	case minLast == "":
		blockers = append(blockers, "无日K同步断点记录（kline_sync 尚未成功跑完过一单）")
	case minLast < tradeDate:
		blockers = append(blockers, fmt.Sprintf("日K同步断点 %s 未覆盖 %s", minLast, tradeDate))
	}
	if covered == 0 {
		blockers = append(blockers, "本地日K无 "+tradeDate+" 的K线")
	}
	if len(blockers) == 0 {
		r.Completed = true
		r.Reason = "日K同步已完成"
	} else {
		r.Reason = strings.Join(blockers, "；")
	}
	return r
}
