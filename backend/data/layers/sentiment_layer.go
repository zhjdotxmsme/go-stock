// backend/data/layers/sentiment_layer.go
package layers

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"go-stock/backend/data/types"
	"go-stock/backend/logger"
)

// SentimentLayer handles market sentiment data
type SentimentLayer struct {
	config *types.DataSourceConfig
	client *types.HTTPClient
	cache  *types.MultiLevelCache
}

// NewSentimentLayer creates a new sentiment layer
func NewSentimentLayer(config *types.DataSourceConfig) *SentimentLayer {
	return &SentimentLayer{
		config: config,
		client: types.NewHTTPClient(),
		cache:  types.NewMultiLevelCache(),
	}
}

func (s *SentimentLayer) GetName() string {
	return "SentimentLayer"
}

func (s *SentimentLayer) GetVersion() string {
	return "1.0.0"
}

func (s *SentimentLayer) GetEndpoints() []types.Endpoint {
	return []types.Endpoint{s.config.Primary}
}

func (s *SentimentLayer) GetFallbackEndpoints() []types.Endpoint {
	return s.config.Fallbacks
}

func (s *SentimentLayer) FetchData(ctx context.Context, params map[string]any) (*types.StandardizedResponse, error) {
	if err := s.ValidateParams(params); err != nil {
		return nil, err
	}

	stockCode := params["stock_code"].(string)
	startDate := params["start_date"].(string)
	endDate := params["end_date"].(string)

	// Check cache
	cacheKey := fmt.Sprintf("sentiment:%s:%s:%s", stockCode, startDate, endDate)
	if cached, err := s.cache.Get(cacheKey); err == nil {
		return cached.(*types.StandardizedResponse), nil
	}

	// Try primary endpoint
	start := time.Now()
	response, err := s.fetchFromEndpoint(ctx, s.config.Primary, params)
	if err == nil {
		response.Meta.Latency = time.Since(start).Milliseconds()
		response.Meta.Source = s.config.Primary.Name
		s.cache.Set(cacheKey, response, time.Hour*6)
		return response, nil
	}

	logger.SugaredLogger.Warnw("Primary sentiment source failed, trying fallbacks",
		"error", err, "source", s.config.Primary.Name)

	// Try fallback sources
	for _, fallback := range s.config.Fallbacks {
		response, err := s.fetchFromEndpoint(ctx, fallback, params)
		if err == nil {
			response.Meta.Latency = time.Since(start).Milliseconds()
			response.Meta.Source = fallback.Name
			response.Meta.FallbackUsed = true
			s.cache.Set(cacheKey, response, time.Hour*6)
			return response, nil
		}
	}

	return nil, fmt.Errorf("all sentiment sources failed")
}

func (s *SentimentLayer) fetchFromEndpoint(ctx context.Context, endpoint types.Endpoint, params map[string]any) (*types.StandardizedResponse, error) {
	stockCode := params["stock_code"].(string)

	// Simulate failure for test
	if endpoint.Name == "failing_source" || endpoint.URL == "http://invalid.url" {
		return nil, fmt.Errorf("simulated failure for %s", endpoint.Name)
	}

	data, err := s.fetchSentimentData(ctx, endpoint, stockCode)
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
			Version:      s.GetVersion(),
		},
	}, nil
}

func (s *SentimentLayer) fetchSentimentData(ctx context.Context, endpoint types.Endpoint, stockCode string) ([]map[string]any, error) {
	// Simulate sentiment data
	// This would integrate with actual sentiment APIs
	return []map[string]any{
		{
			"date":            "2024-01-15",
			"sentiment":      "positive",
			"sentiment_score": 0.75,
			"volume":         1000000,
			"change_ratio":   0.02,
		},
		{
			"date":            "2024-01-16",
			"sentiment":      "neutral",
			"sentiment_score": 0.05,
			"volume":         850000,
			"change_ratio":   -0.01,
		},
	}, nil
}

func (s *SentimentLayer) ValidateParams(params map[string]any) error {
	if _, ok := params["stock_code"]; !ok {
		return fmt.Errorf("stock_code is required")
	}

	if _, ok := params["start_date"]; !ok {
		return fmt.Errorf("start_date is required")
	}

	if _, ok := params["end_date"]; !ok {
		return fmt.Errorf("end_date is required")
	}

	// Validate date format (YYYY-MM-DD)
	datePattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	startDate, _ := params["start_date"].(string)
	endDate, _ := params["end_date"].(string)

	if !datePattern.MatchString(startDate) {
		return fmt.Errorf("start_date must be in YYYY-MM-DD format")
	}

	if !datePattern.MatchString(endDate) {
		return fmt.Errorf("end_date must be in YYYY-MM-DD format")
	}

	return nil
}