package data

// signal_wire.go 信号引擎与业务层接线：
//   - 资金流 Provider（东财 F62 主力净额，近5日合计）
//   - 单股最新/历史时点信号判定（持仓徽章、时机快照、工作台本地验证共用）
//   - 持仓批量判定（并发 + 当日缓存）
//   - 逃顶预警（命中即系统通知，同股同信号同日去重）

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"go-stock/backend/data/datasource"
	"go-stock/backend/data/signal"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/util/timeutil"
)

// sharedSignalEngine 全局共享引擎（资金流 Provider 已注入）
var sharedSignalEngine = &signal.Engine{Funds: signalFundsProvider}

// signalFundsProvider 资金流 Provider：近5日主力净额合计（元）。
func signalFundsProvider(code string) (float64, bool) {
	defer func() { _ = recover() }() // 数据源 panic 不应击垮判定
	rows := NewStockDataApi().GetStockHistoryMoneyData(code)
	if len(rows) == 0 {
		return 0, false
	}
	return sumMainNetFlow(rows, 5), true
}

// EvaluateStockSignals 判定单只股票最新时点的全部命中信号。
// 失败/数据不足返回空（降级语义，不报错）。
func EvaluateStockSignals(ctx context.Context, stockCode string) []signal.SignalMatch {
	bars := fetchSignalBars(ctx, stockCode, 120)
	if len(bars) == 0 {
		return nil
	}
	return sharedSignalEngine.EvaluateLatest(stockCode, bars)
}

// EvaluateStockSignalsAsOf 判定历史时点（tradeDate 当日收盘后）的信号——买入时机快照用。
// tradeDate 为 "2006-01-02"；找不到对应当日或之前的K线时返回空。
func EvaluateStockSignalsAsOf(ctx context.Context, stockCode, tradeDate string) []signal.SignalMatch {
	bars := fetchSignalBars(ctx, stockCode, 500)
	if len(bars) == 0 {
		return nil
	}
	// 找最后一个交易日 <= tradeDate 的下标
	asOf := -1
	for i, b := range bars {
		day := b.Time.In(time.Local).Format(timeutil.DateLayout)
		if day <= tradeDate {
			asOf = i
		} else {
			break
		}
	}
	if asOf < 0 {
		return nil
	}
	return sharedSignalEngine.Evaluate(stockCode, bars, asOf)
}

// fetchSignalBars 拉取日K线（复用数据源路由），失败返回 nil。
func fetchSignalBars(ctx context.Context, stockCode string, count int) []datasource.KLineBar {
	apiCode := normalizeTradingRecordAPI(stockCode)
	kl, err := datasource.GetRouter().GetKLine(ctx, apiCode, "101", count)
	if err != nil || kl == nil || len(kl.Bars) == 0 {
		return nil
	}
	return kl.Bars
}

// ---------------------------------------------------------------------------
// 持仓批量判定（徽章）——并发 + 当日缓存
// ---------------------------------------------------------------------------

type holdingsSignalsCacheT struct {
	mu   sync.Mutex
	date string
	data map[string][]signal.SignalMatch
}

var holdingsSignalsCache = &holdingsSignalsCacheT{}

// GetHoldingsSignals 持仓信号徽章数据：stockCode → 命中信号列表。
// 并发判定（上限4），单股失败不影响其他；当日缓存避免重复拉K线。
func GetHoldingsSignals(ctx context.Context, positions []HoldingsPosition) map[string][]signal.SignalMatch {
	today := timeutil.TodayStr()

	holdingsSignalsCache.mu.Lock()
	if holdingsSignalsCache.date != today {
		holdingsSignalsCache.date = today
		holdingsSignalsCache.data = map[string][]signal.SignalMatch{}
	}
	result := make(map[string][]signal.SignalMatch, len(positions))
	todo := make([]string, 0, len(positions))
	seen := map[string]bool{}
	for _, p := range positions {
		code := p.StockCode
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		if cached, ok := holdingsSignalsCache.data[code]; ok {
			result[code] = cached
		} else {
			todo = append(todo, code)
		}
	}
	holdingsSignalsCache.mu.Unlock()

	if len(todo) > 0 {
		var wg sync.WaitGroup
		sem := make(chan struct{}, 4)
		var mu sync.Mutex
		for _, code := range todo {
			wg.Add(1)
			go func(code string) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						logger.SugaredLogger.Warnf("持仓信号判定 panic %s: %v", code, r)
					}
				}()
				sem <- struct{}{}
				matches := EvaluateStockSignals(ctx, code)
				<-sem
				mu.Lock()
				result[code] = matches
				holdingsSignalsCache.mu.Lock()
				holdingsSignalsCache.data[code] = matches
				holdingsSignalsCache.mu.Unlock()
				mu.Unlock()
			}(code)
		}
		wg.Wait()
	}
	return result
}

