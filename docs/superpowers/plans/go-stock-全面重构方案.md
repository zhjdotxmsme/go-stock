# go-stock 全面重构方案（统一文档）

> **文档版本**: v2.2（合并版 + 实施进度）
> **日期**: 2026-08-03
> **分支**: feat/multi-agent-analysis
> **说明**: 合并之前的 6 个独立文档为一个统一方案，包含功能清单、架构设计、代码迁移、接口设计、外部借鉴、股票代码归一化。v2.1 更新：§2.10/§5.2/§5.5/§11 反映大宗商品 AI 专家路由架构重构（已完成）。v2.2 更新（2026-08-05）：§十一 迁移路线图标注实际完成状态——**Phase 0-8 已全部实施完毕并接入生产管线**，详细进度见 `docs/superpowers/progress-board.md`。

---

# 第一部分：功能清单与审计

## 一、项目概况

- **项目**: go-stock — AI 驱动的股票分析桌面应用
- **技术栈**: Wails v2 (Go) + Vue 3 + Naive UI + ECharts + Lightweight Charts
- **当前分支**: `feat/multi-agent-analysis`（181 commits，相对 dev 分支）
- **规模**: 后端 ~126 个 Go 文件（`backend/data/` 单包），`app.go` 3488 行；前端 81 个 Vue 组件，最大单文件 4832 行
- **Wails 绑定方法**: ~165 个（App 140 + BacktestService 11 + DailyPickService 13 + DailyPickBacktestService 1）
- **前端路由**: 15 个页面，12 个顶级导航菜单

## 二、功能清单（按模块）

### 2.1 股票自选

| # | 功能 | 后端方法 | 前端组件 | 状态 | 决策 |
|---|------|----------|----------|------|------|
| S1 | 股票搜索/自动补全 | `GetStockList`, `SearchStock` | `stock.vue`, `SelectStock.vue` | ✅ 活跃 | **保留** |
| S2 | 自选关注/取消 | `Follow`, `UnFollow` | `stock.vue` | ✅ 活跃 | **保留** |
| S3 | 自选列表（含分组Tab） | `GetFollowList` | `stock.vue` | ✅ 活跃 | **保留** |
| S4 | 群组CRUD | `AddGroup/RemoveGroup/GetGroupList/AddStockGroup/RemoveStockGroup` | `stock.vue` | ✅ 活跃 | **保留** |
| S5 | 成本价/持仓量设置 | `SetCostPriceAndVolume` | `stock.vue` | ✅ 活跃 | **保留** |
| S6 | 交易价格设置（入场/止盈/止损/成本） | `SetTradingPrice` | `stock.vue` | ✅ 活跃 | **保留** |
| S7 | 涨跌报警设置 | `SetAlarmChangePercent` | `stock.vue` | ✅ 活跃 | **保留** |
| S8 | 股票排序 | `SetStockSort` | `stock.vue` | ✅ 活跃 | **保留** |
| S9 | 实时价格获取 | `GetStockRealTimePrice` | `stock.vue` | ✅ 活跃 | **保留** |
| S10 | K线数据（多数据源） | `GetStockKLine`, `GetStockKLineWithFallback`, `GetStockEastMoneyKLine` 等 | `StockLightweightKlineChart.vue` | ✅ 活跃 | **保留** |
| S11 | 分钟价格线 | `GetStockMinutePriceLineData` | `stock.vue` | ✅ 活跃 | **保留** |
| S12 | 筹码分布 | `GetChipDistribution` | `stock.vue` | ✅ 活跃 | **保留** |
| S13 | 通达信数据（集合竞价/公司信息/财务/除权/板块） | `GetTdxCallAuction/CompanyInfo/FinanceInfo/XDXRInfo/SymbolBelongBoard` 等 | `stock.vue` 弹窗 | ✅ 活跃 | **保留** — 整合到个股详情面板 |
| S14 | 弹幕功能 | — | `stock.vue` | ⚠️ 半成品 | **删除** — 低价值，增加噪音 |
| S15 | 全部股票列表（含指标） | `GetAllStocks` | `allStockList.vue` | ✅ 活跃 | **保留** |
| S16 | 股票基本信息管理 | `GetAllStockInfoList/AddAllStockInfo/DeleteAllStockInfo/BatchDeleteAllStockInfo` | `allStockInfoList.vue`, `DataManager.vue` | ✅ 活跃 | **保留** — 仅 DataManager 使用 |
| S17 | 市场/行业/概念列表 | `GetAllMarkets/GetAllIndustries/GetAllConcepts` | `allStockList.vue` 过滤器 | ✅ 活跃 | **保留** |
| S18 | 个股研报 | `StockResearchReport` | `StockResearchReportList.vue` | ✅ 活跃 | **保留** |
| S19 | 公司公告 | `StockNotice` | `StockNoticeList.vue` | ✅ 活跃 | **保留** |

### 2.2 交易日志

| # | 功能 | 后端方法 | 前端组件 | 状态 | 决策 |
|---|------|----------|----------|------|------|
| T1 | 交易记录CRUD | `AddTradingRecord/GetTradingRecordList/UpdateTradingRecord/DeleteTradingRecord` | `TradingRecordManager.vue` | ✅ 活跃 | **保留** |
| T2 | 交易统计 | `GetTradingRecordStatistics` | `TradingRecordManager.vue` | ✅ 活跃 | **保留** |
| T3 | 频繁交易检测 | `CheckFrequentTrading` | `TradingRecordManager.vue` | ✅ 活跃 | **保留** |
| T4 | 交易记录查看K线 | `GetStockRealTimePrice` | `TradingRecordManager.vue` | ⚠️ VIP2 限制 | **取消VIP限制** — 核心功能不应付费 |

### 2.3 异动监控

| # | 功能 | 后端方法 | 前端组件 | 状态 | 决策 |
|---|------|----------|----------|------|------|
| C1 | 实时异动获取 | `GetStockChanges`, `GetAllStockChangesWithPaging` | `stockChangesMonitor.vue` | ✅ 活跃 | **保留** |
| C2 | 异动历史 | `GetStockChangeHistory` | `stockChangesMonitor.vue` | ✅ 活跃 | **保留** |
| C3 | 异动保存到历史 | `SaveStockChangesToHistory` | `stockChangesMonitor.vue` | ✅ 活跃 | **保留** |
| C4 | 每日异动统计 | `GetDailyChangeStats/GetChangeTypeDailyStats/GetChangeRank/GetDailyDimensionStats/GetTypeStatsByDate` | `AnalyzeMartket.vue` | ✅ 活跃 | **保留** |
| C5 | 异动历史删除 | `DeleteStockChangeHistory` | `stockChangesMonitor.vue` | ✅ 活跃 | **保留** |

### 2.4 市场行情

| # | 功能 | 后端方法 | 前端组件 | 状态 | 决策 |
|---|------|----------|----------|------|------|
| M1 | 财联社电报 | `GetTelegraphList/ReFleshTelegraphList` | `newsList.vue` | ✅ 活跃 | **保留** |
| M2 | 全球股指 | `GlobalStockIndexes/GlobalStockIndexesReadable` | `market.vue` | ✅ 活跃 | **保留** |
| M3 | 行业排名 | `GetIndustryRank/GetIndustryMoneyRankSina` | `industryMoneyRank.vue` | ✅ 活跃 | **保留** |
| M4 | 个股资金流向 | `GetMoneyRankSina/GetStockMoneyTrendByDay` | `rankTable.vue`, `moneyTrend.vue` | ✅ 活跃 | **保留** |
| M5 | 板块资金流 | `GetBKFundFlowList/GetBKFundFlowListByDate/GetBKFundFlowTopList/GetBKFundFlowTopListByDate/GetAllBKCodes` | `bkFundFlowChart.vue` | ✅ 活跃 | **保留** |
| M6 | 概念资金流 | `GetConceptFundFlowList/.../GetAllConceptCodes` | `conceptFundFlowChart.vue` | ✅ 活跃 | **保留** — 与 M5 合并为一个组件 |
| M7 | 龙虎榜 | `LongTigerRank` | `LongTigerRankList.vue` | ✅ 活跃 | **保留** |
| M8 | 行业研究 | `IndustryResearchReport`, `EMDictCode` | `IndustryResearchReportList.vue` | ✅ 活跃 | **保留** |
| M9 | 当前热门（雪球/东财） | `HotStock/HotEvent/HotTopic` | `HotStockList.vue`, `HotEvents.vue`, `HotTopics.vue` | ✅ 活跃 | **保留** |
| M10 | 投资日历 | `InvestCalendarTimeLine`, `ClsCalendar` | `InvestCalendarTimeLine.vue`, `ClsCalendarTimeLine.vue` | ✅ 活跃 | **保留** — 两个日历合并为一个 |
| M11 | 涨停排行 | `GetUplimitHot` | `uplimitLadder.vue` | ✅ 活跃 | **保留** |
| M12 | 市场统计 | `FetchAndSaveMarketStatistic/GetTodayMarketStatistic/GetMarketStatisticByDate/GetRecentDaysMarketStatistic` | `AnalyzeMartket.vue` | ✅ 活跃 | **保留** |
| M13 | 外部网站嵌入（名站优选） | — | `stockhotmap.vue`, `EmbeddedUrl.vue` | ✅ 活跃 | **精简** — 去掉失效链接，保留3-5个 |
| M14 | 交易时间判断 | `IsTradingTime/IsHKTradingTime/IsUSTradingTime/IsTradingDay/GetLatestTradingDay` | `market.vue`, `stock.vue` | ✅ 活跃 | **保留** |
| M15 | 情绪分析 | `AnalyzeSentiment/AnalyzeSentimentWithFreqWeight` | 内部使用 | ✅ 活跃 | **保留** |

### 2.5 AI 分析

| # | 功能 | 后端方法 | 前端组件 | 状态 | 决策 |
|---|------|----------|----------|------|------|
| A1 | 单股AI对话流 | `NewChatStream` | `stock.vue` AI 弹窗 | ✅ 活跃 | **保留** — 核心功能 |
| A2 | 多Agent分析（7分析师+多空辩论） | 内部调用 | `MultiAgentResult.vue`, `DecisionDashboard.vue` | ✅ 活跃 | **保留** — 核心功能 |
| A3 | AI分析结果管理 | `SaveAIResponseResult/GetAIResponseResult/GetAIResponseResultList/DeleteAIResponseResult/BatchDeleteAIResponseResult` | `researchReport.vue` | ✅ 活跃 | **保留** |
| A4 | 导出Markdown/Word/图片 | `SaveAsMarkdown/SaveWordFile/SaveImage` | `stock.vue`, `researchReport.vue` | ✅ 活跃 | **保留** |
| A5 | 分享分析 | `ShareAnalysis/ShareText` | `stock.vue` | ✅ 活跃 | **保留** |
| A6 | AI对话Agent（浮动面板） | `ChatWithAgent/AbortChatWithAgent` | `FloatingAgentAssistant.vue` | ✅ 活跃 | **保留** |
| A7 | AI Agent 独立页面 | `ChatWithAgent` | `agent-chat.vue` | ✅ 活跃 | **保留** |
| A8 | AI新闻摘要 | `SummaryStockNews/AbortSummaryStockNews` | `market.vue` | ✅ 活跃 | **保留** |
| A9 | AI推荐股票 | `GetAiRecommendStocksList/DeleteAiRecommendStocks/UpdateAiRecommendStocksAlert/GetAiRecommendStats` | `aiRecommendStocksList.vue` | ✅ 活跃 | **保留** |
| A10 | AI配置策略选股 | `AIConfiguredStockPick/GetHotStrategy` | `SelectStock.vue` | ✅ 活跃 | **保留** |

### 2.6 提示词与策略

| # | 功能 | 后端方法 | 前端组件 | 状态 | 决策 |
|---|------|----------|----------|------|------|
| P1 | 提示词模板管理 | `GetPromptTemplates/AddPrompt/DelPrompt/GetPromptTemplateList/AddPromptTemplate/UpdatePromptTemplate/DeletePromptTemplate` | `promptTemplateList.vue` | ✅ 活跃 | **保留** |
| P2 | 多Agent提示词配置 | `GetMultiAgentPrompts/UpdateMultiAgentPrompt` | `settings.vue` | ✅ 活跃 | **保留** |
| P3 | 自定义策略管理 | `GetCustomStrategyList/GetAllCustomStrategies/SaveCustomStrategy/DeleteCustomStrategy` | `SelectStock.vue` | ✅ 活跃 | **保留** |
| P4 | 提示词广场（远程API） | `AddPromptTemplate` (remote) | `promptPlaza.vue` | ⚠️ 依赖外部API | **删除** — 外部API不可控，已硬编码地址 |
| P5 | 问答广场（远程API） | — | `promptQa.vue` | ⚠️ 依赖外部API | **删除** — 外部API不可控 |

