# 狩猎场风险控制与全链路回测 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复狩猎场时点、行情质量和重复入池问题，以盘后观察、1～3 交易日确认、结构/ATR 退出和全链路组合回测降低大幅亏损。

**Architecture:** 现有形态扫描与五维评分继续产生观察候选；新增数据闸门、风险过滤、确认和退出组件共同驱动生产状态机。历史研究、实时运行和回测复用同一套规则，但研究/回测不得写当前狩猎池；缺少历史时点快照时明确标记不可回测。

**Tech Stack:** Go、GORM、SQLite、Wails 2、Vue 3、TypeScript、Node 24 内置测试；不引入新依赖。

## Global Constraints

- 内部成交量单位为股、成交额单位为人民币元；单位未知、量为零、额为零或 OHLC 异常时不得确认推荐。
- 所有历史输入必须满足 `availableDate/tradeDate <= asOfDate`；缺历史基本面或板块快照时标记 `not_backtestable`，不得用最新值替代。
- 盘中只观察；收盘且同步完成后才允许确认。历史研究、盘中运行和回测均不得写当前可交易狩猎池。
- 信号日 T 不买入；T+1～T+3 才确认，最大开盘跳空 2%，最迟在入场后的第 5 个交易日退出。
- 信号指标使用同一种前复权序列；实际确认和成交使用不复权行情，信号价位按信号日不复权/前复权收盘比例转换。
- A 股 T+1：入场当日不能卖出；入场日触发的失效或突破失败只记录待退出，下一可交易日按实际可成交价处理。
- 日线只把一字涨停视为确定不可买、一字跌停视为确定不可卖；非一字板成交报告必须注明不包含真实排队数据。
- 初始风险参数集中在 `config/khunter_risk_control.json`，不得根据五只案例股调参，不在第一阶段调整五维权重。
- 回测默认全市场本地可研究清单，不使用当前狩猎池；按时间顺序 60%/20%/20% 切分，样本外确认交易少于 100 笔时结论不足。
- 五股案例只验证状态轨迹可解释，不断言“避开四只并抓住恒帅股份”。
- 各任务末尾的提交命令只在用户要求提交后执行；当前仅编写计划。

## File Structure

| 责任 | 修改 | 新增 |
|---|---|---|
| 时点与研究隔离 | `backend/data/khunter/pipeline.go`、`repo.go`、`strategy/loader.go`、`tech_profile.go`、`fundamental_fetch.go`、`sector_fetch.go` | `backend/data/khunter/research_record.go` |
| 数据闸门与单位 | `backend/models/kline_models.go`、`backend/internal/port/datasource/provider.go`、`backend/data/datasource/kline_store.go`、`fallback/kline_chain.go`、相关 provider | `backend/data/khunter/data_gate.go` 及测试 |
| 真实涨跌停 | `backend/data/khunter/strategy/limit_up_pullback.go`、`backend/data/backtest/engine.go` | `backend/data/trading/price_limit.go` 及测试 |
| 观察与交易状态 | `backend/models/khunter_models.go`、`backend/data/khunter/repo.go`、`pipeline.go` | `backend/data/khunter/observation.go` 及测试 |
| 风险、确认、退出 | `backend/data/khunter/pipeline.go`、`repo.go` | `risk_filter.go`、`risk_config.go`、`confirmation.go`、`exit.go` 及测试 |
| 组合回测与回放 | `backend/data/khunter/backtest_adapter.go`、`backend/data/backtest/engine.go` | `portfolio_backtest.go`、`backtest_report.go`、`backtest_runner.go`、`backend/cmd/khunterreplay/main.go` 及测试 |
| API 与同步前置 | `backend/data/khunter/service.go`、`backend/handler/khunter_handler.go`、`backend/agent/cron_task_api.go`、`backend/handler/system_handler.go` | 聚焦测试 |
| 前端 | `frontend/src/api/khunter.ts`、`frontend/src/components/KhunterPanel.vue`、生成的 `frontend/wailsjs/` | `frontend/src/components/khunter/presentation.ts`、`frontend/tests/khunter-presentation.test.ts` |

## Shared Types

以下类型由对应任务创建，后续任务只能按这里的字段名使用：

```go
type RiskMetrics struct {
    FiveDayReturn, MA20Deviation, ATR14, ATRRatio, MedianAmount20 float64
}

type RiskDecision struct {
    AllowConfirm bool
    Reasons      []string
    Metrics      RiskMetrics
}

type RiskControlConfig struct {
    MaxFiveDayReturn, MaxMA20Deviation, MaxATRRatio, MinMedianAmount20 float64
    ATRPeriod, ConfirmTradingDays, MaxHoldingTradingDays               int
    ATRStopMultiple, MaxEntryGap, AccountRiskFraction                  float64
    InitialCash                                                        float64
    CostConfig
}

type ConfirmationDecision struct {
    Confirmed, SkipDay       bool
    Reason                   string
    EntryPrice, StopPrice    float64
    InitialRisk              float64
}

type ExitDecision struct {
    Exit, Pending       bool
    ExitPrice           float64
    Reason              string
    NewProtectPrice     float64
    HoldingDays         int
}

type DataReadiness struct {
    TradeDate          string
    Completed          bool
    Pending, Running   int
    Failed             int
    Reason             string
}

type CostConfig struct {
    CommissionRate, MinCommission, StampDutyRate float64
    TransferFeeRate, SlippageRate                float64
}

type BacktestTrade struct {
    Code, SignalDate, EntryDate, ExitDate       string
    EntryPrice, ExitPrice                       float64
    Shares                                      int
    GrossReturn, NetReturn                      float64
    HoldingDays                                 int
    ExitReason                                  string
}

type BacktestMetrics struct {
    TradeCount, InvalidCount, UntradeableCount int
    WorstDecileAvg, MaxSingleLoss              float64
    PortfolioMaxDD, FiveDayAvgReturn           float64
    ProfitLossRatio, BenchmarkReturn           float64
    ExcessReturn                               float64
    SampleInsufficient                         bool
}

type BacktestInputManifest struct {
    Codes              []string
    StartDate, EndDate string
    CostHash           string
}

type BacktestReport struct {
    Variant          BacktestVariant
    Approximate      bool
    NotBacktestable  bool
    Stable           bool
    Limitations      []string
    SignalCount      int
    ConfirmedCount   int
    InvalidCount     int
    UntradeableCount int
    ExpiredCount     int
    Metrics          BacktestMetrics
    Groups           map[string]BacktestMetrics
}
```

---

### Task 1: 统一 K 线、资金流和技术画像的历史时点

**Files:**
- Modify: `backend/data/khunter/strategy/loader.go`
- Modify: `backend/data/khunter/strategy/limit_up_pullback.go`
- Modify: `backend/data/khunter/strategy/w_bottom.go`
- Modify: `backend/data/khunter/strategy/uptrend_dip_buy.go`
- Modify: `backend/data/khunter/strategy/immortal_guidance.go`
- Modify: `backend/data/khunter/strategy/bottom_inflection.go`
- Modify: `backend/data/khunter/repo.go`
- Modify: `backend/data/khunter/pipeline.go`
- Modify: `backend/data/khunter/tech_profile.go`
- Test: `backend/data/khunter/asof_test.go`

**Interfaces:**
- Produces: `strategy.LoadDailyBarsAsOf(ctx context.Context, code, asOfDate string, minBars int) ([]models.KLineBar, error)`。
- Produces: `Repo.GetMoneyFlowAsOf(code, asOfDate string, days int) ([]models.KhunterMoneyFlowDaily, error)`。
- Consumes: 现有 `KLineStore.QueryKLinesPreferAdjusted` 和 `setupTestDB(t)`。

- [ ] **Step 1: 写未来 K 线和未来资金流的失败测试**

```go
package khunter

import (
    "context"
    "testing"

    "go-stock/backend/data/khunter/strategy"
    "go-stock/backend/db"
    "go-stock/backend/models"
)

func TestLoadDailyBarsAsOfExcludesFuture(t *testing.T) {
    setupTestDB(t)
    if err := db.Dao.AutoMigrate(&models.KLineBar{}); err != nil {
        t.Fatal(err)
    }
    for _, d := range []string{"2026-11-10", "2026-11-11", "2026-11-12"} {
        bar := models.KLineBar{
            StockCode: "sh600000", Period: "day", TradeDate: d,
            Adjusted: true, Open: 10, High: 11, Low: 9, Close: 10,
            Volume: 10000, Amount: 100000,
        }
        if err := db.Dao.Create(&bar).Error; err != nil {
            t.Fatal(err)
        }
    }
    bars, err := strategy.LoadDailyBarsAsOf(context.Background(), "600000", "2026-11-11", 1)
    if err != nil {
        t.Fatal(err)
    }
    if len(bars) != 2 || bars[len(bars)-1].TradeDate != "2026-11-11" {
        t.Fatalf("历史加载包含未来数据: %+v", bars)
    }
}

func TestGetMoneyFlowAsOfUsesUpperBound(t *testing.T) {
    setupTestDB(t)
    r := NewRepo()
    rows := []models.KhunterMoneyFlowDaily{
        {Code: "600000", Date: "2026-11-11", MainNet: 1},
        {Code: "600000", Date: "2026-11-12", MainNet: 999},
    }
    if err := r.UpsertMoneyFlow(rows); err != nil {
        t.Fatal(err)
    }
    got, err := r.GetMoneyFlowAsOf("600000", "2026-11-11", 5)
    if err != nil || len(got) != 1 || got[0].MainNet != 1 {
        t.Fatalf("资金流越过评分日: rows=%+v err=%v", got, err)
    }
}
```

