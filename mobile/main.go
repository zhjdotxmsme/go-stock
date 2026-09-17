// go-stock Android 移动端入口（Wails v3）。
//
// 与桌面端 go-stock 复用同一套 backend 业务代码：mobile 通过 go.mod 的
// replace go-stock => ../ 复用 backend/ 的 handler、data、datasource 与 models。
// 与桌面的唯一差异是事件发射适配——桌面端经 Wails v2 runtime.EventsEmit(ctx) 推事件，
// 这里经 v3 app.Event.Emit 推事件，二者的业务层代码完全一致（backend/emitter 抽象）。
//
// 这里注册的 handler 与桌面 main.go 的 Bind 列表一一对应（14 个业务 handler +
// 2 个回测服务 + 移动端轻量 MobileService），因此手机端具备与桌面相同的功能面：
//   - 行情/K线/指标:  StockHandler + MarketHandler + CommodityHandler(+Tdx*)
//   - AI 分析/智能体:  AgentHandler + AnalysisHandler
//   - 每日选股/复盘:   DailyPickHandler + DailyPickBacktestService + Backtest(Service)
//   - 持仓/自选/组合:  TradingHandler + FundHandler + StockChangeHandler + NewsHandler
//   - 配置/系统:       SystemHandler（配置读写、AI 配置、技能/策略、MCP 等）
// 非核心能力(桌面窗口/托盘/本地定时监控循环)属桌面 App 独占，移动端按需降级。
package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/coocood/freecache"
	cronv3 "github.com/robfig/cron/v3"

	backtestService "go-stock/backend/data/backtest"
	"go-stock/backend/data"
	"go-stock/backend/data/datasource"
	"go-stock/backend/data/datasource/fallback"
	dailyPickBacktestService "go-stock/backend/service"
	"go-stock/backend/db"
	"go-stock/backend/emitter"
	"go-stock/backend/handler"
	"go-stock/backend/machineid"
	"go-stock/backend/models"
)

//go:embed all:frontend/dist
var assets embed.FS

// 版本信息：由 CI/构建注入（与桌面一致：-ldflags -X）。
var (
	Version            = "mobile"
	VersionCommit      = ""
	OFFICIAL_STATEMENT = ""
	BuildKey           = ""
)

// migrate 复刻根 main.go 的 AutoMigrate：mobile 是独立 main 包，拿不到根包的
// AutoMigrate()，若不建表，自选/持仓/每日选股/基金/交易记录等表将缺失，功能面不完整。
func migrate() {
	auto := func(v ...any) { _ = db.Dao.AutoMigrate(v) }
	auto(&data.StockInfo{})
	auto(&data.StockBasic{})
	auto(&data.FollowedStock{})
	auto(&data.IndexBasic{})
	auto(&data.Settings{})
	auto(&models.AIResponseResult{})
	auto(&models.StockInfoHK{})
	auto(&models.StockInfoUS{})
	auto(&data.FollowedFund{})
	auto(&data.FundBasic{})
	auto(&data.FundShareHistory{})
	auto(&models.PromptTemplate{})
	auto(&data.Group{})
	auto(&data.GroupStock{})
	auto(&models.Tags{})
	auto(&models.Telegraph{})
	auto(&models.TelegraphTags{})
	auto(&models.LongTigerRankData{})
	auto(&data.AIConfig{})
	auto(&models.BKDict{})
	auto(&models.WordAnalyze{})
	auto(&models.SentimentResultAnalyze{})
	auto(&models.AiRecommendStocks{})
	auto(&models.AllStockInfo{})
	auto(&models.CronTask{})
	auto(&models.AiAssistantSession{})
	auto(&models.GlobalStockIndex{})
	auto(&data.TradingRecord{})
	auto(&models.MCPServer{})
	auto(&models.MCPServerTool{})
	auto(&models.Skill{})
	auto(&models.SkillUsageRecord{})
	auto(&models.CustomStrategy{})
	auto(&models.BKFundFlow{})
	auto(&models.ConceptFundFlow{})
	auto(&models.KLineBar{})
	auto(&models.KLineSyncLog{})
	auto(&models.AiRecommendBacktest{})
	auto(&models.DailyPick{})
	auto(&models.StockChangeHistory{})
	auto(&models.MarketStatistic{})
	auto(&models.HoldingsDailySummary{})
	auto(&db.ChatMemory{})

	// 复刻根 main.go AutoMigrate 尾部的默认数据种子。
	data.InitDefaultSkills()
	data.InitDefaultMultiAgentPrompts()
}

// initDataSources 复刻根 main.go：注册 datasource Router + 缓存层 + 各数据源的
// 多源降级链（K线/行情/新闻/基本面/板块/快照/免费源）。移动端所有行情数据都经
// Router 快照链（腾讯全字段 → 东财兜底 → 缓存），不做这步 K线/盯盘会退化为单源。
func initDataSources() {
	router := datasource.GetRouter()
	cache := datasource.NewCacheLayer(256)
	cache.AutoMigrate()
	router.SetCache(cache)

	fallback.RegisterQuoteChain(router)
	fallback.RegisterKLineChain(router)
	fallback.RegisterNewsChain(router)
	fallback.RegisterFundamentalChain(router)
	fallback.RegisterSectorChain(router)
	fallback.RegisterSnapshotChain(router)
	fallback.RegisterFreeDataSources(router)
}

