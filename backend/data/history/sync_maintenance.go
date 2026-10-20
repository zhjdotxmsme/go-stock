package history

import (
	"context"
	"time"

	"go-stock/backend/data/datasource"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
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

// PrepareSyncQueueOnStartup 启动时回收僵死任务、延长过期 end_date，并后台扫描缺口。
func PrepareSyncQueueOnStartup(onMorePending func()) {
	ctx := context.Background()
	today := timeutil.TodayStr()

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
