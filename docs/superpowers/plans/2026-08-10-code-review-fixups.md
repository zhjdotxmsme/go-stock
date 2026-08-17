# Code Review Fixups Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复近两周代码审查发现的安全/架构/缺陷问题，补齐完整度缺口。

**Architecture:** 按风险分 4 个子计划：A 安全护栏（最高优先级，防崩防泄漏）→ B 架构统一（双Router合并+占位补齐）→ C 缺陷修复（边界bug）→ D 完整度补齐（测试+一致性+死代码）。每个子计划内 TDD 推进。

**Tech Stack:** Go 1.23, GORM, SQLite, Wails, Vue 3 + TypeScript

---

## 子计划 A：多 Agent 安全护栏（最高优先级）

### Task A1: engine.Run goroutine panic 防护

**Files:**
- Modify: `backend/agent/multi/engine.go:47-56`
- Test: `backend/agent/multi/engine_panic_test.go` (新建)

- [ ] **Step 1: 写测试 — panic 不崩溃进程且 channel 正常关闭**

```go
package multi

import (
	"context"
	"testing"
)

func TestRunPanicDoesNotCrash(t *testing.T) {
	// 用 custom analyst hook 触发 panic（待实现注入点）
	// 此处先用简化验证：channel 被关闭且不 panic
	e := NewMultiAgentEngine(0)
	ch := e.Run(context.Background(), "600000", "测试", "sh", "价格", "")
	// 消费所有消息直到 channel 关闭
	count := 0
	for range ch {
		count++
	}
	if count == 0 {
		t.Error("expected at least some messages on channel")
	}
}
```

- [ ] **Step 2: 运行测试验证当前可通过（simple query 路径不 panic）**

运行: `cd backend && go test ./agent/multi/ -run TestRunPanicDoesNotCrash -v`
Expected: PASS（simple query 路径）

- [ ] **Step 3: 在 engine.go Run goroutine 顶部加 recover**

在 `go func() {` 下方 `defer close(ch)` 之后加：

```go
defer func() {
    if r := recover(); r != nil {
        logger.SugaredLogger.Errorf("multi-agent engine panic recovered: %v", r)
        // 发送错误事件，前端能感知
        emitEvent(ch, "agent:phase", map[string]string{
            "phase": "error", "status": "error",
            "label": "分析引擎异常，已恢复",
        })
    }
}()
```

位置：`defer close(ch)` 之后（recover 必须在外层 defer，close 在内层，保证 recover 后仍能 close）。

- [ ] **Step 4: 调整 defer 顺序**

将 `defer close(ch)` 移到 recover defer 之后（先声明的 defer 后执行，要保证 close 在 recover 之前执行完）：

```go
go func() {
    defer func() {
        if r := recover(); r != nil {
            logger.SugaredLogger.Errorf("multi-agent engine panic recovered: %v", r)
            emitEvent(ch, "agent:phase", map[string]string{
                "phase": "error", "status": "error",
                "label": "分析引擎异常，已恢复",
            })
        }
    }()
    defer close(ch)
    // ... 原有代码
```

- [ ] **Step 5: 运行测试验证通过**

运行: `cd backend && go test ./agent/multi/ -run TestRunPanicDoesNotCrash -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add backend/agent/multi/engine.go backend/agent/multi/engine_panic_test.go
git commit -m "fix(multi-agent): add panic recovery to Run goroutine"
```

### Task A2: channel 发送防阻塞（ctx 取消时不泄漏 goroutine）

**Files:**
- Modify: `backend/agent/multi/engine.go:230-262` (emitEvent, emitFinalReport)
- Modify: `backend/agent/multi/engine.go` (所有调用 emitEvent/emitFinalReport 的地方)

- [ ] **Step 1: 给 emitEvent 和 emitFinalReport 加 ctx 参数**

```go
func emitEvent(ctx context.Context, ch chan<- *schema.Message, eventType string, data map[string]string) {
    // ... marshal ...
    select {
    case ch <- &schema.Message{Role: schema.Assistant, Content: string(raw)}:
    case <-ctx.Done():
    }
}

func emitFinalReport(ctx context.Context, ch chan<- *schema.Message, report *FinalReport) {
    // ... marshal ...
    select {
    case ch <- &schema.Message{Role: schema.Assistant, Content: string(raw)}:
    case <-ctx.Done():
    }
}
```

