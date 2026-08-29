package backtest

import (
	"bytes"
	"regexp"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go-stock/backend/data/history"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// seed log output directory
var seedLogDir = filepath.Join("data", "logs")

// seedImportOutput stores the last seed import run output (non-persistent).
var seedImportOutput string

// Service is a Wails-bindable wrapper around backtest operations.
type Service struct{}

// NewService creates a new Service.
func NewService() *Service {
	return &Service{}
}

// SanitizeStockCodeInput 从展示串中恢复标准代码。
// 前端可能传入 "兴业银锡 - 000426.SZ" 这类 名字+代码 的展示串，
// 先尝试标准解析，失败则提取串中最后一组 6 位数字并推断交易所。
func SanitizeStockCodeInput(input string) string {
	// 注意：NormalizeStockCode 对无法识别的输入会“原样返回”（非空），
	// 不能以非空判断成功——必须先显式 ParseStockCode。
	if digits, exchange, ok := ParseStockCode(input); ok && exchange != "" {
		return exchange + digits
	}
	runs := sixDigitRuns.FindAllString(input, -1)
	if len(runs) == 0 {
		return input
	}
	digits := runs[len(runs)-1]
	if ex := inferExchange(digits); ex != "" {
		return ex + digits
	}
	return input
}

var sixDigitRuns = regexp.MustCompile(`\d{6}`)

// RunSingleBacktest runs a single backtest for a stock on a given signal date.
func (s *Service) RunSingleBacktest(stockCode, signalDate string, entryPrice float64, holdingDays int, stopLoss, stopProfit float64, adjusted bool) (*Result, error) {
	stockCode = SanitizeStockCodeInput(stockCode)
	ctx := context.Background()
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
func (s *Service) RunBatchBacktest(stockCode, startDate, endDate, period string, adjusted bool, entryPrice float64, holdingDays int, stopLoss, stopProfit float64) (*BatchResult, error) {
	stockCode = SanitizeStockCodeInput(stockCode)
	ctx := context.Background()
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
func (s *Service) GetBacktestResults(stockCode string, page, pageSize int) (*models.AiRecommendBacktestPageData, error) {
	ctx := context.Background()
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

func (s *Service) StartHistoricalSync(years int) error {
	ctx := context.Background()
	now := time.Now()
	startDate := now.AddDate(-years, 0, 0).Format("2006-01-02")
	endDate := now.Format("2006-01-02")

	var infos []models.AllStockInfo
	if err := db.Dao.WithContext(ctx).
		Where("secucode LIKE ? OR secucode LIKE ?", "%.SH", "%.SZ").
		Find(&infos).Error; err != nil {
		return err
	}

	// Stocks whose sync is already in flight are left untouched, so re-clicking
	// the button while workers are running does not reset running tasks.
	running := make(map[string]struct{})
	var runningLogs []models.KLineSyncLog
	if err := db.Dao.WithContext(ctx).
		Where("status = ?", history.SyncStatusRunning).
		Find(&runningLogs).Error; err == nil {
		for _, l := range runningLogs {
			running[l.StockCode] = struct{}{}
		}
	}

	created := 0
	for _, info := range infos {
		if _, ok := running[info.SECUCODE]; ok {
			continue
		}
		if err := history.CreateSyncTask(ctx, info.SECUCODE, "day", startDate, endDate, true); err != nil {
			logger.SugaredLogger.Errorf("create sync task failed for %s: %v", info.SECUCODE, err)
			continue
		}
		created++
	}

	logger.SugaredLogger.Infof("created %d sync tasks for historical data sync (%d years)", created, years)

	startSyncWorkers()
	return nil
}

// syncWorkerCount is the number of goroutines consuming pending sync tasks.
const syncWorkerCount = 3

var (
	syncWorkersMu      sync.Mutex
	syncWorkersRunning bool
)

// startSyncWorkers launches background workers to consume pending sync tasks.
// It is a no-op if a batch of workers is already running.
func startSyncWorkers() {
	syncWorkersMu.Lock()
	if syncWorkersRunning {
		syncWorkersMu.Unlock()
		logger.SugaredLogger.Infof("sync workers already running, skip starting a new batch")
		return
	}
	syncWorkersRunning = true
	syncWorkersMu.Unlock()

	go func() {
		defer func() {
			syncWorkersMu.Lock()
			syncWorkersRunning = false
			syncWorkersMu.Unlock()
		}()

		var wg sync.WaitGroup
		for i := 0; i < syncWorkerCount; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				syncWorkerLoop()
			}()
		}
		wg.Wait()
		logger.SugaredLogger.Infof("all sync workers finished, no pending tasks left")
	}()
}

// syncWorkerLoop claims and executes pending sync tasks until none remain.
func syncWorkerLoop() {
	ctx := context.Background()
	for {
		task := claimNextPendingSyncTask(ctx)
		if task == nil {
			return
		}
		if err := history.RunSyncTask(ctx, task); err != nil {
			logger.SugaredLogger.Errorf("sync task failed for %s: %v", task.StockCode, err)
		}
	}
}

// claimNextPendingSyncTask atomically marks the oldest pending task as running
// and returns it. Returns nil when no pending task is left.
func claimNextPendingSyncTask(ctx context.Context) *history.SyncTask {
	// Retry a few times in case several workers race for the same task.
	for i := 0; i < 10; i++ {
		var log models.KLineSyncLog
		err := db.Dao.WithContext(ctx).
			Where("status = ?", history.SyncStatusPending).
			Order("id ASC").
			First(&log).Error
		if err != nil {
			return nil
		}

		res := db.Dao.WithContext(ctx).Model(&models.KLineSyncLog{}).
			Where("id = ? AND status = ?", log.ID, history.SyncStatusPending).
			Updates(map[string]interface{}{
				"status":     history.SyncStatusRunning,
				"updated_at": time.Now(),
			})
		if res.Error == nil && res.RowsAffected == 1 {
			return &history.SyncTask{
				ID:        log.ID,
				StockCode: log.StockCode,
				Period:    log.Period,
				StartDate: log.StartDate,
				EndDate:   log.EndDate,
				Adjusted:  log.Adjusted,
				Status:    history.SyncStatusRunning,
			}
		}
	}
	return nil
}

func (s *Service) GetSyncProgress() ([]syncTaskItem, error) {
	ctx := context.Background()
	var logs []models.KLineSyncLog
	if err := db.Dao.WithContext(ctx).Order("updated_at DESC").Find(&logs).Error; err != nil {
		return nil, err
	}

	items := make([]syncTaskItem, 0, len(logs))
	for _, l := range logs {
		progress := 0
		if l.ExpectedCount > 0 {
			progress = l.SyncedCount * 100 / l.ExpectedCount
			if progress > 100 {
				progress = 100
			}
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
func (s *Service) GetSeedImportStatus() (map[string]any, error) {
	ctx := context.Background()
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

	// Try to find Python. Wails GUI 进程的 PATH 可能不含 Python
	// （或命中 WindowsApps 占位别名，运行脚本时报 exit 9009），
	// 因此额外探测 py 启动器与常见安装路径。
	for _, name := range []string{"python3", "python", "py"} {
		if path, err := exec.LookPath(name); err == nil {
			result["pythonFound"] = true
			result["pythonPath"] = path
			break
		}
	}
	if found, _ := result["pythonFound"].(bool); !found {
		if matches, _ := filepath.Glob(`C:\Program Files\Python3*\python.exe`); len(matches) > 0 {
			result["pythonFound"] = true
			result["pythonPath"] = matches[0]
		} else if matches, _ := filepath.Glob(os.Getenv("LOCALAPPDATA") + `\Programs\Python\Python3*\python.exe`); len(matches) > 0 {
			result["pythonFound"] = true
			result["pythonPath"] = matches[0]
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
func (s *Service) RunSeedImport(pythonPath, startDate, endDate string, limit int) (string, error) {
	ctx := context.Background()
	status, err := s.GetSeedImportStatus()
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

	// Write full log to file regardless of success/failure
	logFile := filepath.Join(seedLogDir, fmt.Sprintf("seed_import_%s.log", time.Now().Format("20060102_150405")))
	_ = os.MkdirAll(seedLogDir, 0755)

	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		fullLog := fmt.Sprintf("STDERR:\n%s\n\nSTDOUT:\n%s", errMsg, stdout.String())
		_ = os.WriteFile(logFile, []byte(fullLog), 0644)

		logger.SugaredLogger.Errorf("seed import failed: %v\n%s", err, errMsg)
		seedImportOutput = errMsg
		return errMsg, fmt.Errorf("种子导入失败，详细日志: %s\n%w\n%s", logFile, err, errMsg)
	}

	output := stdout.String()
	_ = os.WriteFile(logFile, []byte(output), 0644)
	seedImportOutput = output
	logger.SugaredLogger.Infof("seed import completed successfully, log: %s", logFile)
	return output, nil
}

// GetLastSeedImportOutput returns the output from the most recent seed import run.
func (s *Service) GetLastSeedImportOutput() (string, error) {
	return seedImportOutput, nil
}

// RunOptimization runs grid search parameter optimization and returns ranked results.
func (s *Service) RunOptimization(input OptimizationInput) ([]OptimizationResult, error) {
	ctx := context.Background()
	results, err := RunGridSearch(ctx, input)
	if err != nil {
		logger.SugaredLogger.Errorf("optimization failed: %v", err)
		return nil, err
	}
	if len(results) > 0 {
		logger.SugaredLogger.Infof("optimization for %s: %d results, best score=%.3f params=%v",
			input.StockCode, len(results), results[0].ObjectiveScore, results[0].Params)
	}
	return results, nil
}

// GetOptimizationPresets returns preset parameter spaces and default objective config.
func (s *Service) GetOptimizationPresets() (map[string]any, error) {
	return map[string]any{
		"presets":  PresetParamSpaces(),
		"objective": DefaultObjective(),
	}, nil
}

func (s *Service) GetKLineCacheStats() (map[string]any, error) {
	ctx := context.Background()
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
