# 统一分析中心（第一阶段：选股预测族）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 选股预测族工具（每日选股/狩猎场/K线分析/选股工作台/Kronos）合并为统一分析中心：khunter 为推荐入口的唯一选股引擎，Kronos 成为统一预测服务，前端单页四 Tab。

**Architecture:** 后端新增 AnalysisCenterService 门面（委托 khunter 引擎与 Kronos 客户端）+ khunter Kronos 排序因子模块（`/predict_batch` 异步、落库明细、可配置、可降级）+ 异步单股预测任务注册表；前端新增 AnalysisCenter.vue Tab 壳（推荐/狩猎场/选股工作台/K线分析），旧路由用路由 redirect（hash 路由无 301），菜单收敛。旧 daily_pick 引擎、cron 与接口第一阶段全部保留（复盘/调权唯一数据源），下一阶段删除。

**Revision:** 2026-11-22 第二轮评审后的修订版，与设计文档「修订记录」一一对应。Task 1 已在工作区完成（未提交），按 Step 4 起执行。

**Tech Stack:** Go、GORM、SQLite、Wails 2、Vue 3 + Naive UI、Node 内置测试（`node --import ./tests/register-ts-ext.mjs --test`）。

**Spec:** `docs/superpowers/specs/2026-11-22-analysis-center-design.md`

## Global Constraints

- 不修改 khunter 引擎既有链路语义（时点约束/数据闸门/风险过滤/确认/退出），**不修改五维评分公式与观察入选阈值**；Kronos 因子只进推荐视图排序分，默认关闭（`enable:false`）。
- Kronos 服务不可用时一律降级（交互面任务挂起、排序面中性分+`HasFactor=false`），不得让预测故障阻塞选股主链；批量预测必须**异步**且不阻塞确认/退出引擎。
- 旧接口响应格式不变（`TradingRecordHandler` 的 Kronos 方法保持签名；`DailyPickHandler` 第一阶段**仍直连旧引擎**，不转调门面，方法与返回结构不变）。
- daily_pick 的 cron（15:30 选股、09:15 复盘）与结果表写入**第一阶段保留**，不得停写（复盘/自动调权唯一数据源）。
- 前端测试命令统一为 `cd frontend && node --import ./tests/register-ts-ext.mjs --test <file>`；类型检查 `node tests/run-vue-tsc.mjs`。仓库无 `@vue/test-utils`/jsdom，**不写组件挂载测试**。
- 提交信息用 Conventional Commits；每任务一次提交。

**导入关系（已验证无环）**：`backend/data/khunter` 已 import `backend/data`（event_classify.go 等）；`backend/data` 不 import khunter 根包。门面放 `backend/data` 可直接调 khunter。

**既有代码事实（实现时以此为准，勿照抄旧稿）**：

- `KronosPrediction` 只有 `Bars []*KronosBar` 与 `Summary *KronosForecast`（`backend/data/kronos_service.go:51`），**没有** `LastClose`/`Forecasts` 字段。
- `KronosBatchPredict(ctx, apiCodes []string, predLen int)` 走 `/predict_batch`，返回 `(results map[string]*KronosPrediction, errs map[string]string, err error)`。
- `NormalizeKronosCode` 委托 `stockcode.Normalize`，进入预测前统一规范化。
- 门面注册点在 `main.go:264` 的 `Bind: []interface{}{...}` 列表（不是 `app.go` 的 startup）。
- `LoadTechnicalOverrides` 全仓无调用，`khunter_technical_weights.json` 的 overrides 当前未接线。

---

### Task 1: 导出 Kronos 客户端（data 包统一入口）

> **状态：已完成（工作区未提交）。** `PredictKLine`(kronos_service.go:957)、`KronosServiceStatus`(:966)、`EnsureKronosRunning`(:974) 已实现；`trading_handler.go` 五个方法已改委托并标注 Deprecated；`kronos_service_test.go` 已有三个测试。**从本任务起只需 Review + 补提交，不要重跑 TDD 步骤。**

**Files:**
- Modify: `backend/data/kronos_service.go`（已改）
- Modify: `backend/handler/trading_handler.go`（已改，签名不变）
- Test: `backend/data/kronos_service_test.go`（已扩展）

**Interfaces（已落地，后续任务按此签名调用）:**
- `func PredictKLine(ctx context.Context, stockCode string, predLen int, asOfDate string) (*KronosPrediction, error)`（内部先 `NormalizeKronosCode`）
- `func KronosServiceStatus() string`（`disabled` / `offline` / `loading` / `online`）
- `func EnsureKronosRunning(ctx context.Context, timeout time.Duration) error`

- [x] **Step 1-4: 实现 + 改委托**（已完成）

- [ ] **Step 5: 验证**

Run: `PATH="/d/Go/bin:$PATH" go test ./backend/data/ -run 'TestKronosServiceStatusDisabledByDefault|TestPredictKLineDisabledByDefault|TestEnsureKronosRunningDisabledByDefault' -v`
Expected: 三个测试 PASS。

- [ ] **Step 6: Commit（连同工作区既有改动一起提交）**

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
- Consumes: Task 1 的 `data.PredictKLine` / `data.KronosBatchPredict`；既有 `models.KhunterObservation`、`Repo`。
- Produces:
  - `type KronosFactorConfig struct { Enable bool; TopN int; PredLen int; Weight float64; MaxWeight float64 }`
  - `func DefaultKronosFactorConfig() KronosFactorConfig`（Enable:false, TopN:30, PredLen:5, Weight:0.05, MaxWeight:0.15）
  - `func LoadKronosFactorConfig(path string) KronosFactorConfig`（缺文件/解析失败回退默认；相对路径回退到可执行文件目录，同 `resolveRiskControlConfigPath`）
  - `type KronosFactorResult struct { ObservationID uint; Code string; FactorScore float64; Weight float64; ForecastJSON string; DegradedReason string }`
  - `func KronosFactorScore(pred *data.KronosPrediction) float64`（直接用 `Summary.ChangePct` 与 `Summary.Confidence`，与 `daily_pick_kronos.kronosFactorScore` 同口径）
  - `func ApplyKronosFactor(ctx context.Context, observations []models.KhunterObservation, cfg KronosFactorConfig, predict func(ctx context.Context, apiCodes []string, predLen int) (map[string]*data.KronosPrediction, map[string]string, error)) ([]KronosFactorResult, error)`
  - 模型 `models.KhunterKronosPrediction{Code, SignalDate, PredLen, ForecastJSON, FactorScore, Weight, DegradedReason}`（唯一键 code+signal_date）
  - `func (r *Repo) SaveKronosPredictions(rows []models.KhunterKronosPrediction) error`

