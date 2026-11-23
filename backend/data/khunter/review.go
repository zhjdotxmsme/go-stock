package khunter

import (
	"context"
	"encoding/json"
	"math"
	"strconv"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/stockcode"
)

// huntingReviewBars 复盘取数根数：覆盖 EnterDate/SignalDate 及其后 5 个交易日。
const huntingReviewBars = 20

// reviewHistoryDays 参与回填的回看上限（自然日），与调权历史窗口（60 天）对齐。
const reviewHistoryDays = 60

// Review 狩猎复盘：收益回填 + Kronos 预测对账（09:15 khunter_review cron 驱动）。
type Review struct {
	repo  *Repo
	kline func(code string) *[]data.KLineData
}

// NewReview 生产构造：K 线取数走本地行情接口。
func NewReview() *Review {
	return &Review{repo: NewRepo(), kline: defaultReviewKlines}
}

func defaultReviewKlines(code string) *[]data.KLineData {
	return data.NewStockDataApi().GetKLineData(stockcode.Normalize(code), "240", huntingReviewBars)
}

// ReviewResult 单次复盘运行汇总（日志/排障用）。
type ReviewResult struct {
	HuntingTotal    int
	HuntingReviewed int
	HuntingPartial  int // 已部分回填但未到期（Reviewed 仍为 false）
	HuntingSkipped  int
	PredTotal       int
	PredJudged      int
	PredPending     int // 预测窗口未走完，跳过
}

type huntingReviewState int

const (
	huntingSkipped huntingReviewState = iota
	huntingPartial
	huntingReviewed
)

// RunDailyReview 执行一轮复盘：先回填狩猎记录窗口收益，再对账 Kronos 预测。
// 幂等：已 Reviewed 记录与已判 Hit 预测自动跳过；单条失败不阻塞整轮。
func (r *Review) RunDailyReview(ctx context.Context) (*ReviewResult, error) {
	res := &ReviewResult{}
	cutoff := time.Now().AddDate(0, 0, -reviewHistoryDays).Format("2006-01-02")

	rows, err := r.repo.HuntingNeedingReview(cutoff)
	if err != nil {
		return res, err
	}
	res.HuntingTotal = len(rows)
	for i := range rows {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		switch r.reviewHunting(&rows[i]) {
		case huntingReviewed:
			res.HuntingReviewed++
		case huntingPartial:
			res.HuntingPartial++
		default:
			res.HuntingSkipped++
		}
	}

	preds, err := r.repo.PredictionsNeedingHit(cutoff)
	if err != nil {
		return res, err
	}
	res.PredTotal = len(preds)
	for i := range preds {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		if r.judgePrediction(&preds[i]) {
			res.PredJudged++
		} else {
			res.PredPending++
		}
	}
	return res, nil
}

// huntingWindowReturns 计算 T+1/3/5 窗口收益（%）：base=EntryPrice（<=0 时回落进入日前收盘）。
// covered=false 表示连 T+1 都不在 K 线窗口内（数据太旧/缺口），调用方应跳过该记录。
func huntingWindowReturns(klines []data.KLineData, enterDate string, entryPrice float64) (t1, r3, r5 float64, covered bool) {
	t1Idx := data.FindBarAfter(klines, enterDate)
	if t1Idx <= 0 {
		return 0, 0, 0, false
	}
	base := entryPrice
	if base <= 0 {
		base = parseClosePx(klines[t1Idx-1].Close)
	}
	if base <= 0 {
		return 0, 0, 0, false
	}
	calc := func(idx int) float64 {
		if idx < 0 || idx >= len(klines) {
			return 0
		}
		c := parseClosePx(klines[idx].Close)
		if c <= 0 {
			return 0
		}
		return math.Round((c/base-1)*10000) / 100
	}
	return calc(t1Idx), calc(t1Idx + 2), calc(t1Idx + 4), true
}

func parseClosePx(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// reviewHunting 回填单条狩猎记录：部分窗口到期即部分写入，三窗齐了才置 Reviewed。
func (r *Review) reviewHunting(h *models.KhunterHunting) huntingReviewState {
	kl := r.kline(h.Code)
	if kl == nil || len(*kl) == 0 {
		return huntingSkipped
	}
	t1, r3, r5, covered := huntingWindowReturns(*kl, h.EnterDate, h.EntryPrice)
	if !covered {
		return huntingSkipped
	}
	changed := false
	if h.ReturnT1 == 0 && t1 != 0 {
		h.ReturnT1 = t1
		changed = true
	}
	if h.Return3D == 0 && r3 != 0 {
		h.Return3D = r3
		changed = true
	}
	if h.Return5D == 0 && r5 != 0 {
		h.Return5D = r5
		changed = true
	}
	if h.ReturnT1 != 0 && h.Return3D != 0 && h.Return5D != 0 && !h.Reviewed {
		h.Reviewed = true
		changed = true
	}
	if !changed {
		if h.Reviewed {
			return huntingReviewed
		}
		return huntingPartial
	}
	if err := r.repo.SaveHuntingReview(h); err != nil {
		logger.SugaredLogger.Warnf("khunter_review: 保存回填失败 %s %s: %v", h.Code, h.EnterDate, err)
		return huntingSkipped
	}
	if h.Reviewed {
		return huntingReviewed
	}
	return huntingPartial
}

// judgePrediction 对账单条 Kronos 预测：预测期末（signal_date+predLen 交易日）实际收盘
// vs 预测方向（Summary.Direction 缺失时按 ChangePct 符号 ±0.3% 方向带推导）。
// 已到期返回 true（Hit 终态已回写，含 "flat"）；未到期/缺数据返回 false。
func (r *Review) judgePrediction(p *models.KhunterKronosPrediction) bool {
	if p.ForecastJSON == "" {
		return false
	}
	var pred data.KronosPrediction
	if err := json.Unmarshal([]byte(p.ForecastJSON), &pred); err != nil || pred.Summary == nil {
		return false
	}
	direction := pred.Summary.Direction
	if direction == "" {
		chg := pred.Summary.ChangePct
		direction = "flat"
		if chg > 0.3 {
			direction = "up"
		} else if chg < -0.3 {
			direction = "down"
		}
	}
	horizon := data.KronosHorizon(p.PredLen)

	kl := r.kline(p.Code)
	if kl == nil || len(*kl) == 0 {
		return false
	}
	klines := *kl
	t1Idx := data.FindBarAfter(klines, p.SignalDate)
	if t1Idx <= 0 {
		return false
	}
	base := parseClosePx(klines[t1Idx-1].Close)
	if base <= 0 {
		return false
	}
	idx := t1Idx + horizon - 1
	if idx >= len(klines) { // 预测窗口未走完，下次再判
		return false
	}
	actualPct := math.Round((parseClosePx(klines[idx].Close)/base-1)*10000) / 100
	hit := data.KronosHorizonHit(direction, actualPct)
	if hit == "" { // 方向带排除：终态标记 flat，不再重判
		hit = "flat"
	}
	if err := r.repo.SavePredictionHit(p.ID, hit, actualPct); err != nil {
		logger.SugaredLogger.Warnf("khunter_review: 保存预测对账失败 %s %s: %v", p.Code, p.SignalDate, err)
		return false
	}
	p.Hit = hit
	p.ActualPct = actualPct
	return true
}
