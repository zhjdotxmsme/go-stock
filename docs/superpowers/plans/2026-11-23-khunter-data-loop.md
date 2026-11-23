# 阶段 2：数据闭环（khunter 收益回填 + Kronos 对账 + 调权切源）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** khunter 侧自建复盘数据（T+1/3/5 收益回填 + Kronos 预测命中对账），因子调权切到 khunter 数据源，拆除 daily_pick 后台自动化，使阶段 3/4 不再依赖 daily_picks 表。

**Architecture:** 新增 `khunter_review` cron（09:15）驱动 `backend/data/khunter/review.go`：先按 `EntryPrice` 基准回填 `KhunterHunting` 三窗口收益，再按预测期末对账 `KhunterKronosPrediction.Hit`。统计/调权新文件 `kronos_stats.go` 读 khunter 表，流水线在既有因子挂接点应用调权后权重。daily_pick 只拆 `app_lifecycle.go` 两个 cron，引擎与 SelectStock 手动链路保留（spec §2.4 修订）。

**Tech Stack:** Go 1.26 / GORM (sqlite) / 既有 cron 注册表（cron_task_api + system_handler）

**Spec:** `docs/superpowers/specs/2026-11-23-deep-integration-phase2-design.md`

## Global Constraints

- Go 工具链：`export PATH="/d/Go/bin:$PATH"`（Go 1.26.3，Windows/Git Bash）。
- 定向测试命令（避开存量 live 网络用例）：
  `go test ./backend/data/khunter/ ./backend/data/ ./backend/handler/ ./backend/agent/ -count=1 -skip 'TestGetHtml|TestUpdateUSName|TestEastMoneyKLineApi|TestRobot|TestInvestCalendar'`
  Expected：全部 `ok`。（失败用例均为环境网络问题，6-8 月存量测试，见阶段 1 验证记录。）
- **不改** `scorer.Score` 五维加权公式；**不改** 推荐视图混分公式；因子仍在排序面。
- `config/khunter_kronos.json` 全程保持 `enable:false`——调权只改系数来源，回放输出必须 DIFF 0。
- **Wails 绑定与前端零改动**：不删 `DailyPickHandler`/`AnalysisHandler` 任何方法签名，不碰前端文件。
- `daily_picks` 表写入与 SelectStock 手动选股链路保留（spec §2.4 修订）。
- 注释/日志用中文，风格沿用周边代码；commit message 用 conventional commits。
- 每个 Task 的 commit 前必须 `go build ./...` 通过 + 相关包测试绿。

---

### Task 1: 冻结回放基线 + 模型字段扩展

**Files:**
- Modify: `backend/models/khunter_models.go`（`KhunterHunting` 加 4 字段；`KhunterKronosPrediction` 加 2 字段）
- Test: `backend/data/khunter/migrate_test.go`（新建）

**Interfaces:**
- Produces（后续 Task 依赖的模型字段）：
  - `models.KhunterHunting.ReturnT1 / Return3D / Return5D float64`（% ，0=未回填）
  - `models.KhunterHunting.Reviewed bool`
  - `models.KhunterKronosPrediction.Hit string`（`hit`/`miss`/`flat`，""=未对账）
  - `models.KhunterKronosPrediction.ActualPct float64`（预测期末实际涨幅 %，0=未对账）
- 迁移自动生效：`backend/data/khunter/repo.go` 的 `EnsureMigrate()`（15-33 行）已 AutoMigrate 两个模型，加字段即自动加列。

- [ ] **Step 0: 冻结回放基线（在任何代码修改之前）**

```bash
export PATH="/d/Go/bin:$PATH"
go build -o /tmp/replay-phase2-base.exe ./backend/cmd/khunterreplay
cp /d/_installers/go-stock/replay-verify/stock.db /tmp/phase2-replay.db
cp /d/_installers/go-stock/replay-verify/stock.db-wal /tmp/phase2-replay.db-wal 2>/dev/null
cp /d/_installers/go-stock/replay-verify/stock.db-shm /tmp/phase2-replay.db-shm 2>/dev/null
cp /tmp/phase2-replay.db /tmp/phase2-replay-after.db
cp /tmp/phase2-replay.db-wal /tmp/phase2-replay-after.db-wal 2>/dev/null
/tmp/replay-phase2-base.exe -db /tmp/phase2-replay.db -start 2026-09-01 -end 2026-11-17 \
  -variants full -config config/khunter_risk_control.json \
  -cases '688322.SH:2026-11-11,300780.SZ:2026-10-29,600259.SH:2026-11-03,300829.SZ:2026-11-12,300969.SZ:2026-10-29' \
  -out "$TEMP/phase2-five-cases-before.json"
```

