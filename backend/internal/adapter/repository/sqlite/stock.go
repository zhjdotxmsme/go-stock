// Package sqlite implements repository port interfaces backed by the app's
// SQLite database (accessed via the global db.Dao handle and, for complex
// legacy logic, by delegating to the existing backend/data APIs).
package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/internal/domain/stock"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// StockRepository implements repository.StockRepository.
type StockRepository struct{}

// NewStockRepository creates a new StockRepository.
func NewStockRepository() *StockRepository {
	return &StockRepository{}
}

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

// FollowedStockToDomain maps legacy data.FollowedStock to domain model.
func FollowedStockToDomain(f *data.FollowedStock) stock.FollowedStock {
	if f == nil {
		return stock.FollowedStock{}
	}
	return stock.FollowedStock{
		StockCode:          f.StockCode,
		Name:               f.Name,
		Volume:             f.Volume,
		CostPrice:          f.CostPrice,
		Price:              f.Price,
		PriceChange:        f.PriceChange,
		ChangePercent:      f.ChangePercent,
		AlarmChangePercent: f.AlarmChangePercent,
		AlarmPrice:         f.AlarmPrice,
		Time:               f.Time,
		Sort:               f.Sort,
		Cron:               f.Cron,
		IsDel:              f.IsDel,
		AiConfigId:         f.AiConfigId,
		EntryPrice:         f.EntryPrice,
		TakeProfitPrice:    f.TakeProfitPrice,
		StopLossPrice:      f.StopLossPrice,
	}
}

// GroupToDomain maps legacy data.Group to domain model.
func GroupToDomain(g *data.Group) stock.Group {
	if g == nil {
		return stock.Group{}
	}
	return stock.Group{Model: g.Model, Name: g.Name, Sort: g.Sort}
}

