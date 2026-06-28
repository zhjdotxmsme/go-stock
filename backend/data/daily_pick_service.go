package data

import (
	"context"
	"math"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// DailyPickService exposes Wails-bindable methods for the daily pick feature.
type DailyPickService struct {
	engine *DailyPickEngine
	review *DailyPickReview
}

// NewDailyPickService creates a new service instance.
func NewDailyPickService() *DailyPickService {
	return &DailyPickService{
		engine: NewDailyPickEngine(),
		review: NewDailyPickReview(),
	}
}

// RunDailyPick executes the daily pick flow and returns the top picks.
// Called from frontend or cron. tradeDate is "YYYY-MM-DD".
func (s *DailyPickService) RunDailyPick(tradeDate string, topN int) ([]models.DailyPick, error) {
	ctx := context.Background()
	picks, err := s.engine.RunDailyPick(ctx, tradeDate, topN)
	if err != nil {
		logger.SugaredLogger.Errorf("daily_pick: RunDailyPick failed: %v", err)
		return nil, err
	}
	return picks, nil
}

// RunDailyPickAsync kicks off the pick in a goroutine and returns immediately.
// The picks are persisted to the database as they complete.
func (s *DailyPickService) RunDailyPickAsync(tradeDate string, topN int) {
	go func() {
		ctx := context.Background()
		_, err := s.engine.RunDailyPick(ctx, tradeDate, topN)
		if err != nil {
			logger.SugaredLogger.Errorf("daily_pick: async pick failed: %v", err)
		}
	}()
}

// RunDailyReview performs next-day review for picks from the previous trading day.
// reviewDate is the date with closing data (e.g. T+1). If pickDate is empty,
// it reviews the most recent unreviewed date. Returns count of reviewed picks.
func (s *DailyPickService) RunDailyReview(reviewDate string, pickDate string) int {
	ctx := context.Background()
	return s.review.RunDailyReview(ctx, reviewDate, pickDate)
}

// ReviewAllUnreviewed reviews all unreviewed picks up to today.
func (s *DailyPickService) ReviewAllUnreviewed() int {
	ctx := context.Background()
	return s.review.ReviewAllUnreviewed(ctx)
}

// GetReviewSummary returns a summary of review results for a given trade date.
func (s *DailyPickService) GetReviewSummary(tradeDate string) map[string]interface{} {
	ctx := context.Background()
	return s.review.GetReviewSummary(ctx, tradeDate)
}

// GetDailyPicks returns a paginated list of daily picks.
func (s *DailyPickService) GetDailyPicks(query models.DailyPickQuery) models.DailyPickPageData {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 || query.PageSize > 100 {
		query.PageSize = 20
	}

	var result models.DailyPickPageData
	var picks []models.DailyPick

	tx := db.Dao.Model(&models.DailyPick{})

	// Apply filters
	if query.TradeDate != "" {
		tx = tx.Where("trade_date = ?", query.TradeDate)
	}
	if query.StartDate != "" {
		tx = tx.Where("trade_date >= ?", query.StartDate)
	}
	if query.EndDate != "" {
		tx = tx.Where("trade_date <= ?", query.EndDate)
	}
	if query.Reviewed != nil {
		tx = tx.Where("reviewed = ?", *query.Reviewed)
	}

	// Count total
	if err := tx.Count(&result.Total).Error; err != nil {
		logger.SugaredLogger.Errorf("daily_pick: count error: %v", err)
		return result
	}

	// Paginate
	offset := (query.Page - 1) * query.PageSize
	if err := tx.Order("trade_date DESC, score DESC").
		Offset(offset).
		Limit(query.PageSize).
		Find(&picks).Error; err != nil {
		logger.SugaredLogger.Errorf("daily_pick: query error: %v", err)
		return result
	}

	result.List = picks
	result.Page = query.Page
	result.PageSize = query.PageSize
	if result.PageSize > 0 {
		result.TotalPages = int(math.Ceil(float64(result.Total) / float64(result.PageSize)))
	}
	return result
}

// GetLatestPicks returns today's or the most recent picks (up to topN).
func (s *DailyPickService) GetLatestPicks(topN int) []models.DailyPick {
	if topN <= 0 {
		topN = 5
	}

	today := time.Now().Format("2006-01-02")
	var picks []models.DailyPick

	// Try today first
	if err := db.Dao.Where("trade_date = ?", today).
		Order("score DESC").
		Limit(topN).
		Find(&picks).Error; err != nil || len(picks) == 0 {

		// Fallback to most recent date
		var latest models.DailyPick
		if err := db.Dao.Order("trade_date DESC").First(&latest).Error; err != nil {
			return nil
		}
		db.Dao.Where("trade_date = ?", latest.TradeDate).
			Order("score DESC").
			Limit(topN).
			Find(&picks)
	}
	return picks
}

// DeleteDailyPick deletes a single pick record.
func (s *DailyPickService) DeleteDailyPick(id uint) error {
	return db.Dao.Delete(&models.DailyPick{}, id).Error
}

// UpdateDailyPickRemarks updates the remarks field of a pick.
func (s *DailyPickService) UpdateDailyPickRemarks(id uint, remarks string) error {
	return db.Dao.Model(&models.DailyPick{}).Where("id = ?", id).Update("remarks", remarks).Error
}

// GetDailyPickStats returns aggregate statistics for all picks.
func (s *DailyPickService) GetDailyPickStats() models.DailyPickStats {
	var stats models.DailyPickStats
	var picks []models.DailyPick

	var totalPicks64, reviewedPicks64 int64
	db.Dao.Model(&models.DailyPick{}).Count(&totalPicks64)
	db.Dao.Model(&models.DailyPick{}).Where("reviewed = ?", true).Count(&reviewedPicks64)
	stats.TotalPicks = int(totalPicks64)
	stats.ReviewedPicks = int(reviewedPicks64)

	if stats.ReviewedPicks == 0 {
		return stats
	}

	db.Dao.Where("reviewed = ?", true).Find(&picks)

	var winCount, lossCount int
	var totalReturn, maxReturn, maxDrawdown float64
	var avgMaxReturnTotal, avgMaxDrawdownTotal float64

	for _, p := range picks {
		if p.NextReturn > 0 {
			winCount++
		} else {
			lossCount++
		}
		totalReturn += p.NextReturn
		if p.NextReturn > maxReturn {
			maxReturn = p.NextReturn
		}
		if p.NextMaxDrawdown < maxDrawdown {
			maxDrawdown = p.NextMaxDrawdown
		}
		avgMaxReturnTotal += p.NextMaxReturn
		avgMaxDrawdownTotal += p.NextMaxDrawdown
	}

	n := float64(len(picks))
	stats.WinCount = winCount
	stats.LossCount = lossCount
	stats.WinRate = math.Round(float64(winCount)/n*10000) / 100
	stats.AvgReturn = math.Round(totalReturn/n*100) / 100
	stats.TotalReturn = math.Round(totalReturn*100) / 100
	stats.MaxReturn = math.Round(maxReturn*100) / 100
	stats.MaxDrawdown = math.Round(maxDrawdown*100) / 100
	stats.AvgMaxReturn = math.Round(avgMaxReturnTotal/n*100) / 100
	stats.AvgMaxDrawdown = math.Round(avgMaxDrawdownTotal/n*100) / 100

	return stats
}

// GetLatestUnreviewedPicks returns picks from the most recent date that haven't been reviewed.
func (s *DailyPickService) GetLatestUnreviewedPicks() []models.DailyPick {
	var latest models.DailyPick
	if err := db.Dao.Where("reviewed = ?", false).
		Order("trade_date DESC").
		First(&latest).Error; err != nil {
		return nil
	}

	var picks []models.DailyPick
	db.Dao.Where("trade_date = ? AND reviewed = ?", latest.TradeDate, false).
		Order("score DESC").
		Find(&picks)
	return picks
}

// GetDateRange returns the earliest and latest trade dates with picks.
func (s *DailyPickService) GetDateRange() (string, string) {
	var first, last models.DailyPick
	start, end := "", ""
	if err := db.Dao.Order("trade_date ASC").First(&first).Error; err == nil {
		start = first.TradeDate
	}
	if err := db.Dao.Order("trade_date DESC").First(&last).Error; err == nil {
		end = last.TradeDate
	}
	return start, end
}

// ---- Cursor-based listing for charts ----

// GetReviewTrend returns daily win-rate trend data.
func (s *DailyPickService) GetReviewTrend(limit int) []map[string]interface{} {
	if limit <= 0 {
		limit = 30
	}

	type dateAgg struct {
		TradeDate string
		WinCount  int
		Total     int
	}

	var agg []dateAgg
	db.Dao.Model(&models.DailyPick{}).
		Select("trade_date, sum(case when next_return > 0 then 1 else 0 end) as win_count, count(*) as total").
		Where("reviewed = ?", true).
		Group("trade_date").
		Order("trade_date DESC").
		Limit(limit).
		Scan(&agg)

	var result []map[string]interface{}
	for _, a := range agg {
		winRate := 0.0
		if a.Total > 0 {
			winRate = math.Round(float64(a.WinCount)/float64(a.Total)*10000) / 100
		}
		result = append(result, map[string]interface{}{
			"date":     a.TradeDate,
			"winRate":  winRate,
			"winCount": a.WinCount,
			"total":    a.Total,
		})
	}
	return result
}

// ensureAutoMigrate registers the DailyPick model for auto-migration.
// Called during app initialization.
func ensureDailyPickAutoMigrate() {
	if db.Dao == nil {
		return
	}
	if err := db.Dao.AutoMigrate(&models.DailyPick{}); err != nil {
		logger.SugaredLogger.Errorf("daily_pick: auto-migrate failed: %v", err)
	} else {
		logger.SugaredLogger.Info("daily_pick: auto-migrate ok")
	}
}

// InitDailyPickService initializes the service, runs auto-migration.
func InitDailyPickService() *DailyPickService {
	ensureDailyPickAutoMigrate()
	return NewDailyPickService()
}