### 2.7 回测系统

| # | 功能 | 后端方法 | 前端组件 | 状态 | 决策 |
|---|------|----------|----------|------|------|
| B1 | 单股回测 | `RunSingleBacktest` | `BacktestPanel.vue` | ✅ 活跃 | **保留** |
| B2 | 批量回测 | `RunBatchBacktest` | `BacktestPanel.vue` | ✅ 活跃 | **保留** |
| B3 | 参数网格搜索优化 | `RunOptimization/GetOptimizationPresets` | `BacktestPanel.vue` | ✅ 活跃 | **保留** |
| B4 | 回测结果管理 | `GetBacktestResults` | `BacktestPanel.vue` | ✅ 活跃 | **保留** |
| B5 | 每日选股回测 | `RunBacktestForDailyPicks` | `BacktestPanel.vue` | ✅ 活跃 | **保留** |
| B6 | K线缓存统计 | `GetKLineCacheStats` | `DataManager.vue` | ✅ 活跃 | **保留** |
| B7 | 历史数据同步 | `StartHistoricalSync/GetSyncProgress` | `DataManager.vue` | ✅ 活跃 | **保留** |
| B8 | 种子数据导入 | `GetSeedImportStatus/RunSeedImport/GetLastSeedImportOutput` | `DataManager.vue` | ⚠️ 一次性操作 | **保留** — 低频功能，不删除 |

### 2.8 每日选股

| # | 功能 | 后端方法 | 前端组件 | 状态 | 决策 |
|---|------|----------|----------|------|------|
| D1 | 执行选股 | `RunDailyPick/RunDailyPickAsync` | `DailyPickPanel.vue` | ✅ 活跃 | **保留** |
| D2 | 选股复评 | `RunDailyReview/ReviewAllUnreviewed/GetReviewSummary` | `DailyPickPanel.vue` | ✅ 活跃 | **保留** |
| D3 | 选股结果管理 | `GetDailyPicks/GetLatestPicks/DeleteDailyPick/UpdateDailyPickRemarks/GetDailyPickStats/GetLatestUnreviewedPicks/GetDateRange/GetReviewTrend` | `DailyPickPanel.vue` | ✅ 活跃 | **保留** |

### 2.9 基金

| # | 功能 | 后端方法 | 前端组件 | 状态 | 决策 |
|---|------|----------|----------|------|------|
| F1 | 基金搜索 | `GetfundList`, `SearchFundCodes` | `FundFollow.vue`, `FundRanking.vue` | ✅ 活跃 | **保留** |
| F2 | 基金关注/取消 | `FollowFund/UnFollowFund` | `FundFollow.vue`, `FundRanking.vue` | ✅ 活跃 | **保留** |
| F3 | 基金K线/历史净值/持仓TOP10 | `GetFundKLine/GetFundHistoryNetValue/GetFundTop10Holdings` | `FundFollow.vue`, `FundKlineChart.vue` | ✅ 活跃 | **保留** |
| F4 | 基金排行 | `GetFundRanking` | `FundRanking.vue` | ✅ 活跃 | **保留** — 取消VIP2 K线限制 |

### 2.10 大宗商品

| # | 功能 | 后端方法 | 前端组件 | 状态 | 决策 |
|---|------|----------|----------|------|------|
| CM1 | 商品报价 | `GetCommodityQuote/GetCommodityQuoteIntl` | `CommodityOverview.vue`, `CommodityFutures.vue` | ✅ 活跃 | **保留** |
| CM2 | 商品K线 | `GetCommodityKLine/GetCommodityKLineIntl` | `CommodityKlineChart.vue`, `CommodityPriceChart.vue` | ✅ 活跃 | **保留** |
| CM3 | 商品注册表 | `GetCommodityRegistry` | `CommodityOverview.vue`, `CommodityFutures.vue`, `CommodityFunds.vue` | ✅ 活跃 | **保留** |
| CM4 | 商品技术/基本面/相关性 | `GetCommodityTechnicals/GetCommodityFundamentals/GetCommodityCorrelation` | `CommodityAnalysis.vue` | ✅ 活跃 | **保留** |
| CM5 | 商品AI分析（3通用+N专属路由） | `NewCommodityAnalysisStream`, `GetCommodityReport` | `CommodityAnalysis.vue` | ✅ 活跃 | **保留** — 已重构为 3通用+N品种专属路由架构（详见§5.5） |
| CM6 | 商品基金 | `GetCommodityQuote` | `CommodityFunds.vue` | ✅ 活跃 | **保留** |

### 2.11 新闻

| # | 功能 | 后端方法 | 前端组件 | 状态 | 决策 |
|---|------|----------|----------|------|------|
| N1 | 板块新闻 | `GetNewsBySector/GetSectors` | `NewsPage.vue` | ✅ 活跃 | **保留** |
| N2 | 个股关联新闻 | `GetStockRelatedNews` | `StockNews.vue` | ✅ 活跃 | **保留** |

### 2.12 定时任务（Cron）

| # | 功能 | 后端方法 | 前端组件 | 状态 | 决策 |
|---|------|----------|----------|------|------|
| CR1 | 个股AI定时分析 | `SetStockAICron` | `stock.vue` | ✅ 活跃 | **保留** |
| CR2 | Cron任务CRUD | `CreateCronTask/UpdateCronTask/DeleteCronTask/GetCronTaskList/EnableCronTask/ExecuteCronTaskNow` 等 | `cron-task-manager.vue` | ✅ 活跃 | **保留** |
| CR3 | 内置定时任务（价格监控/新闻刷新/市场统计等） | `InitCronTasks` 内部 | — | ✅ 活跃 | **保留** |

### 2.13 MCP 服务器管理

| # | 功能 | 后端方法 | 前端组件 | 状态 | 决策 |
|---|------|----------|----------|------|------|
| MCP1 | MCP服务器CRUD | `CreateMCPServer/UpdateMCPServer/DeleteMCPServer/GetMCPServerList/EnableMCPServer/TestMCPServer` | `mcp-server-manager.vue` | ✅ 活跃 | **保留** |
| MCP2 | MCP工具查看 | `GetMCPToolsByServerID/GetAllMCPTools` | `mcp-server-manager.vue` | ✅ 活跃 | **保留** |

### 2.14 技能管理

| # | 功能 | 后端方法 | 前端组件 | 状态 | 决策 |
|---|------|----------|----------|------|------|
| SK1 | 技能CRUD | `CreateSkill/UpdateSkill/DeleteSkill/GetSkillList/EnableSkill/GetAllSkills` | `skill-manager.vue` | ✅ 活跃 | **保留** |
| SK2 | URL生成技能 | `GenerateSkillFromURL` | `skill-manager.vue` | ✅ 活跃 | **保留** |
| SK3 | 技能效果分析 | `AnalyzeSkillEffectiveness` | `skill-manager.vue` | ⚠️ 未验证 | **保留** — 待完善 |
| SK4 | 技能推荐 | — | `skill-recommend.vue` | ✅ 活跃 | **保留** |

### 2.15 通知

| # | 功能 | 后端方法 | 前端组件 | 状态 | 决策 |
|---|------|----------|----------|------|------|
| NT1 | 钉钉通知 | `SendDingDingMessage/SendDingDingMessageByType` | `stock.vue`, `settings.vue` | ✅ 活跃 | **保留** |
| NT2 | 多渠道测试通知 | `SendTestNotification` | `settings.vue` | ✅ 活跃 | **保留** |
| NT3 | 内置5渠道（钉钉/企业微信/飞书/Telegram/邮件） | `notify/` 包 | `settings.vue` 配置 | ✅ 活跃 | **保留** |

### 2.16 系统设置

| # | 功能 | 后端方法 | 前端组件 | 状态 | 决策 |
|---|------|----------|----------|------|------|
| SY1 | 配置管理（API Key等） | `GetConfig/UpdateConfig/ExportConfig` | `settings.vue` | ✅ 活跃 | **保留** |
| SY2 | AI模型配置 | `GetAiConfigs/FetchAiModels/FetchAiModelInfo` | `settings.vue` | ✅ 活跃 | **保留** |
| SY3 | 策略列表 | `GetAllStrategies` | `stock.vue`, `settings.vue` | ✅ 活跃 | **保留** |
| SY4 | 版本信息 | `GetVersionInfo` | `about.vue` | ✅ 活跃 | **保留** |
| SY5 | 用户手册 | `GetUserManual` | `about.vue` | ✅ 活跃 | **保留** |
| SY6 | 机器ID | `GetMachineId` | `about.vue`, `promptPlaza.vue` | ✅ 活跃 | **保留** |
| SY7 | 打开URL | `OpenURL` | 多组件 | ✅ 活跃 | **保留** |
| SY8 | 时区 | `GetTimezone` | `app.go` 内部 | ✅ 活跃 | **保留** |
| SY9 | AI助手会话管理 | `GetAiAssistantSession/SaveAiAssistantSession` | `FloatingAgentAssistant.vue` | ✅ 活跃 | **保留** |

### 2.17 VIP / 赞助

| # | 功能 | 后端方法 | 前端组件 | 状态 | 决策 |
|---|------|----------|----------|------|------|
| VIP1 | 赞助信息展示 | `GetSponsorInfo/GetEffectiveSponsorVip` | `about.vue` | ✅ 活跃 | **保留** |
| VIP2 | 赞助码验证 | `CheckSponsorCode/CheckDeviceBinding` | `about.vue`, `settings.vue` | ✅ 活跃 | **保留** |
| VIP3 | CDN自动更新（VIP1+） | `CheckUpdate` 内部逻辑 | — | ✅ 活跃 | **保留** |
| VIP4 | 外国新闻同步（VIP2+） | `syncNews` 内部逻辑 | — | ✅ 活跃 | **保留** |
| VIP5 | AI助手浮动面板（VIP2+） | — | `FloatingAgentAssistant.vue` | ✅ 活跃 | **重新设计VIP策略**（见2.19） |
| VIP6 | 基金排行K线（VIP2+） | — | `FundRanking.vue` | ⚠️ 限制体验 | **取消VIP限制** |
| VIP7 | 选股K线（VIP2+） | — | `SelectStock.vue` | ⚠️ 限制体验 | **取消VIP限制** |
| VIP8 | 交易日志K线（VIP2+） | — | `TradingRecordManager.vue` | ⚠️ 限制体验 | **取消VIP限制** |

### 2.18 确认删除的功能

| 功能 | 原因 | 涉及文件 |
|------|------|----------|
| 弹幕功能 | 低价值，增加噪音，几乎无人使用 | `stock.vue` 中弹幕相关代码 |
| 提示词广场（promptPlaza） | 依赖不可控的外部API（硬编码地址），社区功能未成规模 | `promptPlaza.vue` (1119行) |
| 问答广场（promptQa） | 同上，依赖外部API | `promptQa.vue` (505行) |
| agent-chat_bk.vue | 旧版备份，从未使用，含mock数据 | `agent-chat_bk.vue` (337行) |
| FloatingAiAssistant.vue | 已被 FloatingAgentAssistant 替代，App.vue 中已注释 | `FloatingAiAssistant.vue` (1344行) |
| allStockInfoList.vue | 已被注释未使用 | `allStockInfoList.vue` (403行) |

### 2.19 VIP 策略重新设计

**当前问题**：
1. VIP 检查方式不统一（`GetSponsorInfo` vs `GetEffectiveSponsorVip`）
2. 核心功能（K线查看）被 VIP 限制，损害用户体验
3. VIP 功能价值不够清晰

**建议新 VIP 策略**：

| 级别 | 价格 | 功能 |
|------|------|------|
| **免费** | 0 | 所有分析功能、K线、选股、回测、通知、数据管理 |
| **VIP1** | 18.8/月 | CDN自动更新 + 高级数据源优先级 |
| **VIP2** | 28.8/月 | VIP1全部 + AI浮动助手面板 + 新闻24h同步 + 多设备绑定 |

