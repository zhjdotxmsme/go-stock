# free-stockdb 接入规划（修订版 · 2026-10-20）

**已锁定产品决策**

| 决策 | 结论 |
|------|------|
| stock-sdk | **不保留**（移除内置 MCP 注册；DB 中旧记录启动时清理） |
| K 线 UI | **不改造**（前端图表仍可走东财等直连） |
| 加强范围 | **仅 Router 侧**（回测、Khunter、Kronos、Agent 工具、`tool_indicator` 等已走 `GetRouter().GetKLine` 的路径） |
| 39 指标 Go 移植 | **不做**（Agent 技术指标改由用户配置 **free-stockdb 原生 MCP**） |

**Goal：** 本机 `127.0.0.1:7899` 作为 Router **最高优先级** K 线/板块源（priority=5），失败时现有 mootdx→腾讯→东财→Yahoo 不变。

**Spec 基线：** `.openteams/specs/2026-08-01-freestockdb-底座接入-design.html`  
**实施任务分解：** `docs/superpowers/plans/2026-08-01-freestockdb-数据接入层.md`（按本文裁剪指标与 UI 部分）

**上游参考：** [hello245m/free-stockdb](https://github.com/hello245m/free-stockdb)（Release 测试版本 0.3.5；HTTP K-V + 独立 MCP `stock_mcp_server.py`）

---

## 架构（Router-only）

```text
回测 / Khunter / Kronos / tool_indicator / 部分 handler
  → datasource.GetRouter().GetKLine
    → freestockdb.Provider (p=5, 7899 Available)
      → KLineService（Go 移植 stock_sdk 语义：查询/复权/聚合）
    → mootdx_kline (10) → tencent (10) → eastmoney (20) → yahoo (25)
  → 命中后仍异步写 KLineStore（router.go 现有逻辑）

前端 K 线图 / 实时盯盘
  → 不改（非 Router 路径）

AI Agent 39 指标 / bk.get / zb.get
  → 设置 → MCP：free-stockdb（python -u …/stock_mcp_server.py），需本机 stockdb 已监听 7899
  → 与 Router 解耦；K 线数据尽量同源（用户先同步 ./data）
```

---

## 里程碑

### M0 — 决策落地（本次）

- [x] 本文档
- [x] 移除 `main.go` / `mobile` 的 stock-sdk 内置注册；启动时删除 DB 中 `name=stock-sdk` 的 MCP 记录
- [x] 可选种子：`free-stockdb` MCP 条目（**默认 Disable**，描述中说明路径与 `pybao/安装.py` 前提）
- [x] M1 起步：`freestockdb/client.go` + 单测

### M1 — Router 数据层（原计划 Task 1～9，无 indicator 包）

- [x] `freestockdb` 包全套 + `code.go`（`EngineCode` / `barTime` 本地时区）
- [x] Settings 四字段 + `main.go` / `mobile` `Setup`（优先于 mootdx 注册）
- [x] 前端设置页（free-stockdb 区块 + 7899 探测）

| 包路径 | 内容 |
|--------|------|
| `backend/data/datasource/freestockdb/client.go` | HTTP K-V |
| `kline.go` / `factor.go` / `aggregate.go` / `service.go` | get_data 语义 |
| `board.go` | 板块索引 → SectorProvider |
| `manager.go` | 可选拉起 stockdb.exe + 健康检查 |
| `provider.go` | KLine + Quote(兜底) + Sector |
| `setup.go` | `Setup(router, Config)` 供 main 调用 |

**Settings（GORM）：** `FreeStockDBEnable`, `FreeStockDBAddr`（默认 `127.0.0.1:7899`）, `FreeStockDBPath`, `FreeStockDBAutoStart`

**main.go：** `initDataSources()` 开头 `freestockdb.Setup(router, cfg)`（从 Settings 读；未启用则 no-op）

**验收：**

1. 开 stockdb + 启用设置 → 日志 `datasource: kline … from freestockdb`
2. 关 stockdb → 自动东财/腾讯，无 panic
3. `go test ./backend/data/datasource/freestockdb/ -v`
4. 回测单票 `sh600519` 与关闭引擎对比（日期连续即可）

### M2 — 与架构债联动（P1，Router 有价值后再做）

- P1-5：`FetchKLineWithFallback` 等层 A 链收敛到 Router
- P1-8：单 provider 超时（kline 建议 15s）
- 数据管理页：**仅展示**本地引擎连接状态 / 最近交易日（只读探针，不改同步逻辑）

### M3 — MCP 文档与预设（无 stock-sdk）

- 使用手册一节：free-stockdb 安装、`stock_mcp_server.py` 路径、与 7899 依赖关系
- MCP 列表模板：`Command=python`, `Args=["-u","<安装目录>/调用方式/ai_mcp/stock_mcp_server.py"]`

### 明确不做

- `backend/data/indicator` 39 指标 Go 移植（原计划「二」）
- 前端 K 线组件改 Router
- internal 六边形 freestockdb adapter（等 M1 稳定 + 调用方收敛）

---

## 与 CNEquity 分工（可选并行）

- **free-stockdb**：本机 K 线/分钟/复权/板块/39 指标（Router + MCP）
- **CNEquity**：财报 PIT、资金面、宏观等厚数据，仅 **外挂 MCP**，不进 Router

---

## 验证命令（完成 M1 后）

```bash
set GOCACHE=E:\open-source\ai\go-stock\.gocache
go test ./backend/data/datasource/freestockdb/ -v
go build ./...
go vet ./backend/...
```

可选黄金对拍：`STOCKDB_ADDR=127.0.0.1:7899 go test ./backend/data/datasource/freestockdb/ -run TestGoldenCompare -v`