// resultErr converts a legacy string result into an error, treating any
// result containing "成功" as success.
func resultErr(result string) error {
	if strings.Contains(result, "成功") {
		return nil
	}
	return fmt.Errorf("%s", result)
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
// Followed stocks
// ---------------------------------------------------------------------------

func (r *StockRepository) AddFollow(ctx context.Context, stockCode, stockName string) error {
	return resultErr((data.StockDataApi{}).Follow(stockCode))
}

func (r *StockRepository) RemoveFollow(ctx context.Context, stockCode string) error {
	return resultErr((data.StockDataApi{}).UnFollow(stockCode))
}

func (r *StockRepository) GetFollowList(ctx context.Context, groupID int) ([]stock.FollowedStock, error) {
	list := (data.StockDataApi{}).GetFollowList(groupID)
	if list == nil {
		return nil, nil
	}
	result := make([]stock.FollowedStock, 0, len(*list))
	for i := range *list {
		result = append(result, FollowedStockToDomain(&(*list)[i]))
	}
	return result, nil
}

func (r *StockRepository) SetCostPriceAndVolume(ctx context.Context, stockCode string, price float64, volume int64) error {
	return resultErr((data.StockDataApi{}).SetCostPriceAndVolume(price, volume, stockCode))
}

func (r *StockRepository) SetTradingPrice(ctx context.Context, stockCode string, entryPrice, takeProfitPrice, stopLossPrice, costPrice float64) error {
	return resultErr((data.StockDataApi{}).SetTradingPrice(entryPrice, takeProfitPrice, stopLossPrice, costPrice, stockCode))
}

func (r *StockRepository) SetAlarmChangePercent(ctx context.Context, stockCode string, changePercent, alarmPrice float64) error {
	return resultErr((data.StockDataApi{}).SetAlarmChangePercent(changePercent, alarmPrice, stockCode))
}

func (r *StockRepository) SetStockSort(ctx context.Context, stockCode string, sort int64) error {
	(data.StockDataApi{}).SetStockSort(sort, stockCode)
	return nil
}

// ---------------------------------------------------------------------------
// Groups
// ---------------------------------------------------------------------------

func (r *StockRepository) AddGroup(ctx context.Context, name string) (*stock.Group, error) {
	api := data.NewStockGroupApi(db.Dao)
	maxSort := 0
	for _, g := range api.GetGroupList() {
		if g.Sort > maxSort {
			maxSort = g.Sort
		}
	}
	group := data.Group{Name: name, Sort: maxSort + 1}
	if !api.AddGroup(group) {
		return nil, fmt.Errorf("添加分组失败")
	}
	return &stock.Group{Name: name, Sort: group.Sort}, nil
}

func (r *StockRepository) RemoveGroup(ctx context.Context, groupID int) error {
	if !data.NewStockGroupApi(db.Dao).RemoveGroup(groupID) {
		return fmt.Errorf("删除分组失败")
	}
	return nil
}

func (r *StockRepository) GetGroupList(ctx context.Context) ([]stock.Group, error) {
	list := data.NewStockGroupApi(db.Dao).GetGroupList()
	result := make([]stock.Group, 0, len(list))
	for i := range list {
		result = append(result, GroupToDomain(&list[i]))
	}
	return result, nil
}

func (r *StockRepository) AddStockToGroup(ctx context.Context, groupID int, stockCode string) error {
	if !data.NewStockGroupApi(db.Dao).AddStockGroup(groupID, stockCode) {
		return fmt.Errorf("添加股票到分组失败")
	}
	return nil
}

func (r *StockRepository) RemoveStockFromGroup(ctx context.Context, groupID int, stockCode, stockName string) error {
	if !data.NewStockGroupApi(db.Dao).RemoveStockGroup(stockCode, stockName, groupID) {
		return fmt.Errorf("从分组移除股票失败")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Stock change history
//
// All methods are pure-DB and written directly with GORM on the legacy
// models.StockChangeHistory model; realtime change items are fetched by the
// caller (handler) and only persisted here.
// ---------------------------------------------------------------------------

// StockChangeHistoryToDomain maps the legacy DB model to the domain model.
func StockChangeHistoryToDomain(m *models.StockChangeHistory) stock.StockChangeHistory {
	if m == nil {
		return stock.StockChangeHistory{}
	}
	return stock.StockChangeHistory{
		ID:         m.ID,
		ChangeTime: m.ChangeTime,
		ChangeDate: m.ChangeDate,
		StockCode:  m.StockCode,
		StockName:  m.StockName,
		Market:     m.Market,
		ChangeType: m.ChangeType,
		TypeName:   m.TypeName,
		Volume:     m.Volume,
		Price:      m.Price,
		ChangeRate: m.ChangeRate,
		Amount:     m.Amount,
		Industry:   m.Industry,
		Concept:    m.Concept,
		CreatedAt:  m.CreatedAt,
	}
}

// StockChangeHistoryFromDomain maps the domain model to the legacy DB model.
func StockChangeHistoryFromDomain(h *stock.StockChangeHistory) models.StockChangeHistory {
	if h == nil {
		return models.StockChangeHistory{}
	}
	return models.StockChangeHistory{
		ID:         h.ID,
		ChangeTime: h.ChangeTime,
		ChangeDate: h.ChangeDate,
		StockCode:  h.StockCode,
		StockName:  h.StockName,
		Market:     h.Market,
		ChangeType: h.ChangeType,
		TypeName:   h.TypeName,
		Volume:     h.Volume,
		Price:      h.Price,
		ChangeRate: h.ChangeRate,
		Amount:     h.Amount,
		Industry:   h.Industry,
		Concept:    h.Concept,
		CreatedAt:  h.CreatedAt,
	}
}

// StockChangeHistoryPageDataFromDomain maps a domain page result back to the
// models-layer type (used by handlers that keep models-typed signatures).
func StockChangeHistoryPageDataFromDomain(p *stock.StockChangeHistoryPageData) *models.StockChangeHistoryPageData {
	if p == nil {
		return &models.StockChangeHistoryPageData{}
	}
	list := make([]models.StockChangeHistory, 0, len(p.List))
	for i := range p.List {
		list = append(list, StockChangeHistoryFromDomain(&p.List[i]))
	}
	return &models.StockChangeHistoryPageData{
		List:       list,
		Total:      p.Total,
		Page:       p.Page,
		PageSize:   p.PageSize,
		TotalPages: p.TotalPages,
	}
}

func (r *StockRepository) SaveStockChangesToHistory(ctx context.Context, changes []stock.StockChangeHistory) error {
	if len(changes) == 0 {
		return nil
	}
	histories := make([]models.StockChangeHistory, 0, len(changes))
	for i := range changes {
		histories = append(histories, StockChangeHistoryFromDomain(&changes[i]))
	}
	return db.Dao.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "change_date"}, {Name: "stock_code"}, {Name: "change_time"}},
		DoNothing: true,
	}).CreateInBatches(histories, 100).Error
}

