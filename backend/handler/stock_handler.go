package handler

import (
	"context"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/data/datasource"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"strconv"
	"strings"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/mathutil"
)

// StockHandler handles all stock-related operations: watchlist, groups, K-lines, search
type StockHandler struct{}

// NewStockHandler creates a new StockHandler
func NewStockHandler() *StockHandler {
	return &StockHandler{}
}

// Greet returns stock info - basic welcome/test method
// 语义与 app.go 原 App.Greet 一致：从关注列表加载股票（含分组、成本价等），
// 叠加实时行情并计算涨跌幅/盈亏字段。
func (h *StockHandler) Greet(stockCode string) *data.StockInfo {
	follow := &data.FollowedStock{
		StockCode: stockCode,
	}
	db.Dao.Model(follow).Where("stock_code = ?", stockCode).Preload("Groups").Preload("Groups.GroupInfo").First(follow)
	return getStockInfo(*follow)
}

// getStockInfo 从 app.go 复制：获取实时行情并叠加关注数据。
func getStockInfo(follow data.FollowedStock) *data.StockInfo {
	stockCode := follow.StockCode
	stockDatas, err := data.NewStockDataApi().GetStockCodeRealTimeData(stockCode)
	if err != nil || stockDatas == nil || len(*stockDatas) == 0 {
		return &data.StockInfo{}
	}
	stockData := (*stockDatas)[0]
	addStockFollowData(follow, &stockData)
	return &stockData
}

// addStockFollowData 从 app.go 复制：将关注信息（成本价、分组、报警阈值等）叠加到行情数据，
// 并计算涨跌幅与盈亏字段；价格变化时异步回写关注表。
func addStockFollowData(follow data.FollowedStock, stockData *data.StockInfo) {
	stockData.PrePrice = follow.Price //上次当前价格
	stockData.Sort = follow.Sort
	stockData.CostPrice = follow.CostPrice //成本价
	stockData.CostVolume = follow.Volume   //成本量
	stockData.AlarmChangePercent = follow.AlarmChangePercent
	stockData.AlarmPrice = follow.AlarmPrice
	stockData.Groups = follow.Groups

	//当前价格
	price, _ := convertor.ToFloat(stockData.Price)
	//当前价格为0 时 使用卖一价格作为当前价格
	if price == 0 {
		price, _ = convertor.ToFloat(stockData.A1P)
	}
	//当前价格依然为0 时 使用买一报价作为当前价格
	if price == 0 {
		price, _ = convertor.ToFloat(stockData.B1P)
	}

	//昨日收盘价
	preClosePrice, _ := convertor.ToFloat(stockData.PreClose)

	//当前价格依然为0 时 使用昨日收盘价为当前价格
	if price == 0 {
		price = preClosePrice
	}

	//今日最高价
	highPrice, _ := convertor.ToFloat(stockData.High)
	if highPrice == 0 {
		highPrice, _ = convertor.ToFloat(stockData.Open)
	}

	//今日最低价
	lowPrice, _ := convertor.ToFloat(stockData.Low)
	if lowPrice == 0 {
		lowPrice, _ = convertor.ToFloat(stockData.Open)
	}

	if price > 0 && preClosePrice > 0 {
		stockData.ChangePrice = mathutil.RoundToFloat(price-preClosePrice, 2)
		stockData.ChangePercent = mathutil.RoundToFloat(mathutil.Div(price-preClosePrice, preClosePrice)*100, 3)
	}
	if highPrice > 0 && preClosePrice > 0 {
		stockData.HighRate = mathutil.RoundToFloat(mathutil.Div(highPrice-preClosePrice, preClosePrice)*100, 3)
	}
	if lowPrice > 0 && preClosePrice > 0 {
		stockData.LowRate = mathutil.RoundToFloat(mathutil.Div(lowPrice-preClosePrice, preClosePrice)*100, 3)
	}
	if follow.CostPrice > 0 && follow.Volume > 0 {
		if price > 0 {
			stockData.Profit = mathutil.RoundToFloat(mathutil.Div(price-follow.CostPrice, follow.CostPrice)*100, 3)
			stockData.ProfitAmount = mathutil.RoundToFloat((price-follow.CostPrice)*float64(follow.Volume), 2)
			stockData.ProfitAmountToday = mathutil.RoundToFloat((price-preClosePrice)*float64(follow.Volume), 2)
		} else {
			//未开盘时当前价格为昨日收盘价
			stockData.Profit = mathutil.RoundToFloat(mathutil.Div(preClosePrice-follow.CostPrice, follow.CostPrice)*100, 3)
			stockData.ProfitAmount = mathutil.RoundToFloat((preClosePrice-follow.CostPrice)*float64(follow.Volume), 2)
			// 未开盘时，今日盈亏为 0
			stockData.ProfitAmountToday = 0
		}

	}

	if follow.Price != price && price > 0 {
		go db.Dao.Model(follow).Where("stock_code = ?", follow.StockCode).Updates(map[string]interface{}{
			"price": price,
		})
	}
}