**核心原则**：所有数据分析功能免费。VIP 仅限运维便利性（更新速度、多设备）和附加体验（浮动面板、新闻同步）。

---

## 三、重复组件清单

| 重复组件 | 行数 | 问题 | 处理方式 |
|----------|------|------|----------|
| `bkFundFlowChart.vue` vs `conceptFundFlowChart.vue` | 650 vs 650 | 几乎相同的代码，仅 API 函数名不同 | **合并**为一个参数化组件 `FundFlowChart.vue` |
| `FloatingAiAssistant.vue` vs `FloatingAgentAssistant.vue` | 1344 vs 1954 | 旧版 vs 新版，旧版已弃用 | **删除** `FloatingAiAssistant.vue` |
| `agent-chat.vue` vs `agent-chat_bk.vue` | 808 vs 337 | 旧版备份 | **删除** `agent-chat_bk.vue` |
| `KLineChart.vue` (ECharts) vs `StockLightweightKlineChart.vue` (lightweight-charts) | 392 vs 4832 | 两套图表库并存 | **逐步统一**到 Lightweight Charts |
| `InvestCalendarTimeLine.vue` vs `ClsCalendarTimeLine.vue` | 107 vs 101 | 结构相同，数据源不同 | **合并**为一个 `CalendarTimeline.vue` |
| `allStockList.vue` vs `allStockInfoList.vue` | 605 vs 403 | 功能重叠 | **删除** `allStockInfoList.vue` |

---

# 第二部分：架构重构方案

## 四、核心架构问题

### 4.1 后端问题

| 级别 | 问题 | 影响 |
|------|------|------|
| ❌ P0 | `backend/data/` 是 126 文件的"上帝包"，所有数据逻辑平铺在一个包内 | 命名冲突，无法按功能域隔离 |
| ❌ P0 | `app.go` 3488 行"上帝对象"，承担所有 Wails 绑定 | 任何功能修改都需触碰此文件 |
| ❌ P1 | 全局变量泛滥（`globalRouter` 等） | 测试困难，并发风险 |
| ❌ P1 | 数据获取层新旧并存 | Router + 旧独立函数共存 |
| ❌ P1 | Agent 工具文件 5292 行，直接依赖 data 包 | 无法 mock 测试 |
| ⚠️ P2 | 缺乏领域模型层 | 业务逻辑散落在 API 函数中 |

### 4.2 前端问题

| 级别 | 问题 | 影响 |
|------|------|------|
| ❌ P0 | 超大单文件组件（4832行、3151行、2032行、1954行） | 难以维护和阅读 |
| ❌ P1 | 无全局状态管理（无 Pinia） | 跨组件状态共享困难 |
| ❌ P1 | 无 TypeScript | 缺少类型安全 |
| ❌ P1 | Research Center 含 13 个子 Tab，严重超载 | 违反任务内聚原则 |
| ⚠️ P2 | 两套图表库并存（ECharts + Lightweight Charts） | 视觉不一致 |
| ⚠️ P2 | 两套 UI 库混用（Naive UI + TDesign） | 风格不统一 |
| ⚠️ P2 | 股票代码转换函数分散在各组件 | 维护困难 |

---

## 五、目标架构

### 5.1 总体原则

1. **分层架构**: Handler → Service → Port ← Adapter
2. **领域驱动**: 按业务领域划分子包
3. **依赖注入**: 通过接口注入，消除全局变量
4. **新旧兼容**: 渐进式迁移
5. **前端状态管理**: 引入 Pinia，组件拆分，渐进 TypeScript 化
6. **股票代码全局统一**: Router 层归一化，内部标准格式 `sh600519`

### 5.2 目标后端包结构

```
backend/
├── stockcode/                    # 股票代码归一化（零外部依赖）
│   ├── normalize.go              #   ParseStockCode, NormalizeStockCode, StockCodeCandidates
│   ├── convert.go                #   ToTushare, ToEastMoney, ToSina, ToTDX
│   └── normalize_test.go
├── internal/
│   ├── domain/                   # 领域模型（纯 Go struct）
│   │   ├── stock/                #   Stock, KLine, Quote, Group
│   │   ├── fund/                 #   Fund, FundFollow
│   │   ├── analysis/             #   Report, Signal, Score, BacktestResult
│   │   ├── agent/                #   AgentConfig, Pipeline, AnalystReport
│   │   ├── market/               #   Sector, HotStock, CapitalFlow
│   │   ├── commodity/            #   Commodity, FuturesPrice
│   │   ├── news/                 #   News, Topic, Sentiment
│   │   └── system/               #   User, Config, CronTask, MCPServer
│   ├── port/                     # 端口层 — 接口定义
│   │   ├── datasource/           #   数据源接口（见第六部分）
│   │   ├── repository/           #   仓储接口（见第六部分）
│   │   └── notification/         #   通知接口（见第六部分）
│   ├── adapter/                  # 适配器层 — 接口实现
│   │   ├── datasource/           #   eastmoney/, sina/, tdx/, yahoo/, tushare/, freestockdb/, crawler/
│   │   ├── repository/sqlite/    #   GORM 实现
│   │   └── notification/         #   dingtalk, email, feishu, telegram, wxwork
│   └── service/                  # 应用服务层（业务编排）
│       ├── stock/                #   行情、K线、自选、群组
│       ├── fund/                 #   基金
│       ├── analysis/             #   多Agent编排、策略评分
│       ├── market/               #   热门、资金流、板块
│       ├── commodity/            #   商品
│       ├── news/                 #   新闻
│       ├── trading/              #   交易分析
│       ├── backtest/             #   回测
│       ├── daily_pick/           #   选股
│       └── system/               #   配置、Cron、MCP、VIP
├── agent/                        # Agent 层
│   ├── core/                     #   chat model factory、memory、tools
│   ├── multi/                    #   多Agent引擎
│   ├── commodity/                #   商品多专家（3通用+N专属路由架构，详见§5.5）
│   │   ├── expert.go             #   Expert/CategoryExpert 接口 + 注册 + 路由
│   │   ├── types.go              #   CommodityContext（含 Category/AssetType）
│   │   ├── engine.go             #   引擎（注入 asset 元数据 + 动态专家选择）
│   │   ├── prompts.go            #   8 个 Prompt 常量 + DB role key 映射
│   │   ├── synthesis.go          #   合成（动态适配专家数量）
│   │   ├── debate.go             #   多空辩论
│   │   ├── helpers.go            #   SSE emit 工具
│   │   ├── macro_expert.go       #   通用宏观（TIPS多期限 + 分类提示）
│   │   ├── technical_expert.go   #   通用技术面
│   │   ├── sentiment_expert.go   #   通用新闻情绪
│   │   ├── monetary_expert.go    #   [贵金属] 货币属性（TIPS+ETF+CFTC+金银比）
│   │   ├── safe_haven_expert.go  #   [贵金属] 避险（VIX+地缘+日历）
│   │   ├── oil_supply_expert.go #   [能源] 供需（OPEC+库存+COT+WTI-Brent价差）
│   │   ├── oil_geopolitical_expert.go # [能源] 地缘+季节性
│   │   └── fund_tracking_expert.go    #   [基金ETF] 跟踪误差+溢价+量价
│   └── strategy/                 #   策略库
├── handler/                      # Wails 绑定层（替代 app.go）
│   ├── stock_handler.go          #   66 methods
│   ├── system_handler.go         #   51 methods
│   ├── market_handler.go         #   34 methods
│   ├── agent_handler.go          #   29 methods
│   ├── analysis_handler.go       #   31 methods
│   ├── commodity_handler.go      #   10 methods
│   ├── fund_handler.go           #   10 methods
│   ├── news_handler.go           #   4 methods
│   └── notification_handler.go   #   3 methods
├── models/                       # GORM 模型（保持精简）
├── db/                           # 数据库初始化
├── logger/
├── machineid/
└── util/
```

### 5.3 目标前端结构

```
frontend/src/
├── utils/                        # 工具函数
│   └── stockCode.js              #   股票代码统一转换（normalizeStockCode, toEastMoneyCode, toTushareCode）
├── api/                          # API 层（封装 Wails 绑定）
│   ├── stock.ts
│   ├── fund.ts
│   ├── agent.ts
│   ├── market.ts
│   ├── analysis.ts
│   ├── commodity.ts
│   ├── news.ts
│   └── system.ts
├── stores/                       # Pinia 状态管理
│   ├── stock.ts                  #   自选股、群组、行情状态
│   ├── agent.ts                  #   Agent 对话状态
│   ├── settings.ts              #   系统设置状态
│   └── app.ts                    #   全局UI状态
├── types/                        # TypeScript 类型定义
├── composables/                  # 组合式函数
├── components/
│   ├── stock/                    #   拆分后的子组件
│   ├── agent/
│   ├── market/
│   ├── fund/
│   ├── commodity/
│   ├── charts/
│   └── common/
├── views/                        # 页面级组件
└── router/
```

### 5.4 前端导航重构

**当前问题**：Research Center 含 13 个子 Tab，严重超载。

**目标导航结构**（扁平化、按功能域分组）：

| 菜单项 | 路由 | 说明 |
|--------|------|------|
| 📈 股票自选 | `/stock` | 自选列表 + K线 + AI分析（保留） |
| 📊 市场行情 | `/market` | 市场快讯/全球股指/资金流/龙虎榜（精简子Tab到 6 个） |
| 📉 K线分析 | `/kline-analysis` | 独立K线分析页（保留） |
| 🧪 回测验证 | `/backtest` | 回测面板（保留） |
| 🔍 每日选股 | `/daily-pick` | 选股面板（保留） |
| 📁 数据管理 | `/data-manager` | 数据管理（保留） |
| 💰 基金自选 | `/fund` | 基金（保留） |
| 🛢️ 大宗商品 | `/commodity` | 商品（保留） |
| 📰 投资资讯 | `/news` | 新闻（保留） |
| 🤖 AI智能体 | `/agent` | Agent聊天（保留） |
| — — | — | **从研究中心拆出的独立页面** ↓ |
| 📋 AI分析报告 | `/research/reports` | 研究报告（从研究中心拆出） |
| 🎯 股票推荐 | `/research/recommends` | AI推荐记录（从研究中心拆出） |
| ⚡ 异动监控 | `/research/changes` | 异动监控（从研究中心拆出） |
| 🚀 涨停梯队 | `/research/uplimit` | 涨停梯队（从研究中心拆出） |
| 🔧 定时任务 | `/system/cron` | Cron任务（从研究中心拆出） |
| 📔 交易日志 | `/system/trading` | 交易日志（从研究中心拆出，去beta标签） |
| 🔌 MCP服务 | `/system/mcp` | MCP管理（从研究中心拆出） |
| ⚡ 技能管理 | `/system/skills` | 技能管理（从研究中心拆出） |
| 📝 提示词 | `/system/prompts` | 提示词管理（从研究中心拆出） |
| 🎯 指标选股 | `/analysis/screening` | 形态选股/指标选股（从研究中心拆出） |
| ⚙️ 设置 | `/settings` | 设置（保留） |

**变更要点**：
- 消除"研究中心"这个超载入口
- Cron/交易日志/MCP/技能/提示词 归入"系统工具"分组
- 选股工具 归入"分析工具"分组
- 删除提示词广场和问答广场

---

### 5.5 大宗商品 AI 专家路由架构 ✅ 已完成

> **实施日期**: 2026-08-03
> **涉及文件**: `backend/agent/commodity/` 全部 + `backend/data/fred_api.go` + `frontend/src/components/CommodityAnalysis.vue`

#### 5.5.1 架构概览

将大宗商品 AI 分析从 **"5 通用专家无条件运行"** 重构为 **"3 通用 + 品种专属路由"** 架构：

