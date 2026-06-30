# 大宗商品数据修复方案 (v2 - Momus审查后修订)

## 问题概述

大宗商品页（commodity.vue）4 个标签页均无数据显示。用户打开页面后，所有价格卡片/表格均显示 `--`，K 线图无数据。

## 根因分析

### P0 — Sina API GBK 编码未处理

6月30日修复 `getFuturesQuote`/`getFuturesKLine` 时使用 Sina API，但：

**`getFuturesQuote`（commodity_api.go:86）：**
`body := string(resp.Body())` — Sina `hq.sinajs.cn` 返回 GB18030 编码，直接转 string 导致中文乱码，`strings.Index(body, "\"")` 找不到引号 → 解析失败 → 返回 error → 前端 `--`

**`getFuturesKLine`（commodity_api.go:232）：**
同样 `string(resp.Body())` 未解码。Sina 每日 K 线 API 返回纯 ASCII JSON（日期/数字），GBK 不会破坏 JSON 解析，但为防御性正确仍需加解码。

## 修复任务

### 任务 1: `getFuturesQuote` — 添加 GB18030 解码 [HIGH]

**文件**: `backend/data/commodity_api.go`
**位置**: `getFuturesQuote` 函数，约第 86 行

**改动**: 将 `body := string(resp.Body())` 替换为 `body := GB18030ToUTF8(resp.Body())`

**QA**: 调用 `GetCommodityQuote("AU")` → 应返回包含 Price/ChangePct 等字段的 QuoteData；若 Sina 返回 `var hq_str_NF_AU0="沪金,..."`，解码后中文可读，解析成功

### 任务 2: `getFuturesKLine` — 添加 GB18030 解码 + 验证 JSONP 解析 [HIGH]

**文件**: `backend/data/commodity_api.go`
**位置**: `getFuturesKLine` 函数，约第 232 行

**改动**：
1. `body := string(resp.Body())` → `body := GB18030ToUTF8(resp.Body())`
2. 修复 JSONP 提取索引逻辑：先用 `strings.Index(body, "[")` 找到 JSON 数组起始，再用 `strings.LastIndex(body, "]")` 找到结束（当前逻辑正确，但需确认 `endIdx` 不包含 `]);` 后缀）

**QA**: 调用 `GetCommodityKLine("AU", "day", 120)` → 应返回 `[]datasource.KLineBar`，长度 > 0，Time/Open/Close/High/Low 字段有效

### 任务 3: 添加 EastMoney fallback [MEDIUM]

**文件**: `backend/data/commodity_api.go`

当 Sina API 失败时，fallback 到 EastMoney 实时报价 API。但 EastMoney `/api/qt/stock/get` 是否支持期货 secid 未经确认，保留为实验性功能。

**改动**: 在 `getFuturesQuote` 中 Sina 失败后，尝试调用 `emClient.GetKLineData(asset.Symbol, "101", "0", 1)`（将 adjustFlag 改为 "0" 而非空字符串），若返回数据则取最新一条 close 作为当前价

**QA**: 断开网络后调用 `GetCommodityQuote("AU")` → 返回具体错误信息而非前端 `--`；网络正常时优先走 Sina

### 任务 4: 验证 ETF 报价 [MEDIUM]

**文件**: 无代码改动（仅验证）

**验证方法**: 调用 `GetCommodityQuote("518880")` → 应返回含 Price 字段的 QuoteData

### 任务 5: WallStreetCN 健康检查 [LOW]

**文件**: `backend/data/commodity_api.go`

**改动**: 在 `getSpotQuote` 返回 error 时使用中文描述 "华尔街见闻数据源未配置或网络不可达"

**QA**: 调用 `GetCommodityQuote("XAUUSD")`，若 WS 未配置 → 返回中文错误提示

### 任务 6: 整体集成验证 [HIGH]

**方法**: 手动验证步骤

| 场景 | 操作 | 预期 |
|------|------|------|
| 行情总览-现货 | 打开页面，查看 XAUUSD/XAGUSD/USCL 卡片 | 价格非 `--`，涨跌幅正确着色（红涨绿跌） |
| 行情总览-期货 | 查看 AU（沪金）卡片 | 同上 |
| 行情总览-K线 | 点击各品种卡片切换 K 线图 | K 线图渲染，有柱状数据 |
| 商品期货 | 切换到"商品期货"标签 | AU/AG/SC 三行数据，价格非 `--` |
| 商品基金 | 切换到"商品基金"标签 | 3 个 ETF 行数据，价格非 `--` |
| AI分析-技术面 | 选择 AU → 技术面 → 日线 → AI 分析 | 返回技术指标文本 |
| AI分析-组合 | 勾选全部三个模式 → 关联品种填写 XAGUSD → AI 分析 | 返回综合报告 |

## 优先级顺序

```
1. 任务 1 (GBK编码 - getFuturesQuote)   → 期货报价立即可见
2. 任务 2 (GBK编码 - getFuturesKLine)   → K线图可用
3. 任务 4 (ETF验证)                       → 基金页可用
4. 任务 3 (EastMoney fallback)           → 容错增强
5. 任务 5 (WS健康检查)                    → 提示改进
6. 任务 6 (整体验证)                      → 确保全链路
```
