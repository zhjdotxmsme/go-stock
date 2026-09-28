# 狩猎场系统（KHunter 移植版）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 go-stock 中新增并行的"狩猎场"量化体系：5 个 K 线形态策略 + 五维评分（35/35/10/10/10 + 一票否决）+ VaR 四档风控 + 半凯利仓位 + 回测适配 + 前端狩猎场页面。

**Architecture:** 独立模块 `backend/data/khunter/`（strategy/scorer/risk 子包 + pipeline/repo/service），表统一 `khunter_` 前缀，懒加载 AutoMigrate；复用 `kline_bars` 本地 K 线、`backend/data/indicator` 指标库、现有 `data/backtest` 引擎、robfig/cron 定时体系、naive-ui + ECharts 前端。详细设计见 `.openteams/specs/2026-09-25-khunter-hunting-ground-design.html`。

**Tech Stack:** Go 1.26（module `go-stock`）、GORM + glebarez/sqlite、robfig/cron/v3、Wails v2、Vue3 + naive-ui + ECharts。

## Global Constraints

- 所有后端包导入前缀 `go-stock/backend/...`。
- K 线统一**正序**（`TradeDate` 升序，index 0 = 最早）；"前 N 日均量"= 不含当日前 N 日均值（对齐 KHunter `shift(1)` 口径）。
- EMA/MACD 用 `indicator.EMA/MACD`（递推式，对齐 pandas `ewm(adjust=False)`）。
- 测试只用标准库 `testing`，不用 testify；测试文件与被测代码同包。
- 新表在 `backend/models/khunter_models.go`，AutoMigrate 在 repo 首用时懒执行。
- 不复制 KHunter 源码文本（许可证 NOASSERTION）；参数与逻辑以 spec 第 4/5/6 节为准。
- 浮点比较用容差 `1e-6`，禁止直接 `==`。
- 过滤 ST/退市股：名称含 `退`/`未知`/`已退` 或以 `ST`/`*ST` 开头。
- 调度用 `models.CronTask` + `agent.NewCronTaskApi()` 体系，6 段 cron 表达式（含秒）。
- 前端组件放 `frontend/src/components/`，API 封装放 `frontend/src/api/`，路由注册在 `frontend/src/router/router.js`。

## 任务总览

| # | 任务 | 依赖 |
|---|---|---|
| 1 | 数据模型 + repo | — |
| 2 | 策略框架 + K线工具 | 1 |
| 3 | 仙人指路策略 | 2 |
| 4 | 涨停回马枪策略 | 2 |
| 5 | W底策略 | 2 |
| 6 | 底部趋势拐点策略 | 2 |
| 7 | 主升低吸策略 | 2 |
| 8 | 技术面评分器 | 2（Signals 表） |
| 9 | 资金流落库 + 资金面评分器 | 1 |
| 10 | 基本面评分器 | 1 |
| 11 | 板块评分器 | 1 |
| 12 | 事件分类器 + 事件评分器 | 1 |
| 13 | 综合评分引擎 | 8-12 |
| 14 | VaR + 风险四档 | 1 |
| 15 | 半凯利仓位 | — |
| 16 | 流水线 + service 门面 | 2-15 |
| 17 | 回测适配器 | 2, 15 |
| 18 | handler + Wails 绑定 + 定时任务 | 16 |
| 19 | 前端狩猎场页面 | 18 |
| 20 | 端到端验证 | 19 |

---

### Task 1: 数据模型 + repo

**Files:**
- Create: `backend/models/khunter_models.go`
- Create: `backend/data/khunter/repo.go`
- Test: `backend/data/khunter/repo_test.go`

**Interfaces:**
- Produces（后续所有任务依赖）:
  - `models.KhunterSignal{ID, Code, Name, Strategy, SignalDate, KeyDate, KeyDateType, Close, VolumeRatio float64→见下, Reasons, Details string(JSON), CreatedAt}` — 字段类型见下方完整定义
  - `models.KhunterScore`, `models.KhunterHunting`, `models.KhunterMoneyFlowDaily`, `models.KhunterRiskLevel`, `models.KhunterEvent`
  - `khunter.Repo` 方法集：`SaveSignals / SaveScores / SaveHunting / UpsertMoneyFlow / SaveRiskLevel / SaveEvents / GetScoresByDate / GetSignalsByDate / GetHuntingList / GetMoneyFlow(code, days) / GetLatestRiskLevel()`

- [ ] **Step 1: 写失败测试**

```go
// backend/data/khunter/repo_test.go
package khunter

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

// setupTestDB 用临时文件库替换全局 db.Dao，测试后恢复
func setupTestDB(t *testing.T) {
	t.Helper()
	old := db.Dao
	d, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	db.Dao = d
	t.Cleanup(func() { db.Dao = old })
	if err := EnsureMigrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
}

func TestRepoSignalAndScoreRoundTrip(t *testing.T) {
	setupTestDB(t)
	r := NewRepo()

	sig := models.KhunterSignal{Code: "600519", Name: "贵州茅台", Strategy: "仙人指路",
		SignalDate: "2026-09-24", Close: 1700.5, VolumeRatio: 2.1, Reasons: `["冲高8%","反包确认"]`}
	if err := r.SaveSignals([]models.KhunterSignal{sig}); err != nil {
		t.Fatalf("SaveSignals: %v", err)
	}
	sigs, err := r.GetSignalsByDate("2026-09-24")
	if err != nil || len(sigs) != 1 || sigs[0].Strategy != "仙人指路" {
		t.Fatalf("GetSignalsByDate: %v %+v", err, sigs)
	}

	sc := models.KhunterScore{Code: "600519", ScoreDate: "2026-09-24", Technical: 70, Moneyflow: 40,
		Fundamental: 60, Sector: 50, Event: 50, Total: 52.5, Level: "中性"}
	if err := r.SaveScores([]models.KhunterScore{sc}); err != nil {
		t.Fatalf("SaveScores: %v", err)
	}
	scores, err := r.GetScoresByDate("2026-09-24")
	if err != nil || len(scores) != 1 || scores[0].Total != 52.5 {
		t.Fatalf("GetScoresByDate: %v %+v", err, scores)
	}
}

func TestRepoMoneyFlowUpsert(t *testing.T) {
	setupTestDB(t)
	r := NewRepo()
	rows := []models.KhunterMoneyFlowDaily{
		{Code: "600519", Date: "2026-09-23", MainNet: 1.2e8, LgNetRatio: 3.5},
		{Code: "600519", Date: "2026-09-23", MainNet: 2.0e8, LgNetRatio: 5.0}, // 重复键 → 覆盖
	}
	if err := r.UpsertMoneyFlow(rows); err != nil {
		t.Fatalf("UpsertMoneyFlow: %v", err)
	}
	got, err := r.GetMoneyFlow("600519", 5)
	if err != nil || len(got) != 1 || got[0].MainNet != 2.0e8 {
		t.Fatalf("GetMoneyFlow: %v %+v", err, got)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./backend/data/khunter/ -run TestRepo -v`
Expected: 编译失败，`NewRepo`/`EnsureMigrate` 未定义。

- [ ] **Step 3: 实现模型**

```go
// backend/models/khunter_models.go
package models

import "time"

// KhunterSignal 形态策略信号
type KhunterSignal struct {
	ID          uint      `gorm:"primarykey"`
	Code        string    `gorm:"size:20;index:idx_khs_code_date"`
	Name        string    `gorm:"size:50"`
	Strategy    string    `gorm:"size:50;index:idx_khs_strategy_date"`
	SignalDate  string    `gorm:"size:10;index:idx_khs_code_date;index:idx_khs_strategy_date"`
	KeyDate     string    `gorm:"size:10"`
	KeyDateType string    `gorm:"size:20"`
	Close       float64
	VolumeRatio float64
	Reasons     string    `gorm:"type:text"` // JSON array
	Details     string    `gorm:"type:text"` // JSON object
	CreatedAt   time.Time
}

func (KhunterSignal) TableName() string { return "khunter_signals" }

// KhunterScore 五维评分
type KhunterScore struct {
	ID          uint      `gorm:"primarykey"`
	Code        string    `gorm:"size:20;index:idx_khsc_code_date,unique"`
	ScoreDate   string    `gorm:"size:10;index:idx_khsc_code_date,unique"`
	Technical   float64
	Moneyflow   float64
	Fundamental float64
	Sector      float64
	Event       float64
	Total       float64
	Level       string `gorm:"size:20"` // 强烈推荐/推荐/中性/谨慎/回避/淘汰
	VetoReason  string `gorm:"size:200"`
	Degraded    bool   // 某维度数据缺失按中性分处理
	Details     string `gorm:"type:text"`
	CreatedAt   time.Time
}

func (KhunterScore) TableName() string { return "khunter_scores" }

// KhunterHunting 狩猎场
type KhunterHunting struct {
	ID           uint      `gorm:"primarykey"`
	Code         string    `gorm:"size:20;index"`
	Name         string    `gorm:"size:50"`
	EnterDate    string    `gorm:"size:10"`
	EnterScore   float64
	SupportPrice float64
	Status       string    `gorm:"size:10"` // 追踪中 / 已移除
	TrackDays    int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (KhunterHunting) TableName() string { return "khunter_hunting" }

// KhunterMoneyFlowDaily 资金流历史落库
type KhunterMoneyFlowDaily struct {
	ID        uint      `gorm:"primarykey"`
	Code      string    `gorm:"size:20;index:idx_khmf_code_date,unique"`
	Date      string    `gorm:"size:10;index:idx_khmf_code_date,unique"`
	MainNet   float64   // 主力净额（元）
	MainRatio float64   // 主力净占比（%）
	SuperLgNet float64  // 超大单净额
	LgNet     float64   // 大单净额
	LgNetRatio float64  // 大单净占比（%）
	MdNet     float64
	SmNet     float64   // 小单净额
	SmNetRatio float64  // 小单净占比（%）
	CreatedAt time.Time
}

func (KhunterMoneyFlowDaily) TableName() string { return "khunter_money_flow_daily" }

// KhunterRiskLevel 每日大盘风险档位
type KhunterRiskLevel struct {
	ID            uint      `gorm:"primarykey"`
	Date          string    `gorm:"size:10;uniqueIndex"`
	Var1d         float64
	Var5d         float64
	Level         string `gorm:"size:10"` // 正常/注意/危险/崩溃
	PositionLimit float64
	ScoreExtra    float64
	CreatedAt     time.Time
}

func (KhunterRiskLevel) TableName() string { return "khunter_risk_level" }

// KhunterEvent 事件打标结果
type KhunterEvent struct {
	ID         uint      `gorm:"primarykey"`
	Code       string    `gorm:"size:20;index:idx_khev_code_date"`
	EventType  string    `gorm:"size:30"` // 业绩预增/股东增持/股东减持/...
	EventDate  string    `gorm:"size:10;index:idx_khev_code_date"`
	Score      float64
	ExpireDate string    `gorm:"size:10"`
	Source     string    `gorm:"size:50"`
	CreatedAt  time.Time
}

func (KhunterEvent) TableName() string { return "khunter_events" }
```

