package datasource

// K 线增量读取与除权防线：
//   - GetKLineIncremental：图表读取入口，store-first；库内新鲜且够量直接返回，
//     否则只走一次小 count 的尾部补拉（替代旧的每次全量 limit 拉取）。
//   - DetectAdjustedSeriesShift：前复权序列整体平移（除权）检测。增量追加前必须
//     比对重叠日收盘价，发现平移立即全量重拉覆盖，否则 qfq 历史会被写坏。
//   - RefreshKLineFull：绕过缺口检测的全量重拉（同步落库覆盖）。

import (
	"context"
	"fmt"
	"math"
	"time"

	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/stockcode"
	"go-stock/backend/util/timeutil"
)

// maxKLineFetchCount 单次拉取上限，与 handler 侧分页上限保持一致。
const maxKLineFetchCount = 5000

// adjustedShiftTolerance 前复权重定价容差：重叠日收盘价相对偏差超过 0.5% 视为
// 除权导致 qfq 序列整体平移。正常增量补拉与库内数据同源同权，偏差应为 0。
const adjustedShiftTolerance = 0.005

// GetKLineIncremental 图表用日线读取：store-first + 尾部缺口增量补拉。
// 返回值 Source 标记数据来源：store / store(stale) / <provider> / <provider>(refreshed)。
// 仅对日线生效；其他周期直接走 GetKLine 原路径（分钟线不落库，无增量价值）。
func (r *Router) GetKLineIncremental(ctx context.Context, code, period string, limit int) (*KLineData, error) {
	if limit <= 0 {
		limit = 500
	}
	normalized := stockcode.Normalize(code)
	canonical := CanonicalKLinePeriod(period)
	if canonical != "day" || globalKLineStore == nil {
		return r.GetKLine(ctx, code, period, limit)
	}

	tomorrow := timeutil.DateStr(time.Now().AddDate(0, 0, 1))
	stored, err := globalKLineStore.QueryKLines(ctx, normalized, canonical, "1990-01-01", tomorrow, true)
	if err != nil {
		logger.SugaredLogger.Warnf("kline incremental: 查询本地库失败 stock=%s err=%v，回退全量拉取", normalized, err)
		return r.GetKLine(ctx, code, period, limit)
	}
	if len(stored) == 0 {
		// 首次访问：全量拉取并落库（GetKLine 内部异步持久化），后续请求即命中增量路径。
		return r.GetKLine(ctx, code, period, limit)
	}

	last := stored[len(stored)-1].TradeDate
	expected := expectedLatestTradeDate(time.Now())
	if last >= expected && len(stored) >= limit {
		logger.SugaredLogger.Debugf("kline incremental: 本地命中 stock=%s bars=%d last=%s（零网络请求）", normalized, len(stored), last)
		return storedBarsToKLineData(normalized, canonical, "store", tailOf(stored, limit)), nil
	}

	tailCount := estimateTailCount(last, limit)
	k, ferr := r.getKLineNoPersist(ctx, normalized, canonical, tailCount)
	if ferr != nil {
		// 网络失败：库内够量就返回旧数据（明确标记 stale），不够量才把错误抛给上层 fallback 链。
		if len(stored) >= limit {
			logger.SugaredLogger.Warnf("kline incremental: 尾部补拉失败 stock=%s err=%v，返回本地旧数据", normalized, ferr)
			return storedBarsToKLineData(normalized, canonical, "store(stale)", tailOf(stored, limit)), nil
		}
		return nil, ferr
	}

	incoming := BarsFromKLineData(normalized, canonical, k.Source, true, k)
	if DetectAdjustedSeriesShift(stored, incoming, 8) {
		logger.SugaredLogger.Warnf("kline incremental: 检测到前复权序列平移（除权）stock=%s last=%s，触发全量重拉覆盖", normalized, last)
		if full, rerr := r.RefreshKLineFull(ctx, normalized, canonical, tailCount); rerr == nil {
			bars := BarsFromKLineData(normalized, canonical, full.Source, true, full)
			if len(bars) > 0 {
				// RefreshKLineFull 内部已同步落库覆盖
				return storedBarsToKLineData(normalized, canonical, full.Source+"(refreshed)", bars), nil
			}
		} else {
			logger.SugaredLogger.Errorf("kline incremental: 除权全量重拉失败 stock=%s err=%v，回退尾部追加", normalized, rerr)
		}
	}

	if len(incoming) > 0 {
		// getKLineNoPersist 跳过了异步落库，这里同步落库，保证返回时读取与库一致。
		if uerr := globalKLineStore.UpsertKLines(ctx, incoming); uerr != nil {
			logger.SugaredLogger.Warnf("kline incremental: 尾部落库失败 stock=%s err=%v", normalized, uerr)
		}
	}
	merged := mergeStoredAndIncoming(stored, incoming, limit)
	return storedBarsToKLineData(normalized, canonical, k.Source, merged), nil
}

