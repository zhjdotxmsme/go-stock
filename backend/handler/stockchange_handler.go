package handler

import (
	"fmt"

	"go-stock/backend/data"
	"go-stock/backend/models"
)

// StockChangeHandler handles stock-change (异动) monitoring Wails bindings.
type StockChangeHandler struct{}

// NewStockChangeHandler creates a new StockChangeHandler.
func NewStockChangeHandler() *StockChangeHandler {
	return &StockChangeHandler{}
}

func (h *StockChangeHandler) GetStockChanges(changeTypes []int, pageIndex, pageSize int) *data.StockChangesResponse {
	return data.NewStockChangesApi().GetStockChanges(changeTypes, pageIndex, pageSize)
}

func (h *StockChangeHandler) GetAllStockChangesWithPaging(pageSize int) *data.StockChangesResponse {
	all := data.NewStockChangesApi().GetAllStockChangesWithPaging(pageSize)
	historyService := data.NewStockChangeHistoryService()
	_, _ = historyService.SaveStockChangesWithDedup(all.Data)
	return all
}

func (h *StockChangeHandler) GetStockChangeHistory(query models.StockChangeHistoryQuery) *models.StockChangeHistoryPageData {
	result, err := data.NewStockChangeHistoryService().GetHistoryList(query)
	if err != nil {
		return &models.StockChangeHistoryPageData{}
	}
	return result
}

func (h *StockChangeHandler) SaveStockChangesToHistory(changeTypes []int) string {
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

func (h *StockChangeHandler) DeleteStockChangeHistory(days int) string {
	err := data.NewStockChangeHistoryService().DeleteOldData(days)
	if err != nil {
		return "删除失败: " + err.Error()
	}
	return fmt.Sprintf("已删除 %d 天前的历史数据", days)
}

func (h *StockChangeHandler) GetDailyChangeStats(days int) []data.DailyChangeStats {
	result, err := data.NewStockChangeHistoryService().GetDailyChangeStats(days)
	if err != nil {
		return []data.DailyChangeStats{}
	}
	return result
}

func (h *StockChangeHandler) GetChangeTypeDailyStats(days int) []data.ChangeTypeDailyStats {
	result, err := data.NewStockChangeHistoryService().GetChangeTypeDailyStats(days)
	if err != nil {
		return []data.ChangeTypeDailyStats{}
	}
	return result
}

func (h *StockChangeHandler) GetChangeRank(days int, topN int) *data.ChangeRankResult {
	result, err := data.NewStockChangeHistoryService().GetChangeRank(days, topN)
	if err != nil {
		return &data.ChangeRankResult{}
	}
	return result
}

func (h *StockChangeHandler) GetDailyDimensionStats(dimension string, name string, days int) []data.DailyDimensionStats {
	result, err := data.NewStockChangeHistoryService().GetDailyDimensionStats(dimension, name, days)
	if err != nil {
		return []data.DailyDimensionStats{}
	}
	return result
}

func (h *StockChangeHandler) GetTypeStatsByDate(date string) []data.TypeCountStats {
	result, err := data.NewStockChangeHistoryService().GetTypeStatsByDate(date)
	if err != nil {
		return []data.TypeCountStats{}
	}
	return result
}
