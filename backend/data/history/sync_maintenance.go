package history

import (
	"context"
	"time"

	"go-stock/backend/data/datasource"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/stockcode"
	"go-stock/backend/util/timeutil"
)

// DefaultStaleRunningAge running 超过该时长视为僵死（进程崩溃未写回状态）。
const DefaultStaleRunningAge = 20 * time.Minute

// ReclaimStaleRunningSyncTasks 进程重启后把长期未更新的 running 任务改回 pending。
func ReclaimStaleRunningSyncTasks(ctx context.Context, maxAge time.Duration) (int64, error) {
	if maxAge <= 0 {
		maxAge = DefaultStaleRunningAge
	}
	cutoff := time.Now().Add(-maxAge)
	res := db.Dao.WithContext(ctx).Model(&models.KLineSyncLog{}).
		Where("status = ? AND updated_at < ?", SyncStatusRunning, cutoff).
		Updates(map[string]interface{}{
			"status":     SyncStatusPending,
			"error_msg":  "recovered: stale running after restart",
			"updated_at": time.Now(),
		})
	return res.RowsAffected, res.Error
}

// BumpDoneEndDateBefore 将已完成但 end_date 早于 today 的任务延长到今天并重新排队（只补增量缺口）。
func BumpDoneEndDateBefore(ctx context.Context, today string) (int64, error) {
	if today == "" {
		today = timeutil.TodayStr()
	}
	res := db.Dao.WithContext(ctx).Model(&models.KLineSyncLog{}).
		Where("status = ? AND end_date < ?", SyncStatusDone, today).
		Updates(map[string]interface{}{
			"end_date":   today,
			"status":     SyncStatusPending,
			"error_msg":  "",
			"updated_at": time.Now(),
		})
	return res.RowsAffected, res.Error
}

// RequeueDoneWithMissingBars 扫描 marked done 但本地仍有缺口的任务，重新 pending（limit 控制启动耗时）。
func RequeueDoneWithMissingBars(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 400
	}
	var logs []models.KLineSyncLog
	if err := db.Dao.WithContext(ctx).
		Where("status = ?", SyncStatusDone).
		Order("updated_at ASC").
		Limit(limit).
		Find(&logs).Error; err != nil {
		return 0, err
	}
	store := datasource.NewKLineStore()
	requeued := 0
	for _, log := range logs {
		missing, err := findMissingRangesLocal(ctx, store, log.StockCode, log.Period, log.StartDate, log.EndDate, log.Adjusted)
		if err != nil || len(missing) == 0 {
			continue
		}
		if err := db.Dao.WithContext(ctx).Model(&models.KLineSyncLog{}).Where("id = ?", log.ID).
			Updates(map[string]interface{}{
				"status":     SyncStatusPending,
				"error_msg":  "",
				"updated_at": time.Now(),
			}).Error; err != nil {
			logger.SugaredLogger.Warnf("requeue sync %s: %v", log.StockCode, err)
			continue
		}
		requeued++
	}
	return requeued, nil
}

// RepairBrokenSyncTasks 修复历史假完成（syncDateRange 旧缺陷：补历史区间时拉的是
// "最新 N 根"，过滤后区间内 0 根也返回成功 → 3892 个 done 任务仅落 55/1310 根）。
//  1. 日志代码统一为内部格式（tushare/bare → sz300780；内部格式行已存在时删旧行）；
//  2. done 但 synced_count 明显低于 expected 的任务批量重置为 pending 重新补齐。
// 幂等：修复后的任务只有真正补齐才会再标 done。
func RepairBrokenSyncTasks(ctx context.Context) (int64, error) {
	// 1) 代码格式收敛
	var rows []models.KLineSyncLog
	if err := db.Dao.WithContext(ctx).
		Where("stock_code NOT GLOB '[a-z]*'").
		Find(&rows).Error; err != nil {
		return 0, err
	}
	renamed := int64(0)
	for _, row := range rows {
		norm := stockcode.Normalize(row.StockCode)
		if norm == "" || norm == row.StockCode {
			continue
		}
		var cnt int64
		if err := db.Dao.WithContext(ctx).Model(&models.KLineSyncLog{}).
			Where("stock_code = ? AND period = ? AND adjusted = ?", norm, row.Period, row.Adjusted).
			Count(&cnt).Error; err != nil {
			continue
		}
		if cnt > 0 {
			// 内部格式行已存在：删除格式错误的旧行，避免重复排队
			if err := db.Dao.WithContext(ctx).Delete(&models.KLineSyncLog{}, row.ID).Error; err != nil {
				logger.SugaredLogger.Warnf("repair sync log: delete %s failed: %v", row.StockCode, err)
			}
			continue
		}
		if err := db.Dao.WithContext(ctx).Model(&models.KLineSyncLog{}).
			Where("id = ?", row.ID).Update("stock_code", norm).Error; err != nil {
			logger.SugaredLogger.Warnf("repair sync log: rename %s failed: %v", row.StockCode, err)
			continue
		}
		renamed++
	}

	// 2) 假完成批量重置（synced_count 低于 expected 90% 视为未完成）
	res := db.Dao.WithContext(ctx).Model(&models.KLineSyncLog{}).
		Where("status = ? AND expected_count > 0 AND synced_count * 10 < expected_count * 9", SyncStatusDone).
		Updates(map[string]interface{}{
			"status":     SyncStatusPending,
			"error_msg":  "repair: done 但同步条数远低于预期（历史缺口），重新补齐",
			"updated_at": time.Now(),
		})
	return res.RowsAffected + renamed, res.Error
}

// PrepareSyncQueueOnStartup 启动时回收僵死任务、延长过期 end_date，并后台扫描缺口。
func PrepareSyncQueueOnStartup(onMorePending func()) {
	ctx := context.Background()
	today := timeutil.TodayStr()

	if n, err := RepairBrokenSyncTasks(ctx); err != nil {
		logger.SugaredLogger.Warnf("repair broken sync tasks: %v", err)
	} else if n > 0 {
		logger.SugaredLogger.Infof("K线同步：修复 %d 条历史假完成/格式错误任务，重新排队补齐", n)
	}

	if n, err := ReclaimStaleRunningSyncTasks(ctx, DefaultStaleRunningAge); err != nil {
		logger.SugaredLogger.Warnf("reclaim stale running sync: %v", err)
	} else if n > 0 {
		logger.SugaredLogger.Infof("K线同步：回收 %d 条僵死 running 任务", n)
	}

	if n, err := BumpDoneEndDateBefore(ctx, today); err != nil {
		logger.SugaredLogger.Warnf("bump sync end_date: %v", err)
	} else if n > 0 {
		logger.SugaredLogger.Infof("K线同步：%d 条 done 任务延长 end_date 至 %s", n, today)
	}

	go func() {
		rq, err := RequeueDoneWithMissingBars(ctx, 500)
		if err != nil {
			logger.SugaredLogger.Warnf("requeue done with gaps: %v", err)
			return
		}
		if rq > 0 {
			logger.SugaredLogger.Infof("K线同步：发现 %d 条 done 任务本地仍有缺口，已重新排队", rq)
			if onMorePending != nil {
				onMorePending()
			}
		}
	}()
}