- [ ] **Step 2: 运行测试并确认因接口不存在而失败**

Run:

```bash
go test ./backend/data/khunter/ -run 'AsOf|UpperBound' -v
```

Expected: FAIL，编译错误包含 `undefined: strategy.LoadDailyBarsAsOf` 或 `undefined: r.GetMoneyFlowAsOf`。

- [ ] **Step 3: 实现带日期上界的加载与查询**

```go
func LoadDailyBarsAsOf(
    ctx context.Context, code, asOfDate string, minBars int,
) ([]models.KLineBar, error) {
    end, err := time.Parse("2006-01-02", asOfDate)
    if err != nil {
        return nil, fmt.Errorf("invalid asOfDate %q: %w", asOfDate, err)
    }
    start := end.AddDate(0, 0, -400).Format("2006-01-02")
    bars, err := datasource.NewKLineStore().QueryKLinesPreferAdjusted(
        ctx, code, "day", start, asOfDate,
    )
    if err != nil {
        return nil, err
    }
    if len(bars) < minBars {
        return nil, nil
    }
    return bars, nil
}
```

```go
func (r *Repo) GetMoneyFlowAsOf(code, asOfDate string, days int) ([]models.KhunterMoneyFlowDaily, error) {
    var rows []models.KhunterMoneyFlowDaily
    err := db.Dao.Where("code = ? AND date <= ?", code, asOfDate).
        Order("date DESC").Limit(days).Find(&rows).Error
    for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
        rows[i], rows[j] = rows[j], rows[i]
    }
    return rows, err
}
```

五种策略统一要求最后一根 K 线恰好等于选股日：

```go
if selectionDate != "" && bars[len(bars)-1].TradeDate != selectionDate {
    return nil
}
```

`RunPipeline` 改用 `LoadDailyBarsAsOf(ctx, code, tradeDate, 30)` 和 `GetMoneyFlowAsOf(code, tradeDate, 5)`；`enrichScoreTechProfiles` 按每行 `ScoreDate` 加载，不再使用当前日期窗口。保留旧 `LoadDailyBars` 和 `GetMoneyFlow` 作为其他调用方兼容包装。

- [ ] **Step 4: 运行时点与现有狩猎场测试**

Run:

```bash
go test ./backend/data/khunter/... -run 'AsOf|UpperBound|Pipeline|TechProfile' -v
```

Expected: PASS；新测试通过，现有流水线和技术画像测试无回归。

- [ ] **Step 5: 提交本任务（仅在用户要求后）**

```bash
git add backend/data/khunter backend/models/khunter_models.go
git commit -m "fix(khunter): enforce point-in-time market inputs"
```

---

### Task 2: 保存时点快照并隔离历史研究运行

**Files:**
- Modify: `backend/models/khunter_models.go`
- Modify: `backend/data/khunter/pipeline.go`
- Modify: `backend/data/khunter/fundamental_fetch.go`
- Modify: `backend/data/khunter/sector_fetch.go`
- Modify: `backend/data/khunter/repo.go`
- Create: `backend/data/khunter/research_record.go`
- Test: `backend/data/khunter/research_record_test.go`

**Interfaces:**
- Produces: `RunModeLive`、`RunModeResearch`、`PipelineOptions{TradeDate string, Mode RunMode, Now time.Time}`。
- Produces: `RunPipelineWithOptions(ctx context.Context, opts PipelineOptions) (*PipelineResult, error)`。
- Produces: `KhunterResearchRecord`、`KhunterFundamentalSnapshot`、`KhunterSectorSnapshot`。
- Produces: `Repo.GetFundamentalSnapshotAsOf(code, asOfDate string) (models.KhunterFundamentalSnapshot, error)` 和 `ErrSnapshotUnavailable`。
- Consumes: Task 1 的日期上界接口。

- [ ] **Step 1: 写研究运行不污染生产表和快照时点失败测试**

```go
func TestResearchRunDoesNotMutateCurrentHunting(t *testing.T) {
    setupTestDB(t)
    old := models.KhunterHunting{Code: "600000", EnterDate: "2026-11-10", Status: "追踪中"}
    if err := db.Dao.Create(&old).Error; err != nil {
        t.Fatal(err)
    }
    _, err := RunPipelineWithOptions(context.Background(), PipelineOptions{
        TradeDate: "2026-11-11", Mode: RunModeResearch,
    })
    if err != nil {
        t.Fatal(err)
    }
    var huntingCount, researchCount int64
    if err := db.Dao.Model(&models.KhunterHunting{}).Count(&huntingCount).Error; err != nil {
        t.Fatal(err)
    }
    if err := db.Dao.Model(&models.KhunterResearchRecord{}).Count(&researchCount).Error; err != nil {
        t.Fatal(err)
    }
    if huntingCount != 1 || researchCount != 1 {
        t.Fatalf("研究运行污染当前池: hunting=%d research=%d", huntingCount, researchCount)
    }
}

func TestFundamentalSnapshotAsOf(t *testing.T) {
    setupTestDB(t)
    rows := []models.KhunterFundamentalSnapshot{
        {Code: "600000", AvailableDate: "2026-11-01", ReportPeriod: "2026-09-30", Payload: "old"},
        {Code: "600000", AvailableDate: "2026-11-15", ReportPeriod: "2026-12-31", Payload: "future"},
    }
    if err := db.Dao.Create(&rows).Error; err != nil {
        t.Fatal(err)
    }
    got, err := NewRepo().GetFundamentalSnapshotAsOf("600000", "2026-11-11")
    if err != nil || got.Payload != "old" {
        t.Fatalf("读取到未来快照: %+v err=%v", got, err)
    }
    _, err = NewRepo().GetFundamentalSnapshotAsOf("600000", "2026-10-31")
    if !errors.Is(err, ErrSnapshotUnavailable) {
        t.Fatalf("缺快照必须返回不可回测错误: %v", err)
    }
}
```

- [ ] **Step 2: 验证测试失败**

Run:

```bash
go test ./backend/data/khunter/ -run 'ResearchRun|Snapshot' -v
```

Expected: FAIL，包含 `undefined: RunModeResearch`、`undefined: RunPipelineWithOptions` 或快照类型不存在。

- [ ] **Step 3: 实现运行模式和研究记录**

```go
type RunMode string

const (
    RunModeLive     RunMode = "live"
    RunModeResearch RunMode = "research"
)

type PipelineOptions struct {
    TradeDate string
    Mode      RunMode
    Now       time.Time
}

func RunPipeline(ctx context.Context, tradeDate string) (*PipelineResult, error) {
    return RunPipelineWithOptions(ctx, PipelineOptions{TradeDate: tradeDate, Mode: RunModeLive})
}
```

新增模型的核心字段：

```go
type KhunterResearchRecord struct {
    ID        uint      `gorm:"primarykey"`
    TradeDate string    `gorm:"size:10;uniqueIndex"`
    Status    string    `gorm:"size:30"`
    ResultJSON string   `gorm:"type:text"`
    CreatedAt time.Time
    UpdatedAt time.Time
}

type KhunterFundamentalSnapshot struct {
    ID            uint   `gorm:"primarykey"`
    Code          string `gorm:"size:20;index:idx_khfs_code_available,unique"`
    AvailableDate string `gorm:"size:10;index:idx_khfs_code_available,unique"`
    ReportPeriod  string `gorm:"size:10"`
    Payload       string `gorm:"type:text"`
    CreatedAt     time.Time
}

type KhunterSectorSnapshot struct {
    ID            uint   `gorm:"primarykey"`
    Code          string `gorm:"size:20;index:idx_khss_code_available,unique"`
    AvailableDate string `gorm:"size:10;index:idx_khss_code_available,unique"`
    Payload       string `gorm:"type:text"`
    CreatedAt     time.Time
}
```

```go
var ErrSnapshotUnavailable = errors.New("point-in-time snapshot unavailable")

func (r *Repo) GetFundamentalSnapshotAsOf(code, asOfDate string) (models.KhunterFundamentalSnapshot, error) {
    var row models.KhunterFundamentalSnapshot
    err := db.Dao.Where("code=? AND available_date<=?", code, asOfDate).
        Order("available_date DESC").First(&row).Error
    if errors.Is(err, gorm.ErrRecordNotFound) {
        return row, ErrSnapshotUnavailable
    }
    return row, err
}
```

实时模式先落快照再评分；研究模式只读取 `available_date <= TradeDate` 的快照，禁止调用最新接口补缺。研究运行只写 `KhunterResearchRecord`，不写 `KhunterScore`、`KhunterObservation`、`KhunterHunting` 和生产运行汇总。历史股票清单无法完整重建时，在结果 `Limitations` 中写入幸存者偏差。

