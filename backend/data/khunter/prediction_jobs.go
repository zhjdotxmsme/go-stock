package khunter

// 异步单股 Kronos 预测任务注册表（内存态，进程级）。
// 解决模型冷启动 30s+ 导致的"点击无回显"：提交即返回任务 ID，前端轮询 Get。
// 语义要点：
//   - 去重只对 pending/running/done 生效，failed 不占键（否则服务恢复后重试永远拿到同一个失败任务）；
//   - 冷启动重试窗口对齐 warmup 上限（8 分钟），不是 16 秒；
//   - 未知 ID 返回 ErrPredictJobNotFound，避免前端把零值当 pending 无限轮询；
//   - done/failed 保留 ttl（24h），提交时顺带清理，常驻进程不膨胀。

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"time"

	"go-stock/backend/data"
)

// ErrPredictJobNotFound 任务不存在或已过期（进程重启 / TTL 清理）。
var ErrPredictJobNotFound = errors.New("预测任务不存在或已过期")

const (
	predictJobPending = "pending"
	predictJobRunning = "running"
	predictJobDone    = "done"
	predictJobFailed  = "failed"
)

// PredictFunc 单股预测注入点（生产传 data.PredictKLine）。
type PredictFunc func(ctx context.Context, stockCode string, predLen int, asOfDate string) (*data.KronosPrediction, error)

type PredictJob struct {
	ID         string    `json:"id"`
	Code       string    `json:"code"`
	PredLen    int       `json:"predLen"`
	AsOfDate   string    `json:"asOfDate"`
	Status     string    `json:"status"` // pending / running / done / failed
	ResultJSON string    `json:"resultJson,omitempty"`
	Err        string    `json:"err,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type PredictionJobRegistry struct {
	mu   sync.Mutex
	jobs map[string]PredictJob
	// byKey 去重键 code|predLen|asOfDate|今日 → jobID；只在非 failed 状态存在
	byKey map[string]string

	// 可注入以便测试；生产用默认值
	retryWindow time.Duration // 冷启动重试总窗口，默认 8min（对齐 kronosWarmup）
	backoff     time.Duration // 首次退避，指数翻倍，默认 2s
	ttl         time.Duration // 终态任务保留时长，默认 24h
	now         func() time.Time
}

func NewPredictionJobRegistry() *PredictionJobRegistry {
	return &PredictionJobRegistry{
		jobs:        map[string]PredictJob{},
		byKey:       map[string]string{},
		retryWindow: 8 * time.Minute,
		backoff:     2 * time.Second,
		ttl:         24 * time.Hour,
		now:         time.Now,
	}
}

func (r *PredictionJobRegistry) dedupeKey(code string, predLen int, asOfDate string) string {
	return code + "|" + strconv.Itoa(predLen) + "|" + asOfDate + "|" + r.now().Format("2006-01-02")
}

func (r *PredictionJobRegistry) Submit(ctx context.Context, code string, predLen int, asOfDate string,
	predict PredictFunc,
) (PredictJob, error) {
	if predict == nil {
		return PredictJob{}, errors.New("缺少预测实现")
	}
	key := r.dedupeKey(code, predLen, asOfDate)

	r.mu.Lock()
	r.pruneLocked(r.now())
	if id, ok := r.byKey[key]; ok {
		if j, exists := r.jobs[id]; exists && j.Status != predictJobFailed {
			r.mu.Unlock()
			return j, nil
		}
	}
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	nowT := r.now()
	j := PredictJob{
		ID: "pj_" + hex.EncodeToString(b), Code: code, PredLen: predLen, AsOfDate: asOfDate,
		Status: predictJobPending, CreatedAt: nowT, UpdatedAt: nowT,
	}
	r.jobs[j.ID] = j
	r.byKey[key] = j.ID
	r.mu.Unlock()

	go r.run(j.ID, predict)
	return j, nil
}

func (r *PredictionJobRegistry) run(id string, predict PredictFunc) {
	r.setStatus(id, predictJobRunning, "", "")
	deadline := r.now().Add(r.retryWindow)
	backoff := r.backoff
	for {
		j, err := r.snapshot(id)
		if err != nil {
			return // 已被清理
		}
		pred, perr := predict(context.Background(), j.Code, j.PredLen, j.AsOfDate)
		if perr == nil {
			b, mErr := json.Marshal(pred)
			if mErr != nil {
				r.fail(id, "序列化预测结果失败: "+mErr.Error())
				return
			}
			r.setStatus(id, predictJobDone, string(b), "")
			return
		}
		if errors.Is(perr, data.ErrKronosModelLoading) && r.now().Before(deadline) {
			time.Sleep(backoff)
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			continue
		}
		r.fail(id, perr.Error())
		return
	}
}

// fail 转 failed 并释放去重键（重试必须能拿到新任务）。
func (r *PredictionJobRegistry) fail(id, msg string) {
	r.mu.Lock()
	j, ok := r.jobs[id]
	if !ok {
		r.mu.Unlock()
		return
	}
	j.Status, j.Err, j.ResultJSON, j.UpdatedAt = predictJobFailed, msg, "", r.now()
	r.jobs[id] = j
	key := r.dedupeKey(j.Code, j.PredLen, j.AsOfDate)
	if r.byKey[key] == id {
		delete(r.byKey, key)
	}
	r.mu.Unlock()
}

func (r *PredictionJobRegistry) setStatus(id, status, result, errMsg string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.jobs[id]
	if !ok {
		return
	}
	j.Status, j.ResultJSON, j.Err, j.UpdatedAt = status, result, errMsg, r.now()
	r.jobs[id] = j
}

func (r *PredictionJobRegistry) snapshot(id string) (PredictJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.jobs[id]
	if !ok {
		return PredictJob{}, ErrPredictJobNotFound
	}
	return j, nil
}

// Get 查询任务；未知 ID 返回 ErrPredictJobNotFound（调用方必须转成前端可见错误）。
func (r *PredictionJobRegistry) Get(id string) (PredictJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pruneLocked(r.now())
	j, ok := r.jobs[id]
	if !ok {
		return PredictJob{}, ErrPredictJobNotFound
	}
	return j, nil
}

// pruneLocked 清理超过 ttl 的终态任务（调用方须持锁）。
func (r *PredictionJobRegistry) pruneLocked(now time.Time) {
	for id, j := range r.jobs {
		if j.Status != predictJobDone && j.Status != predictJobFailed {
			continue
		}
		if now.Sub(j.UpdatedAt) < r.ttl {
			continue
		}
		delete(r.jobs, id)
		key := r.dedupeKey(j.Code, j.PredLen, j.AsOfDate)
		if r.byKey[key] == id {
			delete(r.byKey, key)
		}
	}
}