// FetchKLineNoPersist 导出包装：标准化代码与周期后走 fallback 链取数，
// 不触发异步落库，落库时机由调用方控制。
func (r *Router) FetchKLineNoPersist(ctx context.Context, code, period string, count int) (*KLineData, error) {
	return r.getKLineNoPersist(ctx, stockcode.Normalize(code), CanonicalKLinePeriod(period), count)
}

// EndAnchoredKLineProvider 支持按结束日期锚定取数的 K 线数据源。
// 历史缺口回补必需：缺口检测给出的区间在历史中段，"最新 N 根"语义的取数
// 永远覆盖不到，导致补齐任务假完成（生产实测 3892 个 done 任务仅落 55/1310 根）。
type EndAnchoredKLineProvider interface {
	KLineProvider
	// GetKLineBefore 取 endDate（含）之前（不含当日及以后）的 count 根 K 线。
	// endDate 格式 YYYY-MM-DD。
	GetKLineBefore(ctx context.Context, code, period string, count int, endDate string) (*KLineData, error)
}

// FetchKLineNoPersistBefore 锚定结束日期取数（不含 endDate 当日及以后）。
// 优先走支持锚定的 provider；全部不支持时退化为"取到今天"的窗口拉取，
// count 按缺口到今天的自然日跨度放大（只对近期缺口有效，远期缺口要求 provider 锚定）。
func (r *Router) FetchKLineNoPersistBefore(ctx context.Context, code, period string, count int, endDate string) (*KLineData, error) {
	normalized := stockcode.Normalize(code)
	canonical := CanonicalKLinePeriod(period)

	r.mu.RLock()
	provs := append([]KLineProvider(nil), r.klineProviders...)
	r.mu.RUnlock()

	var lastErr error
	for _, p := range provs {
		ap, ok := p.(EndAnchoredKLineProvider)
		if !ok {
			continue
		}
		k, err := ap.GetKLineBefore(ctx, normalized, canonical, count, endDate)
		if err == nil && k != nil && len(k.Bars) > 0 {
			return k, nil
		}
		if err != nil {
			lastErr = err
			logger.SugaredLogger.Warnf("kline anchored fetch via %s failed for %s before %s: %v", p.Name(), normalized, endDate, err)
		}
	}

	// 退化路径：窗口拉到今天，覆盖近期缺口
	if endT, e := timeutil.ParseDate(endDate); e == nil {
		days := int(time.Since(endT).Hours()/24) + 1
		count += days*5/7 + 10
		if count > maxKLineFetchCount {
			count = maxKLineFetchCount
		}
	}
	k, err := r.getKLineNoPersist(ctx, normalized, canonical, count)
	if err != nil && lastErr != nil {
		return nil, fmt.Errorf("anchored providers failed (%v) and tail fetch failed: %w", lastErr, err)
	}
	return k, err
}

// getKLineNoPersist 与 GetKLine 相同的 fallback 链，但不触发异步落库。
// 增量路径用它取尾部数据，落库时机由调用方控制（先做除权检测再决定怎么写）。
func (r *Router) getKLineNoPersist(ctx context.Context, code, period string, count int) (*KLineData, error) {
	key := CacheKey(DataTypeKLine, code, period, fmt.Sprintf("%d", count))
	v, err, _ := r.flights.Do(key, func() (interface{}, error) {
		return r.getKLineInner(ctx, code, period, count, key, false)
	})
	if err != nil {
		return nil, err
	}
	return v.(*KLineData), nil
}

// RefreshKLineFull 绕过缺口检测的全量重拉并同步落库覆盖。
// count = 库内已有条数 + extra + 缓冲，钳位 [200, 5000]。
// 用于前复权序列整体平移（除权）后的历史覆盖：缺口检测认为日期都已存在不会发请求，
// 必须直接全量取数，靠 UpsertKLines 的 OnConflict UpdateAll 覆盖旧值。
func (r *Router) RefreshKLineFull(ctx context.Context, code, period string, extra int) (*KLineData, error) {
	normalized := stockcode.Normalize(code)
	canonical := CanonicalKLinePeriod(period)

	storedCount := 0
	if globalKLineStore != nil {
		storedCount = globalKLineStore.countStoredAdjusted(ctx, normalized, canonical)
	}
	count := storedCount + extra + 50
	if count < 200 {
		count = 200
	}
	if count > maxKLineFetchCount {
		count = maxKLineFetchCount
	}

	k, err := r.getKLineNoPersist(ctx, normalized, canonical, count)
	if err != nil {
		return nil, err
	}
	if globalKLineStore != nil {
		bars := BarsFromKLineData(normalized, canonical, k.Source, true, k)
		if len(bars) > 0 {
			if uerr := globalKLineStore.UpsertKLines(ctx, bars); uerr != nil {
				return k, fmt.Errorf("refresh full upsert failed: %w", uerr)
			}
			logger.SugaredLogger.Infof("kline refresh: 除权全量覆盖完成 stock=%s bars=%d count=%d", normalized, len(bars), count)
		}
	}
	return k, nil
}

