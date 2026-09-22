# go-stock 开发全局约定（AI 辅助开发必读）

本文件是 AI 辅助开发的全局指令。**改代码前必须遵守以下约定**，这些规则全部来自真实事故，
违反过一次并造成了线上 bug，不允许再犯。

---

## 规则 1：禁止用零值结构体调用带资源的方法（血案：关注股票全挂）

**事故**：分层重构时 sqlite 适配层写了 `(data.StockDataApi{}).Follow(...)`，
零值结构体的 `client` 字段是 nil，`Follow` 内部拉实时行情时 `receiver.client.R()` 直接
nil pointer panic。前端表现为"点关注没任何反应"，且**取消关注/设置成本价/止盈止损/
报警设置/排序等 7 个接口全部静默崩溃**，而 `GetFollowList` 正常（只查库不发 HTTP）——
故障极难从表象定位，最后是靠 wails.log 里的 panic 才抓到（提交 ffa5645）。

**规则**：
- 任何携带资源（HTTP client、DB 句柄、配置）的结构体，**必须通过构造函数获取实例**，
  禁止 `SomeStruct{}` 零值直接调方法。
- go-stock 里统一用 `data.NewStockDataApi()`，禁止手写 `data.StockDataApi{}`。
- **代码评审/重构时的检查清单**：把旧代码搬到新分层（handler→service→adapter）时，
  逐行核对原代码的接收者是怎么构造的；`grep "Api{}"` 应当为空。
- 重构涉及网络/DB 调用的方法后，**必须实际调用一次验证**（编译通过 ≠ 能跑），
  只查库的方法正常不代表同文件里发 HTTP 的方法正常。

## 规则 2：时间一律走 timeutil，禁止裸 time.Parse

- 解析外部时间用 `backend/util/timeutil`（`ParseDate/ParseDateTime/MustParseDateTime`），
  它们按本地时区解析。裸 `time.Parse` 得到 UTC，与本地存储比较会产生最多 8h 偏移。
- 业务日期统一 `"2006-01-02"` 本地字符串（字典序可安全比较）：`timeutil.DateLayout` /
  `TodayStr()` / `DateStr()`。禁止 "2006/01/02"、"20060102" 变体进入比较链路。

## 规则 3：股票代码一律走 stockcode，外部格式过转换层

- 内部唯一格式：`sh600519 / sz000001 / bj430047 / hk00700 / usAAPL`，
  转换用 `backend/stockcode`（`Normalize` / `ToTushare` / `ToEastMoney` / `ToSina` …）。
- 外部数据源（东财 secid、tushare ts_code、腾讯、新浪）的代码进入系统前先 `Normalize`，
  出系统前用对应 To 转换。禁止在业务代码里手写前缀拼接/首位数字猜测。
- 前端对应 `frontend/src/utils/stockCode.js`（`normalizeStockCode/resolveFollowCode`）。

## 规则 4：wails 绑定是手工维护的

构建用 `wails build -s -m -skipbindings`，**不会**重新生成前端绑定。
后端 handler/service 新增或修改方法签名后，必须同步手工更新：
`frontend/wailsjs/go/handler/*.js`（及对应 .d.ts），否则运行时 `undefined` 报错。

## 规则 5：构建与部署口径

- 后端：`wails.exe build -s -m -skipbindings`（PATH 需加 `D:\go\bin;C:\Users\54389\go\bin`，
  `GOCACHE=E:\open-source\ai\go-stock\.gocache`）。
- 前端：`pnpm run build` 的 precheck 已坏，用
  `node node_modules\vite\bin\vite.js build`（frontend/ 下，约 2 分钟）。
- 测试：全量 `go test ./backend/data/` 会因网络用例超时，跑定向 `-run` 子集。
- 部署：停 go-stock 进程后拷 `build\bin\go-stock.exe` → `D:\_installers\go-stock\`。
- 改完必做：`go build ./... && go vet ./backend/...` + 定向测试，然后构建、部署、提交。

## 规则 6：已知坑位速查

- `hq.sinajs.cn` 的 list 为空时返回 200 + 空内容，不是错误；
  北交所（bj）代码不在新浪/腾讯行情分支里，走第三方前先确认市场归属。
- TradingView/sina/fincalapi 的网络报错是长期噪音，排查问题先过滤掉它们再找业务日志。
- 前端运行时错误记录在 `logs/info.log` 的 `[FRONTEND]` 行；
  wails 绑定调用失败（panic/参数不匹配）在 `logs/wails.log`——
  **"界面点了没反应" 先看这两个文件**，不要先怀疑前端。
- glebarez/sqlite 把 time.Time 存为本地时区 TEXT；kline_bars 表内代码格式混杂
  （裸/带前缀/ts_code），查询用 StockCodeCandidates 容错，禁止盲目迁移。