- [ ] **Step 2: 更新所有调用点，传入 ctx**

engine.go 中所有 `emitEvent(ch, ...)` → `emitEvent(ctx, ch, ...)`
所有 `emitFinalReport(ch, ...)` → `emitFinalReport(ctx, ch, ...)`

runParallelAnalysts 不改（它用内部 buffered chan，大小=7，发送完即关，不会阻塞）。

- [ ] **Step 3: runModePipeline 和 runSimpleQuery 同步更新**

检查 engine_mode.go 中是否也调用了 emitEvent/emitFinalReport，同步加 ctx。

- [ ] **Step 4: 编译验证**

运行: `go build ./...`
Expected: 无编译错误

- [ ] **Step 5: Commit**

```bash
git add backend/agent/multi/engine.go
git commit -m "fix(multi-agent): prevent goroutine leak on channel send with ctx select"
```

### Task A3: 记忆注入避免并发 AutoMigrate

**Files:**
- Modify: `backend/agent/memory/sqlite_memory.go:40-47`
- Modify: `backend/agent/multi/memory_inject.go:46-69`
- Test: `backend/agent/memory/sqlite_memory_concurrent_test.go` (新建)

- [ ] **Step 1: 写测试 — 并发 NewSQLiteMemory 不报错**

```go
package memory

import (
	"sync"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestConcurrentNewSQLiteMemory(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(role string) {
			defer wg.Done()
			_, err := NewSQLiteMemory(db, role)
			if err != nil {
				t.Errorf("NewSQLiteMemory(%s) error: %v", role, err)
			}
		}(string(rune('a' + i)))
	}
	wg.Wait()
}
```

- [ ] **Step 2: 运行测试看是否报错（SQLite busy / table lock）**

运行: `cd backend && go test ./agent/memory/ -run TestConcurrentNewSQLiteMemory -v -race`
Expected: 可能 FAIL（并发 DDL 冲突）

- [ ] **Step 3: 在 NewSQLiteMemory 中加 sync.Once 做一次性迁移**

```go
var migrateOnce sync.Once
var migrateErr error

func NewSQLiteMemory(db *gorm.DB, agentRole string) (*SQLiteMemory, error) {
    migrateOnce.Do(func() {
        migrateErr = db.AutoMigrate(&memoryRow{})
        if migrateErr == nil {
            // 同时建 FTS 表（只建一次）
            create := fmt.Sprintf(`CREATE VIRTUAL TABLE IF NOT EXISTS %s USING fts5(situation_text, lesson_text)`, ftsTable)
            if err := db.Exec(create).Error; err != nil {
                // FTS 不可用不阻断，probeFTS5 会探测
            }
        }
    })
    if migrateErr != nil {
        return nil, fmt.Errorf("记忆表迁移失败: %w", migrateErr)
    }
    m := &SQLiteMemory{db: db, role: agentRole}
    m.fts = m.probeFTS5()
    return m, nil
}
```

注意：sync.Once 是包级单例，不同 db 实例会出问题。但本项目只有一个 db.Dao，所以 OK。如果测试用多个内存 db，每个 db 需要自己的 once。改成 db 级 once：

```go
var migrateOnce sync.Map // map[*gorm.DB]*sync.Once

func NewSQLiteMemory(db *gorm.DB, agentRole string) (*SQLiteMemory, error) {
    once, _ := migrateOnce.LoadOrStore(db, &sync.Once{})
    once.(*sync.Once).Do(func() {
        // ... 迁移
    })
    // ...
}
```

- [ ] **Step 4: 同步更新 probeFTS5 逻辑**

probeFTS5 中的 `CREATE VIRTUAL TABLE IF NOT EXISTS` 可以保留（幂等），但 AutoMigrate 移到 once 内。

- [ ] **Step 5: 运行测试验证通过**

