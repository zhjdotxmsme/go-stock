# 数据源接入与 AI 集成架构审查及规划

> 2026-08-27 审查。基于对 `backend/data`、`backend/data/datasource`、`backend/internal`、`backend/agent`、`ai-assistant-web` 的全量代码走读。

---

## 一、现状结论

### 1.1 三层并存的数据通路（迁移半途的真实状态）

```
┌─ 层C internal/ 六边形架构（新）─────────────────────────────┐
│ port/datasource 接口 + adapter/datasource 包装旧实现        │
│ 仅 1 个调用方（trading_handler 实时价）；无缓存、无落盘      │
└──────────────────────────────────────────────────────────┘
┌─ 层B data/datasource（过渡）───────────────────────────────┐
│ Router 单例 + 两级缓存(L1 freecache/L2 SQLite) + K线落盘    │
│ 5 条 fallback 链已装配；但全仓仅 3 处调用方（indicator/     │
│ backtest/history sync），quote/news/fundamental/sector     │
│ 链"注册即休眠"                                             │
└──────────────────────────────────────────────────────────┘
┌─ 层A backend/data legacy 直连（现实主力）──────────────────┐
│ 绝大多数 handler / agent 工具 / engine 直接 new API 调用    │
│ 自带手写内联 fallback（commodity/新浪K线），无统一缓存      │
└──────────────────────────────────────────────────────────┘
```

**核心判断**：架构方向正确（port/adapter + fallback 链 + 两级缓存），但收敛未完成——
同一数据源最多存在 3 份适配代码、3 条顺序各异的 fallback 链，缓存层因缺陷长期无效，
绝大多数调用方仍在绕过它直连。

### 1.2 本次已修复的 P0 缺陷（commit 待填）

| # | 缺陷 | 影响 | 修复 |
|---|------|------|------|
| 1 | `CacheLayer.Get` 反序列化到 `interface{}`，Router 侧 `.(*QuoteData)` 断言恒失败 | **5 条 TTL 全部失效**，每次请求全链穿透+重写缓存 | 新增 `GetInto(ctx,key,target)` 类型化读取；Router 5 处改用；L2→L1 提升 TTL 改为剩余时间（封顶300s）；`db.Dao` nil 保护；补回归测试 `TestCacheLayerTypedRoundTrip` |
| 2 | `SharedHTTPClient.SetTimeout()` 全局篡改超时（5 个构造器持有共享实例 + 45 个调用点） | 并发下一个数据源可把另一个的超时改成 300s，互相覆盖 | 构造器改 `ConfiguredHTTPClient()`（独立 http.Client 共享 Transport）；调用点 sed 改 `CreateHTTPClientWithTimeout()`；仅保留 `UpdateHTTPClientTimeout` 一处全局配置入口 |
| 3 | fundamental 链把"财报条数"当 Revenue 写入缓存（tushare/xueqiu 两 provider） | **数据正确性事故**：假数据进缓存被下游消费 | 重写为东财 F10 杜邦分析真实实现（ROE/营收/净利/资产负债率），A 股守卫 + Yahoo 兜底全球 |
| 4 | quote 链 tdx/sina 两 provider 恒返回错误且 Available=true | 每次报价请求先吃 1-2 次确定性失败（日志噪音+延迟） | `Available()` 改 false，链直接跳过 |
| 5 | `FindMissingDateRanges` 对区间内每个交易日调一次 timor 节假日 API（1年≈250次） | 历史同步极慢（原作者已在 sync.go 绕开本函数） | 批量端点（50日/请求）+ 进程级 `sync.Map` 缓存；失败退回单日查询 |
| 6 | commodity `emitFinalReport` 裸 `ch <-` 无 ctx select | 消费者关闭时 goroutine 永久泄漏（multi 侧已修，commodity 漏同步） | 对齐 multi 实现：`select { case ch<-…; case <-ctx.Done() }` |
| 7 | Router `sort.Slice` 非稳定排序 | 同优先级 provider 顺序随机，行为不可复现 | `sort.SliceStable`（与 internal/adapter 一致） |

---

## 二、遗留问题清单（P1/P2，按收益排序）

### 2.1 数据源层（P1 = 一周内可完成、高收益）