- [ ] **Step 4: 实现 repo**

```go
// backend/data/khunter/repo.go
package khunter

import (
	"go-stock/backend/db"
	"go-stock/backend/models"

	"gorm.io/gorm/clause"
)

// EnsureMigrate 懒迁移 khunter 全部表（幂等）
func EnsureMigrate() error {
	if db.Dao == nil {
		return nil
	}
	return db.Dao.AutoMigrate(
		&models.KhunterSignal{}, &models.KhunterScore{}, &models.KhunterHunting{},
		&models.KhunterMoneyFlowDaily{}, &models.KhunterRiskLevel{}, &models.KhunterEvent{},
	)
}

type Repo struct{}

func NewRepo() *Repo { return &Repo{} }

func (r *Repo) SaveSignals(rows []models.KhunterSignal) error {
	if len(rows) == 0 {
		return nil
	}
	return db.Dao.Create(&rows).Error
}

func (r *Repo) GetSignalsByDate(date string) ([]models.KhunterSignal, error) {
	var rows []models.KhunterSignal
	err := db.Dao.Where("signal_date = ?", date).Order("strategy, code").Find(&rows).Error
	return rows, err
}

func (r *Repo) SaveScores(rows []models.KhunterScore) error {
	if len(rows) == 0 {
		return nil
	}
	return db.Dao.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "code"}, {Name: "score_date"}},
		UpdateAll: true,
	}).Create(&rows).Error
}

func (r *Repo) GetScoresByDate(date string) ([]models.KhunterScore, error) {
	var rows []models.KhunterScore
	err := db.Dao.Where("score_date = ?", date).Order("total DESC").Find(&rows).Error
	return rows, err
}

func (r *Repo) UpsertMoneyFlow(rows []models.KhunterMoneyFlowDaily) error {
	if len(rows) == 0 {
		return nil
	}
	return db.Dao.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "code"}, {Name: "date"}},
		UpdateAll: true,
	}).Create(&rows).Error
}

// GetMoneyFlow 返回最近 days 天资金流，正序（日期升序）
func (r *Repo) GetMoneyFlow(code string, days int) ([]models.KhunterMoneyFlowDaily, error) {
	var rows []models.KhunterMoneyFlowDaily
	err := db.Dao.Where("code = ?", code).Order("date DESC").Limit(days).Find(&rows).Error
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return rows, err
}

func (r *Repo) SaveHunting(h *models.KhunterHunting) error {
	return db.Dao.Save(h).Error
}

func (r *Repo) GetHuntingList(status string) ([]models.KhunterHunting, error) {
	var rows []models.KhunterHunting
	q := db.Dao.Order("enter_date DESC")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	return rows, q.Find(&rows).Error
}

func (r *Repo) SaveRiskLevel(rl *models.KhunterRiskLevel) error {
	return db.Dao.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "date"}},
		UpdateAll: true,
	}).Create(rl).Error
}

func (r *Repo) GetLatestRiskLevel() (*models.KhunterRiskLevel, error) {
	var row models.KhunterRiskLevel
	err := db.Dao.Order("date DESC").First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *Repo) SaveEvents(rows []models.KhunterEvent) error {
	if len(rows) == 0 {
		return nil
	}
	return db.Dao.Create(&rows).Error
}

// GetActiveEvents 查询 code 在 date 当天仍在有效期内的事件
func (r *Repo) GetActiveEvents(code, date string) ([]models.KhunterEvent, error) {
	var rows []models.KhunterEvent
	err := db.Dao.Where("code = ? AND event_date <= ? AND expire_date >= ?", code, date, date).
		Order("event_date DESC").Find(&rows).Error
	return rows, err
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./backend/data/khunter/ -run TestRepo -v`
Expected: PASS（2 个测试）。

- [ ] **Step 6: Commit**

```bash
git add backend/models/khunter_models.go backend/data/khunter/repo.go backend/data/khunter/repo_test.go
git commit -m "feat(khunter): 数据模型与 repo（6 张 khunter_ 表）"
```

---

### Task 2: 策略框架 + K线工具

**Files:**
- Create: `backend/data/khunter/strategy/strategy.go`（接口 + 注册表 + Signal）
- Create: `backend/data/khunter/strategy/klutil.go`（正序 K 线工具函数）
- Create: `backend/data/khunter/strategy/loader.go`（从 kline_bars 加载）
- Test: `backend/data/khunter/strategy/klutil_test.go`

**Interfaces:**
- Consumes: `models.KLineBar`（`backend/models/kline_models.go`）、`datasource.NewKLineStore().QueryKLines(ctx, code, period, startDate, endDate string, adjusted bool) ([]models.KLineBar, error)`
- Produces（Task 3-7、16、17 依赖）:
  - `strategy.Signal{StrategyName, Code, Name, Date, KeyDate, KeyDateType string; Close, VolumeRatio float64; Reasons []string; Details map[string]any}`
  - `strategy.Strategy` 接口：`Name() string`、`Weight() int`、`MinBars() int`、`Select(bars []models.KLineBar, stockName string, selectionDate string) *Signal`（bars 正序）
  - `strategy.Registry()` → 返回全部已注册策略 `[]Strategy`
  - 工具：`Closes/Highs/Lows/Opens/Volumes(bars) []float64`、`AvgVolumeBefore(vols []float64, idx, n int) float64`、`IsInvalidName(name string) bool`、`PctChange(bars, i) float64`、`SwingHighs(highs []float64, leftRight int) []int`（返回正序下标）、`LoadDailyBars(ctx, code string, minBars int) ([]models.KLineBar, error)`（前复权）

- [ ] **Step 1: 写失败测试**

```go
// backend/data/khunter/strategy/klutil_test.go
package strategy

import (
	"fmt"
	"testing"
	"time"

	"go-stock/backend/models"
)

// mkBars 按正序生成 K 线；o/h/l/c/v 等长
func mkBars(dates []string, o, h, l, c []float64, v []int64) []models.KLineBar {
	bars := make([]models.KLineBar, len(dates))
	for i := range dates {
		prev := 0.0
		if i > 0 {
			prev = c[i-1]
		}
		bars[i] = models.KLineBar{StockCode: "600000", Period: "day", TradeDate: dates[i],
			Adjusted: true, Open: o[i], High: h[i], Low: l[i], Close: c[i], PrevClose: prev, Volume: v[i]}
	}
	return bars
}

// genDates 生成 n 个连续日期字符串（不需要是真实交易日，策略只看顺序）
func genDates(n int) []string {
	dates := make([]string, n)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		dates[i] = base.AddDate(0, 0, i).Format("2006-01-02")
	}
	return dates
}

// mkConst 生成等长常量序列的快捷方式（构造测试数据用）
func mkConst(n int, val float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = val
	}
	return out
}

// mkVol 生成等长常量成交量
func mkVol(n int, val int64) []int64 {
	out := make([]int64, n)
	for i := range out {
		out[i] = val
	}
	return out
}

var _ = fmt.Sprintf // 保留 fmt 供各策略测试打印调试

func TestAvgVolumeBefore(t *testing.T) {
	vols := []float64{10, 20, 30, 40, 50, 60}
	// idx=5，前5日均量 = (10+20+30+40+50)/5 = 30，不含 idx=5 的 60
	if got := AvgVolumeBefore(vols, 5, 5); got != 30 {
		t.Fatalf("expect 30, got %v", got)
	}
	// 数据不足返回 0
	if got := AvgVolumeBefore(vols, 3, 5); got != 0 {
		t.Fatalf("expect 0, got %v", got)
	}
}

func TestIsInvalidName(t *testing.T) {
	for _, n := range []string{"ST中安", "*ST左江", "退市博天", "未知"} {
		if !IsInvalidName(n) {
			t.Fatalf("%s 应被过滤", n)
		}
	}
	if IsInvalidName("贵州茅台") {
		t.Fatal("正常名称不应被过滤")
	}
}

func TestPctChange(t *testing.T) {
	bars := mkBars(genDates(3),
		[]float64{10, 10, 10}, []float64{10, 10, 11}, []float64{10, 10, 10},
		[]float64{10, 10, 11}, []int64{1, 1, 1})
	if got := PctChange(bars, 2); got < 0.099 || got > 0.101 {
		t.Fatalf("expect ~0.10, got %v", got)
	}
	if got := PctChange(bars, 0); got != 0 {
		t.Fatalf("首根无前收，expect 0, got %v", got)
	}
}

func TestSwingHighs(t *testing.T) {
	highs := []float64{1, 3, 2, 5, 4, 6, 1, 2, 1}
	// leftRight=2：i=3(5) 左右两根均小于它 → 是；i=5(6) 右侧只有 3 根且 1<6,2<6 但右窗需满 2 → i=5 右侧有 [1,2] 满足
	got := SwingHighs(highs, 2)
	if len(got) != 2 || got[0] != 3 || got[1] != 5 {
		t.Fatalf("expect [3 5], got %v", got)
	}
	// 最右侧 2 根不参与（右窗不足）
	highs2 := []float64{1, 3, 2, 5, 4, 6, 1, 9, 1}
	got2 := SwingHighs(highs2, 2)
	for _, i := range got2 {
		if i == 7 {
			t.Fatalf("i=7 右窗不足 2，不应成为 swing high")
		}
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./backend/data/khunter/strategy/ -v`
Expected: 编译失败。

