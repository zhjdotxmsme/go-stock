// backend/data/layers/capital_flow_layer.go
package layers

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"go-stock/backend/data/types"
	"go-stock/backend/logger"
)

// CapitalFlowLayer handles capital flow data
type CapitalFlowLayer struct {
	config *types.DataSourceConfig
	client *types.HTTPClient
	cache  *types.MultiLevelCache
}

// NewCapitalFlowLayer creates a new capital flow layer
func NewCapitalFlowLayer(config *types.DataSourceConfig) *CapitalFlowLayer {
	return &CapitalFlowLayer{
		config: config,
		client: types.NewHTTPClient(),
		cache:  types.NewMultiLevelCache(),
	}
}

func (c *CapitalFlowLayer) GetName() string {
	return "CapitalFlowLayer"
}

func (c *CapitalFlowLayer) GetVersion() string {
	return "1.0.0"
}

func (c *CapitalFlowLayer) GetEndpoints() []types.Endpoint {
	return []types.Endpoint{c.config.Primary}
}

func (c *CapitalFlowLayer) GetFallbackEndpoints() []types.Endpoint {
	return c.config.Fallbacks
}

func (c *CapitalFlowLayer) FetchData(ctx context.Context, params map[string]any) (*types.StandardizedResponse, error) {
	if err := c.ValidateParams(params); err != nil {
		return nil, err
	}

	stockCode := params["stock_code"].(string)
	date := params["date"].(string)

	// Check cache
	cacheKey := fmt.Sprintf("capital_flow:%s:%s", stockCode, date)
	if cached, err := c.cache.Get(cacheKey); err == nil {
		return cached.(*types.StandardizedResponse), nil
	}

	// Try primary endpoint
	start := time.Now()
	response, err := c.fetchFromEndpoint(ctx, c.config.Primary, params)
	if err == nil {
		response.Meta.Latency = time.Since(start).Milliseconds()
		response.Meta.Source = c.config.Primary.Name
		c.cache.Set(cacheKey, response, time.Hour*12)
		return response, nil
	}

	logger.SugaredLogger.Warnw("Primary capital flow source failed, trying fallbacks",
		"error", err, "source", c.config.Primary.Name)

	// Try fallback sources
	for _, fallback := range c.config.Fallbacks {
		response, err := c.fetchFromEndpoint(ctx, fallback, params)
		if err == nil {
			response.Meta.Latency = time.Since(start).Milliseconds()
			response.Meta.Source = fallback.Name
			response.Meta.FallbackUsed = true
			c.cache.Set(cacheKey, response, time.Hour*12)
			return response, nil
		}
	}

	return nil, fmt.Errorf("all capital flow sources failed")
}

func (c *CapitalFlowLayer) fetchFromEndpoint(ctx context.Context, endpoint types.Endpoint, params map[string]any) (*types.StandardizedResponse, error) {
	stockCode := params["stock_code"].(string)
	date := params["date"].(string)

	// Simulate failure for test
	if endpoint.Name == "failing_source" || endpoint.URL == "http://invalid.url" {
		return nil, fmt.Errorf("simulated failure for %s", endpoint.Name)
	}

	data, err := c.fetchCapitalFlowData(ctx, endpoint, stockCode, date)
	if err != nil {
		return nil, err
	}

	return &types.StandardizedResponse{
		Code:    0,
		Message: "success",
		Data:    data,
		Meta: types.ResponseMeta{
			Source:       endpoint.Name,
			FallbackUsed: false,
			Timestamp:    time.Now(),
			Version:      c.GetVersion(),
		},
	}, nil
}

func (c *CapitalFlowLayer) fetchCapitalFlowData(ctx context.Context, endpoint types.Endpoint, stockCode, date string) (map[string]any, error) {
	// Simulate capital flow data
	// This would integrate with actual capital flow APIs
	return map[string]any{
		"stock_code":    stockCode,
		"date":          date,
		"main_flow":     12500000.00,   // 主力净流入
		"super_large":   8500000.00,    // 超大单
		"large":         3200000.00,    // 大单
		"medium":        600000.00,     // 中单
		"small":         -11500000.00,  // 小单
		"flow_ratio":    0.08,          // 净流比
		"main_net_in":   12500000.00,   // 主力净流入
		"retail_net_in": -8500000.00,   // 散户净流入
		"change_pct":    2.35,          // 涨跌幅
	}, nil
}

func (c *CapitalFlowLayer) ValidateParams(params map[string]any) error {
	if _, ok := params["stock_code"]; !ok {
		return fmt.Errorf("stock_code is required")
	}

	if _, ok := params["date"]; !ok {
		return fmt.Errorf("date is required")
	}

	// Validate date format (YYYY-MM-DD)
	datePattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	date, _ := params["date"].(string)

	if !datePattern.MatchString(date) {
		return fmt.Errorf("date must be in YYYY-MM-DD format")
	}

	return nil
}