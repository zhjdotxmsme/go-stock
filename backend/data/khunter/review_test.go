package khunter

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
)

// klineRows 构造假 K 线：每对 "YYYY-MM-DD=收盘价"（只填 Day/Close，复盘只看这两列）。
func klineRows(pairs ...string) *[]data.KLineData {
	out := make([]data.KLineData, 0, len(pairs))
	for _, p := range pairs {
		kv := strings.SplitN(p, "=", 2)
		out = append(out, data.KLineData{Day: kv[0], Close: kv[1]})
	}
	return &out
}

// fullWindowKlines 覆盖 11-03（进入日）及其后 5 个交易日（11-04~11-10 均为交易日）。
func fullWindowKlines() *[]data.KLineData {
	return klineRows(
		"2026-11-02=9.80", "2026-11-03=10.00", "2026-11-04=10.50",
		"2026-11-05=11.00", "2026-11-06=11.50", "2026-11-09=12.00", "2026-11-10=12.50",
	)
}

func TestHuntingWindowReturns(t *testing.T) {
	t1, r3, r5, covered := huntingWindowReturns(*fullWindowKlines(), "2026-11-03", 10.0)
	if !covered {
		t.Fatal("covered should be true")
	}
	if t1 != 5.0 || r3 != 15.0 || r5 != 25.0 {
		t.Fatalf("got t1=%v r3=%v r5=%v, want 5/15/25", t1, r3, r5)
	}
	// EntryPrice 缺失时回落到进入日前一根收盘（11-03 的 10.00）。
	t1, _, _, covered = huntingWindowReturns(*fullWindowKlines(), "2026-11-03", 0)
	if !covered || t1 != 5.0 {
		t.Fatalf("fallback base failed: t1=%v covered=%v", t1, covered)
	}
	// 进入日不在 K 线窗口内 → covered=false。
	if _, _, _, covered = huntingWindowReturns(*fullWindowKlines(), "2026-10-01", 10.0); covered {
		t.Fatal("covered should be false for out-of-window enter date")
	}
}

func TestRunDailyReviewBackfillsHunting(t *testing.T) {
	setupTestDB(t)
	h := models.KhunterHunting{Code: "sh600519", Name: "贵州茅台", SignalDate: "2026-11-03",
		EnterDate: "2026-11-03", EntryPrice: 10.0, Status: HuntingStatusHolding}
	if err := db.Dao.Create(&h).Error; err != nil {
		t.Fatalf("seed hunting: %v", err)
	}
	rv := NewReview()
	rv.kline = func(code string) *[]data.KLineData { return fullWindowKlines() }
	res, err := rv.RunDailyReview(context.Background())
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if res.HuntingReviewed != 1 {
		t.Fatalf("HuntingReviewed=%d want 1 (total=%d partial=%d skipped=%d)",
			res.HuntingReviewed, res.HuntingTotal, res.HuntingPartial, res.HuntingSkipped)
	}
	var got models.KhunterHunting
	db.Dao.First(&got, h.ID)
	if !got.Reviewed || got.ReturnT1 != 5.0 || got.Return3D != 15.0 || got.Return5D != 25.0 {
		t.Fatalf("row not backfilled: %+v", got)
	}
}

func TestRunDailyReviewPartialWindowStaysUnreviewed(t *testing.T) {
	setupTestDB(t)
	h := models.KhunterHunting{Code: "sh600519", SignalDate: "2026-11-03", EnterDate: "2026-11-03",
		EntryPrice: 10.0, Status: HuntingStatusHolding}
	db.Dao.Create(&h)
	short := klineRows("2026-11-02=9.80", "2026-11-03=10.00", "2026-11-04=10.50")
	rv := NewReview()
	rv.kline = func(code string) *[]data.KLineData { return short }
	res, _ := rv.RunDailyReview(context.Background())
	if res.HuntingReviewed != 0 || res.HuntingPartial != 1 {
		t.Fatalf("want partial backfill, got reviewed=%d partial=%d", res.HuntingReviewed, res.HuntingPartial)
	}
	var got models.KhunterHunting
	db.Dao.First(&got, h.ID)
	if got.Reviewed || got.ReturnT1 != 5.0 || got.Return3D != 0 {
		t.Fatalf("partial state wrong: %+v", got)
	}
	// 到期后补齐（幂等重跑，窗口数据变长）。
	rv.kline = func(code string) *[]data.KLineData { return fullWindowKlines() }
	res, _ = rv.RunDailyReview(context.Background())
	if res.HuntingReviewed != 1 {
		t.Fatalf("backfill completion failed: reviewed=%d", res.HuntingReviewed)
	}
	db.Dao.First(&got, h.ID)
	if !got.Reviewed || got.Return5D != 25.0 {
		t.Fatalf("completion state wrong: %+v", got)
	}
}

