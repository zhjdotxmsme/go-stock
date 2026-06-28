package data

import (
	"context"
	"math"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// DailyPickReview handles the next-day review of previous picks.
type DailyPickReview struct {
	engine *DailyPickEngine
}

// NewDailyPickReview creates a new review instance.
func NewDailyPickReview() *DailyPickReview {
	return &DailyPickReview{
		engine: NewDailyPickEngine(),
	}
}

// RunDailyReview performs review for picks from the previous trading day.
// reviewDate is the date whose closing data to use for review (e.g. T+1).
// If pickDate is empty, it reviews the most recent unreviewed date.
func (r *DailyPickReview) RunDailyReview(ctx context.Context, reviewDate string, pickDate string) int {
	var picks []models.DailyPick

	if pickDate != "" {
		db.Dao.WithContext(ctx).
			Where("trade_date = ? AND reviewed = ?", pickDate, false).
			Find(&picks)
	} else {
		// Find the most recent unreviewed date
		var latest models.DailyPick
		if err := db.Dao.WithContext(ctx).
			Where("reviewed = ?", false).
			Order("trade_date ASC").
			First(&latest).Error; err != nil {
			logger.SugaredLogger.Info("daily_review: no unreviewed picks found")
			return 0
		}
		db.Dao.WithContext(ctx).
			Where("trade_date = ? AND reviewed = ?", latest.TradeDate, false).
			Find(&picks)
	}

	if len(picks) == 0 {
		logger.SugaredLogger.Info("daily_review: no picks to review")
		return 0
	}

	logger.SugaredLogger.Infof("daily_review: reviewing %d picks with %s data", len(picks), reviewDate)

	reviewed := 0
	for i := range picks {
		ok := r.reviewOne(ctx, &picks[i], reviewDate)
		if ok {
			reviewed++
		}
	}
	return reviewed
}

// reviewOne reviews a single pick against reviewDate's market data.
func (r *DailyPickReview) reviewOne(ctx context.Context, pick *models.DailyPick, reviewDate string) bool {
	apiCode := normalizeCode(pick.StockCode)

	// Fetch K-line data for the review date - need at least the most recent daily bar
	klineData := NewStockDataApi().GetKLineData(apiCode, "101", 5)
	if klineData == nil || len(*klineData) == 0 {
		logger.SugaredLogger.Warnf("daily_review: no kline data for %s", pick.StockCode)
		return false
	}

	klines := *klineData
	latest := klines[len(klines)-1]

	pick.NextOpen = parseFloat64(latest.Open)
	pick.NextHigh = parseFloat64(latest.High)
	pick.NextLow = parseFloat64(latest.Low)
	pick.NextClose = parseFloat64(latest.Close)

	// Compute returns
	if pick.NextOpen > 0 {
		// Simple return: close / open - 1
		pick.NextReturn = math.Round((pick.NextClose/pick.NextOpen-1)*10000) / 100

		// Max return: high / open - 1
		pick.NextMaxReturn = math.Round((pick.NextHigh/pick.NextOpen-1)*10000) / 100

		// Max drawdown: low / open - 1
		drawdown := (pick.NextLow/pick.NextOpen - 1) * 100
		drawdown = math.Round(drawdown*100) / 100
		pick.NextMaxDrawdown = drawdown
	}

	pick.Reviewed = true

	if err := db.Dao.WithContext(ctx).Save(pick).Error; err != nil {
		logger.SugaredLogger.Errorf("daily_review: save review failed for %s: %v", pick.StockCode, err)
		return false
	}
	return true
}

// ReviewAllUnreviewed reviews all unreviewed picks up to today.
func (r *DailyPickReview) ReviewAllUnreviewed(ctx context.Context) int {
	today := time.Now().Format("2006-01-02")
	return r.RunDailyReview(ctx, today, "")
}

// GetUnreviewedDate returns the earliest date with unreviewed picks, or empty string.
func (r *DailyPickReview) GetUnreviewedDate(ctx context.Context) string {
	var pick models.DailyPick
	if err := db.Dao.WithContext(ctx).
		Where("reviewed = ?", false).
		Order("trade_date ASC").
		First(&pick).Error; err != nil {
		return ""
	}
	return pick.TradeDate
}

// GetReviewSummary returns a summary of review results for a given date.
func (r *DailyPickReview) GetReviewSummary(ctx context.Context, tradeDate string) map[string]interface{} {
	var picks []models.DailyPick
	tx := db.Dao.WithContext(ctx).Where("reviewed = ?", true)
	if tradeDate != "" {
		tx = tx.Where("trade_date = ?", tradeDate)
	}
	tx.Find(&picks)

	n := len(picks)
	if n == 0 {
		return map[string]interface{}{
			"count":        0,
			"avgReturn":    0.0,
			"winRate":      0.0,
			"totalWin":     0,
			"totalLoss":    0,
			"maxReturn":    0.0,
			"maxDrawdown":  0.0,
		}
	}

	var win, loss int
	var totalReturn, maxReturn, maxDrawdown float64
	for _, p := range picks {
		if p.NextReturn > 0 {
			win++
		} else {
			loss++
		}
		totalReturn += p.NextReturn
		if p.NextReturn > maxReturn {
			maxReturn = p.NextReturn
		}
		if p.NextMaxDrawdown < maxDrawdown {
			maxDrawdown = p.NextMaxDrawdown
		}
	}

	return map[string]interface{}{
		"count":       n,
		"avgReturn":   math.Round(totalReturn/float64(n)*100) / 100,
		"winRate":     math.Round(float64(win)/float64(n)*10000) / 100,
		"totalWin":    win,
		"totalLoss":   loss,
		"maxReturn":   math.Round(maxReturn*100) / 100,
		"maxDrawdown": math.Round(maxDrawdown*100) / 100,
	}
}
