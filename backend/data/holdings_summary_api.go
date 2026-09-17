package data

// 持仓 AI 每日总结：单股技术指标公共封装 + 总结表的按天 upsert 存取。
// 表模型 models.HoldingsDailySummary，建表注册在 backend/db/chat_memory.go AutoMigrate。

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"

	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// HoldingsPosition 持仓明细（Wails 前端传输结构，字段与 domain stock.HoldingsPosition 一致）
type HoldingsPosition struct {
	StockCode     string  `json:"stockCode"`
	StockName     string  `json:"stockName"`
	Volume        int64   `json:"volume"`
	CostPrice     float64 `json:"costPrice"`
	CostAmount    float64 `json:"costAmount"`
	CurrentPrice  float64 `json:"currentPrice"`
	MarketValue   float64 `json:"marketValue"`
	ProfitAmount  float64 `json:"profitAmount"`
	ProfitPercent float64 `json:"profitPercent"`
}

// StockIndicatorsResult 单股技术指标：数值 + 文字解读（Wails 只支持单返回值+error，故打包）
type StockIndicatorsResult struct {
	Code       string             `json:"code"`
	Indicators *IndicatorResult   `json:"indicators"`
	Summary    *IndicatorSummary  `json:"summary"`
}

// GetHoldingsStockIndicators 规范化交易日志股票代码后，取日线全套技术指标与文字解读。
// 数据不足时返回空指标与"数据不足"解读而不是 error，由调用方决定如何标注。
func GetHoldingsStockIndicators(ctx context.Context, code string) (*StockIndicatorsResult, error) {
	apiCode := normalizeTradingRecordAPI(code)
	ind, err := GetTechnicalIndicators(ctx, apiCode, "101", 60)
	if err != nil {
		return nil, err
	}
	return &StockIndicatorsResult{
		Code:       apiCode,
		Indicators: ind,
		Summary:    GetIndicatorSummary(ind),
	}, nil
}

// SaveHoldingsDailySummary 按 SummaryDate upsert：同一天重新生成覆盖原记录。
func SaveHoldingsDailySummary(s *models.HoldingsDailySummary) error {
	if s == nil || strings.TrimSpace(s.SummaryDate) == "" {
		return errors.New("持仓总结保存参数不完整")
	}
	var existing models.HoldingsDailySummary
	err := db.Dao.Where("summary_date = ?", s.SummaryDate).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return db.Dao.Create(s).Error
		}
		return err
	}
	s.ID = existing.ID
	return db.Dao.Model(&models.HoldingsDailySummary{}).Where("id = ?", existing.ID).Updates(map[string]any{
		"content":           s.Content,
		"model_name":        s.ModelName,
		"holdings_snapshot": s.HoldingsSnapshot,
		"stock_count":       s.StockCount,
		"total_profit":      s.TotalProfit,
		"profit_rate":       s.ProfitRate,
	}).Error
}

// HoldingsSummaryPageData 总结历史分页结果（Content 截断为预览，全文走 Detail）
type HoldingsSummaryPageData struct {
	List       []models.HoldingsDailySummary `json:"list"`
	Total      int64                         `json:"total"`
	Page       int                           `json:"page"`
	PageSize   int                           `json:"pageSize"`
	TotalPages int                           `json:"totalPages"`
}

// GetHoldingsSummaryList 总结历史列表（按日期倒序，Content 只保留前 120 字预览）
func GetHoldingsSummaryList(page, pageSize int) (*HoldingsSummaryPageData, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	var total int64
	if err := db.Dao.Model(&models.HoldingsDailySummary{}).Count(&total).Error; err != nil {
		logger.SugaredLogger.Errorf("获取持仓总结总数失败: %s", err.Error())
		return nil, err
	}
	var list []models.HoldingsDailySummary
	offset := (page - 1) * pageSize
	if err := db.Dao.Model(&models.HoldingsDailySummary{}).
		Order("summary_date DESC, id DESC").
		Offset(offset).Limit(pageSize).
		Find(&list).Error; err != nil {
		logger.SugaredLogger.Errorf("获取持仓总结列表失败: %s", err.Error())
		return nil, err
	}
	for i := range list {
		list[i].Content = truncateRunes(list[i].Content, 120)
	}
	totalPages := (total + int64(pageSize) - 1) / int64(pageSize)
	if totalPages < 1 {
		totalPages = 1
	}
	return &HoldingsSummaryPageData{
		List:       list,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: int(totalPages),
	}, nil
}

// GetHoldingsSummaryDetail 单条总结全文（含持仓快照 JSON）
func GetHoldingsSummaryDetail(id uint) (*models.HoldingsDailySummary, error) {
	var summary models.HoldingsDailySummary
	err := db.Dao.Where("id = ?", id).First(&summary).Error
	if err != nil {
		return nil, err
	}
	return &summary, nil
}

// GetLatestHoldingsSummary 取指定日期（yyyy-MM-dd）当天的总结，没有则返回 nil。
func GetHoldingsSummaryByDate(summaryDate string) (*models.HoldingsDailySummary, error) {
	var summary models.HoldingsDailySummary
	err := db.Dao.Where("summary_date = ?", summaryDate).First(&summary).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &summary, nil
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return string(runes[:n]) + "…"
}
