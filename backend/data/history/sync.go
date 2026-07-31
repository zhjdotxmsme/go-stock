package history

import (
	"context"
	"fmt"
	"time"

	"go-stock/backend/data/datasource"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
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

	startT, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, fmt.Errorf("parse start date: %w", err)
	}
	endT, err := time.Parse("2006-01-02", endDate)
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

	var existing models.KLineSyncLog
	err := db.Dao.WithContext(ctx).
		Where("stock_code = ? AND period = ? AND adjusted = ?", stockCode, period, adjusted).
		First(&existing).Error

	if err == nil {
		existing.StartDate = startDate
		existing.EndDate = endDate
		existing.ExpectedCount = estimateExpectedCount(period, startDate, endDate)
		existing.Status = SyncStatusPending
		existing.ErrorMsg = ""
		existing.UpdatedAt = time.Now()
		
		if err := db.Dao.WithContext(ctx).Save(&existing).Error; err != nil {
			return fmt.Errorf("failed to update existing sync task: %w", err)
		}
		
		logger.SugaredLogger.Infof("Updated existing sync task for %s %s", stockCode, period)
		return nil
	}

	if err.Error() != "record not found" {
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
	count := 2000

	data, err := router.GetKLine(ctx, stockCode, period, count)
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
		logger.SugaredLogger.Warnf("No bars in date range [%s - %s] for %s", startDate, endDate, stockCode)
		return nil
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
				UpdatedAt:     time.Now(),
			}
			return db.Dao.Create(&log).Error
		}
		return fmt.Errorf("failed to find sync task: %w", err)
	}

	return db.Dao.Model(&log).Updates(map[string]interface{}{
		"status":     task.Status,
		"error_msg":  task.ErrorMsg,
		"updated_at": time.Now(),
	}).Error
}

// syncLogToTask converts a KLineSyncLog to a SyncTask.
// LastSyncDate is left empty: the log has no dedicated checkpoint column, and
// resume is handled by gap detection inside SyncKLineForStock.
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
		LastSyncDate: "",
		CreatedAt:    log.UpdatedAt,
		UpdatedAt:    log.UpdatedAt,
	}
}

// estimateExpectedCount estimates the number of K-line bars in [startDate, endDate].
// day: weekdays (~5/7 of days); week: weekly bars; month: monthly bars.
func estimateExpectedCount(period, startDate, endDate string) int {
	start, err1 := time.Parse("2006-01-02", startDate)
	end, err2 := time.Parse("2006-01-02", endDate)
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
	
	if _, err := time.Parse("2006-01-02", startDate); err != nil {
		return fmt.Errorf("invalid start date format: %s (expected YYYY-MM-DD)", startDate)
	}
	if _, err := time.Parse("2006-01-02", endDate); err != nil {
		return fmt.Errorf("invalid end date format: %s (expected YYYY-MM-DD)", endDate)
	}
	
	start, _ := time.Parse("2006-01-02", startDate)
	end, _ := time.Parse("2006-01-02", endDate)
	if start.After(end) {
		return fmt.Errorf("start date cannot be after end date")
	}
	
	return nil
}