// Follow adds a stock to watchlist
func (h *StockHandler) Follow(stockCode string) string {
	return data.NewStockDataApi().Follow(stockCode)
}

// UnFollow removes a stock from watchlist
func (h *StockHandler) UnFollow(stockCode string) string {
	return data.NewStockDataApi().UnFollow(stockCode)
}

// GetFollowList returns all followed stocks for a group
func (h *StockHandler) GetFollowList(groupId int) *[]data.FollowedStock {
	return data.NewStockDataApi().GetFollowList(groupId)
}

// GetStockList searches stocks by keyword
func (h *StockHandler) GetStockList(key string) []data.StockBasic {
	return data.NewStockDataApi().GetStockList(key)
}

// SetCostPriceAndVolume sets cost price and volume for a stock
func (h *StockHandler) SetCostPriceAndVolume(stockCode string, price float64, volume int64) string {
	return data.NewStockDataApi().SetCostPriceAndVolume(price, volume, stockCode)
}

// SetTradingPrice sets trading-related prices for a stock
func (h *StockHandler) SetTradingPrice(stockCode string, entryPrice, takeProfitPrice, stopLossPrice, costPrice float64) string {
	return data.NewStockDataApi().SetTradingPrice(entryPrice, takeProfitPrice, stopLossPrice, costPrice, stockCode)
}

// SetAlarmChangePercent sets alarm threshold for a stock
func (h *StockHandler) SetAlarmChangePercent(val, alarmPrice float64, stockCode string) string {
	return data.NewStockDataApi().SetAlarmChangePercent(val, alarmPrice, stockCode)
}

// SetStockSort sets sort order for a stock in watchlist
func (h *StockHandler) SetStockSort(sort int64, stockCode string) {
	data.NewStockDataApi().SetStockSort(sort, stockCode)
}

// AddGroup adds a new stock group
func (h *StockHandler) AddGroup(group data.Group) string {
	ok := data.NewStockGroupApi(db.Dao).AddGroup(group)
	if ok {
		return "添加成功"
	}
	return "添加失败"
}

// GetGroupList returns all stock groups
func (h *StockHandler) GetGroupList() []data.Group {
	return data.NewStockGroupApi(db.Dao).GetGroupList()
}

// UpdateGroupSort updates sort order for a group
func (h *StockHandler) UpdateGroupSort(id int, newSort int) bool {
	return data.NewStockGroupApi(db.Dao).UpdateGroupSort(id, newSort)
}

// InitializeGroupSort initializes sort order for all groups
func (h *StockHandler) InitializeGroupSort() bool {
	return data.NewStockGroupApi(db.Dao).InitializeGroupSort()
}

// GetGroupStockList returns stocks in a group
func (h *StockHandler) GetGroupStockList(groupId int) []data.GroupStock {
	return data.NewStockGroupApi(db.Dao).GetGroupStockByGroupId(groupId)
}

// AddStockGroup adds a stock to a group
func (h *StockHandler) AddStockGroup(groupId int, stockCode string) string {
	ok := data.NewStockGroupApi(db.Dao).AddStockGroup(groupId, stockCode)
	if ok {
		return "添加成功"
	}
	return "添加失败"
}

// RemoveStockGroup removes a stock from a group
func (h *StockHandler) RemoveStockGroup(code, name string, groupId int) string {
	ok := data.NewStockGroupApi(db.Dao).RemoveStockGroup(code, name, groupId)
	if ok {
		return "移除成功"
	}
	return "移除失败"
}

