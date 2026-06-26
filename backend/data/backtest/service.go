package backtest

import (
	"context"
	"time"

	"go-stock/backend/data/history"
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

type syncTaskItem struct {
	StockCode string `json:"stockCode"`
	Period    string `json:"period"`
	Status    string `json:"status"`
	Progress  int    `json:"progress"`
	ErrorMsg  string `json:"errorMsg"`
}

func (s *Service) StartHistoricalSync(ctx context.Context, years int) error {
	now := time.Now()
	startDate := now.AddDate(-years, 0, 0).Format("2006-01-02")
	endDate := now.Format("2006-01-02")

	var infos []models.AllStockInfo
	if err := db.Dao.WithContext(ctx).
		Where("secucode LIKE ? OR secucode LIKE ?", "sh%", "sz%").
		Find(&infos).Error; err != nil {
		return err
	}

	for _, info := range infos {
		if err := history.CreateSyncTask(ctx, info.SECUCODE, "day", startDate, endDate, true); err != nil {
			logger.SugaredLogger.Errorf("create sync task failed for %s: %v", info.SECUCODE, err)
			continue
		}
	}

	logger.SugaredLogger.Infof("created %d sync tasks for historical data sync (%d years)", len(infos), years)
	return nil
}

func (s *Service) GetSyncProgress(ctx context.Context) ([]syncTaskItem, error) {
	var logs []models.KLineSyncLog
	if err := db.Dao.WithContext(ctx).Order("updated_at DESC").Find(&logs).Error; err != nil {
		return nil, err
	}

	items := make([]syncTaskItem, 0, len(logs))
	for _, l := range logs {
		progress := 0
		if l.ExpectedCount > 0 {
			progress = l.SyncedCount * 100 / l.ExpectedCount
		} else if l.Status == history.SyncStatusDone {
			progress = 100
		}

		items = append(items, syncTaskItem{
			StockCode: l.StockCode,
			Period:    l.Period,
			Status:    l.Status,
			Progress:  progress,
			ErrorMsg:  l.ErrorMsg,
		})
	}

	return items, nil
}

func (s *Service) GetKLineCacheStats(ctx context.Context) (map[string]any, error) {
	var totalBars int64
	var uniqueStocks int64
	var minDate, maxDate string
	var lastSyncTime string

	db.Dao.WithContext(ctx).Model(&models.KLineBar{}).Select("COUNT(*)").Scan(&totalBars)
	db.Dao.WithContext(ctx).Model(&models.KLineBar{}).Select("COUNT(DISTINCT stock_code)").Scan(&uniqueStocks)
	db.Dao.WithContext(ctx).Model(&models.KLineBar{}).Select("MIN(trade_date), MAX(trade_date)").Row().Scan(&minDate, &maxDate)

	var latest models.KLineSyncLog
	if err := db.Dao.WithContext(ctx).Order("updated_at DESC").First(&latest).Error; err == nil {
		lastSyncTime = latest.UpdatedAt.Format("2006-01-02 15:04:05")
	}

	return map[string]any{
		"totalBars":    totalBars,
		"uniqueStocks": uniqueStocks,
		"dateRange": map[string]string{
			"min": minDate,
			"max": maxDate,
		},
		"lastSyncTime": lastSyncTime,
	}, nil
}
