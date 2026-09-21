package data

// Kronos K线预测服务：进程管理器（托管 Python FastAPI 子进程）+ HTTP 客户端。
// 参考 https://github.com/shiyu-coder/Kronos ，服务端实现在 python/kronos_service/。
// 功能默认关闭（Settings.KronosEnable=false），未开启时对现有流程零影响。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-stock/backend/data/datasource"
	"go-stock/backend/logger"
)

// KronosForecast 单只持仓/股票的预测摘要（供数据包与 prompt 使用）
type KronosForecast struct {
	Direction   string  `json:"direction"`   // up / down
	ChangePct   float64 `json:"changePct"`   // 预测期末相对现价涨跌 %
	PredEnd     float64 `json:"predEnd"`     // 预测期末收盘
	PredHigh    float64 `json:"predHigh"`    // 预测期最高
	PredLow     float64 `json:"predLow"`     // 预测期最低
	Confidence  float64 `json:"confidence"`  // 0~100
	PredLen     int     `json:"predLen"`
}

// KronosBar 一根预测K线
type KronosBar struct {
	Date   string  `json:"date"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
}

// KronosPrediction 预测结果（前端 K 线叠加使用）
type KronosPrediction struct {
	Bars     []*KronosBar    `json:"bars"`
	Summary  *KronosForecast `json:"summary"`
}

// 预测错误码（前端按码提示）
var (
	ErrKronosDisabled      = errors.New("KRONOS_DISABLED")
	ErrKronosOffline       = errors.New("KRONOS_OFFLINE")
	ErrKronosTimeout       = errors.New("KRONOS_TIMEOUT")
	ErrKronosModelMissing  = errors.New("KRONOS_MODEL_MISSING")
	ErrKronosInsuffData    = errors.New("KRONOS_INSUFFICIENT_DATA")
)

// KronosConfigSnapshot 预测所需配置快照
type KronosConfigSnapshot struct {
	Enable     bool
	PythonPath string
	Port       int
	Model      string
	Device     string
	T          float64
	TopP       float64
	PredLen    int
	SampleCnt  int
	LazyStart  bool
}

// applyKronosDefaults 补齐 Kronos 配置默认值（读取设置后调用）
func applyKronosDefaults(s *Settings) {
	if s.KronosPort <= 0 {
		s.KronosPort = 8765
	}
	if s.KronosModel == "" {
		s.KronosModel = "NeoQuasar/Kronos-small"
	}
	if s.KronosDevice == "" {
		s.KronosDevice = "auto"
	}
	if s.KronosT <= 0 {
		s.KronosT = 1.0
	}
	if s.KronosTopP <= 0 {
		s.KronosTopP = 0.9
	}
	if s.KronosPredLen <= 0 {
		s.KronosPredLen = 5
	}
	if s.KronosSampleCnt <= 0 {
		s.KronosSampleCnt = 4
	}
}

// GetKronosConfig 从设置读取 Kronos 配置快照
func GetKronosConfig() KronosConfigSnapshot {
	cfg := GetSettingConfigSafe()
	s := cfg.Settings
	if s == nil {
		return KronosConfigSnapshot{}
	}
	return KronosConfigSnapshot{
		Enable:     s.KronosEnable,
		PythonPath: s.KronosPythonPath,
		Port:       s.KronosPort,
		Model:      s.KronosModel,
		Device:     s.KronosDevice,
		T:          s.KronosT,
		TopP:       s.KronosTopP,
		PredLen:    s.KronosPredLen,
		SampleCnt:  s.KronosSampleCnt,
		LazyStart:  s.KronosLazyStart,
	}
}

// kronosProcess 管理托管的 Python 子进程（全局单例）
type kronosProcess struct {
	mu        sync.Mutex
	cmd       *exec.Cmd
	starting  bool
	startedAt time.Time
}

var kronosProc = &kronosProcess{}

// kronosAssetsFS 由 main 包通过 RegisterKronosAssets 注入（go:embed python/kronos_service），
// 使打包后的 exe 不依赖源码仓库目录即可释放 Python 服务。
var kronosAssetsFS fs.FS

// RegisterKronosAssets 注入内嵌的 Kronos Python 服务源码。
func RegisterKronosAssets(fsys fs.FS) {
	kronosAssetsFS = fsys
}

// normalizePythonExe 规整用户配置的 Python 路径：
// - 空 → 依次尝试 PATH 中的 python / py 启动器
// - 目录 → 自动补 python.exe
// - 无 .exe 后缀且不存在 → 尝试补 .exe
func normalizePythonExe(p string) (string, error) {
	p = strings.Trim(strings.TrimSpace(p), `"'`)
	if p == "" {
		if path, err := exec.LookPath("python"); err == nil {
			return path, nil
		}
		if path, err := exec.LookPath("py"); err == nil {
			return path, nil
		}
		return "", errors.New("未找到 Python：请在 PATH 安装 Python 3.10+，或在设置页填写 python.exe 完整路径")
	}
	if st, err := os.Stat(p); err == nil && st.IsDir() {
		p = filepath.Join(p, "python.exe")
	} else if err != nil && !strings.EqualFold(filepath.Ext(p), ".exe") && strings.ContainsAny(p, `\/`) {
		// 看起来像路径但不存在且没带扩展名：尝试补 .exe
		if _, err2 := os.Stat(p + ".exe"); err2 == nil {
			p += ".exe"
		}
	}
	if _, err := os.Stat(p); err != nil {
		if strings.ContainsAny(p, `\/`) {
			return "", fmt.Errorf("Python 路径不存在: %s", p)
		}
		if _, err2 := exec.LookPath(p); err2 != nil {
			return "", fmt.Errorf("未找到 Python: %s", p)
		}
	}
	return p, nil
}

