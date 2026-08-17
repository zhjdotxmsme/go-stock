// Package data ai_recommend_stocks_api.go
package data

import (
	"fmt"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"math"
	"time"

	"github.com/duke-git/lancet/v2/datetime"
	"github.com/duke-git/lancet/v2/slice"
	"github.com/duke-git/lancet/v2/strutil"
)

type AiRecommendStocksService struct{}

func NewAiRecommendStocksService() *AiRecommendStocksService {
	return &AiRecommendStocksService{}
}

// CreateAiRecommendStocks 创建AI推荐股票记录
func (s *AiRecommendStocksService) CreateAiRecommendStocks(recommend *models.AiRecommendStocks) error {
	result := db.Dao.Create(recommend)
	return result.Error
}

func (s *AiRecommendStocksService) BatchCreateAiRecommendStocks(recommends []*models.AiRecommendStocks) error {
	result := db.Dao.Create(recommends)
	return result.Error
}

// GetAiRecommendStocksList 分页查询AI推荐股票记录
func (s *AiRecommendStocksService) GetAiRecommendStocksList(query *models.AiRecommendStocksQuery) (*models.AiRecommendStocksPageData, error) {
	var list []models.AiRecommendStocks
	var total int64

	q := db.Dao.Model(&models.AiRecommendStocks{})

	// 构建关键词搜索条件（股票代码、股票名称、板块名称使用 OR 关系）
	keyword := query.StockCode
	if keyword == "" {
		keyword = query.StockName
	}
	if keyword == "" {
		keyword = query.BkName
	}
	if keyword == "" {
		keyword = query.ModelName
	}

	if keyword != "" {
		q = q.Where("(stock_code LIKE ? OR stock_name LIKE ? OR bk_name LIKE ? OR model_name LIKE ?)",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}

	// 日期范围查询
	if query.StartDate != "" && query.EndDate != "" {
		query.StartDate = strutil.ReplaceWithMap(query.StartDate, map[string]string{
			"T": " ",
			"Z": "",
		})
		query.EndDate = strutil.ReplaceWithMap(query.EndDate, map[string]string{
			"T": " ",
			"Z": "",
		})
		startDate, err := time.Parse("2006-01-02 15:04:05", query.StartDate)
		if err != nil {
			startDate, _ = time.Parse("2006-01-02", query.StartDate)
		}

		endDate, err := time.Parse("2006-01-02 15:04:05", query.EndDate)
		if err != nil {
			endDate, _ = time.Parse("2006-01-02", query.EndDate)
		}

		q = q.Where("data_time BETWEEN ? AND ?", datetime.BeginOfDay(startDate), datetime.EndOfDay(endDate))
	} else if query.StartDate == "" && query.EndDate == "" && keyword == "" {
		// 只有在没有关键词时才默认查询今天的数据
		q = q.Where("data_time BETWEEN ? AND ?", datetime.BeginOfDay(time.Now()), datetime.EndOfDay(time.Now()))
	} else if query.StartDate != "" && query.EndDate == "" {
		query.StartDate = strutil.ReplaceWithMap(query.StartDate, map[string]string{
			"T": " ",
			"Z": "",
		})
		startDate, _ := time.Parse("2006-01-02", query.StartDate)
		q = q.Where("data_time BETWEEN ? AND ?", datetime.BeginOfDay(startDate), datetime.EndOfDay(startDate))
	}

	// 预警状态筛选
	if query.EnableAlert != nil {
		q = q.Where("enable_alert = ?", *query.EnableAlert)
	}

	// 计算总数
	err := q.Count(&total).Error
	if err != nil {
		return nil, err
	}

	// 设置默认分页参数
	page := query.Page
	pageSize := query.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	// 执行分页查询
	offset := (page - 1) * pageSize
	err = q.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&list).Error
	if err != nil {
		return nil, err
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	stockCodes := slice.Map(list, func(index int, item models.AiRecommendStocks) string {
		return ConvertTushareCodeToStockCode(item.StockCode)
	})
	stockData, _ := NewStockDataApi().GetStockCodeRealTimeData(stockCodes...)
	for _, info := range *stockData {
		for idx, item := range list {
			if ConvertTushareCodeToStockCode(item.StockCode) == ConvertTushareCodeToStockCode(info.Code) {
				list[idx].StockCurrentPrice = info.Price
				list[idx].StockPrePrice = info.PreClose
				list[idx].StockCurrentPriceTime = info.Date + " " + info.Time
			}
		}
	}

	return &models.AiRecommendStocksPageData{
		List:       list,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// GetAiRecommendStocksByID 根据ID获取AI推荐股票记录
func (s *AiRecommendStocksService) GetAiRecommendStocksByID(id uint) (*models.AiRecommendStocks, error) {
	var recommend models.AiRecommendStocks
	err := db.Dao.First(&recommend, id).Error
	if err != nil {
		return nil, err
	}
	return &recommend, nil
}

// UpdateAiRecommendStocks 更新AI推荐股票记录
func (s *AiRecommendStocksService) UpdateAiRecommendStocks(id uint, recommend *models.AiRecommendStocks) error {
	result := db.Dao.Model(&models.AiRecommendStocks{}).Where("id = ?", id).Updates(recommend)
	return result.Error
}

// DeleteAiRecommendStocks 根据ID删除AI推荐股票记录
func (s *AiRecommendStocksService) DeleteAiRecommendStocks(id uint) error {
	// 使用软删除
	result := db.Dao.Where("id = ?", id).Delete(&models.AiRecommendStocks{})
	return result.Error
}

// UpdateAiRecommendStocksAlert 更新AI推荐股票的预警状态
func (s *AiRecommendStocksService) UpdateAiRecommendStocksAlert(id uint, enableAlert bool) error {
	result := db.Dao.Model(&models.AiRecommendStocks{}).Where("id = ?", id).Update("enable_alert", enableAlert)
	return result.Error
}

// BatchDeleteAiRecommendStocks 批量删除AI推荐股票记录
func (s *AiRecommendStocksService) BatchDeleteAiRecommendStocks(ids []uint) error {
	// 使用软删除
	result := db.Dao.Where("id IN ?", ids).Delete(&models.AiRecommendStocks{})
	return result.Error
}

// ── Stats ──

type ModelStat struct {
	ModelName string  `json:"modelName"`
	WinRate   float64 `json:"winRate"`
	AvgReturn float64 `json:"avgReturn"`
	Count     int     `json:"count"`
}

type SectorStat struct {
	BkName string `json:"bkName"`
	Count  int    `json:"count"`
}

type DailyCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type AiRecommendStats struct {
	ByModel    []ModelStat  `json:"byModel"`
	BySector   []SectorStat `json:"bySector"`
	DailyCount []DailyCount `json:"dailyCount"`
}

func parsePrice(s string) float64 {
	if s == "" {
		return 0
	}
	var v float64
	if _, err := fmt.Sscanf(s, "%f", &v); err == nil {
		return v
	}
	return 0
}

func (s *AiRecommendStocksService) GetAiRecommendStats() (*AiRecommendStats, error) {
	var all []models.AiRecommendStocks
	if err := db.Dao.Model(&models.AiRecommendStocks{}).Find(&all).Error; err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return &AiRecommendStats{}, nil
	}

	// ByModel
	modelMap := make(map[string]struct{ wins int; total int; retSum float64 })
	for _, r := range all {
		key := r.ModelName
		if key == "" {
			key = "unknown"
		}
		entry := modelMap[key]
		entry.total++
		current := parsePrice(r.StockCurrentPrice)
		orig := parsePrice(r.StockPrice)
		if orig > 0 && current > 0 {
			if current > orig {
				entry.wins++
			}
			entry.retSum += (current - orig) / orig
		}
		modelMap[key] = entry
	}
	var byModel []ModelStat
	for name, m := range modelMap {
		wr := 0.0
		if m.total > 0 {
			wr = float64(m.wins) / float64(m.total) * 100
		}
		avgRet := 0.0
		if m.total > 0 {
			avgRet = m.retSum / float64(m.total) * 100
		}
		byModel = append(byModel, ModelStat{
			ModelName: name,
			WinRate:   math.Round(wr*10) / 10,
			AvgReturn: math.Round(avgRet*10) / 10,
			Count:     m.total,
		})
	}

	// BySector
	sectorMap := make(map[string]int)
	for _, r := range all {
		key := r.BkName
		if key == "" {
			key = "未知"
		}
		sectorMap[key]++
	}
	var bySector []SectorStat
	for name, cnt := range sectorMap {
		bySector = append(bySector, SectorStat{BkName: name, Count: cnt})
	}

	// DailyCount
	dayMap := make(map[string]int)
	for _, r := range all {
		if r.DataTime != nil {
			day := r.DataTime.Format("2006-01-02")
			dayMap[day]++
		}
	}
	var dailyCount []DailyCount
	for d, cnt := range dayMap {
		dailyCount = append(dailyCount, DailyCount{Date: d, Count: cnt})
	}

	return &AiRecommendStats{
		ByModel:    byModel,
		BySector:   bySector,
		DailyCount: dailyCount,
	}, nil
}