```
Phase 1: 通用分析层 (3 Expert, 始终运行)
  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
  │  Technical   │ │   Macro      │ │  Sentiment   │
  │  Expert      │ │   Expert     │ │  Expert      │
  │  (通用技术面) │ │  (通用宏观)   │ │ (通用新闻情绪)│
  └──────────────┘ └──────────────┘ └──────────────┘

Phase 2: 品种专属层 (CategoryExpert, 按 CommodityContext.Category 路由)
  Category = 贵金属(PreciousMetal):
  ┌──────────────┐ ┌──────────────┐
  │  Monetary    │ │   SafeHaven  │
  │  Expert      │ │   Expert     │
  │(货币属性+ETF)│ │(避险+地缘风险)│
  └──────────────┘ └──────────────┘
  Category = 能源(Energy):
  ┌──────────────┐ ┌──────────────┐
  │  OilSupply  │ │  Geopolitical│
  │  Expert     │ │   Expert    │
  │(OPEC+库存+  │ │(地缘+制裁+  │
  │ COT+新闻)   │ │ 季节性)     │
  └──────────────┘ └──────────────┘
  Category = 基金(Fund):
  ┌──────────────┐
  │  FundTrack  │
  │  Expert     │
  │(跟踪误差+   │
  │ 溢价+量价)  │
  └──────────────┘

Phase 3-5: Bull/Bear Debate → Synthesis → Persist + Emit (不变)
```

#### 5.5.2 核心接口设计

```go
// Expert 基础接口（不变）
type Expert interface {
    Role() string
    Run(ctx context.Context, cc *CommodityContext) (*ExpertReport, error)
}

// CategoryExpert 按 Category 路由的专属专家
type CategoryExpert interface {
    Expert
    Categories() []models.CommodityCategory
}

// 路由函数：通用专家 + 匹配的品种专属专家
func GetExpertsForCategory(cat models.CommodityCategory) []Expert

// 注册机制
func RegisterExpert(e Expert)           // 通用专家
func RegisterCategoryExpert(e CategoryExpert) // 专属专家
```

#### 5.5.3 专家路由矩阵

| Category | 通用专家 | 专属专家 | 总计 | 专属数据源 |
|----------|----------|----------|:----:|------------|
| 贵金属 `CategoryPreciousMetal` | technical + macro + sentiment | monetary + safehaven | 5 | TIPS多期限(5Y/10Y/20Y/30Y) + GLD/SLV ETF流 + CFTC GC/SI + 金银比 + VIX + 经济日历 |
| 能源 `CategoryEnergy` | technical + macro + sentiment | oil_supply + oil_geo | 5 | CFTC CL + WTI-Brent价差(USCL/USCO) + DXY + 经济日历 |
| 基金 `CategoryFund` | technical + macro + sentiment | fund_tracking | 4 | ETF报价 + 国际参考价(GetQuoteIntl) + 60日K线量比 |

#### 5.5.4 已删除的旧专家

| 旧专家 | 删除原因 | 逻辑去向 |
|--------|----------|----------|
| `correlation_expert.go` | 硬编码3个品种代码，不灵活 | 金银比→`monetary_expert`，金油比/WTI-Brent→`oil_supply_expert` |
| `supply_expert.go` | COT仅覆盖GC/SI/CL，品种路由用字符串map | CFTC+新闻→`monetary_expert`(贵金属) 和 `oil_supply_expert`(能源) |

#### 5.5.5 新增数据源

| 数据源 | 方法 | 使用专家 |
|--------|------|----------|
| VIX 恐慌指数 | `fred_api.GetVIX()` (FRED `VIXCLS`) | safe_haven_expert |
| 经济日历 | `GetCalendarReadable(7)` (WallStreetCN) | safe_haven_expert, oil_geopolitical_expert |
| WTI-Brent 价差 | `GetQuote("USCL")` + `GetQuote("USCO")` | oil_supply_expert |
| 国际参考价 | `GetQuoteIntl(code)` | fund_tracking_expert |
| 多期限 TIPS | `GetMacroIndicatorsEnhanced()` | macro_expert, monetary_expert |

#### 5.5.6 DB Prompt Role Key 映射

| Role Key | Expert | 说明 |
|----------|--------|------|
| `commodity_macro` | MacroExpert | 通用宏观（已更新为通用版） |
| `commodity_technical` | TechnicalExpert | 通用技术面 |
| `commodity_sentiment` | SentimentExpert | 通用情绪 |
| `commodity_monetary` | MonetaryExpert | **新增** — 贵金属货币属性 |
| `commodity_safehaven` | SafeHavenExpert | **新增** — 贵金属避险 |
| `commodity_oil_supply` | OilSupplyExpert | **新增** — 能源供需 |
| `commodity_oil_geo` | OilGeopoliticalExpert | **新增** — 能源地缘 |
| `commodity_fund_tracking` | FundTrackingExpert | **新增** — 基金跟踪 |

#### 5.5.7 前端适配

- `CommodityAnalysis.vue` 不再硬编码 `expertOrder`
- 改为从 SSE `agent:token` 事件到达顺序动态展示
- `expertTitles` 扩展到 11 个角色（含 debate/synthesis）
- 快速分析工具（GetCommodityCorrelation 等）不受影响，仍可在数据层独立使用

---

## 六、Port 层接口设计

### 6.1 数据源接口

```go
package datasource

// DataSourceProvider 所有数据源的基础接口
type DataSourceProvider interface {
    Name() string
    Priority() int
    Available(ctx context.Context) bool
}

// QuoteProvider 实时行情
type QuoteProvider interface {
    DataSourceProvider
    GetQuote(ctx context.Context, code string) (*QuoteData, error)
    GetBatchQuote(ctx context.Context, codes []string) ([]QuoteData, error)
}

// KLineProvider K线数据
type KLineProvider interface {
    DataSourceProvider
    GetKLine(ctx context.Context, code string, period KLinePeriod, count int, adjust AdjustType) ([]KLineBar, error)
    GetKLineByDate(ctx context.Context, code string, period KLinePeriod, start, end string, adjust AdjustType) ([]KLineBar, error)
    SupportsPeriod(period KLinePeriod) bool
}

// NewsProvider 新闻
type NewsProvider interface {
    DataSourceProvider
    GetLatestNews(ctx context.Context, count int) ([]NewsItem, error)
    GetNewsByStock(ctx context.Context, code string, count int) ([]NewsItem, error)
    GetNewsBySector(ctx context.Context, sectorID string, count int) ([]NewsItem, error)
}

// FundamentalProvider 基本面
type FundamentalProvider interface {
    DataSourceProvider
    GetFundamental(ctx context.Context, code string) (*FundamentalData, error)
    GetBatchFundamental(ctx context.Context, codes []string) ([]FundamentalData, error)
}

// SectorProvider 板块
type SectorProvider interface {
    DataSourceProvider
    GetSectorData(ctx context.Context, sectorType SectorType) ([]SectorData, error)
    GetSectorFundFlow(ctx context.Context, code string, date string) (*SectorData, error)
    GetSectorTopList(ctx context.Context, sectorType SectorType, topN int) ([]SectorData, error)
}

// CommodityProvider 商品
type CommodityProvider interface {
    DataSourceProvider
    GetCommodityQuote(ctx context.Context, code string) (*CommodityQuote, error)
    GetCommodityKLine(ctx context.Context, code string, period KLinePeriod, count int) ([]KLineBar, error)
}

// FundProvider 基金
type FundProvider interface {
    DataSourceProvider
    GetFundNAV(ctx context.Context, code string) (*FundNAV, error)
    GetFundKLine(ctx context.Context, code string, period KLinePeriod, count int) ([]KLineBar, error)
    GetFundHistoryNAV(ctx context.Context, code string, startDate, endDate string, pageSize int) ([]FundNAV, error)
}

// Router 数据源路由器（带 fallback）
type Router struct { /* ... */ }
func NewRouter() *Router
// Register + Route 方法（GetQuote/GetKLine/GetLatestNews/...）
```

### 6.2 仓储接口

```go
package repository

// PageQuery / PageResult[T] 通用分页

// StockRepository 自选股 + 群组 + 交易记录 + 异动 + 股票信息
type StockRepository interface {
    GetFollowList(ctx, groupId) ([]FollowedStock, error)
    AddFollow(ctx, stockCode, stockName) error
    RemoveFollow(ctx, stockCode) error
    GetGroups(ctx) ([]StockGroup, error)
    AddGroup(ctx, name) (*StockGroup, error)
    RemoveGroup(ctx, groupId) error
    // ... 交易记录、异动、股票信息等
}

// FundRepository 基金
// AnalysisRepository AI响应 + 推荐 + 策略 + 回测 + 选股
// MarketRepository 市场统计 + 板块资金流 + 助手会话
// SystemRepository 配置 + AI配置 + Cron + MCP + 技能 + 提示词 + VIP + 电报缓存
```

### 6.3 通知接口

```go
package notification

type Notifier interface {
    Channel() NotificationChannel
    Send(ctx, msg) error
    Test(ctx) error
    IsConfigured() bool
}

type NotifierManager struct { /* ... */ }
func NewNotifierManager() *NotifierManager
func (m *NotifierManager) Send(ctx, channel, msg) error
func (m *NotifierManager) SendToEnabled(ctx, msg) error
func (m *NotifierManager) TestChannel(ctx, channel) error
```

### 6.4 依赖关系

```
Handler → Service → Port(接口) ← Adapter(实现)
                     ↓
                  Domain(纯模型)
Agent → Port/Datasource（不直接依赖 data 包）
```

---

## 七、股票代码全局统一归一化

### 7.1 问题

系统中存在 7 种股票代码格式，6 个后端转换函数 + 5 个前端独立转换函数，`kline_bars` 表混合存储多种格式。

### 7.2 解决方案

1. **新建 `backend/stockcode/` 归一化包**（零外部依赖）：
   - `ParseStockCode()` — 从任意格式提取 6 位数字 + 交易所
   - `NormalizeStockCode()` — 任意格式 → `sh600519` 内部标准
   - `ToTushare()` / `ToEastMoney()` / `ToTDX()` — 内部标准 → 外部格式
   - `StockCodeCandidates()` — 历史数据兼容查询候选列表

2. **Router 层归一化**：所有 `GetQuote/GetKLine/GetNews/...` 入口加 `stockcode.Normalize()`

3. **数据库迁移**：启动时执行一次性 SQL，将 `kline_bars` 中的裸码和 ts_code 统一为前缀格式

4. **Provider 简化**：Router 保证只传 prefix 格式，Provider 不再需要解析多种输入

5. **前端统一**：新建 `frontend/src/utils/stockCode.js`，替换 8 个组件中的 5 处独立转换函数

### 7.3 实施步骤

| Phase | 内容 | 预计时间 |
|-------|------|----------|
| 1 | 新建 stockcode 包 + 测试 | 1-2 天 |
| 2 | Router 层归一化 + KLineStore 候选查询 | 1-2 天 |
| 3 | 历史数据迁移 SQL | 0.5 天 |
| 4 | 后端旧代码迁移（deprecated 委托） | 1-2 天 |
| 5 | 前端统一（共享工具模块） | 1 天 |
| 6 | 验收 | 0.5 天 |

---

## 八、外部项目功能借鉴

### 8.1 daily_stock_analysis (DSA) 借鉴方案（重点）

> **项目**: `ZhuLinsen/daily_stock_analysis`
> **语言**: Python
> **架构**: 11 步选股流水线 + 4 模式多 Agent 编排
> **核心价值**: DSA 提供了 go-stock 当前最缺乏的能力——**量化评分体系**和**风控叠加层**。

#### D1. 9 因子量化评分系统（P0 — 高价值）

**来源**: `src/services/screening/scorer.py`

go-stock 的选股引擎只有简单的基线评分 + LLM 研究，没有任何多因子量化框架。DSA 提供了完整的 9 因子评分体系，68 个可调参数：

| 因子 | 说明 | 关键参数数 | 公式概要 |
|------|------|:----------:|----------|
| **Value** | 价值因子 | — | PE(0.35权重) + PB(0.65权重) 的百分位排名加权混合 |
| **Liquidity** | 流动性因子 | — | log10(成交额) 百分位排名 |
| **Momentum** | 动量因子 | 12 | 日内(60分起步+涨跌幅×斜率) + 60日趋势(55分起步) + MACD信号；追涨(>5%)/破位(<-20%)/过热(>45%)各有惩罚 |
| **Reversal** | 反转因子 | 7 | 理想跌幅-3%，偏离越远扣分越多；崩盘(<-8%)追加惩罚；RSI超卖(+10)/超买(-14) |
| **Activity** | 活跃度因子 | 8 | 量比(理想值2.0)和换手率(理想值4.0)均使用"理想值距离"公式 |
| **Stability** | 稳定性因子 | 18 | 78分起步，逐项扣减：波动率>45%、最大回撤<-12%、ATR>6%、低质量日线等 |
| **Size** | 市值因子 | — | log10(总市值) 百分位排名 |
| **Theme Heat** | 板块热度因子 | 15 | 多信号综合：趋势分/持续分/降温分/过热分，各有上限和斜率 |
| **Topic Alignment** | 主题匹配因子 | 4 | token集合重叠匹配（候选股票行业/概念 vs 热点主题关键词） |

