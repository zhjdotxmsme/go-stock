package history

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go-stock/backend/data/datasource"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/stockcode"
	"go-stock/backend/util/timeutil"

	"gorm.io/gorm"
)

const (
	SyncStatusPending   = "pending"
	SyncStatusRunning   = "running"
	SyncStatusDone      = "done"
	SyncStatusFailed    = "failed"
	SyncStatusPartial   = "partial"
)

// SyncTask represents a single K-line synchronization task.
type SyncTask struct {
	ID           uint      `json:"id"`
	StockCode    string    `json:"stockCode"`
	Period       string    `json:"period"`
	StartDate    string    `json:"startDate"`
	EndDate      string    `json:"endDate"`
	Adjusted     bool      `json:"adjusted"`
	Status       string    `json:"status"`
	ErrorMsg     string    `json:"errorMsg"`
	LastSyncDate string    `json:"lastSyncDate"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// SyncKLineForStock syncs K-line data for a single stock within a date range.
// It finds missing date ranges and fetches data from router.GetKLine for each gap.
func SyncKLineForStock(ctx context.Context, stockCode, period, startDate, endDate string, adjusted bool) error {
	if err := validateSyncParams(stockCode, period, startDate, endDate); err != nil {
		return fmt.Errorf("invalid sync parameters: %w", err)
	}
	// Rule 3：落库统一内部格式（生产库混有 tushare/bare 格式，读者按内部格式取数会漏）
	stockCode = stockcode.Normalize(stockCode)

	logger.SugaredLogger.Infof("Starting K-line sync for %s %s [%s - %s] adjusted=%v", stockCode, period, startDate, endDate, adjusted)

	store := datasource.NewKLineStore()
	router := datasource.GetRouter()

	missingRanges, err := findMissingRangesLocal(ctx, store, stockCode, period, startDate, endDate, adjusted)
	if err != nil {
		return fmt.Errorf("failed to find missing ranges for %s: %w", stockCode, err)
	}

	if len(missingRanges) == 0 {
		logger.SugaredLogger.Infof("No missing data for %s %s [%s - %s]", stockCode, period, startDate, endDate)
		return nil
	}

	logger.SugaredLogger.Infof("Found %d missing range(s) for %s %s", len(missingRanges), stockCode, period)

	for i, rng := range missingRanges {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			logger.SugaredLogger.Debugf("Syncing range %d/%d for %s: [%s - %s]", i+1, len(missingRanges), stockCode, rng.Start, rng.End)
			
			if err := syncDateRange(ctx, router, store, stockCode, period, rng.Start, rng.End, adjusted); err != nil {
				logger.SugaredLogger.Errorf("Failed to sync range [%s - %s] for %s: %v", rng.Start, rng.End, stockCode, err)
				return fmt.Errorf("sync failed for range [%s - %s]: %w", rng.Start, rng.End, err)
			}
		}
	}

	logger.SugaredLogger.Infof("Successfully completed K-line sync for %s %s [%s - %s]", stockCode, period, startDate, endDate)

	// 完成前校验：重算缺口。仍有缺口 = 源数据暂缺或拉取失败，
	// 返回错误让任务标 failed（可重试），而不是假 done 把缺口永久掩盖。
	remaining, err := findMissingRangesLocal(ctx, store, stockCode, period, startDate, endDate, adjusted)
	if err != nil {
		return fmt.Errorf("verify gaps after sync: %w", err)
	}
	if len(remaining) > 0 {
		first, last := remaining[0], remaining[len(remaining)-1]
		summary := first.Start
		if len(remaining) > 1 {
			summary = fmt.Sprintf("%s ~ %s 等 %d 段", first.Start, last.End, len(remaining))
		}
		return fmt.Errorf("补齐后仍有缺口未覆盖（%s），源数据暂缺或连续拉取失败", summary)
	}

	return nil
}

// findMissingRangesLocal computes missing date intervals using a local trading-day
// approximation (weekdays only). Unlike KLineStore.FindMissingDateRanges it does not
// call the holiday HTTP API per day, which would be prohibitively slow for multi-year
// ranges. Holidays simply yield no bars from the datasource and are skipped on upsert.
func findMissingRangesLocal(ctx context.Context, store *datasource.KLineStore, stockCode, period, startDate, endDate string, adjusted bool) ([]datasource.DateRange, error) {
	bars, err := store.QueryKLines(ctx, stockCode, period, startDate, endDate, adjusted)
	if err != nil {
		return nil, fmt.Errorf("query existing bars: %w", err)
	}

	existing := make(map[string]struct{}, len(bars))
	for _, b := range bars {
		existing[b.TradeDate] = struct{}{}
	}

	startT, err := timeutil.ParseDate(startDate)
	if err != nil {
		return nil, fmt.Errorf("parse start date: %w", err)
	}
	endT, err := timeutil.ParseDate(endDate)
	if err != nil {
		return nil, fmt.Errorf("parse end date: %w", err)
	}

	var missing []datasource.DateRange
	var rangeStart *time.Time

	for d := startT; !d.After(endT); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")

		// Skip non-trading days (weekends)
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			continue
		}

		_, exists := existing[dateStr]
		if !exists {
			if rangeStart == nil {
				ts := d
				rangeStart = &ts
			}
		} else if rangeStart != nil {
			prev := d.AddDate(0, 0, -1)
			missing = append(missing, datasource.DateRange{
				Start: rangeStart.Format("2006-01-02"),
				End:   prev.Format("2006-01-02"),
			})
			rangeStart = nil
		}
	}

	if rangeStart != nil {
		missing = append(missing, datasource.DateRange{
			Start: rangeStart.Format("2006-01-02"),
			End:   endT.Format("2006-01-02"),
		})
	}

	return missing, nil
}

// RunSyncTask executes a single sync task with checkpointing.
func RunSyncTask(ctx context.Context, task *SyncTask) error {
	if task == nil {
		return fmt.Errorf("sync task cannot be nil")
	}

	if task.Status == SyncStatusDone {
		logger.SugaredLogger.Infof("Task already completed: %s", task.StockCode)
		return nil
	}

	task.Status = SyncStatusRunning
	task.ErrorMsg = ""
	if err := updateSyncTask(task); err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	logger.SugaredLogger.Infof("Running sync task: %s %s [%s - %s] adjusted=%v", task.StockCode, task.Period, task.StartDate, task.EndDate, task.Adjusted)

	startDate := task.StartDate
	if task.LastSyncDate != "" && task.LastSyncDate >= startDate {
		startDate = task.LastSyncDate
	}

	err := SyncKLineForStock(ctx, task.StockCode, task.Period, startDate, task.EndDate, task.Adjusted)

	if err != nil {
		task.Status = SyncStatusFailed
		task.ErrorMsg = err.Error()
		logger.SugaredLogger.Errorf("Sync task failed: %s - %v", task.StockCode, err)
	} else {
		task.Status = SyncStatusDone
		task.LastSyncDate = task.EndDate
		task.ErrorMsg = ""
		logger.SugaredLogger.Infof("Sync task completed successfully: %s", task.StockCode)
	}

	if updateErr := updateSyncTask(task); updateErr != nil {
		logger.SugaredLogger.Errorf("Failed to update final task status: %v", updateErr)
		if err == nil {
			return updateErr
		}
	}

	return err
}

// GetAllSyncTasks retrieves all sync tasks from the database.
func GetAllSyncTasks(ctx context.Context) ([]*SyncTask, error) {
	var syncLogs []models.KLineSyncLog
	err := db.Dao.WithContext(ctx).Order("created_at DESC").Find(&syncLogs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query sync tasks: %w", err)
	}

	tasks := make([]*SyncTask, 0, len(syncLogs))
	for _, log := range syncLogs {
		tasks = append(tasks, syncLogToTask(&log))
	}

	return tasks, nil
}

// CreateSyncTask creates a new sync task and persists it.
func CreateSyncTask(ctx context.Context, stockCode, period, startDate, endDate string, adjusted bool) error {
	if err := validateSyncParams(stockCode, period, startDate, endDate); err != nil {
		return fmt.Errorf("invalid sync parameters: %w", err)
	}
	// Rule 3：任务与落库统一内部格式
	stockCode = stockcode.Normalize(stockCode)

	var existing models.KLineSyncLog
	err := db.Dao.WithContext(ctx).
		Where("stock_code = ? AND period = ? AND adjusted = ?", stockCode, period, adjusted).
		First(&existing).Error

	if err == nil {
		if existing.Status == SyncStatusRunning && time.Since(existing.UpdatedAt) < DefaultStaleRunningAge {
			return nil
		}
		if existing.Status == SyncStatusDone &&
			existing.EndDate >= endDate && existing.StartDate <= startDate {
			return nil
		}
		if startDate < existing.StartDate || existing.StartDate == "" {
			existing.StartDate = startDate
		}
		if endDate > existing.EndDate {
			existing.EndDate = endDate
		}
		existing.ExpectedCount = estimateExpectedCount(period, existing.StartDate, existing.EndDate)
		existing.Status = SyncStatusPending
		existing.ErrorMsg = ""
		existing.UpdatedAt = time.Now()

		if err := db.Dao.WithContext(ctx).Save(&existing).Error; err != nil {
			return fmt.Errorf("failed to update existing sync task: %w", err)
		}

		logger.SugaredLogger.Debugf("Updated sync task for %s %s [%s-%s]", stockCode, period, existing.StartDate, existing.EndDate)
		return nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to check existing sync task: %w", err)
	}

	syncLog := &models.KLineSyncLog{
		StockCode:     stockCode,
		Period:        period,
		Adjusted:      adjusted,
		StartDate:     startDate,
		EndDate:       endDate,
		SyncedCount:   0,
		ExpectedCount: estimateExpectedCount(period, startDate, endDate),
		Status:        SyncStatusPending,
		ErrorMsg:      "",
		UpdatedAt:     time.Now(),
	}

	if err := db.Dao.WithContext(ctx).Create(syncLog).Error; err != nil {
		return fmt.Errorf("failed to create sync task: %w", err)
	}

	logger.SugaredLogger.Infof("Created new sync task for %s %s [%s - %s]", stockCode, period, startDate, endDate)
	return nil
}

// syncDateRange syncs K-line data for a specific date range.
func syncDateRange(ctx context.Context, router *datasource.Router, store *datasource.KLineStore, stockCode, period, startDate, endDate string, adjusted bool) error {
	// Estimate bars from the range span instead of always pulling 2000:
	// trading days ≈ calendar days × 5/7, clamped to a sane minimum.
	startT, err := timeutil.ParseDate(startDate)
	if err != nil {
		return fmt.Errorf("invalid start date %q: %w", startDate, err)
	}
	endT, err := timeutil.ParseDate(endDate)
	if err != nil {
		return fmt.Errorf("invalid end date %q: %w", endDate, err)
	}
	days := int(endT.Sub(startT).Hours()/24) + 1
	count := days * 5 / 7
	if period != "day" {
		// week/month bars span more per bar; keep a small multiplier
		count = days/7 + 2
		if period == "month" {
			count = days/28 + 2
		}
	}
	if count < 50 {
		count = 50
	}
	if count > 2000 {
		count = 2000
	}

	// 落库时机由本函数控制（先做除权检测），不走 GetKLine 的异步持久化。
	// 锚定 endDate 取数：历史中段缺口用"最新 N 根"语义永远覆盖不到（旧实现
	// 取到的是尾部数据，过滤后区间内 0 根还返回成功 → 任务假 done，缺口永不补）。
	data, err := router.FetchKLineNoPersistBefore(ctx, stockCode, period, count, endDate)
	if err != nil {
		return fmt.Errorf("failed to fetch K-line data from router: %w", err)
	}

	if data == nil || len(data.Bars) == 0 {
		logger.SugaredLogger.Warnf("No K-line data returned for %s %s [%s - %s]", stockCode, period, startDate, endDate)
		return nil
	}

	bars := datasource.BarsFromKLineData(stockCode, period, "sync", adjusted, data)

	var filtered []models.KLineBar
	for _, bar := range bars {
		if bar.TradeDate >= startDate && bar.TradeDate <= endDate {
			filtered = append(filtered, bar)
		}
	}

	if len(filtered) == 0 {
		// 旧行为是 Warn+return nil（任务标 done）——历史中段缺口被永久掩盖。
		// 改为报错：任务标 failed 可重试；长期停牌属源数据事实，同样如实暴露。
		return fmt.Errorf("区间 [%s - %s] 未取到任何 K 线（源数据暂缺或长期停牌）", startDate, endDate)
	}

	// 除权防线：前复权序列与库内重叠日收盘系统性错位（除权导致 qfq 整体平移）时，
	// 缺口检测认为日期已存在不会重拉，必须全量重拉覆盖，否则增量追加会写坏历史。
	if adjusted {
		storedSnapshot, qerr := store.QueryKLines(ctx, stockCode, period, filtered[0].TradeDate, filtered[len(filtered)-1].TradeDate, true)
		if qerr != nil {
			logger.SugaredLogger.Warnf("除权检测读库失败 %s: %v", stockCode, qerr)
		} else if datasource.DetectAdjustedSeriesShift(storedSnapshot, filtered, 8) {
			logger.SugaredLogger.Warnf("检测到前复权序列平移（除权）%s [%s - %s]，全量重拉覆盖", stockCode, startDate, endDate)
			if _, rerr := router.RefreshKLineFull(ctx, stockCode, period, len(filtered)); rerr == nil {
				logger.SugaredLogger.Infof("除权全量覆盖完成 %s [%s - %s]", stockCode, startDate, endDate)
				updateCheckpoint(stockCode, period, adjusted, filtered[len(filtered)-1].TradeDate, len(filtered))
				return nil
			} else {
				logger.SugaredLogger.Errorf("除权全量重拉失败 %s: %v，回退常规增量追加", stockCode, rerr)
			}
		}
	}

	if err := store.UpsertKLines(ctx, filtered); err != nil {
		return fmt.Errorf("failed to upsert K-line bars: %w", err)
	}

	logger.SugaredLogger.Infof("Synced %d bars for %s [%s - %s]", len(filtered), stockCode, startDate, endDate)

	updateCheckpoint(stockCode, period, adjusted, filtered[len(filtered)-1].TradeDate, len(filtered))

	return nil
}

// updateCheckpoint updates the sync log checkpoint for a stock.
func updateCheckpoint(stockCode, period string, adjusted bool, lastDate string, count int) {
	var log models.KLineSyncLog
	err := db.Dao.Where("stock_code = ? AND period = ? AND adjusted = ?", stockCode, period, adjusted).First(&log).Error

	if err != nil {
		if err.Error() != "record not found" {
			logger.SugaredLogger.Errorf("Failed to query sync log for checkpoint: %v", err)
		}
		return
	}

	updates := map[string]interface{}{
		"synced_count": log.SyncedCount + count,
		"updated_at":   time.Now(),
	}
	// 断点前移：YYYY-MM-DD 字典序即时间序，取 max 防止并发区间乱序回退。
	// 即使任务最终 failed，已落库区间的进度也保留，重启后从断点续传。
	if lastDate > log.LastSyncDate {
		updates["last_sync_date"] = lastDate
	}

	if err := db.Dao.Model(&log).Updates(updates).Error; err != nil {
		logger.SugaredLogger.Errorf("Failed to update checkpoint for %s: %v", stockCode, err)
	}
}

// updateSyncTask updates the sync task in the database.
func updateSyncTask(task *SyncTask) error {
	var log models.KLineSyncLog
	err := db.Dao.Where("stock_code = ? AND period = ? AND adjusted = ?", task.StockCode, task.Period, task.Adjusted).First(&log).Error

	if err != nil {
		if err.Error() == "record not found" {
			log = models.KLineSyncLog{
				StockCode:     task.StockCode,
				Period:        task.Period,
				Adjusted:      task.Adjusted,
				StartDate:     task.StartDate,
				EndDate:       task.EndDate,
				SyncedCount:   0,
				ExpectedCount: estimateExpectedCount(task.Period, task.StartDate, task.EndDate),
				Status:        task.Status,
				ErrorMsg:      task.ErrorMsg,
				LastSyncDate:  task.LastSyncDate,
				UpdatedAt:     time.Now(),
			}
			return db.Dao.Create(&log).Error
		}
		return fmt.Errorf("failed to find sync task: %w", err)
	}

	return db.Dao.Model(&log).Updates(map[string]interface{}{
		"status":         task.Status,
		"error_msg":      task.ErrorMsg,
		"last_sync_date": task.LastSyncDate,
		"updated_at":     time.Now(),
	}).Error
}

// syncLogToTask converts a KLineSyncLog to a SyncTask.
// LastSyncDate resumes from the persisted checkpoint so restarts only fetch
// bars after the last successfully synced trade date.
func syncLogToTask(log *models.KLineSyncLog) *SyncTask {
	return &SyncTask{
		ID:           log.ID,
		StockCode:    log.StockCode,
		Period:       log.Period,
		StartDate:    log.StartDate,
		EndDate:      log.EndDate,
		Adjusted:     log.Adjusted,
		Status:       log.Status,
		ErrorMsg:     log.ErrorMsg,
		LastSyncDate: log.LastSyncDate,
		CreatedAt:    log.UpdatedAt,
		UpdatedAt:    log.UpdatedAt,
	}
}

// estimateExpectedCount estimates the number of K-line bars in [startDate, endDate].
// day: weekdays (~5/7 of days); week: weekly bars; month: monthly bars.
func estimateExpectedCount(period, startDate, endDate string) int {
	start, err1 := timeutil.ParseDate(startDate)
	end, err2 := timeutil.ParseDate(endDate)
	if err1 != nil || err2 != nil || end.Before(start) {
		return 0
	}

	days := int(end.Sub(start).Hours()/24) + 1
	switch period {
	case "week":
		return days/7 + 1
	case "month":
		return (end.Year()-start.Year())*12 + int(end.Month()-start.Month()) + 1
	default: // day
		return days * 5 / 7
	}
}

// validateSyncParams validates the sync parameters.
func validateSyncParams(stockCode, period, startDate, endDate string) error {
	if stockCode == "" {
		return fmt.Errorf("stock code cannot be empty")
	}
	if period == "" {
		return fmt.Errorf("period cannot be empty")
	}
	if period != "day" && period != "week" && period != "month" {
		return fmt.Errorf("invalid period: %s (must be day, week, or month)", period)
	}
	if startDate == "" {
		return fmt.Errorf("start date cannot be empty")
	}
	if endDate == "" {
		return fmt.Errorf("end date cannot be empty")
	}
	
	if _, err := timeutil.ParseDate(startDate); err != nil {
		return fmt.Errorf("invalid start date format: %s (expected YYYY-MM-DD)", startDate)
	}
	if _, err := timeutil.ParseDate(endDate); err != nil {
		return fmt.Errorf("invalid end date format: %s (expected YYYY-MM-DD)", endDate)
	}
	
	start, _ := timeutil.ParseDate(startDate)
	end, _ := timeutil.ParseDate(endDate)
	if start.After(end) {
		return fmt.Errorf("start date cannot be after end date")
	}
	
	return nil
}