- [ ] **Step 4: 验证研究隔离和快照读取**

Run:

```bash
go test ./backend/data/khunter/ -run 'ResearchRun|Snapshot|PointInTime' -v
```

Expected: PASS；研究记录恰好一条，原狩猎池保持不变，缺快照返回可诊断状态。

- [ ] **Step 5: 提交本任务（仅在用户要求后）**

```bash
git add backend/models/khunter_models.go backend/data/khunter
git commit -m "fix(khunter): isolate research runs and persist input snapshots"
```

---

### Task 3: 统一量额单位并建立数据质量闸门

**Files:**
- Modify: `backend/models/kline_models.go`
- Modify: `backend/internal/port/datasource/provider.go`
- Modify: `backend/data/datasource/kline_store.go`
- Modify: `backend/data/datasource/fallback/kline_chain.go`
- Modify: `backend/data/datasource/freestockdb/provider.go`
- Modify: `backend/data/eastmoney_kline_api.go`
- Create: `backend/data/khunter/data_gate.go`
- Test: `backend/data/khunter/data_gate_test.go`

**Interfaces:**
- Produces: `KLineBar.VolumeUnit`、`KLineBar.AmountUnit`。
- Produces: `NormalizeVolume(source string, raw int64) (int64, string)`。
- Produces: `DataDecision{AllowObservation, AllowConfirm, AllowBacktest bool; Status string; Reasons []string}`。
- Produces: `EvaluateDataGate(bars []models.KLineBar, asOfDate string, syncCompleted bool) DataDecision`。

- [ ] **Step 1: 写单位、零量额和盘中失败测试**

```go
func TestNormalizeVolumeUsesExplicitSourceUnits(t *testing.T) {
    tests := []struct {
        source string
        raw, want int64
        unit string
    }{
        {"eastmoney_kline", 120, 12000, "share"},
        {"mootdx_kline", 12000, 12000, "share"},
        {"freestockdb", 12000, 12000, "share"},
        {"unknown", 120, 120, "unknown"},
    }
    for _, tt := range tests {
        got, unit := NormalizeVolume(tt.source, tt.raw)
        if got != tt.want || unit != tt.unit {
            t.Fatalf("%s: got=(%d,%s)", tt.source, got, unit)
        }
    }
}

func TestDataGateRejectsInvalidBarsAndIntraday(t *testing.T) {
    valid := models.KLineBar{
        TradeDate: "2026-11-11", Open: 10, High: 11, Low: 9, Close: 10,
        Volume: 10000, Amount: 100000, VolumeUnit: "share", AmountUnit: "CNY",
    }
    zeroAmount := valid
    zeroAmount.Amount = 0
    if d := EvaluateDataGate([]models.KLineBar{zeroAmount}, "2026-11-11", true); d.AllowConfirm {
        t.Fatal("零成交额不得确认")
    }
    badOHLC := valid
    badOHLC.High = 8
    if d := EvaluateDataGate([]models.KLineBar{badOHLC}, "2026-11-11", true); d.AllowConfirm {
        t.Fatal("OHLC 异常不得确认")
    }
    d := EvaluateDataGate([]models.KLineBar{valid}, "2026-11-11", false)
    if !d.AllowObservation || d.AllowConfirm || d.Status != "intraday" {
        t.Fatalf("盘中闸门错误: %+v", d)
    }
}
```

- [ ] **Step 2: 验证测试失败**

Run:

```bash
go test ./backend/data/khunter/ -run 'DataGate|NormalizeVolume' -v
```

Expected: FAIL，包含单位字段或数据闸门接口不存在。

- [ ] **Step 3: 实现标准单位和质量校验**

```go
func NormalizeVolume(source string, raw int64) (int64, string) {
    switch source {
    case "eastmoney_kline":
        return raw * 100, "share"
    case "mootdx_kline", "freestockdb", "seed":
        return raw, "share"
    default:
        return raw, "unknown"
    }
}

func ValidateDailyBar(b models.KLineBar) []string {
    var reasons []string
    if b.Open <= 0 || b.High <= 0 || b.Low <= 0 || b.Close <= 0 {
        reasons = append(reasons, "OHLC 非正数")
    }
    if b.High < math.Max(b.Open, b.Close) ||
        b.Low > math.Min(b.Open, b.Close) || b.High < b.Low {
        reasons = append(reasons, "OHLC 关系异常")
    }
    if b.Volume <= 0 || b.VolumeUnit != "share" {
        reasons = append(reasons, "成交量缺失或单位未知")
    }
    if b.Amount <= 0 || b.AmountUnit != "CNY" {
        reasons = append(reasons, "成交额缺失或单位未知")
    }
    return reasons
}
```

`EvaluateDataGate` 只检查目标日最后一根及序列日期：`syncCompleted=false` 时允许观察、禁止确认并返回 `intraday`；质量失败返回 `invalid`；全部通过返回 `valid`。`fallback.ConvertKLineData` 同步转换 `Amount`，只有数据源明确提供人民币成交额时才设置 `AmountUnit="CNY"`。未确认单位的来源保持 `unknown`，不得根据价格反推单位。旧记录单位为空时允许展示但禁止确认。

- [ ] **Step 4: 运行数据层和闸门测试**

Run:

```bash
go test ./backend/data/khunter/ ./backend/data/datasource/... -run 'DataGate|NormalizeVolume|ConvertKLine|Upsert' -v
```

Expected: PASS；零量额、异常 OHLC、未知单位和盘中数据均不能确认。

- [ ] **Step 5: 提交本任务（仅在用户要求后）**

```bash
git add backend/models/kline_models.go backend/internal/port/datasource/provider.go backend/data/datasource backend/data/eastmoney_kline_api.go backend/data/khunter/data_gate*
git commit -m "fix(khunter): normalize kline units and gate invalid bars"
```

---

### Task 4: 按板块和最小报价单位计算真实涨跌停

**Files:**
- Create: `backend/data/trading/price_limit.go`
- Create: `backend/data/trading/price_limit_test.go`
- Modify: `backend/data/khunter/strategy/limit_up_pullback.go`
- Modify: `backend/data/backtest/engine.go`
- Test: `backend/data/khunter/strategy/limit_up_pullback_test.go`

**Interfaces:**
- Produces: `DailyPriceLimit(stockCode string, prevClose float64, isST bool) (PriceLimit, bool)`。
- Produces: `ClosedAtLimitUp`、`OnePriceLimitUp`、`OnePriceLimitDown`。
- Consumes: `models.KLineBar`；不导入 `backtest`，避免循环依赖。

- [ ] **Step 1: 写板块幅度和 20% 涨停失败测试**

```go
func TestDailyPriceLimitByBoardAndTick(t *testing.T) {
    tests := []struct {
        code string
        st bool
        up float64
    }{
        {"600000", false, 11.00},
        {"300780", false, 12.00},
        {"688322", false, 12.00},
        {"920001", false, 13.00},
        {"600000", true, 10.50},
        {"300780", true, 12.00},
    }
    for _, tt := range tests {
        limit, ok := DailyPriceLimit(tt.code, 10, tt.st)
        if !ok || math.Abs(limit.UpPrice-tt.up) > 1e-9 {
            t.Fatalf("%s: limit=%+v ok=%v", tt.code, limit, ok)
        }
    }
}

func TestFifteenPercentRiseOnChiNextIsNotLimitUp(t *testing.T) {
    limit, ok := DailyPriceLimit("300780", 10, false)
    if !ok {
        t.Fatal("expected limit rule")
    }
    bar := models.KLineBar{Open: 10, High: 11.5, Low: 10, Close: 11.5, PrevClose: 10}
    if ClosedAtLimitUp(bar, limit) {
        t.Fatal("创业板 15% 上涨不能命名为涨停")
    }
    bar.High, bar.Close = 12, 12
    if !ClosedAtLimitUp(bar, limit) {
        t.Fatal("创业板 20% 收盘应识别为涨停")
    }
}
```

- [ ] **Step 2: 验证测试失败**

Run:

```bash
go test ./backend/data/trading/ -run 'PriceLimit|ChiNext' -v
```

Expected: FAIL，新包或 `DailyPriceLimit` 尚不存在。

- [ ] **Step 3: 实现真实涨跌停价并接入口吻**

```go
type PriceLimit struct {
    UpPrice, DownPrice, Rate float64
}

func DailyPriceLimit(stockCode string, prevClose float64, isST bool) (PriceLimit, bool) {
    code := strings.ToLower(strings.TrimSpace(stockCode))
    if i := strings.LastIndex(code, "."); i >= 0 {
        code = code[:i]
    }
    code = strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(code, "sh"), "sz"), "bj")
    if len(code) != 6 || prevClose <= 0 {
        return PriceLimit{}, false
    }
    rate := 0.10
    switch {
    case strings.HasPrefix(code, "300"), strings.HasPrefix(code, "301"), strings.HasPrefix(code, "688"):
        rate = 0.20
    case code[0] == '4', code[0] == '8', strings.HasPrefix(code, "920"):
        rate = 0.30
    case isST:
        rate = 0.05
    }
    return PriceLimit{
        Rate: rate,
        UpPrice: math.Round(prevClose*(1+rate)*100) / 100,
        DownPrice: math.Round(prevClose*(1-rate)*100) / 100,
    }, true
}
```