**默认权重分布**（通过 `tech_weight=0.35` 推导）：

| 因子 | 权重 | 来源 |
|------|:----:|------|
| Value | 0.325 | `(1-0.35) × 0.50` |
| Momentum | 0.193 | `0.35 × 0.55` |
| Activity | 0.158 | `0.35 × 0.45` |
| Liquidity | 0.163 | `(1-0.35) × 0.25` |
| Stability | 0.163 | `(1-0.35) × 0.25` |
| Reversal / Size / Theme / Topic | 0.00 | 默认不启用 |

**与 go-stock 的差异**：go-stock 的 `daily_pick_engine.go` 只有简单的基线评分（基于涨跌/量比/换手等少量指标），没有因子权重配置、没有趋势/反转/稳定性等多维度评估。

**迁移方案**：

```
backend/agent/strategy/scoring/
├── factors.go          # 9 因子接口定义 + FactorResult struct
├── value.go            # Value 价值因子
├── liquidity.go        # Liquidity 流动性因子
├── momentum.go         # Momentum 动量因子
├── reversal.go         # Reversal 反转因子
├── activity.go         # Activity 活跃度因子
├── stability.go        # Stability 稳定性因子
├── size.go             # Size 市值因子
├── theme_heat.go       # ThemeHeat 板块热度因子
├── topic_alignment.go  # TopicAlignment 主题匹配因子
├── scorer.go           # Scorer：因子加权聚合，YAML 策略加载
└── scorer_test.go      # 全因子测试
```

**预估工作量**：3-5 天（可复用 go-stock 现有 KLine 数据和板块热度数据）

---

#### D2. LLM 二次排序（P0 — 高价值）

**来源**: `src/services/screening/ranker.py`

go-stock 没有任何 LLM 重排序能力。DSA 实现了完整的 LLM 排名系统：

**候选池格式化**（每只股票 30+ 字段）：
价格/涨跌幅/成交额/换手率/量比/总市值/PE/PB、行业/概念/行业排名/行业涨跌、板块热度6维评分（最新/趋势/持续/降温/观察数/状态/摘要）、60日涨跌/信号分/MACD状态/RSI状态/突破幅度/振幅/20日量比/K线实体/回踩MA20/盘整天数/波动率/最大回撤/ATR、9因子评分、新闻标题列表、基本面覆盖情况

**结构化 JSON 输出要求**：

```json
{
  "market_view": "整体观点",
  "selection_logic": "选股逻辑",
  "portfolio_risk": "组合风险",
  "ranked": [
    {
      "code": "600519",
      "llm_score": 82,
      "confidence": 0.85,
      "sector": "白酒",
      "theme": "消费复苏",
      "thesis": "核心论点",
      "reason": "推荐理由",
      "risk": "主要风险",
      "catalysts": ["催化剂1"],
      "risk_flags": ["风险标记1"],
      "tags": ["标签1"],
      "style_fit": "风格匹配",
      "watch_items": ["观察项1"],
      "invalidators": ["失效条件1"]
    }
  ]
}
```

**模型链降级**：`[主模型, 备选模型...]` 去重 → 每模型最多 1 次重试 → 覆盖率 ≥ 60% 算成功 → 全部失败时退化为原始 screen_score 排序

**JSON 修复**：尾逗号删除、括号闭合、多 JSON 对象部分恢复

**分数混合**：`final_score = screen_score × 0.60 + llm_score × 0.40`（`rank_weight=0.40`）

**与 go-stock 的差异**：go-stock 的 `AIConfiguredStockPick` 只用 LLM 做单股票的独立分析，没有跨股票对比排序能力。

**迁移方案**：

```
backend/agent/strategy/ranking/
├── ranker.go           # LLM 重排序器（模型链 + 降级 + JSON 修复）
├── prompt.go           # 排序 Prompt 模板（中文，结构化 JSON 输出）
├── formatter.go         # 候选池格式化（30+ 字段 → LLM 可读文本）
├── json_repair.go      # JSON 修复工具
└── ranker_test.go
```

**预估工作量**：2-3 天（复用现有 LLM 调用基础设施）

---

#### D3. 独立风控叠加层（P0 — 高价值）

**来源**: `src/services/screening/risk.py`

go-stock 没有任何选股风控评分。DSA 有 17 项独立风险检查，每项扣分后累加，分级判定风险等级：

| 检查项 | 阈值 | 扣分 | 说明 |
|--------|------|:----:|------|
| 单日追高风险 | 涨幅 ≥ 8% | 4.0 | 防止追高被套 |
| 单日破位风险 | 跌幅 ≤ -7% | 3.5 | 趋势破坏信号 |
| 异常量比 | 量比 ≥ 6.0 | 3.0 | 对倒/异常交易嫌疑 |
| 高换手 | 换手率 ≥ 15% | 3.0 | 短期资金博弈 |
| 无效PE | PE ≤ 0 | 3.0 | 亏损企业 |
| 高PB | PB ≥ 8.0 | 2.0 | 估值过高 |
| 弱日线信号 | signal_score < 45 | 2.5 | 技术面弱势 |
| MACD空头 | bearish | 2.0 | 中期趋势向下 |
| RSI超买 | overbought | 1.5 | 短期超买风险 |
| 低LLM置信度 | confidence < 0.35 | 1.5 | LLM 不确定 |
| LLM风险标记 | 每条 | 1.2（上限4.0） | LLM 识别的风险 |
| 深度分析风险 | 每条 | 1.5（上限4.5） | 多 Agent 识别的风险 |
| 低日线质量 | quality < 70 | 2.0 | 数据质量差 |
| 日线获取失败 | fetch_failed | 6.0 | 严重数据问题 |
| 日线缓存过期 | stale_cache | 2.5 | 数据时效性差 |
| 数据源降级 | fallback_errors | 1.5 | 数据可靠性下降 |
| 异常数据标记 | invalid_ohlc等 | 3.0 | 数据完整性问题 |

**风险分级**：`points ≥ 7.92 → high`，`≥ 3.96 → medium`，否则 `low`（基于 `max_penalty=12.0`）

**组合多样性约束**（7 个板块桶）：

| 板块桶 | 关键词 |
|--------|--------|
| 金融 | 券商, 银行, 保险, 金融 |
| 地产链 | 地产, 房地产, 建材, 家居, 物业 |
| 新能源 | 新能源, 光伏, 锂电, 电池, 储能 |
| AI算力 | AI算力, 算力, 数据中心, 服务器, 光模块 |
| 消费 | 白酒, 食品, 家电, 零售, 消费 |
| 医药 | 医药, 医疗, 创新药 |
| 半导体 | 半导体, 芯片 |

同板块最多 N 只（默认1），超额每只扣 4.0 分。

**迁移方案**：

```
backend/agent/strategy/risk/
├── risk.go             # RiskOverlay：17 项检查 + 分级 + 扣分
├── portfolio.go         # PortfolioDiversity：板块桶匹配 + 集中度惩罚
├── risk_profile.go     # RiskProfile：阈值配置（27 个参数，YAML 加载）
└── risk_test.go
```

**预估工作量**：2-3 天

---

#### D4. 风控否决/降级状态机（P1）

**来源**: `src/agent/risk_override.py`

go-stock 没有风控覆盖能力。DSA 实现了完整的风控否决/降级机制：

- **否决(Veto)**：buy → hold（不允许直接跳到 sell，给用户留余地）
- **降级(Downgrade)**：buy→hold 或 buy→sell（一步 downgrade_one / 两步 downgrade_two）
- **合法转换**：仅 `(buy,hold)` `(buy,sell)` `(hold,sell)` 三种
- **6 个触发来源**：risk_agent 原始数据 / signal_adjustment=="veto" / 任意 high 级 risk_flag / risk_level=="high" 等
- **低敏感度输出**：给 LLM 的上下文中风控标记使用简化描述，避免 LLM 过度反应

**与 go-stock 的差异**：go-stock 的多 Agent 合成阶段会生成 Score/Trend/EntryZone/ExitZone，但没有强制风控覆盖——如果 7 个分析师中有 5 个看多但风控信号极差，合成结果仍然可能是"强烈买入"。

**迁移方案**：在 `backend/agent/multi/synthesis.go` 合成阶段后增加 `riskOverride` 步骤，检查 RiskLevel 并按状态机调整最终信号。

**预估工作量**：2 天

---

#### D5. 5 档决策标尺（P1）

**来源**: `src/schemas/decision_scale.py`

go-stock 没有标准的分数→操作映射。DSA 定义了 5 档标尺：

| 分数区间 | 信号键 | 操作 | 中文标签 |
|:--------:|--------|:----:|:--------:|
| 80-100 | strong_buy | 买入 | 强烈买入 |
| 60-79 | buy | 买入 | 买入 |
| 40-59 | watch | 观望 | 观望 |
| 20-39 | reduce | 卖出 | 减仓 |
| 0-19 | sell | 卖出 | 卖出 |

**护栏机制**：如果分数 ≥ 60 但 action 是 hold/watch（或反过来），必须填写 `guardrail_reason` 说明原因。风控覆盖后信号会钳位到对应区间。

**与 go-stock 的差异**：go-stock 的 `FinalReport` 有 `Score` 字段但没有标准化的分数→操作映射，前端 DecisionDashboard 的展示也没有统一的标尺参考。

**迁移方案**：

```go
// backend/internal/domain/analysis/decision_scale.go
package analysis

type DecisionSignal string
const (
    SignalStrongBuy DecisionSignal = "strong_buy"
    SignalBuy       DecisionSignal = "buy"
    SignalWatch     DecisionSignal = "watch"
    SignalReduce    DecisionSignal = "reduce"
    SignalSell      DecisionSignal = "sell"
)

type DecisionBand struct {
    MinScore float64
    MaxScore float64
    Signal   DecisionSignal
    Action   string  // buy / hold / sell
    LabelZH  string
}

var DecisionScale = []DecisionBand{
    {80, 100, SignalStrongBuy, "buy", "强烈买入"},
    {60, 79,  SignalBuy,       "buy", "买入"},
    {40, 59,  SignalWatch,     "hold", "观望"},
    {20, 39,  SignalReduce,    "sell", "减仓"},
    {0,  19,  SignalSell,      "sell", "卖出"},
}
```

**预估工作量**：0.5 天

---

#### D6. 11 类多 Agent 分歧分类（P1）

**来源**: `src/agent/disagreement.py`

go-stock 的多 Agent 只有简单的多方/空方辩论。DSA 将 Agent 意见分歧归类为 11 种类型：

| 类型 | 含义 | 决策路径提示 |
|------|------|-------------|
| `risk_override` | 风控触发 | 优先执行风控，限制买入信号 |
| `mixed_directional` | 多空混杂 | 综合评估后降级信号 |
| `degraded_only` | 仅有降级输入 | 保守处理，建议观望 |
| `partial_bullish_with_degraded` | 部分看多 + 降级 | 谨慎看多 |
| `partial_bearish_with_degraded` | 部分看空 + 降级 | 谨慎看空 |
| `aligned_bullish` | 全部看多（无中性） | 可确认看多 |
| `bullish_with_neutral` | 看多 + 部分中性 | 看多但需注意 |
| `aligned_bearish` | 全部看空 | 可确认看空 |
| `bearish_with_neutral` | 看空 + 部分中性 | 看空但需注意 |
| `aligned_neutral` | 全部中性 | 观望 |
| `insufficient_opinions` | 无有效意见 | 保守处理 |

**关键设计**：风控 Agent 的看多信号被**强制转为 hold**，防止风控产生看多偏差。

**与 go-stock 的差异**：go-stock 的 `multi/synthesis.go` 直接汇总 7 个分析师报告给 LLM 合成，没有预先分类分歧类型来引导合成策略。

**迁移方案**：在多 Agent 管线中增加 `classifyDisagreement()` 步骤（7 个分析师并行完成后、多空辩论前），根据分歧类型注入不同的合成引导 Prompt。

**预估工作量**：2 天

---