// resolveKronosServiceDir 解析 Python 服务目录：
// 1) 工作目录/python/kronos_service（开发模式）
// 2) exe 同级/python/kronos_service（绿色部署）
// 3) 都不存在时从内嵌资源释放到 exe 同级
func resolveKronosServiceDir() (string, error) {
	if wd, err := os.Getwd(); err == nil {
		dir := filepath.Join(wd, "python", "kronos_service")
		if fileExists(filepath.Join(dir, "app.py")) {
			return dir, nil
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("无法定位程序目录: %w", err)
	}
	dir := filepath.Join(filepath.Dir(exe), "python", "kronos_service")
	if fileExists(filepath.Join(dir, "app.py")) {
		return dir, nil
	}
	if kronosAssetsFS == nil {
		return "", fmt.Errorf("缺少 Kronos 服务源码目录: %s", dir)
	}
	if err := extractKronosAssets(dir); err != nil {
		return "", fmt.Errorf("释放 Kronos 服务源码失败: %w", err)
	}
	logger.SugaredLogger.Infof("已释放 Kronos 服务源码到 %s", dir)
	return dir, nil
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// extractKronosAssets 将内嵌的 python/kronos_service 释放到目标目录。
func extractKronosAssets(dstDir string) error {
	const srcRoot = "python/kronos_service"
	return fs.WalkDir(kronosAssetsFS, srcRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dstDir, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, err := fs.ReadFile(kronosAssetsFS, path)
		if err != nil {
			return err
		}
		// 已存在则跳过（允许用户自行修改释放后的脚本）
		if fileExists(target) {
			return nil
		}
		return os.WriteFile(target, content, 0o644)
	})
}

func kronosBaseURL(port int) string {
	if port <= 0 {
		port = 8765
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

func (p *kronosProcess) isAlive(ctx context.Context, port int) bool {
	url := kronosBaseURL(port) + "/health"
	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// EnsureRunning 确保服务在线；lazyStart 时未运行则拉起并等待就绪。
func (p *kronosProcess) EnsureRunning(cfg KronosConfigSnapshot) error {
	if !cfg.Enable {
		return ErrKronosDisabled
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if p.isAlive(ctx, cfg.Port) {
		return nil
	}
	if !cfg.LazyStart {
		return ErrKronosOffline
	}
	if err := p.Start(cfg); err != nil {
		return err
	}
	// 首次加载模型/启动 Python 可能较慢，最长等 120s
	deadline := time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		c2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
		ok := p.isAlive(c2, cfg.Port)
		cancel2()
		if ok {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return ErrKronosTimeout
}

// Start 拉起 Python 服务子进程（幂等：已在线或启动中直接返回）
func (p *kronosProcess) Start(cfg KronosConfigSnapshot) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd != nil && p.starting {
		return nil
	}
	python, err := normalizePythonExe(cfg.PythonPath)
	if err != nil {
		logger.SugaredLogger.Errorf("Kronos Python 解释器不可用: %v", err)
		return err
	}
	port := cfg.Port
	if port <= 0 {
		port = 8765
	}
	// 服务目录：工作目录 → exe 同级 → 从内嵌资源释放到 exe 同级
	dir, err := resolveKronosServiceDir()
	if err != nil {
		logger.SugaredLogger.Errorf("Kronos 服务目录不可用: %v", err)
		return err
	}
	cmd := exec.Command(python, "app.py", "--port", fmt.Sprintf("%d", port))
	cmd.Dir = dir
	if err := cmd.Start(); err != nil {
		logger.SugaredLogger.Errorf("启动 Kronos 服务失败: %v", err)
		return fmt.Errorf("启动 Kronos 服务失败: %w（Python=%s, 目录=%s）", err, python, dir)
	}
	p.cmd = cmd
	p.starting = true
	p.startedAt = time.Now()
	go func(c *exec.Cmd) {
		_ = c.Wait() // 回收子进程，防止僵尸
		p.mu.Lock()
		p.starting = false
		p.mu.Unlock()
	}(cmd)
	logger.SugaredLogger.Infof("Kronos 服务已拉起 (port=%d)", port)
	return nil
}

// Stop 停止托管的 Python 服务
func (p *kronosProcess) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
		p.cmd = nil
		p.starting = false
		logger.SugaredLogger.Info("Kronos 服务已停止")
	}
}

// Status 返回服务状态：online / offline / disabled
func (p *kronosProcess) Status(cfg KronosConfigSnapshot) string {
	if !cfg.Enable {
		return "disabled"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if p.isAlive(ctx, cfg.Port) {
		return "online"
	}
	return "offline"
}

// kronosPredict 调用 /predict（内部：确保服务在线 → POST → 解析）
func kronosPredict(ctx context.Context, apiCode string, predLen int) (*KronosPrediction, error) {
	return kronosPredictEx(ctx, apiCode, predLen, "")
}

// kronosPredictEx 同 kronosPredict，支持 asOfDate 历史切点（回测模式用）
func kronosPredictEx(ctx context.Context, apiCode string, predLen int, asOfDate string) (*KronosPrediction, error) {
	cfg := GetKronosConfig()
	if !cfg.Enable {
		return nil, ErrKronosDisabled
	}
	if err := kronosProc.EnsureRunning(cfg); err != nil {
		return nil, err
	}
	if predLen <= 0 {
		if cfg.PredLen > 0 {
			predLen = cfg.PredLen
		} else {
			predLen = 5
		}
	}

	// 取历史日K（lookback 120）
	klineData, err := datasource.GetRouter().GetKLine(ctx, apiCode, "101", 120)
	if err != nil || klineData == nil {
		if err == nil {
			err = ErrKronosInsuffData
		}
		return nil, err
	}
	kline := klineData.Bars
	if len(kline) < 30 {
		return nil, ErrKronosInsuffData
	}
	if len(kline) > 512 {
		kline = kline[len(kline)-512:]
	}

	candles := make([]map[string]any, 0, len(kline))
	for _, b := range kline {
		candles = append(candles, map[string]any{
			"date": b.Time.Format("2006-01-02"), "open": b.Open, "high": b.High, "low": b.Low,
			"close": b.Close, "volume": float64(b.Volume), "amount": b.Amount,
		})
	}
	return kronosPostPredict(ctx, cfg, candles, predLen, asOfDate, 30*time.Second)
}

// kronosPostPredict 向 Python 服务发 /predict 并解析（kronosPredictEx 与回测共用）。
func kronosPostPredict(ctx context.Context, cfg KronosConfigSnapshot, candles []map[string]any, predLen int, asOfDate string, timeout time.Duration) (*KronosPrediction, error) {
	payload := map[string]any{
		"candles":     candles,
		"predLen":     predLen,
		"T":           orDefault(cfg.T, 1.0),
		"topP":        orDefault(cfg.TopP, 0.9),
		"sampleCount": orDefaultI(cfg.SampleCnt, 4),
		"model":       orDefaultStr(cfg.Model, "NeoQuasar/Kronos-small"),
		"device":      orDefaultStr(cfg.Device, "auto"),
	}
	if asOfDate != "" {
		payload["asOfDate"] = asOfDate
	}
	body, _ := json.Marshal(payload)

	pCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, rerr := http.NewRequestWithContext(pCtx, http.MethodPost, kronosBaseURL(cfg.Port)+"/predict", bytes.NewReader(body))
	if rerr != nil {
		return nil, rerr
	}
	req.Header.Set("Content-Type", "application/json")
	resp, derr := (&http.Client{Timeout: timeout + 5*time.Second}).Do(req)
	if derr != nil {
		var netErr net.Error
		if errors.Is(derr, context.DeadlineExceeded) || (errors.As(derr, &netErr) && netErr.Timeout()) {
			return nil, ErrKronosTimeout
		}
		return nil, ErrKronosOffline
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// continue below
	case 503:
		return nil, ErrKronosModelMissing
	case 422:
		return nil, ErrKronosInsuffData
	default:
		return nil, fmt.Errorf("KRONOS_HTTP_%d", resp.StatusCode)
	}

	var out struct {
		Bars    []*KronosBar     `json:"bars"`
		Summary *KronosForecast  `json:"summary"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.Summary != nil && out.Summary.PredLen == 0 {
		out.Summary.PredLen = predLen
	}
	return &KronosPrediction{Bars: out.Bars, Summary: out.Summary}, nil
}

// KronosBacktestResult 历史切点回测结果：预测 vs 真实对照
type KronosBacktestResult struct {
	AsOfDate      string            `json:"asOfDate"`
	Prediction    *KronosPrediction `json:"prediction"`
	ActualBars    []*KronosBar      `json:"actualBars"`    // 切点后的真实K线（与预测等长或更短）
	DirectionHit  bool              `json:"directionHit"`  // 预测方向与真实涨跌是否一致
	MeanAbsErrPct float64           `json:"meanAbsErrPct"` // 预测收盘 vs 真实收盘的平均绝对偏差 %
}

// KronosBacktest 历史切点回测：以 asOfDate 为切点预测，与切点后真实走势对照。
func KronosBacktest(ctx context.Context, stockCode string, asOfDate string, predLen int) (*KronosBacktestResult, error) {
	cfg := GetKronosConfig()
	if !cfg.Enable {
		return nil, ErrKronosDisabled
	}
	if err := kronosProc.EnsureRunning(cfg); err != nil {
		return nil, err
	}
	if predLen <= 0 {
		predLen = orDefaultI(cfg.PredLen, 5)
	}
	apiCode := NormalizeKronosCode(stockCode)
	klineData, err := datasource.GetRouter().GetKLine(ctx, apiCode, "101", 200)
	if err != nil || klineData == nil || len(klineData.Bars) < 30 {
		return nil, ErrKronosInsuffData
	}
	bars := klineData.Bars

	// 未指定切点时默认取倒数第 predLen+1 根（保证有真实对照数据）
	cutIdx := -1
	if asOfDate != "" {
		for i, b := range bars {
			if b.Time.Format("2006-01-02") <= asOfDate {
				cutIdx = i
			}
		}
	} else {
		cutIdx = len(bars) - 1 - predLen
	}
	if cutIdx < 29 {
		return nil, ErrKronosInsuffData
	}
	hist := bars[:cutIdx+1]
	actual := bars[cutIdx+1:]
	if len(actual) == 0 {
		return nil, fmt.Errorf("切点 %s 之后没有真实K线可对照", hist[cutIdx].Time.Format("2006-01-02"))
	}
	if len(hist) > 512 {
		hist = hist[len(hist)-512:]
	}
	candles := make([]map[string]any, 0, len(hist))
	for _, b := range hist {
		candles = append(candles, map[string]any{
			"date": b.Time.Format("2006-01-02"), "open": b.Open, "high": b.High, "low": b.Low,
			"close": b.Close, "volume": float64(b.Volume), "amount": b.Amount,
		})
	}
	pred, err := kronosPostPredict(ctx, cfg, candles, predLen, "", 60*time.Second)
	if err != nil {
		return nil, err
	}

	// 对照统计
	actBars := make([]*KronosBar, 0, len(actual))
	for _, b := range actual {
		actBars = append(actBars, &KronosBar{
			Date: b.Time.Format("2006-01-02"), Open: b.Open, High: b.High, Low: b.Low,
			Close: b.Close, Volume: float64(b.Volume),
		})
	}
	n := len(pred.Bars)
	if len(actBars) < n {
		n = len(actBars)
	}
	res := &KronosBacktestResult{
		AsOfDate:   hist[len(hist)-1].Time.Format("2006-01-02"),
		Prediction: pred,
		ActualBars: actBars,
	}
	if n > 0 {
		actualEnd := actBars[n-1].Close
		lastClose := hist[len(hist)-1].Close
		actualDir := "up"
		if actualEnd < lastClose {
			actualDir = "down"
		}
		res.DirectionHit = pred.Summary != nil && pred.Summary.Direction == actualDir
		var sumErr float64
		for i := 0; i < n; i++ {
			if actBars[i].Close > 0 {
				sumErr += abs64(pred.Bars[i].Close-actBars[i].Close) / actBars[i].Close * 100
			}
		}
		res.MeanAbsErrPct = roundPrice(sumErr / float64(n))
	}
	return res, nil
}

// kronosBatchItem 批量预测单项（code 为标识，可带任意业务含义）
type kronosBatchItem struct {
	Code    string           `json:"code"`
	Candles []map[string]any `json:"candles"`
}

// barsToKronosCandles 转 /predict 协议 candles（截断到 512 根由调用方负责）
func barsToKronosCandles(bars []datasource.KLineBar) []map[string]any {
	candles := make([]map[string]any, 0, len(bars))
	for _, b := range bars {
		candles = append(candles, map[string]any{
			"date": b.Time.Format("2006-01-02"), "open": b.Open, "high": b.High, "low": b.Low,
			"close": b.Close, "volume": float64(b.Volume), "amount": b.Amount,
		})
	}
	return candles
}

// KronosBatchPredict 批量预测多只股票（每日推荐因子用）。
// 返回 per-code 预测结果与失败原因；整体失败（服务离线等）返回 error。
func KronosBatchPredict(ctx context.Context, apiCodes []string, predLen int) (map[string]*KronosPrediction, map[string]string, error) {
	cfg := GetKronosConfig()
	if !cfg.Enable {
		return nil, nil, ErrKronosDisabled
	}
	if err := kronosProc.EnsureRunning(cfg); err != nil {
		return nil, nil, err
	}
	if predLen <= 0 {
		predLen = orDefaultI(cfg.PredLen, 5)
	}
	items := make([]kronosBatchItem, 0, len(apiCodes))
	preErrs := map[string]string{}
	for _, code := range apiCodes {
		klineData, err := datasource.GetRouter().GetKLine(ctx, code, "101", 120)
		if err != nil || klineData == nil || len(klineData.Bars) < 30 {
			preErrs[code] = "K线数据不足"
			continue
		}
		bars := klineData.Bars
		if len(bars) > 512 {
			bars = bars[len(bars)-512:]
		}
		items = append(items, kronosBatchItem{Code: code, Candles: barsToKronosCandles(bars)})
	}
	if len(items) == 0 {
		return nil, preErrs, ErrKronosInsuffData
	}
	results, berrs, err := kronosPostBatch(ctx, cfg, items, predLen)
	for code, msg := range berrs {
		preErrs[code] = msg
	}
	return results, preErrs, err
}

// kronosPostBatch 向 Python 服务发 /predict_batch（KronosBatchPredict 与滚动回测共用）。
func kronosPostBatch(ctx context.Context, cfg KronosConfigSnapshot, items []kronosBatchItem, predLen int) (map[string]*KronosPrediction, map[string]string, error) {
	payload := map[string]any{
		"items":       items,
		"predLen":     predLen,
		"T":           orDefault(cfg.T, 1.0),
		"topP":        orDefault(cfg.TopP, 0.9),
		"sampleCount": orDefaultI(cfg.SampleCnt, 4),
		"model":       orDefaultStr(cfg.Model, "NeoQuasar/Kronos-small"),
		"device":      orDefaultStr(cfg.Device, "auto"),
	}
	body, _ := json.Marshal(payload)

	// 批量可能耗时较长：每项最多 ~10s 的预算，上限 10 分钟
	budget := time.Duration(len(items))*10*time.Second + 60*time.Second
	if budget > 10*time.Minute {
		budget = 10 * time.Minute
	}
	pCtx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	req, rerr := http.NewRequestWithContext(pCtx, http.MethodPost, kronosBaseURL(cfg.Port)+"/predict_batch", bytes.NewReader(body))
	if rerr != nil {
		return nil, nil, rerr
	}
	req.Header.Set("Content-Type", "application/json")
	resp, derr := (&http.Client{Timeout: budget + 30*time.Second}).Do(req)
	if derr != nil {
		var netErr net.Error
		if errors.Is(derr, context.DeadlineExceeded) || (errors.As(derr, &netErr) && netErr.Timeout()) {
			return nil, nil, ErrKronosTimeout
		}
		return nil, nil, ErrKronosOffline
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("KRONOS_HTTP_%d", resp.StatusCode)
	}
	var out struct {
		Results map[string]*KronosPrediction `json:"results"`
		Errors  map[string]string            `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, nil, err
	}
	return out.Results, out.Errors, nil
}

func orDefault(v, d float64) float64 {
	if v <= 0 {
		return d
	}
	return v
}
func orDefaultI(v, d int) int {
	if v <= 0 {
		return d
	}
	return v
}
func orDefaultStr(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

// NormalizeKronosCode 将前端 K 线组件使用的代码（东经前缀 "1.600519"/"0.000001"/"105.AAPL"、
// 后缀 "600519.SH"、裸码 "600519" 等）规范为 K 线接口使用的代码。
func NormalizeKronosCode(code string) string {
	code = strings.TrimSpace(code)
	if idx := strings.Index(code, "."); idx > 0 {
		prefix, rest := code[:idx], code[idx+1:]
		switch prefix {
		case "0": // 深市
			code = "sz" + rest
		case "1": // 沪市
			code = "sh" + rest
		default:
			if _, err := strconv.Atoi(prefix); err == nil && len(prefix) <= 3 && len(rest) != 6 {
				// 美股/港股等东财市场前缀（105/106/107/116...），保留主体
				code = rest
			}
		}
	}
	return normalizeTradingRecordAPI(code)
}

// PredictKLineForStock 预测单只股票未来日K（供 Wails handler 调用）。
func PredictKLineForStock(ctx context.Context, stockCode string, predLen int) (*KronosPrediction, error) {
	apiCode := NormalizeKronosCode(stockCode)
	if apiCode == "" {
		return nil, ErrKronosInsuffData
	}
	return kronosPredict(ctx, apiCode, predLen)
}

// KronosServiceStatus 返回服务状态：disabled / offline / online
func KronosServiceStatus() string {
	return kronosProc.Status(GetKronosConfig())
}

// KronosServiceStart 手动启动托管服务（忽略惰性启动配置）
func KronosServiceStart() error {
	cfg := GetKronosConfig()
	if !cfg.Enable {
		return ErrKronosDisabled
	}
	if err := kronosProc.Start(cfg); err != nil {
		return err
	}
	deadline := time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		c2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
		ok := kronosProc.isAlive(c2, cfg.Port)
		cancel2()
		if ok {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return ErrKronosTimeout
}

// KronosServiceStop 停止托管服务
func KronosServiceStop() {
	kronosProc.Stop()
}