- [ ] **Step 3: 实现 strategy.go**

```go
// backend/data/khunter/strategy/strategy.go
package strategy

import "go-stock/backend/models"

// Signal 统一选股信号（对齐 KHunter 输出格式）
type Signal struct {
	StrategyName string
	Code         string
	Name         string
	Date         string  // 信号日（= 选股日）
	KeyDate      string  // 关键日期（形态信号日/突破日等）
	KeyDateType  string  // 关键日期类型
	Close        float64 // 选股日收盘
	VolumeRatio  float64 // 选股日量比（对前5日均量，不含当日）
	Reasons      []string
	Details      map[string]any
}

// Strategy 形态策略接口。bars 为正序（index 0 = 最早）。
// 返回 nil 表示未命中。
type Strategy interface {
	Name() string
	Weight() int   // 技术面评分权重
	MinBars() int  // 最少 K 线根数
	Select(bars []models.KLineBar, stockName, selectionDate string) *Signal
}

var registry []Strategy

func register(s Strategy) { registry = append(registry, s) }

// Registry 返回全部已注册策略（各策略文件 init() 中注册）
func Registry() []Strategy { return registry }
```

- [ ] **Step 4: 实现 klutil.go**

```go
// backend/data/khunter/strategy/klutil.go
package strategy

import (
	"math"
	"strings"

	"go-stock/backend/models"
)

const eps = 1e-6

func feq(a, b float64) bool { return math.Abs(a-b) < eps }

func Closes(bars []models.KLineBar) []float64 {
	out := make([]float64, len(bars))
	for i, b := range bars {
		out[i] = b.Close
	}
	return out
}

func Opens(bars []models.KLineBar) []float64 {
	out := make([]float64, len(bars))
	for i, b := range bars {
		out[i] = b.Open
	}
	return out
}

func Highs(bars []models.KLineBar) []float64 {
	out := make([]float64, len(bars))
	for i, b := range bars {
		out[i] = b.High
	}
	return out
}

func Lows(bars []models.KLineBar) []float64 {
	out := make([]float64, len(bars))
	for i, b := range bars {
		out[i] = b.Low
	}
	return out
}

func Volumes(bars []models.KLineBar) []float64 {
	out := make([]float64, len(bars))
	for i, b := range bars {
		out[i] = float64(b.Volume)
	}
	return out
}

// AvgVolumeBefore idx 日之前（不含 idx）前 n 日均量；数据不足返回 0
func AvgVolumeBefore(vols []float64, idx, n int) float64 {
	if idx-n < 0 {
		return 0
	}
	sum := 0.0
	for i := idx - n; i < idx; i++ {
		sum += vols[i]
	}
	return sum / float64(n)
}

// IsInvalidName 过滤 ST/退市/异常名称
func IsInvalidName(name string) bool {
	if name == "" {
		return false
	}
	if strings.HasPrefix(name, "ST") || strings.HasPrefix(name, "*ST") {
		return true
	}
	for _, kw := range []string{"退", "未知", "已退"} {
		if strings.Contains(name, kw) {
			return true
		}
	}
	return false
}

// PctChange 第 i 根涨幅（对前收）；i=0 返回 0
func PctChange(bars []models.KLineBar, i int) float64 {
	if i <= 0 || i >= len(bars) {
		return 0
	}
	prev := bars[i-1].Close
	if prev == 0 {
		return 0
	}
	return (bars[i].Close - prev) / prev
}

// SwingHighs 返回正序下标：high[i] 严格大于左右各 leftRight 根的最高价。
// 右侧不足 leftRight 根的下标不参与（无未来函数）。
func SwingHighs(highs []float64, leftRight int) []int {
	var out []int
	for i := leftRight; i+leftRight < len(highs); i++ {
		ok := true
		for j := i - leftRight; j <= i+leftRight; j++ {
			if j == i {
				continue
			}
			if highs[j] >= highs[i] {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, i)
		}
	}
	return out
}

// IsLocalLow low[i] 为前后 window 根内最低（容差 eps）
func IsLocalLow(lows []float64, i, window int) bool {
	if i-window < 0 || i+window >= len(lows) {
		return false
	}
	for j := i - window; j <= i+window; j++ {
		if lows[j] < lows[i]-eps {
			return false
		}
	}
	return true
}
```

- [ ] **Step 5: 实现 loader.go**

```go
// backend/data/khunter/strategy/loader.go
package strategy

import (
	"context"
	"time"

	"go-stock/backend/data/datasource"
	"go-stock/backend/models"
)

// LoadDailyBars 从本地 kline_bars 读前复权日K（正序），不足 minBars 返回 nil。
// lookback 固定 400 自然日（≈270 个交易日），覆盖所有策略的最长窗口（120 根）。
func LoadDailyBars(ctx context.Context, code string, minBars int) ([]models.KLineBar, error) {
	end := time.Now().Format("2006-01-02")
	start := time.Now().AddDate(0, 0, -400).Format("2006-01-02")
	bars, err := datasource.NewKLineStore().QueryKLines(ctx, code, "day", start, end, true)
	if err != nil {
		return nil, err
	}
	if len(bars) < minBars {
		return nil, nil
	}
	return bars, nil
}
```

- [ ] **Step 6: 运行测试确认通过**

Run: `go test ./backend/data/khunter/strategy/ -v`
Expected: PASS。

- [ ] **Step 7: Commit**

```bash
git add backend/data/khunter/strategy/
git commit -m "feat(khunter): 策略框架、注册表与正序K线工具"
```

---

### Task 3: 仙人指路策略

**Files:**
- Create: `backend/data/khunter/strategy/ind.go`（共享指标工具：SMAAt、LinReg）
- Create: `backend/data/khunter/strategy/immortal_guidance.go`
- Test: `backend/data/khunter/strategy/immortal_guidance_test.go`

**Interfaces:**
- Consumes: Task 2 的 `Strategy`/`Signal`/`register`/工具函数
- Produces: `SMAAt(values []float64, idx, period int) float64`（含 idx、向前 period 根均值，不足返回 0）、`LinReg(values []float64) (slope, r2 float64)`；策略 `ImmortalGuidance{}`（Name="仙人指路", Weight=70, MinBars=30）

- [ ] **Step 1: 写失败测试**

```go
// backend/data/khunter/strategy/immortal_guidance_test.go
package strategy

import (
	"testing"
)

// 构造 40 根正序 K 线：缓升趋势 + 倒数第 2 根（信号日）冲高回落长上影 + 今日反包
func mkImmortalBars() []models.KLineBar {
	n := 40
	dates := genDates(n)
	o := mkConst(n, 0)
	h := mkConst(n, 0)
	l := mkConst(n, 0)
	c := mkConst(n, 0)
	v := mkVol(n, 1000)
	// 前 38 根缓升（保证 MA 多头 + R² 高）
	for i := 0; i < n-2; i++ {
		c[i] = 10 + 0.1*float64(i)
		o[i] = c[i] - 0.05
		h[i] = c[i] + 0.06
		l[i] = c[i] - 0.08
	}
	// 信号日 i = n-2：昨收 prev = c[n-3]，冲高 10%，收阳 +2%，上影 8%
	sig := n - 2
	prev := c[sig-1]
	o[sig] = prev
	c[sig] = prev * 1.02
	h[sig] = prev * 1.10 // surge = 10% ≥ 8%；us/h = 0.08/1.10 ≈ 7.3% ≥ 4%
	l[sig] = prev * 0.99
	v[sig] = 3000        // 量比 3 ≥ 1.5
	// 今日 i = n-1：反包（close ≥ (c[sig]+h[sig])/2 = prev*1.06），且 close ≥ MA5
	t := n - 1
	c[t] = prev * 1.07
	o[t] = prev * 1.05
	h[t] = prev * 1.08
	l[t] = prev * 1.04
	return mkBars(dates, o, h, l, c, v)
}

func TestImmortalGuidanceHit(t *testing.T) {
	bars := mkImmortalBars()
	sig := ImmortalGuidance{}.Select(bars, "测试股份", bars[len(bars)-1].TradeDate)
	if sig == nil {
		t.Fatal("应命中仙人指路")
	}
	if sig.StrategyName != "仙人指路" || sig.Close <= 0 || sig.VolumeRatio <= 0 {
		t.Fatalf("信号字段不完整: %+v", sig)
	}
}

func TestImmortalGuidanceNoSurge(t *testing.T) {
	bars := mkImmortalBars()
	// 把信号日冲高压到 5%（< 8%），不应命中
	n := len(bars)
	bars[n-2].High = bars[n-3].Close * 1.05
	if sig := ImmortalGuidance{}.Select(bars, "测试股份", bars[n-1].TradeDate); sig != nil {
		t.Fatalf("冲高不足不应命中: %+v", sig)
	}
}

func TestImmortalGuidanceSuspended(t *testing.T) {
	bars := mkImmortalBars()
	// 选股日比最后一根晚一天 → 停牌过滤
	if sig := ImmortalGuidance{}.Select(bars, "测试股份", "2999-01-01"); sig != nil {
		t.Fatal("停牌股不应命中")
	}
}

func TestImmortalGuidanceShortData(t *testing.T) {
	bars := mkImmortalBars()[:20] // < MinBars 30
	if sig := ImmortalGuidance{}.Select(bars, "测试股份", ""); sig != nil {
		t.Fatal("数据不足不应命中")
	}
}
```