`ClosedAtLimitUp` 用 0.001 容差比较 `bar.Close` 与 `UpPrice`；一字板要求 `Open=High=Low=Close=对应限价`。`limit_up_pullback.go` 使用关键日有效 `PrevClose` 和真实涨停价，不再以 `9.5%` 判定涨停。`backtest.Engine` 复用同一助手；`PrevClose` 无效时新狩猎场链路必须拒绝确认。

- [ ] **Step 4: 运行涨跌停、形态和回测测试**

Run:

```bash
go test ./backend/data/trading/ ./backend/data/khunter/strategy/ ./backend/data/backtest/ -run 'Limit|Pullback' -v
```

Expected: PASS；创业板 15% 不再被命名为涨停，ST 创业板仍按 20%。

- [ ] **Step 5: 提交本任务（仅在用户要求后）**

```bash
git add backend/data/trading backend/data/khunter/strategy/limit_up_pullback* backend/data/backtest/engine*
git commit -m "fix(khunter): use board-specific daily price limits"
```

---

### Task 5: 建立观察记录、交易唯一约束和幂等状态迁移

**Files:**
- Modify: `backend/models/khunter_models.go`
- Modify: `backend/data/khunter/repo.go`
- Modify: `backend/data/khunter/pipeline.go`
- Create: `backend/data/khunter/observation.go`
- Test: `backend/data/khunter/observation_test.go`
- Test: `backend/data/khunter/repo_test.go`

**Interfaces:**
- Produces: `KhunterObservation`，唯一键 `(code, signal_date)`。
- Produces: `UpsertObservation`、`GetPendingObservations(asOfDate)`、`ConfirmHunting`、`ExitHunting`。
- Produces: 扩展后的 `KhunterHunting`，包含真实信号日、入场、失效、保护、退出字段。
- Consumes: Task 3 的数据状态和 Task 4 的成交约束。

- [ ] **Step 1: 写观察幂等和候选不直接入池测试**

```go
func TestObservationUpsertIsIdempotent(t *testing.T) {
    setupTestDB(t)
    r := NewRepo()
    row := models.KhunterObservation{
        Code: "688322", SignalDate: "2026-11-11", Status: "观察中", DataStatus: "valid",
        TriggerPrice: 50, SignalLow: 45, ATR14: 2,
    }
    if err := r.UpsertObservation(&row); err != nil {
        t.Fatal(err)
    }
    row.ID = 0
    if err := r.UpsertObservation(&row); err != nil {
        t.Fatal(err)
    }
    var count int64
    if err := db.Dao.Model(&models.KhunterObservation{}).
        Where("code=? AND signal_date=?", "688322", "2026-11-11").Count(&count).Error; err != nil {
        t.Fatal(err)
    }
    if count != 1 {
        t.Fatalf("同股同信号日应只有一条观察，got %d", count)
    }
}

func TestPersistObservationsDoesNotCreateHunting(t *testing.T) {
    setupTestDB(t)
    r := NewRepo()
    rows := []models.KhunterObservation{{
        Code: "600000", SignalDate: "2026-11-11", Status: "观察中", DataStatus: "valid", Score: 80,
    }}
    if err := r.SaveObservations(rows); err != nil {
        t.Fatal(err)
    }
    var count int64
    if err := db.Dao.Model(&models.KhunterHunting{}).Count(&count).Error; err != nil {
        t.Fatal(err)
    }
    if count != 0 {
        t.Fatalf("候选直接写入狩猎池: %d", count)
    }
}
```

- [ ] **Step 2: 验证测试失败**

Run:

```bash
go test ./backend/data/khunter/ -run 'Observation|PersistObservations' -v
```

Expected: FAIL，观察模型和仓储接口不存在。

- [ ] **Step 3: 实现观察模型、交易字段和事务接口**

```go
type KhunterObservation struct {
    ID              uint      `gorm:"primarykey" json:"id"`
    Code            string    `gorm:"size:20;uniqueIndex:idx_kho_code_signal" json:"code"`
    Name            string    `gorm:"size:50" json:"name"`
    SignalDate      string    `gorm:"size:10;uniqueIndex:idx_kho_code_signal" json:"signalDate"`
    Status          string    `gorm:"size:20" json:"status"`
    DataStatus      string    `gorm:"size:30" json:"dataStatus"`
    TriggerPrice    float64   `json:"triggerPrice"`
    SignalLow       float64   `json:"signalLow"`
    ATR14           float64   `json:"atr14"`
    ConfirmExpire   string    `gorm:"size:10" json:"confirmExpire"`
    Score           float64   `json:"score"`
    SignalRiskJSON  string    `gorm:"type:text" json:"signalRiskJson"`
    RiskHistoryJSON string    `gorm:"type:text" json:"riskHistoryJson"`
    RejectReason    string    `gorm:"type:text" json:"rejectReason"`
    CreatedAt       time.Time `json:"createdAt"`
    UpdatedAt       time.Time `json:"updatedAt"`
}
```

`KhunterHunting` 新增 `SignalDate`、`EntryPrice`、`InitialStopPrice`、`ProtectPrice`、`InitialRisk`、`Shares`、`ExitDate`、`ExitPrice`、`ExitReason`；新状态只允许 `持仓中 → 已退出`。唯一键为 `(code,signal_date)` 和 `(code,enter_date)`。迁移前在事务内把旧记录状态改为 `历史待核`，同组重复记录保留最新 ID 并删除其余行；旧 `signal_date` 用 `enter_date` 补齐后再建唯一索引。

```go
func (r *Repo) SaveObservations(rows []models.KhunterObservation) error {
    for i := range rows {
        if err := r.UpsertObservation(&rows[i]); err != nil {
            return err
        }
    }
    return nil
}

func (r *Repo) UpsertObservation(row *models.KhunterObservation) error {
    return db.Dao.Clauses(clause.OnConflict{
        Columns: []clause.Column{{Name: "code"}, {Name: "signal_date"}},
        DoUpdates: clause.AssignmentColumns([]string{
            "name", "data_status", "trigger_price", "signal_low", "atr14",
            "confirm_expire", "score", "signal_risk_json", "risk_history_json",
            "reject_reason", "updated_at",
        }),
    }).Create(row).Error
}
```

`UpsertObservation` 故意不覆盖已有 `status`：已确认或已过期状态只能由对应事务迁移，不能被同日重跑重置为观察中。

流水线删除“分数达阈值直接 `SaveHunting`”逻辑，改为保存观察；无新信号也必须继续处理旧观察和已有持仓。`ConfirmHunting` 在一个事务内校验观察状态、创建交易并更新观察；`ExitHunting` 使用 `WHERE id=? AND status='持仓中'`。研究和盘中运行不得调用生产确认。

- [ ] **Step 4: 运行幂等、迁移和流水线测试**

Run:

```bash
go test ./backend/data/khunter/ -run 'Observation|Idempotent|HuntingMigration|PersistObservations|Pipeline' -v
```

Expected: PASS；候选不直接入池，旧重复数据迁移成功，旧记录显示为历史待核。

- [ ] **Step 5: 提交本任务（仅在用户要求后）**

```bash
git add backend/models/khunter_models.go backend/data/khunter/observation* backend/data/khunter/repo* backend/data/khunter/pipeline*
git commit -m "refactor(khunter): add idempotent observation and position states"
```

---

### Task 6: 将技术画像转为可配置风险过滤

**Files:**
- Create: `backend/data/khunter/risk_filter.go`
- Create: `backend/data/khunter/risk_filter_test.go`
- Create: `backend/data/khunter/risk_config.go`
- Create: `backend/data/khunter/risk_config_test.go`
- Create: `config/khunter_risk_control.json`
- Modify: `backend/data/khunter/pipeline.go`

**Interfaces:**
- Produces: `RiskMetrics`、`RiskDecision`、`BuildRiskMetrics`、`EvaluateRisk`、`DefaultRiskControlConfig`。
- Consumes: Task 3 的 `DataDecision` 和现有 `risk.Level`。

- [ ] **Step 1: 写四类风险阈值和降级否决测试**