Expected：最后一行 `已写出 …（13332 字节）`（与阶段 1 基线同尺寸）。

- [ ] **Step 1: 写失败测试**

新建 `backend/data/khunter/migrate_test.go`：

```go
package khunter

import (
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

// 阶段 2 复盘字段必须随 EnsureMigrate 自动建列（GORM AutoMigrate 幂等）。
func TestEnsureMigrateAddsReviewColumns(t *testing.T) {
	setupTestDB(t)
	for _, col := range []string{"ReturnT1", "Return3D", "Return5D", "Reviewed"} {
		if !db.Dao.Migrator().HasColumn(&models.KhunterHunting{}, col) {
			t.Fatalf("KhunterHunting 缺列 %s", col)
		}
	}
	for _, col := range []string{"Hit", "ActualPct"} {
		if !db.Dao.Migrator().HasColumn(&models.KhunterKronosPrediction{}, col) {
			t.Fatalf("KhunterKronosPrediction 缺列 %s", col)
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
export PATH="/d/Go/bin:$PATH" && go test ./backend/data/khunter/ -run TestEnsureMigrateAddsReviewColumns -count=1
```

Expected：FAIL（`缺列 ReturnT1`）。

- [ ] **Step 3: 加模型字段**

`backend/models/khunter_models.go` `KhunterHunting` 中，`ExitReason` 行之后、`CreatedAt` 之前插入：

```go
	// 复盘回填（09:15 khunter_review，基准=确认日真实成交价 EntryPrice，单位 %）：
	// 0 表示该窗口未回填/未到期；存在才回填，到期后重复运行自动补齐。
	ReturnT1 float64 `json:"returnT1"` // T+1：EnterDate 后第 1 个交易日收盘/EntryPrice-1
	Return3D float64 `json:"return3d"` // 3 日持有：第 3 个交易日收盘/EntryPrice-1
	Return5D float64 `json:"return5d"` // 5 日持有：第 5 个交易日收盘/EntryPrice-1
	Reviewed bool   `json:"reviewed" gorm:"default:false;index"`
```

`KhunterKronosPrediction` 中，`DegradedReason` 行之后插入：

```go
	// Hit 预测期末对账结果（09:15 khunter_review 回写）：hit/miss/flat，"" 表示未对账/未到期。
	// flat = 实际涨跌绝对值 ≤0.3%（方向带）被排除，不计入命中率统计，但为终态不再重判。
	Hit       string  `gorm:"size:10" json:"hit"`
	ActualPct float64 `json:"actualPct"` // 预测期末实际涨幅 %（0=未对账）
```

- [ ] **Step 4: 跑测试确认通过 + 全量构建**

```bash
export PATH="/d/Go/bin:$PATH" && go test ./backend/data/khunter/ -run TestEnsureMigrateAddsReviewColumns -count=1 && go build ./...
```

Expected：ok + 构建无输出。

- [ ] **Step 5: Commit**

```bash
git add backend/models/khunter_models.go backend/data/khunter/migrate_test.go
git commit -m "feat(khunter): review backfill columns on hunting and kronos predictions"
```

---

### Task 2: data 包导出可复用纯函数

**Files:**
- Modify: `backend/data/daily_pick_review.go`（`findBarAfter` → 导出 `FindBarAfter`，2 处调用点）
- Modify: `backend/data/daily_pick_kronos.go`（`kronosHorizon` → `KronosHorizon`、`kronosHorizonHit` → `KronosHorizonHit`，各调用点同步）

**Interfaces:**
- Produces（Task 3/5 依赖）：
  - `data.FindBarAfter(klines []KLineData, tradeDate string) int`——找 tradeDate 之后第一根 K 线下标（即 T+1），找不到返回 -1。
  - `data.KronosHorizon(predLen int) int`——预测对账交易日数，<=0 按 5。
  - `data.KronosHorizonHit(direction string, horizonRet float64) string`——`hit`/`miss`/`""`（|ret|≤0.3% 方向带排除）。

