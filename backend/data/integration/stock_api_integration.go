// backend/data/integration/stock_api_integration.go
package integration

import (
	"context"
	"fmt"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/data/layers"
	"go-stock/backend/data/types"
	"go-stock/backend/logger"
)

// StockApiIntegration wraps the existing StockDataApi with the new layered architecture
type StockApiIntegration struct {
	originalApi   *data.StockDataApi
	marketLayer   *layers.MarketDataLayer
	sentimentLayer *layers.SentimentLayer
	capitalFlowLayer *layers.CapitalFlowLayer
	announcementLayer *layers.AnnouncementLayer
}

// NewStockApiIntegration creates a new integration with layered architecture
func NewStockApiIntegration(originalApi *data.StockDataApi) *StockApiIntegration {
	// Create market data layer config with existing data sources
	marketConfig := &types.DataSourceConfig{
		Primary: types.Endpoint{
			Name:   "sina",
			URL:    "http://hq.sinajs.cn",
			Method: types.MethodGet,
		},
		Fallbacks: []types.Endpoint{
			{
				Name:   "tencent",
				URL:    "http://qt.gtimg.cn",
				Method: types.MethodGet,
			},
		},
		Strategy: types.FailoverStrategy,
	}

	// Create sentiment layer config
	sentimentConfig := &types.DataSourceConfig{
		Primary: types.Endpoint{
			Name:   "eastmoney_sentiment",
			URL:    "http://sentiment.eastmoney.com",
			Method: types.MethodGet,
		},
		Strategy: types.FailoverStrategy,
	}

	// Create capital flow layer config
	capitalFlowConfig := &types.DataSourceConfig{
		Primary: types.Endpoint{
			Name:   "eastmoney_flow",
			URL:    "http://flow.eastmoney.com",
			Method: types.MethodGet,
		},
		Strategy: types.FailoverStrategy,
	}

	// Create announcement layer config
	announcementConfig := &types.DataSourceConfig{
		Primary: types.Endpoint{
			Name:   "eastmoney_ann",
			URL:    "http://ann.eastmoney.com",
			Method: types.MethodGet,
		},
		Strategy: types.FailoverStrategy,
	}

	return &StockApiIntegration{
		originalApi:   originalApi,
		marketLayer:   layers.NewMarketDataLayer(marketConfig),
		sentimentLayer: layers.NewSentimentLayer(sentimentConfig),
		capitalFlowLayer: layers.NewCapitalFlowLayer(capitalFlowConfig),
		announcementLayer: layers.NewAnnouncementLayer(announcementConfig),
	}
}

// GetStockCodeRealTimeDataWithFallback uses the new layered architecture for real-time stock data
func (s *StockApiIntegration) GetStockCodeRealTimeDataWithFallback(ctx context.Context, stockCodes ...string) (map[string]*data.StockInfoExtended, error) {
	result := make(map[string]*data.StockInfoExtended)

	for _, stockCode := range stockCodes {
		// Use the new MarketDataLayer
		response, err := s.marketLayer.FetchData(ctx, map[string]any{
			"stock_code": stockCode,
		})

		if err != nil {
			logger.SugaredLogger.Warnw("Failed to fetch stock data with layers, falling back to original API",
				"stock_code", stockCode, "error", err)

			// Fallback to original API
			originalData, err := s.originalApi.GetStockCodeRealTimeData(stockCode)
			if err != nil {
				return nil, fmt.Errorf("both layered and original API failed for %s: %w", stockCode, err)
			}

			// Convert original data format to new format
			if originalData != nil && len(*originalData) > 0 {
				stockInfoExtended := s.convertOriginalToExtended(&(*originalData)[0], stockCode)
				result[stockCode] = stockInfoExtended
			}
			continue
		}

		// Convert layered response to extended format
		stockInfoExtended, err := s.convertLayerResponseToExtended(response, stockCode)
		if err != nil {
			logger.SugaredLogger.Warnw("Failed to convert layer response, falling back to original API",
				"stock_code", stockCode, "error", err)

			originalData, err := s.originalApi.GetStockCodeRealTimeData(stockCode)
			if err != nil {
				return nil, fmt.Errorf("original API fallback also failed for %s: %w", stockCode, err)
			}

			if originalData != nil && len(*originalData) > 0 {
				stockInfoExtended := s.convertOriginalToExtended(&(*originalData)[0], stockCode)
				result[stockCode] = stockInfoExtended
			}
			continue
		}

		result[stockCode] = stockInfoExtended
	}

	return result, nil
}

