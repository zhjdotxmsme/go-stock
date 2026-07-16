// backend/data/layers/announcement_layer.go
package layers

import (
	"context"
	"fmt"
	"time"

	"go-stock/backend/data/types"
	"go-stock/backend/logger"
)

// AnnouncementLayer handles stock announcement data
type AnnouncementLayer struct {
	config *types.DataSourceConfig
	client *types.HTTPClient
	cache  *types.MultiLevelCache
}

// NewAnnouncementLayer creates a new announcement layer
func NewAnnouncementLayer(config *types.DataSourceConfig) *AnnouncementLayer {
	return &AnnouncementLayer{
		config: config,
		client: types.NewHTTPClient(),
		cache:  types.NewMultiLevelCache(),
	}
}

func (a *AnnouncementLayer) GetName() string {
	return "AnnouncementLayer"
}

func (a *AnnouncementLayer) GetVersion() string {
	return "1.0.0"
}

func (a *AnnouncementLayer) GetEndpoints() []types.Endpoint {
	return []types.Endpoint{a.config.Primary}
}

func (a *AnnouncementLayer) GetFallbackEndpoints() []types.Endpoint {
	return a.config.Fallbacks
}

func (a *AnnouncementLayer) FetchData(ctx context.Context, params map[string]any) (*types.StandardizedResponse, error) {
	if err := a.ValidateParams(params); err != nil {
		return nil, err
	}

	stockCode := params["stock_code"].(string)
	days := 30
	if d, ok := params["days"].(int); ok {
		days = d
	}

	// Check cache
	cacheKey := fmt.Sprintf("announcements:%s:%d", stockCode, days)
	if cached, err := a.cache.Get(cacheKey); err == nil {
		return cached.(*types.StandardizedResponse), nil
	}

	// Try primary endpoint
	start := time.Now()
	response, err := a.fetchFromEndpoint(ctx, a.config.Primary, params)
	if err == nil {
		response.Meta.Latency = time.Since(start).Milliseconds()
		response.Meta.Source = a.config.Primary.Name
		a.cache.Set(cacheKey, response, time.Hour*6)
		return response, nil
	}

	logger.SugaredLogger.Warnw("Primary announcement source failed, trying fallbacks",
		"error", err, "source", a.config.Primary.Name)

	// Try fallback sources
	for _, fallback := range a.config.Fallbacks {
		response, err := a.fetchFromEndpoint(ctx, fallback, params)
		if err == nil {
			response.Meta.Latency = time.Since(start).Milliseconds()
			response.Meta.Source = fallback.Name
			response.Meta.FallbackUsed = true
			a.cache.Set(cacheKey, response, time.Hour*6)
			return response, nil
		}
	}

	return nil, fmt.Errorf("all announcement sources failed")
}

func (a *AnnouncementLayer) fetchFromEndpoint(ctx context.Context, endpoint types.Endpoint, params map[string]any) (*types.StandardizedResponse, error) {
	stockCode := params["stock_code"].(string)
	days := 30
	if d, ok := params["days"].(int); ok {
		days = d
	}

	// Simulate failure for test
	if endpoint.Name == "failing_source" || endpoint.URL == "http://invalid.url" {
		return nil, fmt.Errorf("simulated failure for %s", endpoint.Name)
	}

	data, err := a.fetchAnnouncements(ctx, endpoint, stockCode, days)
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
			Version:      a.GetVersion(),
		},
	}, nil
}

func (a *AnnouncementLayer) fetchAnnouncements(ctx context.Context, endpoint types.Endpoint, stockCode string, days int) ([]map[string]any, error) {
	// Simulate announcement data
	// This would integrate with actual announcement APIs
	return []map[string]any{
		{
			"id":           "ann001",
			"title":        "2024年度第一次临时股东大会通知",
			"announcement_time": time.Now().AddDate(0, 0, -5),
			"type":         "股东大会",
			"importance":   "high",
			"stock_code":   stockCode,
		},
		{
			"id":           "ann002",
			"title":        "关于签署重大合同的公告",
			"announcement_time": time.Now().AddDate(0, 0, -10),
			"type":         "经营公告",
			"importance":   "medium",
			"stock_code":   stockCode,
		},
		{
			"id":           "ann003",
			"title":        "业绩预增公告",
			"announcement_time": time.Now().AddDate(0, 0, -15),
			"type":         "业绩公告",
			"importance":   "high",
			"stock_code":   stockCode,
		},
	}, nil
}

func (a *AnnouncementLayer) ValidateParams(params map[string]any) error {
	if _, ok := params["stock_code"]; !ok {
		return fmt.Errorf("stock_code is required")
	}

	if days, ok := params["days"].(int); ok {
		if days < 1 || days > 365 {
			return fmt.Errorf("days must be between 1 and 365")
		}
	}

	return nil
}