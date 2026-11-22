package khunter

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-stock/backend/data"
)

func okPredict(delay time.Duration) PredictFunc {
	return func(ctx context.Context, code string, predLen int, asOf string) (*data.KronosPrediction, error) {
		if delay > 0 {
			time.Sleep(delay)
		}
		return &data.KronosPrediction{Summary: &data.KronosForecast{Direction: "up", ChangePct: 2, PredLen: 5}}, nil
	}
}

func waitStatus(t *testing.T, reg *PredictionJobRegistry, id, want string, timeout time.Duration) PredictJob {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		got, err := reg.Get(id)
		if err == nil && got.Status == want {
			return got
		}
		if time.Now().After(deadline) {
			t.Fatalf("等待 %s 超时，当前: %+v err=%v", want, got, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestSubmitRunsPredictAsync(t *testing.T) {
	reg := NewPredictionJobRegistry()
	j, err := reg.Submit(context.Background(), "sh600519", 5, "", okPredict(0))
	if err != nil {
		t.Fatal(err)
	}
	if j.Status == predictJobDone {
		t.Fatalf("提交后不应已是 done: %+v", j)
	}
	got := waitStatus(t, reg, j.ID, predictJobDone, 3*time.Second)
	if got.ResultJSON == "" {
		t.Fatalf("done 必须带结果: %+v", got)
	}
}

func TestSubmitDeduplicatesInFlight(t *testing.T) {
	reg := NewPredictionJobRegistry()
	slow := okPredict(80 * time.Millisecond)
	j1, _ := reg.Submit(context.Background(), "sh600519", 5, "", slow)
	j2, _ := reg.Submit(context.Background(), "sh600519", 5, "", slow)
	if j1.ID != j2.ID {
		t.Fatalf("在途同参数应复用任务: %s vs %s", j1.ID, j2.ID)
	}
}

func TestSubmitAfterFailureStartsNewJob(t *testing.T) {
	reg := NewPredictionJobRegistry()
	failing := func(ctx context.Context, code string, predLen int, asOf string) (*data.KronosPrediction, error) {
		return nil, errors.New("boom")
	}
	j1, _ := reg.Submit(context.Background(), "sh600519", 5, "", failing)
	waitStatus(t, reg, j1.ID, predictJobFailed, 3*time.Second)

	j2, _ := reg.Submit(context.Background(), "sh600519", 5, "", okPredict(0))
	if j2.ID == j1.ID {
		t.Fatal("failed 不得占用去重键，重试必须拿到新任务")
	}
	waitStatus(t, reg, j2.ID, predictJobDone, 3*time.Second)
}

func TestGetUnknownIDReturnsError(t *testing.T) {
	reg := NewPredictionJobRegistry()
	if _, err := reg.Get("pj_nope"); !errors.Is(err, ErrPredictJobNotFound) {
		t.Fatalf("未知 ID 必须返回 ErrPredictJobNotFound: %v", err)
	}
}

func TestModelLoadingEventuallyFailsAfterWindow(t *testing.T) {
	reg := NewPredictionJobRegistry()
	reg.retryWindow = 60 * time.Millisecond // 测试压缩窗口，避免跑满 8 分钟
	reg.backoff = 10 * time.Millisecond
	alwaysLoading := func(ctx context.Context, code string, predLen int, asOf string) (*data.KronosPrediction, error) {
		return nil, data.ErrKronosModelLoading
	}
	j, _ := reg.Submit(context.Background(), "sh600519", 5, "", alwaysLoading)
	got := waitStatus(t, reg, j.ID, predictJobFailed, 5*time.Second)
	if got.Err == "" {
		t.Fatal("失败应记录错误原因")
	}
}

func TestRegistryPrunesExpiredJobs(t *testing.T) {
	reg := NewPredictionJobRegistry()
	reg.ttl = 30 * time.Millisecond
	j, _ := reg.Submit(context.Background(), "sh600519", 5, "", okPredict(0))
	waitStatus(t, reg, j.ID, predictJobDone, 3*time.Second)
	time.Sleep(60 * time.Millisecond)
	reg.pruneLocked(time.Now())
	if _, err := reg.Get(j.ID); !errors.Is(err, ErrPredictJobNotFound) {
		t.Fatalf("过期任务应被清理: %v", err)
	}
}
