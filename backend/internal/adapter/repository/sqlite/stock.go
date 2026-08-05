// Package sqlite implements repository port interfaces backed by the app's
// SQLite database (accessed via the global db.Dao handle and, for complex
// legacy logic, by delegating to the existing backend/data APIs).
package sqlite

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/internal/domain/stock"
	"go-stock/backend/logger"
)

// StockRepository implements repository.StockRepository.
//
// Only the TradingRecord group is implemented in this slice; the remaining
// groups are placeholders that fail loudly until their own vertical slices
// are migrated (see TODOs below).
type StockRepository struct{}

// NewStockRepository creates a new StockRepository.
func NewStockRepository() *StockRepository {
	return &StockRepository{}
}

var errNotImplemented = fmt.Errorf("not implemented")

// ---------------------------------------------------------------------------
// data <-> domain mapping (explicit, no reflection)
// ---------------------------------------------------------------------------

// TradingRecordToDomain maps a data-layer GORM model to the domain model.
func TradingRecordToDomain(r *data.TradingRecord) *stock.TradingRecord {
	if r == nil {
		return nil
	}
	return &stock.TradingRecord{
		ID:                 r.ID,
		StockCode:          r.StockCode,
		StockName:          r.StockName,
		Direction:          r.Direction,
		Price:              r.Price,
		Volume:             r.Volume,
		Amount:             r.Amount,
		TradingTime:        r.TradingTime,
		Reason:             r.Reason,
		StopLossPrice:      r.StopLossPrice,
		TakeProfitPrice:    r.TakeProfitPrice,
		Fee:                r.Fee,
		MarketValue:        r.MarketValue,
		Mindset:            r.Mindset,
		RecordedClosePrice: r.RecordedClosePrice,
		CreatedAt:          r.CreatedAt,
		UpdatedAt:          r.UpdatedAt,
	}
}

// TradingRecordFromDomain maps a domain model to the data-layer GORM model.
func TradingRecordFromDomain(r *stock.TradingRecord) *data.TradingRecord {
	if r == nil {
		return nil
	}
	return &data.TradingRecord{
		ID:                 r.ID,
		StockCode:          r.StockCode,
		StockName:          r.StockName,
		Direction:          r.Direction,
		Price:              r.Price,
		Volume:             r.Volume,
		Amount:             r.Amount,
		TradingTime:        r.TradingTime,
		Reason:             r.Reason,
		StopLossPrice:      r.StopLossPrice,
		TakeProfitPrice:    r.TakeProfitPrice,
		Fee:                r.Fee,
		MarketValue:        r.MarketValue,
		Mindset:            r.Mindset,
		RecordedClosePrice: r.RecordedClosePrice,
		CreatedAt:          r.CreatedAt,
		UpdatedAt:          r.UpdatedAt,
	}
}

func tradingRecordPageDataToDomain(p *data.TradingRecordPageData) stock.TradingRecordPageData {
	if p == nil {
		return stock.TradingRecordPageData{}
	}
	items := make([]stock.TradingRecordItem, 0, len(p.List))
	for _, item := range p.List {
		record := item.TradingRecord
		items = append(items, stock.TradingRecordItem{
			TradingRecord: *TradingRecordToDomain(&record),
			ClosePrice:    item.ClosePrice,
			ProfitAmount:  item.ProfitAmount,
			ProfitPercent: item.ProfitPercent,
		})
	}
	return stock.TradingRecordPageData{
		List:       items,
		Total:      p.Total,
		Page:       p.Page,
		PageSize:   p.PageSize,
		TotalPages: p.TotalPages,
	}
}

// TradingRecordListQueryToDomain maps a data-layer list query to the domain query.
func TradingRecordListQueryToDomain(q data.TradingRecordListQuery) stock.TradingRecordListQuery {
	return stock.TradingRecordListQuery{
		Page:      q.Page,
		PageSize:  q.PageSize,
		Keyword:   q.Keyword,
		Direction: q.Direction,
		StartDate: q.StartDate,
		EndDate:   q.EndDate,
	}
}

// TradingRecordPageDataFromDomain maps a domain page result back to the
// data-layer type (used by handlers that keep data-typed signatures).
func TradingRecordPageDataFromDomain(p *stock.TradingRecordPageData) *data.TradingRecordPageData {
	if p == nil {
		return &data.TradingRecordPageData{}
	}
	items := make([]data.TradingRecordItem, 0, len(p.List))
	for _, item := range p.List {
		record := item.TradingRecord
		items = append(items, data.TradingRecordItem{
			TradingRecord: *TradingRecordFromDomain(&record),
			ClosePrice:    item.ClosePrice,
			ProfitAmount:  item.ProfitAmount,
			ProfitPercent: item.ProfitPercent,
		})
	}
	return &data.TradingRecordPageData{
		List:       items,
		Total:      p.Total,
		Page:       p.Page,
		PageSize:   p.PageSize,
		TotalPages: p.TotalPages,
	}
}

// ---------------------------------------------------------------------------
// Trading records
//
// Add/GetList/Update/Delete delegate to the existing data-layer API: those
// paths embed intricate legacy behavior (frequent-trade re-check, close-price
// snapshot fills and backfill writes, FIFO profit assembly) that is coupled to
// realtime market data and not yet expressible through the port. The new
// primitives (GetById/ListAll/CountBuy) are simple queries written directly
// with GORM.
// ---------------------------------------------------------------------------

func (r *StockRepository) AddTradingRecord(ctx context.Context, record *stock.TradingRecord) error {
	d := TradingRecordFromDomain(record)
	id, err := data.NewStockDataApi().AddTradingRecord(*d)
	if err != nil {
		return err
	}
	record.ID = id
	return nil
}