运行: `cd backend && go test ./agent/memory/ -run TestConcurrentNewSQLiteMemory -v -race`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add backend/agent/memory/sqlite_memory.go backend/agent/memory/sqlite_memory_concurrent_test.go
git commit -m "fix(memory): prevent concurrent AutoMigrate with per-db sync.Once"
```

### Task A4: risk_debate Engine.Run 防 nil DebateCall

**Files:**
- Modify: `backend/agent/multi/risk_debate/risk_debate.go:51-66`
- Test: 已有 `risk_debate_test.go`，加一个用例

- [ ] **Step 1: NewEngine 增加 debateCall nil 校验**

```go
func NewEngine(maxRounds int, debateModel, judgeModel string, debateCall, judgeCall LLMCallFunc) *Engine {
    if debateCall == nil {
        panic("risk_debate: NewEngine called with nil debateCall")
    }
    // ... 原有代码
}
```

- [ ] **Step 2: 同时给 Run 加防御性检查（运行时安全网）**

```go
func (e *Engine) Run(ctx context.Context, dc DebateContext) (*RiskDebateResult, error) {
    if e.DebateCall == nil {
        return nil, fmt.Errorf("risk_debate: DebateCall is nil")
    }
    // ... 原有代码
}
```

- [ ] **Step 3: 运行现有测试**

运行: `cd backend && go test ./agent/multi/risk_debate/ -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add backend/agent/multi/risk_debate/risk_debate.go
git commit -m "fix(risk-debate): guard nil DebateCall in NewEngine and Run"
```

---

## 子计划 B：架构统一

### Task B1: 接入 free-stockdb 到 internal 六边形 Router

**Files:**
- Modify: `backend/internal/port/datasource/provider.go`（如果 SectorProvider 不存在，需补）
- Create: `backend/internal/adapter/datasource/freestockdb_adapter.go`
- Modify: `backend/internal/adapter/datasource/composite.go`（注册 freestockdb）

- [ ] **Step 1: 确认 port 接口定义**

读 `backend/internal/port/datasource/provider.go`，确认已有接口（DataSourceProvider / QuoteProvider / KLineProvider / SectorProvider）。若缺 SectorProvider，补：

```go
type SectorProvider interface {
    DataSourceProvider
    GetSectorData(ctx context.Context, code string) (*SectorData, error)
}

type SectorData struct {
    Code   string
    Sector string
}
```

- [ ] **Step 2: 创建 freestockdb adapter**

```go
// Package datasource internal adapter for freestockdb provider.
// Wraps the existing freestockdb.Provider to implement port interfaces.
package datasource

import (
    "context"
    portds "go-stock/backend/internal/port/datasource"
    "go-stock/backend/data/datasource/freestockdb"
)

// FreestockdbAdapter wraps freestockdb.Provider as a port-compliant adapter.
type FreestockdbAdapter struct {
    p *freestockdb.Provider
}

func NewFreestockdbAdapter(p *freestockdb.Provider) *FreestockdbAdapter {
    return &FreestockdbAdapter{p: p}
}

func (a *FreestockdbAdapter) Name() string     { return a.p.Name() }
func (a *FreestockdbAdapter) Priority() int    { return a.p.Priority() }
func (a *FreestockdbAdapter) Available(ctx context.Context) bool { return a.p.Available(ctx) }

func (a *FreestockdbAdapter) GetKLine(ctx context.Context, code, period string, count int) (*portds.KLineData, error) {
    kd, err := a.p.GetKLine(ctx, code, period, count)
    if err != nil {
        return nil, err
    }
    bars := make([]portds.KLineBar, len(kd.Bars))
    for i, b := range kd.Bars {
        bars[i] = portds.KLineBar{
            Time: b.Time, Open: b.Open, High: b.High, Low: b.Low,
            Close: b.Close, PreClose: b.PreClose, Volume: b.Volume, Amount: b.Amount,
        }
    }
    return &portds.KLineData{Code: kd.Code, Period: kd.Period, Bars: bars}, nil
}

