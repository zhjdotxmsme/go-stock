package handler

import (
	"encoding/json"
	"strings"

	"go-stock/backend/data"
	"go-stock/backend/internal/port/datasource"
	"go-stock/backend/models"
)

// CommodityHandler handles commodity-related Wails bindings.
type CommodityHandler struct{}

// NewCommodityHandler creates a new CommodityHandler.
func NewCommodityHandler() *CommodityHandler {
	return &CommodityHandler{}
}

func (h *CommodityHandler) GetCommodityKLine(code string, period string, count int) ([]datasource.KLineBar, error) {
	api := data.NewCommodityApi()
	return api.GetKLine(code, period, count)
}

func (h *CommodityHandler) GetCommodityKLineIntl(code string, period string, count int) ([]datasource.KLineBar, error) {
	api := data.NewCommodityApi()
	return api.GetKLineIntl(code, period, count)
}

func (h *CommodityHandler) GetCommodityQuote(code string) (*datasource.QuoteData, error) {
	api := data.NewCommodityApi()
	return api.GetQuote(code)
}

func (h *CommodityHandler) GetCommodityQuoteIntl(code string) (*datasource.QuoteData, error) {
	api := data.NewCommodityApi()
	return api.GetQuoteIntl(code)
}

func (h *CommodityHandler) GetCommodityRegistry() []models.CommodityAsset {
	return data.CommodityRegistry
}

func (h *CommodityHandler) GetCommodityTechnicals(code string, period string) (string, error) {
	output, err := data.GetCommodityTechnicalsOutput(code, period)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(output)
	return string(b), nil
}

func (h *CommodityHandler) GetCommodityFundamentals(code string) (string, error) {
	output, err := data.GetCommodityFundamentalsOutput(code)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(output)
	return string(b), nil
}

func (h *CommodityHandler) GetCommodityCorrelation(primaryCode string, secondaryCodes string) (string, error) {
	list := []string{}
	for _, s := range strings.Split(secondaryCodes, ",") {
		list = append(list, strings.TrimSpace(s))
	}
	output, err := data.GetCorrelationOutput(primaryCode, list)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(output)
	return string(b), nil
}

func (h *CommodityHandler) GetMacroIndicatorsEnhanced() (*data.MacroSnapshotEnhanced, error) {
	api := data.NewCommodityApi()
	return api.GetMacroIndicatorsEnhanced()
}

// GetCommodityFuturesPanel 期货盘面数据（持仓/期限结构/库存/COT/金银比/ATR/季节性），
// 后端 60s 缓存，缺失数据点在 failed 列表中。
func (h *CommodityHandler) GetCommodityFuturesPanel() (*data.CommodityFuturesPanel, error) {
	return data.GetCommodityFuturesPanel()
}

// GetCommoditySignalBoard 全品种策略信号排行（趋势/动量/突破/carry/持仓象限 + 综合分）。
func (h *CommodityHandler) GetCommoditySignalBoard() (*data.CommoditySignalBoard, error) {
	return data.GetCommoditySignalBoard()
}

func (h *CommodityHandler) GetCommodityReport(codes string, reportType string) (string, error) {
	output, err := data.GetCommodityReportOutput(codes, reportType)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(output)
	return string(b), nil
}

func (h *CommodityHandler) GetTradableCommodities() []models.CommodityAsset {
	return data.TradableCommodities()
}
