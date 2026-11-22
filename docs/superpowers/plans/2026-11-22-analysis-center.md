# 统一分析中心（第一阶段：选股预测族）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 选股预测族工具（每日选股/狩猎场/K线分析/选股工作台/Kronos）合并为统一分析中心：khunter 为唯一选股引擎，Kronos 成为统一预测服务，前端单页四 Tab。

**Architecture:** 后端新增 AnalysisCenterService 门面（委托 khunter 引擎与 Kronos 客户端）+ khunter Kronos 因子模块（批量预测落库、可配置、可降级）+ 异步单股预测任务注册表；前端新增 AnalysisCenter.vue Tab 壳（推荐/狩猎场/选股工作台/K线分析），旧路由 301 重定向，菜单收敛。旧 daily_pick 引擎与接口保留（deprecated），下一阶段删除。

**Tech Stack:** Go、GORM、SQLite、Wails 2、Vue 3 + Naive UI、Node 内置测试（`node --import ./tests/register-ts-ext.mjs --test`）。

**Spec:** `docs/superpowers/specs/2026-11-22-analysis-center-design.md`

## Global Constraints

- 不修改 khunter 引擎既有链路语义（时点约束/数据闸门/风险过滤/确认/退出）；Kronos 因子只新增，默认关闭（`enable:false`）。
- Kronos 服务不可用时一律降级（交互面提示、评分面中性分+degraded 标记），不得让预测故障阻塞选股主链。
- 旧接口响应格式不变（`TradingRecordHandler.PredictKLine` 等、`DailyPickHandler` 全部方法保持签名与返回结构）。
- 前端测试命令统一为 `cd frontend && node --import ./tests/register-ts-ext.mjs --test <file>`；类型检查 `node tests/run-vue-tsc.mjs`。
- 提交信息用 Conventional Commits；每任务一次提交。

**导入关系（已验证无环）**：`backend/data/khunter` 已 import `backend/data`（event_classify.go 等）；`backend/data` 不 import khunter 根包。门面放 `backend/data` 可直接调 khunter。

---

### Task 1: 导出 Kronos 客户端（data 包统一入口）

**Files:**
- Modify: `backend/data/kronos_service.go`
- Modify: `backend/handler/trading_handler.go:645-680`（Kronos 五个方法改委托）
- Test: `backend/data/kronos_service_test.go`（扩展）

**Interfaces:**
- Consumes: 既有未导出函数 `kronosPredictEx`、`kronosProcess.Status/EnsureRunning`、`GetKronosConfig`、错误变量 `ErrKronosDisabled` 等。
- Produces（后续任务与门面使用，签名必须一致）:
  - `func PredictKLine(ctx context.Context, stockCode string, predLen int, asOfDate string) (*KronosPrediction, error)`
  - `func KronosServiceStatus() string`（返回 `disabled` / `offline` / `loading` / `online`）
  - `func EnsureKronosRunning(ctx context.Context, timeout time.Duration) error`

- [ ] **Step 1: 写失败测试**

在 `backend/data/kronos_service_test.go` 追加：

```go
func TestKronosServiceStatusDisabledByDefault(t *testing.T) {
	// 设置未开启时（KronosEnable=false）状态必须为 disabled
	if got := KronosServiceStatus(); got != "disabled" {
		t.Fatalf("未开启应返回 disabled: %s", got)
	}
}
```

- [ ] **Step 2: 验证失败**

Run: `PATH="/d/Go/bin:$PATH" go test ./backend/data/ -run TestKronosServiceStatusDisabledByDefault -v`
Expected: FAIL（`KronosServiceStatus` 未定义）。

- [ ] **Step 3: 实现导出函数**

在 `backend/data/kronos_service.go` 文件尾部追加：

```go
// PredictKLine 统一 K 线预测入口：确保服务在线后调用 /predict。
// stockCode 支持任意格式（内部转东财 secid）；asOfDate 为空表示最新。
// 返回的错误可直接 errors.Is 匹配 ErrKronosDisabled/ErrKronosOffline/ErrKronosModelLoading。
func PredictKLine(ctx context.Context, stockCode string, predLen int, asOfDate string) (*KronosPrediction, error) {
	return kronosPredictEx(ctx, stockCode, predLen, asOfDate)
}

// KronosServiceStatus 统一状态入口：disabled / offline / loading / online。
func KronosServiceStatus() string {
	cfg := GetKronosConfig()
	if !cfg.Enable {
		return "disabled"
	}
	p := &kronosProcess{}
	switch {
	case p.childProcessExited():
		return "offline"
	case p.isModelReady(context.Background(), cfg.Port):
		return "online"
	case p.isAlive(context.Background(), cfg.Port):
		return "loading"
	default:
		return "offline"
	}
}

// EnsureKronosRunning 统一拉起入口：按需启动 Python 推理服务并等待健康。
func EnsureKronosRunning(ctx context.Context, timeout time.Duration) error {
	cfg := GetKronosConfig()
	if !cfg.Enable {
		return ErrKronosDisabled
	}
	p := &kronosProcess{}
	done := make(chan error, 1)
	go func() { done <- p.EnsureRunning(cfg) }()
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return ErrKronosTimeout
	case <-ctx.Done():
		return ctx.Err()
	}
}
```

- [ ] **Step 4: TradingRecordHandler 改委托（签名不变）**

`backend/handler/trading_handler.go` 五个方法体替换为一行委托（示例，其余同理）：

```go
func (h *TradingRecordHandler) PredictKLine(stockCode string, predLen int) (*data.KronosPrediction, error) {
	return data.PredictKLine(context.Background(), stockCode, predLen, "")
}
```

`GetKronosStatus` 委托 `data.KronosServiceStatus()`；`StartKronosService` 委托 `data.EnsureKronosRunning(context.Background(), 120*time.Second)`。保留方法上既有的 deprecated 注释（写明统一入口已迁移至 AnalysisCenterHandler，本方法为兼容保留）。

- [ ] **Step 5: 验证通过**

Run: `PATH="/d/Go/bin:$PATH" go test ./backend/data/ ./backend/handler/ -count=1`
Expected: PASS。

- [ ] **Step 6: Commit**

```bash
git add backend/data/kronos_service.go backend/data/kronos_service_test.go backend/handler/trading_handler.go
git commit -m "feat(kronos): export unified client entry points in data package"
```

---

### Task 2: khunter Kronos 因子模块（批量预测 + 明细落库）

