package handler

import (
	"context"
	"strings"
	"time"

	"github.com/coocood/freecache"
	"github.com/duke-git/lancet/v2/slice"

	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// MarketHandler handles market-related Wails bindings.
type MarketHandler struct {
	cache *freecache.Cache
	ctxFn func() context.Context
}

// NewMarketHandler creates a new MarketHandler.
// ctxFn should return the current App context (set after Wails startup).
func NewMarketHandler(cache *freecache.Cache, ctxFn func() context.Context) *MarketHandler {
	return &MarketHandler{cache: cache, ctxFn: ctxFn}
}

func (h *MarketHandler) currentCtx() context.Context {
	if h.ctxFn != nil {
		return h.ctxFn()
	}
	return context.Background()
}

var shanghaiTimezone = time.FixedZone("CST", 8*60*60)

func (h *MarketHandler) LongTigerRank(date string) *[]models.LongTigerRankData {
	return data.NewMarketNewsApi().LongTiger(date)
}

func (h *MarketHandler) StockResearchReport(stockCode string) []any {
	return data.NewMarketNewsApi().StockResearchReport(stockCode, 7)
}

func (h *MarketHandler) StockNotice(stockCode string) []any {
	return data.NewMarketNewsApi().StockNotice(stockCode)
}

func (h *MarketHandler) IndustryResearchReport(industryCode string) []any {
	return data.NewMarketNewsApi().IndustryResearchReport(industryCode, 7)
}

func (h *MarketHandler) EMDictCode(code string) []any {
	return data.NewMarketNewsApi().EMDictCode(code, h.cache)
}

func (h *MarketHandler) HotStock(marketType string) *[]models.HotItem {
	return data.NewMarketNewsApi().XUEQIUHotStock(100, marketType)
}

func (h *MarketHandler) HotEvent(size int) *[]models.HotEvent {
	if size <= 0 {
		size = 10
	}
	return data.NewMarketNewsApi().HotEvent(size)
}

func (h *MarketHandler) HotTopic(size int) []any {
	if size <= 0 {
		size = 10
	}
	return data.NewMarketNewsApi().HotTopic(size)
}

func (h *MarketHandler) InvestCalendarTimeLine(yearMonth string) []any {
	return data.NewMarketNewsApi().InvestCalendar(yearMonth)
}

func (h *MarketHandler) ClsCalendar() []any {
	return data.NewMarketNewsApi().ClsCalendar()
}

func (h *MarketHandler) GetUplimitHot(date string, limit int) map[string]any {
	return data.NewMarketNewsApi().GetUplimitHot(date, limit)
}

func (h *MarketHandler) IsTradingTime() bool {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = shanghaiTimezone
	}
	return isTradingTime(time.Now().In(loc))
}

func (h *MarketHandler) IsHKTradingTime() bool {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = shanghaiTimezone
	}
	return isHKTradingTime(time.Now().In(loc))
}

func (h *MarketHandler) IsUSTradingTime() bool {
	return isUSTradingTime(time.Now())
}

// IsTradingDay 判断 yyyy-MM-dd 是否为 A 股交易日（周末、法定节假日为 false）。
func (h *MarketHandler) IsTradingDay(date string) bool {
	date = strings.TrimSpace(date)
	if date == "" {
		return false
	}
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = shanghaiTimezone
	}
	t, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil {
		return false
	}
	return isTradingDay(t)
}

func (h *MarketHandler) GetLatestTradingDay() string {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = shanghaiTimezone
	}
	now := time.Now().In(loc)
	if isTradingDay(now) {
		hour, minute, _ := now.Clock()
		if hour < 15 || (hour == 15 && minute == 0) {
			return now.AddDate(0, 0, -1).Format("2006-01-02")
		}
		return now.Format("2006-01-02")
	}
	for i := 1; i <= 7; i++ {
		d := now.AddDate(0, 0, -i)
		if isTradingDay(d) {
			return d.Format("2006-01-02")
		}
	}
	return now.Format("2006-01-02")
}

func (h *MarketHandler) GetIndustryRank(sort string, cnt int) []any {
	res := data.NewMarketNewsApi().GetIndustryRank(sort, cnt)
	return res["data"].([]any)
}

func (h *MarketHandler) GetIndustryMoneyRankSina(fenlei, sort string) []map[string]any {
	res := data.NewMarketNewsApi().GetIndustryMoneyRankSina(fenlei, sort)
	return res
}