// initMCP 复刻根 main.go registerStockSDKMCP：给 AI 智能体提供 stock-sdk 技术指标工具。
func initMCP() {
	_ = db.Dao.Where("name = ?", "stock-sdk").FirstOrCreate(&models.MCPServer{
		Name:        "stock-sdk",
		Description: "Stock data SDK with 14 technical indicators",
		Type:        "stdio",
		Command:     "npx",
		Args:        `["-y","stock-sdk","mcp"]`,
		Enable:      true,
		Status:      "stopped",
	})
}

func main() {
	// Android 上把工作目录切到应用私有目录，使 db.Init("") 的默认相对路径
	// data/stock.db 落在应用沙箱内；桌面(开发模式)StoragePath 为空，行为不变。
	if storage := application.Mobile.StoragePath(); storage != "" {
		if err := os.MkdirAll(filepath.Join(storage, "data"), 0o755); err != nil {
			log.Printf("create data dir failed: %v", err)
		}
		if err := os.Chdir(storage); err != nil {
			log.Printf("chdir to storage failed: %v", err)
		}
	}

	if BuildKey == "" {
		BuildKey = "cc1e0d684e32f176c56ff1fcf384dcd9"
	}
	machineid.Init(BuildKey)
	data.SponsorDecryptKeyHex = BuildKey

	db.Init("") // 打开 SQLite 并建基础表
	migrate()  // 建全量业务表 + 种子默认数据

	// HTTP 客户端/代理/超时按当前配置初始化（行情抓取依赖）。
	data.ConfigureFromSettings(data.GetSettingConfig())
	data.InitAnalyzeSentiment()

	// 行情/新闻/基本面多源数据链（Router + fallback）。
	initDataSources()
	initMCP()

	// v3 事件发射适配：backend 层所有流式/告警事件经此推送到 WebView。
	// application.Get() 在 app.Run 之前为 nil，故在调用期惰性取用。
	emit := emitter.Emitter(func(event string, payload ...any) {
		if app := application.Get(); app != nil {
			app.Event.Emit(event, payload...)
		}
	})
	data.SetEventEmitter(emit)

	// v3 绑定方法自带 context 参数；ctxFn 供尚未接收 v3 ctx 的 handler
	// 内部（HTTP 请求 context / v2 文件对话框兜底）使用，回退到 Background。
	ctxFn := func() context.Context { return context.Background() }

	// 共享缓存 + 定时器（与桌面 NewApp 相同），供 Market/Notification/System handler 使用。
	cache := freecache.NewCache(512 * 1024)
	c := cronv3.New(cronv3.WithSeconds(), cronv3.WithChain(cronv3.Recover(cronv3.DefaultLogger)))
	c.Start()

	// 14 个业务 handler，构造器与桌面 NewApp 完全一致。
	notificationHandler := handler.NewNotificationHandler(cache, ctxFn, emit)
	dailyPickHandler := handler.NewDefaultDailyPickHandler(emit)
	fundHandler := handler.NewDefaultFundHandler(ctxFn)
	commodityHandler := handler.NewCommodityHandler()
	newsHandler := handler.NewDefaultNewsHandler(ctxFn)
	marketHandler := handler.NewMarketHandler(cache, ctxFn)
	agentHandler := handler.NewAgentHandler(ctxFn, emit)
	analysisHandler := handler.NewDefaultAnalysisHandler(ctxFn)
	stockHandler := handler.NewStockHandler()
	systemHandler := handler.NewSystemHandler(cache, ctxFn, emit, c, Version, VersionCommit, OFFICIAL_STATEMENT, BuildKey, nil, nil, nil, nil, nil)
	tradingHandler := handler.NewDefaultTradingRecordHandler(ctxFn, emit)
	stockChangeHandler := handler.NewDefaultStockChangeHandler(ctxFn)

	app := application.New(application.Options{
		Name:        "go-stock",
		Description: "go-stock AI 股票分析 移动端",
		Services: []application.Service{
			application.NewService(analysisHandler),
			application.NewService(stockHandler),
			application.NewService(systemHandler),
			application.NewService(marketHandler),
			application.NewService(agentHandler),
			application.NewService(tradingHandler),
			application.NewService(stockChangeHandler),
			application.NewService(notificationHandler),
			application.NewService(fundHandler),
			application.NewService(commodityHandler),
			application.NewService(newsHandler),
			application.NewService(dailyPickHandler),
			application.NewService(backtestService.NewService()),
			application.NewService(dailyPickBacktestService.NewDailyPickBacktestService()),
			application.NewService(NewMobileService()),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Android: application.AndroidOptions{},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "go-stock",
		BackgroundColour: application.NewRGB(18, 18, 18),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
	fmt.Println("go-stock mobile exited")
}