func TestRunDailyReviewSkipsLegacyAndExitedCoverage(t *testing.T) {
	setupTestDB(t)
	db.Dao.Create(&models.KhunterHunting{Code: "sh600000", SignalDate: "2026-11-03",
		EnterDate: "2026-11-03", EntryPrice: 10.0, Status: HuntingStatusLegacy})
	db.Dao.Create(&models.KhunterHunting{Code: "sh600519", SignalDate: "2026-11-03",
		EnterDate: "2026-11-03", EntryPrice: 10.0, Status: HuntingStatusExited})
	rv := NewReview()
	rv.kline = func(code string) *[]data.KLineData { return fullWindowKlines() }
	res, _ := rv.RunDailyReview(context.Background())
	if res.HuntingTotal != 1 || res.HuntingReviewed != 1 {
		t.Fatalf("legacy must be excluded: total=%d reviewed=%d", res.HuntingTotal, res.HuntingReviewed)
	}
}

func forecastJSON(direction string, chg float64) string {
	return `{"summary":{"direction":"` + direction + `","changePct":` +
		strconv.FormatFloat(chg, 'f', 2, 64) + `,"confidence":80,"predLen":5}}`
}

func TestJudgePredictionHitUp(t *testing.T) {
	setupTestDB(t)
	p := models.KhunterKronosPrediction{Code: "sh600519", SignalDate: "2026-11-03", PredLen: 5,
		ForecastJSON: forecastJSON("up", 5), FactorScore: 70, Weight: 0.05}
	db.Dao.Create(&p)
	rv := NewReview()
	rv.kline = func(code string) *[]data.KLineData { return fullWindowKlines() } // T+5=11-10 收 12.50
	res, _ := rv.RunDailyReview(context.Background())
	if res.PredJudged != 1 || res.PredTotal != 1 {
		t.Fatalf("pred judged=%d total=%d", res.PredJudged, res.PredTotal)
	}
	var got models.KhunterKronosPrediction
	db.Dao.First(&got, p.ID)
	if got.Hit != "hit" || got.ActualPct != 25.0 {
		t.Fatalf("want hit/25.0, got %s/%v", got.Hit, got.ActualPct)
	}
	// 幂等：已判记录不再重复对账。
	res, _ = rv.RunDailyReview(context.Background())
	if res.PredTotal != 0 {
		t.Fatalf("judged prediction re-selected: total=%d", res.PredTotal)
	}
}

func TestJudgePredictionMissAndFlatBand(t *testing.T) {
	setupTestDB(t)
	down := models.KhunterKronosPrediction{Code: "sh600519", SignalDate: "2026-11-03", PredLen: 5,
		ForecastJSON: forecastJSON("down", -5)}
	db.Dao.Create(&down)
	rv := NewReview()
	rv.kline = func(code string) *[]data.KLineData { return fullWindowKlines() }
	rv.RunDailyReview(context.Background())
	var got models.KhunterKronosPrediction
	db.Dao.First(&got, down.ID)
	if got.Hit != "miss" {
		t.Fatalf("want miss, got %s", got.Hit)
	}
}

func TestJudgePredictionWaitsMaturity(t *testing.T) {
	setupTestDB(t)
	p := models.KhunterKronosPrediction{Code: "sh600519", SignalDate: "2026-11-03", PredLen: 5,
		ForecastJSON: forecastJSON("up", 5)}
	db.Dao.Create(&p)
	rv := NewReview()
	rv.kline = func(code string) *[]data.KLineData {
		return klineRows("2026-11-02=9.80", "2026-11-03=10.00", "2026-11-04=10.50") // 窗口未走完
	}
	res, _ := rv.RunDailyReview(context.Background())
	if res.PredPending != 1 {
		t.Fatalf("want pending=1, got judged=%d pending=%d", res.PredJudged, res.PredPending)
	}
	var got models.KhunterKronosPrediction
	db.Dao.First(&got, p.ID)
	if got.Hit != "" {
		t.Fatalf("immature prediction must stay unjudged, got %q", got.Hit)
	}
}