- [ ] **Step 1: 重命名并同步调用点**

`backend/data/daily_pick_review.go`：
- `func findBarAfter(` 改为 `func FindBarAfter(`（函数 doc 注释保留，名字同步）。
- 调用点 2 处：`reviewOne` 内 `t1 := findBarAfter(klines, pick.TradeDate)`、`backfillWindows` 内 `t1 := findBarAfter(klines, p.TradeDate)`，改为 `data.FindBarAfter` 同包直接 `FindBarAfter`。

`backend/data/daily_pick_kronos.go`：
- `func kronosHorizon(` → `func KronosHorizon(`。
- `func kronosHorizonHit(` → `func KronosHorizonHit(`。
- 调用点：`applyKronosHorizonHit` 内 2 处（`horizon := kronosHorizon(...)`、`want = kronosHorizonHit(...)`）、`judgeKronosPick` 内 2 处（`horizon := kronosHorizon(...)`、`hit = kronosHorizonHit(...)`），同步改大写。

- [ ] **Step 2: 验证构建 + daily_pick 相关测试绿（行为不变的保障）**

```bash
export PATH="/d/Go/bin:$PATH" && go build ./... && \
go test ./backend/data/ -count=1 -skip 'TestGetHtml|TestUpdateUSName|TestEastMoneyKLineApi|TestRobot|TestInvestCalendar' -run 'Kronos|Review'
```

Expected：构建通过；测试 `ok`（无 FAIL）。若 `-run 'Kronos|Review'` 匹配不到用例导致 `testing: warning: no tests to run`，属正常（覆盖由 Task 6 全量回归兜住）。

- [ ] **Step 3: Commit**

```bash
git add backend/data/daily_pick_review.go backend/data/daily_pick_kronos.go
git commit -m "refactor(data): export review window and kronos hit helpers for khunter reuse"
```

---

### Task 3: khunter 复盘服务（回填 + 对账）

**Files:**
- Create: `backend/data/khunter/review.go`
- Modify: `backend/data/khunter/repo.go`（新增 4 个方法；`import` 加 `"fmt"` 若缺）
- Test: `backend/data/khunter/review_test.go`（新建；复用 `repo_test.go` 的 `setupTestDB`）

**Interfaces:**
- Consumes: Task 1 模型字段；Task 2 的 `data.FindBarAfter / data.KronosHorizon / data.KronosHorizonHit`；`data.KronosPrediction/KronosForecast`；`stockcode.Normalize`（import `go-stock/backend/stockcode`）。
- Produces（Task 4 依赖）：
  - `khunter.NewReview() *Review`
  - `(*Review).RunDailyReview(ctx context.Context) (*ReviewResult, error)`
  - `ReviewResult{HuntingTotal, HuntingReviewed, HuntingPartial, HuntingSkipped, PredTotal, PredJudged, PredPending int}`
  - `(*Repo).HuntingNeedingReview(cutoff string) ([]models.KhunterHunting, error)`
  - `(*Repo).SaveHuntingReview(h *models.KhunterHunting) error`
  - `(*Repo).PredictionsNeedingHit(cutoff string) ([]models.KhunterKronosPrediction, error)`
  - `(*Repo).SavePredictionHit(id uint, hit string, actualPct float64) error`
  - 纯函数 `huntingWindowReturns(klines []data.KLineData, enterDate string, entryPrice float64) (t1, r3, r5 float64, covered bool)`（同包测试直接调用）

- [ ] **Step 1: 写失败测试**

新建 `backend/data/khunter/review_test.go`：

```go
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
```

- [ ] **Step 2: 跑测试确认失败**

```bash
export PATH="/d/Go/bin:$PATH" && go test ./backend/data/khunter/ -run 'TestHuntingWindowReturns|TestRunDailyReview|TestJudgePrediction' -count=1
```

Expected：编译失败（`undefined: NewReview` 等）。

- [ ] **Step 3: 实现 review.go**

新建 `backend/data/khunter/review.go`：