func (r *StockRepository) SaveStockChangesToHistoryWithDedup(ctx context.Context, changes []stock.StockChangeHistory) (int, error) {
	if len(changes) == 0 {
		return 0, nil
	}
	histories := make([]models.StockChangeHistory, 0, len(changes))
	for i := range changes {
		histories = append(histories, StockChangeHistoryFromDomain(&changes[i]))
	}
	result := db.Dao.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "change_date"}, {Name: "stock_code"}, {Name: "change_time"}, {Name: "change_type"}, {Name: "price"}, {Name: "change_rate"}, {Name: "amount"}, {Name: "volume"}},
		DoNothing: true,
	}).CreateInBatches(histories, 100)
	if result.Error != nil {
		return 0, result.Error
	}
	return int(result.RowsAffected), nil
}

func (r *StockRepository) GetStockChangeHistory(ctx context.Context, query stock.StockChangeHistoryQuery) (stock.StockChangeHistoryPageData, error) {
	dbQuery := db.Dao.Model(&models.StockChangeHistory{})

	if query.StockCode != "" {
		dbQuery = dbQuery.Where("stock_code LIKE ?", "%"+query.StockCode+"%")
	}
	if query.StockName != "" {
		dbQuery = dbQuery.Where("stock_name LIKE ?", "%"+query.StockName+"%")
	}
	if query.ChangeType > 0 {
		dbQuery = dbQuery.Where("change_type = ?", query.ChangeType)
	}
	if len(query.ChangeTypes) > 0 {
		dbQuery = dbQuery.Where("change_type IN ?", query.ChangeTypes)
	}
	if query.TypeName != "" {
		dbQuery = dbQuery.Where("type_name = ?", query.TypeName)
	}
	if query.StartDate != "" {
		dbQuery = dbQuery.Where("change_date >= ?", query.StartDate)
	}
	if query.EndDate != "" {
		dbQuery = dbQuery.Where("change_date <= ?", query.EndDate)
	}
	if query.StartTime != "" {
		dbQuery = dbQuery.Where("change_time >= ?", query.StartTime)
	}
	if query.EndTime != "" {
		dbQuery = dbQuery.Where("change_time <= ?", query.EndTime)
	}
	if query.MinVolume > 0 {
		dbQuery = dbQuery.Where("volume >= ?", query.MinVolume)
	}
	if query.MinAmount > 0 {
		dbQuery = dbQuery.Where("amount >= ?", query.MinAmount)
	}
	if query.MinChangeRate != 0 {
		dbQuery = dbQuery.Where("change_rate >= ?", query.MinChangeRate)
	}
	if query.MaxChangeRate != 0 {
		dbQuery = dbQuery.Where("change_rate <= ?", query.MaxChangeRate)
	}
	if query.Industry != "" {
		dbQuery = dbQuery.Where("industry LIKE ?", "%"+query.Industry+"%")
	}
	if query.Concept != "" {
		dbQuery = dbQuery.Where("concept LIKE ?", "%"+query.Concept+"%")
	}

	var total int64
	if err := dbQuery.Count(&total).Error; err != nil {
		return stock.StockChangeHistoryPageData{}, err
	}

	var list []models.StockChangeHistory
	offset := (query.Page - 1) * query.PageSize
	if err := dbQuery.Order("change_date DESC, change_time DESC").Offset(offset).Limit(query.PageSize).Find(&list).Error; err != nil {
		return stock.StockChangeHistoryPageData{}, err
	}

	totalPages := int(total) / query.PageSize
	if int(total)%query.PageSize > 0 {
		totalPages++
	}

	items := make([]stock.StockChangeHistory, 0, len(list))
	for i := range list {
		items = append(items, StockChangeHistoryToDomain(&list[i]))
	}
	return stock.StockChangeHistoryPageData{
		List:       items,
		Total:      total,
		Page:       query.Page,
		PageSize:   query.PageSize,
		TotalPages: totalPages,
	}, nil
}

func (r *StockRepository) DeleteStockChangeHistoryBefore(ctx context.Context, cutoffDate string) error {
	return db.Dao.Where("change_date < ?", cutoffDate).Delete(&models.StockChangeHistory{}).Error
}

// ---------------------------------------------------------------------------
// Stock change statistics (pure DB aggregations, SQL ported verbatim)
// ---------------------------------------------------------------------------

