// backend/data/layers/research_report_layer.go
package layers

import (
	"context"
	"fmt"
	"time"

	"go-stock/backend/data/types"
	"go-stock/backend/logger"
)

// ResearchReportLayer handles research report data
type ResearchReportLayer struct {
	config *types.DataSourceConfig
	client *types.HTTPClient
	cache  *types.MultiLevelCache
}

// NewResearchReportLayer creates a new research report layer
func NewResearchReportLayer(config *types.DataSourceConfig) *ResearchReportLayer {
	return &ResearchReportLayer{
		config: config,
		client: types.NewHTTPClient(),
		cache:  types.NewMultiLevelCache(),
	}
}

func (r *ResearchReportLayer) GetName() string {
	return "ResearchReportLayer"
}

func (r *ResearchReportLayer) GetVersion() string {
	return "1.0.0"
}

func (r *ResearchReportLayer) GetEndpoints() []types.Endpoint {
	return []types.Endpoint{r.config.Primary}
}

func (r *ResearchReportLayer) GetFallbackEndpoints() []types.Endpoint {
	return r.config.Fallbacks
}

func (r *ResearchReportLayer) FetchData(ctx context.Context, params map[string]any) (*types.StandardizedResponse, error) {
	stockCode, ok := params["stock_code"].(string)
	if !ok {
		return nil, fmt.Errorf("stock_code is required")
	}

	days := 30
	if d, ok := params["days"].(int); ok {
		days = d
	}

	// Check cache
	cacheKey := fmt.Sprintf("research_reports:%s:%d", stockCode, days)
	if cached, err := r.cache.Get(cacheKey); err == nil {
		return cached.(*types.StandardizedResponse), nil
	}

	// Fetch data with fallback logic
	start := time.Now()
	response, err := r.fetchReports(ctx, r.config.Primary, stockCode, days)
	if err == nil {
		response.Meta.Latency = time.Since(start).Milliseconds()
		response.Meta.Source = r.config.Primary.Name
		r.cache.Set(cacheKey, response, time.Hour*24)
		return response, nil
	}

	logger.SugaredLogger.Warnw("Primary research report source failed, trying fallbacks",
		"error", err, "source", r.config.Primary.Name)

	// Try fallback sources
	for _, fallback := range r.config.Fallbacks {
		response, err := r.fetchReports(ctx, fallback, stockCode, days)
		if err == nil {
			response.Meta.Latency = time.Since(start).Milliseconds()
			response.Meta.Source = fallback.Name
			response.Meta.FallbackUsed = true
			r.cache.Set(cacheKey, response, time.Hour*24)
			return response, nil
		}
	}

	return nil, fmt.Errorf("all research report sources failed")
}

func (r *ResearchReportLayer) fetchReports(ctx context.Context, endpoint types.Endpoint, stockCode string, days int) (*types.StandardizedResponse, error) {
	// Integrate with existing research report fetching logic
	// This would call methods from tool_stock_research_report.go

	reports := []map[string]any{
		{
			"id":           "report1",
			"title":        "Test Report 1",
			"institution":  "Test Institution",
			"author":       "Test Author",
			"rating":       "BUY",
			"target_price": 12.50,
			"publish_time": time.Now(),
		},
	}

	return &types.StandardizedResponse{
		Code:    0,
		Message: "success",
		Data:    reports,
		Meta: types.ResponseMeta{
			Source:       endpoint.Name,
			FallbackUsed: false,
			Timestamp:    time.Now(),
			Version:      r.GetVersion(),
		},
	}, nil
}

func (r *ResearchReportLayer) ValidateParams(params map[string]any) error {
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