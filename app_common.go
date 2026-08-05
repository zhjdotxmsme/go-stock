package main

import (
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/data/datasource"
	"go-stock/backend/models"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
)

// @Author spark
// @Date 2025/6/8 20:45
// @Desc
//--------------------------------------------------------------------------------

var ShanghaiTimezone = time.FixedZone("CST", 8*60*60)

func GetShanghaiTime() time.Time {
	return time.Now().In(ShanghaiTimezone)
}

func FormatShanghaiTime(t time.Time) string {
	return t.In(ShanghaiTimezone).Format("2006-01-02 15:04:05")
}

func (a *App) GetTimezone() map[string]any {
	return map[string]any{
		"offset":   8 * 60 * 60,
		"location": "Asia/Shanghai",
	}
}

func (a *App) LongTigerRank(date string) *[]models.LongTigerRankData {
	return a.marketHandler.LongTigerRank(date)
}

func (a *App) StockResearchReport(stockCode string) []any {
	return data.NewMarketNewsApi().StockResearchReport(stockCode, 7)
}
func (a *App) StockNotice(stockCode string) []any {
	return data.NewMarketNewsApi().StockNotice(stockCode)
}

func (a *App) IndustryResearchReport(industryCode string) []any {
	return a.marketHandler.IndustryResearchReport(industryCode)
}
func (a *App) EMDictCode(code string) []any {
	return a.marketHandler.EMDictCode(code)
}

func (a *App) AnalyzeSentiment(text string) models.SentimentResult {
	return a.analysisHandler.AnalyzeSentiment(text)
}

func (a *App) HotStock(marketType string) *[]models.HotItem {
	return a.marketHandler.HotStock(marketType)
}

func (a *App) HotEvent(size int) *[]models.HotEvent {
	return a.marketHandler.HotEvent(size)
}
func (a *App) HotTopic(size int) []any {
	return a.marketHandler.HotTopic(size)
}

func (a *App) InvestCalendarTimeLine(yearMonth string) []any {
	return a.marketHandler.InvestCalendarTimeLine(yearMonth)
}
func (a *App) ClsCalendar() []any {
	return a.marketHandler.ClsCalendar()
}

func (a *App) GetUplimitHot(date string, limit int) map[string]any {
	return a.marketHandler.GetUplimitHot(date, limit)
}

func (a *App) IsTradingTime() bool {
	return a.marketHandler.IsTradingTime()
}

func (a *App) IsHKTradingTime() bool {
	return a.marketHandler.IsHKTradingTime()
}

func (a *App) IsUSTradingTime() bool {
	return a.marketHandler.IsUSTradingTime()
}

// IsTradingDay 判断 yyyy-MM-dd 是否为 A 股交易日（周末、法定节假日为 false）。
func (a *App) IsTradingDay(date string) bool {
	return a.marketHandler.IsTradingDay(date)
}

func (a *App) GetLatestTradingDay() string {
	return a.marketHandler.GetLatestTradingDay()
}

func (a *App) SearchStock(words string) map[string]any {
	return a.analysisHandler.SearchStock(words)
}
func (a *App) GetHotStrategy() map[string]any {
	return a.analysisHandler.GetHotStrategy()
}

func (a *App) AIConfiguredStockPick(query string, topN int) ([]models.DailyPick, error) {
	return a.analysisHandler.AIConfiguredStockPick(query, topN)
}

func (a *App) GetCustomStrategyList(query models.CustomStrategyQuery) *models.CustomStrategyPageData {
	return a.analysisHandler.GetCustomStrategyList(query)
}

func (a *App) GetAllCustomStrategies() *[]models.CustomStrategy {
	return a.analysisHandler.GetAllCustomStrategies()
}

func (a *App) SaveCustomStrategy(strategy models.CustomStrategy) string {
	return a.analysisHandler.SaveCustomStrategy(strategy)
}

func (a *App) DeleteCustomStrategy(id uint) string {
	return a.analysisHandler.DeleteCustomStrategy(id)
}

func (a *App) GetAllStocks(page int, pageSize int, name string, technicalIndicators models.TechnicalIndicators) *models.AllStocksResp {
	return a.analysisHandler.GetAllStocks(page, pageSize, name, technicalIndicators)
}