#### D7. 36 项硬过滤器 + 瀑布诊断（P1）

**来源**: `src/services/screening/filter.py`

go-stock 没有可配置的选股硬过滤。DSA 支持 36 个参数的硬过滤：

**快照级过滤（13 项）**：排除ST（正则 `ST|退`）、成交额/价格/市值范围、PE/PB范围、量比/换手下限、涨跌幅范围

**日线级过滤（23 项）**：60日涨跌幅、MA多头排列、价格站上MA20、信号分下限、MACD/RSI状态白名单、20日突破幅度/振幅/量比、K线实体比例、回踩MA20幅度、盘整天数、波动率/最大回撤/ATR

**瀑布诊断**：顺序展示每层过滤淘汰了多少只，附带样本行和自动建议（>90% 被淘汰时告警）。

**迁移方案**：

```
backend/agent/strategy/filter/
├── filter.go            # HardFilter 接口 + 管道执行
├── snapshot_filter.go   # 快照级过滤（13 项）
├── daily_filter.go      # 日线级过滤（23 项）
├── config.go            # HardFilterConfig YAML 配置
├── diagnostic.go        # 瀑布诊断输出
└── filter_test.go
```

**预估工作量**：2-3 天

---

#### D8. 5 阶段热点生命周期追踪（P1）

**来源**: `src/services/screening/hotspot.py`

go-stock 没有热点生命周期管理。DSA 定义了 5 个阶段：

```
初次异动 → 确认扩散 → 加速主升 → 分歧放量 → 降温退潮
```

每阶段有明确的分类条件（结合 state、趋势分、降温分、持续分、最新分、观察次数）。热点中的股票还分配角色：

| 角色 | 条件 |
|------|------|
| **核心龙头** | 排名第1 且分≥70，或排名前3 且分≥max(68, top-8) 且涨幅≥5% |
| **助攻** | 分≥62 且 涨幅≥3% |
| **补涨** | 分≥48 且 涨幅≥0 |
| **后排** | 分≥38 |
| **掉队** | 其余 |

**与 go-stock 的差异**：go-stock 有板块热度数据但没有生命周期阶段分类和龙头/助攻角色分配。

**迁移方案**：可作为 `backend/internal/service/market/hotspot.go`，复用现有的板块资金流和热度数据。

**预估工作量**：3-4 天

---

#### D9. 种子化选股旋转（P2）

**来源**: `src/services/screening/selection_variant.py`

解决"每次运行选出的都是同一批股票"的问题：
- 尾部最多旋转 `output_count // 2` 个位置（保持第一名稳定）
- 旋转池：分数在 cutoff ± 1.5 范围内且完成所有后分析的候选
- 用 `SHA256(seed + period + code)` 确定性排序
- 只改变成员资格，不改变相对排名

**迁移方案**：在 `backend/agent/strategy/scoring/` 下增加 `selection_variant.go`，约 100 行代码。

**预估工作量**：1 天

---

#### D10. 可插拔后分析链（P2）

**来源**: `src/services/screening/post_analysis.py`

3 种分析器串联运行：
- **本地评分卡**：18 个参数，6 个加分条件（价值质量+2.4、资金确认+1.8、控制反转+1.2、高置信度+0.8、催化剂+0.5/条）+ 3 个减分条件（热钱不稳-2.5、量比异常-1.2、低置信度-1.0），±8.0 分上限
- **远程 DSA 服务**：HTTP POST 候选列表，接收分数调整
- **外部 HTTP**：通用远程分析器，POST JSON → 接收分数调整

**迁移方案**：本地评分卡可直接迁移为 Go 实现，远程分析器可选。

**预估工作量**：2-3 天

---

#### D11. 4 模式 Agent 编排（P2）

**来源**: `src/agent/orchestrator.py`

| 模式 | Agent 链 | LLM 调用次数 | 适用场景 |
|------|---------|:-----------:|----------|
| quick | Technical → Decision | ~2 | 快速初步分析 |
| standard | Technical → Intel → Decision | ~3 | 日常分析（默认） |
| full | Technical → Intel → Risk → Decision | ~4 | 重要决策 |
| specialist | Technical → Intel → Risk → [技能Agent×N] → Decision | ~6+ | 深度研究 |

技能 Agent 通过 SkillRouter 延迟插入（Decision 前），最多 4 个并发执行。每阶段有独立超时预算，剩余 < 15s 时跳过而非启动注定超时的阶段。

**与 go-stock 的差异**：go-stock 的多 Agent 管线固定为 7 个分析师 + 辩论 + 合成，没有模式选择、没有预算控制、没有延迟技能插入。

**迁移方案**：重构 `backend/agent/multi/engine.go`，增加模式枚举和预算控制。Phase 3 架构重构时可实施。

**预估工作量**：3-5 天

---

#### D12. 68 字段选股模型（P1）

**来源**: `src/services/screening/models.py`

go-stock 的 `models.DailyPick` 只有约 9 个因子字段。DSA 的 Pick 模型覆盖完整的选股→排名→风控→后分析生命周期：

| 分类 | 字段数 | 示例字段 |
|------|:------:|----------|
| 核心 | 7 | rank, code, name, final_score, screen_score, llm_score, ranking_reason |
| 价格/量 | 6 | price, change_pct, amount, total_mv, turnover_rate, volume_ratio |
| 基本面 | 2 | pe_ratio, pb_ratio |
| 行业/主题 | 11 | industry, concepts, industry_rank, board_heat_score(6维), board_heat_state |
| 技术面 | 15 | change_60d, signal_score, macd_status, rsi_status, breakout_20d_pct, volatility_20d_pct 等 |
| 日线质量 | 3 | daily_quality_score, daily_quality_flags, daily_source |
| 因子评分 | 1 | factor_scores (map[9因子]float) |
| LLM 丰富 | 11 | llm_confidence, llm_sector, llm_theme, llm_thesis, llm_risks, llm_catalysts 等 |
| 风控 | 6 | risk_score, risk_level, risk_penalty, risk_flags, excluded_by_risk |
| 组合 | 2 | portfolio_penalty, portfolio_flags |
| 后分析 | 6 | post_analysis_status, post_analysis_score_deltas 等 |

**迁移方案**：扩展 `backend/models/daily_pick.go`，新增字段。通过 GORM AutoMigrate 自动迁移数据库。

**预估工作量**：1 天

---

### 8.2 DSA 借鉴实施路线图

| 阶段 | 功能 | 预估时间 | 前置依赖 |
|------|------|----------|----------|
| **Phase A（P0）** | D1 9因子评分 + D2 LLM重排序 + D3 风控叠加 | 7-11 天 | 股票代码归一化（第七部分） |
| **Phase B（P1 核心模型）** | D12 扩展 Pick 模型 + D5 决策标尺 | 1.5 天 | Phase A |
| **Phase C（P1 Agent 增强）** | D6 分歧分类 + D4 风控否决 | 4 天 | Phase B |
| **Phase D（P1 选股增强）** | D7 硬过滤器 + D8 热点生命周期 | 5-7 天 | Phase A |
| **Phase E（P2 增强）** | D9 种子旋转 + D10 后分析链 + D11 模式编排 | 6-9 天 | Phase C |

**建议优先级**：先实施 Phase A（P0），显著提升选股质量。Phase B 紧随其后。Phase C-E 在架构重构 Phase 2-3 中同步推进。

---

### 8.3 TradingAgents-CN 借鉴方案（重点）

> **项目**: `hsliuping/TradingAgents-CN`
> **语言**: Python
> **架构**: LangGraph 状态机，9 个 Agent（4 分析师 + 多空研究员 + Trader + 投资裁判 + 风控层）
> **核心价值**: 独立于 DSA 的**定性风控辩论**和**跨会话学习**能力，与 DSA 量化风控互补。

#### T1. 三方风控辩论 + 风控裁判（P0 — 高价值）

**来源**: `tradingagents/graph/setup.py`, `agents/risk_mgmt/`, `agents/managers/risk_manager.py`

go-stock 的多 Agent 管线是 `7 分析师 → 多空辩论 → 合成`。TradingAgents-CN 在此基础上增加了一个**独立的风控辩论层**：

```
4 分析师(并行) → 多空辩论(2轮) → Research Manager(投资裁判) → Trader →
  [Risky → Safe → Neutral (三方循环辩论)] → Risk Judge(可否决 Trader) → END
```

**三个风控角色**：

| 角色 | 文件 | 核心使命 |
|------|------|----------|
| **Risky Analyst** | `aggresive_debator.py` | 主张高收益，挑战保守，推动大胆操作 |
| **Safe Analyst** | `conservative_debator.py` | 保护资产，最小化波动，质疑每个风险点 |
| **Neutral Analyst** | `neutral_debator.py` | 权衡双方，挑战极端，主张平衡/适度 |

**循环辩论机制**：Risky → Safe → Neutral → Risky → ...，每方看到其他两方的最新回复并必须直接回应。最多 3 轮后强制进入 Risk Judge 裁决。

**Risk Judge 否决权**：最终裁决 BUY/SELL/HOLD，可以**否决 Trader 的决策**。Prompt 明确指示："只有在有具体论据强烈支持时才选择持有，而不是在所有方面都似乎有效时作为后备选择"。

**Graph 边定义**（关键代码）：
```python
workflow.add_edge("Trader", "Risky Analyst")
workflow.add_conditional_edges("Risky Analyst",
    should_continue_risk_analysis,
    {"Safe Analyst": "Safe Analyst", "Risk Judge": "Risk Judge"})
workflow.add_conditional_edges("Safe Analyst",
    should_continue_risk_analysis,
    {"Neutral Analyst": "Neutral Analyst", "Risk Judge": "Risk Judge"})
workflow.add_conditional_edges("Neutral Analyst",
    should_continue_risk_analysis,
    {"Risky Analyst": "Risky Analyst", "Risk Judge": "Risk Judge"})
```

**与 DSA 风控的关系**：DSA 的风控是**量化打分**（17 项检查扣分 + 分级），TradingAgents-CN 是**定性辩论**（Agent 间论证），两者互补：
1. DSA 量化风控：快速筛除明显高风险标的（第一阶段）
2. T1 定性风控辩论：对通过的标的进行深度风控论证（第二阶段）

**与 go-stock 的差异**：go-stock 只有 2 方（多/空）辩论，没有独立的风控层，合成阶段无法否决分析师的结论。

**迁移方案**：

```
backend/agent/multi/risk_debate/
├── risk_debate.go        # 风控辩论引擎（三方循环 + 轮次控制）
├── risky_analyst.go      # 激进分析师 Prompt
├── safe_analyst.go        # 保守分析师 Prompt
├── neutral_analyst.go    # 中立分析师 Prompt
├── risk_judge.go         # 风控裁判（LLMTierDeep，可否决）
├── risk_debate_state.go  # 三方历史 + 轮次 + 否决记录
└── risk_debate_test.go
```

在现有管线 `synthesis` 后插入 `riskDebate` 阶段。

**预估工作量**：3-4 天

---

#### T2. 基于 ChromaDB 的反思记忆系统（P0 — 高价值）

**来源**: `agents/utils/memory.py`, `graph/reflection.py`

go-stock **完全没有跨会话学习机制**。TradingAgents-CN 为每个 Agent 建立独立的向量记忆库：

**反思流程**（分析结束后，实际收益已知时触发）：

```python
# 入口：传入实际收益率
ta.reflect_and_remember(1000)  # 1000 = 持仓期间收益率(%)
```

**4 步结构化反思 Prompt**：

| 步骤 | 问题 | 输出 |
|------|------|------|
| **推理评估** | 决策是否正确？哪些因素导致了正确/错误？ | 因果分析 |
| **改进建议** | 如果错误，什么修订能最大化回报？ | 具体改进方向 |
| **经验总结** | 学到了什么？与相似情况的关联？ | 泛化经验 |
| **浓缩查询** | 将经验压缩为单句（<1000 tokens） | 向量检索用 |

**记忆存储**：每个 Agent（bull_researcher / bear_researcher / trader / invest_judge / risk_manager）拥有**独立的 ChromaDB 集合**。

**未来分析时检索**：
```python
# 在每个 Agent 分析前，检索 2 条最相关的历史记忆
if memory is not None:
    past_memories = memory.get_memories(curr_situation, n_matches=2)
    # 注入到 Agent 的 Prompt 中作为历史参考
```