| # | 问题 | 位置 | 建议方案 |
|---|------|------|---------|
| P1-1 | Yahoo 双熔断不共享：包级 `yahooRateLimited` + 3 个 provider 各持独立 circuit，整体故障需 3×5 次失败才全熔断；注释宣称的指数退避不存在 | `data/yahoo_finance_api.go:26-44`、`fallback/yahoo_provider.go:42-107` | 收敛为单一包级熔断器（三态+half-open 单飞），3 provider 共享；补 5min/15min/60min 三档退避 |
| P2-2 | Yahoo PowerShell 降级每请求起一个 powershell 进程 | `data/yahoo_finance_api.go:85-104` | 降级成功后冷却期内直接走 PowerShell 通道（进程复用/常驻 job），或迁移到 curl_cffi 等价方案 |
| P1-3 | quote 链 mootdx/eastmoney 同上游（都是东财 HTML 爬价）；sector 链 tdx_sector/eastmoney_sector/baidu 三 provider 同上游 `GetStockConceptInfo` | `fallback/quote_chain.go:27-58`、`sector_chain.go`、`free_data.go:257-268` | 去重：quote 链保留一个东财 provider + 腾讯 qt.gtimg 做真第二源；sector 链保留一个，其余移除注册 |
| P1-4 | K 线链 tencent_kline（free_data.go:137）与 TencentKLineApi（sina_kline_api.go:420）双实现 | 同上 | 收敛为一份，另一份删除 |
| P1-5 | `FetchKLineWithFallback`（层A手写链）、层B Router 链、层C adapter 链三条链三种顺序 | `sina_kline_api.go:562-622` 等 | 统一收敛到层B Router：StockHandler/agent 工具的 K 线调用全部改走 `Router.GetKLine`（自动获得缓存+落盘） |
| P1-6 | 层C `internal/adapter/datasource` 无缓存、仅 1 调用方，与层B 重复 | 整个目录 | 二选一：层C 改为薄包装层B Router（推荐），或删除层B。迁移期建议前者 |
| P1-7 | ctx 未透传进 legacy HTTP 调用（provider 拿到 ctx 不用于 HTTP） | 各 provider | 分批改造各 API 函数签名接受 ctx；退路：provider 层用 `context.WithTimeout` 包裹 + `resty` 请求级 ctx |
| P1-8 | 无单 provider 超时上限，最坏耗时=全链超时之和 | `datasource/router.go` | Router 内 `ctx, cancel := context.WithTimeout(ctx, perProviderTimeout)` 逐源包裹（quote 5s/kline 15s/news 8s） |
| P2-9 | `history/sync.go:257-259` 每缺口区间固定拉 2000 根再过滤 | `history/sync.go` | 按 `FindMissingDateRanges` 实际缺口长度计算 count |
| P2-10 | 预期跳过（Yahoo 跳过 A 股）以 error 返回污染 Warn 日志 | `fallback/yahoo_provider.go` | 引入 `ErrUnsupported` 哨兵，Router 静默跳过 |

### 2.2 AI 集成层

| # | 问题 | 位置 | 建议方案 |
|---|------|------|---------|
| A1 | 工具三套并存：agent/tools(112个 eino) vs data/tools.go(自研 schema) vs openai_tools 回调注册表 | `agent/agent.go`、`data/tools.go`、`data/tool_registry.go` | 收敛为单一工具注册中心（eino 为准），data/tools.go 标记 Deprecated 后删除；`thsResultToMarkdown` 双实现合并 |
| A2 | 工具调用无缓存无超时：7 分析师对同一股票各自独立抓数；GetCurrentTime 都发网络请求 | `data_tools_wrapper.go`、`multi/*.go` | **工具数据门面**（见 3.2）：统一走 Router 缓存；工具层加 10s 超时；multi 引擎预抓一次共享数据包传入各分析师 |
| A3 | 记忆会话串扰：sessionID 默认 `ai-config-{id}`，前端 sessionId 未传后端 | `agent_api.go:57-61`、`agent_handler.go:99` | 前端透传 sessionId → 后端按会话隔离记忆 |
| A4 | `createFallbackReactAgent` 恒 nil，PlanExecute 降级分支死代码；`GetAllTools` 无调用方 | `agent_api.go:504-512`、`agent.go:329-349` | 删除死代码或实现真降级 |
| A5 | 关键词工具过滤近乎失效（中文关键词覆盖英文缺失，常见短语命中全部组） | `tool_groups.go:264-302` | 关键词表双语化 + 提高门槛；或直接按 token 预算全量注册（工具描述已精简时） |
| A6 | maxStep=工具数×2+10（百级工具时空转）、safeSend 丢帧无感知 | `agent.go:137-140`、`agent_api.go:1050` | maxStep 上限 30；丢帧改为重试或向前端发事件 |
| A7 | 分析师 LLM 失败不重试直接产出 Error 报告；辩论失败占位吞掉 | `multi/fundamental.go:43`、`researcher.go:46` | 单次重试 + 显式降级文案（保留可观测性） |
| A8 | 强制系统通知无开关；分享接口 CORS `*` | `agent_handler.go:167`、`ai-assistant-web/server.go:303` | 设置项开关；CORS 白名单 |

### 2.3 内部分层

