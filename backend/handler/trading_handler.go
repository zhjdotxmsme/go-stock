package handler

import (
	"go-stock/backend/data"
)

// TradingRecordHandler handles trading-record-related Wails bindings.
type TradingRecordHandler struct{}

// NewTradingRecordHandler creates a new TradingRecordHandler.
func NewTradingRecordHandler() *TradingRecordHandler {
	return &TradingRecordHandler{}
}

// AddTradingRecord 添加交易记录
func (h *TradingRecordHandler) AddTradingRecord(record data.TradingRecord) (uint, error) {
	return data.NewStockDataApi().AddTradingRecord(record)
}

// GetTradingRecordList 获取交易记录列表（分页与筛选，返回结构与 AI 推荐列表一致）
func (h *TradingRecordHandler) GetTradingRecordList(query data.TradingRecordListQuery) *data.TradingRecordPageData {
	page, err := data.NewStockDataApi().GetTradingRecordList(query)
	if err != nil {
		return &data.TradingRecordPageData{}
	}
	return page
}

// GetTradingRecordById 根据ID获取单个交易记录
func (h *TradingRecordHandler) GetTradingRecordById(id uint) (*data.TradingRecord, error) {
	return data.NewStockDataApi().GetTradingRecordById(id)
}

// GetTradingRecordStatistics 获取交易记录统计数据
func (h *TradingRecordHandler) GetTradingRecordStatistics() *data.TradingRecordStatistics {
	stats, err := data.NewStockDataApi().GetTradingRecordStatistics()
	if err != nil {
		return &data.TradingRecordStatistics{}
	}
	return stats
}

// UpdateTradingRecord 更新交易记录
func (h *TradingRecordHandler) UpdateTradingRecord(record data.TradingRecord) error {
	return data.NewStockDataApi().UpdateTradingRecord(record)
}

// DeleteTradingRecord 删除交易记录
func (h *TradingRecordHandler) DeleteTradingRecord(id uint) error {
	return data.NewStockDataApi().DeleteTradingRecord(id)
}

// CheckFrequentTrading 检查是否频繁交易
func (h *TradingRecordHandler) CheckFrequentTrading(stockCode string) map[string]any {
	canTrade, msg := data.NewStockDataApi().CheckFrequentTrading(stockCode)
	return map[string]any{
		"canTrade": canTrade,
		"msg":      msg,
	}
}
