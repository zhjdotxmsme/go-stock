package backtest

import (
	"context"

	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// Service is a Wails-bindable wrapper around backtest operations.
type Service struct{}

// NewService creates a new Service.
func NewService() *Service {
	return &Service{}
}

// RunSingleBacktest runs a single backtest for a stock on a given signal date.
func (s *Service) RunSingleBacktest(ctx context.Context, stockCode, signalDate string, entryPrice float64, holdingDays int, stopLoss, stopProfit float64, adjusted bool) (*Result, error) {
	engine := NewEngine()
	result, err := engine.Run(ctx, Input{
		StockCode:   stockCode,
		SignalDate:  signalDate,
		EntryPrice:  entryPrice,
		HoldingDays: holdingDays,
		StopLoss:    stopLoss,
		StopProfit:  stopProfit,
		Adjusted:    adjusted,
	})
	if err != nil {
		logger.SugaredLogger.Errorf("single backtest failed for %s on %s: %v", stockCode, signalDate, err)
		return nil, err
	}
	logger.SugaredLogger.Infof("single backtest for %s on %s: return=%.4f, win=%v", stockCode, signalDate, result.TotalReturn, result.Win)
	return result, nil
}

// RunBatchBacktest runs batch backtests for all trading days in a date range.
func (s *Service) RunBatchBacktest(ctx context.Context, stockCode, startDate, endDate, period string, adjusted bool, entryPrice float64, holdingDays int, stopLoss, stopProfit float64) (*BatchResult, error) {
	if period == "" {
		period = "day"
	}
	result, err := RunBatchBacktest(ctx, stockCode, startDate, endDate, period, adjusted, entryPrice, holdingDays, stopLoss, stopProfit)
	if err != nil {
		logger.SugaredLogger.Errorf("batch backtest failed for %s [%s - %s]: %v", stockCode, startDate, endDate, err)
		return nil, err
	}
	logger.SugaredLogger.Infof("batch backtest for %s [%s - %s]: trades=%d winRate=%.4f", stockCode, startDate, endDate, result.TotalTrades, result.WinRate)
	return result, nil
}

// GetBacktestResults queries persisted backtest results with pagination.
func (s *Service) GetBacktestResults(ctx context.Context, stockCode string, page, pageSize int) (*models.AiRecommendBacktestPageData, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	var list []models.AiRecommendBacktest
	var total int64

	q := db.Dao.WithContext(ctx).Model(&models.AiRecommendBacktest{})
	if stockCode != "" {
		q = q.Where("stock_code = ?", stockCode)
	}
	q.Count(&total)
	q.Offset((page - 1) * pageSize).Limit(pageSize).Order("created_at DESC").Find(&list)

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	return &models.AiRecommendBacktestPageData{
		List:       list,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}