func (a *App) ChatWithAgent(question string, aiConfigId int, sysPromptId *int, memoryMode bool, memoryCount int, thinkingMode bool, agentMode string) {
	a.agentHandler.ChatWithAgent(question, aiConfigId, sysPromptId, memoryMode, memoryCount, thinkingMode, agentMode)
}

func (a *App) AbortChatWithAgent() {
	a.agentHandler.AbortChatWithAgent()
}

func (a *App) AnalyzeSentimentWithFreqWeight(text string) map[string]any {
	return a.analysisHandler.AnalyzeSentimentWithFreqWeight(text)
}

func (a *App) GetAIResponseResultList(query models.AIResponseResultQuery) *models.AIResponseResultPageData {
	return a.analysisHandler.GetAIResponseResultList(query)
}
func (a *App) DeleteAIResponseResult(id uint) string {
	return a.analysisHandler.DeleteAIResponseResult(id)
}
func (a *App) BatchDeleteAIResponseResult(ids []uint) string {
	return a.analysisHandler.BatchDeleteAIResponseResult(ids)
}

func (a *App) GetStockChanges(changeTypes []int, pageIndex, pageSize int) *data.StockChangesResponse {
	return data.NewStockChangesApi().GetStockChanges(changeTypes, pageIndex, pageSize)
}

func (a *App) GetAllStockChangesWithPaging(pageSize int) *data.StockChangesResponse {
	all := data.NewStockChangesApi().GetAllStockChangesWithPaging(pageSize)
	historyService := data.NewStockChangeHistoryService()
	_, _ = historyService.SaveStockChangesWithDedup(all.Data)
	return all
}

func (a *App) GetStockChangeHistory(query models.StockChangeHistoryQuery) *models.StockChangeHistoryPageData {
	result, err := data.NewStockChangeHistoryService().GetHistoryList(query)
	if err != nil {
		return &models.StockChangeHistoryPageData{}
	}
	return result
}

func (a *App) SaveStockChangesToHistory(changeTypes []int) string {
	api := data.NewStockChangesApi()
	result := api.GetStockChanges(changeTypes, 0, 500)
	if result == nil || len(result.Data) == 0 {
		return "没有获取到异动数据"
	}

	err := data.NewStockChangeHistoryService().SaveStockChanges(result.Data)
	if err != nil {
		return "保存失败: " + err.Error()
	}
	return fmt.Sprintf("成功保存 %d 条异动数据", len(result.Data))
}

func (a *App) DeleteStockChangeHistory(days int) string {
	err := data.NewStockChangeHistoryService().DeleteOldData(days)
	if err != nil {
		return "删除失败: " + err.Error()
	}
	return fmt.Sprintf("已删除 %d 天前的历史数据", days)
}

func (a *App) GetDailyChangeStats(days int) []data.DailyChangeStats {
	result, err := data.NewStockChangeHistoryService().GetDailyChangeStats(days)
	if err != nil {
		return []data.DailyChangeStats{}
	}
	return result
}

func (a *App) GetChangeTypeDailyStats(days int) []data.ChangeTypeDailyStats {
	result, err := data.NewStockChangeHistoryService().GetChangeTypeDailyStats(days)
	if err != nil {
		return []data.ChangeTypeDailyStats{}
	}
	return result
}

func (a *App) GetChangeRank(days int, topN int) *data.ChangeRankResult {
	result, err := data.NewStockChangeHistoryService().GetChangeRank(days, topN)
	if err != nil {
		return &data.ChangeRankResult{}
	}
	return result
}

func (a *App) GetDailyDimensionStats(dimension string, name string, days int) []data.DailyDimensionStats {
	result, err := data.NewStockChangeHistoryService().GetDailyDimensionStats(dimension, name, days)
	if err != nil {
		return []data.DailyDimensionStats{}
	}
	return result
}

func (a *App) GetTypeStatsByDate(date string) []data.TypeCountStats {
	result, err := data.NewStockChangeHistoryService().GetTypeStatsByDate(date)
	if err != nil {
		return []data.TypeCountStats{}
	}
	return result
}

