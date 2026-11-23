package khunter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/data/datasource"
	"go-stock/backend/data/khunter/risk"
	"go-stock/backend/data/khunter/scorer"
	"go-stock/backend/data/khunter/strategy"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/stockcode"
	"go-stock/backend/util/timeutil"
)

// SupportPrice 支撑位 = 近 20 根（不足则全部）收盘价的最低值
func SupportPrice(bars []models.KLineBar) float64 {
	if len(bars) == 0 {
		return 0
	}
	n := len(bars)
	from := n - 20
	if from < 0 {
		from = 0
	}
	support := bars[from].Close
	for i := from; i < n; i++ {
		if bars[i].Close < support {
			support = bars[i].Close
		}
	}
	return support
}

// HuntingThreshold 狩猎场入选分数阈值 = 60 + 风险档加成
func HuntingThreshold(lv risk.Level) float64 { return 60 + lv.ScoreExtra }

// PipelineResult 流水线执行结果
type PipelineResult struct {
	Signals             int
	Candidates          int
	Scored              int
	Observed            int // 本次落库的观察数（候选不再直接入池）
	Hunted              int // 本次确认入池的交易数（确认引擎接入前恒为 0）
	Removed             int
	RiskLevel           string
	TradeDate           string
	StockTotal          int `json:"stockTotal"`
	FreshStocks         int `json:"freshStocks"`
	InsufficientHistory int `json:"insufficientHistory"`
	MissingTradeDate    int `json:"missingTradeDate"`
	QueryFailed         int `json:"queryFailed"`
	// Readiness 当日日K同步就绪结论（确认前置条件）；Completed=false 时确认被
	// 跳过但盘中观察照常保存（observe-only），Reason 为诊断信息
	Readiness *DataReadiness `json:"readiness,omitempty"`
	// 观察/确认会计（运行汇总同名字段一并持久化）
	InvalidDataCount int `json:"invalidDataCount"` // 数据闸门不放行观察的候选数
	IntradayCount    int `json:"intradayCount"`    // 以盘中/未同步数据保存的观察数
	ConfirmedCount   int `json:"confirmedCount"`   // 确认入池交易数（= Hunted）
	ExpiredCount     int `json:"expiredCount"`     // 确认窗口过期观察数
	UntradeableCount int `json:"untradeableCount"` // 已确认触发但不足一手
	// Limitations 研究运行的数据局限说明（幸存者偏差、清单读取失败等）
	Limitations []string `json:"limitations,omitempty"`
	// NotBacktestable 缺时点快照、不得用最新值替代的标的（研究运行）
	NotBacktestable []string `json:"notBacktestable,omitempty"`
}

// RunMode 运行模式：live 实时生产运行（写生产表）；research 历史研究运行（只写研究记录）
type RunMode string

const (
	RunModeLive     RunMode = "live"
	RunModeResearch RunMode = "research"
)

// PipelineOptions 流水线运行选项。Mode 为空按 live 处理；Now 为空取当前时间。
type PipelineOptions struct {
	TradeDate string
	Mode      RunMode
	Now       time.Time
}

// resolveTradeDateAsOf 解析流水线交易日：显式传入则照用；为空时取本地日 K 最新交易日
// （周末/节假日手动运行时回退到最近有数据的交易日），本地无数据回退 now（Now 为零值取今天）。
func resolveTradeDateAsOf(tradeDate string, now time.Time) string {
	if tradeDate != "" {
		return tradeDate
	}
	if db.Dao != nil {
		if d := datasource.NewKLineStore().LatestMarketTradeDate(context.Background(), 45, 500); d != "" {
			return d
		}
	}
	if !now.IsZero() {
		return now.Format("2006-01-02")
	}
	return timeutil.TodayStr()
}

func resolveTradeDate(tradeDate string) string {
	return resolveTradeDateAsOf(tradeDate, time.Time{})
}

func hasBarOnDate(bars []models.KLineBar, tradeDate string) bool {
	for i := len(bars) - 1; i >= 0; i-- {
		if bars[i].TradeDate == tradeDate {
			return true
		}
		if bars[i].TradeDate < tradeDate {
			return false
		}
	}
	return false
}

// ErrPipelineRunning 表示已有流水线在跑。并发扫描会重复写库并触发 SQLITE_BUSY，
// 因此同一进程内只允许一个运行实例。
var ErrPipelineRunning = errors.New("狩猎场流水线正在运行，请等待当前运行结束")