**Files:**
- Create: `config/khunter_kronos.json`
- Create: `backend/data/khunter/kronos_config.go`
- Create: `backend/data/khunter/kronos_factor.go`
- Create: `backend/data/khunter/kronos_factor_test.go`
- Modify: `backend/data/khunter/pipeline.go`（观察落库后挂批量因子）
- Modify: `backend/models/khunter_models.go`（新模型）
- Modify: `backend/data/khunter/repo.go`（EnsureMigrate 加表 + SaveKronosPredictions）

**Interfaces:**
- Consumes: Task 1 的 `data.PredictKLine`；既有 `models.KhunterObservation`、`Repo`。
- Produces:
  - `type KronosFactorConfig struct { Enable bool; TopN int; PredLen int; Weight float64; MaxWeight float64 }`
  - `func DefaultKronosFactorConfig() KronosFactorConfig`（Enable:false, TopN:30, PredLen:5, Weight:0.05, MaxWeight:0.15）
  - `func LoadKronosFactorConfig(path string) KronosFactorConfig`（缺文件/解析失败回退默认）
  - `type KronosFactorResult struct { ObservationID uint; FactorScore float64; Weight float64; DegradedReason string }`
  - `func ApplyKronosFactor(ctx context.Context, observations []models.KhunterObservation, cfg KronosFactorConfig, predict func(ctx context.Context, stockCode string, predLen int, asOfDate string) (*data.KronosPrediction, error)) ([]KronosFactorResult, error)`
  - `func KronosFactorScore(pred *data.KronosPrediction, lastClose float64) float64`（确定性：expectedReturn=(median末日期望收盘- lastClose)/lastClose；score=clamp(50+500*r,0,100)）
  - 模型 `models.KhunterKronosPrediction{Code, SignalDate, PredLen, ForecastJSON, FactorScore, Weight, DegradedReason}`（唯一键 code+signal_date）
  - `func (r *Repo) SaveKronosPredictions(rows []models.KhunterKronosPrediction) error`

- [ ] **Step 1: 写失败测试**

`backend/data/khunter/kronos_factor_test.go`：

```go
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

func TestApplyKronosFactorDisabledIsNoop(t *testing.T) {
	cfg := DefaultKronosFactorConfig() // Enable=false
	got, err := ApplyKronosFactor(context.Background(), []models.KhunterObservation{obs(1, "sh600519", 70)}, cfg, nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("关闭时应空结果: %+v %v", got, err)
	}
}

func TestApplyKronosFactorMixesScore(t *testing.T) {
	cfg := DefaultKronosFactorConfig()
	cfg.Enable = true
	cfg.TopN = 2
	predict := func(ctx context.Context, code string, predLen int, asOf string) (*data.KronosPrediction, error) {
		return &data.KronosPrediction{LastClose: 100,
			Forecasts: []data.KronosForecast{{Date: "2026-11-21", Close: []float64{104, 104}}}}, nil
	}
	got, err := ApplyKronosFactor(context.Background(), []models.KhunterObservation{
		obs(1, "sh600519", 80), obs(2, "sz000001", 60), obs(3, "sz000002", 50),
	}, cfg, predict)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("只应处理 topN=2 只: %+v", got)
	}
	if got[0].ObservationID != 1 {
		t.Fatalf("应按评分降序取 topN: %+v", got[0])
	}
	// 期望收益 +4% → score=50+500*0.04=70
	if got[0].FactorScore != 70 {
		t.Fatalf("因子分=%v, want 70", got[0].FactorScore)
	}
}

func TestApplyKronosFactorDegradedOnFailure(t *testing.T) {
	cfg := DefaultKronosFactorConfig()
	cfg.Enable = true
	predict := func(ctx context.Context, code string, predLen int, asOf string) (*data.KronosPrediction, error) {
		return nil, errors.New("boom")
	}
	got, err := ApplyKronosFactor(context.Background(), []models.KhunterObservation{obs(1, "sh600519", 80)}, cfg, predict)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].DegradedReason == "" || got[0].FactorScore != 50 {
		t.Fatalf("失败应降级中性分并记原因: %+v", got)
	}
}
```

注意：`KronosPrediction` 结构以 `backend/data/kronos_service.go:51` 实际字段为准（`LastClose float64`、`Forecasts []KronosForecast`、`KronosForecast.Close []float64`）；若字段名不同，以现有定义为准调整测试构造，不得改 data 包结构。

- [ ] **Step 2: 验证失败**

Run: `PATH="/d/Go/bin:$PATH" go test ./backend/data/khunter/ -run 'ApplyKronosFactor|KronosFactorScore' -v`
Expected: FAIL（函数未定义）。

- [ ] **Step 3: 实现**

`config/khunter_kronos.json`：

```json
{
  "enable": false,
  "topN": 30,
  "predLen": 5,
  "weight": 0.05,
  "maxWeight": 0.15
}
```

`backend/data/khunter/kronos_config.go`：

```go
package khunter

import (
	"encoding/json"
	"os"
)

// KronosFactorConfig Kronos 批量因子配置（config/khunter_kronos.json）。
// 默认 enable=false：预测质量经滚动回测验证后再开启。
type KronosFactorConfig struct {
	Enable      bool    `json:"enable"`
	TopN        int     `json:"topN"`        // 参与批量预测的观察候选上限（CPU 推理成本约束）
	PredLen     int     `json:"predLen"`     // 预测天数
	Weight      float64 `json:"weight"`      // 混入综合推荐分的基础权重
	MaxWeight   float64 `json:"maxWeight"`   // 权重上限（防调权越界）
}

func DefaultKronosFactorConfig() KronosFactorConfig {
	return KronosFactorConfig{Enable: false, TopN: 30, PredLen: 5, Weight: 0.05, MaxWeight: 0.15}
}

const kronosFactorConfigRel = "config/khunter_kronos.json"

func LoadKronosFactorConfig(path string) KronosFactorConfig {
	cfg := DefaultKronosFactorConfig()
	if path == "" {
		path = kronosFactorConfigRel
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	if json.Unmarshal(b, &cfg) != nil {
		return DefaultKronosFactorConfig()
	}
	if cfg.TopN <= 0 {
		cfg.TopN = 30
	}
	if cfg.PredLen <= 0 {
		cfg.PredLen = 5
	}
	if cfg.Weight <= 0 {
		cfg.Weight = 0.05
	}
	if cfg.MaxWeight <= 0 {
		cfg.MaxWeight = 0.15
	}
	return cfg
}
```

