package backtest

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go-stock/backend/data/history"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// seedImportOutput stores the last seed import run output (non-persistent).
var seedImportOutput string

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

// GetSeedImportStatus returns information about seed data availability
// and environment readiness for running the baostock seed script.
func (s *Service) GetSeedImportStatus(ctx context.Context) (map[string]any, error) {
	result := map[string]any{
		"seedBars":       int64(0),
		"seedStocks":     int64(0),
		"dbPath":         "",
		"scriptPath":     "",
		"pythonFound":    false,
		"pythonPath":     "",
		"hasSeedData":    false,
	}

	// Count existing seed records
	var seedBars int64
	var seedStocks int64
	db.Dao.WithContext(ctx).Model(&models.KLineBar{}).
		Where("source = ?", "seed").Select("COUNT(*)").Scan(&seedBars)
	db.Dao.WithContext(ctx).Model(&models.KLineBar{}).
		Where("source = ?", "seed").Select("COUNT(DISTINCT stock_code)").Scan(&seedStocks)
	result["seedBars"] = seedBars
	result["seedStocks"] = seedStocks
	result["hasSeedData"] = seedBars > 0

	// Resolve DB path from the active connection
	var dbPath string
	row := db.Dao.WithContext(ctx).Raw("SELECT file FROM pragma_database_list WHERE name='main'").Row()
	if row != nil {
		row.Scan(&dbPath)
	}
	if dbPath == "" {
		dbPath = "data/stock.db"
	}
	result["dbPath"] = dbPath

	// Find seed script relative to executable
	exe, err := os.Executable()
	if err == nil {
		candidates := []string{
			filepath.Join(filepath.Dir(exe), "scripts", "history_seed", "baostock_seed.py"),
			filepath.Join(filepath.Dir(exe), "..", "scripts", "history_seed", "baostock_seed.py"),
			"scripts/history_seed/baostock_seed.py",
			"../scripts/history_seed/baostock_seed.py",
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				result["scriptPath"] = c
				break
			}
		}
	}

	// Try to find Python
	for _, name := range []string{"python3", "python"} {
		if path, err := exec.LookPath(name); err == nil {
			result["pythonFound"] = true
			result["pythonPath"] = path
			break
		}
	}

	return result, nil
}

// RunSeedImport executes the baostock seed Python script as a subprocess.
// pythonPath: optional path to Python interpreter (empty = auto-detect)
// startDate: optional start date YYYYMMDD (empty = default 20100101)
// endDate: optional end date YYYYMMDD (empty = default yesterday)
// limit: optional max stocks to process (0 = all)
// Returns the combined stdout+stderr output of the script.
func (s *Service) RunSeedImport(ctx context.Context, pythonPath, startDate, endDate string, limit int) (string, error) {
	status, err := s.GetSeedImportStatus(ctx)
	if err != nil {
		return "", fmt.Errorf("检查环境失败: %w", err)
	}

	scriptPath, _ := status["scriptPath"].(string)
	if scriptPath == "" {
		return "", fmt.Errorf("未找到种子脚本 baostock_seed.py，请确认 scripts/history_seed/ 目录存在")
	}

	python, _ := status["pythonPath"].(string)
	if pythonPath != "" {
		python = pythonPath
	}
	if python == "" {
		return "", fmt.Errorf("未找到 Python 解释器，请安装 Python 3 并确保在 PATH 中")
	}

	dbPath, _ := status["dbPath"].(string)

	args := []string{scriptPath, "--db-path", dbPath}
	if startDate != "" {
		args = append(args, "--start-date", startDate)
	}
	if endDate != "" {
		args = append(args, "--end-date", endDate)
	}
	if limit > 0 {
		args = append(args, "--limit", fmt.Sprintf("%d", limit))
	}

	logger.SugaredLogger.Infof("running seed import: %s %s", python, strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, python, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		logger.SugaredLogger.Errorf("seed import failed: %v\n%s", err, errMsg)
		seedImportOutput = errMsg
		return errMsg, fmt.Errorf("种子导入失败: %w\n%s", err, errMsg)
	}

	output := stdout.String()
	seedImportOutput = output
	logger.SugaredLogger.Info("seed import completed successfully")
	return output, nil
}

// GetLastSeedImportOutput returns the output from the most recent seed import run.
func (s *Service) GetLastSeedImportOutput(ctx context.Context) (string, error) {
	return seedImportOutput, nil
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