// GetStockSentiment uses the new sentiment layer
func (s *StockApiIntegration) GetStockSentiment(ctx context.Context, stockCode, startDate, endDate string) ([]map[string]any, error) {
	response, err := s.sentimentLayer.FetchData(ctx, map[string]any{
		"stock_code": stockCode,
		"start_date": startDate,
		"end_date":   endDate,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to fetch sentiment data: %w", err)
	}

	sentimentData, ok := response.Data.([]map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid sentiment data format")
	}

	return sentimentData, nil
}

// GetStockCapitalFlow uses the new capital flow layer
func (s *StockApiIntegration) GetStockCapitalFlow(ctx context.Context, stockCode, date string) (map[string]any, error) {
	response, err := s.capitalFlowLayer.FetchData(ctx, map[string]any{
		"stock_code": stockCode,
		"date":       date,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to fetch capital flow data: %w", err)
	}

	capitalFlowData, ok := response.Data.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid capital flow data format")
	}

	return capitalFlowData, nil
}

// GetStockAnnouncements uses the new announcement layer
func (s *StockApiIntegration) GetStockAnnouncements(ctx context.Context, stockCode string, days int) ([]map[string]any, error) {
	response, err := s.announcementLayer.FetchData(ctx, map[string]any{
		"stock_code": stockCode,
		"days":       days,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to fetch announcement data: %w", err)
	}

	announcementData, ok := response.Data.([]map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid announcement data format")
	}

	return announcementData, nil
}

// convertLayerResponseToExtended converts the layered response format to the extended StockInfo format
func (s *StockApiIntegration) convertLayerResponseToExtended(response *types.StandardizedResponse, stockCode string) (*data.StockInfoExtended, error) {
	layerData, ok := response.Data.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid layer response data format")
	}

	stockInfoExtended := &data.StockInfoExtended{
		StockInfo: data.StockInfo{
			Date: time.Now().Format("2006-01-02"),
			Time: time.Now().Format("15:04:05"),
		},
		TSCode: stockCode,
	}

	// Map fields from layer data to extended format
	if tsCode, ok := layerData["ts_code"].(string); ok {
		stockInfoExtended.TSCode = tsCode
		stockInfoExtended.Code = tsCode
	}

	if name, ok := layerData["name"].(string); ok {
		stockInfoExtended.Name = name
	}

	if current, ok := layerData["current"].(float64); ok {
		stockInfoExtended.Current = current
		stockInfoExtended.Price = fmt.Sprintf("%.2f", current)
	}

	if change, ok := layerData["change"].(float64); ok {
		stockInfoExtended.Change = change
		stockInfoExtended.ChangePrice = change
	}

	if pctChg, ok := layerData["pct_chg"].(float64); ok {
		stockInfoExtended.PctChg = pctChg
		stockInfoExtended.ChangePercent = pctChg
	}

	// Add metadata from the layered response
	stockInfoExtended.Latency = response.Meta.Latency
	stockInfoExtended.DataSource = response.Meta.Source
	stockInfoExtended.Cached = response.Meta.Cached
	stockInfoExtended.Version = response.Meta.Version
	stockInfoExtended.RequestTime = response.Meta.Timestamp

	return stockInfoExtended, nil
}

// convertOriginalToExtended converts original StockInfo to StockInfoExtended
func (s *StockApiIntegration) convertOriginalToExtended(originalInfo *data.StockInfo, stockCode string) *data.StockInfoExtended {
	return &data.StockInfoExtended{
		StockInfo:   *originalInfo,
		TSCode:      stockCode,
		Latency:     0, // No latency info from original API
		DataSource:  "original_api",
		Cached:      false,
		Version:     "legacy",
		RequestTime: time.Now(),
	}
}

// GetLayerStatus returns status information about each layer
func (s *StockApiIntegration) GetLayerStatus(ctx context.Context) map[string]map[string]any {
	return map[string]map[string]any{
		"market": {
			"name":        s.marketLayer.GetName(),
			"version":     s.marketLayer.GetVersion(),
			"endpoints":   s.marketLayer.GetEndpoints(),
			"fallbacks":   s.marketLayer.GetFallbackEndpoints(),
			"cache_status": "active",
		},
		"sentiment": {
			"name":        s.sentimentLayer.GetName(),
			"version":     s.sentimentLayer.GetVersion(),
			"endpoints":   s.sentimentLayer.GetEndpoints(),
			"fallbacks":   s.sentimentLayer.GetFallbackEndpoints(),
			"cache_status": "active",
		},
		"capital_flow": {
			"name":        s.capitalFlowLayer.GetName(),
			"version":     s.capitalFlowLayer.GetVersion(),
			"endpoints":   s.capitalFlowLayer.GetEndpoints(),
			"fallbacks":   s.capitalFlowLayer.GetFallbackEndpoints(),
			"cache_status": "active",
		},
		"announcement": {
			"name":        s.announcementLayer.GetName(),
			"version":     s.announcementLayer.GetVersion(),
			"endpoints":   s.announcementLayer.GetEndpoints(),
			"fallbacks":   s.announcementLayer.GetFallbackEndpoints(),
			"cache_status": "active",
		},
	}
}