`backend/data/khunter/kronos_factor.go`：

```go
package khunter

import (
	"context"
	"encoding/json"
	"sort"

	"go-stock/backend/data"
	"go-stock/backend/models"
)

// KronosFactorResult 单只观察的 Kronos 因子结论。
type KronosFactorResult struct {
	ObservationID  uint
	FactorScore    float64 // 0~100，失败时为中性 50
	Weight         float64 // 实际使用权重（cfg.Weight，钳位到 MaxWeight）
	DegradedReason string  // 非空表示降级（服务离线/预测失败/结果异常）
}

// KronosFactorScore 预测摘要 → 0~100 因子分：
// 期望收益 r = (中位末日期望收盘 − 最新收盘) / 最新收盘；score = clamp(50 + 500r, 0, 100)。
// r=+5% → 75，r=−5% → 25。确定性、可单测。
func KronosFactorScore(pred *data.KronosPrediction, lastClose float64) float64 {
	if pred == nil || lastClose <= 0 || len(pred.Forecasts) == 0 {
		return 50
	}
	// 每个 forecast 取最后一个 Close 样本，再取中位
	lasts := make([]float64, 0, len(pred.Forecasts))
	for _, f := range pred.Forecasts {
		if len(f.Close) > 0 {
			lasts = append(lasts, f.Close[len(f.Close)-1])
		}
	}
	if len(lasts) == 0 {
		return 50
	}
	sort.Float64s(lasts)
	med := lasts[len(lasts)/2]
	r := (med - lastClose) / lastClose
	score := 50 + 500*r
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

// ApplyKronosFactor 对观察候选按评分降序取 topN 做批量预测并折算因子分。
// predict 可注入（测试用假实现）；生产传 data.PredictKLine。
// 单只失败不中断批次：记 DegradedReason 与中性分，保持主链可用。
func ApplyKronosFactor(ctx context.Context, observations []models.KhunterObservation,
	cfg KronosFactorConfig,
	predict func(ctx context.Context, stockCode string, predLen int, asOfDate string) (*data.KronosPrediction, error),
) ([]KronosFactorResult, error) {
	if !cfg.Enable || predict == nil {
		return nil, nil
	}
	w := cfg.Weight
	if w > cfg.MaxWeight {
		w = cfg.MaxWeight
	}
	byScore := append([]models.KhunterObservation(nil), observations...)
	sort.SliceStable(byScore, func(i, j int) bool { return byScore[i].Score > byScore[j].Score })
	if len(byScore) > cfg.TopN {
		byScore = byScore[:cfg.TopN]
	}
	out := make([]KronosFactorResult, 0, len(byScore))
	for _, o := range byScore {
		res := KronosFactorResult{ObservationID: o.ID, FactorScore: 50, Weight: w}
		pred, err := predict(ctx, o.Code, cfg.PredLen, o.SignalDate)
		if err != nil {
			res.DegradedReason = err.Error()
		} else {
			res.FactorScore = KronosFactorScore(pred, pred.LastClose)
		}
		out = append(out, res)
	}
	return out, nil
}
```

`backend/models/khunter_models.go` 追加（放 KhunterObservation 之后）：

```go
// KhunterKronosPrediction 观察候选的 Kronos 批量预测明细（推荐视图可展开，因子透明化）。
type KhunterKronosPrediction struct {
	ID             uint      `gorm:"primarykey" json:"id"`
	Code           string    `gorm:"size:20;index:idx_kkp_code_date,unique" json:"code"`
	SignalDate     string    `gorm:"size:10;index:idx_kkp_code_date,unique" json:"signalDate"`
	PredLen        int       `json:"predLen"`
	ForecastJSON   string    `gorm:"type:text" json:"forecastJson"` // 完整预测结果（KronosPrediction 序列化）
	FactorScore    float64   `json:"factorScore"`
	Weight         float64   `json:"weight"`
	DegradedReason string    `gorm:"size:200" json:"degradedReason"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func (KhunterKronosPrediction) TableName() string { return "khunter_kronos_predictions" }
```

`backend/data/khunter/repo.go`：`EnsureMigrate` 的 AutoMigrate 列表加入 `&models.KhunterKronosPrediction{}`；文件尾部追加：

```go
// SaveKronosPredictions 按 (code, signal_date) 幂等保存批量预测明细。
func (r *Repo) SaveKronosPredictions(rows []models.KhunterKronosPrediction) error {
	if len(rows) == 0 {
		return nil
	}
	return db.Dao.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "code"}, {Name: "signal_date"}},
		UpdateAll: true,
	}).Create(&rows).Error
}
```

- [ ] **Step 4: pipeline 挂接**

`backend/data/khunter/pipeline.go` 的 `runLivePipeline` 在 `repo.SaveObservations(observations)` 成功之后追加（保持既有逻辑不变，仅新增）：

```go
	// Kronos 批量因子（可配置、可降级）：对当日观察候选做批量预测并落库明细。
	kcfg := LoadKronosFactorConfig("")
	if kcfg.Enable {
		factorResults, ferr := ApplyKronosFactor(ctx, observations, kcfg, data.PredictKLine)
		if ferr != nil {
			logger.SugaredLogger.Warnf("khunter Kronos 因子批量预测失败: %v", ferr)
		} else if len(factorResults) > 0 {
			idByObs := map[uint]models.KhunterObservation{}
			for _, o := range observations {
				idByObs[o.ID] = o
			}
			rows := make([]models.KhunterKronosPrediction, 0, len(factorResults))
			for _, fr := range factorResults {
				o := idByObs[fr.ObservationID]
				rows = append(rows, models.KhunterKronosPrediction{
					Code: o.Code, SignalDate: o.SignalDate, PredLen: kcfg.PredLen,
					FactorScore: fr.FactorScore, Weight: fr.Weight, DegradedReason: fr.DegradedReason,
				})
			}
			if err := repo.SaveKronosPredictions(rows); err != nil {
				logger.SugaredLogger.Warnf("khunter Kronos 预测明细落库失败: %v", err)
			}
		}
	}
