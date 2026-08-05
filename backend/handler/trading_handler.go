package handler

import (
	"context"

	"github.com/duke-git/lancet/v2/convertor"

	"go-stock/backend/data"
	"go-stock/backend/internal/adapter/repository/sqlite"
	"go-stock/backend/internal/service/trading"
)

// TradingRecordHandler handles trading-record-related Wails bindings.
// It keeps data-layer types in its signatures (so App and the frontend see no
// change) and delegates business logic to the trading service.
type TradingRecordHandler struct {
	svc *trading.Service
}

// NewTradingRecordHandler creates a new TradingRecordHandler.
func NewTradingRecordHandler(svc *trading.Service) *TradingRecordHandler {
	return &TradingRecordHandler{svc: svc}
}

// NewDefaultTradingRecordHandler wires the production dependencies
// (sqlite repository + realtime price lookup) and returns the handler.
// The wiring lives here because backend/internal packages cannot be
// imported by the main package at the repository root.
func NewDefaultTradingRecordHandler() *TradingRecordHandler {
	priceFn := func(stockCode string) (float64, error) {
		stockDatas, err := data.NewStockDataApi().GetStockCodeRealTimeData(stockCode)
		if err != nil || stockDatas == nil || len(*stockDatas) == 0 {
			return 0, err
		}
		price, _ := convertor.ToFloat((*stockDatas)[0].Price)
		if price == 0 {
			price, _ = convertor.ToFloat((*stockDatas)[0].A1P)
		}
		return price, nil
	}
	return NewTradingRecordHandler(trading.NewService(sqlite.NewStockRepository(), priceFn))
}

// AddTradingRecord 添加交易记录
func (h *TradingRecordHandler) AddTradingRecord(record data.TradingRecord) (uint, error) {
	return h.svc.AddTradingRecord(context.Background(), sqlite.TradingRecordToDomain(&record))
}

// GetTradingRecordList 获取交易记录列表（分页与筛选，返回结构与 AI 推荐列表一致）
func (h *TradingRecordHandler) GetTradingRecordList(query data.TradingRecordListQuery) *data.TradingRecordPageData {
	page, err := h.svc.GetTradingRecordList(context.Background(), sqlite.TradingRecordListQueryToDomain(query))
	if err != nil {
		return &data.TradingRecordPageData{}
	}
	return sqlite.TradingRecordPageDataFromDomain(&page)
}

// GetTradingRecordById 根据ID获取单个交易记录
func (h *TradingRecordHandler) GetTradingRecordById(id uint) (*data.TradingRecord, error) {
	record, err := h.svc.GetTradingRecordById(context.Background(), id)
	if err != nil || record == nil {
		return nil, err
	}
	return sqlite.TradingRecordFromDomain(record), nil
}

// GetTradingRecordStatistics 获取交易记录统计数据
func (h *TradingRecordHandler) GetTradingRecordStatistics() *data.TradingRecordStatistics {
	stats, err := h.svc.GetTradingRecordStatistics(context.Background())
	if err != nil {
		return &data.TradingRecordStatistics{}
	}
	return &data.TradingRecordStatistics{
		TotalBuyAmount:  stats.TotalBuyAmount,
		TotalSellAmount: stats.TotalSellAmount,
		TotalProfit:     stats.TotalProfit,
		ProfitRate:      stats.ProfitRate,
		HoldingsAmount:  stats.HoldingsAmount,
		CurrentValue:    stats.CurrentValue,
		StockCount:      stats.StockCount,
	}
}

// UpdateTradingRecord 更新交易记录
func (h *TradingRecordHandler) UpdateTradingRecord(record data.TradingRecord) error {
	return h.svc.UpdateTradingRecord(context.Background(), sqlite.TradingRecordToDomain(&record))
}

// DeleteTradingRecord 删除交易记录
func (h *TradingRecordHandler) DeleteTradingRecord(id uint) error {
	return h.svc.DeleteTradingRecord(context.Background(), id)
}

// CheckFrequentTrading 检查是否频繁交易
func (h *TradingRecordHandler) CheckFrequentTrading(stockCode string) map[string]any {
	canTrade, msg := h.svc.CheckFrequentTrading(context.Background(), stockCode)
	return map[string]any{
		"canTrade": canTrade,
		"msg":      msg,
	}
}
