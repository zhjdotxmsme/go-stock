package handler

import (
	"go-stock/backend/data"
)

// FundHandler handles fund-related Wails bindings.
type FundHandler struct{}

// NewFundHandler creates a new FundHandler.
func NewFundHandler() *FundHandler {
	return &FundHandler{}
}

// GetfundList searches fund basic info by keyword.
func (h *FundHandler) GetfundList(key string) []data.FundBasic {
	return data.NewFundApi().GetFundList(key)
}

// GetFollowedFund returns all followed funds.
func (h *FundHandler) GetFollowedFund() []data.FollowedFund {
	return data.NewFundApi().GetFollowedFund()
}

// FollowFund adds a fund to the follow list.
func (h *FundHandler) FollowFund(fundCode string) string {
	return data.NewFundApi().FollowFund(fundCode)
}

// UnFollowFund removes a fund from the follow list.
func (h *FundHandler) UnFollowFund(fundCode string) string {
	return data.NewFundApi().UnFollowFund(fundCode)
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
	return data.NewFundApi().GetFollowedFundPaged(pageIndex, pageSize, keyword)
}
