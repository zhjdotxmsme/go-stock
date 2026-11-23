package khunter

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

// seedJudgedPredictions 生成 n 条已对账预测（信号日=成熟阈值之前），前 hits 条为 hit。
func seedJudgedPredictions(t *testing.T, n, hits int) {
	t.Helper()
	mature := time.Now().AddDate(0, 0, -8).Format("2006-01-02")
	for i := 0; i < n; i++ {
		hit := "miss"
		if i < hits {
			hit = "hit"
		}
		p := models.KhunterKronosPrediction{
			Code: fmt.Sprintf("sh6005%02d", i), SignalDate: mature, PredLen: 5,
			ForecastJSON: `{"summary":{"direction":"up","changePct":5,"confidence":80,"predLen":5}}`,
			Hit:          hit, ActualPct: 3.5,
		}
		if err := db.Dao.Create(&p).Error; err != nil {
			t.Fatalf("seed prediction: %v", err)
		}
	}
}

func TestKronosFactorMultiplierGuards(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	// 样本不足 20 → 恒 1.0。
	seedJudgedPredictions(t, 19, 19)
	if got := KronosFactorMultiplier(ctx); got != 1.0 {
		t.Fatalf("small sample: got %v want 1.0", got)
	}
}

func TestKronosFactorMultiplierBands(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		hits int
		want float64
	}{
		{14, 3.0}, // 70% → 1+(0.70-0.5)*10=3.0（封顶）
		{12, 2.0}, // 60% → 2.0
		{11, 1.5}, // 55% → 1.5
		{10, 1.0}, // 50% → 1.0（[0.45,0.55) 区间不调）
		{9, 1.0},  // 45% → 1.0（区间下限，不降权）
		{8, 0.5},  // 40% → 0.5
		{7, 0},    // 35% → 0（权重归零）
	}
	for _, c := range cases {
		setupTestDB(t)
		seedJudgedPredictions(t, 20, c.hits)
		if got := KronosFactorMultiplier(ctx); got != c.want {
			t.Fatalf("hits=%d: got %v want %v", c.hits, got, c.want)
		}
	}
}

func TestKronosFactorStatsValues(t *testing.T) {
	setupTestDB(t)
	seedJudgedPredictions(t, 20, 12) // 60% 命中
	stats := KronosFactorStats(context.Background())
	if stats.Samples != 20 || stats.Hits != 12 || stats.HitRate != 0.6 {
		t.Fatalf("stats wrong: %+v", stats)
	}
	if stats.AvgPred != 5.0 || stats.AvgT1 != 3.5 {
		t.Fatalf("avg wrong: %+v", stats)
	}
}

func TestKronosFactorStatsExcludesFlatAndImmature(t *testing.T) {
	setupTestDB(t)
	seedJudgedPredictions(t, 20, 12)
	// flat 与未成熟样本不计入。
	recent := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	db.Dao.Create(&models.KhunterKronosPrediction{Code: "sh601000", SignalDate: recent,
		PredLen: 5, ForecastJSON: `{"summary":{"direction":"up","changePct":5}}`, Hit: "flat", ActualPct: 0.1})
	db.Dao.Create(&models.KhunterKronosPrediction{Code: "sh601001", SignalDate: recent,
		PredLen: 5, ForecastJSON: `{"summary":{"direction":"up","changePct":5}}`, Hit: ""})
	stats := KronosFactorStats(context.Background())
	if stats.Samples != 20 {
		t.Fatalf("flat/immature leaked into stats: %+v", stats)
	}
}

func TestAdjustedKronosWeightClampsToMax(t *testing.T) {
	setupTestDB(t)
	seedJudgedPredictions(t, 20, 14) // 70% → 系数 3（封顶）
	cfg := KronosFactorConfig{Enable: true, TopN: 30, PredLen: 5, Weight: 0.05, MaxWeight: 0.15}
	got := AdjustedKronosWeight(context.Background(), cfg)
	if got.Weight != 0.15 {
		t.Fatalf("weight not clamped: %v", got.Weight)
	}
}