func (a *FreestockdbAdapter) GetQuote(ctx context.Context, code string) (*portds.QuoteData, error) {
    q, err := a.p.GetQuote(ctx, code)
    if err != nil {
        return nil, err
    }
    return &portds.QuoteData{
        Code: q.Code, Name: q.Name, Price: q.Price,
        Change: q.Change, ChangePct: q.ChangePct,
        Volume: q.Volume, Amount: q.Amount,
        High: q.High, Low: q.Low, Open: q.Open,
        PrevClose: q.PrevClose, Time: q.Time,
    }, nil
}
```

- [ ] **Step 3: 在 composite.go 中注册 freestockdb**

找到 composite.go（默认装配函数），在 Router Register 中加入 freestockdb adapter，注意它需要 Manager/Provider 依赖，而 Manager 需要 Config。在启动装配时（app_lifecycle.go 或类似位置）从 Settings 读配置后构造并注册。

- [ ] **Step 4: 在 Register 中支持 SectorProvider**

Router.Register 当前只识别 QuoteProvider 和 KLineProvider。如果补了 SectorProvider 接口，同步加注册链：

```go
if sp, ok := p.(portds.SectorProvider); ok {
    r.sectorProviders = append(r.sectorProviders, sp)
}
```

并加 `GetSectorData` 方法（模式同 GetQuote/GetKLine）。

- [ ] **Step 5: 编译验证**

运行: `go build ./...`
Expected: 无编译错误

- [ ] **Step 6: Commit**

```bash
git add backend/internal/adapter/datasource/freestockdb_adapter.go backend/internal/adapter/datasource/router.go backend/internal/adapter/datasource/composite.go
git commit -m "feat(hex): wire freestockdb into internal datasource router"
```

### Task B2: 删除 internal router 死代码 normalizePeriod

**Files:**
- Modify: `backend/internal/adapter/datasource/router.go:124-139`

- [ ] **Step 1: 删除 normalizePeriod 函数**

整段删除（14行）。

- [ ] **Step 2: 编译验证**

运行: `go build ./...`
Expected: 无编译错误

- [ ] **Step 3: Commit**

```bash
git add backend/internal/adapter/datasource/router.go
git commit -m "refactor(hex): remove dead normalizePeriod function in internal router"
```

### Task B3: 补齐 StockRepository 关注股 + 分组实现

**Files:**
- Modify: `backend/internal/adapter/repository/sqlite/stock.go:240-306`
- 参考: `backend/data/` 中关注股和分组的现有实现

- [ ] **Step 1: 找到现有关注股实现**

搜索 data 包中 AddFollow / GetFollowList / SetCostPriceAndVolume 等函数的位置，理解数据模型（用的是 models.FollowedStock？还是自定义表？）。

- [ ] **Step 2: 实现 Followed stocks 5 个方法**

逐个实现：AddFollow, RemoveFollow, GetFollowList, SetCostPriceAndVolume, SetTradingPrice, SetAlarmChangePercent, SetStockSort。
每个方法：从 port 接口入参 → 调用 data 包对应函数或直接 GORM 操作 → 返回 domain 模型。

- [ ] **Step 3: 实现 Groups 5 个方法**

AddGroup, RemoveGroup, GetGroupList, AddStockToGroup, RemoveStockFromGroup。

- [ ] **Step 4: 删除 errNotImplemented 变量和所有 TODO 注释**

- [ ] **Step 5: 编译 + 运行 stock 相关测试**

运行: `go test ./internal/adapter/repository/sqlite/ -v -run Stock`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add backend/internal/adapter/repository/sqlite/stock.go
git commit -m "feat(hex): implement followed-stock and group repository adapters"
```

### Task B4: StockChangeHistory 分页参数校验

**Files:**
- Modify: `backend/internal/adapter/repository/sqlite/stock.go`（GetStockChangeHistory 附近）

- [ ] **Step 1: 找到分页查询方法**

搜索 `offset := (query.Page-1)*query.PageSize`，确认方法名和 query 结构。

- [ ] **Step 2: 加参数校验**

```go
if query.Page <= 0 {
    query.Page = 1
}
if query.PageSize <= 0 {
    query.PageSize = 20
}
if query.PageSize > 500 {
    query.PageSize = 500
}
```

- [ ] **Step 3: 写测试覆盖边界**

在对应测试文件中加 Page=0 / PageSize=0 / PageSize=1000 的用例。

- [ ] **Step 4: 运行测试**