func (a *App) GetAiRecommendStocksList(query models.AiRecommendStocksQuery) *models.AiRecommendStocksPageData {
	return a.analysisHandler.GetAiRecommendStocksList(query)
}
func (a *App) DeleteAiRecommendStocks(id uint) string {
	return a.analysisHandler.DeleteAiRecommendStocks(id)
}

func (a *App) UpdateAiRecommendStocksAlert(id uint, enableAlert bool) string {
	return a.analysisHandler.UpdateAiRecommendStocksAlert(id, enableAlert)
}

func (a *App) GetAiRecommendStats() *data.AiRecommendStats {
	return a.analysisHandler.GetAiRecommendStats()
}

func (a *App) GetPromptTemplateList(query models.PromptTemplateQuery) *models.PromptTemplatePageData {
	return a.analysisHandler.GetPromptTemplateList(query)
}

func (a *App) AddPromptTemplate(template models.PromptTemplate) string {
	return a.analysisHandler.AddPromptTemplate(template)
}

func (a *App) UpdatePromptTemplate(template models.PromptTemplate) string {
	return a.analysisHandler.UpdatePromptTemplate(template)
}

func (a *App) DeletePromptTemplate(id uint) string {
	return a.analysisHandler.DeletePromptTemplate(id)
}

func (a *App) GetAllStockInfoList(query data.AllStockInfoQuery) *data.AllStockInfoPageData {
	page, err := data.NewStockDataApi().GetAllStockInfoList(&query)
	if err != nil {
		return &data.AllStockInfoPageData{}
	}
	return page
}

func (a *App) GetAllStockInfoById(id uint) *models.AllStockInfo {
	stock, err := data.NewStockDataApi().GetAllStockInfoById(id)
	if err != nil {
		return &models.AllStockInfo{}
	}
	return stock
}

func (a *App) AddAllStockInfo(stock models.AllStockInfo) string {
	err := data.NewStockDataApi().AddAllStockInfo(stock)
	if err != nil {
		return "操作失败: " + err.Error()
	}
	return "操作成功"
}

func (a *App) DeleteAllStockInfo(id uint) string {
	err := data.NewStockDataApi().DeleteAllStockInfo(id)
	if err != nil {
		return "删除失败: " + err.Error()
	}
	return "删除成功"
}

func (a *App) BatchDeleteAllStockInfo(ids []uint) string {
	err := data.NewStockDataApi().BatchDeleteAllStockInfo(ids)
	if err != nil {
		return "批量删除失败: " + err.Error()
	}
	return "批量删除成功"
}

func (a *App) GetAllMarkets() []string {
	markets, err := data.NewStockDataApi().GetAllMarkets()
	if err != nil {
		return []string{}
	}
	return markets
}

func (a *App) GetAllIndustries() []string {
	industries, err := data.NewStockDataApi().GetAllIndustries()
	if err != nil {
		return []string{}
	}
	return industries
}

func (a *App) GetAllConcepts() []string {
	concepts, err := data.NewStockDataApi().GetAllConcepts()
	if err != nil {
		return []string{}
	}
	return concepts
}

func (a *App) GetStockRealTimePrice(stockCode string) map[string]any {
	stockDatas, err := data.NewStockDataApi().GetStockCodeRealTimeData(stockCode)
	if err != nil || stockDatas == nil || len(*stockDatas) == 0 {
		return map[string]any{
			"code":    -1,
			"message": "获取股票价格失败",
			"price":   0,
		}
	}
	stock := (*stockDatas)[0]
	price, _ := convertor.ToFloat(stock.Price)
	if price == 0 {
		price, _ = convertor.ToFloat(stock.A1P)
	}
	if price == 0 {
		price, _ = convertor.ToFloat(stock.B1P)
	}
	if price == 0 {
		price, _ = convertor.ToFloat(stock.PreClose)
	}
	return map[string]any{
		"code":    0,
		"message": "success",
		"price":   price,
		"name":    stock.Name,
	}
}