```go
func TestEvaluateRiskThresholds(t *testing.T) {
    cfg := DefaultRiskControlConfig()
    base := RiskMetrics{FiveDayReturn: 0.10, MA20Deviation: 0.05, ATRRatio: 0.03, MedianAmount20: 2e8}
    tests := []struct {
        name string
        mutate func(*RiskMetrics)
    }{
        {"近5日过热", func(m *RiskMetrics) { m.FiveDayReturn = 0.2001 }},
        {"MA20乖离过大", func(m *RiskMetrics) { m.MA20Deviation = 0.1501 }},
        {"ATR过高", func(m *RiskMetrics) { m.ATRRatio = 0.0801 }},
        {"流动性不足", func(m *RiskMetrics) { m.MedianAmount20 = 99999999 }},
    }
    for _, tt := range tests {
        m := base
        tt.mutate(&m)
        d := EvaluateRisk(m, DataDecision{AllowConfirm: true}, risk.LevelOf(-0.01), cfg)
        if d.AllowConfirm || len(d.Reasons) == 0 {
            t.Fatalf("%s 应拒绝确认: %+v", tt.name, d)
        }
    }
}

func TestEvaluateRiskBlocksCrashAndDegradedData(t *testing.T) {
    cfg := DefaultRiskControlConfig()
    m := RiskMetrics{MedianAmount20: 2e8}
    if d := EvaluateRisk(m, DataDecision{AllowConfirm: true}, risk.LevelOf(-0.09), cfg); d.AllowConfirm {
        t.Fatal("崩溃档不得新增风险")
    }
    if d := EvaluateRisk(m, DataDecision{AllowConfirm: false}, risk.LevelOf(-0.01), cfg); d.AllowConfirm {
        t.Fatal("数据降级不得确认")
    }
}
```

- [ ] **Step 2: 验证测试失败**

Run:

```bash
go test ./backend/data/khunter/ -run 'EvaluateRisk|RiskControlConfig' -v
```

Expected: FAIL，风险配置和评估函数不存在。

- [ ] **Step 3: 实现配置、指标和纯风险判断**

```json
{
  "maxFiveDayReturn": 0.20,
  "maxMA20Deviation": 0.15,
  "maxATRRatio": 0.08,
  "minMedianAmount20": 100000000,
  "atrPeriod": 14,
  "atrStopMultiple": 1.5,
  "maxEntryGap": 0.02,
  "confirmTradingDays": 3,
  "maxHoldingTradingDays": 5,
  "accountRiskFraction": 0.005,
  "initialCash": 1000000,
  "commissionRate": 0.0003,
  "minCommission": 5,
  "stampDutyRate": 0.0005,
  "transferFeeRate": 0,
  "slippageRate": 0.0005
}
```

```go
func EvaluateRisk(m RiskMetrics, data DataDecision, market risk.Level, cfg RiskControlConfig) RiskDecision {
    d := RiskDecision{Metrics: m}
    if !data.AllowConfirm {
        d.Reasons = append(d.Reasons, "数据不允许确认")
    }
    if m.FiveDayReturn > cfg.MaxFiveDayReturn {
        d.Reasons = append(d.Reasons, "近5日涨幅过热")
    }
    if m.MA20Deviation > cfg.MaxMA20Deviation {
        d.Reasons = append(d.Reasons, "MA20正向乖离过大")
    }
    if m.ATRRatio > cfg.MaxATRRatio {
        d.Reasons = append(d.Reasons, "ATR波动过高")
    }
    if m.MedianAmount20 < cfg.MinMedianAmount20 {
        d.Reasons = append(d.Reasons, "流动性不足")
    }
    if market.PositionLimit <= 0 {
        d.Reasons = append(d.Reasons, "市场风险档禁止新增")
    }
    d.AllowConfirm = len(d.Reasons) == 0
    return d
}
```

`BuildRiskMetrics` 使用 `T close/T-5 close-1`、`close/MA20-1`、现有指标包的 ATR(14) 和最近 20 根有效 K 线成交额中位数；不足 20 根返回错误并禁止确认。信号日和每个拟入场日都重新计算，结果追加到 `RiskHistoryJSON`；风险过滤不修改形态或五维权重。

- [ ] **Step 4: 运行风险测试**

Run:

```bash
go test ./backend/data/khunter/ -run 'RiskMetrics|EvaluateRisk|RiskControlConfig' -v
```

Expected: PASS；阈值由配置驱动，信号日和入场日结果均可持久化。

- [ ] **Step 5: 提交本任务（仅在用户要求后）**

```bash
git add backend/data/khunter/risk_* config/khunter_risk_control.json backend/data/khunter/pipeline.go
git commit -m "feat(khunter): add configurable pre-trade risk filters"
```

---

### Task 7: 实现 T+1～T+3 确认、跳空约束和风险预算仓位

**Files:**
- Create: `backend/data/khunter/confirmation.go`
- Create: `backend/data/khunter/confirmation_test.go`
- Modify: `backend/data/khunter/pipeline.go`
- Modify: `backend/data/khunter/repo.go`

**Interfaces:**
- Produces: `EvaluateConfirmationDay`、`ConfirmationWindowState`、`RiskBudgetShares`、`RawSignalLevel`。
- Consumes: Task 4 的 `PriceLimit`、Task 5 的观察仓储、Task 6 的风险配置。
- Produces: 真实不复权 `EntryPrice`、`StopPrice`、`InitialRisk` 和股数。

- [ ] **Step 1: 写确认价、窗口、复权转换和风险预算失败测试**

```go
func TestConfirmationPriceAndGap(t *testing.T) {
    cfg := DefaultRiskControlConfig()
    allow := RiskDecision{AllowConfirm: true}
    limit, _ := trading.DailyPriceLimit("600000", 90, false)
    tests := []struct {
        name string
        bar models.KLineBar
        entry float64
        confirmed, skip bool
    }{
        {"开盘位于H与H+2%之间", models.KLineBar{Open: 101, High: 102, Low: 100, Close: 101}, 101, true, false},
        {"开盘低于H且盘中触及", models.KLineBar{Open: 99, High: 100, Low: 98, Close: 99.5}, 100, true, false},
        {"开盘高于H+2%", models.KLineBar{Open: 102.01, High: 103, Low: 101, Close: 102}, 0, false, true},
        {"未触及H", models.KLineBar{Open: 99, High: 99.9, Low: 98, Close: 99}, 0, false, false},
    }
    for _, tt := range tests {
        d := EvaluateConfirmationDay(100, 95, 2, tt.bar, limit, allow, cfg)
        if d.Confirmed != tt.confirmed || d.SkipDay != tt.skip || math.Abs(d.EntryPrice-tt.entry) > 1e-9 {
            t.Fatalf("%s: %+v", tt.name, d)
        }
    }
}

func TestConfirmationWindowAndRawPrice(t *testing.T) {
    days := []string{"2026-11-10", "2026-11-11", "2026-11-12", "2026-11-13", "2026-11-16"}
    if got := ConfirmationWindowState(days[0], days[3], days); got != "open" {
        t.Fatalf("T+3 应可确认: %s", got)
    }
    if got := ConfirmationWindowState(days[0], days[4], days); got != "expired" {
        t.Fatalf("T+4 应过期: %s", got)
    }
    raw, err := RawSignalLevel(120, 100, 90)
    if err != nil || raw != 108 {
        t.Fatalf("不复权信号价转换错误: raw=%v err=%v", raw, err)
    }
}

func TestInitialStopAndRiskBudget(t *testing.T) {
    d := EvaluateConfirmationDay(100, 96, 3,
        models.KLineBar{Open: 99, High: 101, Low: 98, Close: 100},
        trading.PriceLimit{UpPrice: 110, DownPrice: 90}, RiskDecision{AllowConfirm: true}, DefaultRiskControlConfig())
    if !d.Confirmed || d.StopPrice != 96 || d.InitialRisk != 4 {
        t.Fatalf("初始失效价错误: %+v", d)
    }
    if shares := RiskBudgetShares(1e6, 100, 96, 1, 0); shares != 1200 {
        t.Fatalf("风险预算股数=%d", shares)
    }
}
```

- [ ] **Step 2: 验证测试失败**

Run:

```bash
go test ./backend/data/khunter/ -run 'Confirmation|InitialStop|RiskBudget|RawPrice' -v
```

Expected: FAIL，确认和转换函数不存在。

- [ ] **Step 3: 实现纯确认函数与生产事务**

```go
func RawSignalLevel(level, adjustedClose, rawClose float64) (float64, error) {
    if level <= 0 || adjustedClose <= 0 || rawClose <= 0 {
        return 0, fmt.Errorf("invalid price conversion inputs")
    }
    return math.Round(level*rawClose/adjustedClose*100) / 100, nil
}

func EvaluateConfirmationDay(
    signalHigh, signalLow, atr float64,
    bar models.KLineBar,
    limit trading.PriceLimit,
    risk RiskDecision,
    cfg RiskControlConfig,
) ConfirmationDecision {
    if !risk.AllowConfirm {
        return ConfirmationDecision{SkipDay: true, Reason: strings.Join(risk.Reasons, "；")}
    }
    if trading.OnePriceLimitUp(bar, limit) {
        return ConfirmationDecision{SkipDay: true, Reason: "一字涨停不可成交"}
    }
    if bar.Open > signalHigh*(1+cfg.MaxEntryGap) {
        return ConfirmationDecision{SkipDay: true, Reason: "开盘跳空超过2%"}
    }
    entry := 0.0
    switch {
    case bar.Open >= signalHigh:
        entry = bar.Open
    case bar.High >= signalHigh:
        entry = signalHigh
    default:
        return ConfirmationDecision{Reason: "未触及确认价"}
    }
    stop := math.Max(signalLow, entry-cfg.ATRStopMultiple*atr)
    if entry <= 0 || stop <= 0 || stop >= entry {
        return ConfirmationDecision{SkipDay: true, Reason: "失效价或风险距离无效"}
    }
    return ConfirmationDecision{Confirmed: true, EntryPrice: entry, StopPrice: stop, InitialRisk: entry-stop}
}
```