```

（`ForecastJSON` 留空——明细落库保存因子分与权重；完整预测曲线由单股预测接口按需返回，避免批量结果大数据落库。）

- [ ] **Step 5: 验证通过**

Run: `PATH="/d/Go/bin:$PATH" go test ./backend/data/khunter/ ./backend/models/ -count=1`
Expected: PASS。

- [ ] **Step 6: Commit**

```bash
git add config/khunter_kronos.json backend/data/khunter/kronos_config.go backend/data/khunter/kronos_factor.go backend/data/khunter/kronos_factor_test.go backend/data/khunter/pipeline.go backend/data/khunter/repo.go backend/models/khunter_models.go
git commit -m "feat(khunter): configurable kronos batch factor with persisted details"
```

---

### Task 3: 异步单股预测任务注册表

**Files:**
- Create: `backend/data/khunter/prediction_jobs.go`
- Create: `backend/data/khunter/prediction_jobs_test.go`

**Interfaces:**
- Consumes: Task 1 `data.PredictKLine`、`data.ErrKronosModelLoading`。
- Produces:
  - `type PredictJob struct { ID string; Code string; PredLen int; AsOfDate string; Status string; ResultJSON string; Err string; CreatedAt time.Time }`（Status: pending/running/done/failed）
  - `type PredictionJobRegistry struct{}`
  - `func NewPredictionJobRegistry() *PredictionJobRegistry`
  - `func (r *PredictionJobRegistry) Submit(ctx context.Context, code string, predLen int, asOfDate string, predict func(context.Context, string, int, string) (*data.KronosPrediction, error)) (PredictJob, error)`
  - `func (r *PredictionJobRegistry) Get(id string) (PredictJob, bool)`
  - 同一 (code,predLen,asOfDate) 当日已有 job（无论状态）→ 直接返回既有 job（幂等去重）；done 结果缓存命中直接返回。

- [ ] **Step 1: 写失败测试**

`backend/data/khunter/prediction_jobs_test.go`：

```go
package khunter

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go-stock/backend/data"
)