（文件头需 `import "go-stock/backend/models"`。）

- [ ] **Step 2: 运行确认失败**

Run: `go test ./backend/data/khunter/strategy/ -run TestImmortalGuidance -v`
Expected: 编译失败（`ImmortalGuidance`/`SMAAt`/`LinReg` 未定义）。

- [ ] **Step 3: 实现 ind.go**

```go
// backend/data/khunter/strategy/ind.go
package strategy

// SMAAt 含 idx 在内的向前 period 根简单均值；不足返回 0
func SMAAt(values []float64, idx, period int) float64 {
	if idx-period+1 < 0 || idx >= len(values) {
		return 0
	}
	sum := 0.0
	for i := idx - period + 1; i <= idx; i++ {
		sum += values[i]
	}
	return sum / float64(period)
}

// LinReg 对 values 做 OLS 线性回归（x=0..n-1），返回斜率和 R²（下限截断 0）
func LinReg(values []float64) (slope, r2 float64) {
	n := float64(len(values))
	if n < 2 {
		return 0, 0
	}
	var sx, sy, sxy, sxx float64
	for i, v := range values {
		x := float64(i)
		sx += x
		sy += v
		sxy += x * v
		sxx += x * x
	}
	denom := n*sxx - sx*sx
	if denom == 0 {
		return 0, 0
	}
	slope = (n*sxy - sx*sy) / denom
	intercept := (sy - slope*sx) / n
	var ssRes, ssTot float64
	mean := sy / n
	for i, v := range values {
		fit := slope*float64(i) + intercept
		ssRes += (v - fit) * (v - fit)
		ssTot += (v - mean) * (v - mean)
	}
	if ssTot == 0 {
		return slope, 0
	}
	r2 = 1 - ssRes/ssTot
	if r2 < 0 {
		r2 = 0
	}
	return slope, r2
}
```

- [ ] **Step 4: 实现 immortal_guidance.go**

```go
// backend/data/khunter/strategy/immortal_guidance.go
package strategy

import "go-stock/backend/models"

// 仙人指路参数（对齐 KHunter 代码实际值）
const (
	igSurgeThreshold   = 0.08 // 冲高幅度
	igUpperShadowRatio = 0.04 // 上影线/high
	igVolumeRatioMin   = 1.5  // 信号日量比（对前5日均量，不含当日）
	igLookbackDays     = 3    // 信号日在 T-1~T-3
	igTrendLookback    = 20
	igTrendR2Min       = 0.5
	igAntiBodyRatio    = 0.50 // 反包目标 = 上影线 50%
)

type ImmortalGuidance struct{}

func init() { register(ImmortalGuidance{}) }

func (s ImmortalGuidance) Name() string { return "仙人指路" }
func (s ImmortalGuidance) Weight() int  { return 70 }
func (s ImmortalGuidance) MinBars() int { return 30 }

func (s ImmortalGuidance) Select(bars []models.KLineBar, stockName, selectionDate string) *Signal {
	n := len(bars)
	if n < s.MinBars() || IsInvalidName(stockName) {
		return nil
	}
	// 停牌/退市过滤：最后一根日期早于选股日则出局
	if selectionDate != "" && bars[n-1].TradeDate < selectionDate {
		return nil
	}
	closes, vols := Closes(bars), Volumes(bars)

	// 趋势过滤：最近 20 日收盘 slope>0 且 R²≥0.5
	slope, r2 := LinReg(closes[n-igTrendLookback:])
	if slope <= 0 || r2 < igTrendR2Min {
		return nil
	}

	today := n - 1
	for off := 1; off <= igLookbackDays; off++ {
		i := today - off
		if i < 21 { // 需前5日均量 + MA20
			continue
		}
		prev := closes[i-1]
		if prev == 0 || (bars[i].High-prev)/prev < igSurgeThreshold {
			continue
		}
		// 上影线（阳线 high-close，阴线 high-open）
		us := bars[i].High - bars[i].Close
		if bars[i].Close < bars[i].Open {
			us = bars[i].High - bars[i].Open
		}
		if bars[i].High == 0 || us/bars[i].High < igUpperShadowRatio {
			continue
		}
		avg5 := AvgVolumeBefore(vols, i, 5)
		if avg5 <= 0 || vols[i]/avg5 < igVolumeRatioMin {
			continue
		}
		ma5, ma10, ma20 := SMAAt(closes, i, 5), SMAAt(closes, i, 10), SMAAt(closes, i, 20)
		if !(ma5 > ma10 && ma10 > ma20 && ma20 > 0) {
			continue
		}
		// 反包目标价 = 上影线 50% 位置
		target := bars[i].Close + us*igAntiBodyRatio
		if bars[i].Close < bars[i].Open {
			target = bars[i].Open + us*igAntiBodyRatio
		}
		// 提前反包排除：信号日之后、今日之前 close 均须 < target
		preBroken := false
		for j := i + 1; j < today; j++ {
			if closes[j] >= target {
				preBroken = true
				break
			}
		}
		if preBroken {
			continue
		}
		// 今日反包确认
		if closes[today] < target || closes[today] < SMAAt(closes, today, 5) {
			continue
		}
		vr := 0.0
		if a := AvgVolumeBefore(vols, today, 5); a > 0 {
			vr = vols[today] / a
		}
		return &Signal{
			StrategyName: s.Name(), Code: bars[0].StockCode, Name: stockName,
			Date: bars[today].TradeDate, KeyDate: bars[i].TradeDate, KeyDateType: "信号日",
			Close: closes[today], VolumeRatio: vr,
			Reasons: []string{"冲高回落长上影", "放量", "均线多头", "今日反包确认"},
			Details: map[string]any{"signal_idx": i, "target": target, "r2": r2},
		}
	}
	return nil
}
```

- [ ] **Step 5: 运行确认通过**

Run: `go test ./backend/data/khunter/strategy/ -run TestImmortalGuidance -v`
Expected: PASS（4 个用例）。若 Hit 用例失败，先检查测试数据的 MA 多头/R² 是否满足，只允许调整测试数据数值，不允许放松策略阈值。

- [ ] **Step 6: Commit**

```bash
git add backend/data/khunter/strategy/ind.go backend/data/khunter/strategy/immortal_guidance*.go
git commit -m "feat(khunter): 仙人指路策略"
```

---

### Task 4: 涨停回马枪策略

**Files:**
- Create: `backend/data/khunter/strategy/limit_up_pullback.go`
- Test: `backend/data/khunter/strategy/limit_up_pullback_test.go`

**Interfaces:**
- Consumes: Task 2/3 工具
- Produces: `LimitUpPullback{}`（Name="涨停回马枪", Weight=50, MinBars=15）

- [ ] **Step 1: 写失败测试**

```go
// backend/data/khunter/strategy/limit_up_pullback_test.go
package strategy

import "testing"

// 25 根：前面平稳，idx=18 涨停(+10%, 3倍量)，idx=19~23 横盘缩量，今日(idx=24) 涨 3% 放量
func mkPullbackBars() []models.KLineBar {
	n := 25
	dates := genDates(n)
	o := mkConst(n, 10)
	h := mkConst(n, 10.1)
	l := mkConst(n, 9.9)
	c := mkConst(n, 10)
	v := mkVol(n, 1000)
	lu := 18
	o[lu], c[lu] = 10, 11.0 // +10%
	h[lu], l[lu] = 11.0, 10.0
	v[lu] = 3000 // 量比 3 ≥ 2.2
	// 回调期 19~23：收盘 ∈ [10.45, 11.55]，有收盘 < 11.0，有缩量日
	pullCloses := []float64{10.8, 10.6, 10.7, 10.55, 10.6}
	for k, pc := range pullCloses {
		i := lu + 1 + k
		o[i], c[i] = pc, pc
		h[i], l[i] = pc+0.05, pc-0.05
		v[i] = 1200
	}
	v[lu+2] = 1400 // ≤ 3000*0.5 缩量日
	// 今日：+3%（10.6→10.918），量 ≥ 昨日×1.5，close > MA5
	t := n - 1
	o[t] = 10.7
	c[t] = c[t-1] * 1.03
	h[t], l[t] = c[t]+0.05, o[t]-0.05
	v[t] = 2500
	return mkBars(dates, o, h, l, c, v)
}

func TestLimitUpPullbackHit(t *testing.T) {
	bars := mkPullbackBars()
	sig := LimitUpPullback{}.Select(bars, "测试股份", bars[len(bars)-1].TradeDate)
	if sig == nil {
		t.Fatal("应命中涨停回马枪")
	}
	if sig.KeyDateType != "涨停日" {
		t.Fatalf("KeyDateType 应为涨停日: %+v", sig)
	}
}

func TestLimitUpPullbackNoShrink(t *testing.T) {
	bars := mkPullbackBars()
	// 回调期全部不缩量（1600 > 3000*0.5=1500）→ 不命中
	for i := 19; i <= 23; i++ {
		bars[i].Volume = 1600
	}
	if sig := LimitUpPullback{}.Select(bars, "测试股份", bars[len(bars)-1].TradeDate); sig != nil {
		t.Fatal("无缩量不应命中")
	}
}

func TestLimitUpPullbackTodayWeak(t *testing.T) {
	bars := mkPullbackBars()
	// 今日涨幅压到 1% → 不命中
	n := len(bars)
	bars[n-1].Close = bars[n-2].Close * 1.01
	if sig := LimitUpPullback{}.Select(bars, "测试股份", bars[n-1].TradeDate); sig != nil {
		t.Fatal("今日涨幅不足不应命中")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./backend/data/khunter/strategy/ -run TestLimitUpPullback -v`
Expected: 编译失败。

- [ ] **Step 3: 实现 limit_up_pullback.go**

