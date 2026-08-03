package handler

import (
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"strings"
)

// StockHandler handles all stock-related operations: watchlist, groups, K-lines, search
type StockHandler struct{}

// NewStockHandler creates a new StockHandler
func NewStockHandler() *StockHandler {
	return &StockHandler{}
}

// Greet returns stock info - basic welcome/test method
func (h *StockHandler) Greet(stockCode string) *data.StockInfo {
	stockDatas, err := data.NewStockDataApi().GetStockCodeRealTimeData(stockCode)
	if err != nil || stockDatas == nil || len(*stockDatas) == 0 {
		return &data.StockInfo{}
	}
	return &(*stockDatas)[0]
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

// GetStockKLine returns stock K-line data (short-term, quick)
func (h *StockHandler) GetStockKLine(stockCode, stockName string, days int64) *[]data.KLineData {
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

// GetStockCommonKLine returns common K-line data
func (h *StockHandler) GetStockCommonKLine(stockCode, stockName string, days int64) *[]data.KLineData {
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

// GetStockKLineWithFallback returns K-line data with source fallback strategy
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
	return data.FetchKLineWithFallback(stockCode, stockName, klt, limit, "")
}

// GetStockKLinePageWithFallback returns K-line data with pagination and source fallback
func (h *StockHandler) GetStockKLinePageWithFallback(stockCode, stockName string, klt string, limit int, end string) *data.KLineSourceResult {
	if limit <= 0 {
		limit = 500
	}
	if limit > 5000 {
		limit = 5000
	}
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