运行: `go test ./internal/adapter/repository/sqlite/ -v -run StockChange`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/adapter/repository/sqlite/stock.go
git commit -m "fix(hex): validate pagination params in StockChangeHistory"
```

---

## 子计划 C：缺陷修复

### Task C1: FactorStore 平行数组长度防御

**Files:**
- Modify: `backend/data/datasource/freestockdb/factor.go:59-104` (Load)
- Modify: `backend/data/datasource/freestockdb/factor.go:156-208` (AdjustBars)
- Test: 已有 `factor_test.go`，加异常输入用例

- [ ] **Step 1: 写测试 — dates/cums 不等长时不 panic**

```go
func TestFactorStoreAdjustBarsUnequalArrays(t *testing.T) {
    fs := NewFactorStore()
    // 手动注入不等长数组（通过 setFactors）
    fs.setFactors("600000",
        []string{"20250101", "20250601", "20251201"},
        []float64{1.5, 1.3}) // 故意短一个
    bars := []Bar{
        {Date: 20251231, Close: 100, PreClose: 99},
    }
    // 不应 panic，最多返回未复权数据
    result := fs.AdjustBars("600000", bars, FQQFQ)
    if len(result) != len(bars) {
        t.Error("expected same length")
    }
}
```

- [ ] **Step 2: 运行测试验证失败（panic）**

运行: `cd backend && go test ./data/datasource/freestockdb/ -run TestFactorStoreAdjustBarsUnequalArrays -v`
Expected: FAIL / panic

- [ ] **Step 3: 修复 AdjustBars 取 latest 前校验**

```go
if len(dates) != len(cums) {
    logger.SugaredLogger.Warnf("freestockdb: factor dates/cums length mismatch for %s (%d vs %d), skip fq",
        code, len(dates), len(cums))
    return bars
}
latest := cums[len(cums)-1]
```

同时在 factorLE 中也加同样防御。

- [ ] **Step 4: 同时在 Load 中做严格校验**

Load 中 sort 之后，确保每只股票 dates 和 cums 等长。不等长则 skip 整只股票（warn）。

- [ ] **Step 5: 运行测试验证通过**

运行: `cd backend && go test ./data/datasource/freestockdb/ -run TestFactorStore -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add backend/data/datasource/freestockdb/factor.go backend/data/datasource/freestockdb/factor_test.go
git commit -m "fix(freestockdb): guard against mismatched factor dates/cums arrays"
```

### Task C2: risk.go 零值 PE 检查与注释矛盾

**Files:**
- Modify: `backend/agent/strategy/risk/risk.go:34-62` (RiskInput 注释)
- 或: `backend/agent/strategy/risk/risk.go:114-117` (PE 检查逻辑)

- [ ] **Step 1: 决定修复方向**

注释说"零值不触发任何检查"，但 PE<=0 检查没有 hasPE 守卫。正确的修复是：PE 是核心估值字段，零值即"无数据"，应该像 SignalScore 一样加 `HasPE bool` 标记。但这是接口变更，影响面大。

**更安全的修复：改注释，符合现有行为。** PE<=0 检查是合理的（亏损股=风险），注释是错的。

- [ ] **Step 2: 修正注释**

```go
// RiskInput 风控检查输入（纯数据，参照 D1 scoring.FactorInput 模式）。
// 零值结构中，PE<=0 视为亏损股会触发 invalid_pe（这是预期行为）；
// SignalScore / LLMConfidence / KLineQuality 等可选字段零值视为"无数据"，
// 由 HasSignalScore / HasLLMConfidence / HasKLineQuality 分别守卫。
```

- [ ] **Step 3: 运行现有测试**

运行: `cd backend && go test ./agent/strategy/risk/ -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add backend/agent/strategy/risk/risk.go
git commit -m "docs(risk): fix misleading zero-value comment on RiskInput"
```

### Task C3: client.Get 响应体截断检测

**Files:**
- Modify: `backend/data/datasource/freestockdb/client.go:29-48`
- Test: `client_test.go` 加用例

- [ ] **Step 1: 改造 LimitReader 为带截断检测**

```go
const maxBodySize = 64 << 20