// RemoveGroup removes a stock group
func (h *StockHandler) RemoveGroup(groupId int) string {
	ok := data.NewStockGroupApi(db.Dao).RemoveGroup(groupId)
	if ok {
		return "移除成功"
	}
	return "移除失败"
}

// GetStockKLine returns stock K-line data (short-term, quick).
// Day klines go through the datasource Router (same tencent upstream plus
// multi-source fallback and caching); legacy direct call kept as fallback.
func (h *StockHandler) GetStockKLine(stockCode, stockName string, days int64) *[]data.KLineData {
	if result := h.getKLineViaRouter(stockCode, "101", int(days)); result != nil {
		return result.Data
	}
	return data.NewStockDataApi().GetHK_KLineData(stockCode, "day", days)
}

// GetStockMinutePriceLineData returns minute-level price data
func (h *StockHandler) GetStockMinutePriceLineData(stockCode, stockName string) map[string]any {
	res := make(map[string]any, 4)
	priceData, date := data.NewStockDataApi().GetStockMinutePriceData(stockCode)
	res["priceData"] = priceData
	res["date"] = date
	res["stockName"] = stockName
	res["stockCode"] = stockCode
	return res
}

// GetStockCommonKLine returns common K-line data via the datasource Router
// (same tencent fqkline upstream, plus fallback and caching).
func (h *StockHandler) GetStockCommonKLine(stockCode, stockName string, days int64) *[]data.KLineData {
	if result := h.getKLineViaRouter(stockCode, "101", int(days)); result != nil {
		return result.Data
	}
	return data.NewStockDataApi().GetCommonKLineData(stockCode, "day", days)
}

// GetStockEastMoneyKLine returns EastMoney K-line data
func (h *StockHandler) GetStockEastMoneyKLine(stockCode, stockName string, klt string, limit int) *[]data.KLineData {
	return h.GetStockEastMoneyKLinePage(stockCode, stockName, klt, limit, "")
}

// GetStockEastMoneyKLinePage returns EastMoney K-line data with pagination
func (h *StockHandler) GetStockEastMoneyKLinePage(stockCode, stockName string, klt string, limit int, end string) *[]data.KLineData {
	if limit <= 0 {
		limit = 500
	}
	if limit > 5000 {
		limit = 5000
	}
	klt = strings.TrimSpace(klt)
	if klt == "" {
		klt = "1"
	}
	api := data.NewEastMoneyKLineApi(data.GetSettingConfig())
	end = strings.TrimSpace(end)
	return api.GetKLineDataBefore(stockCode, klt, "", limit, end)
}

// GetStockKLineWithFallback returns K-line data with source fallback strategy.
// Day lines (klt=101) go through the datasource Router (cache + multi-source
// fallback + SQLite persistence); other periods keep the legacy serial chain
// because Router providers do not uniformly cover intraday klt codes.
func (h *StockHandler) GetStockKLineWithFallback(stockCode, stockName string, klt string, limit int) *data.KLineSourceResult {
	if limit <= 0 {
		limit = 500
	}
	if limit > 5000 {
		limit = 5000
	}
	klt = strings.TrimSpace(klt)
	if klt == "" {
		klt = "101"
	}
	if klt == "101" {
		if result := h.getKLineViaRouter(stockCode, klt, limit); result != nil {
			return result
		}
	}
	return data.FetchKLineWithFallback(stockCode, stockName, klt, limit, "")
}

// getKLineViaRouter fetches a day kline via the datasource Router and maps it
// to the legacy wire format. Returns nil on failure so callers can fall back
// to the legacy chain.
func (h *StockHandler) getKLineViaRouter(stockCode, klt string, limit int) *data.KLineSourceResult {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	k, err := datasource.GetRouter().GetKLine(ctx, stockCode, "day", limit)
	if err != nil || k == nil || len(k.Bars) == 0 {
		return nil
	}
	return barsToLegacyKLineResult(stockCode, k)
}

