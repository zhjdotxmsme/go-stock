# VIP 功能设计：AI 推荐回测 + 本地 SQLite 数据缓存 + A 股历史数据初始化

> 目标：让 go-stock 的 VIP 用户可以在不依赖付费服务的前提下，对 AI 推荐股票进行本地回测验证、把查询到的行情数据持久化到 SQLite，并自助初始化/增量更新 A 股历史日线数据。
> 
> 日期：2026-06-25
> 修订：2026-06-25（基于外部 7 个开源项目研究与当前代码库现状细化）

---

## 1. 背景与现状

go-stock 已经完成：

- 7 位并行分析师（基本面 / 技术面 / 情绪 / 新闻 / 政策 / 游资 / 解禁）
- 双 LLM 分层路由（quick / deep）
- 免费数据源链：mootdx（TDX TCP）、腾讯财经 HTTP、东方财富 F10、同花顺基本面、百度板块
- `datasource` 包：统一 Provider 接口 + 优先级回退 + L1/L2 两级缓存（freecache + SQLite `datasource_cache`）
- `AiRecommendStocks` 模型：存储 AI 推荐记录（含评级、推荐理由、建议买卖价/止盈止损价）
- `AllStockInfo` 模型：内置 A 股代码表（SECUCODE / SECURITYCODE / SECURITYNAMEABBR）
- `CronTask` 模型：可复用的定时任务框架
- stock-sdk MCP server 已注册，用于技术指标计算

当前缺口：

1. **没有回测能力**：无法验证某条 AI 推荐在 N 天后是否盈利。
2. **缓存以 TTL 为主**：`datasource_cache` 是通用 KV，不适合按日期/股票索引的海量 K 线。
3. **没有历史数据初始化入口**：新用户首次使用或重装后，无法离线批量补齐 A 股历史日线。

---

## 2. 竞品/开源项目研究摘要

对 7 个相关开源项目做了 README 级调研：