lr := io.LimitReader(resp.Body, maxBodySize+1)
body, err := io.ReadAll(lr)
if err != nil {
    return nil, err
}
if int64(len(body)) > maxBodySize {
    return nil, fmt.Errorf("stockdb: response exceeds %d bytes for %q", maxBodySize, expr)
}
```

思路：多读一个字节，如果超限说明被截断，返回错误。

- [ ] **Step 2: 写测试**

用 httptest server 返回超过 64MB 的响应。或者更简单：单元测试中构造一个超过 maxBodySize 的 reader。

- [ ] **Step 3: 运行测试**

运行: `cd backend && go test ./data/datasource/freestockdb/ -run TestClient -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add backend/data/datasource/freestockdb/client.go backend/data/datasource/freestockdb/client_test.go
git commit -m "fix(freestockdb): detect response body truncation instead of silent cutoff"
```

### Task C4: Available 持写锁做 Ping 优化

**Files:**
- Modify: `backend/data/datasource/freestockdb/manager.go:113-126`

- [ ] **Step 1: 将 Available 改为 RWMutex，Ping 在锁外做**

```go
type Manager struct {
    // ...
    mu            sync.RWMutex // 改 RWMutex
    // ...
}

func (m *Manager) Available(ctx context.Context) bool {
    if !m.cfg.Enabled {
        return false
    }
    // 快速路径：读锁检查缓存
    m.mu.RLock()
    if time.Since(m.checkedAt) < m.availableTTL {
        ok := m.ok
        m.mu.RUnlock()
        return ok
    }
    m.mu.RUnlock()
    // 缓存过期：锁外做 Ping（网络 I/O 不持锁）
    ok := m.client.Ping(ctx)
    // 写锁更新状态
    m.mu.Lock()
    m.ok = ok
    m.checkedAt = time.Now()
    m.mu.Unlock()
    return ok
}
```

注意：并发场景下多个 goroutine 同时过期会同时 Ping。这是可接受的（thundering herd，但 30s TTL 内最多一次风暴）。如果要严格单飞，再加一个 CAS 守卫（atomic.Bool）。

- [ ] **Step 2: 同步更新 setOK / takeCmd / Start 中的锁使用**

Start/Stop/setOK/takeCmd 中用的是 `mu.Lock()`，它们修改 cmd/ok/checkedAt，需要写锁。把 `mu` 的类型从 `sync.Mutex` 改为 `sync.RWMutex` 不影响这些写操作。

- [ ] **Step 3: 运行现有测试 + race**

运行: `cd backend && go test ./data/datasource/freestockdb/ -run TestManager -v -race`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add backend/data/datasource/freestockdb/manager.go
git commit -m "perf(freestockdb): avoid holding lock during Ping in Available"
```

### Task C5: provider triggerLazyLoad / Setup goroutine 加 recover

**Files:**
- Modify: `backend/data/datasource/freestockdb/provider.go:65-75` (triggerLazyLoad)
- Modify: `backend/data/datasource/freestockdb/provider.go:188-197` (Setup goroutine)

- [ ] **Step 1: triggerLazyLoad 加 recover**

```go
go func() {
    defer func() {
        if r := recover(); r != nil {
            logger.SugaredLogger.Errorf("freestockdb lazy load panic: %v", r)
        }
        p.loading.Store(false)
    }()
    // ... 原有代码
}()
```

- [ ] **Step 2: Setup 中的 goroutine 也加 recover**

```go
go func() {
    defer func() {
        if r := recover(); r != nil {
            logger.SugaredLogger.Errorf("freestockdb setup panic: %v", r)
        }
    }()
    // ... 原有代码
}()
```

- [ ] **Step 3: 运行测试**

运行: `cd backend && go test ./data/datasource/freestockdb/ -run TestProvider -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add backend/data/datasource/freestockdb/provider.go
git commit -m "fix(freestockdb): add panic recovery to background goroutines"
```

### Task C6: saveMultiAgentResult 错误检查

**Files:**
- Modify: `backend/agent/multi/engine.go:366-378`

- [ ] **Step 1: 检查 db.Dao.Create 返回值**

```go
if err := db.Dao.Create(&models.AIResponseResult{
    StockCode: ac.StockCode,
    StockName: ac.StockName,
    ModelName: "multi-agent-7",
    Content:   combined.String(),
    Question:  ac.UserQuery,
}).Error; err != nil {
    logger.SugaredLogger.Errorf("save multi-agent result failed for %s: %v", ac.StockCode, err)
    return
}
```

- [ ] **Step 2: 编译验证**

运行: `go build ./...`
Expected: 无编译错误