**与 go-stock 的差异**：go-stock 的 `MultiAgentResult` 保存到 SQLite，但从不回溯学习。每次分析都是全新的，Agent 无法从过去的错误中改进。

**迁移方案**（轻量级，无需外部向量数据库）：

```
backend/agent/memory/
├── memory.go             # AgentMemory 接口定义
├── sqlite_memory.go      # SQLite FTS5 实现（初期方案）
├── reflector.go          # Reflector：4 步反思流程
├── prompt.go             # 反思 Prompt 模板
└── memory_test.go
```

- 初期用 SQLite FTS5 全文搜索代替向量数据库（足够满足需求，无需引入新依赖）
- 存储结构：`(agent_role, situation_hash, situation_text, lesson_text, returns_pct, created_at)`
- 检索：FTS5 MATCH + returns_pct 排序，取 top-2 注入 Prompt
- 触发时机：用户手动触发 或 选股引擎跑完后自动对比 N 日收益

**预估工作量**：3-5 天（核心 2 天 + 反思 Prompt 调优 1-3 天）

---

#### T3. 多层降级信号提取（P0 — 高价值）

**来源**: `graph/signal_processing.py`

go-stock 的 `extractStructuredFields` 在 JSON 解析失败时静默失败。TradingAgents-CN 有 4 层降级策略：

| 层级 | 方法 | 说明 |
|:----:|------|------|
| 1 | LLM 直接输出 JSON | 要求 LLM 返回 `{"action", "target_price", "confidence", "risk_score", "reasoning"}` |
| 2 | 正则关键词提取 | 匹配 `买入/卖出/持有/加仓/减仓/观望` |
| 3 | 13 种中文价格模式 | `目标价位：¥123.45`、`上涨到200元`、`第一目标 150` 等 |
| 4 | 智能价格估算 | 基于当前价 × 预期涨跌幅（A股买入默认 ×1.15，卖出默认 ×0.90） |

**13 种价格正则模式**（直接可移植到 Go）：
```python
price_patterns = [
    r'目标价[位格]?[：:]?\s*[¥\$]?(\d+(?:\.\d+)?)',
    r'[¥\$](\d+(?:\.\d+)?)',
    r'(\d+(?:\.\d+)?)元',
    r'上涨[到至]\s*[¥\$]?(\d+(?:\.\d+)?)',
    r'下跌[到至]\s*[¥\$]?(\d+(?:\.\d+)?)',
    r'第一[目目][标标]\s*[：:]?\s*[¥\$]?(\d+(?:\.\d+)?)',
    r'第二[目目][标标]\s*[：:]?\s*[¥\$]?(\d+(?:\.\d+)?)',
    r'阻力[位线]\s*[：:]?\s*[¥\$]?(\d+(?:\.\d+)?)',
    r'支撑[位线]\s*[：:]?\s*[¥\$]?(\d+(?:\.\d+)?)',
    r'空间[到至达]\s*[¥\$]?(\d+(?:\.\d+)?)',
    r'看到\s*[¥\$]?(\d+(?:\.\d+)?)',
    r'预期[高到]\s*[¥\$]?(\d+(?:\.\d+)?)',
    r'止损[位线]\s*[：:]?\s*[¥\$]?(\d+(?:\.\d+)?)',
]
```

**智能价格估算**（当所有模式都匹配不到时）：
```python
def _smart_price_estimation(text, action, current_price, is_china):
    if not current_price:
        return None
    if action == "买入":
        multiplier = 1.15 if is_china else 1.12  # A股涨停 10% + 5% 余量
    elif action == "卖出":
        multiplier = 0.90 if is_china else 0.92
    else:
        return None
    return round(current_price * multiplier, 2)
```

**与 go-stock 的差异**：go-stock 的合成阶段 `extractStructuredFields` 仅用正则尝试匹配 JSON 块，失败时字段为零值。没有关键词提取、没有价格模式库、没有智能估算。

**迁移方案**：

```go
// backend/agent/multi/signal/extractor.go
type SignalExtractor struct {
    pricePatterns []*regexp.Regexp  // 13 种中文价格模式
}

func (e *SignalExtractor) Extract(text string, currentPrice float64, market string) *StructuredSignal {
    // Layer 1: JSON 解析
    if sig := e.tryJSON(text); sig != nil {
        return sig
    }
    // Layer 2: 关键词提取 action
    action := e.extractAction(text)
    // Layer 3: 价格模式匹配
    price := e.extractPrice(text)
    // Layer 4: 智能估算
    if price == 0 && currentPrice > 0 {
        price = e.smartEstimate(action, currentPrice, market)
    }
    return &StructuredSignal{Action: action, TargetPrice: price}
}
```

**预估工作量**：1-2 天

---

#### T4. 双层 LLM 分配策略（P1）

**来源**: `graph/trading_graph.py`, `graph/setup.py`

TradingAgents-CN 将 9 个 Agent 精确分配到两种 LLM 层级：

| 层级 | 模型特征 | 分配角色 | 占比 |
|------|---------|---------|:----:|
| **Quick Think** | 便宜、快速、max_tokens=4000 | 4 分析师 + 多空研究员 + 3 风控辩论者 + Trader | 8/9 (89%) |
| **Deep Think** | 贵、深入、max_tokens=4000 | Research Manager (投资裁判) + Risk Manager (风控裁判) | 2/9 (11%) |

支持**跨供应商混用**（如 Qwen-72B 做 Quick，DeepSeek-V3 做 Deep）。

**与 go-stock 的差异**：go-stock 有 `LLMTierQuick/Deep` 但未精细到角色。当前 `callResearcher`（多空辩论研究员）使用 `LLMTierDeep`，而实际上辩论研究员更适合用 Quick 层。

**迁移方案**：在 `model_config.go` 中明确角色→层级映射表，确保只有**合成**和**风控裁判**用 Deep 层。

**预估工作量**：0.5 天

---

#### T5. 数据完整性预检器（P1）

**来源**: `dataflows/data_completeness_checker.py`

分析启动前的数据质量验证：

| 检查项 | 条件 | 处理 |
|--------|------|------|
| 数据是否为空 | len(data) == 0 | 阻止分析 |
| 是否可解析 | parse error | 阻止分析 |
| 是否包含最新交易日 | latest_date < today - 1 | 告警/重取 |
| 是否有 >3 天缺口 | gap > 3 days | 告警/补充 |
| 完整率 | completeness_ratio < 0.5 | 阻止分析 |

**与 go-stock 的差异**：go-stock 的分析师在数据不足时仍然产出"自信"的分析结果。没有预检机制。

**迁移方案**：在 `backend/agent/multi/engine.go` 启动并行 goroutine 前增加 `checkDataCompleteness()` 调用。

**预估工作量**：1-2 天

---

#### T6. 交易标的上下文注入（P1）

**来源**: `agents/utils/instrument_utils.py`

向所有决策 Agent 注入市场特定的交易规则：

| 字段 | A股示例 | 美股示例 |
|------|--------|----------|
| 市场类型 | A-share | US |
| 交易时间 | 09:30-15:00 | 09:30-16:00 ET |
| 涨跌停 | ±10%/±20% | 无 |
| 手数 | 100 股/手 | 1 股 |
| 结算规则 | T+1 | T+2 |
| 货币 | CNY (¥) | USD ($) |

Trader Prompt 注入示例：
```
⚠️ 重要提醒：当前分析的股票代码是 {company_name}，请使用正确的货币单位：{currency}（{currency_symbol}）
```

**与 go-stock 的差异**：go-stock 支持 A/HK/US 三个市场，但 Agent Prompt 中没有注入这些规则，LLM 可能产生不切实际的价格建议（如建议 A 股日内回转 T+0）。

**迁移方案**：在 `domain/stock/` 增加 `InstrumentContext` struct，根据股票代码前缀（sh/sz/hk/us）自动填充，注入到所有分析师 Prompt。

**预估工作量**：1 天

---

#### T7. 每分析师工具调用计数器防死循环（P2）

**来源**: `graph/conditional_logic.py`, `agents/utils/agent_states.py`

每个分析师有独立的 `tool_call_count` 计数器：

```python
class AgentState(TypedDict):
    market_tool_call_count: int      # 市场分析师
    news_tool_call_count: int        # 新闻分析师
    sentiment_tool_call_count: int   # 情绪分析师
    fundamentals_tool_call_count: int # 基本面分析师
```

达到上限（默认 3 次）时**强制退出**，避免 LLM 陷入工具调用死循环导致 API 成本失控。

**与 go-stock 的差异**：go-stock 的每个分析师通过 Eino Graph 执行，有一定的重试机制但没有显式的工具调用上限。

**迁移方案**：在 `MultiAgentEngine` 的 context 中增加 per-analyst 计数器，达到上限时中断该分析师的 tool loop。

**预估工作量**：0.5 天

---

#### T8. 自适应三级缓存（P2 — 参考）

**来源**: `dataflows/cache/adaptive.py`

Redis → MongoDB → File 三级降级，按市场设置不同 TTL，MD5 键，自动过期清理，统计命中率。

**与 go-stock 的差异**：go-stock 已有 `backend/data/cache/`（freecache 内存缓存）和 KLineStore（SQLite 持久化），架构差异较大。仅参考其按市场类型设置不同 TTL 和缓存统计的设计思路。

**预估工作量**：不需要完整迁移，仅需在现有缓存层增加 TTL 配置和命中率统计。

---

### 8.4 TradingAgents-CN 借鉴实施路线图

| 阶段 | 功能 | 预估时间 | 前置依赖 |
|------|------|----------|----------|
| **Phase A（P0）** | T1 三方风控辩论 + T3 多层信号提取 | 4-6 天 | Phase 3 架构迁移 |
| **Phase B（P0）** | T2 反思记忆系统 | 3-5 天 | 分析结果持久化 |
| **Phase C（P1）** | T4 LLM 分层优化 + T5 数据预检 + T6 上下文注入 | 2.5-4.5 天 | Phase A |
| **Phase D（P2）** | T7 工具调用计数 + T8 缓存增强 | 1-2 天 | 随时可实施 |

**与 DSA 路线图的关系**：
- DSA Phase A（量化评分+风控）→ 独立实施，无依赖
- TACN Phase A（定性风控辩论）→ 在 DSA Phase A 之后实施（先量化筛除，再定性辩论）
- 两者共同构成 **双层风控体系**（量化 + 定性）

---

### 8.5 其他外部项目借鉴清单

| # | 功能 | 来源项目 | go-stock 现状 | 优先级 |
|---|------|----------|-------------|--------|
| F1 | Quality Gate（质量门控：过滤低信心分析） | TradingAgents-CN | ❌ 缺失（→ 见 T5 数据预检） | **P0** |
| F2 | 决策日志 + 预测追踪 | TradingAgents-CN | ❌ 缺失（→ 见 T2 反思系统） | **P0** |
| F3 | Checkpoint/Resume（分析断点续传） | TradingAgents | ❌ 缺失 | **P0** |
| F6 | 9 个额外技术指标（OBV/CCI/Williams/MFI等） | stock-sdk | ❌ 缺失 | P1 |
| F7 | 信号系统（买入/卖出信号评分） | stock-sdk | ❌ 缺失 | P1 |
| F8 | 连板筛选器 | daily_stock_analysis | ❌ 缺失（→ 见 D8 热点生命周期） | P1 |
| F19 | 代码归一化层 | 多个项目 | ✅ 已在第七部分完成设计 | P1 |
| F9-F14 | 新数据源（Tushare/同花绣/Choice等） | QuantDinger 等 | ⚠️ 部分已有 | P2 |
| F15-F20 | 增强功能（回测可视化/MCP网关等） | 多个项目 | ❌ 缺失 | P2 |

> **注**：原 F1/F2 已分别展开为 T5（数据预检）和 T2（反思记忆）；原 F4/F5 已展开为 T1（三方风控辩论）；原 F8 已在 D8（热点生命周期）中覆盖。F3（Checkpoint/Resume）暂未深入分析，保留为待评估项。

---

## 九、后端文件迁移映射

### 9.1 Handler 方法映射

