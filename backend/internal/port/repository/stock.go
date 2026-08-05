package repository

import (
	"context"
	"time"

	"go-stock/backend/internal/domain/stock"
)

// StockRepository abstracts persistence for stock-related entities.
// Implementations live in backend/internal/adapter/repository/sqlite.
type StockRepository interface {
	// Followed stocks
	AddFollow(ctx context.Context, stockCode, stockName string) error
	RemoveFollow(ctx context.Context, stockCode string) error
	GetFollowList(ctx context.Context, groupID int) ([]stock.FollowedStock, error)
	SetCostPriceAndVolume(ctx context.Context, stockCode string, price float64, volume int64) error
	SetTradingPrice(ctx context.Context, stockCode string, entryPrice, takeProfitPrice, stopLossPrice, costPrice float64) error
	SetAlarmChangePercent(ctx context.Context, stockCode string, changePercent, alarmPrice float64) error
	SetStockSort(ctx context.Context, stockCode string, sort int64) error

	// Groups
	AddGroup(ctx context.Context, name string) (*stock.Group, error)
	RemoveGroup(ctx context.Context, groupID int) error
	GetGroupList(ctx context.Context) ([]stock.Group, error)
	AddStockToGroup(ctx context.Context, groupID int, stockCode string) error
	RemoveStockFromGroup(ctx context.Context, groupID int, stockCode, stockName string) error

	// Trading records
	AddTradingRecord(ctx context.Context, record *stock.TradingRecord) error
	GetTradingRecordList(ctx context.Context, query stock.TradingRecordListQuery) (stock.TradingRecordPageData, error)
	GetTradingRecordById(ctx context.Context, id uint) (*stock.TradingRecord, error)
	UpdateTradingRecord(ctx context.Context, record *stock.TradingRecord) error
	DeleteTradingRecord(ctx context.Context, id uint) error
	// ListAllTradingRecords returns all trading records in chronological order
	// (trading_time ASC, id ASC), used by service-level FIFO/statistics computation.
	ListAllTradingRecords(ctx context.Context) ([]stock.TradingRecord, error)
	// CountBuyTradingRecords counts 买入 records with trading_time after since;
	// empty stockCode counts across all stocks.
	CountBuyTradingRecords(ctx context.Context, stockCode string, since time.Time) (int64, error)

	// Stock change history
	// SaveStockChangesToHistory 按 (change_date, stock_code, change_time) 去重落库。
	SaveStockChangesToHistory(ctx context.Context, changes []stock.StockChangeHistory) error
	// SaveStockChangesToHistoryWithDedup 按全字段维度去重落库,返回实际新增条数。
	SaveStockChangesToHistoryWithDedup(ctx context.Context, changes []stock.StockChangeHistory) (int, error)
	GetStockChangeHistory(ctx context.Context, query stock.StockChangeHistoryQuery) (stock.StockChangeHistoryPageData, error)
	// DeleteStockChangeHistoryBefore 删除 change_date 早于 cutoffDate 的历史数据。
	DeleteStockChangeHistoryBefore(ctx context.Context, cutoffDate string) error

	// Stock change statistics (pure DB aggregations)
	GetDailyChangeStats(ctx context.Context, startDate string) ([]stock.DailyChangeStats, error)
	GetChangeTypeDailyStats(ctx context.Context, startDate string) ([]stock.ChangeTypeDailyStats, error)
	GetChangeRank(ctx context.Context, startDate string, topN int) (*stock.ChangeRankResult, error)
	// GetDailyDimensionStats dimension 取值为 stock/industry/concept/type。
	GetDailyDimensionStats(ctx context.Context, dimension, name, startDate string) ([]stock.DailyDimensionStats, error)
	GetTypeStatsByDate(ctx context.Context, date string) ([]stock.TypeCountStats, error)
}
