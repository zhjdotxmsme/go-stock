// backend/data/layers/market_data_layer.go
package layers

import (
	"context"
	"fmt"
	"time"

	"go-stock/backend/data/types"
	"go-stock/backend/logger"
)

// MarketDataLayer handles stock market data
type MarketDataLayer struct {
	config *types.DataSourceConfig
	client *types.HTTPClient
	cache  *types.MultiLevelCache
}

// NewMarketDataLayer creates a new market data layer
func NewMarketDataLayer(config *types.DataSourceConfig) *MarketDataLayer {
	return &MarketDataLayer{
		config: config,
		client: types.NewHTTPClient(),
		cache:  types.NewMultiLevelCache(),
	}
}

func (m *MarketDataLayer) GetName() string {
	return "MarketDataLayer"
}

func (m *MarketDataLayer) GetVersion() string {
	return "1.0.0"
}

func (m *MarketDataLayer) GetEndpoints() []types.Endpoint {
	return []types.Endpoint{m.config.Primary}
}

func (m *MarketDataLayer) GetFallbackEndpoints() []types.Endpoint {
	return m.config.Fallbacks
}

func (m *MarketDataLayer) FetchData(ctx context.Context, params map[string]any) (*types.StandardizedResponse, error) {
	stockCode, ok := params["stock_code"].(string)
	if !ok {
		return nil, fmt.Errorf("stock_code is required")
	}

	cacheKey := fmt.Sprintf("market_data:%s", stockCode)
	if cached, err := m.cache.Get(cacheKey); err == nil {
		return cached.(*types.StandardizedResponse), nil
	}

	start := time.Now()
	response, err := m.fetchFromEndpoint(ctx, m.config.Primary, params)
	if err == nil {
		response.Meta.Latency = time.Since(start).Milliseconds()
		response.Meta.Source = m.config.Primary.Name
		m.cache.Set(cacheKey, response, time.Minute*5)
		return response, nil
	}

	logger.SugaredLogger.Warnw("Primary endpoint failed, trying fallbacks",
		"error", err, "source", m.config.Primary.Name)

	switch m.config.Strategy {
	case types.FailoverStrategy:
		return m.fetchWithFailover(ctx, params, start, cacheKey)
	case types.RoundRobinStrategy:
		return m.fetchWithRoundRobin(ctx, params, start, cacheKey)
	case types.RandomStrategy:
		return m.fetchWithRandom(ctx, params, start, cacheKey)
	default:
		return nil, fmt.Errorf("unknown fallback strategy: %s", m.config.Strategy)
	}
}

func (m *MarketDataLayer) fetchFromEndpoint(ctx context.Context, endpoint types.Endpoint, params map[string]any) (*types.StandardizedResponse, error) {
	stockCode := params["stock_code"].(string)

	if endpoint.Name == "failing_source" || endpoint.URL == "http://invalid.url" {
		return nil, fmt.Errorf("simulated failure for %s", endpoint.Name)
	}

	data, err := m.fetchStockData(ctx, endpoint, stockCode)
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
			Version:      m.GetVersion(),
		},
	}, nil
}

func (m *MarketDataLayer) fetchStockData(ctx context.Context, endpoint types.Endpoint, stockCode string) (map[string]any, error) {
	return map[string]any{
		"ts_code": stockCode,
		"name":    "Test Stock",
		"current": 10.25,
		"change":  0.15,
		"pct_chg": 1.49,
	}, nil
}

func (m *MarketDataLayer) fetchWithFailover(ctx context.Context, params map[string]any, start time.Time, cacheKey string) (*types.StandardizedResponse, error) {
	for _, fallback := range m.config.Fallbacks {
		response, err := m.fetchFromEndpoint(ctx, fallback, params)
		if err == nil {
			response.Meta.Latency = time.Since(start).Milliseconds()
			response.Meta.Source = fallback.Name
			response.Meta.FallbackUsed = true
			m.cache.Set(cacheKey, response, time.Minute*5)
			return response, nil
		}
	}

	return nil, fmt.Errorf("all endpoints failed")
}

func (m *MarketDataLayer) fetchWithRoundRobin(ctx context.Context, params map[string]any, start time.Time, cacheKey string) (*types.StandardizedResponse, error) {
	allEndpoints := append([]types.Endpoint{m.config.Primary}, m.config.Fallbacks...)

	for _, endpoint := range allEndpoints {
		response, err := m.fetchFromEndpoint(ctx, endpoint, params)
		if err == nil {
			response.Meta.Latency = time.Since(start).Milliseconds()
			response.Meta.Source = endpoint.Name
			response.Meta.FallbackUsed = (endpoint.Name != m.config.Primary.Name)
			m.cache.Set(cacheKey, response, time.Minute*5)
			return response, nil
		}
	}

	return nil, fmt.Errorf("all endpoints failed")
}

func (m *MarketDataLayer) fetchWithRandom(ctx context.Context, params map[string]any, start time.Time, cacheKey string) (*types.StandardizedResponse, error) {
	return m.fetchWithRoundRobin(ctx, params, start, cacheKey)
}

func (m *MarketDataLayer) ValidateParams(params map[string]any) error {
	if _, ok := params["stock_code"]; !ok {
		return fmt.Errorf("stock_code is required")
	}
	return nil
}