`ConfirmationWindowState` 按传入的真实交易日索引计算，T 返回 `signal_day`，T+1～T+3 返回 `open`，之后返回 `expired`；信号不在交易日序列时返回 `invalid`。`RiskBudgetShares` 同时受 `equity×0.005/(entry-stop)`、`equity×positionLimit-usedExposure` 和 100 股整数手约束。流水线加载不复权执行 K 线，先转换 H/L/ATR，再调用确认；不足一手、跳空、风险失败只记录当日原因，窗口内后续交易日仍可确认。只有 `ConfirmHunting` 事务成功后才把观察改为已确认。

- [ ] **Step 4: 运行确认和幂等测试**

Run:

```bash
go test ./backend/data/khunter/ -run 'Confirmation|ConfirmWindow|InitialStop|RiskBudget|RawPrice|Idempotent' -v
```

Expected: PASS；无信号日收盘买入，无复权价冒充真实成交价，同一观察只生成一笔交易。

- [ ] **Step 5: 提交本任务（仅在用户要求后）**

```bash
git add backend/data/khunter/confirmation* backend/data/khunter/pipeline.go backend/data/khunter/repo.go
git commit -m "feat(khunter): confirm raw-price entries within three sessions"
```

---

### Task 8: 实现结构/ATR、突破失败、1R 锁盈和五日退出

**Files:**
- Create: `backend/data/khunter/exit.go`
- Create: `backend/data/khunter/exit_test.go`
- Modify: `backend/data/khunter/pipeline.go`
- Modify: `backend/data/khunter/repo.go`

**Interfaces:**
- Produces: `EvaluateExitDay(position, signalHigh, bar, limit, isEntryDay, holdingDays, cfg) ExitDecision`。
- Consumes: Task 5 的交易记录、Task 7 的真实入场和失效价。

- [ ] **Step 1: 写跳空、T+1 和不利顺序失败测试**

```go
func TestExitGapThroughStopUsesOpen(t *testing.T) {
    p := models.KhunterHunting{EntryPrice: 100, InitialStopPrice: 90, ProtectPrice: 90, InitialRisk: 10, Status: "持仓中"}
    d := EvaluateExitDay(p, 100,
        models.KLineBar{Open: 85, High: 88, Low: 84, Close: 86},
        trading.PriceLimit{DownPrice: 81}, false, 1, DefaultRiskControlConfig())
    if !d.Exit || d.ExitPrice != 85 || d.Reason != "失效位跳空退出" {
        t.Fatalf("跳空退出错误: %+v", d)
    }
}

func TestOneRDoesNotUseOptimisticIntradayOrder(t *testing.T) {
    p := models.KhunterHunting{EntryPrice: 100, InitialStopPrice: 90, ProtectPrice: 90, InitialRisk: 10}
    d := EvaluateExitDay(p, 100,
        models.KLineBar{Open: 100, High: 111, Low: 89, Close: 105},
        trading.PriceLimit{DownPrice: 80}, false, 1, DefaultRiskControlConfig())
    if !d.Exit || d.ExitPrice != 90 {
        t.Fatalf("不得假定先到1R再保本退出: %+v", d)
    }
}

func TestEntryDayStopWaitsForTPlusOne(t *testing.T) {
    p := models.KhunterHunting{EntryPrice: 100, InitialStopPrice: 90, ProtectPrice: 90, InitialRisk: 10}
    entryDay := EvaluateExitDay(p, 100,
        models.KLineBar{Open: 100, High: 101, Low: 85, Close: 88},
        trading.PriceLimit{DownPrice: 80}, true, 0, DefaultRiskControlConfig())
    if entryDay.Exit || !entryDay.Pending {
        t.Fatalf("A股入场日不得假设卖出: %+v", entryDay)
    }
    nextDay := EvaluateExitDay(p, 100,
        models.KLineBar{Open: 80, High: 82, Low: 78, Close: 79},
        trading.PriceLimit{DownPrice: 79.2}, false, 1, DefaultRiskControlConfig())
    if !nextDay.Exit || nextDay.ExitPrice != 80 {
        t.Fatalf("下一交易日应按实际开盘价退出: %+v", nextDay)
    }
}
```

- [ ] **Step 2: 验证测试失败**

Run:

```bash
go test ./backend/data/khunter/ -run 'Exit|OneR|TPlusOne' -v
```

Expected: FAIL，`EvaluateExitDay` 不存在。

- [ ] **Step 3: 实现固定顺序的退出状态机**

```go
func EvaluateExitDay(
    p models.KhunterHunting,
    signalHigh float64,
    bar models.KLineBar,
    limit trading.PriceLimit,
    isEntryDay bool,
    holdingDays int,
    cfg RiskControlConfig,
) ExitDecision {
    if isEntryDay {
        pending := bar.Low <= p.ProtectPrice || bar.Close < signalHigh
        return ExitDecision{Pending: pending, NewProtectPrice: p.ProtectPrice, HoldingDays: 0}
    }
    if trading.OnePriceLimitDown(bar, limit) {
        return ExitDecision{Pending: true, NewProtectPrice: p.ProtectPrice, HoldingDays: holdingDays}
    }
    if bar.Open <= p.ProtectPrice {
        return ExitDecision{Exit: true, ExitPrice: bar.Open, Reason: "失效位跳空退出", NewProtectPrice: p.ProtectPrice, HoldingDays: holdingDays}
    }
    if bar.Low <= p.ProtectPrice {
        return ExitDecision{Exit: true, ExitPrice: p.ProtectPrice, Reason: "失效位退出", NewProtectPrice: p.ProtectPrice, HoldingDays: holdingDays}
    }
    if bar.Close < signalHigh {
        return ExitDecision{Exit: true, ExitPrice: bar.Close, Reason: "突破失败", NewProtectPrice: p.ProtectPrice, HoldingDays: holdingDays}
    }
    protect := p.ProtectPrice
    if bar.High >= p.EntryPrice+p.InitialRisk && protect < p.EntryPrice {
        protect = p.EntryPrice
    }
    if holdingDays >= cfg.MaxHoldingTradingDays {
        return ExitDecision{Exit: true, ExitPrice: bar.Close, Reason: "到期退出", NewProtectPrice: protect, HoldingDays: holdingDays}
    }
    return ExitDecision{NewProtectPrice: protect, HoldingDays: holdingDays}
}
```

处理顺序固定为：入场日只记录待退出；一字跌停继续持有；使用本日开始前旧保护位处理跳空和触及；处理突破失败；未退出时才根据 1R 提高后续保护位；最后处理到期退出。`TrackDays` 按真实交易日递增，退出重跑不重复更新。

- [ ] **Step 4: 运行退出测试**

Run:

```bash
go test ./backend/data/khunter/ -run 'Exit|OneR|TPlusOne|FiveDay|LimitDown' -v
```

Expected: PASS；入场日没有卖出，跳空按实际开盘价，退出由真实状态和交易日驱动。

- [ ] **Step 5: 提交本任务（仅在用户要求后）**

```bash
git add backend/data/khunter/exit* backend/data/khunter/pipeline.go backend/data/khunter/repo.go
git commit -m "feat(khunter): add structural ATR and time-based exits"
```

---

### Task 9: 实现成本、组合资金和尾部风险指标

**Files:**
- Modify: `backend/data/backtest/engine.go`
- Modify: `backend/data/khunter/backtest_adapter.go`
- Create: `backend/data/khunter/portfolio_backtest.go`
- Create: `backend/data/khunter/portfolio_backtest_test.go`
- Create: `backend/data/khunter/backtest_report.go`
- Create: `backend/data/khunter/backtest_report_test.go`

**Interfaces:**
- Produces: `CostConfig`、`BacktestTrade`、`BacktestMetrics`、`EquityMaxDrawdown`、`SummarizeReturns`。
- Consumes: Task 7 的股数、Task 8 的退出轨迹和 Task 6 的成本配置。

- [ ] **Step 1: 写尾部亏损、权益回撤和失败样本守恒测试**

```go
func TestWorstDecileAndMaximumLoss(t *testing.T) {
    returns := []float64{-0.20, -0.10, -0.05, 0, 0.01, 0.02, 0.03, 0.04, 0.05, 0.10}
    m := SummarizeReturns(returns)
    if m.WorstDecileAvg != -0.20 || m.MaxSingleLoss != -0.20 {
        t.Fatalf("尾部风险指标错误: %+v", m)
    }
}

func TestPortfolioDrawdownUsesDailyEquityPath(t *testing.T) {
    got := EquityMaxDrawdown([]float64{100000, 110000, 99000, 105000})
    want := (110000.0 - 99000.0) / 110000.0
    if math.Abs(got-want) > 1e-9 {
        t.Fatalf("组合最大回撤=%v, want=%v", got, want)
    }
}

func TestFailureSamplesRemainInReport(t *testing.T) {
    report := NewBacktestReport(2)
    report.AddInvalid("缺成交额")
    report.AddTrade(BacktestTrade{NetReturn: 0.01})
    if report.SignalCount != 2 || report.InvalidCount != 1 || report.TradeCount != 1 {
        t.Fatalf("失败样本被静默丢弃: %+v", report)
    }
}
```