| Handler | 方法数 | 原 App 方法数 | 说明 |
|---------|--------|-------------|------|
| StockHandler | 66 | 66 | 股票/群组/交易记录/异动/搜索 |
| SystemHandler | 51 | 51 | 配置/Cron/MCP/技能/提示词/VIP |
| MarketHandler | 34 | 34 | 市场数据/资金流/新闻/统计 |
| AgentHandler | 29 | 29 | AI对话/分析/推荐/模型配置 |
| AnalysisHandler | 31 | 6 App + 11 Backtest + 13 DailyPick + 1 DailyPickBacktest | 回测/选股/策略 |
| CommodityHandler | 10 | 10 | 商品数据/AI分析 |
| FundHandler | 10 | 10 | 基金关注/K线/排行 |
| NewsHandler | 4 | 4 | 新闻/电报 |
| NotificationHandler | 3 | 3 | 通知发送 |

### 9.2 主要文件迁移

| 源文件（backend/data/） | 目标路径 |
|--------------------------|----------|
| `eastmoney_api.go` (1274行) | `adapter/datasource/eastmoney/quote.go` |
| `eastmoney_kline_api.go` (704行) | `adapter/datasource/eastmoney/kline.go` |
| `f10_data_api.go` (798行) | `adapter/datasource/eastmoney/fundamental.go` |
| `fund_data_api.go` (2185行) | `adapter/datasource/eastmoney/fund.go` |
| `sina_kline_api.go` (790行) | `adapter/datasource/sina/kline.go` |
| `tdx_kline_api.go` (1033行) | `adapter/datasource/tdx/kline.go` |
| `stock_data_api.go` (3066行) | 拆分到 `service/stock/` + `adapter/repository/sqlite/` |
| `tools.go` (2760行) | 拆分到 `agent/tools/*.go` |
| `data_tools_wrapper.go` (5292行) | 拆分为 10 个 tool 文件 |
| `utils.go` (733KB) | 拆分到 `util/` + `stockcode/` |

### 9.3 DI 组装

```
main.go NewApp()
├── router := datasource.NewRouter()
│   └── eastmoney.Register / sina.Register / tdx.Register / ...
├── repos := sqlite.New*(db)
├── notifierMgr := notification.NewManager()
├── services := New*(repos, router, notifierMgr)
└── handlers := New*(services)
    └── App{stockHandler, fundHandler, ...}
```

---

## 十、前端迁移方案

### 10.1 组件拆分

| 超大组件 | 当前行数 | 拆分方案 |
|----------|----------|----------|
| `StockLightweightKlineChart.vue` | 4832 | → `KLineChart.vue` + `KLineToolbar.vue` + `KLineIndicators/` + `KLineOverlays.vue` |
| `stock.vue` | 3151 | → `StockWatchList.vue` + `StockDetail.vue` + `StockAIChat.vue` + `StockSettings.vue` |
| `FloatingAgentAssistant.vue` | 1954 | → `AgentDrawer.vue` + `AgentSession.vue` + `AgentInput.vue` |
| `AnalyzeMartket.vue` | 2032 | → `MarketStats.vue` + `ChangeRanking.vue` + `BullBearChart.vue` |

### 10.2 技术升级

| 项目 | 当前 | 目标 |
|------|------|------|
| 状态管理 | 无（localStorage + props） | Pinia stores |
| 类型安全 | 全 JS | TypeScript（渐进式） |
| API 调用 | 直接 import wailsjs | `api/` 层封装 |
| 复用逻辑 | 内联在各组件 | `composables/` |
| 图表库 | ECharts + Lightweight Charts | 统一 Lightweight Charts |

---

## 十一、迁移路线图

### Phase 0: 大宗商品 AI 专家路由重构 ✅ 已完成（2026-08-03）

> 已在当前分支 `feat/multi-agent-analysis` 上独立实施，不依赖其他 Phase。

- [x] `CommodityContext` 新增 `Category`/`AssetType` 字段
- [x] 新增 `CategoryExpert` 接口 + `RegisterCategoryExpert()` + `GetExpertsForCategory()` 路由
- [x] `MacroExpert` 通用化（TIPS 多期限 + 分类分析提示）
- [x] 新建 5 个品种专属专家（monetary / safehaven / oil_supply / oil_geo / fund_tracking）
- [x] 删除旧 `correlation_expert.go` + `supply_expert.go`（逻辑已拆入专属专家）
- [x] `CommodityEngine` 注入 asset 元数据 + 动态专家选择
- [x] `Synthesis` 动态适配专家数量
- [x] `fred_api.go` 新增 `GetVIX()` 便捷方法
- [x] `CommodityAnalysis.vue` 动态专家展示（SSE 事件驱动顺序）
- [x] **验收**: `go build` 通过，贵金属/能源/基金路由正确，前端无 stale 引用
- **详见**: §5.5 大宗商品 AI 专家路由架构

### Phase 1: 基础设施（1-2 周）✅ 已完成（2026-08-05）

- [x] 新建 `backend/stockcode/` 归一化包
- [x] 新建 `backend/internal/domain/` 领域模型
- [x] 新建 `backend/internal/port/` 接口定义
- [x] 新建 `backend/handler/` 框架
- [x] 新建 `frontend/src/utils/stockCode.js`
- [x] 删除确认废弃的 6 个组件（7c7f904 删除组件文件；2026-08-05 清除提示词广场全部残留）
- [x] **验收**: `go build` 通过，删除功能不影响启动

### Phase 2: 股票核心模块（2-3 周）🟡 大部分完成（2026-08-05）

- [x] Router 层归一化
- [x] KLineStore 候选查询 + 历史数据迁移（代码归一化完成）
- [x] 数据源适配器迁入 `adapter/datasource/`（以"包装 data + 显式映射"方式落地：Router + 5 K 线源 fallback 链 + 腾讯行情；未做物理搬迁，见长尾项）
- [x] 股票 Service 层建立（trading/stockchange/fund/analysis/news/system/market 7 个切片；stock 域读路径留 handler 直连）
- [x] StockHandler 拆分
- [ ] **验收**: 股票自选/K线/搜索功能正常（待 Wails 实测）

### Phase 3: Agent 与分析（2-3 周）🟡 大部分完成（2026-08-05）

- [x] Agent tools 拆分（实际为 `tools.go` 2,760 行 → 581 行 + 10 个域文件，94 个工具字节级一致；`data_tools_wrapper.go` 已不存在）
- [ ] Agent 通过 port 接口获取数据（未做：Agent 仍直连 data 包；datasource Router 已备好供后续切换）
- [ ] 回测/选股 Service 层建立（未做：选股引擎增强直接在 data 层接线，见 Phase 6）
- [x] AnalysisHandler 拆分
- [x] **T3 多层降级信号提取**（13 种中文价格模式 + 智能估算）— 前期已完成
- [x] **T4 LLM 双层分配优化**（Quick/Deep 精确到角色）— 前期已完成
- [ ] **T5 数据完整性预检器**（未做）
- [ ] **T6 交易标的上下文注入**（未做）
- [ ] **T7 工具调用计数器**（未做）
- [ ] **验收**: AI分析/回测/选股功能正常（待 Wails 实测）

### Phase 4: 前端重构（2-3 周）✅ 完成（2026-08-05）

- [x] 引入 Pinia，建立 stores/
- [x] 建立 api/ 层（已迁移 TypeScript，并切换到 handler 命名空间）
- [x] 拆分超大组件（StockLightweightKlineChart 4,832→1,372 / stock.vue 3,121→1,442 / FloatingAgentAssistant 1,954→862；删除弹幕功能）
- [x] 前端导航重构（配置与 composable 已建；研究中心 11 子 Tab 已拆分为 /research、/analysis、/system 独立路由，2026-08-28 落地）
- [x] 渐进 TypeScript
- [ ] **验收**: 所有页面功能正常（待 Wails 实测）

### Phase 5: 剩余模块 + 清理（1-2 周）✅ 完成（2026-08-05）

- [x] Market/Fund/Commodity/News/System Handler 拆分（11 handler + trading/stockchange 共 13 个，Wails 直连绑定）
- [x] `app.go` 缩减到 < 200 行（3,488 → **184 行**；生命周期拆至 app_lifecycle/app_monitor/app_tradingtime）
- [ ] `backend/data/` 包清空（未做：适配器以包装方式落地，物理搬迁留作长尾）
- [x] VIP 策略调整（**实际执行为完全移除**：全部功能免费，EffectiveSponsorVipLevel 恒返回 (2,true)，267af56）
- [ ] **验收**: 全量功能测试（待 Wails 实测）

### Phase 6: DSA 量化选股增强（2-3 周）✅ 已完成并接入生产（2026-08-05）

- [x] **D1** 9 因子量化评分系统（`strategy/scoring/`，JSON 配置代替 YAML）
- [x] **D2** LLM 二次排序（30+ 字段候选池，模型链降级）— 接入选股管线（无 AI 配置静默跳过）
- [x] **D3** 独立风控叠加层（17 项检查 + 组合多样性）— 接入选股管线（标记不剔除）
- [x] **D12** 扩展 Pick 模型（实际 +62 字段，camelCase，AutoMigrate）
- [x] **D5** 5 档决策标尺 — 接入合成输出 + 前端 DecisionScaleBar
- [x] **验收**: 选股引擎产出量化评分 + 风控评级（单测全绿；待 Wails 实测）

### Phase 7: Agent 风控辩论 + 学习系统（2-3 周）✅ 已完成并接入生产（2026-08-05）

- [x] **T1** 三方风控辩论（Risky/Safe/Neutral + Risk Judge 否决权）— 接入 multi 引擎 full/specialist 模式
- [x] **D4** 风控否决/降级状态机（buy→hold veto）— 接入合成后钳位
- [x] **D6** 11 类分歧分类（引导合成策略）— 接入合成 Prompt 引导
- [x] **T2** 反思记忆系统（SQLite FTS5 + LIKE 降级）— 接入分析师 Prompt 注入 + `ReflectOnAnalysis` 绑定
- [x] **验收**: 分析结果包含风控辩论记录 + 历史记忆引用（单测全绿；待 Wails 实测）

### Phase 8: 选股高级功能（2-3 周）✅ 已完成并接入生产（2026-08-05）

- [x] **D7** 硬过滤器 + 瀑布诊断（实际 38 参数）— 接入选股管线候选池阶段
- [x] **D8** 5 阶段热点生命周期 + 角色（龙头/助攻/补涨）
- [x] **D9** 种子化选股旋转（SHA256 确定性轮换）— 已接线，默认关闭
- [x] **D10** 可插拔后分析链（本地评分卡 + 远程 HTTP）— 接入 FinalScore
- [x] **D11** 4 模式 Agent 编排（quick/standard/full/specialist；standard=原管线逐字一致，`agentMode` 参数生效）
- [x] **验收**: 选股引擎全流水线（筛选→评分→排序→风控→旋转）（单测全绿；待 Wails 实测）

---

## 十二、预期收益

| 维度 | 当前 | 重构后 |
|------|------|--------|
| 最大文件行数 | 5292 行 | < 500 行 |
| 后端包文件数 | 126 文件/包 | < 15 文件/包 |
| app.go 行数 | 3488 行 | < 200 行 |
| 全局变量 | 大量 | 基本消除 |
| 前端最大组件 | 4832 行 | < 500 行 |
| 状态管理 | 分散 | Pinia 集中 |
| 数据获取 | 新旧并存 | 统一 Router |
| 股票代码格式 | 7 种混用 | 1 种（sh600519） |
| 转换函数 | 11 个独立函数 | 1 个 stockcode 包 |
| 前端导航 | 12 项（研究中心 13 子Tab） | ~20 项（扁平化） |
| VIP 检查 | 不统一 | 统一 GetEffectiveSponsorVip |
| 可测试性 | 困难 | DI + 接口 mock |
| **商品AI分析** | **5 通用专家无差异化** | **3通用+N专属路由（按品种分类）** ✅ |

---

## 十三、风险评估

| 风险 | 级别 | 缓解措施 |
|------|------|----------|
| 渐进迁移过程中新旧并存 | 中 | 旧包保留兼容别名，逐步 deprecate |
| Router 归一化破坏数据获取 | 中 | 归一化对未知格式原样返回 |
| 数据库迁移丢失数据 | 中 | 迁移 SQL 幂等，执行前备份 |
| 前端拆分引入回归 | 中 | 每个 Phase 结束手动验证 |
| 循环依赖 | 低 | 严格单向: Domain ← Port ← Adapter → Service → Handler |
| VIP 策略变更影响收入 | 低 | 核心功能免费反而增加用户留存 |
