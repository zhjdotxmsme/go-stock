package handler

import (
	"context"

	"go-stock/backend/data"
	"go-stock/backend/internal/adapter/repository/sqlite"
	"go-stock/backend/internal/domain/fund"
	fundsvc "go-stock/backend/internal/service/fund"
)

// FundHandler handles fund-related Wails bindings.
// External data-source calls (ranking/K-line/net-value/holdings/search) still
// go directly through the data-layer API; the followed-fund CRUD is delegated
// to the fund service.
type FundHandler struct {
	ctxFn func() context.Context
	svc *fundsvc.Service
}

// NewFundHandler creates a new FundHandler.
func NewFundHandler(ctxFn func() context.Context, svc *fundsvc.Service) *FundHandler {
	return &FundHandler{ctxFn: ctxFn, svc: svc}
}

// NewDefaultFundHandler wires the production dependencies (sqlite repository
// + fund-basic crawl) and returns the handler. The wiring lives here because
// backend/internal packages cannot be imported by the main package.
func NewDefaultFundHandler(ctxFn func() context.Context) *FundHandler {
	crawlFn := func(fundCode string) (*fund.FundBasic, error) {
		crawled, err := data.NewFundApi().CrawlFundBasic(fundCode)
		if err != nil || crawled == nil {
			return nil, err
		}
		return sqlite.FundBasicToDomain(crawled), nil
	}
	return NewFundHandler(ctxFn, fundsvc.NewService(sqlite.NewFundRepository(), crawlFn))
}

// GetfundList searches fund basic info by keyword.
func (h *FundHandler) GetfundList(key string) []data.FundBasic {
	return data.NewFundApi().GetFundList(key)
}

// GetFollowedFund returns all followed funds.
func (h *FundHandler) GetFollowedFund() []data.FollowedFund {
	funds, err := h.svc.GetFollowedFund(h.currentCtx())
	if err != nil {
		return []data.FollowedFund{}
	}
	result := make([]data.FollowedFund, 0, len(funds))
	for i := range funds {
		result = append(result, *sqlite.FollowedFundFromDomain(&funds[i]))
	}
	return result
}

// FollowFund adds a fund to the follow list.
func (h *FundHandler) FollowFund(fundCode string) string {
	return h.svc.FollowFund(h.currentCtx(), fundCode)
}

// UnFollowFund removes a fund from the follow list.
func (h *FundHandler) UnFollowFund(fundCode string) string {
	return h.svc.UnFollowFund(h.currentCtx(), fundCode)
}

// GetFundKLine returns fund K-line data with source fallback.
func (h *FundHandler) GetFundKLine(fundCode string, klt string, limit int) *data.KLineSourceResult {
	return data.NewFundKLineApi().GetFundKLineWithFallback(fundCode, klt, limit)
}

// GetFundHistoryNetValue returns fund historical net value.
func (h *FundHandler) GetFundHistoryNetValue(fundCode string, pageSize int, startDate string, endDate string) []data.FundHistoryNetValue {
	res, _ := data.NewFundApi().GetFundHistoryNetValue(fundCode, 1, pageSize, startDate, endDate)
	if res == nil {
		return []data.FundHistoryNetValue{}
	}
	return res
}

// GetFundTop10Holdings returns the top 10 holdings of a fund.
func (h *FundHandler) GetFundTop10Holdings(fundCode string) []data.FundHoldingStock {
	res, err := data.NewFundApi().GetFundTop10Holdings(fundCode)
	if err != nil || res == nil {
		return []data.FundHoldingStock{}
	}
	return res
}

// GetFundRanking returns fund ranking list.
func (h *FundHandler) GetFundRanking(marketType, fundType, sortField, sortOrder string, pageIndex, pageSize int) *data.FundRankingResult {
	res, err := data.NewFundApi().GetFundRanking(marketType, fundType, sortField, sortOrder, pageIndex, pageSize)
	if err != nil || res == nil {
		return &data.FundRankingResult{}
	}
	return res
}

// SearchFundCodes searches funds by keyword.
func (h *FundHandler) SearchFundCodes(keyword string) []data.FundSearchItem {
	return data.NewFundApi().SearchFundCodes(keyword)
}

// GetFollowedFundPaged returns followed funds with pagination.
func (h *FundHandler) GetFollowedFundPaged(pageIndex, pageSize int, keyword string) *data.FollowedFundPagedResult {
	paged, err := h.svc.GetFollowedFundPaged(h.currentCtx(), pageIndex, pageSize, keyword)
	if err != nil || paged == nil {
		return &data.FollowedFundPagedResult{}
	}
	return sqlite.FollowedFundPagedResultFromDomain(paged)
}

// currentCtx returns the Wails app context (set after startup), falling back
// to context.Background when not wired — so in-flight service calls observe
// app shutdown instead of running detached.
func (h *FundHandler) currentCtx() context.Context {
	if h.ctxFn != nil {
		return h.ctxFn()
	}
	return context.Background()
}