- [ ] **Step 2: 验证测试失败**

Run:

```bash
go test ./backend/data/khunter/ -run 'WorstDecile|PortfolioDrawdown|FailureSamples' -v
```

Expected: FAIL，组合报告和指标函数不存在。

- [ ] **Step 3: 实现成本后收益和每日组合权益**

```go
func EquityMaxDrawdown(equity []float64) float64 {
    if len(equity) == 0 {
        return 0
    }
    peak := equity[0]
    maxDD := 0.0
    for _, value := range equity {
        if value > peak {
            peak = value
        }
        if peak > 0 {
            maxDD = math.Max(maxDD, (peak-value)/peak)
        }
    }
    return maxDD
}
```

组合引擎按交易日处理：先退出，再按确认时间和代码确定性排序入场；现金被持仓占用，总敞口不得超过当日 `PositionLimit`，不足一手记为不可成交。买入价加滑点、卖出价减滑点；净收益扣双边佣金（最低 5 元）、卖出印花税和配置过户费。每日权益为现金加全部持仓当日市值，最大回撤从该序列计算。基准固定使用 `sh000300`。零收益交易单独计数，不得混入平均亏损分母。

- [ ] **Step 4: 运行组合、成本和旧回测回归测试**

Run:

```bash
go test ./backend/data/khunter/ ./backend/data/backtest/ -run 'Portfolio|Equity|WorstDecile|FailureSamples|Cost' -v
```

Expected: PASS；重叠交易不透支，成本可复算，失败样本计数守恒。

- [ ] **Step 5: 提交本任务（仅在用户要求后）**

```bash
git add backend/data/khunter/*backtest* backend/data/khunter/portfolio* backend/data/backtest/engine.go
git commit -m "feat(khunter): model portfolio costs execution and tail risk"
```

---

### Task 10: 实现全链路四组回测、时间切分和案例回放 CLI

**Files:**
- Modify: `backend/data/khunter/backtest_adapter.go`
- Create: `backend/data/khunter/backtest_runner.go`
- Create: `backend/data/khunter/backtest_runner_test.go`
- Create: `backend/cmd/khunterreplay/main.go`

**Interfaces:**
- Produces: `BacktestVariant`、`BacktestRequest`、`BacktestReport`、`SplitTradingDays`、`RunFullBacktest`。
- Consumes: Task 1～9 的全部生产规则，禁止复制简化形态逻辑。

- [ ] **Step 1: 写四组一致输入和时间切分失败测试**

```go
func TestBacktestVariantsShareUniverseAndDates(t *testing.T) {
    req := BacktestRequest{
        StartDate: "2026-01-01", EndDate: "2026-12-31", InitialCash: 1e6,
        Variants: []BacktestVariant{VariantBaseline, VariantDataOnly, VariantRiskOnly, VariantFull},
    }
    normalized, err := NormalizeBacktestRequest(req)
    if err != nil || len(normalized.Variants) != 4 {
        t.Fatalf("四组配置错误: %+v err=%v", normalized, err)
    }
    manifest := BacktestInputManifest{Codes: []string{"600000", "300780"}, StartDate: req.StartDate, EndDate: req.EndDate, CostHash: "same"}
    for _, variant := range normalized.Variants {
        if got := BuildVariantInput(variant, manifest); got.CostHash != "same" || len(got.Codes) != 2 {
            t.Fatalf("%s 改变了输入: %+v", variant, got)
        }
    }
}

func TestSplitsAreChronologicalAndKeepExitData(t *testing.T) {
    days := []string{"2026-01-05", "2026-01-06", "2026-01-07", "2026-01-08", "2026-01-09", "2026-01-12", "2026-01-13", "2026-01-14", "2026-01-15", "2026-01-16"}
    splits := SplitTradingDays(days, 120, 8)
    if len(splits) != 3 || splits[0].SignalEnd >= splits[1].SignalStart || splits[1].SignalEnd >= splits[2].SignalStart {
        t.Fatalf("时间切分交叉: %+v", splits)
    }
    if splits[2].ExitDataEnd != days[len(days)-1] {
        t.Fatalf("样本外缺少退出数据: %+v", splits[2])
    }
}
```

- [ ] **Step 2: 验证测试失败**

Run:

```bash
go test ./backend/data/khunter/ -run 'BacktestVariants|Splits' -v
```

Expected: FAIL，完整回测类型和切分函数不存在。

- [ ] **Step 3: 实现四组策略、报告守恒和 CLI**

```go
type BacktestVariant string

const (
    VariantBaseline BacktestVariant = "baseline"
    VariantDataOnly BacktestVariant = "data_only"
    VariantRiskOnly BacktestVariant = "risk"
    VariantFull BacktestVariant = "full"
)

type BacktestRequest struct {
    StartDate, EndDate string
    Codes []string
    InitialCash float64
    Variants []BacktestVariant
    ConfigPath string
}

func BuildVariantInput(_ BacktestVariant, manifest BacktestInputManifest) BacktestInputManifest {
    manifest.Codes = append([]string(nil), manifest.Codes...)
    return manifest
}
```

`BuildVariantInput` 是四组共享输入的显式边界：只允许附加策略开关，不允许改变股票、日期或成本哈希。

`Codes` 为空时使用本地全市场可研究清单，并在报告写入当前股票清单带来的幸存者偏差。四组共享输入清单、日期和成本；`baseline` 缺快照时标 `Approximate=true`，`data_only` 只启用数据修复，`risk` 增加风险过滤，`full` 复用确认和退出。缺历史快照时完整五维结果标 `NotBacktestable=true`，不得调用最新接口。

`SplitTradingDays` 按 60%/20%/20% 切分信号日，预热向前读、确认退出向后读，不在区间外生成交易。报告满足 `信号=确认+过期+无效+不可成交+仍在窗口`；运行结束仍有窗口内记录则标未完成。样本外确认少于 100 笔设置 `SampleInsufficient=true`；不同市场阶段结论方向不一致设置 `Stable=false`。

CLI 支持 `-db -start -end -all-market -variants -config -cases -out`，输出逐日观察、风险、确认、入场、保护和退出轨迹。中稀有色的案例代码必须是 `600259.SH`。

- [ ] **Step 4: 运行全链路测试并构建 CLI**

Run:

```bash
go test ./backend/data/khunter/ -run 'BacktestVariants|Splits|PointInTime|Replay|Report' -v
go build ./backend/cmd/khunterreplay
```

Expected: PASS 且构建成功；四组输入一致，缺快照和样本不足均明确显示。

- [ ] **Step 5: 提交本任务（仅在用户要求后）**

```bash
git add backend/data/khunter/backtest* backend/data/khunter/portfolio* backend/cmd/khunterreplay
git commit -m "feat(khunter): add full-chain comparative backtests and replay"
```

---

### Task 11: 接通 Service、Handler 和同步完成前置条件

**Files:**
- Modify: `backend/data/khunter/service.go`
- Modify: `backend/handler/khunter_handler.go`
- Modify: `backend/agent/cron_task_api.go`
- Modify: `backend/handler/system_handler.go`
- Test: `backend/data/khunter/readiness_test.go`
- Test: `backend/handler/khunter_handler_test.go`
- Test: `backend/agent/cron_task_api_test.go`

**Interfaces:**
- Produces: `Service.RunResearchPipeline`、`Service.GetObservations`、`Service.RunFullBacktest`。
- Produces: `KhunterHandler.RunResearchPipelineAsync`、`GetObservations`、`RunFullBacktestAsync`。
- Produces: `RequireConfirmationReadiness(DataReadiness) error` 和 `ErrDataNotReady`。

- [ ] **Step 1: 写同步未完成的失败测试**

```go
func TestPipelineRejectsUnfinishedCurrentDaySync(t *testing.T) {
    readiness := DataReadiness{TradeDate: "2026-11-18", Completed: false, Reason: "存在 running 日K同步任务"}
    err := RequireConfirmationReadiness(readiness)
    if !errors.Is(err, ErrDataNotReady) {
        t.Fatalf("同步未完成仍允许确认: %v", err)
    }
}

func TestReadinessFailureHasDiagnosticReason(t *testing.T) {
    readiness := DataReadiness{TradeDate: "2026-11-18", Completed: false, Pending: 2, Running: 1, Reason: "pending=2 running=1"}
    err := RequireConfirmationReadiness(readiness)
    if err == nil || !strings.Contains(err.Error(), "pending=2") {
        t.Fatalf("错误缺少诊断信息: %v", err)
    }
}
```

- [ ] **Step 2: 验证测试失败**