// EvaluateStocksSignals 批量判定任意股票列表的最新信号（选股工作台「本地验证」列用）。
// 复用持仓批量判定的并发与当日缓存。
func EvaluateStocksSignals(ctx context.Context, codes []string) map[string][]signal.SignalMatch {
	positions := make([]HoldingsPosition, 0, len(codes))
	for _, c := range codes {
		if c = strings.TrimSpace(c); c != "" {
			positions = append(positions, HoldingsPosition{StockCode: c})
		}
	}
	return GetHoldingsSignals(ctx, positions)
}

// ---------------------------------------------------------------------------
// 逃顶预警——命中即系统通知，同股同信号同日去重
// ---------------------------------------------------------------------------

// signalAlertLog 预警去重记录
type signalAlertLog struct {
	ID        uint      `gorm:"primaryKey"`
	AlertDate string    `gorm:"uniqueIndex:uk_signal_alert;size:10"`
	StockCode string    `gorm:"uniqueIndex:uk_signal_alert;size:20"`
	SignalKey string    `gorm:"uniqueIndex:uk_signal_alert;size:40"`
	CreatedAt time.Time
}

func (signalAlertLog) TableName() string { return "signal_alert_logs" }

func ensureSignalAlertMigrate() {
	if err := db.Dao.AutoMigrate(&signalAlertLog{}); err != nil {
		logger.SugaredLogger.Warnf("signal_alert_logs AutoMigrate 失败: %v", err)
	}
}

// CheckHoldingsExitAlerts 检查持仓逃顶类信号并推送系统通知（同日同股同信号仅一次）。
// signalsByCode 通常来自 GetHoldingsSignals 的结果；names 用于通知文案。
func CheckHoldingsExitAlerts(signalsByCode map[string][]signal.SignalMatch, names map[string]string) {
	ensureSignalAlertMigrate()
	today := timeutil.TodayStr()
	for code, matches := range signalsByCode {
		for _, m := range matches {
			if !signal.IsExitAlert(m.Key) {
				continue
			}
			rec := signalAlertLog{AlertDate: today, StockCode: code, SignalKey: m.Key}
			res := db.Dao.Where("alert_date = ? AND stock_code = ? AND signal_key = ?",
				today, code, m.Key).FirstOrCreate(&rec)
			if res.Error != nil || res.RowsAffected == 0 {
				continue // 今日已报过（已存在记录）或写入失败
			}
			name := names[code]
			if name == "" {
				name = code
			}
			logger.SugaredLogger.Infof("持仓逃顶预警: %s(%s) 命中 %s", name, code, m.Name)
			go NewAlertWindowsApi("go-stock",
				"持仓信号预警",
				name+" 出现「"+m.Name+"」："+m.Tip, "").SendNotification()
		}
	}
}

// ---------------------------------------------------------------------------
// 买入时机快照
// ---------------------------------------------------------------------------

// snapshotBuySignals 判定买入时点的信号并序列化为 JSON（落库 TradingRecord.SignalSnapshot）。
// 失败返回空字符串（快照是增强信息，缺失不影响交易记录保存）。
func snapshotBuySignals(ctx context.Context, stockCode string, tradingTime time.Time) string {
	tradeDate := tradingTime.In(time.Local).Format(timeutil.DateLayout)
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	matches := EvaluateStockSignalsAsOf(cctx, stockCode, tradeDate)
	if len(matches) == 0 {
		return ""
	}
	bs, err := json.Marshal(matches)
	if err != nil {
		return ""
	}
	return string(bs)
}
