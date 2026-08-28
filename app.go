package main

import (
	"context"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/handler"
	"sync"
	"time"

	"github.com/coocood/freecache"
	"github.com/robfig/cron/v3"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx                 context.Context
	cache               *freecache.Cache
	cron                *cron.Cron
	cronEntrys          map[string]cron.EntryID
	cronEntrysMu        sync.Mutex
	AiTools             []data.Tool
	stockAlertMu        sync.Mutex
	stockAlertLastSent  map[string]time.Time
	priceAtAlertReset   map[string]float64
	notificationHandler *handler.NotificationHandler
	dailyPickHandler    *handler.DailyPickHandler
	fundHandler         *handler.FundHandler
	commodityHandler    *handler.CommodityHandler
	newsHandler         *handler.NewsHandler
	marketHandler       *handler.MarketHandler
	agentHandler        *handler.AgentHandler
	analysisHandler     *handler.AnalysisHandler
	stockHandler        *handler.StockHandler
	systemHandler       *handler.SystemHandler
	tradingHandler      *handler.TradingRecordHandler
	stockChangeHandler  *handler.StockChangeHandler
}

// NewApp creates a new App application struct
func NewApp() *App {
	cacheSize := 512 * 1024
	cache := freecache.NewCache(cacheSize)
	c := cron.New(cron.WithSeconds(), cron.WithChain(cron.Recover(cron.DefaultLogger)))
	c.Start()
	var tools []data.Tool
	tools = data.Tools(tools)
	app := &App{
		cache:              cache,
		cron:               c,
		cronEntrys:         make(map[string]cron.EntryID),
		AiTools:            tools,
		stockAlertLastSent: make(map[string]time.Time),
		priceAtAlertReset:  make(map[string]float64),
	}
	app.notificationHandler = handler.NewNotificationHandler(cache, func() context.Context { return app.ctx })
	app.dailyPickHandler = handler.NewDefaultDailyPickHandler(func() context.Context { return app.ctx })
	app.fundHandler = handler.NewDefaultFundHandler(func() context.Context { return app.ctx })
	app.commodityHandler = handler.NewCommodityHandler()
	app.newsHandler = handler.NewDefaultNewsHandler(func() context.Context { return app.ctx })
	app.marketHandler = handler.NewMarketHandler(cache, func() context.Context { return app.ctx })
	app.agentHandler = handler.NewAgentHandler(func() context.Context { return app.ctx })
	app.analysisHandler = handler.NewDefaultAnalysisHandler(func() context.Context { return app.ctx })
	app.stockHandler = handler.NewStockHandler()
	app.systemHandler = handler.NewSystemHandler(cache, func() context.Context { return app.ctx }, c, Version, VersionCommit, OFFICIAL_STATEMENT, BuildKey, icon, alipay, wxpay, wxgzh, userManual)
	app.tradingHandler = handler.NewDefaultTradingRecordHandler(func() context.Context { return app.ctx })
	app.stockChangeHandler = handler.NewDefaultStockChangeHandler(func() context.Context { return app.ctx })
	return app
}

func (a *App) setCronEntry(key string, id cron.EntryID) {
	a.cronEntrysMu.Lock()
	a.cronEntrys[key] = id
	a.cronEntrysMu.Unlock()
}

func (a *App) getCronEntry(key string) (cron.EntryID, bool) {
	a.cronEntrysMu.Lock()
	id, exists := a.cronEntrys[key]
	a.cronEntrysMu.Unlock()
	return id, exists
}

func (a *App) removeCronEntry(key string) {
	a.cronEntrysMu.Lock()
	delete(a.cronEntrys, key)
	a.cronEntrysMu.Unlock()
}

func (a *App) QuitApp() {
	if a.ctx != nil {
		if a.cron != nil {
			a.cron.Stop()
		}
		runtime.Quit(a.ctx)
	}
}

// domReady is called after front-end resources have been loaded
func (a *App) domReady(ctx context.Context) {
	defer PanicHandler()
	defer func() {
		// 增加延迟确保前端已准备好接收事件
		go func() {
			time.Sleep(2 * time.Second)
			runtime.EventsEmit(a.ctx, "loadingMsg", "done")
		}()
	}()

	if stocksBin != nil && len(stocksBin) > 0 {
		go runtime.EventsEmit(a.ctx, "loadingMsg", "检查A股基础信息...")
		go initStockData(a.ctx)
	}

	if stocksBinHK != nil && len(stocksBinHK) > 0 {
		go runtime.EventsEmit(a.ctx, "loadingMsg", "检查港股基础信息...")
		go initStockDataHK(a.ctx)
	}

	if stocksBinUS != nil && len(stocksBinUS) > 0 {
		go runtime.EventsEmit(a.ctx, "loadingMsg", "检查美股基础信息...")
		go initStockDataUS(a.ctx)
	}
	updateBasicInfo()
	a.registerStartupCronTasks()
}

// shutdown is called at application termination
func (a *App) shutdown(ctx context.Context) {
	defer PanicHandler()
	if freestockdbManager != nil {
		freestockdbManager.Stop()
	}
	// 记录当前窗口大小，供下次启动时还原
	if a.ctx != nil {
		if w, h := runtime.WindowGetSize(a.ctx); w > 0 && h > 0 {
			cfg := data.GetSettingConfig()
			cfg.WindowWidth = w
			cfg.WindowHeight = h
			data.UpdateConfig(cfg)
			//logger.SugaredLogger.Infof("save window size: %dx%d", w, h)
		}
	}
	//logger.SugaredLogger.Infof("application shutdown Version:%s", Version)
}
func onExit(a *App) {
	// 清理操作
	//logger.SugaredLogger.Infof("systray onExit")
	//systray.Quit()
	//runtime.Quit(a.ctx)
}

func (a *App) UpdateConfig(settingConfig *data.SettingConfig) string {
	if settingConfig.RefreshInterval > 0 {
		// 价格监控定时任务仍由 App 侧管理（MonitorStockPrices 含完整报警逻辑），
		// 避免 systemHandler 重复注册其内部的简化版监控。
		if entryID, exists := a.getCronEntry("MonitorStockPrices"); exists {
			a.cron.Remove(entryID)
		}
		id, _ := a.cron.AddFunc(fmt.Sprintf("@every %ds", settingConfig.RefreshInterval), func() {
			MonitorStockPrices(a)
		})
		a.setCronEntry("MonitorStockPrices", id)

		cfg := *settingConfig
		cfg.RefreshInterval = 0
		return a.systemHandler.UpdateConfig(&cfg)
	}

	return a.systemHandler.UpdateConfig(settingConfig)
}

// InitCronTasks 在应用启动时，自动为启用状态的定时任务创建调度
func (a *App) InitCronTasks() {
	a.systemHandler.InitCronTasks()
}

func (a *App) GenerateSkillFromURL(url string) string {
	skill, confidence, err := a.systemHandler.GenerateSkillFromURL(url)
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("技能草稿生成成功（置信度: %.2f），请前往技能管理查看并启用。\n名称: %s\n分类: %s\n描述: %s\n触发关键词: %s",
		confidence, skill.Name, skill.Category, skill.Description, skill.TriggerKeywords)
}
