package khunter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

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
	Hunted              int
	Removed             int
	RiskLevel           string
	TradeDate           string
	StockTotal          int `json:"stockTotal"`
	FreshStocks         int `json:"freshStocks"`
	InsufficientHistory int `json:"insufficientHistory"`
	MissingTradeDate    int `json:"missingTradeDate"`
	QueryFailed         int `json:"queryFailed"`
}

// resolveTradeDate 解析流水线交易日：显式传入则照用；为空时取本地日 K 最新交易日
// （周末/节假日手动运行时回退到最近有数据的交易日），本地无数据回退今天。
func resolveTradeDate(tradeDate string) string {
	if tradeDate != "" {
		return tradeDate
	}
	if db.Dao != nil {
		if d := datasource.NewKLineStore().LatestMarketTradeDate(context.Background(), 45, 500); d != "" {
			return d
		}
	}
	return timeutil.TodayStr()
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
func marketRisk(ctx context.Context, tradeDate string) (risk.Level, float64) {
	lv := risk.LevelOf(0)
	var1d := 0.0
	if bars, err := datasource.NewKLineStore().QueryKLines(ctx, "sh000300", "day",
		time.Now().AddDate(0, 0, -750).Format("2006-01-02"), tradeDate, false); err == nil && len(bars) > 100 {
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

// RunPipeline 每日流水线（spec 第 7 节 8 步）。tradeDate 为空取最近交易日。
func RunPipeline(ctx context.Context, tradeDate string) (res *PipelineResult, err error) {
	if !pipelineRunning.CompareAndSwap(false, true) {
		return nil, ErrPipelineRunning
	}
	// 互斥标志必须在此释放：后面的 defer 依赖 res，而 res 要到迁移成功后才赋值。
	defer pipelineRunning.Store(false)

	started := time.Now()
	input := tradeDate
	tradeDate = resolveTradeDate(tradeDate)
	logger.SugaredLogger.Infof("khunter pipeline start: tradeDate=%s (input=%q)", tradeDate, input)
	if err := EnsureMigrate(); err != nil {
		return nil, err
	}
	repo := NewRepo()
	res = &PipelineResult{TradeDate: tradeDate}
	defer func() {
		duration := time.Since(started)
		status := "success"
		message := "运行完成"
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
	var infos []models.AllStockInfo
	if err := db.Dao.Model(&models.AllStockInfo{}).
		Where("secucode LIKE ? OR secucode LIKE ?", "%.SH", "%.SZ").
		Find(&infos).Error; err != nil {
		return nil, fmt.Errorf("读取股票清单: %w", err)
	}
	type stockBasic struct {
		Code string
		Name string
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
	res.StockTotal = len(stocks)

	// 2. 5 策略并行扫描（纯本地 K 线，8 并发；停牌过滤：bars 末日 < tradeDate 跳过）
	strategies := strategy.Registry()
	var mu sync.Mutex
	var signals []models.KhunterSignal
	hits := map[string][]scorer.Hit{}
	lastBars := map[string][]models.KLineBar{}
	sem := make(chan struct{}, 8)
	insufficientHistory := 0
	missingTradeDate := 0
	queryFailed := 0
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
			bars, err := strategy.LoadDailyBars(ctx, code, 30)
			if err != nil {
				mu.Lock()
				queryFailed++
				mu.Unlock()
				return
			}
			if len(bars) == 0 {
				mu.Lock()
				insufficientHistory++
				mu.Unlock()
				return
			}
			if !hasBarOnDate(bars, tradeDate) {
				mu.Lock()
				missingTradeDate++
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
	res.FreshStocks = len(lastBars)
	res.InsufficientHistory = insufficientHistory
	res.MissingTradeDate = missingTradeDate
	res.QueryFailed = queryFailed
	logger.SugaredLogger.Infof("khunter pipeline scan done: %d/%d 只本地K线在 %s 有 bar，历史不足 %d，当日无 bar %d，查询失败 %d，信号 %d 条",
		len(lastBars), len(stocks), tradeDate, insufficientHistory, missingTradeDate, queryFailed, len(signals))
	if len(lastBars) < 100 {
		logger.SugaredLogger.Warnf("khunter 所选日期本地日K覆盖较低：%s，当日有 bar %d/%d", tradeDate, len(lastBars), len(stocks))
	}
	if err := repo.ReplaceSignalsForDate(tradeDate, signals); err != nil {
		return res, err
	}
	res.Signals = len(signals)

	// 3. 候选池（去重）
	names := map[string]string{}
	var candidates []string
	for _, sig := range signals {
		if _, ok := names[sig.Code]; !ok {
			names[sig.Code] = sig.Name
			candidates = append(candidates, sig.Code)
		}
	}
	res.Candidates = len(candidates)

	// 风险档与候选无关：无信号也要持久化，避免运行记录缺当日市场状态。
	lv, var1d := marketRisk(ctx, tradeDate)
	if err := repo.SaveRiskLevel(&models.KhunterRiskLevel{
		Date: tradeDate, Var1d: var1d, Var5d: risk.MultiDay(var1d, 5),
		Level: lv.Name, PositionLimit: lv.PositionLimit, ScoreExtra: lv.ScoreExtra,
	}); err != nil {
		logger.SugaredLogger.Warnf("khunter 保存风险档失败 %s: %v", tradeDate, err)
	}
	res.RiskLevel = lv.Name

	if len(candidates) == 0 {
		// 重跑同日且本次无候选：清空旧评分，保证库内结果和本次扫描一致。
		if err := repo.ReplaceScoresForDate(tradeDate, nil); err != nil {
			return res, err
		}
		return res, nil
	}

	// 4. 候选池数据预取（限流在 sync 函数内）
	if _, err := SyncMoneyFlow(ctx, candidates); err != nil {
		logger.SugaredLogger.Warnf("khunter 资金流同步中断: %v", err)
	}
	if _, err := SyncEvents(ctx, candidates); err != nil {
		logger.SugaredLogger.Warnf("khunter 事件同步中断: %v", err)
	}

	// 5. 五维评分
	var scores []models.KhunterScore
	for _, code := range candidates {
		flows, _ := repo.GetMoneyFlow(code, 5)
		events, evErr := repo.GetActiveEvents(code, tradeDate)
		in := scorer.FiveDimInput{
			Hits:      hits[code],
			MoneyFlow: flows,
			Finance:   FetchFundamentalRetry(code, 1),
			Sectors:   fetchSectorInputs(code),
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

	// 7. 狩猎场筛选：总分 ≥ 阈值 且 现价 ≥ 支撑位×0.98
	threshold := HuntingThreshold(lv)
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
		h := models.KhunterHunting{
			Code: sc.Code, Name: names[sc.Code], EnterDate: tradeDate,
			EnterScore: sc.Total, SupportPrice: support, Status: "追踪中",
		}
		if err := repo.SaveHunting(&h); err == nil {
			res.Hunted++
		}
	}

	// 8. 狩猎场追踪：追踪中标的每日 TrackDays+1；最新收盘跌破支撑位（无 ×0.98 缓冲）→ 已移除
	tracked, err := repo.GetHuntingList("追踪中")
	if err != nil {
		logger.SugaredLogger.Warnf("khunter 追踪列表读取失败: %v", err)
		return res, nil
	}
	for _, h := range tracked {
		if h.EnterDate == tradeDate {
			continue // 当日新入选不计天数
		}
		bars := lastBars[h.Code]
		if len(bars) == 0 {
			if b, err := strategy.LoadDailyBars(ctx, h.Code, 30); err == nil {
				bars = b
			}
		}
		if len(bars) == 0 || !hasBarOnDate(bars, tradeDate) {
			continue // 停牌/无当日行情，无法判定破位也不计天数
		}
		if bars[len(bars)-1].Close < h.SupportPrice {
			if err := repo.UpdateHuntingStatus(h.ID, "已移除"); err == nil {
				res.Removed++
			}
			continue
		}
		if err := repo.IncrTrackDays(h.ID); err != nil {
			logger.SugaredLogger.Warnf("khunter 追踪天数更新失败 %s: %v", h.Code, err)
		}
	}
	return res, nil
}