| 项目 | 核心定位 | 与 go-stock 的相关点 |
|---|---|---|
| [OpenStock](https://github.com/Open-Dev-Society/OpenStock) | Next.js 全球行情看板 | MongoDB + Finnhub/TradingView，偏 UI，无 A 股特化 |
| [stock-sdk](https://github.com/chengzuopeng/stock-sdk) | 前端 JS 股票数据 SDK | 已作为 MCP 接入 go-stock；提供 `backtest({klines, strategy})` 极简 API，返回 totalReturn / winRate / maxDrawdown |
| [daily_stock_analysis](https://github.com/ZhuLinsen/daily_stock_analysis) | LLM 多市场日报 + Web 工作台 | 数据源极丰富（AkShare/Tushare/Pytdx/Baostock/YFinance），含回测、持仓、定时任务 |
| [TradingAgents](https://github.com/TauricResearch/TradingAgents) | 多 Agent 金融交易框架 | 4 分析师 + Bull/Bear 辩论 + Trader/Risk/PM；LangGraph；SQLite checkpoint；决策日志与反思 |
| [TradingAgents-CN](https://github.com/hsliuping/TradingAgents-CN) | 中文增强版 TradingAgents | FastAPI + Vue；MongoDB + Redis；Tushare/AkShare/BaoStock；多级缓存；模拟交易 |
| [TradingAgents-astock](https://github.com/simonlin1212/TradingAgents-astock) | A 股深度特化版 | **7 分析师（+政策/游资/解禁）**、免费数据源（mootdx/腾讯/东财/新浪/同花顺/百度）、A 股交易规则、SQLite checkpoint、CSI 300 基准 |
| [QuantDinger](https://github.com/brokermr810/QuantDinger) | 全栈量化平台 | Python Flask + PostgreSQL + Redis；IndicatorStrategy/ScriptStrategy；实盘/模拟交易；Agent Gateway + MCP |

### 2.1 可借鉴的通用模式

1. **回测最小闭环**：stock-sdk 的 `backtest({klines, strategy})` 证明回测 API 可以极简单：输入 K 线 + 信号函数，输出收益/胜率/最大回撤。
2. **多 Agent A 股化**：TradingAgents-astock 验证了“政策 / 游资 / 解禁”三位 A 股特化分析师的价值，与 go-stock 最新 7 分析师设计高度一致。
3. **数据分层**：TradingAgents-CN 使用 MongoDB/Redis/文件三级缓存；go-stock 已有 SQLite，应把“持久化 K 线”落到 SQLite 而非再引入 Redis/MongoDB。
4. **免费数据源组合**：mootdx（不封 IP）+ 腾讯（实时）+ 东财（龙虎榜/解禁/板块）是 A 股自托管的最佳免费组合，go-stock 已具备。
5. **A 股交易约束**：T+1、涨跌停、手数、ST，回测必须考虑；沪深 300 是合适的 A 股基准。
6. **持久化与恢复**：TradingAgents 使用 SQLite checkpoint 做断点续跑，go-stock 可用同样思路做“大规模历史数据初始化断点续传”。
7. **多 Agent 信号加权**：TradingAgents 在 debate 后会给出 Bull/Bear 权重， trader 再根据权重生成仓位。go-stock 回测可复用 7 位分析师的评级分布，给不同推荐赋权。

### 2.2 明确不照搬的地方

- **不引入 Python 运行时**：go-stock 是 Go + Wails，回测必须在 Go 中实现。
- **不引入 PostgreSQL/Redis/MongoDB**：继续复用现有 SQLite（WAL 模式），保持绿色版免部署。
- **不做实盘交易接口**：回测仅用于验证 AI 推荐，不涉及券商接口。

---

## 3. 设计目标

1. **AI 推荐回测**：对单条/批量 `AiRecommendStocks` 记录，模拟买入并持有 N 个交易日，计算收益率、胜率、最大回撤、相对沪深 300 超额收益。
2. **本地 SQLite K 线缓存**：把从免费源拉取的日线按 `(code, date, period)` 持久化，支持回测离线运行与重复查询零成本。
3. **A 股历史数据初始化**：首次启动或手动触发时，批量下载全部 A 股日线历史；支持断点续传、增量更新、限速防封。

---

## 4. 推荐方案：三层架构

```
┌─────────────────────────────────────────────────────────────┐
│  表现层 / Wails API                                          │
│  - 单条回测 / 批量回测                                       │
│  - 历史数据初始化进度                                        │
│  - 缓存命中率统计                                            │
└─────────────────────────────────────────────────────────────┘
                               │
┌─────────────────────────────────────────────────────────────┐
│  业务层                                                      │
│  backend/data/backtest/       回测引擎（策略、仓位、结算）   │
│  backend/data/history/        历史数据初始化任务             │
│  backend/data/datasource/     现有 Provider 路由 + 缓存      │
│  backend/agent/multi/         7 分析师信号 → 评级权重       │
└─────────────────────────────────────────────────────────────┘
                               │
┌─────────────────────────────────────────────────────────────┐
│  数据层                                                      │
│  SQLite (GORM)                                               │
│  - kline_bars          持久化 K 线                           │
│  - kline_sync_log      每只股票的同步状态/进度               │
│  - ai_recommend_backtests  回测结果                          │
│  - datasource_cache    现有通用缓存                          │
└─────────────────────────────────────────────────────────────┘
```

---

## 5. 数据模型

### 5.1 K 线持久化表

复用现有 `datasource.KLineData` / `datasource.KLineBar` 结构，新增 GORM 模型做持久化。注意与内存/接口类型的区分：

```go
type KLineBar struct {
    ID        uint      `gorm:"primarykey"`
    StockCode string    `gorm:"index:idx_kline_code_period_date,unique;size:20"`
    Period    string    `gorm:"index:idx_kline_code_period_date,unique;size:10"` // day/week/month
    TradeDate string    `gorm:"index:idx_kline_code_period_date,unique;size:10"` // YYYY-MM-DD
    Open      float64
    Close     float64
    High      float64
    Low       float64
    Volume    int64
    Amount    float64
    Adjusted  bool      // 是否复权
    Source    string    `gorm:"size:20"` // tdx / tencent / eastmoney
    CreatedAt time.Time
    UpdatedAt time.Time
}

func (KLineBar) TableName() string { return "kline_bars" }
```

> 说明：
> - 复权数据单独存或加 `Adjusted` 字段；推荐用 `qfq` 前复权做回测，避免分红送股导致的跳变。
> - 唯一索引 `(code, period, trade_date, adjusted)` 防止重复。
> - 与 `datasource.KLineBar`（接口内存类型，含 `Time time.Time`）命名冲突，建议持久化模型命名为 `models.KLineBar` 或 `models.PersistentKLineBar`。

### 5.2 同步进度表

```go
type KLineSyncLog struct {
    ID            uint      `gorm:"primarykey"`
    StockCode     string    `gorm:"index;size:20"`
    Period        string    `gorm:"size:10"`
    Adjusted      bool
    StartDate     string    `gorm:"size:10"`
    EndDate       string    `gorm:"size:10"`
    SyncedCount   int
    ExpectedCount int
    Status        string    `gorm:"size:20"` // pending / running / done / failed
    ErrorMsg      string    `gorm:"type:text"`
    UpdatedAt     time.Time
}

func (KLineSyncLog) TableName() string { return "kline_sync_log" }
```

### 5.3 AI 推荐回测结果表

```go
type AiRecommendBacktest struct {
    gorm.Model
    AiRecommendID uint      `gorm:"index"`
    StockCode     string    `gorm:"index;size:20"`
    StockName     string    `gorm:"size:50"`
    SignalDate    string    `gorm:"index;size:10"` // 推荐日期
    SignalRating  string    `gorm:"size:10"`       // buy / hold / sell
    EntryPrice    float64   // 买入价（推荐日收盘价或建议买入价中点）
    ExitPrice     float64   // 卖出价
    ExitDate      string    `gorm:"size:10"`
    HoldingDays   int       // 实际持有交易日
    TotalReturn   float64   // 总收益率（小数）
    MaxDrawdown   float64   // 最大回撤（小数）
    Csi300Return  float64   // 同期沪深 300 收益率
    Alpha         float64   // 超额收益
    Win           bool      // 是否盈利
    Source        string    `gorm:"size:20"` // 数据来源：cached / live
}

func (AiRecommendBacktest) TableName() string { return "ai_recommend_backtests" }
```

---

## 6. 回测引擎设计

### 6.1 输入

```go
type BacktestInput struct {
    StockCode    string
    SignalDate   string          // YYYY-MM-DD，推荐产生日
    SignalRating string          // buy / hold / sell
    EntryPrice   float64         // 0 表示用 signalDate 收盘价
    HoldingDays  int             // 默认 5、10、20、60
    StopLoss     float64         // 可选止损比例，如 0.05
    StopProfit   float64         // 可选止盈比例，如 0.10
    Adjusted     bool            // 是否使用复权数据
    Benchmark    string          // 默认 "000300" 沪深 300
}
```

### 6.2 输出

```go
type BacktestResult struct {
    StockCode       string
    SignalDate      string
    EntryPrice      float64
    ExitPrice       float64
    ExitDate        string
    HoldingDays     int
    TotalReturn     float64
    MaxDrawdown     float64
    BenchmarkReturn float64
    Alpha           float64
    Win             bool
    Trades          []TradeRecord
    EquityCurve     []EquityPoint
}
```

### 6.3 核心规则（A 股特化）

1. **T+1 结算**：signalDate 当天买入，最早 signalDate+1 开盘才能卖出。回测中简化为“signalDate 收盘价买入，持有 N 个交易日后收盘价卖出”。
2. **涨跌停过滤**：若买入日涨停或卖出日跌停，标记为 `SlippageWarning`，但仍在结果中保留理论收益。
3. **复权默认前复权**：使用 `qfq` 数据，保证分红送股后收益连续。TDX 的 `types.AdjustQFQ` 已直接返回前复权数据。
4. **沪深 300 基准**：同期买入并持有沪深 300 ETF（如 510300）N 个交易日，计算 alpha。
5. **止盈止损**：可选在盘中触及止损/止盈价时提前离场；若无触发，按 N 日收盘价离场。

### 6.4 批量回测

- 对 `ai_recommend_stocks` 表按日期范围筛选。
- 对每条推荐异步跑回测，结果写入 `ai_recommend_backtests`。
- 聚合统计：总推荐数、胜率、平均收益、平均最大回撤、按评级/模型分组表现。

### 6.5 多分析师信号加权（借鉴 TradingAgents）

7 位分析师输出 `Rating ∈ {buy, hold, sell}` 或倾向性文本。回测聚合时可将评级量化为权重：

```go
// 示例：bullish=+1, neutral=0, bearish=-1；7 位分析师加权后得到 [-7, +7] 的信号强度
signalStrength := sum(analystWeights)
```

在批量回测报告中可按 `signalStrength` 分组：
- 强看多（≥5）推荐的表现
- 中性（-2 ~ +2）推荐的表现
- 强看空（≤-5）推荐的表现

这样可验证“7 位分析师一致看多”是否真的优于单一分析师推荐。

---

## 7. 历史数据初始化设计

### 7.1 数据范围

- **A 股全市场**：使用现有 `models.AllStockInfo` 表或 `build/stock_basic.json` 中的代码列表。
- **日线前复权**：至少拉取最近 5 年数据，支持用户配置年限。
- **沪深 300**：作为基准必须初始化（代码 000300.SH / 510300.SH）。

### 7.2 公开可下载历史数据源的结论

经过调研，**目前不存在稳定、完整、免费、可直接下载全量 A 股历史日线 CSV/Parquet 的公开源**。具体结论：

| 来源 | 是否可直接下载 | 说明 |
|---|---|---|
| GitHub 开源项目（如 facecat-kronos 等） | 否 | 多使用自研行情服务器、AKShare/Baostock/Tushare 现拉 |
| AKShare / Baostock | 否 | 免费 API，可批量拉取，但本质仍是接口调用 |
| Tushare | 否 | 免费额度有限，大量数据需要积分/付费 |
| mootdx / TDX | 否 | 通过 TCP 协议实时拉取，不封 IP，适合增量 |
| 聚宽 / 米筐 / 果仁等量化平台 | 否 | 数据完整，但通常不允许导出大量原始 K 线 |

因此，go-stock 应采取 **"一次性本地种子 + 后续增量更新"** 策略：
- 首次由用户在有 Python 的环境运行种子脚本，批量拉取全市场历史数据并打包。
- 后续 go-stock 仅通过 mootdx/腾讯/东财补最新数据，避免全量走 API 被封 IP。

### 7.3 数据源优先级（增量更新阶段）

| 优先级 | 来源 | 用途 | 备注 |
|---|---|---|---|
| 1 | mootdx/TDX | 日线前复权 | TCP，不封 IP，量大首选，适合日常增量 |
| 2 | 腾讯财经 | 日线前复权 | HTTP，备用 |
| 3 | 东方财富 | 日线前复权 | HTTP，封 IP 风险，限速使用 |

> 参考 TradingAgents-astock 的节流策略：东财请求统一走串行 ≥1s + 随机抖动；mootdx/腾讯不受限。

### 7.4 本地种子生成方案（推荐）

由于 go-stock 不引入 Python 运行时，提供一组 **独立的 Python 脚本**（放在 `scripts/history_seed/`），用于在本地一次性生成全量 A 股历史日线种子：

```
scripts/history_seed/
├── baostock_seed.py     # 推荐：用 Baostock 拉 A 股日线，输出 csv/parquet/sqlite
├── akshare_seed.py      # 备选：用 AKShare 拉日线，字段更丰富但接口偶有变动
└── README.md            # 使用说明与字段映射
```

**推荐 Baostock 的理由：**
- 完全免费，无需注册 API Key
- 支持前复权/后复权/不复权
- 接口稳定，批量拉取限流较宽松
- 支持 `query_history_k_data_plus` 按日期段拉取

**种子脚本输出格式（CSV 示例）：**

```csv
date,code,open,high,low,close,preclose,volume,amount,pctChg
2020-01-02,sh600519,1120.00,1130.00,1115.00,1125.00,1110.00,10000,11250000.00,1.35
```

**使用流程：**
1. 用户安装 Python 依赖：`pip install baostock pandas`（或 AKShare）。
2. 运行 `python scripts/history_seed/baostock_seed.py --start 20190101 --output ./history_seed/`。
3. 脚本自动拉取全部 A 股日线（默认前复权），按股票代码保存为 CSV 或合并为单个 SQLite/Parquet。
4. go-stock 提供 **"导入本地历史数据"** 功能：选择种子目录，批量 upsert 到 `kline_bars`。
5. 后续日常增量完全走 mootdx/腾讯/东财，不再依赖 Python。

**种子共享：**
- 由于种子文件较大（5000+ 只股票 × 5 年 ≈ 数百 MB 到 1GB），不随安装包分发。
- 可由社区或项目 Release 中提供打包好的 SQLite/Parquet 种子供用户下载。

### 7.5 任务流程

```go
func RunHistoricalSync(ctx context.Context, cfg SyncConfig) error {
    codes := loadAllAShareCodes() // 从 all_stock_info 读取
    for _, code := range codes {
        // 1. 检查 sync_log，跳过已完成的
        // 2. 标记 running
        // 3. 按优先级拉取日线（mootdx → 腾讯 → 东财）
        // 4. 写入 kline_bars（upsert）
        // 5. 标记 done
        // 6. 每批次 sleep，避免触发东财风控
    }
}
```

### 7.6 断点续传

- 按 `(stock_code, period, adjusted)` 粒度记录 `KLineSyncLog`。
- 任务中断后重启，跳过 `status = done` 的记录。
- 对 `failed` 记录支持单独重试。

### 7.7 增量更新

- 每天收盘后/启动时，对每只股票补拉到最新交易日。
- 通过 `max(trade_date)` 确定缺失区间，避免全量重拉。
- 可复用 `CronTask` 框架创建定时任务：`0 0 17 * * 1-5`（收盘后）。

---

## 8. 缓存策略

### 8.1 现有缓存 `datasource_cache`

继续用于：实时行情、F10 财务、新闻、板块等**非时间序列**数据。

### 8.2 新增 `kline_bars`

用于：日线/周线/月线等**时间序列**数据。

- **读路径**：
  1. 按 `(code, period, start, end, adjusted)` 查询 `kline_bars`。
  2. 若完整命中，直接返回。
  3. 若部分缺失，计算缺失区间，走数据源拉取并回填。
- **写路径**：
  - 批量 upsert，`ON CONFLICT(code, period, trade_date, adjusted) DO UPDATE`。
  - GORM 可用 `Save()` 或原生 SQLite upsert。

### 8.3 缓存失效

- 当日数据：每天收盘后自动失效并补拉。
- 历史数据：默认长期有效，仅在数据源格式升级或复权因子更新时手动重建。

---

## 9. API 设计（Wails / 内部）

### 9.1 回测 API

```go
// 单条回测
BacktestRecommend(ctx context.Context, recommendID uint, holdingDays int) (*BacktestResult, error)

// 批量回测（按日期范围）
BacktestRecommendBatch(ctx context.Context, req BacktestBatchRequest) (*BacktestBatchResult, error)

// 查询回测结果
ListBacktestResults(ctx context.Context, query BacktestResultQuery) (*BacktestResultPageData, error)
```

### 9.2 历史数据 API

```go
// 启动全量初始化（异步）
StartHistoricalSync(ctx context.Context, cfg SyncConfig) error

// 查询进度
GetSyncProgress(ctx context.Context) (*SyncProgress, error)

// 单只股票补数据
SyncSingleStock(ctx context.Context, code string, period string, years int) error

// 导入本地种子（AKShare/Baostock 生成的 CSV/Parquet）
ImportLocalSeed(ctx context.Context, filePath string) error
```

### 9.3 缓存统计 API

```go
GetKLineCacheStats(ctx context.Context) (*KLineCacheStats, error)
```

---

## 10. 实现计划

### Phase 1：数据层（1-2 天）

1. 在 `backend/models/models.go` 新增 `KLineBar`、`KLineSyncLog`、`AiRecommendBacktest`。
2. 在 `main.go` 的 `AutoMigrate()` 中注册新表（注意：当前 `AutoMigrate` 在 `main.go`，不在 `db/db.go`）。
3. 在 `backend/data/datasource/` 下新增 `kline_store.go`：
   - `QueryKLines(ctx, code, period, start, end, adjusted)`
   - `UpsertKLines(ctx, bars)`
   - `GetLatestTradeDate(ctx, code, period, adjusted)`

### Phase 2：历史数据初始化（2-3 天）

1. 新增 `backend/data/history/sync.go`：
   - 读取 A 股代码列表（优先 `models.AllStockInfo`）
   - 断点续传逻辑
   - 限速控制
2. 包装现有 Provider：
   - `MootdxKLineProvider` 拉前复权日线
   - `TencentKLineProvider` 备用
3. 新增 `scripts/history_seed/baostock_seed.py` 与 `akshare_seed.py`（可选），输出 CSV/SQLite/Parquet 种子。
4. 新增 Wails 绑定方法暴露进度查询与本地种子导入。

### Phase 3：回测引擎（2-3 天）

1. 新增 `backend/data/backtest/engine.go`：
   - `Run(ctx, input)`
   - A 股 T+1、涨跌停、止盈止损逻辑
   - 沪深 300 基准计算
2. 新增 `backend/data/backtest/batch.go`：批量回测与聚合（含多分析师信号强度分组）。
3. 新增服务层 `backend/data/backtest_service.go` 对接 Wails。

### Phase 4：前端与测试（2-3 天）

1. 前端新增“回测验证”页面：
   - 单条推荐回测
   - 批量回测筛选
   - 回测统计卡片（含信号强度分组）
2. 前端新增“数据管理”页面：
   - 历史数据初始化按钮与进度条
   - 本地种子导入
   - 缓存统计
3. 单元测试覆盖回测引擎核心计算与 K 线存储 upsert。
4. `go vet ./backend/...` 与 `go build ./backend/...` 通过。

---

## 11. 两种备选方案对比

| 维度 | 推荐方案：SQLite 本地持久化 | 备选方案：文件级 Parquet |
|---|---|---|
| 依赖 | 无新增，复用 GORM + SQLite | 需引入 parquet 库 |
| 查询 | SQL 按日期范围极快 | 需自己实现过滤/索引 |
| 容量 | 5000 只股票 × 5 年 ≈ 500 万条，SQLite 轻松承受 | 文件体积小，但管理复杂 |
| 增量更新 | upsert 天然支持 | 需重写文件 |
| 与现有架构 | 与 `datasource_cache`、`AiRecommendStocks` 同库 | 需要额外同步机制 |
| 推荐度 | ✅ 首选 | 适合未来导出/分析场景 |

---

## 12. 关键风险与缓解

| 风险 | 影响 | 缓解措施 |
|---|---|---|
| 东财/腾讯接口变更 | 数据拉取失败 | 多源回退（mootdx → 腾讯 → 东财） |
| 东财封 IP | 初始化中断 | mootdx 为主；东财限速 ≥1s + 抖动 |
| SQLite 写入慢 | 5000 只股票初始化耗时 | WAL 模式 + 批量 upsert（每批 1000 条）+ 异步任务 |
| 前复权数据漂移 | 历史收益计算偏差 | 定期全量重拉复权因子；记录 source 字段 |
| 回测过拟合 | 用户盲目相信历史收益 | 页面明确标注“历史收益不代表未来；仅供研究” |
| 数据量大导致安装包膨胀 | 绿色版体积变大 | 历史数据不在安装包内，用户通过种子脚本或社区 Release 获取 |
| 种子脚本需 Python 环境 | 部分用户无法生成种子 | 种子脚本可选；提供打包好的 SQLite/Parquet 种子下载；日常增量完全走 Go 数据源 |

---

## 13. 与近期已完成功能的关系

- **7 分析师扩展**：政策 / 游资 / 解禁三位新分析师产出的报告，是回测“信号来源”的重要输入；回测将验证这些信号的实际表现，并可按信号强度分组。
- **免费数据源链**：mootdx、腾讯、东财、10jqka、百度 Provider 已注册，可直接用于历史数据初始化和回测数据供给。
- **datasource 缓存层**：新增 `kline_bars` 是对现有 `datasource_cache` 的时间序列补充，两者共存。
- **stock-sdk MCP**：回测引擎可调用 stock-sdk 的指标函数做更复杂的策略信号，但核心收益计算保持纯 Go。
- **CronTask**：可用于配置收盘后自动增量更新 K 线。

---

## 14. 下一步

本设计文档待确认后，进入 Phase 1 实现：

1. 新增 3 个 GORM 模型。
2. 在 `main.go` 的 `AutoMigrate()` 注册新表。
3. 实现 `kline_store.go` 的查询与 upsert。
4. 编写 `backtest/engine.go` 骨架并通过单元测试。

如需调整（例如优先做历史数据初始化而非回测，或改用 Parquet），请在此文档基础上修改。