var pipelineRunning atomic.Bool

// marketRisk 计算并返回当日沪深300风险档；数据不足时按保守默认档处理。
func marketRisk(ctx context.Context, tradeDate string, now time.Time) (risk.Level, float64) {
	lv := risk.LevelOf(0)
	var1d := 0.0
	if bars, err := datasource.NewKLineStore().QueryKLines(ctx, "sh000300", "day",
		now.AddDate(0, 0, -750).Format("2006-01-02"), tradeDate, false); err == nil && len(bars) > 100 {
		returns := make([]float64, 0, len(bars)-1)
		for i := 1; i < len(bars); i++ {
			if bars[i-1].Close > 0 {
				returns = append(returns, (bars[i].Close-bars[i-1].Close)/bars[i-1].Close)
			}
		}
		_, _, hybrid := risk.VaR(returns, 0.99)
		var1d = hybrid
		lv = risk.LevelOf(hybrid)
	}
	return lv, var1d
}

// snapshotAvailableDate 时点快照的可用日：常规当日运行即交易日；补跑历史交易日时
// 数据是取数当天才拿到的，按取数日归档，可用日绝不早于取数时刻，避免研究读到未来数据。
func snapshotAvailableDate(tradeDate string, now time.Time) string {
	fetched := now.Format("2006-01-02")
	if tradeDate > fetched {
		return tradeDate
	}
	return fetched
}

type stockBasic struct {
	Code string
	Name string
}

// loadStockList 全市场股票清单（A 股，当前时点的在市股票）
func loadStockList() ([]stockBasic, error) {
	var infos []models.AllStockInfo
	if err := db.Dao.Model(&models.AllStockInfo{}).
		Where("secucode LIKE ? OR secucode LIKE ?", "%.SH", "%.SZ").
		Find(&infos).Error; err != nil {
		return nil, fmt.Errorf("读取股票清单: %w", err)
	}
	stocks := make([]stockBasic, 0, len(infos))
	for _, info := range infos {
		code := stockcode.Normalize(info.SECURITYCODE)
		if code == "" {
			code = stockcode.Normalize(info.SECUCODE)
		}
		if code == "" {
			continue
		}
		stocks = append(stocks, stockBasic{Code: code, Name: info.SECURITYNAMEABBR})
	}
	return stocks, nil
}

type scanStat struct {
	InsufficientHistory int
	MissingTradeDate    int
	QueryFailed         int
}

// scanStrategySignals 5 策略并行扫描（纯本地 K 线，8 并发；停牌过滤：bars 末日 < tradeDate 跳过）。
// 实时/研究两模式共用，保证扫描规则一致。
func scanStrategySignals(ctx context.Context, tradeDate string, stocks []stockBasic) (
	signals []models.KhunterSignal, hits map[string][]scorer.Hit, lastBars map[string][]models.KLineBar, stat scanStat,
) {
	strategies := strategy.Registry()
	var mu sync.Mutex
	hits = map[string][]scorer.Hit{}
	lastBars = map[string][]models.KLineBar{}
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for _, st := range stocks {
		if strategy.IsInvalidName(st.Name) {
			continue
		}
		wg.Add(1)
		go func(code, name string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			bars, err := strategy.LoadDailyBarsAsOf(ctx, code, tradeDate, 30)
			if err != nil {
				mu.Lock()
				stat.QueryFailed++
				mu.Unlock()
				return
			}
			if len(bars) == 0 {
				mu.Lock()
				stat.InsufficientHistory++
				mu.Unlock()
				return
			}
			if !hasBarOnDate(bars, tradeDate) {
				mu.Lock()
				stat.MissingTradeDate++
				mu.Unlock()
				return // 停牌/本地 K 未同步到 tradeDate
			}
			mu.Lock()
			lastBars[code] = bars
			mu.Unlock()
			for _, s := range strategies {
				if len(bars) < s.MinBars() {
					continue
				}
				if sig := s.Select(bars, name, tradeDate); sig != nil {
					reasons, _ := json.Marshal(sig.Reasons)
					details, _ := json.Marshal(sig.Details)
					row := models.KhunterSignal{
						Code: code, Name: name, Strategy: sig.StrategyName,
						SignalDate: sig.Date, KeyDate: sig.KeyDate, KeyDateType: sig.KeyDateType,
						Close: sig.Close, VolumeRatio: sig.VolumeRatio,
						Reasons: string(reasons), Details: string(details),
					}
					mu.Lock()
					signals = append(signals, row)
					hits[code] = append(hits[code], scorer.Hit{Name: s.Name(), Weight: s.Weight(), Reasons: sig.Reasons})
					mu.Unlock()
				}
			}
		}(st.Code, st.Name)
	}
	wg.Wait()
	return signals, hits, lastBars, stat
}