**关键约束**：`KronosPrediction` 实际只有 `Bars []*KronosBar` 与 `Summary *KronosForecast`（无 `LastClose`/`Forecasts`）。因子分必须走
`Summary.ChangePct` + `Summary.Confidence`，与 `daily_pick_kronos.go:26` 的既有口径一致，保证历史统计可比。代码内部先 `data.NormalizeKronosCode` 规范化再批量提交。

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
```

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
	"path/filepath"
)

// KronosFactorConfig Kronos 排序因子配置（config/khunter_kronos.json）。
// 默认 enable=false：预测质量经滚动回测验证后再开启。
// 该权重只作用于推荐视图排序分，不进入五维评分，与 khunter_technical_weights.json 无关。
type KronosFactorConfig struct {
	Enable    bool    `json:"enable"`
	TopN      int     `json:"topN"`      // 参与批量预测的观察候选上限（CPU 推理成本约束）
	PredLen   int     `json:"predLen"`   // 预测天数
	Weight    float64 `json:"weight"`    // 推荐视图混分权重
	MaxWeight float64 `json:"maxWeight"` // 权重上限（防配置越界）
}

func DefaultKronosFactorConfig() KronosFactorConfig {
	return KronosFactorConfig{Enable: false, TopN: 30, PredLen: 5, Weight: 0.05, MaxWeight: 0.15}
}

const kronosFactorConfigRel = "config/khunter_kronos.json"

// resolveKronosFactorConfigPath 与 resolveRiskControlConfigPath 同款：先当前目录，
// 再可执行文件目录（安装版 cwd 不是程序目录）。
func resolveKronosFactorConfigPath(path string) string {
	if path != "" {
		return path
	}
	if _, err := os.Stat(kronosFactorConfigRel); err == nil {
		return kronosFactorConfigRel
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), kronosFactorConfigRel)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return kronosFactorConfigRel
}

func LoadKronosFactorConfig(path string) KronosFactorConfig {
	b, err := os.ReadFile(resolveKronosFactorConfigPath(path))
	if err != nil {
		return DefaultKronosFactorConfig()
	}
	cfg := DefaultKronosFactorConfig()
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
	if cfg.MaxWeight <= 0 || cfg.MaxWeight < cfg.Weight {
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
	"math"
	"sort"

	"go-stock/backend/data"
	"go-stock/backend/models"
)

// KronosFactorResult 单只观察的 Kronos 因子结论。
type KronosFactorResult struct {
	ObservationID  uint
	Code           string
	FactorScore    float64 // 0~100，失败时为中性 50
	Weight         float64 // 实际使用权重（cfg.Weight，钳位到 MaxWeight）
	ForecastJSON   string  // 完整预测序列化（KronosPrediction），降级时为空
	DegradedReason string  // 非空表示降级（服务离线/预测失败/结果异常）
}

// KronosFactorScore 预测摘要 → 0~100 因子分，与 daily_pick_kronos.kronosFactorScore 同口径：
// 预期涨幅按 ±10% 截断映射到 0~100（0%→50），再按一致度（Confidence/100）向中性 50 收缩。
// 口径一致的目的是让新链路与历史复盘命中率统计可比。
func KronosFactorScore(pred *data.KronosPrediction) float64 {
	if pred == nil || pred.Summary == nil {
		return 50
	}
	chg := pred.Summary.ChangePct
	if chg > 10 {
		chg = 10
	} else if chg < -10 {
		chg = -10
	}
	base := (chg + 10) / 20 * 100
	conf := pred.Summary.Confidence / 100
	if conf < 0 {
		conf = 0
	} else if conf > 1 {
		conf = 1
	}
	return math.Round((base*conf+50*(1-conf))*10) / 10
}

// BatchPredictFunc 批量预测注入点：生产传 data.KronosBatchPredict。
type BatchPredictFunc func(ctx context.Context, apiCodes []string, predLen int) (map[string]*data.KronosPrediction, map[string]string, error)

// ApplyKronosFactor 对观察候选按评分降序取 topN，用一次 /predict_batch 批量预测并折算因子分。
// 单只失败不中断批次；服务离线整体降级为中性分且返回 nil error——调用方（流水线）不得被预测故障阻塞。
func ApplyKronosFactor(ctx context.Context, observations []models.KhunterObservation,
	cfg KronosFactorConfig, predict BatchPredictFunc,
) ([]KronosFactorResult, error) {
	if !cfg.Enable || predict == nil || len(observations) == 0 {
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

	// 代码规范化 + 反查表：apiCode → 观察下标
	codes := make([]string, 0, len(byScore))
	idxByCode := make(map[string]int, len(byScore))
	for i, o := range byScore {
		api := data.NormalizeKronosCode(o.Code)
		if api == "" {
			continue
		}
		codes = append(codes, api)
		idxByCode[api] = i
	}
	out := make([]KronosFactorResult, 0, len(byScore))
	if len(codes) == 0 {
		return out, nil
	}

	preds, errs, err := predict(ctx, codes, cfg.PredLen)
	for _, o := range byScore {
		res := KronosFactorResult{ObservationID: o.ID, Code: o.Code, FactorScore: 50, Weight: w}
		api := data.NormalizeKronosCode(o.Code)
		switch {
		case err != nil:
			res.DegradedReason = err.Error()
		case errs[api] != "":
			res.DegradedReason = errs[api]
		case preds[api] == nil || preds[api].Summary == nil:
			res.DegradedReason = "预测结果缺 summary"
		default:
			res.FactorScore = KronosFactorScore(preds[api])
			if b, mErr := json.Marshal(preds[api]); mErr == nil {
				res.ForecastJSON = string(b)
			}
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
	ForecastJSON   string    `gorm:"type:text" json:"forecastJson"` // 完整预测（KronosPrediction 序列化）；降级时为空
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

- [ ] **Step 4: pipeline 挂接（异步、不阻塞确认）**

`backend/data/khunter/pipeline.go` 的 `runLivePipeline`：在 `res.Observed = len(observations)` 之后、第 8 步确认引擎**之前**启动 goroutine（确认/退出紧接着执行，绝不能被 30 只 × 数分钟的推理拖住）：

```go
	// 8. Kronos 排序因子（异步旁路）：不参与五维评分、不改观察名单，
	//    只在后台批量预测并落库明细，供推荐视图排序与展开。
	//    结果按 (code, signal_date) 关联，不依赖 upsert 后的自增 ID。
	if kcfg := LoadKronosFactorConfig(""); kcfg.Enable && len(observations) > 0 {
		go func(rows []models.KhunterObservation, byCode map[string]models.KhunterObservation, kc KronosFactorConfig) {
			defer func() {
				if r := recover(); r != nil {
					logger.SugaredLogger.Errorf("khunter Kronos 因子 panic: %v", r)
				}
			}()
			frs, ferr := ApplyKronosFactor(context.Background(), rows, kc, data.KronosBatchPredict)
			if ferr != nil {
				logger.SugaredLogger.Warnf("khunter Kronos 因子批量预测失败: %v", ferr)
				return
			}
			recs := make([]models.KhunterKronosPrediction, 0, len(frs))
			degraded := 0
			for _, fr := range frs {
				if fr.DegradedReason != "" {
					degraded++
				}
				signalDate := tradeDate
				if o, ok := byCode[fr.Code]; ok && o.SignalDate != "" {
					signalDate = o.SignalDate
				}
				recs = append(recs, models.KhunterKronosPrediction{
					Code: fr.Code, SignalDate: signalDate, PredLen: kc.PredLen,
					ForecastJSON: fr.ForecastJSON, FactorScore: fr.FactorScore,
					Weight: fr.Weight, DegradedReason: fr.DegradedReason,
				})
			}
			if err := repo.SaveKronosPredictions(recs); err != nil {
				logger.SugaredLogger.Warnf("khunter Kronos 预测明细落库失败: %v", err)
				return
			}
			logger.SugaredLogger.Infof("khunter Kronos 因子完成 %s: %d 只，降级 %d 只", tradeDate, len(frs), degraded)
		}(append([]models.KhunterObservation(nil), observations...), buildObsByCode(observations), kcfg)
	}