Run:

```bash
go test ./backend/data/khunter/ -run 'Readiness' -v
```

Expected: FAIL，`DataReadiness` 和 `RequireConfirmationReadiness` 不存在。

- [ ] **Step 3: 实现数据就绪检查和 API**

```go
var ErrDataNotReady = errors.New("kline data not ready")

func RequireConfirmationReadiness(r DataReadiness) error {
    if r.Completed {
        return nil
    }
    return fmt.Errorf("%w: %s (pending=%d running=%d failed=%d)", ErrDataNotReady, r.Reason, r.Pending, r.Running, r.Failed)
}
```

`DataReadiness` 从 `KLineSyncLog` 的 pending/running/failed、`LastSyncDate` 和当日覆盖计算；15:30、17:00 只作为触发时间。未就绪可保存盘中观察，但不得确认，事件返回具体错误。保留旧 `RunPipelineAsync` 和旧回测入口兼容；新历史按钮调研究接口，新回测传完整报告。`PipelineResult` 和运行汇总增加 `InvalidDataCount`、`IntradayCount`、`ConfirmedCount`、`ExpiredCount`、`UntradeableCount`。

- [ ] **Step 4: 运行服务、Handler 和调度测试**

Run:

```bash
go test ./backend/data/khunter/ ./backend/handler/ ./backend/agent/ -run 'Readiness|ResearchPipeline|FullBacktest|Observation' -v
```

Expected: PASS；同步未完成不能确认，研究/回测不写生产池，错误原因完整传递。

- [ ] **Step 5: 提交本任务（仅在用户要求后）**

```bash
git add backend/data/khunter backend/handler/khunter_handler* backend/agent/cron_task_api* backend/handler/system_handler.go
git commit -m "feat(khunter): expose research workflows and enforce sync readiness"
```

---

### Task 12: 前端状态分层、运行诊断和全市场回测

**Files:**
- Modify: `frontend/src/api/khunter.ts`
- Modify: `frontend/src/components/KhunterPanel.vue`
- Modify: `frontend/wailsjs/`（通过 Wails 生成）
- Create: `frontend/src/components/khunter/presentation.ts`
- Test: `frontend/tests/khunter-presentation.test.ts`

**Interfaces:**
- Consumes: Task 11 的研究、观察和完整回测 API。
- Produces: `observationLabel`、`huntingStatusType`、`backtestSampleWarning`。

- [ ] **Step 1: 写状态展示和样本不足失败测试**

```ts
import test from 'node:test'
import assert from 'node:assert/strict'
import { observationLabel, huntingStatusType, backtestSampleWarning } from '../src/components/khunter/presentation.ts'

test('观察与确认状态不会混为买入推荐', () => {
  assert.equal(observationLabel({ status: '观察中', dataStatus: 'intraday' }), '盘中观察')
  assert.equal(observationLabel({ status: '观察中', dataStatus: 'invalid' }), '数据无效')
  assert.equal(observationLabel({ status: '已过期', dataStatus: 'valid' }), '观察过期')
})

test('历史待核不显示为持仓中', () => {
  assert.equal(huntingStatusType('历史待核'), 'default')
  assert.equal(huntingStatusType('持仓中'), 'success')
  assert.equal(huntingStatusType('已退出'), 'info')
})

test('样本外不足100笔必须显示警告', () => {
  assert.match(backtestSampleWarning({ confirmedCount: 99 }), /样本不足/)
  assert.equal(backtestSampleWarning({ confirmedCount: 100 }), '')
})
```

- [ ] **Step 2: 验证前端测试失败**

Run:

```bash
cd frontend
node --import ./tests/register-ts-ext.mjs --test tests/khunter-presentation.test.ts
```

Expected: FAIL，无法加载 `presentation.ts`。

- [ ] **Step 3: 实现展示函数并接入页面**

```ts
export function observationLabel(row: { status?: string; dataStatus?: string; rejectReason?: string }): string {
  if (row.dataStatus === 'intraday') return '盘中观察'
  if (row.dataStatus === 'invalid') return '数据无效'
  if (row.dataStatus === 'not_backtestable') return '不可回测'
  if (row.status === '已确认') return '已确认'
  if (row.status === '已过期') return '观察过期'
  if (row.rejectReason?.includes('跳空')) return '过度跳空'
  return '观察中'
}

export function huntingStatusType(status?: string): 'success' | 'info' | 'default' {
  if (status === '持仓中') return 'success'
  if (status === '已退出') return 'info'
  return 'default'
}

export function backtestSampleWarning(row: { confirmedCount?: number }): string {
  return (row.confirmedCount ?? 0) < 100
    ? '样本外确认交易少于100笔，样本不足，不用于替换默认策略'
    : ''
}
```

页面将“候选评分”明确展示为“观察候选”，显示信号日、数据状态、原因、触发价和有效期；确认推荐显示真实入场、失效/保护价、股数、持有交易日和退出。运行概览增加无效、盘中、确认、过期和不可成交数量。回测默认全市场，显示四组对照、近似/不可回测、样本不足、尾部亏损、组合回撤和超额收益；不再默认读取当前狩猎池。Wails 绑定通过命令生成，不手编。

- [ ] **Step 4: 生成绑定并运行前端测试、类型检查和构建**

Run:

```bash
wails generate module
cd frontend
node --import ./tests/register-ts-ext.mjs --test tests/khunter-presentation.test.ts
node tests/check-vue-script.mjs src/components/KhunterPanel.vue
pnpm exec vue-tsc --noEmit
pnpm run build
```

Expected: 全部 PASS；页面无类型错误，生产构建成功，候选不再被文案暗示为买入结论。

- [ ] **Step 5: 提交本任务（仅在用户要求后）**

```bash
git add frontend/src/api/khunter.ts frontend/src/components/KhunterPanel.vue frontend/src/components/khunter frontend/tests/khunter-presentation.test.ts frontend/wailsjs
git commit -m "feat(khunter): separate observations confirmations and diagnostics"
```

---

## Final Verification

- [ ] **运行后端全量测试和构建**

```bash
gofmt -w backend/data/khunter/*.go backend/data/khunter/strategy/*.go backend/data/trading/*.go backend/models/khunter_models.go backend/models/kline_models.go backend/cmd/khunterreplay/*.go
go test ./backend/data/khunter/... ./backend/data/backtest/... ./backend/data/datasource/... ./backend/data/trading/... ./backend/handler/... ./backend/agent/...
go test -race ./backend/data/khunter/... ./backend/data/backtest/...
go test ./...
go build ./...
```

Expected: 全部 PASS；若失败必须保留具体失败日志并修复，不把环境问题当作功能通过。

- [ ] **运行前端完整验证**

```bash
wails generate module
cd frontend
node --import ./tests/register-ts-ext.mjs --test tests/khunter-presentation.test.ts
node tests/check-vue-script.mjs src/components/KhunterPanel.vue
pnpm exec vue-tsc --noEmit
pnpm run build
```

Expected: 全部 PASS。

- [ ] **在安装版数据库副本上运行四组回测**

先停止正在写入安装版数据库的应用，再复制数据库及 WAL/SHM 到临时目录；不得直接迁移安装版原库：

```bash
CASE_DB='/d/_installers/go-stock/data/stock.db'
REPLAY_DIR="$(mktemp -d)"
cp "$CASE_DB" "$REPLAY_DIR/stock.db"
if [ -f "$CASE_DB-wal" ]; then cp "$CASE_DB-wal" "$REPLAY_DIR/stock.db-wal"; fi
if [ -f "$CASE_DB-shm" ]; then cp "$CASE_DB-shm" "$REPLAY_DIR/stock.db-shm"; fi

go run ./backend/cmd/khunterreplay \
  -db "$REPLAY_DIR/stock.db" \
  -start 2026-01-05 -end 2026-11-17 -all-market \
  -variants baseline,data_only,risk,full \
  -config config/khunter_risk_control.json \
  -out "$REPLAY_DIR/four-variants.json"
```

人工核对四组输入、成本和日期一致；旧历史缺快照时显示近似或不可回测；失败样本计数守恒；样本外不足 100 笔不替换默认策略。

- [ ] **运行五股案例回放**

```bash
go run ./backend/cmd/khunterreplay \
  -db "$REPLAY_DIR/stock.db" \
  -start 2026-09-01 -end 2026-11-17 \
  -variants full -config config/khunter_risk_control.json \
  -cases '688322.SH:2026-11-11,300780.SZ:2026-10-29,600259.SH:2026-11-03,300829.SZ:2026-11-12,300969.SZ:2026-10-29' \
  -out "$REPLAY_DIR/five-cases.json"
```

Expected: 五例都有可解释轨迹、日期顺序正确、状态机合法、信号未使用未来数据；不要求特定股票必须盈利或被过滤。

- [ ] **样本外验收**

只有样本外最差 10% 交易平均收益优于基线、组合最大回撤小于基线、确认数量不接近零、至少 100 笔、至少两个市场阶段方向一致且全部测试通过时，才允许把完整方案设为默认。否则保留旧默认策略并输出样本不足或结论不稳定。