// barsToLegacyKLineResult converts Router float64 bars to the legacy
// string-field wire format consumed by the frontend.
func barsToLegacyKLineResult(stockCode string, k *datasource.KLineData) *data.KLineSourceResult {
	rows := make([]data.KLineData, 0, len(k.Bars))
	dayOnly := k.Period == "" || k.Period == "day" || k.Period == "101"
	for _, b := range k.Bars {
		layout := "2006-01-02 15:04:05"
		if dayOnly {
			layout = "2006-01-02"
		}
		rows = append(rows, data.KLineData{
			Day:    b.Time.Format(layout),
			Open:   strconv.FormatFloat(b.Open, 'f', 2, 64),
			Close:  strconv.FormatFloat(b.Close, 'f', 2, 64),
			High:   strconv.FormatFloat(b.High, 'f', 2, 64),
			Low:    strconv.FormatFloat(b.Low, 'f', 2, 64),
			Volume: strconv.FormatInt(b.Volume, 10),
		})
	}
	return &data.KLineSourceResult{Data: &rows, Source: k.Source}
}

// GetStockKLinePageWithFallback returns K-line data with pagination and source fallback
func (h *StockHandler) GetStockKLinePageWithFallback(stockCode, stockName string, klt string, limit int, end string) *data.KLineSourceResult {
	if limit <= 0 {
		limit = 500
	}
	if limit > 5000 {
		limit = 5000
	}
	klt = strings.TrimSpace(klt)
	if klt == "" {
		klt = "101"
	}
	end = strings.TrimSpace(end)
	return data.FetchKLineWithFallback(stockCode, stockName, klt, limit, end)
}

// GetChipDistribution returns chip distribution analysis
func (h *StockHandler) GetChipDistribution(stockCode string, days int, bins int, adjustFlag string) (*data.ChipDistributionResult, error) {
	stockCode = strings.TrimSpace(stockCode)
	if stockCode == "" {
		return nil, fmt.Errorf("stockCode 不能为空")
	}
	if days <= 0 {
		days = 120
	}
	if bins <= 0 {
		bins = 80
	}
	adjustFlag = strings.TrimSpace(strings.ToLower(adjustFlag))
	if adjustFlag != "" && adjustFlag != "qfq" && adjustFlag != "hfq" {
		adjustFlag = "qfq"
	}

	api := data.NewEastMoneyKLineApi(data.GetSettingConfig())
	if !api.ValidateStockCode(stockCode) {
		return nil, fmt.Errorf("股票代码无效：%s", stockCode)
	}

	var kLines *[]data.KLineData

	if adjustFlag != "" {
		kLines = api.GetKLineData(stockCode, "101", adjustFlag, days)
	} else {
		result := data.FetchKLineWithFallback(stockCode, "", "101", days, "")
		if result != nil && result.Data != nil {
			kLines = result.Data
		}
	}

	if kLines == nil || len(*kLines) == 0 {
		return nil, fmt.Errorf("未获取到K线数据")
	}
	calculator := data.NewChipDistributionCalculator()
	return calculator.Calculate(stockCode, *kLines, bins)
}

// GetTdxCallAuction returns TongDaXin call auction data
func (h *StockHandler) GetTdxCallAuction(stockCode string, start uint32, count uint32) *[]data.TdxCallAuctionData {
	if count <= 0 {
		count = 500
	}
	api := data.NewTdxKLineApi()
	return api.GetCallAuction(stockCode, start, count)
}

// GetTdxCompanyInfo returns company info from TDX
func (h *StockHandler) GetTdxCompanyInfo(stockCode string) *data.TdxCompanyInfoBundle {
	api := data.NewTdxKLineApi()
	return api.GetF10Data(stockCode)
}

// GetTdxFinanceInfo returns finance info from TDX
func (h *StockHandler) GetTdxFinanceInfo(stockCode string) *data.TdxFinanceInfo {
	api := data.NewTdxKLineApi()
	return api.GetFinanceInfo(stockCode)
}

// GetTdxXDXRInfo returns XDXR (dividend/rights) info from TDX
func (h *StockHandler) GetTdxXDXRInfo(stockCode string) *[]data.TdxXDXRItem {
	api := data.NewTdxKLineApi()
	return api.GetXDXRInfo(stockCode)
}

// GetTdxCompanyCategoryList returns F10 category list from TDX
func (h *StockHandler) GetTdxCompanyCategoryList(stockCode string) *[]data.TdxCompanyCategory {
	api := data.NewTdxKLineApi()
	return api.GetF10CategoryList(stockCode)
}