```

配套的小工具（同文件或 `kronos_factor.go`）：

```go
// buildObsByCode 观察按代码索引（goroutine 内按 code 关联 signal_date）。
func buildObsByCode(rows []models.KhunterObservation) map[string]models.KhunterObservation {
	out := make(map[string]models.KhunterObservation, len(rows))
	for _, o := range rows {
		out[o.Code] = o
	}
	return out
}
```

- **运行汇总**：因子异步完成，**不写 `KhunterRunSummary`**（该行在 `defer` 中已提交）；降级数只进日志与 `khunter_kronos_predictions.DegradedReason`，前端按 `HasFactor` 展示。
- **并发安全**：goroutine 只用局部副本与 `repo`（GORM 每次调用自取连接），不碰 `res`。

- [ ] **Step 5: 验证通过**

Run: `PATH="/d/Go/bin:$PATH" go test ./backend/data/khunter/ ./backend/models/ -count=1`
Expected: PASS（含 `TestApplyKronosFactorOfflineReturnsDegradedNotError`、`TestApplyKronosFactorTakesTopNAndBatchesOnce`）。
Run: `PATH="/d/Go/bin:$PATH" go vet ./backend/data/khunter/`
Expected: 无输出。

- [ ] **Step 6: Commit**

```bash
git add config/khunter_kronos.json backend/data/khunter/kronos_config.go backend/data/khunter/kronos_factor.go backend/data/khunter/kronos_factor_test.go backend/data/khunter/pipeline.go backend/data/khunter/repo.go backend/models/khunter_models.go
git commit -m "feat(khunter): async kronos ranking factor with persisted forecasts"
```

---

### Task 3: 异步单股预测任务注册表

**Files:**
- Create: `backend/data/khunter/prediction_jobs.go`
- Create: `backend/data/khunter/prediction_jobs_test.go`

**Interfaces:**
- Consumes: Task 1 `data.PredictKLine`、`data.ErrKronosModelLoading`。
- Produces:
  - `type PredictJob struct { ID, Code string; PredLen int; AsOfDate, Status, ResultJSON, Err string; CreatedAt, UpdatedAt time.Time }`（Status: pending/running/done/failed）
  - `type PredictionJobRegistry struct{}`；`func NewPredictionJobRegistry() *PredictionJobRegistry`
  - `func (r *PredictionJobRegistry) Submit(ctx context.Context, code string, predLen int, asOfDate string, predict PredictFunc) (PredictJob, error)`
  - `func (r *PredictionJobRegistry) Get(id string) (PredictJob, error)`，未知 ID 返回 `ErrPredictJobNotFound`（**不是零值 + nil**）
  - 去重键 `code|predLen|asOfDate|自然日`，**只对 pending/running/done 生效**；`failed` 不占键，可直接重试
  - 冷启动重试窗口对齐 warmup 上限（8 分钟），指数退避；超过才转 `failed`
  - TTL：done/failed 任务保留 24h，提交时顺带清理过期条目（常驻进程内存不膨胀）

**为什么这样改**：旧稿「无论状态都复用」会让用户在服务恢复后拿到同一个 failed 任务、永远失败；
旧稿 8×2s=16s 远短于 `kronosWarmup` 的 8 分钟，用户看到的是"失败"而不是"加载中"；
旧稿 `Get` 返回 `(零值, false)`，前端 `pollJob` 会无限轮询。

- [ ] **Step 1: 写失败测试**

`backend/data/khunter/prediction_jobs_test.go`：

```go
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
	got := waitStatus(t, reg, j.ID, predictJobDone, 2*time.Second)
	if got.ResultJSON == "" {
		t.Fatalf("done 必须带结果: %+v", got)
	}
}

func TestSubmitDeduplicatesInFlight(t *testing.T) {
	reg := NewPredictionJobRegistry()
	slow := okPredict(80*time.Millisecond)
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
	waitStatus(t, reg, j1.ID, predictJobFailed, 2*time.Second)

	j2, _ := reg.Submit(context.Background(), "sh600519", 5, "", okPredict(0))
	if j2.ID == j1.ID {
		t.Fatal("failed 不得占用去重键，重试必须拿到新任务")
	}
	waitStatus(t, reg, j2.ID, predictJobDone, 2*time.Second)
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
	got := waitStatus(t, reg, j.ID, predictJobFailed, 3*time.Second)
	if got.Err == "" {
		t.Fatal("失败应记录错误原因")
	}
}

func TestRegistryPrunesExpiredJobs(t *testing.T) {
	reg := NewPredictionJobRegistry()
	reg.ttl = 30 * time.Millisecond
	j, _ := reg.Submit(context.Background(), "sh600519", 5, "", okPredict(0))
	waitStatus(t, reg, j.ID, predictJobDone, 2*time.Second)
	time.Sleep(60 * time.Millisecond)
	reg.pruneLocked(time.Now())
	if _, err := reg.Get(j.ID); !errors.Is(err, ErrPredictJobNotFound) {
		t.Fatalf("过期任务应被清理: %v", err)
	}
}

```

- [ ] **Step 2: 验证失败**

Run: `PATH="/d/Go/bin:$PATH" go test ./backend/data/khunter/ -run 'Submit|GetUnknownID|ModelLoading|Prunes' -v`
Expected: FAIL（类型/函数未定义）。

- [ ] **Step 3: 实现**

`backend/data/khunter/prediction_jobs.go`：

```go
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
```

- [ ] **Step 4: 验证通过**

Run: `PATH="/d/Go/bin:$PATH" go test ./backend/data/khunter/ -run 'Submit|GetUnknownID|ModelLoading|Prunes' -count=1 -v`
Expected: 6 个测试 PASS（压缩窗口，秒级）。

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
- Modify: `app.go`（`App` 结构体加字段 + `newApp` 里构造，照 `dailyPickHandler` / `khunterHandler` 既有模式）
- Modify: `main.go`（**注册点**：`main.go:264` 的 `Bind: []interface{}{...}` 加一行 `app.analysisCenterHandler`；**`main.go` 当前未导入 `handler` 包**，所以不要在 Bind 里直接 `handler.New...()`）

**Interfaces:**
- Consumes: Task 2 `KronosFactorConfig/LoadKronosFactorConfig`、`models.KhunterKronosPrediction`；Task 3 `PredictionJobRegistry/PredictJob/ErrPredictJobNotFound`；既有 `models.KhunterObservation`、`models.KhunterRunSummary`、`data.KronosServiceStatus`。
- Produces（前端与测试依赖）:
  - `type RecommendRow struct { Code, Name, SignalDate, Status, DataStatus, RejectReason string; Score, FactorScore, BlendedScore, FactorWeight float64; HasFactor, Degraded bool; DegradedReason string }`
  - `func (s *AnalysisCenterService) GetRecommendations(tradeDate string, topN int) ([]RecommendRow, error)`（tradeDate 空=最新有观察的日期；**DB 错误必须返回 error**）
  - `func (s *AnalysisCenterService) GetAnalysisStatus() (AnalysisCenterStatus, error)`
  - `func (s *AnalysisCenterService) GetForecast(code, signalDate string) (string, error)`（返回该候选落库的 `ForecastJSON`，供推荐视图展开曲线）
  - Handler（Wails 绑定）：`GetRecommendations`、`GetAnalysisStatus`、`GetForecast`、`SubmitPredictJob`、`GetPredictJob`

**与旧稿的区别（必须遵守）**：

- 旧稿 `if err := db.Dao.Find(&preds).Error; err == nil {...}` 在查询失败时返回空列表 + nil error，前端显示"今天没候选"——**改为返回错误**。
- 旧稿 `GetPredictJob` 未命中返回零值 + nil，前端无限轮询——**改为返回 `ErrPredictJobNotFound`**。
- 旧稿 `app.go` 注册处是占位代码（`_ = analysisCenterHandler`）——**删除占位，直接进 `main.go` 的 Bind 列表**。
- `db.Dao` 可能为 nil（移动端），门面入口先判空。

- [ ] **Step 1: 写失败测试**

`backend/data/analysis_center_service_test.go`：

```go
package data

import (
	"errors"
	"testing"

	"go-stock/backend/data/khunter"
	"go-stock/backend/db"
	"go-stock/backend/models"
)

