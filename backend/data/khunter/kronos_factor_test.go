package khunter

import (
	"context"
	"errors"
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/models"
)

func obs(id uint, code string, score float64) models.KhunterObservation {
	return models.KhunterObservation{ID: id, Code: code, SignalDate: "2026-11-20", Score: score, Status: ObsStatusPending}
}

func predOf(changePct, confidence float64) *data.KronosPrediction {
	return &data.KronosPrediction{Summary: &data.KronosForecast{
		Direction: "up", ChangePct: changePct, Confidence: confidence, PredLen: 5,
	}}
}

// batchPredict 测试替身：按 apiCode 返回固定摘要，errs 用于模拟单只失败。
type batchPredict struct {
	preds map[string]*data.KronosPrediction
	errs  map[string]string
	calls int
}

func (b *batchPredict) fn(ctx context.Context, codes []string, predLen int) (map[string]*data.KronosPrediction, map[string]string, error) {
	b.calls++
	return b.preds, b.errs, nil
}

func TestKronosFactorScoreNeutralOnNil(t *testing.T) {
	if got := KronosFactorScore(nil); got != 50 {
		t.Fatalf("空预测应中性 50: %v", got)
	}
	if got := KronosFactorScore(&data.KronosPrediction{}); got != 50 {
		t.Fatalf("无 summary 应中性 50: %v", got)
	}
}

func TestKronosFactorScoreMatchesDailyPickFormula(t *testing.T) {
	// 与 daily_pick_kronos.kronosFactorScore 同口径：chg=+4% → base=70
	if got := KronosFactorScore(predOf(4, 100)); got != 70 {
		t.Fatalf("chg=+4%% conf=100 应为 70: %v", got)
	}
	// conf=50：70*0.5 + 50*0.5 = 60
	if got := KronosFactorScore(predOf(4, 50)); got != 60 {
		t.Fatalf("chg=+4%% conf=50 应为 60: %v", got)
	}
}

// TestKronosFactorScoreUnchangedByBars 因子分只依赖 Summary：Bars 存在与否不影响
// （证明不需要旧稿假设的 LastClose/Forecasts 字段）。
func TestKronosFactorScoreUnchangedByBars(t *testing.T) {
	p := predOf(4, 100)
	p.Bars = []*data.KronosBar{{Date: "2026-11-21", Close: 104}}
	if got := KronosFactorScore(p); got != 70 {
		t.Fatalf("Bars 不应改变因子分: %v", got)
	}
}

func TestApplyKronosFactorDisabledIsNoop(t *testing.T) {
	cfg := DefaultKronosFactorConfig() // Enable=false
	b := &batchPredict{}
	got, err := ApplyKronosFactor(context.Background(), []models.KhunterObservation{obs(1, "sh600519", 70)}, cfg, b.fn)
	if err != nil || len(got) != 0 || b.calls != 0 {
		t.Fatalf("关闭时应空结果且不调用预测: %+v %v calls=%d", got, err, b.calls)
	}
}

func TestApplyKronosFactorTakesTopNAndBatchesOnce(t *testing.T) {
	cfg := DefaultKronosFactorConfig()
	cfg.Enable = true
	cfg.TopN = 2
	b := &batchPredict{preds: map[string]*data.KronosPrediction{
		"sh600519": predOf(4, 100),
		"sz000001": predOf(-4, 100),
	}}
	got, err := ApplyKronosFactor(context.Background(), []models.KhunterObservation{
		obs(1, "sh600519", 80), obs(2, "sz000001", 60), obs(3, "sz000002", 50),
	}, cfg, b.fn)
	if err != nil {
		t.Fatal(err)
	}
	if b.calls != 1 {
		t.Fatalf("应只发一次批量请求，实际 %d 次", b.calls)
	}
	if len(got) != 2 || got[0].ObservationID != 1 {
		t.Fatalf("应按评分降序取 topN=2: %+v", got)
	}
	if got[0].FactorScore != 70 || got[1].FactorScore != 30 {
		t.Fatalf("因子分错误: %+v", got)
	}
	if got[0].ForecastJSON == "" {
		t.Fatalf("预测明细应序列化落库: %+v", got[0])
	}
}

func TestApplyKronosFactorDegradedOnFailure(t *testing.T) {
	cfg := DefaultKronosFactorConfig()
	cfg.Enable = true
	b := &batchPredict{
		preds: map[string]*data.KronosPrediction{"sh600519": nil},
		errs:  map[string]string{"sh600519": "boom"},
	}
	got, err := ApplyKronosFactor(context.Background(), []models.KhunterObservation{obs(1, "sh600519", 80)}, cfg, b.fn)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].DegradedReason == "" || got[0].FactorScore != 50 {
		t.Fatalf("失败应降级中性分并记原因: %+v", got)
	}
	if got[0].ForecastJSON != "" {
		t.Fatalf("降级不应写预测明细: %+v", got[0])
	}
}

func TestApplyKronosFactorOfflineReturnsDegradedNotError(t *testing.T) {
	cfg := DefaultKronosFactorConfig()
	cfg.Enable = true
	fn := func(ctx context.Context, codes []string, predLen int) (map[string]*data.KronosPrediction, map[string]string, error) {
		return nil, nil, data.ErrKronosOffline
	}
	got, err := ApplyKronosFactor(context.Background(), []models.KhunterObservation{obs(1, "sh600519", 80)}, cfg, fn)
	if err != nil {
		t.Fatalf("服务离线不得让调用方拿 error（会阻塞主链）: %v", err)
	}
	if len(got) != 1 || got[0].DegradedReason == "" || got[0].FactorScore != 50 {
		t.Fatalf("离线应整体降级中性分: %+v", got)
	}
}

var _ = errors.Is // 保留 errors import（离线用例可直接扩展成 errors.Is 断言）
