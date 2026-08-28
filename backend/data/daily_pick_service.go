package data

import (
	"context"
	"math"
	"sync/atomic"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"

)

// DailyPickProgressEvent is the Wails event name used to report async
// daily-pick progress to the frontend.
const DailyPickProgressEvent = "dailyPickProgress"

// DailyPickService exposes Wails-bindable methods for the daily pick feature.
type DailyPickService struct {
	engine *DailyPickEngine
	review *DailyPickReview
	repo   *DailyPickRepository

	asyncRunning atomic.Bool // guards against concurrent async runs

	// emit 是可选的进度事件发射器（S4 解耦）：由装配方注入（如 handler 用
	// Wails runtime 转发到前端）；nil 时进度事件被丢弃，服务自身不依赖
	// 任何 GUI runtime。
	emit ProgressEmitter
}

// ProgressEmitter 接收进度事件（事件名 + 负载）。
type ProgressEmitter func(event string, payload map[string]any)

// NewDailyPickService creates a new service instance.
func NewDailyPickService() *DailyPickService {
	return &DailyPickService{
		engine: NewDailyPickEngine(),
		review: NewDailyPickReview(),
		repo:   NewDailyPickRepository(),
	}
}

// WithEmitter 注入进度事件发射器，返回自身以便链式装配。
func (s *DailyPickService) WithEmitter(emit ProgressEmitter) *DailyPickService {
	s.emit = emit
	return s
}

// emitProgress 发送一条进度事件；未注入发射器时静默丢弃。
func (s *DailyPickService) emitProgress(payload map[string]any) {
	if s.emit != nil {
		s.emit(DailyPickProgressEvent, payload)
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
// Wails injects the application context as the first argument. Progress and the
// final result are pushed to the frontend via "dailyPickProgress" events:
//
//	{stage:"start",    total:int}
//	{stage:"baseline", done:int, total:int}  // stage-1 K-line scoring
//	{stage:"research", done:int, total:int}  // stage-2 research report pre-fetch
//	{stage:"final",    done:int, total:int}  // stage-2 full scoring
//	{stage:"done",     count:int}            // success, count = persisted picks
//	{stage:"error",    message:string}       // failure
//	{stage:"busy",     message:string}       // another run is in progress
//
// The picks are persisted to the database as they complete.
func (s *DailyPickService) RunDailyPickAsync(tradeDate string, topN int) {
	ctx := context.Background()
	if !s.asyncRunning.CompareAndSwap(false, true) {
		s.emitProgress(map[string]any{
			"stage":   "busy",
			"message": "选股任务正在运行中，请稍候",
		})
		return
	}
	go func() {
		defer s.asyncRunning.Store(false)
		defer func() {
			if err := recover(); err != nil {
				logger.SugaredLogger.Errorf("daily_pick: async pick panic: %v", err)
				s.emitProgress(map[string]any{
					"stage":   "error",
					"message": "选股任务异常",
				})
			}
		}()

		s.engine.WithProgressHook(func(stage string, done, total int) {
			s.emitProgress(map[string]any{
				"stage": stage,
				"done":  done,
				"total": total,
			})
		})
		defer s.engine.WithProgressHook(nil)

		picks, err := s.engine.RunDailyPick(ctx, tradeDate, topN)
		if err != nil {
			logger.SugaredLogger.Errorf("daily_pick: async pick failed: %v", err)
			s.emitProgress(map[string]any{
				"stage":   "error",
				"message": err.Error(),
			})
			return
		}
		s.emitProgress(map[string]any{
			"stage": "done",
			"count": len(picks),
		})
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

	total, picks, err := s.repo.QueryDailyPicks(context.Background(), query)
	if err != nil {
		logger.SugaredLogger.Errorf("daily_pick: query error: %v", err)
		return result
	}
	result.Total = total

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

	// Try today first
	picks, err := s.repo.TodayTopPicks(context.Background(), today, topN)
	if err != nil || len(picks) == 0 {
		// Fallback to most recent date
		latest, err := s.repo.LatestPick(context.Background())
		if err != nil {
			return nil
		}
		picks = s.repo.PicksByDateTop(context.Background(), latest.TradeDate, topN)
	}
	return picks
}

// DeleteDailyPick deletes a single pick record.
func (s *DailyPickService) DeleteDailyPick(id uint) error {
	return s.repo.DeletePick(context.Background(), id)
}

// UpdateDailyPickRemarks updates the remarks field of a pick.
func (s *DailyPickService) UpdateDailyPickRemarks(id uint, remarks string) error {
	return s.repo.UpdateRemarks(context.Background(), id, remarks)
}

// GetDailyPickStats returns aggregate statistics for all picks.
func (s *DailyPickService) GetDailyPickStats() models.DailyPickStats {
	var stats models.DailyPickStats

	totalPicks64, reviewedPicks64 := s.repo.CountPicks(context.Background())
	stats.TotalPicks = int(totalPicks64)
	stats.ReviewedPicks = int(reviewedPicks64)

	if stats.ReviewedPicks == 0 {
		return stats
	}

	picks := s.repo.FindReviewed(context.Background())

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
	latest, err := s.repo.LatestUnreviewed(context.Background())
	if err != nil {
		return nil
	}
	return s.repo.UnreviewedByDate(context.Background(), latest.TradeDate)
}

// GetDateRange returns the earliest and latest trade dates with picks.
func (s *DailyPickService) GetDateRange() (string, string) {
	return s.repo.DateRange(context.Background())
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