```go
// backend/data/khunter/strategy/limit_up_pullback.go
package strategy

import "go-stock/backend/models"

// 涨停回马枪参数（对齐 KHunter 代码实际值）
const (
	lpLimitUpThreshold   = 0.095 // 涨停判定涨幅
	lpLimitLookback      = 6     // 近 6 日内有涨停
	lpVolRatioLimitUp    = 2.2   // 涨停日量比（前5日均量）
	lpPullbackMaxDays    = 9
	lpPullbackRangeMax   = 0.15  // 回调振幅上限
	lpSupportRatio       = 0.95  // 回调收盘下限（对涨停收盘）
	lpResistanceRatio    = 1.05  // 回调收盘上限
	lpVolShrinkRatio     = 0.5   // 缩量判定
	lpTodayRiseMin       = 0.02  // 今日涨幅
	lpTodayVolRatio      = 1.5   // 今日量 ≥ 昨日×1.5
)

type LimitUpPullback struct{}

func init() { register(LimitUpPullback{}) }

func (s LimitUpPullback) Name() string { return "涨停回马枪" }
func (s LimitUpPullback) Weight() int  { return 50 }
func (s LimitUpPullback) MinBars() int { return 15 }

func (s LimitUpPullback) Select(bars []models.KLineBar, stockName, selectionDate string) *Signal {
	n := len(bars)
	if n < s.MinBars() || IsInvalidName(stockName) {
		return nil
	}
	if selectionDate != "" && bars[n-1].TradeDate < selectionDate {
		return nil
	}
	closes, vols := Closes(bars), Volumes(bars)
	today := n - 1

	// 1. 近 6 日找最近一个涨停日（涨幅≥9.5% 且量比≥2.2）
	lu := -1
	for i := today - 1; i >= 0 && i >= today-lpLimitLookback; i-- {
		if PctChange(bars, i) >= lpLimitUpThreshold {
			if a := AvgVolumeBefore(vols, i, 5); a > 0 && vols[i]/a >= lpVolRatioLimitUp {
				lu = i
				break
			}
		}
	}
	if lu < 0 || today-lu > lpPullbackMaxDays {
		return nil
	}
	luClose, luVol := closes[lu], vols[lu]

	// 2. 回调期（lu+1 ~ today-1）检查
	maxHigh, minLow := 0.0, 0.0
	hasLower, hasShrink := false, false
	for j := lu + 1; j < today; j++ {
		if bars[j].High > maxHigh {
			maxHigh = bars[j].High
		}
		if minLow == 0 || bars[j].Low < minLow {
			minLow = bars[j].Low
		}
		c := closes[j]
		if c < luClose*lpSupportRatio || c > luClose*lpResistanceRatio {
			return nil
		}
		if c < luClose {
			hasLower = true
		}
		if vols[j] <= luVol*lpVolShrinkRatio {
			hasShrink = true
		}
	}
	if today-lu >= 2 {
		if maxHigh > 0 && (maxHigh-minLow)/maxHigh > lpPullbackRangeMax {
			return nil
		}
		if !hasLower || !hasShrink {
			return nil
		}
	}

	// 3. 今日确认：涨幅>2%，量≥昨日×1.5，close>MA5
	if PctChange(bars, today) <= lpTodayRiseMin {
		return nil
	}
	if vols[today] < vols[today-1]*lpTodayVolRatio {
		return nil
	}
	if closes[today] <= SMAAt(closes, today, 5) {
		return nil
	}
	vr := 0.0
	if a := AvgVolumeBefore(vols, today, 5); a > 0 {
		vr = vols[today] / a
	}
	return &Signal{
		StrategyName: s.Name(), Code: bars[0].StockCode, Name: stockName,
		Date: bars[today].TradeDate, KeyDate: bars[lu].TradeDate, KeyDateType: "涨停日",
		Close: closes[today], VolumeRatio: vr,
		Reasons: []string{"涨停后回调企稳", "缩量洗盘", "今日放量启动"},
		Details: map[string]any{"limit_up_close": luClose, "pullback_days": today - lu},
	}
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./backend/data/khunter/strategy/ -run TestLimitUpPullback -v`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add backend/data/khunter/strategy/limit_up_pullback*.go
git commit -m "feat(khunter): 涨停回马枪策略"
```

---

### Task 5: W底策略

**Files:**
- Create: `backend/data/khunter/strategy/w_bottom.go`
- Test: `backend/data/khunter/strategy/w_bottom_test.go`

**Interfaces:**
- Consumes: Task 2/3 工具（`IsLocalLow`、`SMAAt`、`AvgVolumeBefore`、`PctChange`）
- Produces: `WBottom{}`（Name="W底", Weight=50, MinBars=60）

- [ ] **Step 1: 写失败测试**

```go
// backend/data/khunter/strategy/w_bottom_test.go
package strategy

import "testing"

// 70 根：0~20 从 20 跌到 10（深跌 50%）→ idx=32 L1=10 → idx=45 颈线 H=12 →
// idx=58 L2=10.1（间隔 26>10，价差 1%≤3%）→ 回升 → idx=66（倒数第4根，含在最后5日内）
// +8.5% 放量日收盘 12.2（≥12*1.01=12.12，前一日 11.2<12）→ 其后收盘 ≥ 12*0.98=11.76
func mkWBottomBars() []models.KLineBar {
	n := 70
	dates := genDates(n)
	o := mkConst(n, 0)
	h := mkConst(n, 0)
	l := mkConst(n, 0)
	c := mkConst(n, 0)
	v := mkVol(n, 1000)
	// idx 0~30：从 20 跌到 10.2（保证 L1 前 30 日内有 >12 的高点）
	for i := 0; i <= 30; i++ {
		c[i] = 20 - (20-10.2)*float64(i)/30
		o[i] = c[i] + 0.1
		h[i] = c[i] + 0.3
		l[i] = c[i] - 0.3
	}
	// idx 31~44：L1=10 在 idx=32，然后反弹到颈线
	for i := 31; i <= 44; i++ {
		switch {
		case i == 32:
			c[i], l[i], o[i], h[i] = 10.4, 10.0, 10.5, 10.6 // L1 低点 low=10
		case i <= 38:
			c[i] = 10.4 + (11.8-10.4)*float64(i-32)/6
			o[i], h[i], l[i] = c[i]-0.1, c[i]+0.15, c[i]-0.2
		default:
			c[i] = 11.8 - (11.8-10.9)*float64(i-38)/6
			o[i], h[i], l[i] = c[i]+0.1, c[i]+0.15, c[i]-0.2
		}
	}
	// idx 45 附近颈线最高 high=12（idx=40 已经是高点区），在 idx=41 放颈线 spike
	h[41] = 12.0
	c[41] = 11.85
	// idx 45~57：回落到 L2
	for i := 45; i <= 57; i++ {
		c[i] = 11.0 - (11.0-10.3)*float64(i-45)/12
		o[i], h[i], l[i] = c[i]+0.1, c[i]+0.15, c[i]-0.2
	}
	// idx=58：L2=10.1
	c[58], l[58], o[58], h[58] = 10.5, 10.1, 10.6, 10.7
	// idx 59~65：缓升到 11.2
	for i := 59; i <= 65; i++ {
		c[i] = 10.5 + (11.2-10.5)*float64(i-58)/7
		o[i], h[i], l[i] = c[i]-0.05, c[i]+0.1, c[i]-0.15
	}
	// idx=66：放量确认日，前收 11.2，+8.93% → 12.2，量比 1.5
	c[66], o[66], h[66], l[66] = 12.2, 11.3, 12.3, 11.25
	v[66] = 1500
	// idx 67~69：站在颈线上方（≥11.76），保持上行（MA10>MA30）
	for i := 67; i <= 69; i++ {
		c[i] = 12.2 + 0.1*float64(i-66)
		o[i], h[i], l[i] = c[i]-0.1, c[i]+0.1, c[i]-0.2
	}
	return mkBars(dates, o, h, l, c, v)
}

func TestWBottomHit(t *testing.T) {
	bars := mkWBottomBars()
	sig := WBottom{}.Select(bars, "测试股份", bars[len(bars)-1].TradeDate)
	if sig == nil {
		t.Fatal("应命中W底")
	}
	if sig.KeyDateType != "放量确认日" {
		t.Fatalf("KeyDateType: %+v", sig)
	}
}

func TestWBottomFakeFilter(t *testing.T) {
	bars := mkWBottomBars()
	// 破坏"L1 之前 30 日内有 >L1×1.2 的高点"：把前 30 根全部压平到 10~11
	for i := 0; i <= 30; i++ {
		bars[i].Open, bars[i].Close = 10.5, 10.5
		bars[i].High, bars[i].Low = 10.9, 10.1
	}
	if sig := WBottom{}.Select(bars, "测试股份", bars[len(bars)-1].TradeDate); sig != nil {
		t.Fatal("无前置深跌不应命中（假W底过滤）")
	}
}