// countStoredAdjusted 统计库内 (code, period, adjusted=true) 条数，用于估算全量重拉的 count。
func (s *KLineStore) countStoredAdjusted(ctx context.Context, code, period string) int {
	tomorrow := timeutil.DateStr(time.Now().AddDate(0, 0, 1))
	bars, err := s.QueryKLines(ctx, code, period, "1990-01-01", tomorrow, true)
	if err != nil {
		return 0
	}
	return len(bars)
}

// DetectAdjustedSeriesShift 检测新拉取 bars 与库内存储 bars 在重叠交易日的收盘价
// 是否系统性错位（除权导致前复权序列整体平移）。
// 只比对最近 overlapLimit 个重叠日：重叠在序列尾部，足够代表整体平移。
// 纯函数：stored 由调用方在拉取前快照，避免与异步落库竞态。
func DetectAdjustedSeriesShift(stored, incoming []models.KLineBar, overlapLimit int) bool {
	if overlapLimit <= 0 {
		overlapLimit = 8
	}
	if len(stored) == 0 || len(incoming) == 0 {
		return false
	}
	inByDate := make(map[string]float64, len(incoming))
	for _, b := range incoming {
		if b.Close > 0 {
			inByDate[b.TradeDate] = b.Close
		}
	}
	checked := 0
	for i := len(stored) - 1; i >= 0 && checked < overlapLimit; i-- {
		s := stored[i]
		c, ok := inByDate[s.TradeDate]
		if !ok || s.Close <= 0 {
			continue
		}
		checked++
		if math.Abs(c-s.Close)/s.Close > adjustedShiftTolerance {
			return true
		}
	}
	return false
}

// expectedLatestTradeDate 按当前时间推算本地应有的最新交易日：
// 当日 09:15 起盘中/收盘均应为今天（盘中是持续变动的当日 bar，靠尾部补拉刷新）；
// 未开盘（<09:15）与周末回退到上一个工作日。节假日盘中多一次空补拉，无副作用。
func expectedLatestTradeDate(now time.Time) string {
	t := now
	if h, m, _ := t.Clock(); h < 9 || (h == 9 && m < 15) {
		t = t.AddDate(0, 0, -1)
	}
	for t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		t = t.AddDate(0, 0, -1)
	}
	return timeutil.DateStr(t)
}

// estimateTailCount 估算从 last 到今天需要补拉的 bar 数：
// 日历天数 × 5/7（交易日近似）+ 缓冲，钳位 [10, limit]。
func estimateTailCount(last string, limit int) int {
	count := 10
	if lt, err := timeutil.ParseDate(last); err == nil {
		days := int(time.Since(lt).Hours()/24) + 1
		if days < 1 {
			days = 1
		}
		count = days*5/7 + 8
	}
	if count > limit {
		count = limit
	}
	if count > maxKLineFetchCount {
		count = maxKLineFetchCount
	}
	return count
}

// mergeStoredAndIncoming 合并库内历史与尾部新数据：与 incoming 重叠/相邻区间的旧值
// 一律以 incoming 为准（数据源的最新快照），两端之外的历史原样保留，最后截取 limit。
func mergeStoredAndIncoming(stored, incoming []models.KLineBar, limit int) []models.KLineBar {
	if len(incoming) == 0 {
		return tailOf(stored, limit)
	}
	first, last := incoming[0].TradeDate, incoming[len(incoming)-1].TradeDate
	merged := make([]models.KLineBar, 0, len(stored)+len(incoming))
	for _, b := range stored {
		if b.TradeDate < first || b.TradeDate > last {
			merged = append(merged, b)
		}
	}
	merged = append(merged, incoming...)
	return tailOf(merged, limit)
}

func tailOf(bars []models.KLineBar, limit int) []models.KLineBar {
	if len(bars) <= limit {
		return bars
	}
	return bars[len(bars)-limit:]
}

// storedBarsToKLineData 库内 bar → 前端 wire 格式（port 层 KLineData）。
func storedBarsToKLineData(code, period, source string, bars []models.KLineBar) *KLineData {
	out := make([]KLineBar, 0, len(bars))
	for _, b := range bars {
		t, err := timeutil.ParseDate(b.TradeDate)
		if err != nil {
			continue
		}
		out = append(out, KLineBar{
			Time:      t,
			Open:      b.Open,
			High:      b.High,
			Low:       b.Low,
			Close:     b.Close,
			PrevClose: b.PrevClose,
			Volume:    b.Volume,
			Amount:    b.Amount,
		})
	}
	return &KLineData{
		Code:   code,
		Period: period,
		Bars:   out,
		Source: source,
	}
}