func (h *MarketHandler) GetMoneyRankSina(sort string) []map[string]any {
	res := data.NewMarketNewsApi().GetMoneyRankSina(sort)
	return res
}

func (h *MarketHandler) GetStockMoneyTrendByDay(stockCode string, days int) []map[string]any {
	res := data.NewMarketNewsApi().GetStockMoneyTrendByDay(stockCode, days)
	slice.Reverse(res)
	return res
}

func (h *MarketHandler) GlobalStockIndexes() map[string]any {
	return data.NewMarketNewsApi().GlobalStockIndexes(30)
}

// GlobalStockIndexesReadable 将全球指数 JSON 转为 AI 易读 Markdown 文本。
func (h *MarketHandler) GlobalStockIndexesReadable() string {
	return data.NewMarketNewsApi().GlobalStockIndexesReadable(30)
}

// GetBKFundFlowList 获取板块资金流向历史数据（折线图用）
func (h *MarketHandler) GetBKFundFlowList(code string, limit int) []models.BKFundFlowPoint {
	return data.NewBKFundFlowApi().GetBKFundFlowList(code, limit)
}

// GetBKFundFlowListByDate 获取板块指定日期的资金流向历史数据
func (h *MarketHandler) GetBKFundFlowListByDate(code string, date string) []models.BKFundFlowPoint {
	return data.NewBKFundFlowApi().GetBKFundFlowListByDate(code, date)
}

// GetBKFundFlowTopList 获取最新板块资金排名
func (h *MarketHandler) GetBKFundFlowTopList(topN int) []models.BKFundFlow {
	return data.NewBKFundFlowApi().GetBKFundFlowTopList(topN)
}

// GetBKFundFlowTopListByDate 获取指定日期的板块资金排名
func (h *MarketHandler) GetBKFundFlowTopListByDate(date string, topN int) []models.BKFundFlow {
	return data.NewBKFundFlowApi().GetBKFundFlowTopListByDate(date, topN)
}

// GetAllBKCodes 获取所有已记录的板块代码
func (h *MarketHandler) GetAllBKCodes() []map[string]string {
	return data.NewBKFundFlowApi().GetAllBKCodes()
}

// GetConceptFundFlowList 获取概念资金流向历史数据（折线图用）
func (h *MarketHandler) GetConceptFundFlowList(code string, limit int) []models.ConceptFundFlowPoint {
	return data.NewConceptFundFlowApi().GetConceptFundFlowList(code, limit)
}

// GetConceptFundFlowListByDate 获取概念指定日期的资金流向历史数据
func (h *MarketHandler) GetConceptFundFlowListByDate(code string, date string) []models.ConceptFundFlowPoint {
	return data.NewConceptFundFlowApi().GetConceptFundFlowListByDate(code, date)
}

// GetConceptFundFlowTopList 获取最新概念资金排名
func (h *MarketHandler) GetConceptFundFlowTopList(topN int) []models.ConceptFundFlow {
	return data.NewConceptFundFlowApi().GetConceptFundFlowTopList(topN)
}

// GetConceptFundFlowTopListByDate 获取指定日期的概念资金排名
func (h *MarketHandler) GetConceptFundFlowTopListByDate(date string, topN int) []models.ConceptFundFlow {
	return data.NewConceptFundFlowApi().GetConceptFundFlowTopListByDate(date, topN)
}

// GetAllConceptCodes 获取所有概念代码
func (h *MarketHandler) GetAllConceptCodes() []map[string]string {
	return data.NewConceptFundFlowApi().GetAllConceptCodes()
}

func (h *MarketHandler) FetchAndSaveMarketStatistic() {
	if !isTradingTime(time.Now()) {
		logger.SugaredLogger.Debugf("当前非交易时间，跳过市场统计数据采集")
		return
	}
	err := data.NewMarketStatisticApi().FetchAndSave()
	if err != nil {
		logger.SugaredLogger.Errorf("获取市场统计数据失败: %v", err)
	}
}

func (h *MarketHandler) GetTodayMarketStatistic() []models.MarketStatistic {
	return data.NewMarketStatisticApi().GetTodayData()
}

func (h *MarketHandler) GetMarketStatisticByDate(date string) []models.MarketStatistic {
	return data.NewMarketStatisticApi().GetByDate(date)
}

func (h *MarketHandler) GetRecentDaysMarketStatistic(days int) []models.MarketStatistic {
	return data.NewMarketStatisticApi().GetRecentDaysData(days)
}