// buildCandidates 候选池（按信号去重）
func buildCandidates(signals []models.KhunterSignal) (names map[string]string, candidates []string) {
	names = map[string]string{}
	for _, sig := range signals {
		if _, ok := names[sig.Code]; !ok {
			names[sig.Code] = sig.Name
			candidates = append(candidates, sig.Code)
		}
	}
	return names, candidates
}

// RunPipeline 每日流水线（实时模式兼容入口，spec 第 7 节 8 步）。tradeDate 为空取最近交易日。
func RunPipeline(ctx context.Context, tradeDate string) (*PipelineResult, error) {
	return RunPipelineWithOptions(ctx, PipelineOptions{TradeDate: tradeDate, Mode: RunModeLive})
}

// RunPipelineWithOptions 按模式运行流水线：实时模式先落时点快照再评分并写生产表；
// 研究模式只用 available_date <= tradeDate 的时点输入，结果只写 KhunterResearchRecord。
func RunPipelineWithOptions(ctx context.Context, opts PipelineOptions) (*PipelineResult, error) {
	if !pipelineRunning.CompareAndSwap(false, true) {
		return nil, ErrPipelineRunning
	}
	// 互斥标志必须在此释放
	defer pipelineRunning.Store(false)

	mode := opts.Mode
	if mode == "" {
		mode = RunModeLive
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	tradeDate := resolveTradeDateAsOf(opts.TradeDate, now)
	logger.SugaredLogger.Infof("khunter pipeline start: mode=%s tradeDate=%s (input=%q)", mode, tradeDate, opts.TradeDate)
	if err := EnsureMigrate(); err != nil {
		return nil, err
	}
	repo := NewRepo()
	if mode == RunModeResearch {
		return runResearchPipeline(ctx, repo, tradeDate, now)
	}
	return runLivePipeline(ctx, repo, tradeDate, now)
}

// tradeDateWindowStart 确认/退出引擎共用的交易日序列窗口起点。
// 默认 30 个日历日；有更旧的开放义务（最旧持仓入场日 / 最旧待确认观察信号日）时
// 向前扩到该义务日前 5 天，最多回看 400 个日历日兜底。窗口不得窄于默认 30 天，
// 否则老信号观察会因日期不在序列内被判 invalid、永远无法进入过期分支。
func tradeDateWindowStart(tradeDate string, holdings []models.KhunterHunting, pending []models.KhunterObservation) string {
	start := daysBefore(tradeDate, 30)
	oldestRelevant := ""
	for _, h := range holdings {
		if h.EnterDate != "" && (oldestRelevant == "" || h.EnterDate < oldestRelevant) {
			oldestRelevant = h.EnterDate
		}
	}
	for _, o := range pending {
		if o.SignalDate != "" && (oldestRelevant == "" || o.SignalDate < oldestRelevant) {
			oldestRelevant = o.SignalDate
		}
	}
	if oldestRelevant != "" {
		if earliest := daysBefore(oldestRelevant, 5); earliest < start {
			start = earliest
		}
	}
	if floor := daysBefore(tradeDate, 400); start < floor {
		start = floor
	}
	return start
}

// runLivePipeline 实时生产运行：写信号/评分/风险档/狩猎池/运行汇总。
// 评分前先把基本面和板块输入落时点快照，供历史研究按时点回读。
func runLivePipeline(ctx context.Context, repo *Repo, tradeDate string, now time.Time) (res *PipelineResult, err error) {
	started := time.Now()
	res = &PipelineResult{TradeDate: tradeDate}
	defer func() {
		duration := time.Since(started)
		status := "success"
		message := "运行完成"
		if res.Readiness != nil && !res.Readiness.Completed {
			// 同步未完成：观察已保存，确认整体跳过；诊断必须完整透出到运行汇总与事件
			message = "运行完成；确认跳过: " + RequireConfirmationReadiness(*res.Readiness).Error()
		}
		if err != nil {
			status = "failed"
			message = err.Error()
		}
		summary := &models.KhunterRunSummary{
			TradeDate:           tradeDate,
			StockTotal:          res.StockTotal,
			FreshStocks:         res.FreshStocks,
			InsufficientHistory: res.InsufficientHistory,
			MissingTradeDate:    res.MissingTradeDate,
			QueryFailed:         res.QueryFailed,
			SignalCount:         res.Signals,
			CandidateCount:      res.Candidates,
			ScoredCount:         res.Scored,
			HuntedCount:         res.Hunted,
			InvalidDataCount:    res.InvalidDataCount,
			IntradayCount:       res.IntradayCount,
			ConfirmedCount:      res.ConfirmedCount,
			ExpiredCount:        res.ExpiredCount,
			UntradeableCount:    res.UntradeableCount,
			RiskLevel:           res.RiskLevel,
			Status:              status,
			Message:             message,
			DurationMs:          duration.Milliseconds(),
		}
		if serr := repo.UpsertRunSummary(summary); serr != nil {
			logger.SugaredLogger.Errorf("khunter save run summary failed for %s: %v", tradeDate, serr)
		}
	}()

	// 1. 全市场股票清单（A 股）
	stocks, err := loadStockList()
	if err != nil {
		return nil, err
	}
	res.StockTotal = len(stocks)

	// 2. 5 策略并行扫描（纯本地 K 线，8 并发；停牌过滤：bars 末日 < tradeDate 跳过）
	signals, hits, lastBars, stat := scanStrategySignals(ctx, tradeDate, stocks)
	res.FreshStocks = len(lastBars)
	res.InsufficientHistory = stat.InsufficientHistory
	res.MissingTradeDate = stat.MissingTradeDate
	res.QueryFailed = stat.QueryFailed
	logger.SugaredLogger.Infof("khunter pipeline scan done: %d/%d 只本地K线在 %s 有 bar，历史不足 %d，当日无 bar %d，查询失败 %d，信号 %d 条",
		len(lastBars), len(stocks), tradeDate, stat.InsufficientHistory, stat.MissingTradeDate, stat.QueryFailed, len(signals))
	if len(lastBars) < 100 {
		logger.SugaredLogger.Warnf("khunter 所选日期本地日K覆盖较低：%s，当日有 bar %d/%d", tradeDate, len(lastBars), len(stocks))
	}
	if err := repo.ReplaceSignalsForDate(tradeDate, signals); err != nil {
		return res, err
	}
	res.Signals = len(signals)

	// 3. 候选池（去重）
	names, candidates := buildCandidates(signals)
	res.Candidates = len(candidates)

	// 风险档与候选无关：无信号也要持久化，避免运行记录缺当日市场状态。
	lv, var1d := marketRisk(ctx, tradeDate, now)
	if err := repo.SaveRiskLevel(&models.KhunterRiskLevel{
		Date: tradeDate, Var1d: var1d, Var5d: risk.MultiDay(var1d, 5),
		Level: lv.Name, PositionLimit: lv.PositionLimit, ScoreExtra: lv.ScoreExtra,
	}); err != nil {
		logger.SugaredLogger.Warnf("khunter 保存风险档失败 %s: %v", tradeDate, err)
	}
	res.RiskLevel = lv.Name

	scores := make([]models.KhunterScore, 0, len(candidates))
	if len(candidates) == 0 {
		// 重跑同日且本次无候选：清空旧评分，保证库内结果和本次扫描一致。
		if err := repo.ReplaceScoresForDate(tradeDate, nil); err != nil {
			return res, err
		}
	} else {
		// 4. 候选池数据预取（限流在 sync 函数内）
		if _, err := SyncMoneyFlow(ctx, candidates); err != nil {
			logger.SugaredLogger.Warnf("khunter 资金流同步中断: %v", err)
		}
		if _, err := SyncEvents(ctx, candidates); err != nil {
			logger.SugaredLogger.Warnf("khunter 事件同步中断: %v", err)
		}

		// 5. 先落时点快照再评分
		snapDate := snapshotAvailableDate(tradeDate, now)
		for _, code := range candidates {
			flows, _ := repo.GetMoneyFlowAsOf(code, tradeDate, 5)
			events, evErr := repo.GetActiveEvents(code, tradeDate)
			fund := FetchAndSnapshotFundamental(repo, code, snapDate)
			sectors := fetchAndSnapshotSectorInputs(repo, code, snapDate)
			in := scorer.FiveDimInput{
				Hits:      hits[code],
				MoneyFlow: flows,
				Finance:   fund,
				Sectors:   sectors,
				Events:    events,
				StockName: names[code],
			}
			r := scorer.Score(in)
			// 逐维解释落库：命中策略+理由、技术画像、缺数据标注，供前端"讲清楚"
			explain := r.Explain
			if explain == nil {
				explain = map[string]any{}
			}
			if tech, ok := explain["technical"].(map[string]any); ok {
				if bars := lastBars[code]; len(bars) > 0 {
					tech["profile"] = BuildTechProfile(bars)
				}
			}
			if evErr != nil {
				if ev, ok := explain["event"].(map[string]any); ok {
					ev["missing"] = true
					ev["reason"] = "事件查询失败: " + evErr.Error()
				}
			}
			details, _ := json.Marshal(explain)
			scores = append(scores, models.KhunterScore{
				Code: code, Name: names[code], ScoreDate: tradeDate,
				Technical: r.Technical, Moneyflow: r.Moneyflow, Fundamental: r.Fundamental,
				Sector: r.Sector, Event: r.Event, Total: r.Total, Level: r.Level,
				VetoReason: r.VetoReason, Degraded: r.Degraded, Details: string(details),
			})
		}
		if err := repo.ReplaceScoresForDate(tradeDate, scores); err != nil {
			return res, err
		}
		res.Scored = len(scores)
	}

	// 7. 观察保存：分数达阈值不再直接写入狩猎池，只落观察（真实入池由确认事务完成）。
	//    数据闸门结论随观察落库：盘中/降级/无效数据只观察不确认，不以中性分冒充买入结论。
	//    同步就绪结论在此一次计算（Task 11 替换 15:00 时点启发式）：它是数据闸门的
	//    syncCompleted 前提，也是第 8 步确认引擎的前置条件——证据只来自日K同步日志。
	//    信号日风险画像同步落库（SignalRiskJSON/RiskHistoryJSON）：风险过滤只输出否决原因，
	//    不修改形态与评分；信号日失败但后续恢复的候选仍可由确认引擎在拟入场日重算确认。
	readiness := ComputeDataReadiness(ctx, tradeDate, now)
	res.Readiness = &readiness
	synced := readiness.Completed
	threshold := HuntingThreshold(lv)
	riskCfg := LoadRiskControlConfig(resolveRiskControlConfigPath())
	observations := make([]models.KhunterObservation, 0, len(scores))
	for _, sc := range scores {
		if sc.Total < threshold {
			continue
		}
		bars := lastBars[sc.Code]
		if len(bars) == 0 {
			continue
		}
		support := SupportPrice(bars)
		latest := bars[len(bars)-1].Close
		if latest < support*0.98 {
			continue
		}
		decision := EvaluateDataGate(bars, tradeDate, synced)
		if !decision.AllowObservation {
			res.InvalidDataCount++
			logger.SugaredLogger.Warnf("khunter 候选 %s 数据闸门不放行观察: %s",
				sc.Code, strings.Join(decision.Reasons, "；"))
			continue
		}
		obs := BuildObservation(sc, bars, decision, tradeDate)
		if d := AttachSignalRisk(&obs, bars, decision, lv, riskCfg); !d.AllowConfirm {
			logger.SugaredLogger.Infof("khunter 候选 %s 信号日风险否决: %s",
				sc.Code, strings.Join(d.Reasons, "；"))
		}
		if obs.DataStatus == "intraday" {
			res.IntradayCount++
		}
		observations = append(observations, obs)
	}
	if err := repo.SaveObservations(observations); err != nil {
		return res, err
	}
	res.Observed = len(observations)

	// 8. Kronos 排序因子（异步旁路）：不参与五维评分、不改观察名单，
	//    只在后台批量预测并落库明细，供推荐视图排序与展开。
	//    结果按 (code, signal_date) 关联，不依赖 upsert 后的自增 ID。
	if kcfg := LoadKronosFactorConfig(""); kcfg.Enable && len(observations) > 0 {
		kcfg = AdjustedKronosWeight(context.Background(), kcfg)
		go func(rows []models.KhunterObservation, byCode map[string]models.KhunterObservation, kc KronosFactorConfig) {
			defer func() {
				if r := recover(); r != nil {
					logger.SugaredLogger.Errorf("khunter Kronos 因子 panic: %v", r)
				}
			}()
			frs, ferr := ApplyKronosFactor(context.Background(), rows, kc, data.KronosBatchPredict)
			if ferr != nil {
				logger.SugaredLogger.Warnf("khunter Kronos 因子批量预测失败: %v", ferr)
				return
			}
			recs := make([]models.KhunterKronosPrediction, 0, len(frs))
			degraded := 0
			for _, fr := range frs {
				if fr.DegradedReason != "" {
					degraded++
				}
				signalDate := tradeDate
				if o, ok := byCode[fr.Code]; ok && o.SignalDate != "" {
					signalDate = o.SignalDate
				}
				recs = append(recs, models.KhunterKronosPrediction{
					Code: fr.Code, SignalDate: signalDate, PredLen: kc.PredLen,
					ForecastJSON: fr.ForecastJSON, FactorScore: fr.FactorScore,
					Weight: fr.Weight, DegradedReason: fr.DegradedReason,
				})
			}
			if err := repo.SaveKronosPredictions(recs); err != nil {
				logger.SugaredLogger.Warnf("khunter Kronos 预测明细落库失败: %v", err)
				return
			}
			logger.SugaredLogger.Infof("khunter Kronos 因子完成 %s: %d 只，降级 %d 只", tradeDate, len(frs), degraded)
		}(append([]models.KhunterObservation(nil), observations...), buildObsByCode(observations), kcfg)
	}

	// 9. T+1~T+3 触发价确认引擎：对当日待确认观察重新评估数据闸门、风险画像、
	//    触发价与跳空约束，确认成功后生成真实不复权交易。
	pending, err := repo.GetPendingObservations(tradeDate)
	if err != nil {
		logger.SugaredLogger.Warnf("khunter 待确认观察读取失败: %v", err)
		return res, nil
	}
	holdings, err := repo.GetHuntingList(HuntingStatusHolding)
	if err != nil {
		logger.SugaredLogger.Warnf("khunter 持仓列表读取失败: %v", err)
		return res, nil
	}
	if len(pending) > 0 || len(holdings) > 0 {
		logger.SugaredLogger.Infof("khunter %s 待确认观察 %d 条，持仓中交易 %d 笔",
			tradeDate, len(pending), len(holdings))
	}

	var tradeDates []string
	if len(pending) > 0 || len(holdings) > 0 {
		// 覆盖最久未了的义务（最旧持仓入场日 / 最旧待确认观察信号日）：
		// 固定 30 天窗口会截断持有超过约 20 个交易日的持仓（连续一字跌停无法退出、
		// 无法到期）与老信号观察（窗口状态判 invalid、永不进入过期分支）。
		// 只取一次，确认与退出引擎共用。
		tradeDates, err = repo.GetTradeDates(ctx, tradeDateWindowStart(tradeDate, holdings, pending), tradeDate)
		if err != nil {
			logger.SugaredLogger.Warnf("khunter 交易日序列读取失败 %s: %v", tradeDate, err)
		}
	}

	if len(pending) > 0 && len(tradeDates) > 0 {
		if confirmErr := RequireConfirmationReadiness(readiness); confirmErr != nil {
			// 日K同步未完成：盘中观察已保存（observe-only），确认不得运行；
			// 诊断已随 res.Readiness 与运行汇总透出，待同步完成后下一触发再确认。
			logger.SugaredLogger.Warnf("khunter %s 确认跳过: %v", tradeDate, confirmErr)
		} else {
			engine := NewConfirmationEngine(repo)
			stats, err := engine.ConfirmForDate(ctx, tradeDate, tradeDates, lv, riskCfg, riskCfg.InitialCash)
			if err != nil {
				logger.SugaredLogger.Warnf("khunter 确认引擎执行失败 %s: %v", tradeDate, err)
			} else {
				res.Hunted += stats.Confirmed
				res.ConfirmedCount += stats.Confirmed
				res.ExpiredCount += stats.Expired
				res.UntradeableCount += stats.Untradeable
				res.InvalidDataCount += stats.InvalidData
			}
		}
	}

	// 9. 退出引擎：对持仓中交易按真实 K 线评估止损、突破失败、1R 锁盈与到期退出。
	if len(holdings) > 0 && len(tradeDates) > 0 {
		ProcessHoldingsForDate(ctx, repo, tradeDate, tradeDates, riskCfg, holdings)
	}
	return res, nil
}