func TestGetRecommendationsBlendsFactor(t *testing.T) {
	setupAnalysisTestDB(t)
	_ = db.Dao.Create(&models.KhunterObservation{
		Code: "sh600519", Name: "茅台", SignalDate: "2026-11-20", Status: "观察中",
		Score: 80, DataStatus: "valid",
	}).Error
	_ = db.Dao.Create(&models.KhunterKronosPrediction{
		Code: "sh600519", SignalDate: "2026-11-20", FactorScore: 60, Weight: 0.05,
		ForecastJSON: `{"summary":{"direction":"up"}}`,
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

func TestGetRecommendationsExcludesDegradedFactor(t *testing.T) {
	setupAnalysisTestDB(t)
	_ = db.Dao.Create(&models.KhunterObservation{
		Code: "sh600519", Name: "茅台", SignalDate: "2026-11-20", Status: "观察中",
		Score: 80, DataStatus: "valid",
	}).Error
	_ = db.Dao.Create(&models.KhunterKronosPrediction{
		Code: "sh600519", SignalDate: "2026-11-20", FactorScore: 50, Weight: 0.05,
		DegradedReason: "KRONOS_OFFLINE",
	}).Error

	rows, err := NewAnalysisCenterService().GetRecommendations("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].HasFactor || rows[0].BlendedScore != 80 {
		t.Fatalf("降级因子不得混入排序分: %+v", rows)
	}
	if !rows[0].Degraded || rows[0].DegradedReason == "" {
		t.Fatalf("降级原因必须透出给前端: %+v", rows[0])
	}
}

func TestGetForecastReturnsPersistedJSON(t *testing.T) {
	setupAnalysisTestDB(t)
	_ = db.Dao.Create(&models.KhunterKronosPrediction{
		Code: "sh600519", SignalDate: "2026-11-20", FactorScore: 60, Weight: 0.05,
		ForecastJSON: `{"summary":{"direction":"up","changePct":4}}`,
	}).Error
	got, err := NewAnalysisCenterService().GetForecast("sh600519", "2026-11-20")
	if err != nil {
		t.Fatal(err)
	}
	if got == "" {
		t.Fatal("应返回落库的预测明细")
	}
}

func TestGetPredictJobUnknownIDReturnsError(t *testing.T) {
	svc := NewAnalysisCenterService()
	if _, err := svc.GetPredictJob("pj_nope"); !errors.Is(err, khunter.ErrPredictJobNotFound) {
		t.Fatalf("未知任务必须返回 ErrPredictJobNotFound: %v", err)
	}
}

func TestGetRecommendationsReturnsErrorWhenDaoNil(t *testing.T) {
	saved := db.Dao
	db.Dao = nil
	defer func() { db.Dao = saved }()
	if _, err := NewAnalysisCenterService().GetRecommendations("", 10); err == nil {
		t.Fatal("db.Dao 为 nil 时必须返回错误，不得返回空列表")
	}
}
```

（`setupAnalysisTestDB` 在 `backend/data/analysis_center_service_test.go` 内实现：保存并替换 `db.Dao` 为临时 sqlite，`AutoMigrate(&models.KhunterObservation{}, &models.KhunterKronosPrediction{}, &models.KhunterRunSummary{})`，`t.Cleanup` 恢复原 Dao；模式照抄 `backend/data/history/sync_create_test.go:16`。）

- [ ] **Step 2: 验证失败**

Run: `PATH="/d/Go/bin:$PATH" go test ./backend/data/ -run 'TestGetRecommendations|TestGetForecast|TestGetPredictJob' -v`
Expected: FAIL（`NewAnalysisCenterService` 未定义）。

- [ ] **Step 3: 实现门面**

`backend/data/analysis_center_service.go`：

```go
package data

// 统一分析中心门面：选股预测族的唯一后端入口。
// 读 khunter 的观察/运行汇总表与 Kronos 因子明细表，写出预测任务；
// 不修改 khunter 引擎语义，不参与五维评分。

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"go-stock/backend/data/khunter"
	"go-stock/backend/db"
	"go-stock/backend/models"
)

type RecommendRow struct {
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	SignalDate     string  `json:"signalDate"`
	Status         string  `json:"status"`
	DataStatus     string  `json:"dataStatus"`
	RejectReason   string  `json:"rejectReason,omitempty"`
	Score          float64 `json:"score"`
	FactorScore    float64 `json:"factorScore,omitempty"`
	FactorWeight   float64 `json:"factorWeight,omitempty"`
	BlendedScore   float64 `json:"blendedScore"`
	HasFactor      bool    `json:"hasFactor"`
	Degraded       bool    `json:"degraded"`
	DegradedReason string  `json:"degradedReason,omitempty"`
}

type AnalysisCenterStatus struct {
	KronosStatus        string                     `json:"kronosStatus"`
	LastRun             *models.KhunterRunSummary  `json:"lastRun,omitempty"`
	PendingObservations int                        `json:"pendingObservations"`
	FactorEnabled       bool                       `json:"factorEnabled"`
}

type AnalysisCenterService struct {
	Jobs *khunter.PredictionJobRegistry
}

func NewAnalysisCenterService() *AnalysisCenterService {
	return &AnalysisCenterService{Jobs: khunter.NewPredictionJobRegistry()}
}

// errNoDB 门面在无数据库（移动端/启动早期）时的统一错误，禁止用空列表冒充。
var errNoDB = errors.New("分析中心：数据库未初始化")

// GetRecommendations 推荐视图：观察中/已确认候选，按混分降序。
// 只有未降级的因子明细参与混分；降级只透出原因（HasFactor=false）。
func (s *AnalysisCenterService) GetRecommendations(tradeDate string, topN int) ([]RecommendRow, error) {
	if db.Dao == nil {
		return nil, errNoDB
	}
	if topN <= 0 || topN > 100 {
		topN = 30
	}
	q := db.Dao.Model(&models.KhunterObservation{}).
		Where("status IN ?", []string{khunter.ObsStatusPending, khunter.ObsStatusConfirmed})
	if strings.TrimSpace(tradeDate) != "" {
		q = q.Where("signal_date = ?", tradeDate)
	} else if latest, err := latestObservationDate(); err != nil {
		return nil, err
	} else if latest != "" {
		q = q.Where("signal_date = ?", latest)
	}
	var obs []models.KhunterObservation
	if err := q.Order("score DESC, code").Limit(topN * 3).Find(&obs).Error; err != nil {
		return nil, err
	}
	if len(obs) == 0 {
		return []RecommendRow{}, nil
	}

	// 因子明细：只取与本批观察同 (code, signal_date) 的记录
	keys := make([]string, 0, len(obs))
	for _, o := range obs {
		keys = append(keys, o.Code+"|"+o.SignalDate)
	}
	var preds []models.KhunterKronosPrediction
	if err := db.Dao.Where("code || '|' || signal_date IN ?", keys).Find(&preds).Error; err != nil {
		// 因子表查询失败不阻断推荐主视图，但降级原因要透出
		preds = nil
	}
	byKey := make(map[string]models.KhunterKronosPrediction, len(preds))
	for _, p := range preds {
		byKey[p.Code+"|"+p.SignalDate] = p
	}

	rows := make([]RecommendRow, 0, len(obs))
	for _, o := range obs {
		row := RecommendRow{
			Code: o.Code, Name: o.Name, SignalDate: o.SignalDate, Status: o.Status,
			DataStatus: o.DataStatus, RejectReason: o.RejectReason,
			Score: o.Score, BlendedScore: o.Score,
		}
		if p, ok := byKey[o.Code+"|"+o.SignalDate]; ok {
			if p.DegradedReason != "" {
				row.Degraded = true
				row.DegradedReason = p.DegradedReason
			} else if p.Weight > 0 {
				row.HasFactor = true
				row.FactorScore = p.FactorScore
				row.FactorWeight = p.Weight
				row.BlendedScore = round2(o.Score*(1-p.Weight) + p.FactorScore*p.Weight)
			}
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].BlendedScore != rows[j].BlendedScore {
			return rows[i].BlendedScore > rows[j].BlendedScore
		}
		return rows[i].Code < rows[j].Code
	})
	if len(rows) > topN {
		rows = rows[:topN]
	}
	return rows, nil
}

// GetForecast 返回候选落库的完整预测明细（推荐视图展开曲线）；无明细返回空串。
func (s *AnalysisCenterService) GetForecast(code, signalDate string) (string, error) {
	if db.Dao == nil {
		return "", errNoDB
	}
	var p models.KhunterKronosPrediction
	err := db.Dao.Where("code = ? AND signal_date = ?", code, signalDate).First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return p.ForecastJSON, nil
}

func (s *AnalysisCenterService) GetAnalysisStatus() (AnalysisCenterStatus, error) {
	if db.Dao == nil {
		return AnalysisCenterStatus{}, errNoDB
	}
	st := AnalysisCenterStatus{
		KronosStatus:  KronosServiceStatus(),
		FactorEnabled: khunter.LoadKronosFactorConfig("").Enable,
	}
	var sum models.KhunterRunSummary
	if err := db.Dao.Order("trade_date DESC").First(&sum).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return st, err
		}
	} else {
		cp := sum
		st.LastRun = &cp
	}
	var n int64
	if err := db.Dao.Model(&models.KhunterObservation{}).
		Where("status = ?", khunter.ObsStatusPending).Count(&n).Error; err != nil {
		return st, err
	}
	st.PendingObservations = int(n)
	return st, nil
}

