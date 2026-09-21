# Kronos K线预测集成 — 实现计划

依据 spec：`2026-09-21-kronos-kline-prediction-design.html`

## M1 Python 推理服务 `python/kronos_service/`
- `app.py`：FastAPI，`GET /health`、`POST /predict`（OHLCV df + 时间戳 + predLen/T/top_p/sampleCount → 预测K线 + summary）、`POST /predict_backtest`
- `requirements.txt`（torch、transformers、fastapi、uvicorn、pandas、huggingface_hub）
- `test_app.py`：固定输入快照断言输出形状（模型缺失时 skip）

## M2 Go 后端
1. `backend/data/settings_api.go`：Settings 增列 `KronosEnable`、`KronosPythonPath`、`KronosPort`、`KronosModel`、`KronosDevice`、`KronosT`、`KronosTopP`、`KronosPredLen`、`KronosSampleCount`、`KronosLazyStart`（AutoMigrate 已保证新列）
2. `backend/data/kronos_service.go`：ProcessManager（启动/心跳/重启一次/退出回收）+ Client（health/predict，30s 超时，结构化错误码）
3. `backend/data/holdings_deep_analysis.go`：`HoldingsDeepStock` 增 `KronosForecast`；`GetHoldingsDeepData` 开关开启时并行预测（失败降级）；`BuildHoldingsDeepPrompt` 注入预测段落
4. `backend/handler/`：新增 `PredictKLine(code, predLen)` handler；`settings_handler` 透传新字段（复用现有 settings 读写）

## M3 前端
1. `frontend/wailsjs` 绑定重新生成（或手写 wrapper 对齐现有 `wailsjs/go/...` 模式）
2. `StockLightweightKlineChart.vue`：工具栏「AI预测」开关 → 调 PredictKLine → 真实K线右侧渲染半透明虚K线 + 分隔线 + 悬浮提示；服务不可用时灰显
3. 设置页（定位现有设置组件）新增「Kronos 预测」分区：开关/Python路径/端口/模型/设备/参数/状态灯/启动停止/一键初始化
4. `HoldingsDeepAnalysisModal.vue`：持仓卡片「Kronos 预测」行

## M4 验证
- Go：`go build ./...`、`go test ./backend/data/ -run Kronos`（client mock、prompt 快照）
- 前端：`pnpm build`（或 vite build）通过
- 手动：无 Python 环境时全部功能回退现状（默认关）