- [ ] **Step 3: Commit**

```bash
git add backend/agent/multi/engine.go
git commit -m "fix(multi-agent): check error when saving multi-agent result"
```

---

## 子计划 D：完整度补齐

### Task D1: ctx 传递一致性（sqlite adapter 中 db.Dao 改用 WithContext）

**Files:**
- Modify: `backend/internal/adapter/repository/sqlite/stock.go`（所有不用 WithContext 的方法）

- [ ] **Step 1: 审计文件，列出所有 db.Dao.XXX 调用但未使用 WithContext(ctx) 的方法**

重点检查：SaveStockChangesToHistory, DeleteStockChangeHistoryBefore, AddTradingRecord 等。

- [ ] **Step 2: 逐个改为 db.Dao.WithContext(ctx).XXX**

注意：方法签名中已经有 ctx 参数但没使用的，全部用上。没有 ctx 的方法（如果有）加 ctx 参数并更新调用点。

- [ ] **Step 3: 编译 + 测试**

运行: `go test ./internal/adapter/repository/sqlite/ -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add backend/internal/adapter/repository/sqlite/stock.go
git commit -m "refactor(hex): consistent WithContext usage in stock repository"
```

### Task D2: 多 Agent engine 单元测试补强

**Files:**
- Create: `backend/agent/multi/engine_test.go`（如果不存在）

- [ ] **Step 1: 写 Run goroutine 关闭测试**

验证：ctx 取消后 channel 最终关闭，无泄漏（用 runtime.NumGoroutine 粗略检测）。

- [ ] **Step 2: 写 panic 恢复测试**

通过注入一个会 panic 的 analyst hook 验证 recover 生效。（如果没有注入点，先加一个测试注入点到 EngineConfig，例如 TestPanicHook func()）。

- [ ] **Step 3: 写 channel 取消测试**

ctx 取消后 emitEvent 不阻塞。

- [ ] **Step 4: 运行测试 + race**

运行: `cd backend && go test ./agent/multi/ -v -race -run TestEngine`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/agent/multi/engine_test.go
git commit -m "test(multi-agent): add engine goroutine lifecycle and panic recovery tests"
```

### Task D3: ranking RankWeight clamp

**Files:**
- Modify: `backend/agent/strategy/ranking/ranker.go`
- Test: `ranker_test.go` 加用例

- [ ] **Step 1: 在 RankerConfig 校验或 normalize 中 clamp RankWeight**

```go
func (cfg RankerConfig) normalize() RankerConfig {
    if cfg.RankWeight < 0 {
        cfg.RankWeight = 0
    }
    if cfg.RankWeight > 1 {
        cfg.RankWeight = 1
    }
    // ... 其他字段
    return cfg
}
```

如果已有 normalize 方法，加进去；没有就创建。

- [ ] **Step 2: 确保 NewRanker / Rank 调用 normalize**

- [ ] **Step 3: 加测试用例（RankWeight=0 / 0.5 / 1.5 / -0.1）**

- [ ] **Step 4: 运行测试**

运行: `cd backend && go test ./agent/strategy/ranking/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/agent/strategy/ranking/ranker.go backend/agent/strategy/ranking/ranker_test.go
git commit -m "fix(ranking): clamp RankWeight to [0, 1] range"
```

### Task D4: filter 死参数清理

**Files:**
- Modify: `backend/agent/strategy/filter/filter.go`

- [ ] **Step 1: 删除 rangeReject 中未使用的 format 参数**

找到 `_ = format` 的行，检查函数签名和调用点。如果 format 参数确实不用，从函数签名和所有调用点删除。

- [ ] **Step 2: 编译 + 测试**

运行: `cd backend && go test ./agent/strategy/filter/ -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add backend/agent/strategy/filter/filter.go
git commit -m "refactor(filter): remove unused format parameter from rangeReject"
```

---

## 执行顺序

必须按字母顺序：A → B → C → D
- A 是安全护栏，必须先上
- B 是架构统一，影响面大但都是新增/补齐
- C 是单个缺陷修复，互相独立
- D 是完整度补齐，最后做

每个 Task 内按步骤 TDD 推进。所有修改必须通过 `go build ./...` 和对应包的单元测试。