const (
	upChangeTypes   = "4,8201,8202,8193,64,8207,8209,8211,8213,8215"
	downChangeTypes = "8,8203,8204,8194,128,8208,8210,8212,8214,8216"
)

func (r *StockRepository) GetDailyChangeStats(ctx context.Context, startDate string) ([]stock.DailyChangeStats, error) {
	type rawDailyStats struct {
		ChangeDate string
		TotalCount int64
		UpCount    int64
		DownCount  int64
	}

	var rawStats []rawDailyStats
	err := db.Dao.Model(&models.StockChangeHistory{}).
		Select("change_date, count(*) as total_count, sum(case when change_type in (4, 8201, 8202, 8193, 64, 8207, 8209, 8211, 8213, 8215) then 1 else 0 end) as up_count, sum(case when change_type in (8, 8203, 8204, 8194, 128, 8208, 8210, 8212, 8214, 8216) then 1 else 0 end) as down_count").
		Where("change_date >= ?", startDate).
		Group("change_date").
		Order("change_date ASC").
		Find(&rawStats).Error
	if err != nil {
		return nil, err
	}

	type limitStats struct {
		ChangeDate string
		LimitUp    int64
		LimitDown  int64
	}
	var limitData []limitStats
	err = db.Dao.Model(&models.StockChangeHistory{}).
		Select("change_date, sum(case when change_type = 4 then 1 else 0 end) as limit_up, sum(case when change_type = 8 then 1 else 0 end) as limit_down").
		Where("change_date >= ? AND change_type IN (4, 8)", startDate).
		Group("change_date").
		Order("change_date ASC").
		Find(&limitData).Error
	if err != nil {
		return nil, err
	}

	limitMap := make(map[string]limitStats)
	for _, l := range limitData {
		limitMap[l.ChangeDate] = l
	}

	var result []stock.DailyChangeStats
	for _, rs := range rawStats {
		l := limitMap[rs.ChangeDate]
		result = append(result, stock.DailyChangeStats{
			ChangeDate: rs.ChangeDate,
			TotalCount: rs.TotalCount,
			UpCount:    rs.UpCount,
			DownCount:  rs.DownCount,
			LimitUp:    l.LimitUp,
			LimitDown:  l.LimitDown,
		})
	}
	return result, nil
}

func (r *StockRepository) GetChangeTypeDailyStats(ctx context.Context, startDate string) ([]stock.ChangeTypeDailyStats, error) {
	var result []stock.ChangeTypeDailyStats
	err := db.Dao.Model(&models.StockChangeHistory{}).
		Select("change_date, type_name, count(*) as count").
		Where("change_date >= ?", startDate).
		Group("change_date, type_name").
		Order("change_date ASC, count DESC").
		Find(&result).Error
	return result, err
}

