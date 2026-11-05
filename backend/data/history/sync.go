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
	SyncStatusPending = "pending"
	SyncStatusRunning = "running"
	SyncStatusDone    = "done"
	SyncStatusFailed  = "failed"
	SyncStatusPartial = "partial"
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
	missingRanges = subtractEmptySpans(missingRanges, loadEmptySpans(ctx, stockCode, period, adjusted))

	if len(missingRanges) == 0 {
		logger.SugaredLogger.Infof("No missing data for %s %s [%s - %s]", stockCode, period, startDate, endDate)
		return nil
	}

	logger.SugaredLogger.Infof("Found %d missing range(s) for %s %s", len(missingRanges), stockCode, period)

	for i, rng := range missingRanges {
		if !shouldSyncGap(rng, startDate, endDate) {
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			logger.SugaredLogger.Debugf("Syncing range %d/%d for %s: [%s - %s]", i+1, len(missingRanges), stockCode, rng.Start, rng.End)

			emptyStart, emptyEnd, err := syncDateRange(ctx, router, store, stockCode, period, rng.Start, rng.End, adjusted, startDate, endDate)
			if err != nil {
				logger.SugaredLogger.Errorf("Failed to sync range [%s - %s] for %s: %v", rng.Start, rng.End, stockCode, err)
				return fmt.Errorf("sync failed for range [%s - %s]: %w", rng.Start, rng.End, err)
			}
			// 贴着任务结束日的空窗不落库。否则数据源少给的尾部会被当成永久停牌，断点再跳过去就补不回来。
			if emptyStart != "" && emptyEnd != "" && emptyEnd < endDate {
				saveEmptySpan(ctx, stockCode, period, adjusted, emptyStart, emptyEnd)
			}
		}
	}

	logger.SugaredLogger.Infof("Successfully completed K-line sync for %s %s [%s - %s]", stockCode, period, startDate, endDate)

	// 完成前只拦截「长于长假、且没有被数据源核实为空窗」的缺口。
	// 周末之外的法定假日、上市前工作日不是失败：假完成的根因是拉错窗口后把 0 根当成功，
	// 那类情况 decideSyncChunk 会标成 unanchored/unproven 并返回错误，不会走到这里。
	remaining, err := findMissingRangesLocal(ctx, store, stockCode, period, startDate, endDate, adjusted)
	if err != nil {
		return fmt.Errorf("verify gaps after sync: %w", err)
	}
	remaining = subtractEmptySpans(remaining, loadEmptySpans(ctx, stockCode, period, adjusted))
	blocking := blockingGaps(remaining)
	if len(blocking) > 0 {
		first, last := blocking[0], blocking[len(blocking)-1]
		summary := first.Start + "~" + first.End
		if len(blocking) > 1 {
			summary = fmt.Sprintf("%s ~ %s 等 %d 段", first.Start, last.End, len(blocking))
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
		task.LastSyncDate = checkpointFromGaps(task.StartDate, task.EndDate, openGaps(ctx, task))
		task.ErrorMsg = ""
		alignDoneTask(ctx, task)
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

// syncDateRange 按结束日向前分块拉取 [startDate, endDate]。
// 返回的空窗是数据源已核实、该区间内没有交易日（上市前或整段停牌）。
// progressStart/progressEnd 是任务全窗，用于把 synced_count 写成库内实数而不是累加。
// 不推进 last_sync_date：尾部块成功不代表前缀已齐，断点只在整单成功时写成 endDate。
func syncDateRange(ctx context.Context, router *datasource.Router, store *datasource.KLineStore, stockCode, period, startDate, endDate string, adjusted bool, progressStart, progressEnd string) (string, string, error) {
	if _, err := timeutil.ParseDate(startDate); err != nil {
		return "", "", fmt.Errorf("invalid start date %q: %w", startDate, err)
	}
	if _, err := timeutil.ParseDate(endDate); err != nil {
		return "", "", fmt.Errorf("invalid end date %q: %w", endDate, err)
	}

	cursorEnd := endDate
	for {
		if err := ctx.Err(); err != nil {
			return "", "", err
		}
		count := chunkCount(period, startDate, cursorEnd)
		// 落库时机由本函数控制（先做除权检测），不走 GetKLine 的异步持久化。
		// 锚定 cursorEnd：历史中段缺口用「最新 N 根」永远覆盖不到。
		data, err := router.FetchKLineNoPersistBefore(ctx, stockCode, period, count, cursorEnd)
		if errors.Is(err, datasource.ErrNoBarsBefore) {
			// 上市前或整段无交易。记空窗后继续后面的缺口，不要拿最新 K 线冒充这段。
			logger.SugaredLogger.Infof("区间 [%s - %s] 锚定源确认没有 K 线，记为空窗 %s", startDate, cursorEnd, stockCode)
			return startDate, cursorEnd, nil
		}
		if err != nil {
			return "", "", fmt.Errorf("failed to fetch K-line data from router: %w", err)
		}
		if data == nil || len(data.Bars) == 0 {
			return "", "", fmt.Errorf("区间 [%s - %s] 未取到任何 K 线（源数据暂缺或长期停牌）", startDate, cursorEnd)
		}

		bars := datasource.BarsFromKLineData(stockCode, period, "sync", adjusted, data)
		dates := make([]string, 0, len(bars))
		for _, bar := range bars {
			if bar.TradeDate != "" {
				dates = append(dates, bar.TradeDate)
			}
		}
		plan := decideSyncChunk(startDate, cursorEnd, dates, count)
		if plan.Action == chunkUnproven {
			return "", "", fmt.Errorf("区间 [%s - %s] 未取到任何 K 线（源数据暂缺或长期停牌）", startDate, cursorEnd)
		}
		if plan.Action == chunkUnanchored {
			return "", "", fmt.Errorf("区间 [%s - %s] 锚定未被遵守，历史缺口不能用最新一段代替", startDate, cursorEnd)
		}

		var filtered []models.KLineBar
		for _, bar := range bars {
			if bar.TradeDate >= startDate && bar.TradeDate <= cursorEnd {
				filtered = append(filtered, bar)
			}
		}
		if len(filtered) > 0 {
			if adjusted {
				storedSnapshot, qerr := store.QueryKLines(ctx, stockCode, period, filtered[0].TradeDate, filtered[len(filtered)-1].TradeDate, true)
				if qerr != nil {
					logger.SugaredLogger.Warnf("除权检测读库失败 %s: %v", stockCode, qerr)
				} else if datasource.DetectAdjustedSeriesShift(storedSnapshot, filtered, 8) {
					logger.SugaredLogger.Warnf("检测到前复权序列平移（除权）%s [%s - %s]，全量重拉覆盖", stockCode, startDate, endDate)
					if _, rerr := router.RefreshKLineFull(ctx, stockCode, period, len(filtered)); rerr == nil {
						logger.SugaredLogger.Infof("除权全量覆盖完成 %s [%s - %s]", stockCode, startDate, endDate)
						refreshSyncedCount(ctx, stockCode, period, adjusted, progressStart, progressEnd)
						return "", "", nil
					}
					logger.SugaredLogger.Errorf("除权全量重拉失败 %s，回退常规增量追加", stockCode)
				}
			}
			if err := store.UpsertKLines(ctx, filtered); err != nil {
				return "", "", fmt.Errorf("failed to upsert K-line bars: %w", err)
			}
			logger.SugaredLogger.Infof("Synced %d bars for %s [%s - %s]", len(filtered), stockCode, startDate, cursorEnd)
			refreshSyncedCount(ctx, stockCode, period, adjusted, progressStart, progressEnd)
		}

		switch plan.Action {
		case chunkEmpty:
			return startDate, plan.EmptyEnd, nil
		case chunkPrefix:
			return startDate, plan.EmptyEnd, nil
		case chunkContinue:
			if plan.NextEnd == "" || plan.NextEnd >= cursorEnd {
				return "", "", fmt.Errorf("区间 [%s - %s] 分块没有向前推进", startDate, cursorEnd)
			}
			cursorEnd = plan.NextEnd
			continue
		default:
			return "", "", nil
		}
	}
}

func loadEmptySpans(ctx context.Context, stockCode, period string, adjusted bool) string {
	var log models.KLineSyncLog
	err := db.Dao.WithContext(ctx).
		Where("stock_code = ? AND period = ? AND adjusted = ?", stockCode, period, adjusted).
		First(&log).Error
	if err != nil {
		return ""
	}
	return log.EmptySpans
}

func saveEmptySpan(ctx context.Context, stockCode, period string, adjusted bool, start, end string) {
	merged := mergeEmptySpan(loadEmptySpans(ctx, stockCode, period, adjusted), start, end)
	if merged == "" {
		return
	}
	if err := db.Dao.WithContext(ctx).Model(&models.KLineSyncLog{}).
		Where("stock_code = ? AND period = ? AND adjusted = ?", stockCode, period, adjusted).
		Update("empty_spans", merged).Error; err != nil {
		logger.SugaredLogger.Warnf("save empty span %s [%s-%s]: %v", stockCode, start, end, err)
	}
}

// refreshSyncedCount 用库内实数覆盖 synced_count。重试同一段不能再累加，否则进度和 90% 修复都会失真。
func refreshSyncedCount(ctx context.Context, stockCode, period string, adjusted bool, startDate, endDate string) {
	if err := db.Dao.WithContext(ctx).Model(&models.KLineSyncLog{}).
		Where("stock_code = ? AND period = ? AND adjusted = ?", stockCode, period, adjusted).
		Updates(map[string]interface{}{
			"synced_count": countStoredBars(ctx, stockCode, period, adjusted, startDate, endDate),
			"updated_at":   time.Now(),
		}).Error; err != nil {
		logger.SugaredLogger.Errorf("Failed to update synced_count for %s: %v", stockCode, err)
	}
}

func countStoredBars(ctx context.Context, stockCode, period string, adjusted bool, startDate, endDate string) int {
	var n int64
	err := db.Dao.WithContext(ctx).Model(&models.KLineBar{}).
		Where("stock_code IN ? AND period IN ? AND adjusted = ? AND trade_date >= ? AND trade_date <= ?",
			stockcode.StockCodeCandidates(stockCode), datasource.KLinePeriodCandidates(period), adjusted, startDate, endDate).
		Count(&n).Error
	if err != nil {
		logger.SugaredLogger.Warnf("count kline bars %s: %v", stockCode, err)
		return 0
	}
	return int(n)
}

// openGaps 读出任务窗口里还没被空窗记录抵消的工作日缺口。
func openGaps(ctx context.Context, task *SyncTask) []datasource.DateRange {
	if task == nil {
		return nil
	}
	store := datasource.NewKLineStore()
	gaps, err := findMissingRangesLocal(ctx, store, task.StockCode, task.Period, task.StartDate, task.EndDate, task.Adjusted)
	if err != nil {
		logger.SugaredLogger.Warnf("checkpoint gaps %s: %v", task.StockCode, err)
		return []datasource.DateRange{{Start: task.StartDate, End: task.EndDate}}
	}
	return subtractEmptySpans(gaps, loadEmptySpans(ctx, task.StockCode, task.Period, task.Adjusted))
}

// alignDoneTask 成功收口：短历史（次新股、长停牌）把 expected 对齐到实数，
// 避免 90% 修复每次启动都把已核实的任务打回 pending。
func alignDoneTask(ctx context.Context, task *SyncTask) {
	if task == nil {
		return
	}
	n := countStoredBars(ctx, task.StockCode, task.Period, task.Adjusted, task.StartDate, task.EndDate)
	var log models.KLineSyncLog
	if err := db.Dao.WithContext(ctx).
		Where("stock_code = ? AND period = ? AND adjusted = ?", task.StockCode, task.Period, task.Adjusted).
		First(&log).Error; err != nil {
		return
	}
	expected := log.ExpectedCount
	if expected <= 0 || int64(n)*10 < int64(expected)*9 {
		expected = n
	}
	if err := db.Dao.WithContext(ctx).Model(&log).Updates(map[string]interface{}{
		"synced_count":   n,
		"expected_count": expected,
	}).Error; err != nil {
		logger.SugaredLogger.Warnf("align done task %s: %v", task.StockCode, err)
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
