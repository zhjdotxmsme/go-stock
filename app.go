package main

import (
	"context"
	"fmt"
	"go-stock/backend/agent/strategy"
	"go-stock/backend/data"
	"go-stock/backend/handler"
	"go-stock/backend/models"
	"sync"
	"time"

	"github.com/samber/lo"

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
	app.fundHandler = handler.NewDefaultFundHandler()
	app.commodityHandler = handler.NewCommodityHandler()
	app.newsHandler = handler.NewNewsHandler()
	app.marketHandler = handler.NewMarketHandler(cache, func() context.Context { return app.ctx })
	app.agentHandler = handler.NewAgentHandler(func() context.Context { return app.ctx })
	app.analysisHandler = handler.NewAnalysisHandler(func() context.Context { return app.ctx })
	app.stockHandler = handler.NewStockHandler()
	app.systemHandler = handler.NewSystemHandler(cache, func() context.Context { return app.ctx }, c, Version, VersionCommit, OFFICIAL_STATEMENT, BuildKey, icon, alipay, wxpay, wxgzh, userManual)
	app.tradingHandler = handler.NewDefaultTradingRecordHandler()
	app.stockChangeHandler = handler.NewDefaultStockChangeHandler()
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

func (a *App) GetSponsorInfo() map[string]any {
	return a.systemHandler.GetSponsorInfo()
}

// GetEffectiveSponsorVip 从本地配置解密赞助信息并判断当前是否在 VIP 有效期内（与 ai-assistant-web / data.EffectiveSponsorVipLevel 一致）。
func (a *App) GetEffectiveSponsorVip() map[string]any {
	return a.systemHandler.GetEffectiveSponsorVip()
}

func (a *App) GetMachineId() string {
	return a.systemHandler.GetMachineId()
}

func (a *App) CheckDeviceBinding(token string, apiBase string) map[string]any {
	return a.systemHandler.CheckDeviceBinding(token, apiBase)
}

func (a *App) QuitApp() {
	if a.ctx != nil {
		if a.cron != nil {
			a.cron.Stop()
		}
		runtime.Quit(a.ctx)
	}
}
func (a *App) CheckSponsorCode(sponsorCode string) map[string]any {
	return a.systemHandler.CheckSponsorCode(sponsorCode)
}

func (a *App) CheckUpdate(flag int) {
	a.systemHandler.CheckUpdate(flag)
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

// Greet returns a greeting for the given name
func (a *App) Greet(stockCode string) *data.StockInfo {
	return a.stockHandler.Greet(stockCode)
}

func (a *App) Follow(stockCode string) string {
	return a.stockHandler.Follow(stockCode)
}

func (a *App) UnFollow(stockCode string) string {
	return a.stockHandler.UnFollow(stockCode)
}

func (a *App) GetFollowList(groupId int) *[]data.FollowedStock {
	return a.stockHandler.GetFollowList(groupId)
}

func (a *App) GetStockList(key string) []data.StockBasic {
	return a.stockHandler.GetStockList(key)
}

func (a *App) SetCostPriceAndVolume(stockCode string, price float64, volume int64) string {
	return a.stockHandler.SetCostPriceAndVolume(stockCode, price, volume)
}

func (a *App) SetTradingPrice(stockCode string, entryPrice, takeProfitPrice, stopLossPrice, costPrice float64) string {
	return a.stockHandler.SetTradingPrice(stockCode, entryPrice, takeProfitPrice, stopLossPrice, costPrice)
}

func (a *App) SetAlarmChangePercent(val, alarmPrice float64, stockCode string) string {
	return a.stockHandler.SetAlarmChangePercent(val, alarmPrice, stockCode)
}
func (a *App) SetStockSort(sort int64, stockCode string) {
	a.stockHandler.SetStockSort(sort, stockCode)
}
func (a *App) SendDingDingMessage(message string, stockCode string) string {
	return a.notificationHandler.SendDingDingMessage(message, stockCode)
}

// SendDingDingMessageByType msgType 报警类型: 1 涨跌报警;2 股价报警 3 成本价报警
func (a *App) SendDingDingMessageByType(message string, stockCode string, msgType int) string {
	return a.notificationHandler.SendDingDingMessageByType(message, stockCode, msgType)
}

// SendTestNotification sends a test notification to the specified channel.
func (a *App) SendTestNotification(channel string) string {
	return a.notificationHandler.SendTestNotification(channel)
}

func (a *App) NewChatStream(stock string, stockCode string, question string, aiConfigId int, sysPromptId *int, enableTools bool, think bool, agentMode string, strategyCode string) {
	a.agentHandler.NewChatStream(stock, stockCode, question, aiConfigId, sysPromptId, enableTools, think, agentMode, strategyCode)
}

func (a *App) NewCommodityAnalysisStream(code string, name string, question string, aiConfigId int) {
	a.agentHandler.NewCommodityAnalysisStream(code, name, question, aiConfigId)
}

func (a *App) GetAllStrategies() []*strategy.Strategy {
	return a.agentHandler.GetAllStrategies()
}

func (a *App) SaveAIResponseResult(stockCode, stockName, result, chatId, question string, aiConfigId int) {
	a.analysisHandler.SaveAIResponseResult(stockCode, stockName, result, chatId, question, aiConfigId)
}
func (a *App) GetAIResponseResult(stock string) *models.AIResponseResult {
	return a.analysisHandler.GetAIResponseResult(stock)
}

func (a *App) GetVersionInfo() *models.VersionInfo {
	return a.systemHandler.GetVersionInfo()
}

func (a *App) GetUserManual() string {
	return a.systemHandler.GetUserManual()
}

//// checkChromeOnWindows 在 Windows 系统上检查谷歌浏览器是否安装
//func checkChromeOnWindows() bool {
//	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\chrome.exe`, registry.QUERY_VALUE)
//	if err != nil {
//		// 尝试在 WOW6432Node 中查找（适用于 64 位系统上的 32 位程序）
//		key, err = registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\App Paths\chrome.exe`, registry.QUERY_VALUE)
//		if err != nil {
//			return false
//		}
//		defer key.Close()
//	}
//	defer key.Close()
//	_, _, err = key.GetValue("Path", nil)
//	return err == nil
//}
//
//// checkEdgeOnWindows 在 Windows 系统上检查Edge浏览器是否安装，并返回安装路径
//func checkEdgeOnWindows() (string, bool) {
//	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\msedge.exe`, registry.QUERY_VALUE)
//	if err != nil {
//		// 尝试在 WOW6432Node 中查找（适用于 64 位系统上的 32 位程序）
//		key, err = registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\App Paths\msedge.exe`, registry.QUERY_VALUE)
//		if err != nil {
//			return "", false
//		}
//		defer key.Close()
//	}
//	defer key.Close()
//	path, _, err := key.GetStringValue("Path")
//	if err != nil {
//		return "", false
//	}
//	return path, true
//}

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

func (a *App) GetConfig() *data.SettingConfig {
	return a.systemHandler.GetConfig()
}

func (a *App) ExportConfig() string {
	return a.systemHandler.ExportConfig()
}

func (a *App) ShareAnalysis(stockCode, stockName string) string {
	return a.analysisHandler.ShareAnalysis(stockCode, stockName)
}

// ShareText 直接把文本分享到社区（用于 AI 助手等非 AIResponseResult 场景）
func (a *App) ShareText(text, title string) string {
	return a.analysisHandler.ShareText(text, title)
}

func (a *App) GetfundList(key string) []data.FundBasic {
	return a.fundHandler.GetfundList(key)
}
func (a *App) GetFollowedFund() []data.FollowedFund {
	return a.fundHandler.GetFollowedFund()
}
func (a *App) FollowFund(fundCode string) string {
	return a.fundHandler.FollowFund(fundCode)
}
func (a *App) UnFollowFund(fundCode string) string {
	return a.fundHandler.UnFollowFund(fundCode)
}
func (a *App) GetFundKLine(fundCode string, klt string, limit int) *data.KLineSourceResult {
	return a.fundHandler.GetFundKLine(fundCode, klt, limit)
}
func (a *App) GetFundHistoryNetValue(fundCode string, pageSize int, startDate string, endDate string) []data.FundHistoryNetValue {
	return a.fundHandler.GetFundHistoryNetValue(fundCode, pageSize, startDate, endDate)
}
func (a *App) GetFundTop10Holdings(fundCode string) []data.FundHoldingStock {
	return a.fundHandler.GetFundTop10Holdings(fundCode)
}
func (a *App) GetFundRanking(marketType, fundType, sortField, sortOrder string, pageIndex, pageSize int) *data.FundRankingResult {
	return a.fundHandler.GetFundRanking(marketType, fundType, sortField, sortOrder, pageIndex, pageSize)
}
func (a *App) SearchFundCodes(keyword string) []data.FundSearchItem {
	return a.fundHandler.SearchFundCodes(keyword)
}
func (a *App) GetFollowedFundPaged(pageIndex, pageSize int, keyword string) *data.FollowedFundPagedResult {
	return a.fundHandler.GetFollowedFundPaged(pageIndex, pageSize, keyword)
}
func (a *App) SaveAsMarkdown(stockCode, stockName string) string {
	return a.analysisHandler.SaveAsMarkdown(stockCode, stockName)
}

func (a *App) GetPromptTemplates(name, promptType string) *[]models.PromptTemplate {
	return a.analysisHandler.GetPromptTemplates(name, promptType)
}
func (a *App) AddPrompt(prompt models.Prompt) string {
	return a.analysisHandler.AddPrompt(prompt)
}
func (a *App) DelPrompt(id uint) string {
	return a.analysisHandler.DelPrompt(id)
}
func (a *App) GetMultiAgentPrompts() []models.PromptTemplate {
	return a.analysisHandler.GetMultiAgentPrompts()
}
func (a *App) UpdateMultiAgentPrompt(roleKey, name, content string) string {
	return a.analysisHandler.UpdateMultiAgentPrompt(roleKey, name, content)
}
func (a *App) SetStockAICron(cronText, stockCode string) {
	a.agentHandler.SetStockAICron(cronText, stockCode)
}
func (a *App) AddGroup(group data.Group) string {
	return a.stockHandler.AddGroup(group)
}
func (a *App) GetGroupList() []data.Group {
	return a.stockHandler.GetGroupList()
}

func (a *App) UpdateGroupSort(id int, newSort int) bool {
	return a.stockHandler.UpdateGroupSort(id, newSort)
}

func (a *App) InitializeGroupSort() bool {
	return a.stockHandler.InitializeGroupSort()
}

func (a *App) GetGroupStockList(groupId int) []data.GroupStock {
	return a.stockHandler.GetGroupStockList(groupId)
}

func (a *App) AddStockGroup(groupId int, stockCode string) string {
	return a.stockHandler.AddStockGroup(groupId, stockCode)
}

func (a *App) RemoveStockGroup(code, name string, groupId int) string {
	return a.stockHandler.RemoveStockGroup(code, name, groupId)
}

func (a *App) RemoveGroup(groupId int) string {
	return a.stockHandler.RemoveGroup(groupId)
}

func (a *App) GetStockKLine(stockCode, stockName string, days int64) *[]data.KLineData {
	return a.stockHandler.GetStockKLine(stockCode, stockName, days)
}

func (a *App) GetStockMinutePriceLineData(stockCode, stockName string) map[string]any {
	return a.stockHandler.GetStockMinutePriceLineData(stockCode, stockName)
}

func (a *App) GetStockCommonKLine(stockCode, stockName string, days int64) *[]data.KLineData {
	return a.stockHandler.GetStockCommonKLine(stockCode, stockName, days)
}

// GetStockEastMoneyKLine 东方财富多周期 K 线（分钟：1/5/10/60/120；日 101、周 102、半年 105、年 106）。
// klt 与东方财富接口一致；10 分钟由 1 分钟数据聚合。limit 为根数上限（最大 5000）。
func (a *App) GetStockEastMoneyKLine(stockCode, stockName string, klt string, limit int) *[]data.KLineData {
	return a.stockHandler.GetStockEastMoneyKLine(stockCode, stockName, klt, limit)
}

// GetStockEastMoneyKLinePage 分页拉取 K 线：end 为东财 end 参数（YYYYMMDD 或 YYYYMMDDHHmmss），空字符串表示取最新一段（同 GetStockEastMoneyKLine）。
func (a *App) GetStockEastMoneyKLinePage(stockCode, stockName string, klt string, limit int, end string) *[]data.KLineData {
	return a.stockHandler.GetStockEastMoneyKLinePage(stockCode, stockName, klt, limit, end)
}

// GetStockKLineWithFallback 多数据源自动切换 K 线：优先东方财富，不可用时自动切换新浪财经。
// 返回 KLineSourceResult，包含 data（K 线数组）和 source（实际使用的数据源标识：eastmoney / sina）。
func (a *App) GetStockKLineWithFallback(stockCode, stockName string, klt string, limit int) *data.KLineSourceResult {
	return a.stockHandler.GetStockKLineWithFallback(stockCode, stockName, klt, limit)
}

// GetStockKLinePageWithFallback 多数据源自动切换 K 线（分页）：优先东方财富，不可用时自动切换新浪财经。
// end 参数仅对东方财富有效；新浪数据源不支持分页，将返回最新一段数据。
func (a *App) GetStockKLinePageWithFallback(stockCode, stockName string, klt string, limit int, end string) *data.KLineSourceResult {
	return a.stockHandler.GetStockKLinePageWithFallback(stockCode, stockName, klt, limit, end)
}

// GetChipDistribution 获取/计算股票筹码分布（筹码图）数据（用于前端绘图）。
// days：近多少个交易日；bins：分箱数量；adjustFlag：""/qfq/hfq
func (a *App) GetChipDistribution(stockCode string, days int, bins int, adjustFlag string) (*data.ChipDistributionResult, error) {
	return a.stockHandler.GetChipDistribution(stockCode, days, bins, adjustFlag)
}

// GetTdxCallAuction 通过通达信协议获取集合竞价数据。
// stockCode 格式如 600519.SH、000001.SZ、430047.BJ；start 为起始位置（0=最新）；count 为请求数量（最大 500）。
func (a *App) GetTdxCallAuction(stockCode string, start uint32, count uint32) *[]data.TdxCallAuctionData {
	return a.stockHandler.GetTdxCallAuction(stockCode, start, count)
}

func (a *App) GetTdxCompanyInfo(stockCode string) *data.TdxCompanyInfoBundle {
	return a.stockHandler.GetTdxCompanyInfo(stockCode)
}

func (a *App) GetTdxFinanceInfo(stockCode string) *data.TdxFinanceInfo {
	return a.stockHandler.GetTdxFinanceInfo(stockCode)
}

func (a *App) GetTdxXDXRInfo(stockCode string) *[]data.TdxXDXRItem {
	return a.stockHandler.GetTdxXDXRInfo(stockCode)
}

func (a *App) GetTdxCompanyCategoryList(stockCode string) *[]data.TdxCompanyCategory {
	return a.stockHandler.GetTdxCompanyCategoryList(stockCode)
}

func (a *App) GetTdxCompanyCategoryContent(stockCode string, categoryName string) *data.TdxCompanyInfoSection {
	return a.stockHandler.GetTdxCompanyCategoryContent(stockCode, categoryName)
}

// GetTdxSymbolBelongBoard 通过通达信 MAC 接口获取股票所属板块信息
func (a *App) GetTdxSymbolBelongBoard(stockCode string) *[]data.MACBelongBoardItem {
	return a.stockHandler.GetTdxSymbolBelongBoard(stockCode)
}

func (a *App) GetTelegraphList(source string) *[]*models.Telegraph {
	return a.newsHandler.GetTelegraphList(source)
}

func (a *App) ReFleshTelegraphList(source string) *[]*models.Telegraph {
	return a.newsHandler.ReFleshTelegraphList(source)
}

func (a *App) GetNewsBySector(sectorID string, limit int) (*data.SectorNewsResponse, error) {
	return a.newsHandler.GetNewsBySector(sectorID, limit)
}

func (a *App) GetStockRelatedNews(code string, limit int) ([]data.SectorNewsItem, error) {
	return a.newsHandler.GetStockRelatedNews(code, limit)
}

func (a *App) GetSectors() []data.Sector {
	return a.newsHandler.GetSectors()
}

func (a *App) GlobalStockIndexes() map[string]any {
	return a.marketHandler.GlobalStockIndexes()
}

// GlobalStockIndexesReadable 将全球指数 JSON 转为 AI 易读 Markdown 文本。
func (a *App) GlobalStockIndexesReadable() string {
	return a.marketHandler.GlobalStockIndexesReadable()
}

func (a *App) SummaryStockNews(question string, aiConfigId int, sysPromptId *int, enableTools bool, think bool, eventName string, historyJSON string) {
	a.agentHandler.SummaryStockNews(question, aiConfigId, sysPromptId, enableTools, think, eventName, historyJSON)
}
func (a *App) GetIndustryRank(sort string, cnt int) []any {
	return a.marketHandler.GetIndustryRank(sort, cnt)
}
func (a *App) GetIndustryMoneyRankSina(fenlei, sort string) []map[string]any {
	return a.marketHandler.GetIndustryMoneyRankSina(fenlei, sort)
}
func (a *App) GetMoneyRankSina(sort string) []map[string]any {
	return a.marketHandler.GetMoneyRankSina(sort)
}

func (a *App) GetStockMoneyTrendByDay(stockCode string, days int) []map[string]any {
	return a.marketHandler.GetStockMoneyTrendByDay(stockCode, days)
}

// OpenURL
//
//	@Description:  跨平台打开默认浏览器
//	@receiver a
//	@param url
func (a *App) OpenURL(url string) {
	a.systemHandler.OpenURL(url)
}

// SaveImage
//
//	@Description: 跨平台保存图片
//	@receiver a
//	@param name
//	@param base64Data
//	@return error
func (a *App) SaveImage(name, base64Data string) string {
	return a.analysisHandler.SaveImage(name, base64Data)
}

// SaveWordFile
//
//	@Description: // 跨平台保存word
//	@receiver a
//	@param filename
//	@param base64Data
//	@return error
func (a *App) SaveWordFile(filename string, base64Data string) string {
	return a.analysisHandler.SaveWordFile(filename, base64Data)
}

// GetAiConfigs
//
//	@Description: // 获取 AiConfig 列表
//	@receiver a
//	@return error
func (a *App) GetAiConfigs() []*data.AIConfig {
	return a.systemHandler.GetAiConfigs()
}

// GetAiAssistantSession 获取 AI 助手会话消息列表，sessionId 为空时获取最新的
func (a *App) GetAiAssistantSession(sessionId string) (*models.AiAssistantSessionResp, error) {
	return a.systemHandler.GetAiAssistantSession(sessionId)
}

// SaveAiAssistantSession 保存 AI 助手会话消息到数据库
func (a *App) SaveAiAssistantSession(sessionId string, messages []models.AiAssistantMessage) error {
	return a.systemHandler.SaveAiAssistantSession(sessionId, messages)
}

// FetchAiModels
//
//	@Description: 根据接口地址与 apiKey 自动获取支持的模型列表（OpenAI/DeepSeek 兼容 /models 接口）
//	@receiver a
//	@param baseUrl 接口地址（如 https://api.deepseek.com）
//	@param apiKey  鉴权令牌
//	@return []string 模型 ID 列表
func (a *App) FetchAiModels(baseUrl, apiKey string) []string {
	return a.systemHandler.FetchAiModels(baseUrl, apiKey)
}

type AiModelInfo struct {
	ModelName string `json:"modelName"`
	MaxTokens int    `json:"maxTokens"`
	Source    string `json:"source"`
}

func (a *App) FetchAiModelInfo(baseUrl, apiKey, modelName string) *AiModelInfo {
	info := a.systemHandler.FetchAiModelInfo(baseUrl, apiKey, modelName)
	if info == nil {
		return nil
	}
	return &AiModelInfo{
		ModelName: info.ModelName,
		MaxTokens: info.MaxTokens,
		Source:    info.Source,
	}
}

// InitCronTasks 在应用启动时，自动为启用状态的定时任务创建调度
func (a *App) InitCronTasks() {
	a.systemHandler.InitCronTasks()
}

// AbortSummaryStockNews 取消当前进行中的 SummaryStockNews 流式回答
func (a *App) AbortSummaryStockNews() {
	a.agentHandler.AbortSummaryStockNews()
}

// CreateCronTask
//
//	@Description: 创建定时任务
//	@receiver a
//	@param task 定时任务信息
//	@return string 操作结果
func (a *App) CreateCronTask(task *models.CronTask) string {
	return a.systemHandler.CreateCronTask(task)
}

func (a *App) UpdateCronTask(task *models.CronTask) string {
	return a.systemHandler.UpdateCronTask(task)
}

// DeleteCronTask
//
//	@Description: 删除定时任务
//	@receiver a
//	@param id 任务 ID
//	@return string 操作结果
func (a *App) DeleteCronTask(id uint) string {
	return a.systemHandler.DeleteCronTask(id)
}

// GetCronTaskByID
//
//	@Description: 根据 ID 获取定时任务
//	@receiver a
//	@param id 任务 ID
//	@return *models.CronTask 任务信息
func (a *App) GetCronTaskByID(id uint) *models.CronTask {
	return a.systemHandler.GetCronTaskByID(id)
}

// GetCronTaskList
//
//	@Description: 获取定时任务列表
//	@receiver a
//	@param query 查询条件
//	@return *models.CronTaskPageResp 分页结果
func (a *App) GetCronTaskList(query *models.CronTaskQuery) *models.CronTaskPageResp {
	return a.systemHandler.GetCronTaskList(query)
}

// EnableCronTask
//
//	@Description: 启用/禁用定时任务
//	@receiver a
func (a *App) EnableCronTask(id uint, enable bool) string {
	return a.systemHandler.EnableCronTask(id, enable)
}

// ExecuteCronTaskNow
//
//	@Description: 立即执行定时任务
//	@receiver a
//	@param id 任务 ID
//	@return string 操作结果
func (a *App) ExecuteCronTaskNow(id uint) string {
	return a.systemHandler.ExecuteCronTaskNow(id)
}

// GetCronTaskTypes
//
//	@Description: 获取所有任务类型
//	@receiver a
//	@return []lo.Tuple2[string, string] 任务类型列表
func (a *App) GetCronTaskTypes() []lo.Tuple2[string, string] {
	return a.systemHandler.GetCronTaskTypes()
}

// ValidateCronExpr
//
//	@Description: 验证 Cron 表达式
//	@receiver a
//	@param expr Cron 表达式
//	@return string 验证结果
func (a *App) ValidateCronExpr(expr string) string {
	return a.systemHandler.ValidateCronExpr(expr)
}

// SearchCronTasks
//
//	@Description: 搜索定时任务
//	@receiver a
//	@param keyword 搜索关键词
//	@return []models.CronTask 搜索结果
func (a *App) SearchCronTasks(keyword string) []models.CronTask {
	return a.systemHandler.SearchCronTasks(keyword)
}

// CalculateNextRunTime 根据 Cron 表达式计算下一次运行时间
// 参数:
//   - cron: Cron 表达式，用于定义任务调度的时间规则
//
// 返回值:
//   - string: 格式化为 "2006-01-02 15:04:05" 的下一次运行时间字符串
func (a *App) CalculateNextRunTime(cron string) string {
	return a.systemHandler.CalculateNextRunTime(cron)
}

// CalculateNextRunTimes 根据 Cron 表达式计算未来多次运行时间
// 参数:
//   - cron: Cron 表达式
//   - count: 需要计算的次数
//
// 返回值:
//   - []string: 按时间顺序排序的运行时间列表，格式为 "2006-01-02 15:04:05"
func (a *App) CalculateNextRunTimes(cron string, count int) []string {
	return a.systemHandler.CalculateNextRunTimes(cron, count)
}

// AddTradingRecord 添加交易记录
// 参数:
//   - record: 交易记录结构体
//
// 返回值:
//   - uint: 新添加的交易记录ID
//   - error: 错误信息
func (a *App) AddTradingRecord(record data.TradingRecord) (uint, error) {
	return a.tradingHandler.AddTradingRecord(record)
}

// GetTradingRecordList 获取交易记录列表（分页与筛选，返回结构与 AI 推荐列表一致）
func (a *App) GetTradingRecordList(query data.TradingRecordListQuery) *data.TradingRecordPageData {
	return a.tradingHandler.GetTradingRecordList(query)
}

// GetTradingRecordById 根据ID获取单个交易记录
// 参数:
//   - id: 交易记录ID
//
// 返回值:
//   - *data.TradingRecord: 交易记录指针
//   - error: 错误信息
func (a *App) GetTradingRecordById(id uint) (*data.TradingRecord, error) {
	return a.tradingHandler.GetTradingRecordById(id)
}

// GetTradingRecordStatistics 获取交易记录统计数据
//
// 返回值:
//   - *data.TradingRecordStatistics: 统计数据指针
func (a *App) GetTradingRecordStatistics() *data.TradingRecordStatistics {
	return a.tradingHandler.GetTradingRecordStatistics()
}

// UpdateTradingRecord 更新交易记录
// 参数:
//   - record: 交易记录结构体
//
// 返回值:
//   - error: 错误信息
func (a *App) UpdateTradingRecord(record data.TradingRecord) error {
	return a.tradingHandler.UpdateTradingRecord(record)
}

// DeleteTradingRecord 删除交易记录
// 参数:
//   - id: 交易记录ID
//
// 返回值:
//   - error: 错误信息
func (a *App) DeleteTradingRecord(id uint) error {
	return a.tradingHandler.DeleteTradingRecord(id)
}

// CheckFrequentTrading 检查是否频繁交易
// 参数:
//   - stockCode: 股票代码
//
// 返回值:
//   - map[string]any: 包含 canTrade (bool) 和 msg (string)
func (a *App) CheckFrequentTrading(stockCode string) map[string]any {
	return a.tradingHandler.CheckFrequentTrading(stockCode)
}

func (a *App) FetchAndSaveMarketStatistic() {
	a.marketHandler.FetchAndSaveMarketStatistic()
}

func (a *App) GetTodayMarketStatistic() []models.MarketStatistic {
	return a.marketHandler.GetTodayMarketStatistic()
}

func (a *App) GetMarketStatisticByDate(date string) []models.MarketStatistic {
	return a.marketHandler.GetMarketStatisticByDate(date)
}

func (a *App) GetRecentDaysMarketStatistic(days int) []models.MarketStatistic {
	return a.marketHandler.GetRecentDaysMarketStatistic(days)
}

func (a *App) CreateMCPServer(server *models.MCPServer) string {
	return a.systemHandler.CreateMCPServer(server)
}

func (a *App) UpdateMCPServer(server *models.MCPServer) string {
	return a.systemHandler.UpdateMCPServer(server)
}

func (a *App) DeleteMCPServer(id uint) string {
	return a.systemHandler.DeleteMCPServer(id)
}

func (a *App) GetMCPServerByID(id uint) *models.MCPServer {
	return a.systemHandler.GetMCPServerByID(id)
}

func (a *App) GetMCPServerList(query *models.MCPServerQuery) *models.MCPServerPageResp {
	return a.systemHandler.GetMCPServerList(query)
}

func (a *App) EnableMCPServer(id uint, enable bool) string {
	return a.systemHandler.EnableMCPServer(id, enable)
}

func (a *App) TestMCPServer(id uint) string {
	return a.systemHandler.TestMCPServer(id)
}

func (a *App) CreateSkill(skill *models.Skill) string {
	return a.systemHandler.CreateSkill(skill)
}

func (a *App) UpdateSkill(skill *models.Skill) string {
	return a.systemHandler.UpdateSkill(skill)
}

func (a *App) DeleteSkill(id uint) string {
	return a.systemHandler.DeleteSkill(id)
}

func (a *App) GetSkillByID(id uint) *models.Skill {
	return a.systemHandler.GetSkillByID(id)
}

func (a *App) GetSkillList(query *models.SkillQuery) *models.SkillPageResp {
	return a.systemHandler.GetSkillList(query)
}

func (a *App) EnableSkill(id uint, enable bool) string {
	return a.systemHandler.EnableSkill(id, enable)
}

func (a *App) GetAllSkills() []models.Skill {
	return a.systemHandler.GetAllSkills()
}

func (a *App) GetMCPToolsByServerID(serverID uint) []models.MCPServerTool {
	return a.systemHandler.GetMCPToolsByServerID(serverID)
}

func (a *App) GetAllMCPTools() []models.MCPServerTool {
	return a.systemHandler.GetAllMCPTools()
}

func (a *App) GenerateSkillFromURL(url string) string {
	skill, confidence, err := a.systemHandler.GenerateSkillFromURL(url)
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("技能草稿生成成功（置信度: %.2f），请前往技能管理查看并启用。\n名称: %s\n分类: %s\n描述: %s\n触发关键词: %s",
		confidence, skill.Name, skill.Category, skill.Description, skill.TriggerKeywords)
}

func (a *App) AnalyzeSkillEffectiveness(id uint) string {
	return a.systemHandler.AnalyzeSkillEffectiveness(id)
}