func (r *StockRepository) GetChangeRank(ctx context.Context, startDate string, topN int) (*stock.ChangeRankResult, error) {
	type rankRow struct {
		Name     string
		Code     string
		TotalCnt int
		UpCnt    int
		DownCnt  int
	}

	var stockRows []rankRow
	err := db.Dao.Model(&models.StockChangeHistory{}).
		Select("stock_name as name, stock_code as code, count(*) as total_cnt, sum(case when change_type IN ("+upChangeTypes+") then 1 else 0 end) as up_cnt, sum(case when change_type IN ("+downChangeTypes+") then 1 else 0 end) as down_cnt").
		Where("change_date >= ?", startDate).
		Group("stock_code, stock_name").
		Order("total_cnt DESC").
		Limit(topN).
		Find(&stockRows).Error
	if err != nil {
		return nil, err
	}

	var topStocks []stock.ChangeRankItem
	for _, row := range stockRows {
		topStocks = append(topStocks, stock.ChangeRankItem{Name: row.Name, Code: row.Code, Count: int64(row.TotalCnt), UpCount: int64(row.UpCnt), DownCount: int64(row.DownCnt)})
	}

	var industryRows []rankRow
	err = db.Dao.Model(&models.StockChangeHistory{}).
		Select("industry as name, '' as code, count(*) as total_cnt, sum(case when change_type IN ("+upChangeTypes+") then 1 else 0 end) as up_cnt, sum(case when change_type IN ("+downChangeTypes+") then 1 else 0 end) as down_cnt").
		Where("change_date >= ? AND industry != '' AND industry IS NOT NULL", startDate).
		Group("industry").
		Order("total_cnt DESC").
		Limit(topN).
		Find(&industryRows).Error
	if err != nil {
		return nil, err
	}

	var topIndustries []stock.ChangeRankItem
	for _, row := range industryRows {
		topIndustries = append(topIndustries, stock.ChangeRankItem{Name: row.Name, Count: int64(row.TotalCnt), UpCount: int64(row.UpCnt), DownCount: int64(row.DownCnt)})
	}

	type conceptRow struct {
		Concept string
		Cnt     int
		UpCnt   int
		DownCnt int
	}
	var conceptRows []conceptRow
	err = db.Dao.Model(&models.StockChangeHistory{}).
		Select("concept, count(*) as cnt, sum(case when change_type IN ("+upChangeTypes+") then 1 else 0 end) as up_cnt, sum(case when change_type IN ("+downChangeTypes+") then 1 else 0 end) as down_cnt").
		Where("change_date >= ? AND concept != '' AND concept IS NOT NULL", startDate).
		Group("concept").
		Find(&conceptRows).Error
	if err != nil {
		return nil, err
	}

	type conceptAgg struct {
		Count     int64
		UpCount   int64
		DownCount int64
	}
	conceptAggMap := make(map[string]conceptAgg)
	for _, row := range conceptRows {
		concepts := splitChangeConcepts(row.Concept)
		for _, c := range concepts {
			agg := conceptAggMap[c]
			agg.Count += int64(row.Cnt)
			agg.UpCount += int64(row.UpCnt)
			agg.DownCount += int64(row.DownCnt)
			conceptAggMap[c] = agg
		}
	}

	var topConcepts []stock.ChangeRankItem
	for name, agg := range conceptAggMap {
		topConcepts = append(topConcepts, stock.ChangeRankItem{Name: name, Count: agg.Count, UpCount: agg.UpCount, DownCount: agg.DownCount})
	}
	sort.Slice(topConcepts, func(i, j int) bool {
		return topConcepts[i].Count > topConcepts[j].Count
	})
	if len(topConcepts) > topN {
		topConcepts = topConcepts[:topN]
	}

	return &stock.ChangeRankResult{
		TopStocks:     topStocks,
		TopIndustries: topIndustries,
		TopConcepts:   topConcepts,
	}, nil
}

func (r *StockRepository) GetDailyDimensionStats(ctx context.Context, dimension, name, startDate string) ([]stock.DailyDimensionStats, error) {
	var result []stock.DailyDimensionStats
	query := db.Dao.Model(&models.StockChangeHistory{}).
		Select("change_date, sum(case when change_type IN ("+upChangeTypes+") then 1 else 0 end) as up_count, sum(case when change_type IN ("+downChangeTypes+") then 1 else 0 end) as down_count, count(*) as total_count").
		Where("change_date >= ?", startDate).
		Group("change_date").
		Order("change_date ASC")

	switch dimension {
	case "stock":
		query = query.Where("stock_code = ? OR stock_name = ?", name, name)
	case "industry":
		query = query.Where("industry = ?", name)
	case "concept":
		query = query.Where("concept LIKE ?", "%"+name+"%")
	case "type":
		query = query.Where("type_name = ?", name)
	default:
		return nil, fmt.Errorf("unsupported dimension: %s", dimension)
	}

	err := query.Find(&result).Error
	return result, err
}

func (r *StockRepository) GetTypeStatsByDate(ctx context.Context, date string) ([]stock.TypeCountStats, error) {
	var result []stock.TypeCountStats
	err := db.Dao.Model(&models.StockChangeHistory{}).
		Select("type_name, sum(case when change_type IN ("+upChangeTypes+") then 1 else 0 end) as up_count, sum(case when change_type IN ("+downChangeTypes+") then 1 else 0 end) as down_count, count(*) as total_count").
		Where("change_date = ?", date).
		Group("type_name").
		Order("total_count DESC").
		Find(&result).Error
	return result, err
}

// splitChangeConcepts 概念字段拆分(与 data 层 splitConcepts 逻辑逐字一致)。
func splitChangeConcepts(conceptStr string) []string {
	conceptStr = strings.TrimSpace(conceptStr)
	if conceptStr == "" {
		return nil
	}
	if strings.HasPrefix(conceptStr, "[") {
		var concepts []string
		if err := json.Unmarshal([]byte(conceptStr), &concepts); err == nil {
			return concepts
		}
	}
	parts := strings.Split(conceptStr, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