func (s *AnalysisCenterService) GetPredictJob(id string) (khunter.PredictJob, error) {
	return s.Jobs.Get(id)
}

func (s *AnalysisCenterService) SubmitPredictJob(code string, predLen int, asOfDate string) (khunter.PredictJob, error) {
	if strings.TrimSpace(code) == "" {
		return khunter.PredictJob{}, fmt.Errorf("股票代码不能为空")
	}
	return s.Jobs.Submit(context.Background(), code, predLen, asOfDate, PredictKLine)
}

// latestObservationDate 最新有观察的信号日（推荐视图默认日期）。
// MAX() 在空表返回 NULL，必须用 sql.NullString 接收，否则 Scan 到 string 会报错。
func latestObservationDate() (string, error) {
	var d sql.NullString
	err := db.Dao.Model(&models.KhunterObservation{}).
		Select("MAX(signal_date)").Scan(&d).Error
	if err != nil {
		return "", err
	}
	if !d.Valid {
		return "", nil
	}
	return d.String, nil
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
```

> 实现注意：
> - `code || '|' || signal_date IN ?` 是 SQLite 写法（本仓库即 SQLite）；也可改为
>   `Where("(code, signal_date) IN ?", pairs)` 让 gorm 展开元组，可移植性更好。
> - import 需含 `context`、`database/sql`、`math`、`gorm.io/gorm`。
> - `round2` 必须与前端 `blendedScore` 同口径，否则单测 `79` / `66.27` 对不上。
> - `GetForecast` 的 `gorm.ErrRecordNotFound` 视为"无明细"，返回空串而非错误。

`backend/handler/analysis_center_handler.go`：

```go
package handler

import (
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

func (h *AnalysisCenterHandler) GetAnalysisStatus() (data.AnalysisCenterStatus, error) {
	return h.svc.GetAnalysisStatus()
}

func (h *AnalysisCenterHandler) GetForecast(code string, signalDate string) (string, error) {
	return h.svc.GetForecast(code, signalDate)
}

func (h *AnalysisCenterHandler) SubmitPredictJob(code string, predLen int, asOfDate string) (khunter.PredictJob, error) {
	return h.svc.SubmitPredictJob(code, predLen, asOfDate)
}

func (h *AnalysisCenterHandler) GetPredictJob(id string) (khunter.PredictJob, error) {
	return h.svc.GetPredictJob(id)
}
```

`app.go`：`App` 结构体加字段（紧邻既有 `khunterHandler` 声明）：

```go
	analysisCenterHandler *handler.AnalysisCenterHandler
```

`newApp` 里构造（紧邻 `app.khunterHandler = handler.NewKhunterHandler(emit)`）：

```go
	app.analysisCenterHandler = handler.NewAnalysisCenterHandler()
```

`main.go` 的 `Bind: []interface{}{...}` 列表末尾（`app.khunterHandler` 之后）加入一行：

```go
			app.analysisCenterHandler,
```

> 注意：`main.go` 当前**没有**导入 `go-stock/backend/handler`，所以不能在 Bind 里写
> `handler.NewAnalysisCenterHandler()`；必须走 `app` 字段（与 `dailyPickHandler`/`khunterHandler` 一致）。
> **不要**在 `app.go` 的 startup 里写任何占位注册。

- [ ] **Step 4: 生成绑定 + 验证**

Run: `PATH="/d/Go/bin:$PATH" go build ./... && PATH="/d/Go/bin:$PATH" go test ./backend/data/ ./backend/handler/ -count=1`
Expected: 编译通过、测试 PASS。
Run: `PATH="/d/Go/bin:$PATH" wails generate module`
Expected: 生成 `frontend/wailsjs/go/handler/AnalysisCenterHandler.{js,d.ts}`。

- [ ] **Step 5: Commit**

```bash
git add backend/data/analysis_center_service.go backend/data/analysis_center_service_test.go backend/handler/analysis_center_handler.go app.go main.go frontend/wailsjs
git commit -m "feat(analysis): unified analysis center facade with blended recommendations"
```

---

### Task 5: 前端分析中心（Tab 壳 + 推荐视图 + 路由菜单收敛）

**Files:**
- Create: `frontend/src/components/analysis/AnalysisCenter.vue`
- Create: `frontend/src/components/analysis/RecommendView.vue`
- Create: `frontend/src/components/analysis/recommend.ts`（纯函数，供单测）
- Create: `frontend/src/api/analysisCenter.ts`
- Create: `frontend/tests/analysis-center.test.ts`
- Modify: `frontend/src/components/ScreeningWorkbench.vue`（**嵌入前必修**：见 Step 3）
- Modify: `frontend/src/components/kline-analysis.vue`（**嵌入前必修**：返回按钮）
- Modify: `frontend/src/router/router.js`
- Modify: `frontend/src/config/navigation.js`

**Interfaces:**
- Consumes: Task 4 的绑定 `AnalysisCenterHandler.GetRecommendations/GetAnalysisStatus/GetForecast/SubmitPredictJob/GetPredictJob`（`frontend/wailsjs/go/handler/AnalysisCenterHandler.*`）；既有 `KhunterPanel`、`ScreeningWorkbench`、`kline-analysis.vue`。
- Produces: 路由 `/analysis?tab=<recommend|khunter|workbench|kline>`；`/daily-pick`、`/khunter`、`/kline-analysis` 用**路由 redirect**（`createWebHashHistory`，不存在 301）。

**嵌入前必修的两个既有阻塞点（不改就会把用户踢出 Tab 容器）**：

| 文件 | 现状 | 必修 |
|---|---|---|
| `ScreeningWorkbench.vue:18` | `watch(activeTab, v => router.replace({name:'analysisWorkbench', query:{tab:v}}))` | 加 `embedded` prop；为 true 时不 `router.replace`，改 `emit('update:tab', v)` |
| `kline-analysis.vue:283` | 返回按钮 `router.push({ path: '/' })` | 加 `embedded` prop；为 true 时 `emit('back')` 由父容器处理 |

- [ ] **Step 1: 写失败测试（只测纯函数）**

`frontend/tests/analysis-center.test.ts`：

```ts
import test from 'node:test'
import assert from 'node:assert/strict'
import {
  blendedScore, tabFromQuery, rowSubtitle, errText,
  type SubtitleInput,
} from '../src/components/analysis/recommend.ts'

test('有因子按权重混分，无因子保持原分（与后端 round2 同口径）', () => {
  assert.equal(blendedScore(80, 60, 0.05), 79)
  assert.equal(blendedScore(70, 50, 0), 70)
  assert.equal(blendedScore(66.6, 60, 0.05), 66.27)
})

test('未知 tab 回落到推荐，合法 tab 原样返回', () => {
  assert.equal(tabFromQuery('khunter'), 'khunter')
  assert.equal(tabFromQuery('nope'), 'recommend')
  assert.equal(tabFromQuery(undefined), 'recommend')
})

test('降级行必须显示原因而不是当成有因子', () => {
  assert.match(
    rowSubtitle({ hasFactor: false, degraded: true, degradedReason: 'KRONOS_OFFLINE' } satisfies SubtitleInput),
    /KRONOS_OFFLINE/,
  )
  assert.equal(rowSubtitle({ hasFactor: false, degraded: false } satisfies SubtitleInput), '')
})

test('异常归一化为可读文案（不依赖 any）', () => {
  assert.equal(errText(new Error('KRONOS_DISABLED')), 'KRONOS_DISABLED')
  assert.equal(errText('boom'), 'boom')
  assert.equal(errText(undefined), '未知错误')
})
```

- [ ] **Step 2: 验证失败**

Run: `cd frontend && node --import ./tests/register-ts-ext.mjs --test tests/analysis-center.test.ts`
Expected: FAIL（模块不存在）。

- [ ] **Step 3: 实现纯函数 + 嵌入改造 + API + 两个组件**

`frontend/src/components/analysis/recommend.ts`：

```ts
// 推荐视图纯函数：与服务端口径一致（后端 round2），便于单测。

export const ANALYSIS_TABS = ['recommend', 'khunter', 'workbench', 'kline'] as const
export type AnalysisTab = (typeof ANALYSIS_TABS)[number]

/** 混分：w<=0 或缺失时保持原分；保留 2 位小数（与 Go round2 一致）。 */
export function blendedScore(score: number, factorScore: number, weight: number): number {
  if (!weight || weight <= 0) return Math.round(score * 100) / 100
  return Math.round((score * (1 - weight) + factorScore * weight) * 100) / 100
}

export function tabFromQuery(v: unknown): AnalysisTab {
  return ANALYSIS_TABS.includes(v as AnalysisTab) ? (v as AnalysisTab) : 'recommend'
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
  degraded?: boolean
  degradedReason?: string
}

export type SubtitleInput = Pick<RecommendRow, 'hasFactor' | 'degraded' | 'degradedReason' | 'rejectReason'>

/** 行内副标题：降级原因 / 数据闸门原因，二者互斥取一个非空。 */
export function rowSubtitle(row: SubtitleInput): string {
  if (row.hasFactor) return ''
  if (row.degraded) return row.degradedReason || '预测降级'
  return row.rejectReason || ''
}

/** 异常 → 可读文案。Wails 绑定抛出的值类型不确定，按 unknown 收窄。 */
export function errText(e: unknown): string {
  if (e instanceof Error) return e.message || e.name
  if (typeof e === 'string') return e
  if (e && typeof e === 'object' && 'message' in e) {
    const m = e.message
    if (typeof m === 'string' && m) return m
  }
  return '未知错误'
}

/** 门面状态：与后端 AnalysisCenterStatus 对齐（未启动时可选字段缺失）。 */
export interface AnalysisCenterStatus {
  kronosStatus: 'disabled' | 'offline' | 'loading' | 'online'
  pendingObservations: number
  factorEnabled: boolean
}

/** Kronos 状态徽标类型：仅 online 为 success。 */
export function kronosTagType(status: AnalysisCenterStatus | null): 'success' | 'warning' | 'default' {
  if (!status) return 'default'
  return status.kronosStatus === 'online' ? 'success' : 'warning'
}
```

`frontend/src/components/ScreeningWorkbench.vue` 改造（只动脚本，不动模板）：

```js
const props = defineProps<{ embedded?: boolean }>()
const emit = defineEmits<{ (e: 'update:tab', v: string): void }>()

const activeTab = ref(route.query.tab === 'strategy' ? 'strategy' : 'pattern')

watch(activeTab, (v) => {
  if (props.embedded) {
    emit('update:tab', v) // 嵌入时不改父路由，否则会被踢出 Tab 容器
    return
  }
  router.replace({ name: 'analysisWorkbench', query: { tab: v } })
})

watch(() => route.query.tab, (v) => {
  if (props.embedded) return
  const t = v === 'strategy' ? 'strategy' : 'pattern'
  if (activeTab.value !== t) activeTab.value = t
})
```

`frontend/src/components/kline-analysis.vue` 改造（只动返回按钮与脚本）：

```js
const props = defineProps<{ embedded?: boolean }>()
const emit = defineEmits<{ (e: 'back'): void }>()

function goBack() {
  if (props.embedded) {
    emit('back')
    return
  }
  router.push({ path: '/' })
}
```

模板中把原来的 `router.push({ path: '/' })` 调用点改为 `@click="goBack"`。

`frontend/src/api/analysisCenter.ts`：

```ts
import {
  GetRecommendations,
  GetAnalysisStatus,
  GetForecast,
  SubmitPredictJob,
  GetPredictJob,
} from '../../wailsjs/go/handler/AnalysisCenterHandler'
import type { RecommendRow, AnalysisCenterStatus } from '../components/analysis/recommend'

export async function fetchRecommendations(tradeDate = '', topN = 30): Promise<RecommendRow[]> {
  return GetRecommendations(tradeDate, topN)
}
export async function fetchAnalysisStatus(): Promise<AnalysisCenterStatus> {
  return GetAnalysisStatus()
}
export async function fetchForecast(code: string, signalDate: string): Promise<string> {
  return GetForecast(code, signalDate)
}
export async function submitPredictJob(code: string, predLen = 5, asOfDate = '') {
  return SubmitPredictJob(code, predLen, asOfDate)
}
export async function fetchPredictJob(id: string) {
  return GetPredictJob(id)
}
```

`frontend/src/components/analysis/RecommendView.vue`（骨架：状态徽标 + 表格 + 明细展开 + 预测按钮轮询）：

```vue
<template>
  <div>
    <n-space align="center" style="margin-bottom: 8px">
      <n-text>推荐视图（观察候选 × Kronos 因子混分）</n-text>
      <n-tag v-if="status" size="small" :type="kronosTagType(status)">{{ status.kronosStatus }}</n-tag>
      <n-tag v-if="status && !status.factorEnabled" size="small" type="default">因子未启用</n-tag>
      <n-tag v-if="status" size="small">待确认 {{ status.pendingObservations }}</n-tag>
      <n-button size="small" :loading="loading" @click="refresh">刷新</n-button>
      <n-button text size="small" @click="$emit('open-research-recommends')">AI 推荐列表 →</n-button>
    </n-space>
    <n-alert v-if="loadError" type="error" :bordered="false" style="margin-bottom: 8px">{{ loadError }}</n-alert>
    <n-data-table :columns="columns" :data="rows" :loading="loading" size="small" />
  </div>
</template>

<script setup lang="ts">
import { h, onMounted, onUnmounted, ref } from 'vue'
import { NButton, NTag, NText, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import {
  fetchRecommendations, fetchAnalysisStatus, fetchForecast, submitPredictJob, fetchPredictJob,
} from '../../api/analysisCenter'
import { rowSubtitle, kronosTagType, errText, type RecommendRow, type AnalysisCenterStatus } from './recommend'

defineEmits<{ (e: 'open-research-recommends'): void }>()

const rows = ref<RecommendRow[]>([])
const status = ref<AnalysisCenterStatus | null>(null)
const loading = ref(false)
const loadError = ref('')
const message = useMessage()
const pollTimers = new Map<string, number>()

const columns: DataTableColumns<RecommendRow> = [
  { title: '代码', key: 'code', width: 96 },
  { title: '名称', key: 'name', width: 96 },
  { title: '信号日', key: 'signalDate', width: 110 },
  { title: '状态', key: 'status', width: 80 },
  { title: '评分', key: 'score', width: 72 },
  {
    title: 'Kronos', key: 'factor', width: 130,
    render(row) {
      if (row.hasFactor) {
        return h(NTag, { size: 'tiny', type: 'info' },
          { default: () => `${row.factorScore} × ${row.factorWeight}` })
      }
      const sub = rowSubtitle(row)
      return sub ? h(NText, { depth: 3 }, { default: () => sub }) : h(NText, { depth: 3 }, { default: () => '—' })
    },
  },
  {
    title: '综合', key: 'blendedScore', width: 80,
    sorter: (a, b) => a.blendedScore - b.blendedScore, defaultSortOrder: 'descend',
  },
  {
    title: '操作', key: 'op', width: 150,
    render(row) {
      return h('span', [
        h(NButton, { size: 'tiny', onClick: () => runPredict(row.code) },
          { default: () => 'AI预测' }),
        row.hasFactor
          ? h(NButton, {
              size: 'tiny', text: true, style: 'margin-left:8px',
              onClick: () => expandForecast(row),
            }, { default: () => '明细' })
          : null,
      ])
    },
  },
]

async function refresh() {
  loading.value = true
  loadError.value = ''
  try {
    rows.value = await fetchRecommendations()
    status.value = await fetchAnalysisStatus()
  } catch (e: unknown) {
    // 门面返回 error 时必须显式报错，不能把空列表显示成"今天没候选"
    loadError.value = `加载失败：${errText(e)}`
  } finally {
    loading.value = false
  }
}

async function expandForecast(row: RecommendRow) {
  try {
    const raw = await fetchForecast(row.code, row.signalDate)
    if (!raw) {
      message.warning('该候选暂无落库预测明细')
      return
    }
    // 明细用 modal/抽屉展示预测曲线；最小实现为 JSON 预览
    message.info(`${row.code} 预测明细已加载（${raw.length} 字节）`)
  } catch (e: unknown) {
    message.error(`读取明细失败：${errText(e)}`)
  }
}

async function runPredict(code: string) {
  try {
    const job = await submitPredictJob(code, 5, '')
    message.info('预测任务已提交（模型冷启动需数分钟），请稍后查看')
    pollJob(job.id)
  } catch (e: unknown) {
    message.error(`提交失败：${errText(e)}`)
  }
}

async function pollJob(id: string) {
  try {
    const j = await fetchPredictJob(id)
    if (j.status === 'done') {
      message.success(`${j.code} 预测完成`)
      return
    }
    if (j.status === 'failed') {
      message.error(`预测失败：${j.err || '未知错误'}`)
      return
    }
  } catch (e: unknown) {
    // 未知 ID / 任务过期：停止轮询并提示，绝不无限循环
    message.error(`预测任务已失效：${errText(e)}`)
    return
  }
  const t = window.setTimeout(() => {
    pollTimers.delete(id)
    pollJob(id)
  }, 2000)
  pollTimers.set(id, t)
}

onMounted(refresh)
onUnmounted(() => {
  for (const t of pollTimers.values()) window.clearTimeout(t)
  pollTimers.clear()
})
</script>
```

`frontend/src/components/analysis/AnalysisCenter.vue`：

```vue
<template>
  <n-tabs v-model:value="tab" type="line" animated display-directive="show" @update:value="syncQuery">
    <n-tab-pane name="recommend" tab="推荐">
      <RecommendView @open-research-recommends="openResearchRecommends" />
    </n-tab-pane>
    <n-tab-pane name="khunter" tab="狩猎场">
      <KhunterPanel />
    </n-tab-pane>
    <n-tab-pane name="workbench" tab="选股工作台">
      <ScreeningWorkbench embedded @update:tab="syncQuery" />
    </n-tab-pane>
    <n-tab-pane name="kline" tab="K线分析">
      <KlineAnalysis embedded @back="syncQuery('recommend')" />
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
import { tabFromQuery, type AnalysisTab } from './recommend'

const route = useRoute()
const router = useRouter()
const tab = ref<AnalysisTab>(tabFromQuery(route.query.tab))

watch(() => route.query.tab, (v) => {
  const t = tabFromQuery(v)
  if (t !== tab.value) tab.value = t
})

function syncQuery(name: string) {
  const t = tabFromQuery(name)
  if (route.query.tab === t) return
  router.replace({ query: { ...route.query, tab: t } })
}

function openResearchRecommends() {
  router.push({ name: 'researchRecommends' })
}
</script>
```

> `display-directive="show"` 保留已挂载 Tab 的状态；缺省 `if` 会卸载 `KhunterPanel`，
> 触发其 `onUnmounted` 里的 `EventsOff`，运行进度事件丢失。
> **不要**用 `<keep-alive>` + `Suspense` 组合（naive-ui Tab 内 Suspense 无收益且增加层级）。

- [ ] **Step 4: 路由与菜单**

`frontend/src/router/router.js`：

```js
import analysisCenter from "../components/analysis/AnalysisCenter.vue"
// 分析中心（放在 /analysis/workbench 附近，redirect 必须先于它之前声明或使用不同 name）
{ path: '/analysis', component: analysisCenter, name: 'analysisCenter' },
{ path: '/analysis/workbench', component: ScreeningWorkbench, name: 'analysisWorkbench' }, // 保留直达
// 重定向：旧入口收敛到对应 Tab（hash 路由，不存在 301）
{ path: '/daily-pick', redirect: { name: 'analysisCenter', query: { tab: 'recommend' } } },
{ path: '/khunter', redirect: { name: 'analysisCenter', query: { tab: 'khunter' } } },
{ path: '/kline-analysis', redirect: { name: 'analysisCenter', query: { tab: 'kline' } } },
```

> 注意 `/analysis` 与 `/analysis/workbench` 是不同 path，vue-router 4 按路径精确匹配，不存在前缀冲突。
> 若担心顺序，把 `/analysis/workbench` 声明在 `/analysis` 之前。

`frontend/src/config/navigation.js` 的"分析工具"组 children 替换为单项：

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
        // 回测验证暂留（第二阶段迁入回测中心）
        routeItem({
          title: '回测验证',
          groupKey: MENU_GROUP_KEYS.ANALYSIS,
          icon: icons.AlarmOutline,
          to: { name: 'backtest' },
        }),
      ],
    },
```

"研究资讯"组删除"股票推荐"菜单项（`to: { name: 'researchRecommends' }` 那段）；该页面仍可从推荐视图页头链接到达。

- [ ] **Step 5: 生成绑定 + 前端验证**

```bash
PATH="/d/Go/bin:/c/Users/54389/go/bin:$PATH" wails generate module
cd frontend
node --import ./tests/register-ts-ext.mjs --test tests/analysis-center.test.ts
node tests/check-vue-script.mjs src/components/analysis/AnalysisCenter.vue
node tests/check-vue-script.mjs src/components/analysis/RecommendView.vue
node tests/run-vue-tsc.mjs
```

Expected: 测试 3 个 PASS、脚本检查 PASS、vue-tsc 无新增错误（仅既有 6 个无关错误）。
**手工冒烟（必做，仓库无组件测试基建）**：
1. 打开 `/analysis`，四个 Tab 依次切换，回来时狩猎场不重新拉取（`display-directive="show"` 生效）；
2. 在"选股工作台"内切换内部 Tab，URL 仍停在 `/analysis?tab=workbench`（不再被踢走）；
3. 在"K线分析"点返回，回到推荐 Tab（不跳首页）；
4. 直接访问 `#/daily-pick`、`#/khunter`、`#/kline-analysis`，各自落到对应 Tab；
5. Kronos 未开启时点"AI预测"，提示"提交失败：KRONOS_DISABLED"或任务转 failed，**不出现无限轮询**。

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/analysis/ frontend/src/api/analysisCenter.ts frontend/tests/analysis-center.test.ts frontend/src/components/ScreeningWorkbench.vue frontend/src/components/kline-analysis.vue frontend/src/router/router.js frontend/src/config/navigation.js frontend/wailsjs
git commit -m "feat(analysis): unified analysis center shell with recommend view"
```

---

### Task 6: 端到端验证与回放口径对照

**Files:**
- Modify: 无（验证任务）

- [ ] **Step 1: 后端全量回归**

```bash
PATH="/d/Go/bin:$PATH" go build ./...
PATH="/d/Go/bin:$PATH" go vet ./backend/...
PATH="/d/Go/bin:$PATH" go test ./backend/data/khunter/ ./backend/data/ ./backend/handler/ ./backend/agent/ -count=1
```

Expected: 全部 ok。（`go test ./backend/data/...` 会因网络用例超时，用上面的定向包组合。）

- [ ] **Step 2: 前端构建**

```bash
cd frontend && node node_modules/vite/bin/vite.js build 2>&1 | tail -2
```

Expected: `✓ built`。

- [ ] **Step 3: 回放基线（合并前后同库同参，可证伪）**

**先建立基线**，再与合并后对照——否则默认 `enable:false` 时"评分不变"是必然结果，什么都证明不了：

```bash
# 1) 取一份数据库副本，基线用合并前的代码（当前 HEAD 27b3ae20）
cp "<安装库>" /tmp/replay-baseline.db
git stash -u && PATH="/d/Go/bin:$PATH" go run ./backend/cmd/khunterreplay \
  -db /tmp/replay-baseline.db -start 2026-09-01 -end 2026-11-17 \
  -variants full -config config/khunter_risk_control.json \
  -cases '688322.SH:2026-11-11,300780.SZ:2026-10-29,600259.SH:2026-11-03,300829.SZ:2026-11-12,300969.SZ:2026-10-29' \
  -out /tmp/five-cases-before.json
git stash pop

# 2) 同一库副本跑合并后代码
cp /tmp/replay-baseline.db /tmp/replay-after.db
PATH="/d/Go/bin:$PATH" go run ./backend/cmd/khunterreplay \
  -db /tmp/replay-after.db -start 2026-09-01 -end 2026-11-17 \
  -variants full -config config/khunter_risk_control.json \
  -cases '688322.SH:2026-11-11,300780.SZ:2026-10-29,600259.SH:2026-11-03,300829.SZ:2026-11-12,300969.SZ:2026-10-29' \
  -out /tmp/five-cases-after.json

# 3) 结构化比对（不是人眼看）
python -c "
import json,sys
a=json.load(open('/tmp/five-cases-before.json')); b=json.load(open('/tmp/five-cases-after.json'))
def flat(o,p=''):
    if isinstance(o,dict):
        for k,v in o.items(): yield from flat(v,p+'/'+k)
    elif isinstance(o,list):
        for i,v in enumerate(o): yield from flat(v,p+f'[{i}]')
    else: yield p,o
fa,fb=dict(flat(a)),dict(flat(b))
diff={k:(fa.get(k),fb.get(k)) for k in set(fa)|set(fb) if fa.get(k)!=fb.get(k)}
print('DIFF COUNT', len(diff))
for k,v in list(diff.items())[:40]: print(k,v)
sys.exit(1 if diff else 0)
"
```

Expected: `DIFF COUNT 0`，退出码 0。**任何差异都是回归**，必须查明后再继续（不允许"允许污染差异"式豁免）。

- [ ] **Step 4: 因子开启后的名单不变性（证明因子未侵入入选项）**

```bash
mkdir -p /tmp/kc && cp config/khunter_kronos.json /tmp/kc/ 2>/dev/null || true
cat > config/khunter_kronos.json <<'JSON'
{ "enable": true, "topN": 30, "predLen": 5, "weight": 0.05, "maxWeight": 0.15 }
JSON
# Kronos 服务不启动 → 全部降级中性分，验证"降级不阻塞主链"
cp /tmp/replay-baseline.db /tmp/replay-factor.db
PATH="/d/Go/bin:$PATH" go run ./backend/cmd/khunterreplay \
  -db /tmp/replay-factor.db -start 2026-09-01 -end 2026-11-17 \
  -variants full -config config/khunter_risk_control.json \
  -cases '688322.SH:2026-11-11,300780.SZ:2026-10-29,600259.SH:2026-11-03,300829.SZ:2026-11-12,300969.SZ:2026-10-29' \
  -out /tmp/five-cases-factor.json
```

比对 `/tmp/five-cases-factor.json` 与 `/tmp/five-cases-before.json` 的 **信号数 / 每案例结局计数**：
Expected: 完全一致（因子不参与评分与入选）。评分字段允许出现 `degraded` 标记，但 `total` 不得变化。

- [ ] **Step 5: 安装版冒烟**

安装新构建、启动，逐项确认（前端无组件测试，此步是唯一 UI 验证）：

1. 分析中心四 Tab 各切一遍；切走再切回，狩猎场不重新跑；
2. 工作台内部 Tab 切换后 URL 仍是 `#/analysis?tab=workbench`；
3. K 线分析返回按钮回到推荐 Tab；
4. `#/daily-pick`、`#/khunter`、`#/kline-analysis` 三个旧 hash 均落到对应 Tab；
5. 推荐视图：有数据或明确空态；Kronos 未开启时徽标 `disabled`；开启后 `online`/`loading`/`offline` 均能显示；
6. 点"AI预测"：未开启 → 立即报错且停止轮询；开启但服务离线 → 任务 `failed` 有原因、可再次点击重试（新任务 ID）。

- [ ] **Step 6: 收尾提交（如上述步骤有代码修正）**

```bash
git add -A && git commit -m "chore(analysis): e2e verification fixes" # 仅当有修正时
```

并把 `config/khunter_kronos.json` 恢复为 `enable:false`（Step 4 临时改动不得带进提交）。

---

## Self-Review 记录

**Spec 覆盖**（对应修订后设计文档）：

| 设计要求 | 落地任务 |
|---|---|
| khunter 为唯一推荐引擎、五维评分不动 | Task 2 Step 4（异步旁路，不写 Total）、Task 6 Step 4（名单不变性对照） |
| Kronos 三消费面：交互/排序/验证 | Task 3（交互异步）、Task 2（排序因子）、既有回测 Tab（验证，第一阶段不新建页） |
| 因子明细落库、可展开 | Task 2 模型 `ForecastJSON` + Task 4 `GetForecast` + Task 5 明细按钮 |
| 服务治理（状态徽标/拉起/lazy start） | Task 1 导出 + Task 4 `GetAnalysisStatus.FactorEnabled` + Task 5 徽标 |
| 错误处理表 | Task 2（离线整体降级返回 nil error）、Task 3（failed 释放键/未知 ID 报错/TTL）、Task 4（DB 错误不吞、`Dao==nil`） |
| 数据流与调度 | 设计文档「数据流」按真实 cron 重写；本计划**不动** daily_pick cron 与表写入 |
| daily_pick 第一阶段保留 | Global Constraints 明确禁止停写；旧 handler 不改签名、不转调门面 |
| 前端 Tab 壳/嵌入/菜单 | Task 5（`display-directive="show"`、`embedded` prop 两处必修、redirect、页头入口） |
| 测试策略 | Task 2/3/4 Go 单测 + Task 5 纯函数单测 + Task 6 回放与手工冒烟；**不写组件挂载测试**（无基建） |

**占位符扫描**：无 TBD/TODO。Task 1 标注为已在工作区完成；Task 4 的 `main.go` 注册给了确定位置，不含 `_ = xxx` 占位行。

**类型/口径一致性**：

- `data.PredictKLine(ctx, code, predLen, asOfDate)`：Task 1 定义，Task 3 `PredictFunc` 直接匹配。
- `data.KronosBatchPredict`：Task 2 `BatchPredictFunc` 匹配（`(ctx, []string, int) → (map, map, error)`）。
- `KronosFactorResult`：Task 2 产出（含 `Code`/`ForecastJSON`），Task 2 Step 4 goroutine 消费。
- `RecommendRow`：Task 4 字段集合 = Task 5 `RecommendRow` TS 接口（含 `degraded`/`degradedReason`）。
- 混分口径：Go `round2` = TS `blendedScore`，单测 79 / 66.27 双端对齐。
- `GetPredictJob` 未知 ID：Task 3 `ErrPredictJobNotFound` → Task 4 直接透传 → Task 5 捕获后停止轮询。

**已知偏差（显式记录，非隐瞒）**：因子不进入五维评分（设计已改为排序面）；"预测质量"独立页第一阶段不做；khunter 侧收益回填不在本阶段（daily_pick 因此保留）。三项均在设计的「非目标」与「阶段划分」中声明。