func TestWBottomNecklineSpace(t *testing.T) {
	bars := mkWBottomBars()
	// 颈线空间不足：h[41] 压到 10.9（< 10×1.1=11）→ 不命中
	bars[41].High = 10.9
	bars[41].Close = 10.8
	if sig := WBottom{}.Select(bars, "测试股份", bars[len(bars)-1].TradeDate); sig != nil {
		t.Fatal("颈线空间不足不应命中")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./backend/data/khunter/strategy/ -run TestWBottom -v`
Expected: 编译失败。注：若实现后 Hit 用例因测试数据细节（如局部低点窗口扫到意外低点）失败，只允许调整测试数据的中间过渡段数值，阈值不得放松。

- [ ] **Step 3: 实现 w_bottom.go**

```go
// backend/data/khunter/strategy/w_bottom.go
package strategy

import (
	"math"

	"go-stock/backend/models"
)

// W底参数（对齐 KHunter 代码实际值）
const (
	wbPatternDays       = 40   // 形态扫描窗口
	wbExcludeLatest     = 5    // 排除最新 5 根（留给突破检测）
	wbLowWindow         = 5    // 局部低点窗口
	wbMinGap            = 10   // 两底间隔（严格大于）
	wbBottomDiff        = 0.03 // 两底价差上限
	wbNecklineSpace     = 1.1  // H ≥ L1 × 1.1
	wbBreakoutMargin    = 1.01 // 突破确认 close ≥ H × 1.01
	wbVolExpandRatio    = 1.2  // 放量确认日量比（代码实际值，非注释的 1.5）
	wbConfirmRise       = 0.08 // 放量确认日涨幅 > 8%
	wbQuickCheckRise    = 0.05 // 预检：近5日有涨幅>5%
	wbPriorDropBars     = 30
	wbPriorDropRatio    = 1.2  // L1 前 30 日最高价 > L1 × 1.2
	wbSupportRatio      = 0.02 // 突破后 close ≥ H × 0.98
)

type WBottom struct{}

func init() { register(WBottom{}) }

func (s WBottom) Name() string { return "W底" }
func (s WBottom) Weight() int  { return 50 }
func (s WBottom) MinBars() int { return 60 }

func (s WBottom) Select(bars []models.KLineBar, stockName, selectionDate string) *Signal {
	n := len(bars)
	if n < s.MinBars() || IsInvalidName(stockName) {
		return nil
	}
	if selectionDate != "" && bars[n-1].TradeDate < selectionDate {
		return nil
	}
	closes, vols := Closes(bars), Volumes(bars)
	lows, highs := Lows(bars), Highs(bars)
	today := n - 1

	// 1. 预检：近5日存在涨幅>5%
	quick := false
	for i := today - 4; i <= today; i++ {
		if i > 0 && PctChange(bars, i) > wbQuickCheckRise {
			quick = true
			break
		}
	}
	if !quick {
		return nil
	}

	// 2. 放量确认日：近5日内涨幅>8% 且量≥前5日均量×1.2（取最近一个）
	confirm := -1
	for i := today; i >= today-4 && i > 0; i-- {
		if PctChange(bars, i) > wbConfirmRise {
			if a := AvgVolumeBefore(vols, i, 5); a > 0 && vols[i] >= a*wbVolExpandRatio {
				confirm = i
				break
			}
		}
	}
	if confirm < 0 {
		return nil
	}

	// 3. 形态窗口（排除最新5根）找局部低点，间隔不足时保留更低者
	end := today - wbExcludeLatest
	start := end - wbPatternDays
	if start < wbLowWindow {
		start = wbLowWindow
	}
	var lowIdx []int
	for i := start; i <= end; i++ {
		if IsLocalLow(lows, i, wbLowWindow) {
			if m := len(lowIdx); m > 0 && i-lowIdx[m-1] <= wbMinGap {
				// 间隔不足：保留价格更低者
				if lows[i] < lows[lowIdx[m-1]] {
					lowIdx[m-1] = i
				}
				continue
			}
			lowIdx = append(lowIdx, i)
		}
	}
	if len(lowIdx) < 2 {
		return nil
	}
	iL2 := lowIdx[len(lowIdx)-1] // 最新低点
	iL1 := lowIdx[len(lowIdx)-2] // 次新低点
	L1, L2 := lows[iL1], lows[iL2]
	if iL2-iL1 <= wbMinGap || L1 <= 0 {
		return nil
	}
	if math.Abs(L2-L1)/L1 > wbBottomDiff {
		return nil
	}
	// 颈线 H = 两底之间最高价
	H := 0.0
	for i := iL1; i <= iL2; i++ {
		if highs[i] > H {
			H = highs[i]
		}
	}
	if H <= L1 || H <= L2 || H < L1*wbNecklineSpace {
		return nil
	}

	// 4. 颈线突破：确认日 close ≥ H×1.01 且前一日 close < H
	if closes[confirm] < H*wbBreakoutMargin || closes[confirm-1] >= H {
		return nil
	}

	// 5. 趋势：MA10 > MA30（今日）
	if SMAAt(closes, today, 10) <= SMAAt(closes, today, 30) {
		return nil
	}

	// 6. 假W底过滤
	priorStart := iL1 - wbPriorDropBars
	if priorStart < 0 {
		priorStart = 0
	}
	priorHigh := 0.0
	for i := priorStart; i < iL1; i++ {
		if highs[i] > priorHigh {
			priorHigh = highs[i]
		}
	}
	if priorHigh <= L1*wbPriorDropRatio {
		return nil
	}
	for i := confirm; i <= today; i++ {
		if closes[i] < H*(1-wbSupportRatio) {
			return nil
		}
	}

	vr := 0.0
	if a := AvgVolumeBefore(vols, today, 5); a > 0 {
		vr = vols[today] / a
	}
	return &Signal{
		StrategyName: s.Name(), Code: bars[0].StockCode, Name: stockName,
		Date: bars[today].TradeDate, KeyDate: bars[confirm].TradeDate, KeyDateType: "放量确认日",
		Close: closes[today], VolumeRatio: vr,
		Reasons: []string{"双底形态", "颈线突破", "MA10上穿MA30"},
		Details: map[string]any{"L1": L1, "L2": L2, "neckline": H,
			"l1_date": bars[iL1].TradeDate, "l2_date": bars[iL2].TradeDate},
	}
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./backend/data/khunter/strategy/ -run TestWBottom -v`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add backend/data/khunter/strategy/w_bottom*.go
git commit -m "feat(khunter): W底策略"
```

---

### Task 6: 底部趋势拐点策略

**Files:**
- Create: `backend/data/khunter/strategy/bottom_inflection.go`
- Test: `backend/data/khunter/strategy/bottom_inflection_test.go`

**Interfaces:**
- Consumes: Task 2/3 工具 + `indicator.MACD(close []float64, fast, slow, signal int) (dif, dea, hist []float64)`（`go-stock/backend/data/indicator`，递推式 EMA，对齐 pandas adjust=False）
- Produces: `BottomInflection{}`（Name="底部趋势拐点", Weight=50, MinBars=120）

- [ ] **Step 1: 写失败测试**

```go
// backend/data/khunter/strategy/bottom_inflection_test.go
package strategy

import "testing"

// 130 根：从 30 跌到 ~16（>45% 深跌，先高后低），锚点（倒数第4根）放量长阳 +8.5%，
// 锚点前 20 日分两半：后半段 low 更低但下跌放缓（MACD 柱抬高 → 底背离）
func mkBottomInflectionBars() []models.KLineBar {
	n := 130
	dates := genDates(n)
	o := mkConst(n, 0)
	h := mkConst(n, 0)
	l := mkConst(n, 0)
	c := mkConst(n, 0)
	v := mkVol(n, 1000)
	// idx 0~85：从 30 匀速跌到 20（窗口最高在头部）
	for i := 0; i <= 85; i++ {
		c[i] = 30 - (30-20)*float64(i)/85
		o[i] = c[i] + 0.15
		h[i] = c[i] + 0.35
		l[i] = c[i] - 0.35
	}
	// idx 86~105（锚点前 20 日的前半段，106~115 正式计入）：先陡跌
	// 简化：86~115 线性从 20 → 16.3（后半段放缓），116~125 在 16.05~16.8 间走平略降
	for i := 86; i <= 115; i++ {
		c[i] = 20 - (20-16.4)*float64(i-86)/29
		o[i] = c[i] + 0.12
		h[i] = c[i] + 0.3
		l[i] = c[i] - 0.3
	}
	for i := 116; i <= 125; i++ {
		c[i] = 16.4 - 0.03*float64(i-116) // 极缓下跌 → MACD 柱抬高
		o[i] = c[i] + 0.08
		h[i] = c[i] + 0.2
		l[i] = c[i] - 0.25
	}
	l[124] = 16.05 // 后半段最低价（< 前半段最低 ~16.1），价格创新低
	l[114] = 16.10 // 前半段最低价
	// 锚点 idx=126（倒数第4根，off=3）：+8.5% 放量长阳
	anchor := 126
	prev := c[anchor-1]
	o[anchor] = prev * 1.01
	c[anchor] = prev * 1.085
	h[anchor] = c[anchor] * 1.01
	l[anchor] = prev * 0.99
	v[anchor] = 3000 // 量比(前10日均量) 3 ≥ 2.5
	// 锚点 close ≈ 17.3，距 120 日最低 16.05：(17.3-16.05)/16.05 ≈ 7.8% ≤ 15% ✓
	// idx 127~129：回调不破锚点开盘价
	for i := 127; i <= 129; i++ {
		c[i] = c[anchor] - 0.15*float64(i-anchor)
		o[i] = c[i] + 0.1
		h[i] = c[i] + 0.15
		l[i] = c[i] - 0.2
		if l[i] < o[anchor] { // 收盘不破开盘价即可，low 允许下探
			l[i] = o[anchor] - 0.3
		}
	}
	return mkBars(dates, o, h, l, c, v)
}

func TestBottomInflectionHit(t *testing.T) {
	bars := mkBottomInflectionBars()
	sig := BottomInflection{}.Select(bars, "测试股份", bars[len(bars)-1].TradeDate)
	if sig == nil {
		t.Fatal("应命中底部趋势拐点")
	}
	if sig.KeyDateType != "放量长阳日" {
		t.Fatalf("KeyDateType: %+v", sig)
	}
}

func TestBottomInflectionNoDeepDecline(t *testing.T) {
	bars := mkBottomInflectionBars()
	// 把前 86 根压成 15~17 的浅跌（< 45%）→ 不命中
	for i := 0; i <= 85; i++ {
		bars[i].Close = 17 - 1.0*float64(i)/85
		bars[i].Open = bars[i].Close + 0.1
		bars[i].High = bars[i].Close + 0.3
		bars[i].Low = bars[i].Close - 0.3
	}
	if sig := BottomInflection{}.Select(bars, "测试股份", bars[len(bars)-1].TradeDate); sig != nil {
		t.Fatal("深跌不足不应命中")
	}
}

func TestBottomInflectionAnchorBroken(t *testing.T) {
	bars := mkBottomInflectionBars()
	// 锚点之后某日收盘跌破锚点开盘价 → 不命中
	bars[128].Close = bars[126].Open * 0.97
	if sig := BottomInflection{}.Select(bars, "测试股份", bars[len(bars)-1].TradeDate); sig != nil {
		t.Fatal("回调破锚点开盘价不应命中")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./backend/data/khunter/strategy/ -run TestBottomInflection -v`
Expected: 编译失败。实现后若 Hit 用例的 MACD 背离不满足，先打印两个半段的 hist 值核对，只允许调整测试数据过渡段斜率。

- [ ] **Step 3: 实现 bottom_inflection.go**

```go
// backend/data/khunter/strategy/bottom_inflection.go
package strategy

import (
	"go-stock/backend/data/indicator"
	"go-stock/backend/models"
)

// 底部趋势拐点参数（对齐 KHunter 代码实际值）
const (
	biLookbackWindow    = 120  // 深跌/最低价窗口
	biAnchorOffMin      = 3    // 锚点在倒数第 3~5 根（跳过最近 2 根）
	biAnchorOffMax      = 5
	biAnchorRise        = 0.08 // 锚点涨幅 > 8%
	biAnchorVolRatio    = 2.5  // 锚点量比（对前10日均量）
	biNearLowMax        = 0.15 // 锚点 close 距窗口最低价 ≤ 15%
	biDeclineMin        = 0.45 // 深跌 > 45%（先高后低）
	biDivergenceDays    = 20   // MACD 背离观察窗口（锚点之前）
)

type BottomInflection struct{}

func init() { register(BottomInflection{}) }

func (s BottomInflection) Name() string { return "底部趋势拐点" }
func (s BottomInflection) Weight() int  { return 50 }
func (s BottomInflection) MinBars() int { return 120 }

func (s BottomInflection) Select(bars []models.KLineBar, stockName, selectionDate string) *Signal {
	n := len(bars)
	if n < s.MinBars() || IsInvalidName(stockName) {
		return nil
	}
	if selectionDate != "" && bars[n-1].TradeDate < selectionDate {
		return nil
	}
	closes, vols := Closes(bars), Volumes(bars)
	lows, highs := Lows(bars), Highs(bars)
	today := n - 1

	for off := biAnchorOffMin; off <= biAnchorOffMax; off++ {
		anchor := today - off
		if anchor < biLookbackWindow {
			continue
		}
		// 锚点：涨幅>8% 且量比（前10日）≥2.5
		if PctChange(bars, anchor) <= biAnchorRise {
			continue
		}
		if a := AvgVolumeBefore(vols, anchor, 10); a <= 0 || vols[anchor]/a < biAnchorVolRatio {
			continue
		}
		// 窗口最低价（含锚点当日往前 120 根）
		winStart := anchor - biLookbackWindow + 1
		lowest := lows[winStart]
		for i := winStart; i <= anchor; i++ {
			if lows[i] < lowest {
				lowest = lows[i]
			}
		}
		if lowest <= 0 || (closes[anchor]-lowest)/lowest > biNearLowMax {
			continue
		}
		// 锚点之后所有 close ≥ 锚点开盘价
		broken := false
		for i := anchor + 1; i <= today; i++ {
			if closes[i] < bars[anchor].Open {
				broken = true
				break
			}
			_ = i
		}
		if broken {
			continue
		}
		// 深跌：窗口内先出现最高价、后出现最低价，跌幅 >45%
		hiIdx, hiVal := winStart, highs[winStart]
		for i := winStart; i <= anchor; i++ {
			if highs[i] > hiVal {
				hiVal, hiIdx = highs[i], i
			}
		}
		loVal := hiVal
		found := false
		for i := hiIdx; i <= anchor; i++ {
			if lows[i] < loVal {
				loVal = lows[i]
				found = true
			}
		}
		if !found || hiVal <= 0 || (hiVal-loVal)/hiVal <= biDeclineMin {
			continue
		}
		// MACD 底背离：锚点前 20 日分两半，各取最低价所在日的 low 与 hist
		dEnd := anchor - 1
		dStart := dEnd - biDivergenceDays + 1
		if dStart < 1 {
			continue
		}
		_, _, hist := indicator.MACD(closes[:dEnd+1], 12, 26, 9)
		mid := dStart + biDivergenceDays/2
		lowA, histA := lows[dStart], hist[dStart]
		for i := dStart; i < mid; i++ {
			if lows[i] < lowA {
				lowA, histA = lows[i], hist[i]
			}
		}
		lowB, histB := lows[mid], hist[mid]
		for i := mid; i <= dEnd; i++ {
			if lows[i] < lowB {
				lowB, histB = lows[i], hist[i]
			}
		}
		if !(lowB < lowA && histB > histA) {
			continue
		}
		vr := 0.0
		if a := AvgVolumeBefore(vols, today, 5); a > 0 {
			vr = vols[today] / a
		}
		return &Signal{
			StrategyName: s.Name(), Code: bars[0].StockCode, Name: stockName,
			Date: bars[today].TradeDate, KeyDate: bars[anchor].TradeDate, KeyDateType: "放量长阳日",
			Close: closes[today], VolumeRatio: vr,
			Reasons: []string{"深度下跌后放量长阳", "回调不破起涨点", "MACD底背离"},
			Details: map[string]any{"anchor": bars[anchor].TradeDate, "decline": (hiVal - loVal) / hiVal,
				"low_a": lowA, "low_b": lowB, "hist_a": histA, "hist_b": histB},
		}
	}
	return nil
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./backend/data/khunter/strategy/ -run TestBottomInflection -v`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add backend/data/khunter/strategy/bottom_inflection*.go
git commit -m "feat(khunter): 底部趋势拐点策略（含MACD底背离）"
```

---

### Task 7: 主升低吸策略

**Files:**
- Create: `backend/data/khunter/strategy/uptrend_dip_buy.go`
- Test: `backend/data/khunter/strategy/uptrend_dip_buy_test.go`

**Interfaces:**
- Consumes: Task 2/3 工具（`SwingHighs`、`SMAAt`、`AvgVolumeBefore`）
- Produces: `UptrendDipBuy{}`（Name="主升低吸", Weight=50, MinBars=106）

- [ ] **Step 1: 写失败测试**

```go
// backend/data/khunter/strategy/uptrend_dip_buy_test.go
package strategy

import "testing"

// 110 根：0~44 平盘 10 元 → 45~59 拉升（L=10 在 idx 45 前 45 日内）→ idx=60 H=14（+40%）
// → 61~79 回撤到 P=11.2（-20%，≥18% 且 < 40%-8%）→ idx=88 R=12.5（swing high < H）
// → 其后高点单调不增 → 今日 idx=109 阳线实体 5.8% 贴紧突破 R（+1.6%≤2%）放量 2 倍
func mkUptrendDipBars() []models.KLineBar {
	n := 110
	dates := genDates(n)
	o := mkConst(n, 10)
	h := mkConst(n, 10.15)
	l := mkConst(n, 9.9)
	c := mkConst(n, 10)
	v := mkVol(n, 1000)
	// idx 45~60：拉升到 14
	for i := 45; i <= 60; i++ {
		c[i] = 10 + (14-10)*float64(i-45)/15
		o[i] = c[i] - 0.1
		h[i] = c[i] + 0.12
		l[i] = c[i] - 0.15
	}
	l[45] = 10.0 // L=10（45 日窗口最低）
	// idx 61~79：回撤到 11.2
	for i := 61; i <= 79; i++ {
		c[i] = 13.9 - (13.9-11.2)*float64(i-61)/18
		o[i] = c[i] + 0.1
		h[i] = c[i] + 0.12
		l[i] = c[i] - 0.12
	}
	l[79] = 11.2 // P=11.2
	// idx 80~95：反弹到 R=12.5（idx 88），随后高点递减
	for i := 80; i <= 88; i++ {
		c[i] = 11.3 + (12.4-11.3)*float64(i-80)/8
		o[i] = c[i] - 0.08
		h[i] = c[i] + 0.1
		l[i] = c[i] - 0.12
	}
	h[88] = 12.5 // R
	c[88] = 12.4
	for i := 89; i <= 108; i++ {
		c[i] = 12.3 - 0.015*float64(i-89)
		o[i] = c[i] + 0.05
		h[i] = c[i] + 0.08 // 高点递减，不产生新 swing high
		l[i] = c[i] - 0.1
	}
	// 今日 idx=109：开 12.0 收 12.7（实体 5.8%），突破 R=12.5（+1.6%），量 2 倍
	t := n - 1
	o[t], c[t] = 12.0, 12.7
	h[t], l[t] = 12.75, 11.95
	v[t] = 2000
	return mkBars(dates, o, h, l, c, v)
}

func TestUptrendDipBuyHit(t *testing.T) {
	bars := mkUptrendDipBars()
	sig := UptrendDipBuy{}.Select(bars, "测试股份", bars[len(bars)-1].TradeDate)
	if sig == nil {
		t.Fatal("应命中主升低吸")
	}
	if sig.KeyDateType != "一拉高点" {
		t.Fatalf("KeyDateType: %+v", sig)
	}
}

func TestUptrendDipBuyChaseHigh(t *testing.T) {
	bars := mkUptrendDipBars()
	// 今日收盘拉到 13.0（突破 R 4% > 2%，追高）→ 不命中
	n := len(bars)
	bars[n-1].Close = 13.0
	bars[n-1].High = 13.05
	if sig := UptrendDipBuy{}.Select(bars, "测试股份", bars[n-1].TradeDate); sig != nil {
		t.Fatal("贴紧突破幅度超限不应命中")
	}
}

func TestUptrendDipBuyPullbackTooDeep(t *testing.T) {
	bars := mkUptrendDipBars()
	// 二调跌破起涨点 L（l[45]=10.0 区域的前低 9.9）→ 一拉被全额回吐，不命中
	bars[70].Low = 9.85
	if sig := UptrendDipBuy{}.Select(bars, "测试股份", bars[len(bars)-1].TradeDate); sig != nil {
		t.Fatal("回撤跌破起涨点不应命中")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./backend/data/khunter/strategy/ -run TestUptrendDipBuy -v`
Expected: 编译失败。实现后若 Hit 失败，先打印 swing high 下标序列核对，只允许调整测试数据过渡段。

- [ ] **Step 3: 实现 uptrend_dip_buy.go**

```go
// backend/data/khunter/strategy/uptrend_dip_buy.go
package strategy

import "go-stock/backend/models"

// 主升低吸参数（对齐 KHunter 代码实际值）
const (
	udRecentDays       = 60   // 一拉 swing high 扫描窗口
	udRallyLowLookback = 45   // L 的回望窗口
	udRallyMinPct      = 0.30 // 一拉涨幅下限
	udRallyMaxPct      = 1.00 // 一拉涨幅上限
	udPullbackMinPct   = 0.18 // 二调回撤下限
	udPullbackGapPct   = 0.08 // 回撤 < 一拉涨幅 - 8%
	udSwingWindow      = 2
	udMinBodyPct       = 0.05 // 今日阳线实体 ≥5%
	udBreakoutMaxPct   = 0.02 // 贴紧突破 ≤2%
	udVolumeRatio      = 1.8
)

type UptrendDipBuy struct{}

func init() { register(UptrendDipBuy{}) }

func (s UptrendDipBuy) Name() string { return "主升低吸" }
func (s UptrendDipBuy) Weight() int  { return 50 }
func (s UptrendDipBuy) MinBars() int { return 106 }

func (s UptrendDipBuy) Select(bars []models.KLineBar, stockName, selectionDate string) *Signal {
	n := len(bars)
	if n < s.MinBars() || IsInvalidName(stockName) {
		return nil
	}
	if selectionDate != "" && bars[n-1].TradeDate < selectionDate {
		return nil
	}
	closes, vols := Closes(bars), Volumes(bars)
	highs, lows := Highs(bars), Lows(bars)
	today := n - 1

	// 快速过滤：今日阳线实体 ≥5%
	if bars[today].Open <= 0 ||
		(bars[today].Close-bars[today].Open)/bars[today].Open < udMinBodyPct {
		return nil
	}

	// C1 一拉：最近 60 日窗口内 swing high，按高度从高到低找首个满足涨幅的 H
	winStart := n - udRecentDays
	var candidates []int
	for _, i := range SwingHighs(highs, udSwingWindow) {
		if i >= winStart && i < today {
			candidates = append(candidates, i)
		}
	}
	// 按高度降序
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if highs[candidates[j]] > highs[candidates[i]] {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}
	for _, iH := range candidates {
		H := highs[iH]
		// L = H 之前 45 日最低价
		lStart := iH - udRallyLowLookback
		if lStart < 0 {
			lStart = 0
		}
		L := lows[lStart]
		for i := lStart; i < iH; i++ {
			if lows[i] < L {
				L = lows[i]
			}
		}
		if L <= 0 {
			continue
		}
		rally := (H - L) / L
		if rally < udRallyMinPct || (udRallyMaxPct > 0 && rally > udRallyMaxPct) {
			continue
		}
		// C2 二调：H 之后最低点 P
		P := lows[iH+1]
		for i := iH + 1; i <= today; i++ {
			if lows[i] < P {
				P = lows[i]
			}
		}
		pullback := (H - P) / H
		if pullback < udPullbackMinPct || pullback >= rally-udPullbackGapPct || P <= L {
			continue
		}
		// C3 R：H 之后、今日之前最后一个 swing high，且 R < H
		R := 0.0
		for _, i := range SwingHighs(highs, udSwingWindow) {
			if i > iH && i < today && highs[i] < H {
				R = highs[i] // SwingHighs 正序返回，取最后一个
			}
		}
		if R <= 0 {
			continue
		}
		// C4 今日贴紧突破
		c := closes[today]
		if c <= R || (c-R)/R > udBreakoutMaxPct {
			continue
		}
		if a := AvgVolumeBefore(vols, today, 5); a <= 0 || vols[today]/a < udVolumeRatio {
			continue
		}
		if c <= SMAAt(closes, today, 5) {
			continue
		}
		vr := vols[today] / AvgVolumeBefore(vols, today, 5)
		return &Signal{
			StrategyName: s.Name(), Code: bars[0].StockCode, Name: stockName,
			Date: bars[today].TradeDate, KeyDate: bars[iH].TradeDate, KeyDateType: "一拉高点",
			Close: c, VolumeRatio: vr,
			Reasons: []string{"一拉二调结构", "贴紧突破R", "放量确认"},
			Details: map[string]any{"H": H, "L": L, "P": P, "R": R, "rally": rally, "pullback": pullback},
		}
	}
	return nil
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./backend/data/khunter/strategy/ -run TestUptrendDipBuy -v`
Expected: PASS。

- [ ] **Step 5: 回归 + Commit**

Run: `go test ./backend/data/khunter/... -v`
Expected: 策略包全部 PASS（Task 2-7 累计约 20 个用例）。

```bash
git add backend/data/khunter/strategy/uptrend_dip_buy*.go
git commit -m "feat(khunter): 主升低吸策略"
```

---

### Task 8: 技术面评分器

**Files:**
- Create: `backend/data/khunter/scorer/types.go`（DimScore 等共享类型）
- Create: `backend/data/khunter/scorer/technical.go`
- Create: `config/khunter_technical_weights.json`
- Test: `backend/data/khunter/scorer/technical_test.go`

**Interfaces:**
- Consumes: 无外部依赖
- Produces（Task 9-13 依赖）:
  - `scorer.DimScore{Score float64; Veto bool; Reason string; Degraded bool; Detail map[string]any}`
  - `scorer.Hit{Name string; Weight int}`
  - `scorer.TechnicalScorer{Overrides map[string]int}`，方法 `Score(hits []Hit) DimScore`
  - `scorer.LoadTechnicalOverrides(path string) map[string]int`（文件不存在返回 nil）

- [ ] **Step 1: 写失败测试**

```go
// backend/data/khunter/scorer/technical_test.go
package scorer

import "testing"

func TestTechnicalScoreSum(t *testing.T) {
	s := TechnicalScorer{}
	d := s.Score([]Hit{{Name: "仙人指路", Weight: 70}, {Name: "W底", Weight: 50}})
	if d.Score != 120 || d.Veto {
		t.Fatalf("expect 120 无否决, got %+v", d)
	}
	// 空命中
	d = s.Score(nil)
	if d.Score != 0 {
		t.Fatalf("expect 0, got %+v", d)
	}
}

func TestTechnicalOverride(t *testing.T) {
	s := TechnicalScorer{Overrides: map[string]int{"W底": 30}}
	d := s.Score([]Hit{{Name: "W底", Weight: 50}})
	if d.Score != 30 {
		t.Fatalf("override 应生效为 30, got %+v", d)
	}
}

func TestTechnicalVeto(t *testing.T) {
	s := TechnicalScorer{}
	d := s.Score([]Hit{{Name: "M头策略", Weight: -80}, {Name: "多死叉共振策略", Weight: -50}})
	if !d.Veto || d.Score != -100 {
		t.Fatalf("M头+多死叉应否决为 -100, got %+v", d)
	}
	// 单独命中 M头 不否决
	d = s.Score([]Hit{{Name: "M头策略", Weight: -80}})
	if d.Veto || d.Score != -80 {
		t.Fatalf("单独M头应为 -80 不否决, got %+v", d)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./backend/data/khunter/scorer/ -v`
Expected: 编译失败。

- [ ] **Step 3: 实现 types.go 与 technical.go**

```go
// backend/data/khunter/scorer/types.go
package scorer

// DimScore 单维度评分结果
type DimScore struct {
	Score    float64
	Veto     bool   // 一票否决
	Reason   string // 否决原因或说明
	Degraded bool   // 数据缺失按中性分处理
	Detail   map[string]any
}

// Hit 一次策略命中
type Hit struct {
	Name   string
	Weight int
}
```

```go
// backend/data/khunter/scorer/technical.go
package scorer

import (
	"encoding/json"
	"os"
)

// 技术面一票否决组合（对齐 KHunter veto_config）
var technicalVetoPair = [2]string{"M头策略", "多死叉共振策略"}

// TechnicalScorer 技术面得分 = Σ 命中策略权重（Overrides 可按名称覆盖权重）
type TechnicalScorer struct {
	Overrides map[string]int
}

func (s TechnicalScorer) Score(hits []Hit) DimScore {
	total := 0.0
	names := map[string]bool{}
	for _, h := range hits {
		w := h.Weight
		if ow, ok := s.Overrides[h.Name]; ok {
			w = ow
		}
		total += float64(w)
		names[h.Name] = true
	}
	d := DimScore{Score: total, Detail: map[string]any{"hits": len(hits)}}
	if names[technicalVetoPair[0]] && names[technicalVetoPair[1]] {
		d.Veto = true
		d.Score = -100
		d.Reason = "M头策略与多死叉共振策略同时命中"
	}
	return d
}

// LoadTechnicalOverrides 加载权重覆盖配置；文件不存在或解析失败返回 nil
func LoadTechnicalOverrides(path string) map[string]int {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cfg struct {
		Overrides map[string]int `json:"overrides"`
	}
	if json.Unmarshal(data, &cfg) != nil {
		return nil
	}
	return cfg.Overrides
}
```

```json
// config/khunter_technical_weights.json
{
  "description": "技术面评分权重覆盖（可选）。缺省时使用各策略内置 Weight()。负向策略预留：M头策略 -80、多死叉共振策略 -50。",
  "overrides": {}
}
```

- [ ] **Step 4: 运行确认通过 + Commit**

Run: `go test ./backend/data/khunter/scorer/ -v`
Expected: PASS（3 个用例）。

```bash
git add backend/data/khunter/scorer/ config/khunter_technical_weights.json
git commit -m "feat(khunter): 技术面评分器（权重求和+否决组合）"
```

---
