package backtest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"go-stock/backend/data/datasource"
	"go-stock/backend/data/history"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/stockcode"
	"go-stock/backend/util/timeutil"

	"gorm.io/gorm"
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

type KLineHistoryItem struct {
	ID        uint    `json:"id"`
	StockCode string  `json:"stockCode"`
	StockName string  `json:"stockName"`
	Period    string  `json:"period"`
	TradeDate string  `json:"tradeDate"`
	Adjusted  bool    `json:"adjusted"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    int64   `json:"volume"`
	Amount    float64 `json:"amount"`
}

type KLineHistoryPageData struct {
	List       []KLineHistoryItem `json:"list"`
	Total      int64              `json:"total"`
	Page       int                `json:"page"`
	PageSize   int                `json:"pageSize"`
	TotalPages int                `json:"totalPages"`
}

type SyncTaskItem struct {
	StockCode string `json:"stockCode"`
	Period    string `json:"period"`
	Status    string `json:"status"`
	Progress  int    `json:"progress"`
	ErrorMsg  string `json:"errorMsg"`
}

type SyncProgressSummary struct {
	Pending int64 `json:"pending"`
	Running int64 `json:"running"`
	Done    int64 `json:"done"`
	Failed  int64 `json:"failed"`
	Partial int64 `json:"partial"`
}

type SyncProgressData struct {
	Summary SyncProgressSummary `json:"summary"`
	Tasks   []SyncTaskItem      `json:"tasks"`
}

// GetKLineHistory returns stored K-line bars with stock code/name search.
func (s *Service) GetKLineHistory(keyword string, page, pageSize int) (*KLineHistoryPageData, error) {
	ctx := context.Background()
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	var codes []string
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		var err error
		codes, err = kLineSearchCodes(ctx, keyword)
		if err != nil {
			return nil, err
		}
	}

	bars, total, err := datasource.NewKLineStore().QueryKLinePage(ctx, codes, page, pageSize)
	if err != nil {
		return nil, err
	}
	names, err := kLineStockNames(ctx, bars)
	if err != nil {
		return nil, err
	}

	items := make([]KLineHistoryItem, 0, len(bars))
	for _, bar := range bars {
		code := stockcode.Normalize(bar.StockCode)
		items = append(items, KLineHistoryItem{
			ID:        bar.ID,
			StockCode: code,
			StockName: names[code],
			Period:    bar.Period,
			TradeDate: bar.TradeDate,
			Adjusted:  bar.Adjusted,
			Open:      bar.Open,
			High:      bar.High,
			Low:       bar.Low,
			Close:     bar.Close,
			Volume:    bar.Volume,
			Amount:    bar.Amount,
		})
	}

	return &KLineHistoryPageData{
		List:       items,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: int((total + int64(pageSize) - 1) / int64(pageSize)),
	}, nil
}

func kLineSearchCodes(ctx context.Context, keyword string) ([]string, error) {
	set := make(map[string]struct{})
	addStockCodeCandidates(set, keyword)

	like := "%" + keyword + "%"
	var infos []models.AllStockInfo
	if err := db.Dao.WithContext(ctx).Model(&models.AllStockInfo{}).
		Select("secucode").
		Where("secucode LIKE ? OR sec_uri_tycode LIKE ? OR sec_uri_tynameabbr LIKE ?", like, like, like).
		Find(&infos).Error; err != nil {
		return nil, err
	}
	for _, info := range infos {
		addStockCodeCandidates(set, info.SECUCODE)
	}

	codes := make([]string, 0, len(set))
	for code := range set {
		codes = append(codes, code)
	}
	return codes, nil
}

func kLineStockNames(ctx context.Context, bars []models.KLineBar) (map[string]string, error) {
	set := make(map[string]struct{})
	for _, bar := range bars {
		addStockCodeCandidates(set, bar.StockCode)
	}
	if len(set) == 0 {
		return map[string]string{}, nil
	}

	codes := make([]string, 0, len(set))
	for code := range set {
		codes = append(codes, code)
	}
	var infos []models.AllStockInfo
	if err := db.Dao.WithContext(ctx).Model(&models.AllStockInfo{}).
		Select("secucode, sec_uri_tynameabbr").
		Where("secucode IN ? OR sec_uri_tycode IN ?", codes, codes).
		Find(&infos).Error; err != nil {
		return nil, err
	}

	names := make(map[string]string, len(infos))
	for _, info := range infos {
		names[stockcode.Normalize(info.SECUCODE)] = info.SECURITYNAMEABBR
	}
	return names, nil
}

func addStockCodeCandidates(set map[string]struct{}, code string) {
	for _, candidate := range stockcode.StockCodeCandidates(code) {
		set[candidate] = struct{}{}
	}
}

func (s *Service) StartHistoricalSync(years int) error {
	ctx := context.Background()
	now := time.Now()
	startDate := now.AddDate(-years, 0, 0).Format("2006-01-02")
	endDate := now.Format("2006-01-02")

	if n, err := history.RepairBrokenSyncTasks(ctx); err == nil && n > 0 {
		logger.SugaredLogger.Infof("historical sync: repaired %d broken/fake-done tasks", n)
	}
	if n, err := history.ReclaimStaleRunningSyncTasks(ctx, history.DefaultStaleRunningAge); err == nil && n > 0 {
		logger.SugaredLogger.Infof("historical sync: reclaimed %d stale running tasks", n)
	}

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
	skipped := 0
	for _, info := range infos {
		// Rule 3：任务代码统一内部格式（修复 done 跳过检查被混格式行误命中）
		code := stockcode.Normalize(info.SECUCODE)
		if code == "" {
			code = info.SECUCODE
		}
		if _, ok := running[code]; ok {
			continue
		}
		var prior models.KLineSyncLog
		if err := db.Dao.WithContext(ctx).
			Where("stock_code = ? AND period = ? AND adjusted = ?", code, "day", true).
			First(&prior).Error; err == nil &&
			prior.Status == history.SyncStatusDone &&
			prior.EndDate >= endDate && prior.StartDate <= startDate &&
			history.SyncCheckpointCaughtUp(prior.LastSyncDate, endDate) {
			skipped++
			continue
		}
		if err := history.CreateSyncTask(ctx, code, "day", startDate, endDate, true); err != nil {
			logger.SugaredLogger.Errorf("create sync task failed for %s: %v", code, err)
			continue
		}
		created++
	}

	logger.SugaredLogger.Infof("historical sync: queued %d, skipped %d already up-to-date (%d years)", created, skipped, years)

	startSyncWorkers()
	return nil
}

// RetryFailedSync 只把 failed 和僵死 running 重新排队。
// 不扫描全市场，也不把已完成任务的 end_date 延长到今天。
func (s *Service) RetryFailedSync() (int64, error) {
	ctx := context.Background()
	res := db.Dao.WithContext(ctx).Model(&models.KLineSyncLog{}).
		Where("status = ?", history.SyncStatusFailed).
		Updates(map[string]interface{}{
			"status":         history.SyncStatusPending,
			"error_msg":      "",
			"last_sync_date": "",
			"updated_at":     time.Now(),
		})
	if res.Error != nil {
		return 0, res.Error
	}
	n := res.RowsAffected
	if reclaimed, err := history.ReclaimStaleRunningSyncTasks(ctx, history.DefaultStaleRunningAge); err != nil {
		return n, err
	} else {
		n += reclaimed
	}
	logger.SugaredLogger.Infof("retry failed sync: requeued %d", n)
	if n > 0 {
		startSyncWorkers()
	}
	return n, nil
}

// CatchUpSync 把截止日或断点早于 through 的已完成任务重新排队，只从断点补尾部。
// 不清 last_sync_date、不清空窗。through 为空时用今天。
func (s *Service) CatchUpSync(through string) (int64, error) {
	through = strings.TrimSpace(through)
	if through == "" {
		through = timeutil.TodayStr()
	}
	if _, err := timeutil.ParseDate(through); err != nil {
		return 0, fmt.Errorf("invalid through date %q", through)
	}
	ctx := context.Background()
	res := db.Dao.WithContext(ctx).Model(&models.KLineSyncLog{}).
		Where("status = ? AND (end_date < ? OR IFNULL(last_sync_date, '') < ?)", history.SyncStatusDone, through, through).
		Updates(map[string]interface{}{
			"status":     history.SyncStatusPending,
			"error_msg":  "",
			"updated_at": time.Now(),
			"end_date":   gorm.Expr("CASE WHEN end_date < ? THEN ? ELSE end_date END", through, through),
		})
	if res.Error != nil {
		return 0, res.Error
	}
	logger.SugaredLogger.Infof("catch-up sync through %s: requeued %d", through, res.RowsAffected)
	if res.RowsAffected > 0 {
		startSyncWorkers()
	}
	return res.RowsAffected, nil
}

// ResumePendingSyncTasks 应用启动时恢复 K 线同步：回收僵死 running、延长过期 end_date、
// 后台扫描 done 但本地仍有缺口的任务；仅 pending 时也会拉起 workers。
func (s *Service) ResumePendingSyncTasks() {
	history.PrepareSyncQueueOnStartup(startSyncWorkers)

	var cnt int64
	if err := db.Dao.Model(&models.KLineSyncLog{}).
		Where("status = ?", history.SyncStatusPending).Count(&cnt).Error; err != nil {
		logger.SugaredLogger.Warnf("查询待同步 K 线任务失败: %v", err)
		return
	}
	if cnt == 0 {
		return
	}
	logger.SugaredLogger.Infof("发现 %d 个待执行 K 线同步任务，恢复后台同步", cnt)
	startSyncWorkers()
}

// syncWorkerCount is the number of goroutines consuming pending sync tasks.
const syncWorkerCount = 6

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
		for range syncWorkerCount {
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
	// 抢到行但更新失败是并发冲突，继续抢；只有队列空了才退出。
	// 固定次数用尽就返回会让 worker 提前结束，剩下的 pending 要等下次启动。
	for range 50 {
		var log models.KLineSyncLog
		err := db.Dao.WithContext(ctx).
			Where("status = ?", history.SyncStatusPending).
			Order("id ASC").
			First(&log).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			logger.SugaredLogger.Warnf("claim sync task: %v", err)
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
				ID:           log.ID,
				StockCode:    log.StockCode,
				Period:       log.Period,
				StartDate:    log.StartDate,
				EndDate:      log.EndDate,
				Adjusted:     log.Adjusted,
				Status:       history.SyncStatusRunning,
				LastSyncDate: log.LastSyncDate,
			}
		}
	}
	return nil
}

func (s *Service) GetSyncProgress() (*SyncProgressData, error) {
	ctx := context.Background()
	var summary SyncProgressSummary
	var counts []struct {
		Status string
		Count  int64
	}
	if err := db.Dao.WithContext(ctx).Model(&models.KLineSyncLog{}).
		Select("status, COUNT(*) AS count").
		Group("status").
		Scan(&counts).Error; err != nil {
		return nil, err
	}
	for _, row := range counts {
		switch row.Status {
		case history.SyncStatusPending:
			summary.Pending = row.Count
		case history.SyncStatusRunning:
			summary.Running = row.Count
		case history.SyncStatusDone:
			summary.Done = row.Count
		case history.SyncStatusFailed:
			summary.Failed = row.Count
		case history.SyncStatusPartial:
			summary.Partial = row.Count
		}
	}

	var active []models.KLineSyncLog
	if err := db.Dao.WithContext(ctx).
		Where("status <> ?", history.SyncStatusDone).
		Order("updated_at DESC").
		Find(&active).Error; err != nil {
		return nil, err
	}
	var done []models.KLineSyncLog
	if err := db.Dao.WithContext(ctx).
		Where("status = ?", history.SyncStatusDone).
		Order("updated_at DESC").
		Limit(100).
		Find(&done).Error; err != nil {
		return nil, err
	}
	logs := append(active, done...)

	items := make([]SyncTaskItem, 0, len(logs))
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

		items = append(items, SyncTaskItem{
			StockCode: l.StockCode,
			Period:    l.Period,
			Status:    l.Status,
			Progress:  progress,
			ErrorMsg:  l.ErrorMsg,
		})
	}

	return &SyncProgressData{Summary: summary, Tasks: items}, nil
}

// GetSeedImportStatus returns information about seed data availability
// and environment readiness for running the baostock seed script.
func (s *Service) GetSeedImportStatus() (map[string]any, error) {
	ctx := context.Background()
	result := map[string]any{
		"seedBars":    int64(0),
		"seedStocks":  int64(0),
		"dbPath":      "",
		"scriptPath":  "",
		"pythonFound": false,
		"pythonPath":  "",
		"hasSeedData": false,
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
		"presets":   PresetParamSpaces(),
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
	// 零值日期 0001-01-01 和未来脏日期不参与起止；结束日取覆盖面接近峰值的最近交易日，
	// 而不是股票数最多的那一天（那一天会停在节后首个交易日）。
	tomorrow := timeutil.DateStr(time.Now().AddDate(0, 0, 1))
	db.Dao.WithContext(ctx).Raw(`
SELECT MIN(substr(trade_date, 1, 10)) FROM kline_bars
WHERE period IN ('day','101')
  AND substr(trade_date, 1, 10) >= '1990-01-01'
  AND substr(trade_date, 1, 10) <= ?`, tomorrow).Row().Scan(&minDate)
	maxDate = datasource.NewKLineStore().LatestMarketTradeDate(ctx, 45, 500)

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