// GetTdxCompanyCategoryContent returns F10 category content from TDX
func (h *StockHandler) GetTdxCompanyCategoryContent(stockCode string, categoryName string) *data.TdxCompanyInfoSection {
	api := data.NewTdxKLineApi()
	return api.GetF10CategoryContent(stockCode, categoryName)
}

// GetTdxSymbolBelongBoard 通过通达信 MAC 接口获取股票所属板块信息
func (h *StockHandler) GetTdxSymbolBelongBoard(stockCode string) *[]data.MACBelongBoardItem {
	api := data.NewTdxKLineApi()
	return api.GetMACSymbolBelongBoard(stockCode)
}

// GetStockRealTimePrice 获取股票实时价格（当前价为 0 时依次回退卖一/买一/昨收）。
// 数据经 datasource Router 快照链（腾讯全字段 → 东财价格兜底），带缓存与降级。
func (h *StockHandler) GetStockRealTimePrice(stockCode string) map[string]any {
	snap, err := datasource.GetRouter().GetSnapshot(context.Background(), stockCode)
	if err != nil || snap == nil {
		return map[string]any{
			"code":    -1,
			"message": "获取股票价格失败",
			"price":   0,
		}
	}
	price, name := resolveSnapshotPrice(snap)
	return map[string]any{
		"code":    0,
		"message": "success",
		"price":   price,
		"name":    name,
	}
}

// resolveSnapshotPrice 复刻原 StockInfo 快照的价格回退链：
// 当前价 → 卖一 → 买一 → 昨收，返回 (price, name)。
func resolveSnapshotPrice(snap *datasource.SnapshotData) (float64, string) {
	if snap == nil {
		return 0, ""
	}
	price := snap.Price
	if price == 0 {
		price = snap.A1P
	}
	if price == 0 {
		price = snap.B1P
	}
	if price == 0 {
		price = snap.PreClose
	}
	return price, snap.Name
}

// GetAllStockInfoList 获取股票基本信息列表（分页）
func (h *StockHandler) GetAllStockInfoList(query data.AllStockInfoQuery) *data.AllStockInfoPageData {
	page, err := data.NewStockDataApi().GetAllStockInfoList(&query)
	if err != nil {
		return &data.AllStockInfoPageData{}
	}
	return page
}

// GetAllStockInfoById 根据 ID 获取股票基本信息
func (h *StockHandler) GetAllStockInfoById(id uint) *models.AllStockInfo {
	stock, err := data.NewStockDataApi().GetAllStockInfoById(id)
	if err != nil {
		return &models.AllStockInfo{}
	}
	return stock
}

// AddAllStockInfo 新增股票基本信息
func (h *StockHandler) AddAllStockInfo(stock models.AllStockInfo) string {
	err := data.NewStockDataApi().AddAllStockInfo(stock)
	if err != nil {
		return "操作失败: " + err.Error()
	}
	return "操作成功"
}

// DeleteAllStockInfo 删除股票基本信息
func (h *StockHandler) DeleteAllStockInfo(id uint) string {
	err := data.NewStockDataApi().DeleteAllStockInfo(id)
	if err != nil {
		return "删除失败: " + err.Error()
	}
	return "删除成功"
}

// BatchDeleteAllStockInfo 批量删除股票基本信息
func (h *StockHandler) BatchDeleteAllStockInfo(ids []uint) string {
	err := data.NewStockDataApi().BatchDeleteAllStockInfo(ids)
	if err != nil {
		return "批量删除失败: " + err.Error()
	}
	return "批量删除成功"
}

// GetAllMarkets 获取所有市场分类
func (h *StockHandler) GetAllMarkets() []string {
	markets, err := data.NewStockDataApi().GetAllMarkets()
	if err != nil {
		return []string{}
	}
	return markets
}

// GetAllIndustries 获取所有行业分类
func (h *StockHandler) GetAllIndustries() []string {
	industries, err := data.NewStockDataApi().GetAllIndustries()
	if err != nil {
		return []string{}
	}
	return industries
}

// GetAllConcepts 获取所有概念分类
func (h *StockHandler) GetAllConcepts() []string {
	concepts, err := data.NewStockDataApi().GetAllConcepts()
	if err != nil {
		return []string{}
	}
	return concepts
}