func (r *StockRepository) GetTradingRecordList(ctx context.Context, query stock.TradingRecordListQuery) (stock.TradingRecordPageData, error) {
	page, err := data.NewStockDataApi().GetTradingRecordList(data.TradingRecordListQuery{
		Page:      query.Page,
		PageSize:  query.PageSize,
		Keyword:   query.Keyword,
		Direction: query.Direction,
		StartDate: query.StartDate,
		EndDate:   query.EndDate,
	})
	if err != nil {
		return stock.TradingRecordPageData{}, err
	}
	return tradingRecordPageDataToDomain(page), nil
}

func (r *StockRepository) GetTradingRecordById(ctx context.Context, id uint) (*stock.TradingRecord, error) {
	var record data.TradingRecord
	err := db.Dao.Model(&data.TradingRecord{}).Where("id = ?", id).First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		logger.SugaredLogger.Errorf("获取交易日志失败: %s", err.Error())
		return nil, err
	}
	return TradingRecordToDomain(&record), nil
}

func (r *StockRepository) UpdateTradingRecord(ctx context.Context, record *stock.TradingRecord) error {
	return data.NewStockDataApi().UpdateTradingRecord(*TradingRecordFromDomain(record))
}

func (r *StockRepository) DeleteTradingRecord(ctx context.Context, id uint) error {
	return data.NewStockDataApi().DeleteTradingRecord(id)
}

func (r *StockRepository) ListAllTradingRecords(ctx context.Context) ([]stock.TradingRecord, error) {
	var records []data.TradingRecord
	err := db.Dao.Model(&data.TradingRecord{}).Order("trading_time ASC, id ASC").Find(&records).Error
	if err != nil {
		logger.SugaredLogger.Errorf("获取全部交易日志失败: %s", err.Error())
		return nil, err
	}
	result := make([]stock.TradingRecord, 0, len(records))
	for i := range records {
		result = append(result, *TradingRecordToDomain(&records[i]))
	}
	return result, nil
}

func (r *StockRepository) CountBuyTradingRecords(ctx context.Context, stockCode string, since time.Time) (int64, error) {
	q := db.Dao.Model(&data.TradingRecord{}).
		Where("direction = ? AND trading_time > ?", "买入", since)
	if stockCode != "" {
		q = q.Where("stock_code = ?", stockCode)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		logger.SugaredLogger.Errorf("统计买入交易次数失败: %s", err.Error())
		return 0, err
	}
	return count, nil
}

// ---------------------------------------------------------------------------
// Followed stocks (TODO: migrate in the followed-stock vertical slice)
// ---------------------------------------------------------------------------

func (r *StockRepository) AddFollow(ctx context.Context, stockCode, stockName string) error {
	// TODO: implement when migrating the followed-stock slice
	return errNotImplemented
}

func (r *StockRepository) RemoveFollow(ctx context.Context, stockCode string) error {
	// TODO: implement when migrating the followed-stock slice
	return errNotImplemented
}

func (r *StockRepository) GetFollowList(ctx context.Context, groupID int) ([]stock.FollowedStock, error) {
	// TODO: implement when migrating the followed-stock slice
	return nil, errNotImplemented
}

func (r *StockRepository) SetCostPriceAndVolume(ctx context.Context, stockCode string, price float64, volume int64) error {
	// TODO: implement when migrating the followed-stock slice
	return errNotImplemented
}

func (r *StockRepository) SetTradingPrice(ctx context.Context, stockCode string, entryPrice, takeProfitPrice, stopLossPrice, costPrice float64) error {
	// TODO: implement when migrating the followed-stock slice
	return errNotImplemented
}

func (r *StockRepository) SetAlarmChangePercent(ctx context.Context, stockCode string, changePercent, alarmPrice float64) error {
	// TODO: implement when migrating the followed-stock slice
	return errNotImplemented
}

func (r *StockRepository) SetStockSort(ctx context.Context, stockCode string, sort int64) error {
	// TODO: implement when migrating the followed-stock slice
	return errNotImplemented
}

// ---------------------------------------------------------------------------
// Groups (TODO: migrate in the group vertical slice)
// ---------------------------------------------------------------------------

func (r *StockRepository) AddGroup(ctx context.Context, name string) (*stock.Group, error) {
	// TODO: implement when migrating the group slice
	return nil, errNotImplemented
}

func (r *StockRepository) RemoveGroup(ctx context.Context, groupID int) error {
	// TODO: implement when migrating the group slice
	return errNotImplemented
}

func (r *StockRepository) GetGroupList(ctx context.Context) ([]stock.Group, error) {
	// TODO: implement when migrating the group slice
	return nil, errNotImplemented
}

func (r *StockRepository) AddStockToGroup(ctx context.Context, groupID int, stockCode string) error {
	// TODO: implement when migrating the group slice
	return errNotImplemented
}

func (r *StockRepository) RemoveStockFromGroup(ctx context.Context, groupID int, stockCode, stockName string) error {
	// TODO: implement when migrating the group slice
	return errNotImplemented
}

// ---------------------------------------------------------------------------
// Stock change history (TODO: migrate in the stock-change vertical slice)
// ---------------------------------------------------------------------------

func (r *StockRepository) SaveStockChangesToHistory(ctx context.Context, changes []stock.StockChangeHistory) error {
	// TODO: implement when migrating the stock-change slice
	return errNotImplemented
}

func (r *StockRepository) GetStockChangeHistory(ctx context.Context, query stock.StockChangeHistoryQuery) (stock.StockChangeHistoryPageData, error) {
	// TODO: implement when migrating the stock-change slice
	return stock.StockChangeHistoryPageData{}, errNotImplemented
}

func (r *StockRepository) DeleteStockChangeHistory(ctx context.Context, id uint) error {
	// TODO: implement when migrating the stock-change slice
	return errNotImplemented
}