| # | 问题 | 建议方案 |
|---|------|---------|
| S1 | handler 80 处 `context.Background()`，ctx 超时/取消传播断裂 | `NewDefaultXxxHandler` 注入 app ctx，请求入口 `WithTimeout` 派生 |
| S2 | 错误处理三风格混用（中文文案字符串 / (T,error) / 吞错返回空结构） | 短期保持（前端依赖文案）；新增接口一律 `(T, error)`，文档标注约定 |
| S3 | 同表双模型（models vs domain）靠 2400 行映射维持 | 接受现状（迁移过渡成本），禁止新增双定义 |
| S4 | `data/daily_pick_*`（约 2800 行）仍直连 DB 且直绑 Wails | 列入迁移 backlog：service 化 + handler 绑定 |

---

## 三、目标架构规划

### 3.1 数据运用：单一数据门面（收敛终点）

```
调用方（handler / agent 工具 / engine / 前端绑定）
                    │
        ┌───────────▼───────────┐
        │   DataFacade（新，薄）  │   ← 唯一推荐入口，包函数语义化
        │ GetQuote/GetKLine/...  │
        └───────────┬───────────┘
                    │ 委托
        ┌───────────▼───────────┐
        │  data/datasource.Router │  ← 唯一 Router（吸收层C后）
        │  + CacheLayer(已修复)   │
        │  + KLineStore 落盘      │
        │  + per-provider 超时    │
        └───────────┬───────────┘
        ┌───────────▼───────────┐
        │ fallback 链（去重后）    │
        │ quote:  eastmoney→tencent→yahoo(非A股)│
        │ kline:  freestockdb→tdx→eastmoney→yahoo│
        │ news:   wallstreetcn→eastmoney→本地DB │
        │ fund:   东财杜邦→yahoo(非A股)          │
        │ sector: freestockdb→eastmoney         │
        └────────────────────────┘
```

**收敛原则**：
1. 新代码一律走 DataFacade；存量调用方按域分批迁移（先 agent 工具 → 再 StockHandler → 再 handler 杂项）。
2. 每个上游数据源在链中只允许一个 provider；"provider 复数=高可用"是错觉，同上游重试只增加延迟。
3. 缓存 TTL 分级已有（quote 60s/kline 300s/news 120s/fund 600s/sector 300s），修复后即生效；后续按数据域增加**单飞(single-flight)**防缓存击穿（同一 key 并发 miss 只放一个请求出去）。

### 3.2 AI 集成：工具数据门面 + 共享数据包

```
multi-agent 引擎
  ├─ 阶段0（新）: PrefetchDataPack(code) ──→ DataFacade 一次性抓取
  │    { quote, kline(day/week), news, moneyflow, fundamental }
  │    → 存入 CommodityContext/AnalysisContext，全分析师共享
  ├─ 分析师: prompt = 角色模板(DB) + DataPack 渲染（不再各自抓数）
  ├─ 单Agent工具: DataToolWrapper.handler → DataFacade（自动缓存+超时+重试）
  └─ 会话记忆: sessionId 全链路透传，按会话隔离
```

**完整性/可靠性/易用性三轴目标**：

| 轴 | 现状缺口 | 目标状态 |
|----|---------|---------|
| 完整性 | fundamental 链 A 股曾有假数据（已修复为杜邦实现）；板块资金/龙虎榜等未入链 | 五类数据全覆盖真实源；新数据域一律先定义 port 接口再实现 provider |
| 可靠性 | 工具无超时/无重试、双熔断、goroutine 泄漏（已修）、错误直抛用户 | 全链路 ctx 超时；单一熔断器；LLM 失败重试+显式降级；错误结构化上报 |
| 易用性 | 三套工具注册、prompt 三处硬编码、会话记忆串扰 | 单一工具注册中心；prompt 全部 DB 化（role_key 已具备）；会话级记忆隔离 |

### 3.3 分阶段路线图

| 阶段 | 内容 | 预估 |
|------|------|------|
| ✅ P0（本次） | 缓存失效/超时篡改/假数据/死 provider/N+1/goroutine 泄漏 | 已完成 |
| P1（数据收敛） | A1 工具注册统一、P1-3/4/5 provider 去重与链收敛、P1-8 per-provider 超时、A3 会话隔离 | 3-5 天 |
| P2（AI 数据面） | A2 PrefetchDataPack 共享数据包、A4/A5/A6 死代码与过滤修复、single-flight 防击穿 | 3-5 天 |
| P3（深层治理） | P1-1/2 Yahoo 熔断统一与降级优化、S1 ctx 传播、P2-9/10、S4 daily_pick service 化 | 按需 |

---

## 四、验证基线（本次修复后）

- `go build ./...` ✅
- `go vet ./...` ✅ 零告警
- `go test ./backend/data/datasource/...` ✅（含新增 `TestCacheLayerTypedRoundTrip`）
- `go test ./backend/internal/... ./backend/models/ ./backend/data/trading/` ✅
