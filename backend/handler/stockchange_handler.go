package handler

import (
	"context"

	"go-stock/backend/data"
	"go-stock/backend/internal/adapter/repository/sqlite"
	"go-stock/backend/internal/domain/stock"
	"go-stock/backend/internal/service/stockchange"
	"go-stock/backend/models"
)

// StockChangeHandler handles stock-change (异动) monitoring Wails bindings.
// It keeps data/models-layer types in its signatures (so App and the frontend
// see no change) and delegates business logic to the stockchange service.
// Realtime change items are still fetched directly via the data-layer API.
type StockChangeHandler struct {
	svc *stockchange.Service
}

// NewStockChangeHandler creates a new StockChangeHandler.
func NewStockChangeHandler(svc *stockchange.Service) *StockChangeHandler {
	return &StockChangeHandler{svc: svc}
}

// NewDefaultStockChangeHandler wires the production dependencies
// (sqlite repository) and returns the handler. The wiring lives here
// because backend/internal packages cannot be imported by the main package.
func NewDefaultStockChangeHandler() *StockChangeHandler {
	return NewStockChangeHandler(stockchange.NewService(sqlite.NewStockRepository()))
}

// stockChangeItemsToDomain maps realtime change items to domain items.
func stockChangeItemsToDomain(items []data.StockChangeItem) []stock.StockChangeItem {
	result := make([]stock.StockChangeItem, 0, len(items))
	for _, item := range items {
		result = append(result, stock.StockChangeItem{
			Time:       item.Time,
			Code:       item.Code,
			Name:       item.Name,
			Market:     item.Market,
			ChangeType: item.ChangeType,
			TypeName:   item.TypeName,
			Volume:     item.Volume,
			Price:      item.Price,
			ChangeRate: item.ChangeRate,
			Amount:     item.Amount,
			Industry:   item.Industry,
			Concept:    item.Concept,
		})
	}
	return result
}

func (h *StockChangeHandler) GetStockChanges(changeTypes []int, pageIndex, pageSize int) *data.StockChangesResponse {
	return data.NewStockChangesApi().GetStockChanges(changeTypes, pageIndex, pageSize)
}

func (h *StockChangeHandler) GetAllStockChangesWithPaging(pageSize int) *data.StockChangesResponse {
	all := data.NewStockChangesApi().GetAllStockChangesWithPaging(pageSize)
	_, _ = h.svc.SaveStockChangesWithDedup(context.Background(), stockChangeItemsToDomain(all.Data))
	return all
}

func (h *StockChangeHandler) GetStockChangeHistory(query models.StockChangeHistoryQuery) *models.StockChangeHistoryPageData {
	result, err := h.svc.GetStockChangeHistory(context.Background(), stock.StockChangeHistoryQuery{
		StockCode:     query.StockCode,
		StockName:     query.StockName,
		ChangeType:    query.ChangeType,
		ChangeTypes:   query.ChangeTypes,
		TypeName:      query.TypeName,
		StartDate:     query.StartDate,
		EndDate:       query.EndDate,
		StartTime:     query.StartTime,
		EndTime:       query.EndTime,
		MinVolume:     query.MinVolume,
		MinAmount:     query.MinAmount,
		MinChangeRate: query.MinChangeRate,
		MaxChangeRate: query.MaxChangeRate,
		Industry:      query.Industry,
		Concept:       query.Concept,
		Page:          query.Page,
		PageSize:      query.PageSize,
	})
	if err != nil {
		return &models.StockChangeHistoryPageData{}
	}
	return sqlite.StockChangeHistoryPageDataFromDomain(&result)
}

func (h *StockChangeHandler) SaveStockChangesToHistory(changeTypes []int) string {
	result := data.NewStockChangesApi().GetStockChanges(changeTypes, 0, 500)
	if result == nil {
		return h.svc.SaveStockChangesToHistory(context.Background(), nil)
	}
	return h.svc.SaveStockChangesToHistory(context.Background(), stockChangeItemsToDomain(result.Data))
}

func (h *StockChangeHandler) DeleteStockChangeHistory(days int) string {
	return h.svc.DeleteStockChangeHistory(context.Background(), days)
}

func (h *StockChangeHandler) GetDailyChangeStats(days int) []data.DailyChangeStats {
	result, err := h.svc.GetDailyChangeStats(context.Background(), days)
	if err != nil {
		return []data.DailyChangeStats{}
	}
	out := make([]data.DailyChangeStats, 0, len(result))
	for _, r := range result {
		out = append(out, data.DailyChangeStats{
			ChangeDate: r.ChangeDate,
			TotalCount: r.TotalCount,
			UpCount:    r.UpCount,
			DownCount:  r.DownCount,
			LimitUp:    r.LimitUp,
			LimitDown:  r.LimitDown,
		})
	}
	return out
}

func (h *StockChangeHandler) GetChangeTypeDailyStats(days int) []data.ChangeTypeDailyStats {
	result, err := h.svc.GetChangeTypeDailyStats(context.Background(), days)
	if err != nil {
		return []data.ChangeTypeDailyStats{}
	}
	out := make([]data.ChangeTypeDailyStats, 0, len(result))
	for _, r := range result {
		out = append(out, data.ChangeTypeDailyStats{
			ChangeDate: r.ChangeDate,
			TypeName:   r.TypeName,
			Count:      r.Count,
		})
	}
	return out
}

func (h *StockChangeHandler) GetChangeRank(days int, topN int) *data.ChangeRankResult {
	result, err := h.svc.GetChangeRank(context.Background(), days, topN)
	if err != nil {
		return &data.ChangeRankResult{}
	}
	mapItems := func(items []stock.ChangeRankItem) []data.ChangeRankItem {
		if items == nil {
			return nil
		}
		out := make([]data.ChangeRankItem, 0, len(items))
		for _, item := range items {
			out = append(out, data.ChangeRankItem{
				Name:      item.Name,
				Code:      item.Code,
				Count:     item.Count,
				UpCount:   item.UpCount,
				DownCount: item.DownCount,
			})
		}
		return out
	}
	return &data.ChangeRankResult{
		TopStocks:     mapItems(result.TopStocks),
		TopIndustries: mapItems(result.TopIndustries),
		TopConcepts:   mapItems(result.TopConcepts),
	}
}

func (h *StockChangeHandler) GetDailyDimensionStats(dimension string, name string, days int) []data.DailyDimensionStats {
	result, err := h.svc.GetDailyDimensionStats(context.Background(), dimension, name, days)
	if err != nil {
		return []data.DailyDimensionStats{}
	}
	out := make([]data.DailyDimensionStats, 0, len(result))
	for _, r := range result {
		out = append(out, data.DailyDimensionStats{
			ChangeDate: r.ChangeDate,
			UpCount:    r.UpCount,
			DownCount:  r.DownCount,
			TotalCount: r.TotalCount,
		})
	}
	return out
}

func (h *StockChangeHandler) GetTypeStatsByDate(date string) []data.TypeCountStats {
	result, err := h.svc.GetTypeStatsByDate(context.Background(), date)
	if err != nil {
		return []data.TypeCountStats{}
	}
	out := make([]data.TypeCountStats, 0, len(result))
	for _, r := range result {
		out = append(out, data.TypeCountStats{
			TypeName:   r.TypeName,
			UpCount:    r.UpCount,
			DownCount:  r.DownCount,
			TotalCount: r.TotalCount,
		})
	}
	return out
}