```go
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
	return calc(t1Idx), calc(t1Idx+2), calc(t1Idx+4), true
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
```

`backend/data/khunter/repo.go` 追加 4 个方法（放在 `SaveKronosPredictions` 之后）：

```go
// HuntingNeedingReview 待回填狩猎记录：真实持仓（持仓中/已退出）、未复盘完、进入日在回看窗口内。
// 历史待核（旧迁移记录，非真实持仓）显式排除，避免假样本污染统计。
func (r *Repo) HuntingNeedingReview(cutoff string) ([]models.KhunterHunting, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	var rows []models.KhunterHunting
	err := db.Dao.
		Where("status IN ? AND reviewed = ? AND enter_date >= ?",
			[]string{HuntingStatusHolding, HuntingStatusExited}, false, cutoff).
		Order("enter_date asc").
		Find(&rows).Error
	return rows, err
}

// SaveHuntingReview 保存回填后的狩猎记录。
func (r *Repo) SaveHuntingReview(h *models.KhunterHunting) error {
	if db.Dao == nil {
		return fmt.Errorf("db not initialized")
	}
	return db.Dao.Save(h).Error
}

// PredictionsNeedingHit 待对账预测：有预测明细、未回写 Hit、信号日在回看窗口内。
func (r *Repo) PredictionsNeedingHit(cutoff string) ([]models.KhunterKronosPrediction, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	var rows []models.KhunterKronosPrediction
	err := db.Dao.
		Where("forecast_json <> '' AND hit = '' AND signal_date >= ?", cutoff).
		Order("signal_date asc").
		Find(&rows).Error
	return rows, err
}

// SavePredictionHit 回写预测对账结果（hit/miss/flat）与期末实际涨幅。
func (r *Repo) SavePredictionHit(id uint, hit string, actualPct float64) error {
	if db.Dao == nil {
		return fmt.Errorf("db not initialized")
	}
	return db.Dao.Model(&models.KhunterKronosPrediction{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{"hit": hit, "actual_pct": actualPct}).Error
}
```

- [ ] **Step 4: 跑测试确认通过**

```bash
export PATH="/d/Go/bin:$PATH" && go test ./backend/data/khunter/ -run 'TestHuntingWindowReturns|TestRunDailyReview|TestJudgePrediction|TestEnsureMigrate' -count=1
```