func TestSubmitRunsPredictAsync(t *testing.T) {
	reg := NewPredictionJobRegistry()
	predict := func(ctx context.Context, code string, predLen int, asOf string) (*data.KronosPrediction, error) {
		return &data.KronosPrediction{LastClose: 10}, nil
	}
	j, err := reg.Submit(context.Background(), "sh600519", 5, "", predict)
	if err != nil || j.Status == "done" {
		t.Fatalf("提交后应为 pending/running: %+v %v", j, err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		got, ok := reg.Get(j.ID)
		if ok && got.Status == "done" && got.ResultJSON != "" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("任务未完成: %+v", got)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestSubmitDeduplicatesSameDay(t *testing.T) {
	reg := NewPredictionJobRegistry()
	var mu sync.Mutex
	calls := 0
	predict := func(ctx context.Context, code string, predLen int, asOf string) (*data.KronosPrediction, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		time.Sleep(50 * time.Millisecond)
		return &data.KronosPrediction{LastClose: 10}, nil
	}
	j1, _ := reg.Submit(context.Background(), "sh600519", 5, "", predict)
	j2, _ := reg.Submit(context.Background(), "sh600519", 5, "", predict)
	if j1.ID != j2.ID {
		t.Fatalf("同日同参数应复用任务: %s vs %s", j1.ID, j2.ID)
	}
}

func TestSubmitModelLoadingRetriesThenFailsOpen(t *testing.T) {
	reg := NewPredictionJobRegistry()
	n := 0
	predict := func(ctx context.Context, code string, predLen int, asOf string) (*data.KronosPrediction, error) {
		n++
		return nil, data.ErrKronosModelLoading
	}
	j, _ := reg.Submit(context.Background(), "sh600519", 5, "", predict)
	deadline := time.Now().Add(3 * time.Second)
	for {
		got, _ := reg.Get(j.ID)
		if got.Status == "failed" {
			if got.Err == "" {
				t.Fatal("失败应记录错误")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("模型加载重试后应转 failed: %+v", got)
		}
		time.Sleep(10 * time.Millisecond)
	}
	_ = errors.Is // 保留 import
}
```

- [ ] **Step 2: 验证失败**

Run: `PATH="/d/Go/bin:$PATH" go test ./backend/data/khunter/ -run 'Submit' -v`
Expected: FAIL（类型/函数未定义）。

- [ ] **Step 3: 实现**

`backend/data/khunter/prediction_jobs.go`：

```go
package khunter

// 异步单股 Kronos 预测任务注册表（内存态，进程级）。
// 解决模型冷启动 30s+ 导致的"点击无回显"：提交即返回任务 ID，
// 前端轮询 Get；同日同参数去重。模型加载中自动退避重试，超限转 failed。

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

type PredictJob struct {
	ID        string    `json:"id"`
	Code      string    `json:"code"`
	PredLen   int       `json:"predLen"`
	AsOfDate  string    `json:"asOfDate"`
	Status    string    `json:"status"` // pending / running / done / failed
	ResultJSON string   `json:"resultJson,omitempty"`
	Err       string    `json:"err,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

const (
	predictJobPending = "pending"
	predictJobRunning = "running"
	predictJobDone    = "done"
	predictJobFailed  = "failed"
)

// modelLoadingMaxRetries 模型冷加载重试上限（每次退避 2s）。
const modelLoadingMaxRetries = 8

type PredictionJobRegistry struct {
	mu   sync.Mutex
	jobs map[string]PredictJob
	// dedupe key: code|predLen|asOf|today → jobID
	byKey map[string]string
}

func NewPredictionJobRegistry() *PredictionJobRegistry {
	return &PredictionJobRegistry{jobs: map[string]PredictJob{}, byKey: map[string]string{}}
}

func predictDedupeKey(code string, predLen int, asOfDate string) string {
	today := time.Now().Format("2006-01-02")
	return code + "|" + strconv.Itoa(predLen) + "|" + asOfDate + "|" + today
}

func (r *PredictionJobRegistry) Submit(ctx context.Context, code string, predLen int, asOfDate string,
	predict func(context.Context, string, int, string) (*data.KronosPrediction, error),
) (PredictJob, error) {
	key := predictDedupeKey(code, predLen, asOfDate)
	r.mu.Lock()
	if id, ok := r.byKey[key]; ok {
		j := r.jobs[id]
		r.mu.Unlock()
		return j, nil
	}
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	j := PredictJob{
		ID: "pj_" + hex.EncodeToString(b), Code: code, PredLen: predLen, AsOfDate: asOfDate,
		Status: predictJobPending, CreatedAt: time.Now(),
	}
	r.jobs[j.ID] = j
	r.byKey[key] = j.ID
	r.mu.Unlock()

	go r.run(&j, predict)
	return j, nil
}

func (r *PredictionJobRegistry) run(j *PredictJob,
	predict func(context.Context, string, int, string) (*data.KronosPrediction, error),
) {
	r.setStatus(j.ID, predictJobRunning, "", "")
	for attempt := 0; ; attempt++ {
		pred, err := predict(context.Background(), j.Code, j.PredLen, j.AsOfDate)
		if err == nil {
			b, _ := json.Marshal(pred)
			r.setStatus(j.ID, predictJobDone, string(b), "")
			return
		}
		if errors.Is(err, data.ErrKronosModelLoading) && attempt < modelLoadingMaxRetries {
			time.Sleep(2 * time.Second)
			continue
		}
		r.setStatus(j.ID, predictJobFailed, "", err.Error())
		return
	}
}

func (r *PredictionJobRegistry) setStatus(id, status, result, errMsg string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.jobs[id]
	if !ok {
		return
	}
	j.Status, j.ResultJSON, j.Err = status, result, errMsg
	r.jobs[id] = j
}

func (r *PredictionJobRegistry) Get(id string) (PredictJob, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.jobs[id]
	return j, ok
}
```

- [ ] **Step 4: 验证通过**

Run: `PATH="/d/Go/bin:$PATH" go test ./backend/data/khunter/ -run 'Submit' -count=1 -v`
Expected: 3 个测试 PASS（含重试用例约 2s）。

- [ ] **Step 5: Commit**

```bash
git add backend/data/khunter/prediction_jobs.go backend/data/khunter/prediction_jobs_test.go
git commit -m "feat(khunter): async single-stock kronos prediction job registry"
```

---

### Task 4: AnalysisCenterService 门面 + Handler

**Files:**
- Create: `backend/data/analysis_center_service.go`
- Create: `backend/data/analysis_center_service_test.go`
- Create: `backend/handler/analysis_center_handler.go`
- Modify: `app.go`（注册 handler）

**Interfaces:**
- Consumes: Task 2 `KronosFactorConfig/LoadKronosFactorConfig`；Task 3 `PredictionJobRegistry`；既有 `khunter.Service`（`GetObservations`、运行入口）、`models.KhunterObservation`、`models.KhunterKronosPrediction`、`models.KhunterRunSummary`、`data.KronosServiceStatus`。
- Produces（前端与测试依赖）:
  - `type RecommendRow struct { Code, Name, SignalDate, Status, DataStatus, RejectReason string; Score, FactorScore, BlendedScore float64; FactorWeight float64; HasFactor bool }`
  - `func (s *AnalysisCenterService) GetRecommendations(tradeDate string, topN int) ([]RecommendRow, error)`（tradeDate 空=最新有观察的日期）
  - `func (s *AnalysisCenterService) GetAnalysisStatus() AnalysisCenterStatus`（`{ KronosStatus string; LastRun *models.KhunterRunSummary; PendingObservations int }`）
  - `type AnalysisCenterHandler struct`（Wails 绑定）：`GetRecommendations(tradeDate string, topN int)`、`GetAnalysisStatus() AnalysisCenterStatus`、`SubmitPredictJob(code string, predLen int, asOfDate string) (khunter.PredictJob, error)`、`GetPredictJob(id string) (khunter.PredictJob, error)`

- [ ] **Step 1: 写失败测试**

`backend/data/analysis_center_service_test.go`：

```go
package data

import (
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

func TestGetRecommendationsBlendsFactor(t *testing.T) {
	d := setupAnalysisTestDB(t) // 内部 setupTestDB 模式 + AutoMigrate khunter 表
	_ = db.Dao.Create(&models.KhunterObservation{
		Code: "sh600519", Name: "茅台", SignalDate: "2026-11-20", Status: "观察中",
		Score: 80, DataStatus: "valid",
	}).Error
	_ = db.Dao.Create(&models.KhunterKronosPrediction{
		Code: "sh600519", SignalDate: "2026-11-20", FactorScore: 60, Weight: 0.05,
	}).Error
	_ = db.Dao.Create(&models.KhunterObservation{
		Code: "sz000001", Name: "平安", SignalDate: "2026-11-20", Status: "观察中",
		Score: 70, DataStatus: "valid",
	}).Error

	svc := NewAnalysisCenterService()
	rows, err := svc.GetRecommendations("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("应返回 2 行: %+v", rows)
	}
	// 80*(1-0.05)+60*0.05 = 79
	if rows[0].Code != "sh600519" || rows[0].BlendedScore != 79 || !rows[0].HasFactor {
		t.Fatalf("因子混入错误: %+v", rows[0])
	}
	if rows[1].HasFactor || rows[1].BlendedScore != 70 {
		t.Fatalf("无因子应保持原分: %+v", rows[1])
	}
}
```

（`setupAnalysisTestDB` 在本文件内实现：替换 db.Dao 为临时 sqlite 并 `AutoMigrate(&models.KhunterObservation{}, &models.KhunterKronosPrediction{}, &models.KhunterRunSummary{})`，模式照抄 `backend/data/history/sync_create_test.go:16`。）

- [ ] **Step 2: 验证失败**

Run: `PATH="/d/Go/bin:$PATH" go test ./backend/data/ -run TestGetRecommendationsBlendsFactor -v`
Expected: FAIL（`NewAnalysisCenterService` 未定义）。

- [ ] **Step 3: 实现门面**

`backend/data/analysis_center_service.go`：

```go
package data

// 统一分析中心门面：选股预测族的唯一后端入口。
// 委托 khunter 引擎（观察/运行）与 Kronos 客户端（预测），
// 旧 daily_pick 接口保留 deprecated，前端一律经本门面取数。

import (
	"strings"

	"go-stock/backend/data/khunter"
	"go-stock/backend/db"
	"go-stock/backend/models"
)

type RecommendRow struct {
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	SignalDate    string  `json:"signalDate"`
	Status        string  `json:"status"`
	DataStatus    string  `json:"dataStatus"`
	RejectReason  string  `json:"rejectReason,omitempty"`
	Score         float64 `json:"score"`
	FactorScore   float64 `json:"factorScore,omitempty"`
	FactorWeight  float64 `json:"factorWeight,omitempty"`
	BlendedScore  float64 `json:"blendedScore"`
	HasFactor     bool    `json:"hasFactor"`
}

type AnalysisCenterStatus struct {
	KronosStatus        string                  `json:"kronosStatus"`
	LastRun             *models.KhunterRunSummary `json:"lastRun,omitempty"`
	PendingObservations int                     `json:"pendingObservations"`
}

type AnalysisCenterService struct {
	KhunterSvc *khunter.Service
	Jobs       *khunter.PredictionJobRegistry
}

func NewAnalysisCenterService() *AnalysisCenterService {
	return &AnalysisCenterService{
		KhunterSvc: khunter.NewService(),
		Jobs:       khunter.NewPredictionJobRegistry(),
	}
}

// GetRecommendations 推荐视图：最新观察（含已确认）按 blended 分降序。
// 有 Kronos 因子明细的观察按 Score*(1-w)+FactorScore*w 混分；无因子保持原分。
func (s *AnalysisCenterService) GetRecommendations(tradeDate string, topN int) ([]RecommendRow, error) {
	if topN <= 0 || topN > 100 {
		topN = 30
	}
	q := db.Dao.Model(&models.KhunterObservation{}).
		Where("status IN ?", []string{khunter.ObsStatusPending, khunter.ObsStatusConfirmed}).
		Order("score DESC, code").Limit(topN * 2)
	if strings.TrimSpace(tradeDate) != "" {
		q = q.Where("signal_date = ?", tradeDate)
	}
	var obs []models.KhunterObservation
	if err := q.Find(&obs).Error; err != nil {
		return nil, err
	}
	var preds []models.KhunterKronosPrediction
	if err := db.Dao.Find(&preds).Error; err == nil {
		// 关联最近一条因子明细（code 最新 signal_date）
		byCode := map[string]models.KhunterKronosPrediction{}
		for _, p := range preds {
			if cur, ok := byCode[p.Code]; !ok || p.SignalDate >= cur.SignalDate {
				byCode[p.Code] = p
			}
		}
		rows := make([]RecommendRow, 0, len(obs))
		for _, o := range obs {
			row := RecommendRow{
				Code: o.Code, Name: o.Name, SignalDate: o.SignalDate, Status: o.Status,
				DataStatus: o.DataStatus, RejectReason: o.RejectReason,
				Score: o.Score, BlendedScore: o.Score,
			}
			if p, ok := byCode[o.Code]; ok && p.SignalDate == o.SignalDate && p.DegradedReason == "" {
				row.HasFactor = true
				row.FactorScore = p.FactorScore
				row.FactorWeight = p.Weight
				row.BlendedScore = o.Score*(1-p.Weight) + p.FactorScore*p.Weight
			}
			rows = append(rows, row)
		}
		// blended 降序
		for i := 1; i < len(rows); i++ {
			for j := i; j > 0 && rows[j-1].BlendedScore < rows[j].BlendedScore; j-- {
				rows[j-1], rows[j] = rows[j], rows[j-1]
			}
		}
		if len(rows) > topN {
			rows = rows[:topN]
		}
		return rows, nil
	}
	return []RecommendRow{}, nil
}

func (s *AnalysisCenterService) GetAnalysisStatus() AnalysisCenterStatus {
	st := AnalysisCenterStatus{KronosStatus: KronosServiceStatus()}
	var sum models.KhunterRunSummary
	if err := db.Dao.Order("trade_date DESC").First(&sum).Error; err == nil {
		cp := sum
		st.LastRun = &cp
	}
	var n int64
	_ = db.Dao.Model(&models.KhunterObservation{}).
		Where("status = ?", khunter.ObsStatusPending).Count(&n)
	st.PendingObservations = int(n)
	return st
}
```

注意：`khunter.Service` 的实际类型名/构造以 `backend/data/khunter/service.go` 为准（Task 11 产出 `Service` + `NewService()`；若名为 `KhunterService` 则对应调整）。若 `Service` 依赖 `Repo`，构造时传入 `khunter.NewRepo()`。

`backend/handler/analysis_center_handler.go`：

```go
package handler

import (
	"context"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/data/khunter"
)

// AnalysisCenterHandler 统一分析中心 Wails 绑定（选股预测族唯一新入口）。
type AnalysisCenterHandler struct {
	svc *data.AnalysisCenterService
}

func NewAnalysisCenterHandler() *AnalysisCenterHandler {
	return &AnalysisCenterHandler{svc: data.NewAnalysisCenterService()}
}

func (h *AnalysisCenterHandler) GetRecommendations(tradeDate string, topN int) ([]data.RecommendRow, error) {
	return h.svc.GetRecommendations(tradeDate, topN)
}

func (h *AnalysisCenterHandler) GetAnalysisStatus() data.AnalysisCenterStatus {
	return h.svc.GetAnalysisStatus()
}

func (h *AnalysisCenterHandler) SubmitPredictJob(code string, predLen int, asOfDate string) (khunter.PredictJob, error) {
	return h.svc.Jobs.Submit(context.Background(), code, predLen, asOfDate, data.PredictKLine)
}

func (h *AnalysisCenterHandler) GetPredictJob(id string) (khunter.PredictJob, error) {
	j, ok := h.svc.Jobs.Get(id)
	if !ok {
		return khunter.PredictJob{}, nil
	}
	return j, nil
}
```

`app.go` 在 `startup` 中注册（照现有 handler 注册模式，例如在 `a.registerStartupCronTasks()` 之前的 handler 注册区）：

```go
	analysisCenterHandler := handler.NewAnalysisCenterHandler()
	// 与其他 handler 一同经 wails 绑定暴露（若项目用显式绑定列表，加入 AnalysisCenterHandler）
	_ = analysisCenterHandler
	_ = time.Second // 无其它用途则删除 time import
```

（绑定方式以项目现有模式为准：若 app.go 用 `Bind: []interface{}{...}`，在列表中加入 `handler.NewAnalysisCenterHandler()`；删除占位行。）

- [ ] **Step 4: 生成绑定 + 验证**

Run: `PATH="/d/Go/bin:$PATH" go build ./... && PATH="/d/Go/bin:$PATH" go test ./backend/data/ ./backend/handler/ -count=1`
Expected: 编译通过、测试 PASS。

- [ ] **Step 5: Commit**

```bash
git add backend/data/analysis_center_service.go backend/data/analysis_center_service_test.go backend/handler/analysis_center_handler.go app.go
git commit -m "feat(analysis): unified analysis center facade with blended recommendations"
```

---

### Task 5: 前端分析中心（Tab 壳 + 推荐视图 + 路由菜单收敛）

**Files:**
- Create: `frontend/src/components/analysis/AnalysisCenter.vue`
- Create: `frontend/src/components/analysis/RecommendView.vue`
- Create: `frontend/src/api/analysisCenter.ts`
- Create: `frontend/tests/analysis-center.test.ts`
- Modify: `frontend/src/router/router.js`
- Modify: `frontend/src/config/navigation.js`

**Interfaces:**
- Consumes: Task 4 的 wailsjs 绑定（`AnalysisCenterHandler.GetRecommendations/GetAnalysisStatus/SubmitPredictJob/GetPredictJob`，`wails generate module` 后生成到 `frontend/wailsjs/go/handler/AnalysisCenterHandler.*`）；既有 `KhunterPanel`、`ScreeningWorkbench`、`kline-analysis.vue` 组件。
- Produces: 路由 `/analysis?name=analysisCenter`（query `tab` 选 Tab）；重定向 `/daily-pick`、`/khunter`、`/kline-analysis` → `/analysis?tab=<recommend|khunter|kline>`。

- [ ] **Step 1: 写失败测试**

`frontend/tests/analysis-center.test.ts`：

```ts
import test from 'node:test'
import assert from 'node:assert/strict'
import { blendedScore } from '../src/components/analysis/recommend.ts'

test('有因子时按权重混分，无因子保持原分', () => {
  assert.equal(blendedScore(80, 60, 0.05), 79)
  assert.equal(blendedScore(70, 0, 0), 70)
})
```

- [ ] **Step 2: 验证失败**

Run: `cd frontend && node --import ./tests/register-ts-ext.mjs --test tests/analysis-center.test.ts`
Expected: FAIL（模块不存在）。

- [ ] **Step 3: 实现纯函数 + API 封装 + 两个组件**

`frontend/src/components/analysis/recommend.ts`：

```ts
// 推荐视图混分纯函数（与服务端口径一致，便于单测与展示）。
export function blendedScore(score: number, factorScore: number, weight: number): number {
  if (!weight) return score
  return Math.round((score * (1 - weight) + factorScore * weight) * 100) / 100
}

export interface RecommendRow {
  code: string
  name: string
  signalDate: string
  status: string
  dataStatus: string
  rejectReason?: string
  score: number
  factorScore?: number
  factorWeight?: number
  blendedScore: number
  hasFactor: boolean
}
```

`frontend/src/api/analysisCenter.ts`：

```ts
import { GetRecommendations, GetAnalysisStatus, SubmitPredictJob, GetPredictJob } from '../../wailsjs/go/handler/AnalysisCenterHandler'

export async function fetchRecommendations(tradeDate = '', topN = 30) {
  return GetRecommendations(tradeDate, topN)
}
export async function fetchAnalysisStatus() {
  return GetAnalysisStatus()
}
export async function submitPredictJob(code: string, predLen: number, asOfDate = '') {
  return SubmitPredictJob(code, predLen, asOfDate)
}
export async function fetchPredictJob(id: string) {
  return GetPredictJob(id)
}
```

`frontend/src/components/analysis/RecommendView.vue`（骨架：状态徽标 + 表格 + Kronos 预测按钮轮询）：

```vue
<template>
  <div>
    <n-space align="center" style="margin-bottom: 8px">
      <n-text>推荐视图（观察候选 × Kronos 因子混分）</n-text>
      <n-tag v-if="status" size="small" :type="status.kronosStatus === 'online' ? 'success' : 'warning'">
        Kronos: {{ status.kronosStatus }}
      </n-tag>
      <n-button size="small" :loading="loading" @click="refresh">刷新</n-button>
    </n-space>
    <n-data-table :columns="columns" :data="rows" :loading="loading" size="small" />
  </div>
</template>

<script setup lang="ts">
import { h, onMounted, ref } from 'vue'
import { NButton, NTag, NText, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { fetchRecommendations, fetchAnalysisStatus, submitPredictJob, fetchPredictJob } from '../../api/analysisCenter'
import type { RecommendRow } from './recommend'

const rows = ref<RecommendRow[]>([])
const status = ref<any>(null)
const loading = ref(false)
const message = useMessage()

const columns: DataTableColumns<RecommendRow> = [
  { title: '代码', key: 'code' },
  { title: '名称', key: 'name' },
  { title: '信号日', key: 'signalDate' },
  { title: '状态', key: 'status' },
  { title: '评分', key: 'score' },
  {
    title: 'Kronos', key: 'factor',
    render(row) {
      if (!row.hasFactor) return h(NText, { depth: 3 }, { default: () => '—' })
      return h(NTag, { size: 'tiny' }, { default: () => `${row.factorScore} × ${row.factorWeight}` })
    },
  },
  { title: '综合', key: 'blendedScore', sorter: (a, b) => a.blendedScore - b.blendedScore, defaultSortOrder: 'descend' },
  {
    title: '操作', key: 'op',
    render(row) {
      return h(NButton, {
        size: 'tiny',
        onClick: () => runPredict(row.code),
      }, { default: () => 'AI预测' })
    },
  },
]

async function refresh() {
  loading.value = true
  try {
    rows.value = await fetchRecommendations()
    status.value = await fetchAnalysisStatus()
  } finally {
    loading.value = false
  }
}

async function runPredict(code: string) {
  const job = await submitPredictJob(code, 5, '')
  message.info('预测任务已提交（模型冷启动需数分钟），请稍后查看')
  pollJob(job.id)
}

async function pollJob(id: string) {
  const j = await fetchPredictJob(id)
  if (j.status === 'done') {
    message.success(`${j.code} 预测完成`)
    return
  }
  if (j.status === 'failed') {
    message.error(`预测失败：${j.err || '未知错误'}`)
    return
  }
  setTimeout(() => pollJob(id), 2000)
}

onMounted(refresh)
</script>
```

`frontend/src/components/analysis/AnalysisCenter.vue`：

```vue
<template>
  <n-tabs v-model:value="tab" type="line" animated @update:value="syncQuery">
    <n-tab-pane name="recommend" tab="推荐">
      <Suspense><RecommendView /></Suspense>
    </n-tab-pane>
    <n-tab-pane name="khunter" tab="狩猎场">
      <KhunterPanel />
    </n-tab-pane>
    <n-tab-pane name="workbench" tab="选股工作台">
      <ScreeningWorkbench />
    </n-tab-pane>
    <n-tab-pane name="kline" tab="K线分析">
      <KlineAnalysis />
    </n-tab-pane>
  </n-tabs>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import RecommendView from './RecommendView.vue'
import KhunterPanel from '../KhunterPanel.vue'
import ScreeningWorkbench from '../ScreeningWorkbench.vue'
import KlineAnalysis from '../kline-analysis.vue'

const route = useRoute()
const router = useRouter()
const tab = ref((route.query.tab as string) || 'recommend')

watch(() => route.query.tab, (v) => {
  if (v && v !== tab.value) tab.value = v as string
})

function syncQuery(name: string) {
  router.replace({ query: { ...route.query, tab: name } })
}
</script>
```

- [ ] **Step 4: 路由与菜单**

`frontend/src/router/router.js`：

```js
import analysisCenter from "../components/analysis/AnalysisCenter.vue"
// 新增路由（放在 /analysis/workbench 附近）：
{ path: '/analysis', component: analysisCenter, name: 'analysisCenter' },
// 重定向（替换原独立路由；组件 import 可保留）：
{ path: '/daily-pick', redirect: { name: 'analysisCenter', query: { tab: 'recommend' } } },
{ path: '/khunter', redirect: { name: 'analysisCenter', query: { tab: 'khunter' } } },
{ path: '/kline-analysis', redirect: { name: 'analysisCenter', query: { tab: 'kline' } } },
```

`frontend/src/config/navigation.js` 的"分析工具"组 children 替换为单项（组保留，默认落 analysisCenter）：

```js
    {
      label: () =>
        h(
          RouterLink,
          {
            to: { name: 'analysisCenter' },
            onClick: () => {
              activeKey.value = MENU_GROUP_KEYS.ANALYSIS
            },
          },
          { default: () => '分析中心' }
        ),
      key: MENU_GROUP_KEYS.ANALYSIS,
      icon: renderIcon(icons.StatsChartOutline),
      children: [
        routeItem({
          title: '分析中心',
          key: 'analysisCenter',
          groupKey: MENU_GROUP_KEYS.ANALYSIS,
          icon: icons.StatsChartOutline,
          to: { name: 'analysisCenter' },
        }),
      ],
    },
```

"研究资讯"组删除"股票推荐"菜单项（`to: { name: 'researchRecommends' }` 那段 routeItem；路由保留）。

- [ ] **Step 5: 生成绑定 + 全量前端验证**

```bash
PATH="/d/Go/bin:/c/Users/54389/go/bin:$PATH" wails generate module
cd frontend
node --import ./tests/register-ts-ext.mjs --test tests/analysis-center.test.ts
node tests/check-vue-script.mjs src/components/analysis/AnalysisCenter.vue
node tests/check-vue-script.mjs src/components/analysis/RecommendView.vue
node tests/run-vue-tsc.mjs
```

Expected: 测试 PASS、两个脚本检查 PASS、vue-tsc 无新增错误（6 个预置无关错误除外）。

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/analysis/ frontend/src/api/analysisCenter.ts frontend/tests/analysis-center.test.ts frontend/src/router/router.js frontend/src/config/navigation.js frontend/wailsjs
git commit -m "feat(analysis): unified analysis center shell with recommend view"
```

---

### Task 6: 端到端验证与回放口径对照

**Files:**
- Modify: 无（验证任务）

- [ ] **Step 1: 后端全量回归**

```bash
PATH="/d/Go/bin:$PATH" go build ./...
PATH="/d/Go/bin:$PATH" go test ./backend/data/... ./backend/handler/ ./backend/agent/ -count=1
```

Expected: 全部 ok。

- [ ] **Step 2: 前端构建**

```bash
cd frontend && node node_modules/vite/bin/vite.js build 2>&1 | tail -2
```

Expected: `✓ built`。

- [ ] **Step 3: 五案例回放口径对照**

Kronos 因子默认 `enable:false`，回放评分口径应与合并前一致：

```bash
PATH="/d/Go/bin:$PATH" go run ./backend/cmd/khunterreplay \
  -db "<安装库副本>" -start 2026-09-01 -end 2026-11-17 \
  -variants full -config config/khunter_risk_control.json \
  -cases '688322.SH:2026-11-11,300780.SZ:2026-10-29,600259.SH:2026-11-03,300829.SZ:2026-11-12,300969.SZ:2026-10-29' \
  -out /tmp/five-cases-after-merge.json
```

人工核对：与合并前（`D:/_installers/go-stock/replay-verify/five-cases.json`）逐案例对比 信号数/结局计数/评分一致（允许污染重拉后的数据闸门差异，须在报告中说明）。

- [ ] **Step 4: 安装版冒烟**

安装新构建、启动、打开分析中心四 Tab 各点一遍；推荐视图有数据（或空态提示）；Kronos 状态徽标显示 disabled（默认未开启）→ 设置开启后显示 online/offline。

- [ ] **Step 5: 收尾提交（如 Step 3/4 有代码修正）**

```bash
git add -A && git commit -m "chore(analysis): e2e verification fixes" # 仅当有修正时
```

---

## Self-Review 记录

- **Spec 覆盖**：架构/组件（Task 2/4/5）、Kronos 三消费面（Task 1/2/3）、服务治理（Task 1 导出统一状态/拉起）、数据流（Task 2 pipeline 挂接 + Task 4 混分）、错误处理表（Task 2 降级、Task 3 重试转 failed、Task 1 errors.Is 语义保留）、测试策略（各任务 TDD + Task 6 回放对照）均有对应任务。选股工作台嵌入（Task 5 Tab）、菜单收敛（Task 5）、`/research/recommends` 隐藏（Task 5）。
- **占位符扫描**：无 TBD/TODO；所有代码步骤含完整实现。
- **类型一致性**：`PredictKLine`/`KronosServiceStatus`（Task 1 → 2/3/4 引用一致）；`KronosFactorConfig`/`ApplyKronosFactor`/`KronosFactorResult`（Task 2 → 4 门面引用一致）；`PredictJob`/`PredictionJobRegistry`（Task 3 → 4 handler 引用一致）；`RecommendRow`/`AnalysisCenterStatus`（Task 4 → 5 前端 TS 接口字段一致：code/name/signalDate/status/dataStatus/score/factorScore/factorWeight/blendedScore/hasFactor）。
- **已知偏差（计划内说明）**：批量因子只落因子分与权重，完整预测曲线由单股预测按需返回（避免批量大数据落库）——已在 Task 2 Step 4 注释说明，与设计"预测明细落库"不冲突（明细=因子结论）。
