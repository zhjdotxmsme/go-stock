package repository

import (
	"context"

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
	UpdateTradingRecord(ctx context.Context, record *stock.TradingRecord) error
	DeleteTradingRecord(ctx context.Context, id uint) error

	// Stock change history
	SaveStockChangesToHistory(ctx context.Context, changes []stock.StockChangeHistory) error
	GetStockChangeHistory(ctx context.Context, query stock.StockChangeHistoryQuery) (stock.StockChangeHistoryPageData, error)
	DeleteStockChangeHistory(ctx context.Context, id uint) error
}