Expected：全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add backend/data/khunter/review.go backend/data/khunter/review_test.go backend/data/khunter/repo.go
git commit -m "feat(khunter): daily review service backfills hunting returns and judges kronos hits"
```

---

### Task 4: cron 接线（khunter_review 注册 + daily_pick 自动化拆除）

**Files:**
- Modify: `backend/agent/cron_task_api.go`（`GetTaskTypes` + `executeTaskByType` + 新 `executeKhunterReview`）
- Modify: `backend/handler/system_handler.go`（`InitCronTasks` 自动创建）
- Modify: `app_lifecycle.go`（删除 Daily Pick cron 段）
- Test: `backend/agent/cron_task_api_test.go`（**已存在**，package agent；追加两个测试函数）

**Interfaces:**
- Consumes: Task 3 的 `khunter.NewReview()` / `RunDailyReview(ctx)` / `ReviewResult` 字段。
- Produces：cron 任务类型 `"khunter_review"`（DB 任务行 + 前端任务类型下拉可见）。

- [ ] **Step 1: 写失败测试**

在既有 `backend/agent/cron_task_api_test.go`（package agent）末尾追加（现有文件已有 `TestCronTaskApiDailyKLineSyncReadiness`，勿重名）：

```go
// 任务类型必须注册到分发器：ctx 已取消时应命中 executeKhunterReview 的守卫并返回 ctx.Err()，
// 而不是落入 default 的"未知任务类型"。
func TestExecuteTaskByTypeKhunterReviewDispatch(t *testing.T) {
	a := NewCronTaskApi()
	task := &models.CronTask{Name: "狩猎场复盘回填", TaskType: "khunter_review"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := a.executeTaskByType(ctx, task)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

// 新任务类型必须出现在类型清单（前端下拉与任务创建依赖）。
func TestGetTaskTypesIncludesKhunterReview(t *testing.T) {
	for _, tt := range NewCronTaskApi().GetTaskTypes() {
		if tt.A == "khunter_review" {
			return
		}
	}
	t.Fatal("khunter_review missing from GetTaskTypes")
}
```

（import 需要 `context`、`errors`、`go-stock/backend/models`，按现有文件 import 块合并补充。）

- [ ] **Step 2: 跑测试确认失败**

```bash
export PATH="/d/Go/bin:$PATH" && go test ./backend/agent/ -run 'TestExecuteTaskByTypeKhunterReviewDispatch|TestGetTaskTypesIncludesKhunterReview' -count=1
```

Expected：FAIL（`expected context.Canceled, got 未知任务类型...`；类型清单测试 FAIL）。

- [ ] **Step 3: 实现接线**

`backend/agent/cron_task_api.go`：
1. `GetTaskTypes()` 的返回切片中，`khunter_daily_pipeline` 项之后追加：
```go
		{A: "khunter_review", B: "狩猎场复盘回填"},
```
2. `executeTaskByType` 的 switch 中，`case "khunter_daily_pipeline"` 之后追加：
```go
	case "khunter_review":
		return a.executeKhunterReview(ctx, task)
```
3. 在 `executeKhunterDailyPipeline` 函数之后新增：
```go
// executeKhunterReview 每日 09:15 狩猎场复盘：回填 T+1/3/5 收益并对账 Kronos 预测命中。
func (a *CronTaskApi) executeKhunterReview(ctx context.Context, task *models.CronTask) error {
	logger.SugaredLogger.Infof("执行狩猎场复盘回填任务：%s", task.Name)
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	res, err := khunter.NewReview().RunDailyReview(ctx)
	if err != nil {
		return err
	}
	logger.SugaredLogger.Infof("khunter_review 完成：狩猎回填 %d（部分 %d，跳过 %d），预测对账 %d（未到期 %d）",
		res.HuntingReviewed, res.HuntingPartial, res.HuntingSkipped, res.PredJudged, res.PredPending)
	return nil
}
```

`backend/handler/system_handler.go` `InitCronTasks` 中，`khunter_daily_pipeline` 的 `ExistsByTaskType` 块之后追加：
```go
	if !cronApi.ExistsByTaskType("khunter_review") {
		task := &models.CronTask{
			Name:        "狩猎场复盘回填",
			CronExpr:    "0 15 9 * * *",
			TaskType:    "khunter_review",
			Enable:      true,
			Status:      "active",
			Description: "每日 09:15 回填狩猎记录 T+1/3/5 收益并对账 Kronos 预测命中（替代 daily_pick 自动复盘）",
		}
		err := cronApi.Create(task)
		if err != nil {
			logger.SugaredLogger.Errorf("自动创建狩猎场复盘回填任务失败：%v", err)
		} else {
			logger.SugaredLogger.Info("已自动创建狩猎场复盘回填定时任务")
		}
	}
```

`app_lifecycle.go`：删除整段 Daily Pick cron（从注释 `// ---- Daily Pick cron tasks ----` 到 `a.setCronEntry("dailyPickAutoReview", reviewEntryID)` 的右大括号止，含 `dailyPickSvc := data.NewDailyPickService()` 与两个 `AddFunc` 块）。删除后若 `data` import 变未使用，`go build` 会报，按报错删除该 import（本文件其它代码仍引用 `data` 的可能性高，以编译器为准）。

- [ ] **Step 4: 跑测试确认通过 + 构建**

```bash
export PATH="/d/Go/bin:$PATH" && go build ./... && \
go test ./backend/agent/ -run 'TestExecuteTaskByTypeKhunterReviewDispatch|TestGetTaskTypesIncludesKhunterReview' -count=1
```

Expected：构建通过；两个测试 PASS。

- [ ] **Step 5: Commit**

```bash
git add backend/agent/cron_task_api.go backend/agent/cron_task_api_test.go backend/handler/system_handler.go app_lifecycle.go
git commit -m "feat(khunter): wire khunter_review cron and retire daily_pick automation"
```

---

### Task 5: 因子统计与调权切源（khunter 侧）

**Files:**
- Create: `backend/data/khunter/kronos_stats.go`
- Modify: `backend/data/khunter/pipeline.go:525`（因子配置加载后应用调权）
- Test: `backend/data/khunter/kronos_stats_test.go`（新建）

**Interfaces:**
- Consumes: Task 1 的 `Hit/ActualPct` 字段；`data.KronosFactorStats` 结构（已存在，`Samples/Hits/HitRate/AvgPred/AvgT1/CurrentW`）；Task 3 的 repo 查询模式。
- Produces（阶段 3 看板与前端复用）：
  - `khunter.KronosFactorStats(ctx context.Context) data.KronosFactorStats`
  - `khunter.KronosFactorMultiplier(ctx context.Context) float64`
  - `khunter.AdjustedKronosWeight(ctx context.Context, cfg KronosFactorConfig) KronosFactorConfig`
- 语义（迁移自 `daily_pick_kronos.go` 同口径）：统计窗 60 自然日、成熟 7 自然日、最小样本 20；命中率 ≥55% 上浮（系数 ≤3）、<45% 降权 0.5、<40% 归零；样本不足/异常恒为 1（不调权）。

- [ ] **Step 1: 写失败测试**

新建 `backend/data/khunter/kronos_stats_test.go`：

```go
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
			Hit: hit, ActualPct: 3.5,
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
```

- [ ] **Step 2: 跑测试确认失败**

```bash
export PATH="/d/Go/bin:$PATH" && go test ./backend/data/khunter/ -run 'TestKronosFactor|TestAdjustedKronosWeight' -count=1
```

Expected：编译失败（undefined 函数）。

- [ ] **Step 3: 实现 kronos_stats.go**

新建 `backend/data/khunter/kronos_stats.go`：

```go
package khunter

import (
	"context"
	"encoding/json"
	"math"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// 因子统计与调权口径迁移自 daily_pick_kronos.go（同口径保证历史可比）：
// 统计窗 60 自然日、样本成熟 7 自然日（≈5 个交易日，保证预测窗口走完）、调权最小样本 20。
const (
	factorStatsHistoryDays = 60
	factorStatsMatureDays  = 7
	factorMinSamples       = 20
)

// KronosFactorStats 读取近窗已对账预测，输出命中率统计（khunter 数据源，替代 daily_pick 口径）。
func KronosFactorStats(ctx context.Context) data.KronosFactorStats {
	var stats data.KronosFactorStats
	if db.Dao == nil {
		return stats
	}
	cutoff := time.Now().AddDate(0, 0, -factorStatsHistoryDays).Format("2006-01-02")
	mature := time.Now().AddDate(0, 0, -factorStatsMatureDays).Format("2006-01-02")
	var rows []models.KhunterKronosPrediction
	err := db.Dao.WithContext(ctx).
		Where("hit IN ? AND signal_date >= ? AND signal_date <= ?",
			[]string{"hit", "miss"}, cutoff, mature).
		Find(&rows).Error
	if err != nil {
		logger.SugaredLogger.Warnf("khunter 因子统计查询失败: %v", err)
		return stats
	}
	var predSum, actualSum float64
	var actualN int
	for _, p := range rows {
		stats.Samples++
		if p.Hit == "hit" {
			stats.Hits++
		}
		if p.ForecastJSON != "" {
			var pred data.KronosPrediction
			if err := json.Unmarshal([]byte(p.ForecastJSON), &pred); err == nil && pred.Summary != nil {
				predSum += pred.Summary.ChangePct
			}
		}
		if p.ActualPct != 0 {
			actualSum += p.ActualPct
			actualN++
		}
	}
	if stats.Samples > 0 {
		stats.HitRate = math.Round(float64(stats.Hits)/float64(stats.Samples)*1000) / 1000
		stats.AvgPred = math.Round(predSum/float64(stats.Samples)*100) / 100
	}
	if actualN > 0 {
		stats.AvgT1 = math.Round(actualSum/float64(actualN)*100) / 100
	}
	return stats
}

// KronosFactorMultiplier 复盘驱动的因子调权系数：
// 命中率 ≥55% 上浮（最高 3 倍），[45%,55%) 不调，<45% 降权一半，<40% 归零；
// 样本 <20 或异常回落 1.0（等价于不调权）。
func KronosFactorMultiplier(ctx context.Context) float64 {
	stats := KronosFactorStats(ctx)
	if stats.Samples < factorMinSamples {
		return 1.0
	}
	switch {
	case stats.HitRate < 0.40:
		logger.SugaredLogger.Infof("khunter: Kronos 因子命中率 %.1f%% < 40%%，权重归零", stats.HitRate*100)
		return 0
	case stats.HitRate < 0.45:
		return 0.5
	case stats.HitRate >= 0.55:
		m := 1 + (stats.HitRate-0.5)*10 // 55%→1.5, 60%→2.0, 65%→2.5, ≥70%→3.0
		if m > 3 {
			m = 3
		}
		return math.Round(m*100) / 100
	default:
		return 1.0
	}
}

// AdjustedKronosWeight 应用调权系数后的因子权重（钳位 [0, MaxWeight]），
// 应用点在流水线因子挂接处；样本不足时系数为 1，行为与阶段 1 完全一致。
func AdjustedKronosWeight(ctx context.Context, cfg KronosFactorConfig) KronosFactorConfig {
	mult := KronosFactorMultiplier(ctx)
	if mult == 1 {
		return cfg
	}
	w := math.Round(cfg.Weight*mult*1000) / 1000
	if w > cfg.MaxWeight {
		w = cfg.MaxWeight
	}
	if w < 0 {
		w = 0
	}
	logger.SugaredLogger.Infof("khunter: Kronos 因子调权 命中率系数 %.2f → 权重 %.3f", mult, w)
	cfg.Weight = w
	return cfg
}
```

`backend/data/khunter/pipeline.go` 第 525 行，把：
```go
	if kcfg := LoadKronosFactorConfig(""); kcfg.Enable && len(observations) > 0 {
```
改为（调权在进 goroutine 前完成，持久化的 Weight 即实际生效权重）：
```go
	if kcfg := LoadKronosFactorConfig(""); kcfg.Enable && len(observations) > 0 {
		kcfg = AdjustedKronosWeight(context.Background(), kcfg)
```
（后续 `go func(..., kcfg)` 形参已按值拷贝调权后的配置，goroutine 体内无需再改。）

- [ ] **Step 4: 跑测试确认通过**

```bash
export PATH="/d/Go/bin:$PATH" && go test ./backend/data/khunter/ -count=1
```

Expected：`ok go-stock/backend/data/khunter`（含 Task 1/3/5 全部测试）。

- [ ] **Step 5: Commit**

```bash
git add backend/data/khunter/kronos_stats.go backend/data/khunter/kronos_stats_test.go backend/data/khunter/pipeline.go
git commit -m "feat(khunter): factor stats and auto weight from khunter review data"
```

---

### Task 6: 端到端验证（回放对照 + 回归 + 交叉验证）

**Files:**
- Create: `backend/data/khunter/review_verify_test.go`（手动验证用，默认 Skip）

**Interfaces:**
- Consumes: 全部前置任务；验证用 DB 副本。

- [ ] **Step 1: 回放对照（证主链未被触碰）**

```bash
export PATH="/d/Go/bin:$PATH"
go build -o /tmp/replay-phase2-head.exe ./backend/cmd/khunterreplay
/tmp/replay-phase2-head.exe -db /tmp/phase2-replay-after.db -start 2026-09-01 -end 2026-11-17 \
  -variants full -config config/khunter_risk_control.json \
  -cases '688322.SH:2026-11-11,300780.SZ:2026-10-29,600259.SH:2026-11-03,300829.SZ:2026-11-12,300969.SZ:2026-10-29' \
  -out "$TEMP/phase2-five-cases-after.json"
python - <<'PY'
import json, os
tmp = os.environ["TEMP"]
a = json.load(open(os.path.join(tmp, "phase2-five-cases-before.json"), encoding="utf-8"))
b = json.load(open(os.path.join(tmp, "phase2-five-cases-after.json"), encoding="utf-8"))
def flat(o, p=""):
    if isinstance(o, dict):
        for k, v in o.items(): yield from flat(v, p + "/" + k)
    elif isinstance(o, list):
        for i, v in enumerate(o): yield from flat(v, p + f"[{i}]")
    else: yield p, o
fa, fb = dict(flat(a)), dict(flat(b))
diff = {k: (fa.get(k), fb.get(k)) for k in set(fa) | set(fb) if fa.get(k) != fb.get(k)}
print("DIFF COUNT", len(diff))
for k, v in list(diff.items())[:40]: print(k, v)
raise SystemExit(1 if diff else 0)
PY
```

Expected：`DIFF COUNT 0`，退出码 0。**任何差异都是回归**——查明并修复后再继续。

- [ ] **Step 2: 定向回归**

```bash
export PATH="/d/Go/bin:$PATH"
go build ./... && go vet ./backend/... && \
go test ./backend/data/khunter/ ./backend/data/ ./backend/handler/ ./backend/agent/ -count=1 \
  -skip 'TestGetHtml|TestUpdateUSName|TestEastMoneyKLineApi|TestRobot|TestInvestCalendar'
```

Expected：全部 `ok`。（已知环境性失败用例已跳过。）

- [ ] **Step 3: 交叉验证（手动，对照 daily_picks 窗口收盘价）**

新建 `backend/data/khunter/review_verify_test.go`：

```go
package khunter

import (
	"os"
	"testing"

	"go-stock/backend/db"
)

// 手动验证（默认 Skip）：对真实库副本跑一轮复盘，再抽查已回填记录，
// 与 daily_picks 中同 (code, 日期) 的窗口收盘价比对（收益因基准不同不比）。
// 运行：KHUNTER_VERIFY_DB=/path/copy.db go test ./backend/data/khunter/ -run TestVerifyReviewAgainstDB -v
func TestVerifyReviewAgainstDB(t *testing.T) {
	path := os.Getenv("KHUNTER_VERIFY_DB")
	if path == "" {
		t.Skip("set KHUNTER_VERIFY_DB to a DB copy to run manual verification")
	}
	db.Init(path)
	if err := EnsureMigrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	res, err := NewReview().RunDailyReview(t.Context())
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	t.Logf("review result: %+v", res)
	if res.HuntingReviewed+res.HuntingPartial == 0 && res.PredJudged == 0 {
		t.Log("nothing to backfill in this copy; pick a DB with unreviewed holdings/predictions")
	}
}
```

跑（对只读副本）：
```bash
export PATH="/d/Go/bin:$PATH"
cp /d/_installers/go-stock/replay-verify/stock.db "$TEMP/verify-copy.db"
cp /d/_installers/go-stock/replay-verify/stock.db-wal "$TEMP/verify-copy.db-wal" 2>/dev/null
KHUNTER_VERIFY_DB="$TEMP/verify-copy.db" go test ./backend/data/khunter/ -run TestVerifyReviewAgainstDB -v -count=1
```

Expected：输出 `review result` 含非零回填/对账数（库内有 11 月以来狩猎记录与预测明细）；若有零值，按日志抽查原因（多数情况是该库副本无未复盘记录，换安装库副本重跑）。

人工抽查（写入验证记录）：从 daily_picks 取一条已复盘记录（有 NextClose），与 verify-copy 中同一股票对应交易日 K 线收盘比对，**收盘价必须一致**；收益不比（基准不同：daily_pick=推荐日收盘，khunter=EntryPrice）。

- [ ] **Step 4: 部署后门控（记录，非本计划可自动完成）**

在计划文件末尾打勾记录：`khunter_review` 首次实跑 = 部署后次日 09:15，查日志 `khunter_review 完成：…`；调权生效观察 = 再下一日 17:00 流水线日志中 `Kronos 因子调权 命中率系数 …`（前提：enable:true 且样本 ≥20）。

- [ ] **Step 5: Commit（验证用测试文件）**

```bash
git add backend/data/khunter/review_verify_test.go
git commit -m "test(khunter): manual review verification harness against db copy"
```

---

### Task 7: 收尾

- [ ] **Step 1: 最终状态确认**

```bash
export PATH="/d/Go/bin:$PATH" && go build ./... && git status --short && git log --oneline -6
```

Expected：工作区干净（仅存量未跟踪文件）；log 依次为本计划 6 个 commit。

- [ ] **Step 2: 汇总验证结果并回报**

按 Task 1 Step 0 / Task 6 Step 1-3 的实际输出汇总：回放 DIFF COUNT、回归 ok 清单、交叉验证 review result 数字。若全部通过，向用户报告并提醒部署门控（Task 6 Step 4）。