// GetBKFundFlowList 获取板块资金流向历史数据（折线图用）
func (a *App) GetBKFundFlowList(code string, limit int) []models.BKFundFlowPoint {
	return a.marketHandler.GetBKFundFlowList(code, limit)
}

// GetBKFundFlowListByDate 获取板块指定日期的资金流向历史数据
func (a *App) GetBKFundFlowListByDate(code string, date string) []models.BKFundFlowPoint {
	return a.marketHandler.GetBKFundFlowListByDate(code, date)
}

// GetBKFundFlowTopList 获取最新板块资金排名
func (a *App) GetBKFundFlowTopList(topN int) []models.BKFundFlow {
	return a.marketHandler.GetBKFundFlowTopList(topN)
}

// GetBKFundFlowTopListByDate 获取指定日期的板块资金排名
func (a *App) GetBKFundFlowTopListByDate(date string, topN int) []models.BKFundFlow {
	return a.marketHandler.GetBKFundFlowTopListByDate(date, topN)
}

// GetAllBKCodes 获取所有已记录的板块代码
func (a *App) GetAllBKCodes() []map[string]string {
	return a.marketHandler.GetAllBKCodes()
}

// GetConceptFundFlowList 获取概念资金流向历史数据（折线图用）
func (a *App) GetConceptFundFlowList(code string, limit int) []models.ConceptFundFlowPoint {
	return a.marketHandler.GetConceptFundFlowList(code, limit)
}

// GetConceptFundFlowListByDate 获取概念指定日期的资金流向历史数据
func (a *App) GetConceptFundFlowListByDate(code string, date string) []models.ConceptFundFlowPoint {
	return a.marketHandler.GetConceptFundFlowListByDate(code, date)
}

// GetConceptFundFlowTopList 获取最新概念资金排名
func (a *App) GetConceptFundFlowTopList(topN int) []models.ConceptFundFlow {
	return a.marketHandler.GetConceptFundFlowTopList(topN)
}

// GetConceptFundFlowTopListByDate 获取指定日期的概念资金排名
func (a *App) GetConceptFundFlowTopListByDate(date string, topN int) []models.ConceptFundFlow {
	return a.marketHandler.GetConceptFundFlowTopListByDate(date, topN)
}

// GetAllConceptCodes 获取所有概念代码
func (a *App) GetAllConceptCodes() []map[string]string {
	return a.marketHandler.GetAllConceptCodes()
}

func (a *App) GetCommodityKLine(code string, period string, count int) ([]datasource.KLineBar, error) {
	return a.commodityHandler.GetCommodityKLine(code, period, count)
}

func (a *App) GetCommodityKLineIntl(code string, period string, count int) ([]datasource.KLineBar, error) {
	return a.commodityHandler.GetCommodityKLineIntl(code, period, count)
}

func (a *App) GetCommodityQuote(code string) (*datasource.QuoteData, error) {
	return a.commodityHandler.GetCommodityQuote(code)
}

func (a *App) GetCommodityQuoteIntl(code string) (*datasource.QuoteData, error) {
	return a.commodityHandler.GetCommodityQuoteIntl(code)
}

func (a *App) GetCommodityRegistry() []models.CommodityAsset {
	return a.commodityHandler.GetCommodityRegistry()
}

func (a *App) GetCommodityTechnicals(code string, period string) (string, error) {
	return a.commodityHandler.GetCommodityTechnicals(code, period)
}

func (a *App) GetCommodityFundamentals(code string) (string, error) {
	return a.commodityHandler.GetCommodityFundamentals(code)
}

func (a *App) GetCommodityCorrelation(primaryCode string, secondaryCodes string) (string, error) {
	return a.commodityHandler.GetCommodityCorrelation(primaryCode, secondaryCodes)
}

func (a *App) GetMacroIndicatorsEnhanced() (*data.MacroSnapshotEnhanced, error) {
	return a.commodityHandler.GetMacroIndicatorsEnhanced()
}

func (a *App) GetCommodityReport(codes string, reportType string) (string, error) {
	return a.commodityHandler.GetCommodityReport(codes, reportType)
}

func (a *App) GetTradableCommodities() []models.CommodityAsset {
	return a.commodityHandler.GetTradableCommodities()
}
