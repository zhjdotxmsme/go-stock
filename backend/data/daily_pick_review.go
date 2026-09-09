package data

import (
	"context"
	"math"
	"time"

	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// DailyPickReview handles the next-day review of previous picks.
type DailyPickReview struct {
	engine *DailyPickEngine
	repo   *DailyPickRepository
}

// NewDailyPickReview creates a new review instance.
func NewDailyPickReview() *DailyPickReview {
	return &DailyPickReview{
		engine: NewDailyPickEngine(),
		repo:   NewDailyPickRepository(),
	}
}

// RunDailyReview performs review for picks from the previous trading day.
// reviewDate is the date whose closing data to use for review (e.g. T+1).
// If pickDate is empty, it reviews the most recent unreviewed date.
func (r *DailyPickReview) RunDailyReview(ctx context.Context, reviewDate string, pickDate string) int {
	var picks []models.DailyPick

	if pickDate != "" {
		picks = r.repo.FindUnreviewedByDate(ctx, pickDate)
	} else {
		// Find the most recent unreviewed date
		latest, err := r.repo.EarliestUnreviewed(ctx)
		if err != nil {
			logger.SugaredLogger.Info("daily_review: no unreviewed picks found")
			return 0
		}
		picks = r.repo.FindUnreviewedByDate(ctx, latest.TradeDate)
	}

	if len(picks) == 0 {
		logger.SugaredLogger.Info("daily_review: no picks to review")
		return 0
	}

	logger.SugaredLogger.Infof("daily_review: reviewing %d picks with %s data", len(picks), reviewDate)

	reviewed := 0
	for i := range picks {
		ok := r.reviewOne(ctx, &picks[i], reviewDate)
		if ok {
			reviewed++
		}
	}

	// 多窗口回填：已复盘但 3/5 日窗口尚未回填的近期 picks（到期后重复运行自动补齐）。
	r.backfillWindows(ctx)

	return reviewed
}

// reviewKlineBars 复盘取数根数：需覆盖 TradeDate 及其后 5 个交易日。
const reviewKlineBars = 20

// findBarAfter 在 K 线序列中定位 TradeDate 之后的下一根 K 线下标（即 T+1）。
// Day 格式为 "YYYY-MM-DD ..."（或纯日期），按 ISO 前缀比较；找不到返回 -1。
// 校验其前一根确实是 TradeDate（停牌等情形下容忍前一根早于 TradeDate）。
func findBarAfter(klines []KLineData, tradeDate string) int {
	for i := range klines {
		day := klines[i].Day
		if len(day) > 10 {
			day = day[:10]
		}
		if day > tradeDate {
			return i
		}
	}
	return -1
}

// reviewOne reviews a single pick against reviewDate's market data.
// 多窗口口径（基准=推荐日收盘价 ClosePrice，均为 %）：
//   - NextReturn/NextMaxReturn/NextMaxDrawdown：次日开盘→收盘（保留旧代理口径）
//   - NextReturnT1：次日收盘/当日收盘-1（可执行口径：尾盘买、T+1 收盘卖）
//   - Return3D / Return5D：第 3/5 个交易日收盘/当日收盘-1（未到期留 0，backfillWindows 补齐）
//
// K 线按日期索引定位（修复旧实现直接取最后一根、晚跑错位的问题）。
func (r *DailyPickReview) reviewOne(ctx context.Context, pick *models.DailyPick, reviewDate string) bool {
	apiCode := normalizeCode(pick.StockCode)

	klineData := NewStockDataApi().GetKLineData(apiCode, "240", reviewKlineBars)
	if klineData == nil || len(*klineData) == 0 {
		logger.SugaredLogger.Warnf("daily_review: no kline data for %s", pick.StockCode)
		return false
	}
	klines := *klineData

	t1 := findBarAfter(klines, pick.TradeDate)
	if t1 <= 0 {
		// t1==0 说明序列第一根就已晚于推荐日，推荐日基准不在窗口内（数据太旧）。
		logger.SugaredLogger.Warnf("daily_review: kline window does not cover trade date %s for %s", pick.TradeDate, pick.StockCode)
		return false
	}
	base := pick.ClosePrice
	if base <= 0 {
		base = parseFloat64(klines[t1-1].Close)
	}

	next := klines[t1]
	pick.NextOpen = parseFloat64(next.Open)
	pick.NextHigh = parseFloat64(next.High)
	pick.NextLow = parseFloat64(next.Low)
	pick.NextClose = parseFloat64(next.Close)

	// 旧口径（次日开盘买→收盘卖）
	if pick.NextOpen > 0 {
		pick.NextReturn = math.Round((pick.NextClose/pick.NextOpen-1)*10000) / 100
		pick.NextMaxReturn = math.Round((pick.NextHigh/pick.NextOpen-1)*10000) / 100
		drawdown := math.Round((pick.NextLow/pick.NextOpen-1)*100*100) / 100
		pick.NextMaxDrawdown = drawdown
	}

	// T+1 可执行口径：尾盘买入 → 次日收盘卖出
	if base > 0 && pick.NextClose > 0 {
		pick.NextReturnT1 = math.Round((pick.NextClose/base-1)*10000) / 100
	}

	// 3/5 日窗口（存在才回填，未到期由 backfillWindows 补齐）
	if idx := t1 + 2; idx < len(klines) && base > 0 {
		pick.Return3D = math.Round((parseFloat64(klines[idx].Close)/base-1)*10000) / 100
	}
	if idx := t1 + 4; idx < len(klines) && base > 0 {
		pick.Return5D = math.Round((parseFloat64(klines[idx].Close)/base-1)*10000) / 100
	}

	pick.Reviewed = true

	if err := r.repo.SavePick(ctx, pick); err != nil {
		logger.SugaredLogger.Errorf("daily_review: save review failed for %s: %v", pick.StockCode, err)
		return false
	}
	return true
}

// backfillWindows 回填近期已复盘 picks 的 3/5 日窗口（到期后数据可用）。
// 范围限定最近 15 个自然日，避免重复拉取历史数据；幂等可重复执行。
func (r *DailyPickReview) backfillWindows(ctx context.Context) {
	picks, err := r.repo.RecentPicksMissingWindows(ctx, 15)
	if err != nil {
		logger.SugaredLogger.Warnf("daily_review: backfill query error: %v", err)
		return
	}
	if len(picks) == 0 {
		return
	}
	backfilled := 0
	for i := range picks {
		p := &picks[i]
		klineData := NewStockDataApi().GetKLineData(normalizeCode(p.StockCode), "240", reviewKlineBars)
		if klineData == nil || len(*klineData) == 0 {
			continue
		}
		klines := *klineData
		t1 := findBarAfter(klines, p.TradeDate)
		if t1 <= 0 {
			continue
		}
		base := p.ClosePrice
		if base <= 0 {
			base = parseFloat64(klines[t1-1].Close)
		}
		if base <= 0 {
			continue
		}
		changed := false
		if p.Return3D == 0 {
			if idx := t1 + 2; idx < len(klines) {
				p.Return3D = math.Round((parseFloat64(klines[idx].Close)/base-1)*10000) / 100
				changed = true
			}
		}
		if p.Return5D == 0 {
			if idx := t1 + 4; idx < len(klines) {
				p.Return5D = math.Round((parseFloat64(klines[idx].Close)/base-1)*10000) / 100
				changed = true
			}
		}
		if changed {
			if err := r.repo.SavePick(ctx, p); err != nil {
				logger.SugaredLogger.Warnf("daily_review: backfill save failed for %s: %v", p.StockCode, err)
				continue
			}
			backfilled++
		}
	}
	if backfilled > 0 {
		logger.SugaredLogger.Infof("daily_review: backfilled %d picks with 3d/5d windows", backfilled)
	}
}

// ReviewAllUnreviewed reviews all unreviewed picks up to today.
func (r *DailyPickReview) ReviewAllUnreviewed(ctx context.Context) int {
	today := time.Now().Format("2006-01-02")
	return r.RunDailyReview(ctx, today, "")
}

// GetUnreviewedDate returns the earliest date with unreviewed picks, or empty string.
func (r *DailyPickReview) GetUnreviewedDate(ctx context.Context) string {
	pick, err := r.repo.EarliestUnreviewed(ctx)
	if err != nil {
		return ""
	}
	return pick.TradeDate
}

// GetReviewSummary returns a summary of review results for a given date.
// 除旧口径（次日开盘→收盘）外，附加多窗口口径：t1*（T+1 可执行）、r3*/r5*（3/5 日持有），
// 每窗口输出 avg（平均收益%）与 winRate（胜率%），仅统计已回填（≠0）的样本。
func (r *DailyPickReview) GetReviewSummary(ctx context.Context, tradeDate string) map[string]interface{} {
	picks := r.repo.ReviewedPicks(ctx, tradeDate)

	n := len(picks)
	if n == 0 {
		return map[string]interface{}{
			"count":       0,
			"avgReturn":   0.0,
			"winRate":     0.0,
			"totalWin":    0,
			"totalLoss":   0,
			"maxReturn":   0.0,
			"maxDrawdown": 0.0,
			"t1Avg":       0.0,
			"t1WinRate":   0.0,
			"t1Count":     0,
			"r3Avg":       0.0,
			"r3WinRate":   0.0,
			"r3Count":     0,
			"r5Avg":       0.0,
			"r5WinRate":   0.0,
			"r5Count":     0,
		}
	}

	var win, loss int
	var totalReturn, maxReturn, maxDrawdown float64
	for _, p := range picks {
		if p.NextReturn > 0 {
			win++
		} else {
			loss++
		}
		totalReturn += p.NextReturn
		if p.NextReturn > maxReturn {
			maxReturn = p.NextReturn
		}
		if p.NextMaxDrawdown < maxDrawdown {
			maxDrawdown = p.NextMaxDrawdown
		}
	}

	t1Avg, t1Win, t1Cnt := windowStats(picks, func(p models.DailyPick) float64 { return p.NextReturnT1 })
	r3Avg, r3Win, r3Cnt := windowStats(picks, func(p models.DailyPick) float64 { return p.Return3D })
	r5Avg, r5Win, r5Cnt := windowStats(picks, func(p models.DailyPick) float64 { return p.Return5D })

	return map[string]interface{}{
		"count":       n,
		"avgReturn":   math.Round(totalReturn/float64(n)*100) / 100,
		"winRate":     math.Round(float64(win)/float64(n)*10000) / 100,
		"totalWin":    win,
		"totalLoss":   loss,
		"maxReturn":   math.Round(maxReturn*100) / 100,
		"maxDrawdown": math.Round(maxDrawdown*100) / 100,
		"t1Avg":       t1Avg,
		"t1WinRate":   t1Win,
		"t1Count":     t1Cnt,
		"r3Avg":       r3Avg,
		"r3WinRate":   r3Win,
		"r3Count":     r3Cnt,
		"r5Avg":       r5Avg,
		"r5WinRate":   r5Win,
		"r5Count":     r5Cnt,
	}
}

// windowStats 计算单一窗口的 平均收益%/胜率%/样本数，跳过未回填（=0）样本。
func windowStats(picks []models.DailyPick, val func(models.DailyPick) float64) (avg, winRate float64, count int) {
	var sum float64
	var win int
	for _, p := range picks {
		v := val(p)
		if v == 0 {
			continue
		}
		sum += v
		count++
		if v > 0 {
			win++
		}
	}
	if count == 0 {
		return 0, 0, 0
	}
	avg = math.Round(sum/float64(count)*100) / 100
	winRate = math.Round(float64(win)/float64(count)*10000) / 100
	return avg, winRate, count
}
