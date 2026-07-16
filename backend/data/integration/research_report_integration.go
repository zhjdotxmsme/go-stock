// backend/data/integration/research_report_integration.go
package integration

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/data/layers"
	"go-stock/backend/data/types"
	"go-stock/backend/logger"
)

// ResearchReportIntegration wraps the existing research report functionality with the new layered architecture
type ResearchReportIntegration struct {
	originalApi   *data.MarketNewsApi
	reportLayer   *layers.ResearchReportLayer
}

// NewResearchReportIntegration creates a new integration for research reports
func NewResearchReportIntegration(originalApi *data.MarketNewsApi) *ResearchReportIntegration {
	// Create research report layer config
	reportConfig := &types.DataSourceConfig{
		Primary: types.Endpoint{
			Name:   "eastmoney_report",
			URL:    "http://report.eastmoney.com",
			Method: types.MethodGet,
		},
		Fallbacks: []types.Endpoint{
			{
				Name:   "eastmoney_backup",
				URL:    "http://report2.eastmoney.com",
				Method: types.MethodGet,
			},
		},
		Strategy: types.FailoverStrategy,
	}

	return &ResearchReportIntegration{
		originalApi: originalApi,
		reportLayer: layers.NewResearchReportLayer(reportConfig),
	}
}

// GetStockResearchReportWithLayer uses the new layered architecture for research reports
func (r *ResearchReportIntegration) GetStockResearchReportWithLayer(ctx context.Context, stockCode string, days int) ([]map[string]any, error) {
	// Use the new ResearchReportLayer
	response, err := r.reportLayer.FetchData(ctx, map[string]any{
		"stock_code": stockCode,
		"days":       days,
	})

	if err != nil {
		logger.SugaredLogger.Warnw("Failed to fetch research report with layers, falling back to original API",
			"stock_code", stockCode, "error", err)

		// Fallback to original API
		originalData := r.originalApi.StockResearchReport(stockCode, days)
		return r.convertOriginalToLayered(originalData), nil
	}

	// Return layered response
	reports, ok := response.Data.([]map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid research report data format")
	}

	// Add metadata to each report
	for i := range reports {
		if reports[i] == nil {
			reports[i] = make(map[string]any)
		}
		reports[i]["data_source"] = response.Meta.Source
		reports[i]["cached"] = response.Meta.Cached
		reports[i]["latency"] = response.Meta.Latency
		reports[i]["fetched_at"] = response.Meta.Timestamp
	}

	return reports, nil
}

// GetStockResearchReportMarkdownWithLayer generates markdown with the layered architecture
func (r *ResearchReportIntegration) GetStockResearchReportMarkdownWithLayer(ctx context.Context, stockCode string, days int) (string, error) {
	// Use the layered architecture
	reports, err := r.GetStockResearchReportWithLayer(ctx, stockCode, days)
	if err != nil {
		return "", fmt.Errorf("failed to get research reports: %w", err)
	}

	// Build markdown output
	var md strings.Builder
	md.WriteString(fmt.Sprintf("### %s 研究报告\r\n", stockCode))

	if len(reports) == 0 {
		md.WriteString(stockCode + "：未查询到相关研究报告。")
		return md.String(), nil
	}

	// Add summary metadata
	md.WriteString(fmt.Sprintf("**数据来源**: %s  |  **缓存状态**: %v  |  **获取时间**: %s\r\n\r\n",
		reports[0]["data_source"],
		reports[0]["cached"],
		time.Now().Format("2006-01-02 15:04:05")))

	// Process each report
	for _, report := range reports {
		if report == nil {
			continue
		}

		// Get industry report info
		infoCode, ok := report["infoCode"].(string)
		if !ok {
			continue
		}

		// Use original API to get detailed report info
		reportInfo := r.originalApi.GetIndustryReportInfo(infoCode)
		md.WriteString(reportInfo)
		md.WriteString("\r\n---\r\n\r\n")
	}

	return strings.TrimSpace(md.String()), nil
}

// GetMultipleStockResearchReportsWithLayer fetches reports for multiple stocks
func (r *ResearchReportIntegration) GetMultipleStockResearchReportsWithLayer(ctx context.Context, stockCodes []string, days int) (map[string][]map[string]any, error) {
	results := make(map[string][]map[string]any)

	for _, stockCode := range stockCodes {
		reports, err := r.GetStockResearchReportWithLayer(ctx, stockCode, days)
		if err != nil {
			logger.SugaredLogger.Warnw("Failed to fetch research report for stock",
				"stock_code", stockCode, "error", err)
			continue
		}

		results[stockCode] = reports
	}

	return results, nil
}

// GetResearchReportStatistics provides statistics about research reports
func (r *ResearchReportIntegration) GetResearchReportStatistics(ctx context.Context, stockCode string, days int) (map[string]any, error) {
	reports, err := r.GetStockResearchReportWithLayer(ctx, stockCode, days)
	if err != nil {
		return nil, fmt.Errorf("failed to get research reports: %w", err)
	}

	stats := map[string]any{
		"stock_code":      stockCode,
		"total_reports":   len(reports),
		"days_analyzed":   days,
		"reports_per_day": float64(len(reports)) / float64(days),
	}

	// Analyze ratings
	ratingCount := make(map[string]int)
	institutionCount := make(map[string]int)

	for _, report := range reports {
		if report == nil {
			continue
		}

		if rating, ok := report["rating"].(string); ok {
			ratingCount[rating]++
		}

		if institution, ok := report["institution"].(string); ok {
			institutionCount[institution]++
		}
	}

	stats["rating_distribution"] = ratingCount
	stats["top_institutions"] = getTopInstitutions(institutionCount, 5)
	stats["dominant_rating"] = getDominantRating(ratingCount)

	return stats, nil
}

// convertOriginalToLayered converts original API response to layered format
func (r *ResearchReportIntegration) convertOriginalToLayered(originalData any) []map[string]any {
	var result []map[string]any

	switch data := originalData.(type) {
	case []any:
		for _, item := range data {
			if report, ok := item.(map[string]any); ok {
				// Add metadata to indicate this came from original API
				report["data_source"] = "original_api"
				report["cached"] = false
				report["latency"] = 0
				report["fetched_at"] = time.Now()
				result = append(result, report)
			}
		}
	case []map[string]any:
		for _, report := range data {
			// Add metadata to indicate this came from original API
			report["data_source"] = "original_api"
			report["cached"] = false
			report["latency"] = 0
			report["fetched_at"] = time.Now()
			result = append(result, report)
		}
	}

	return result
}

// getTopInstitutions returns the top N institutions by report count
func getTopInstitutions(institutionCount map[string]int, topN int) []map[string]any {
	// Convert to slice for sorting
	type institution struct {
		name  string
		count int
	}

	var institutions []institution
	for name, count := range institutionCount {
		institutions = append(institutions, institution{name: name, count: count})
	}

	// Simple selection sort (topN is small)
	for i := 0; i < min(topN, len(institutions)); i++ {
		for j := i + 1; j < len(institutions); j++ {
			if institutions[j].count > institutions[i].count {
				institutions[i], institutions[j] = institutions[j], institutions[i]
			}
		}
	}

	// Convert to result format
	var result []map[string]any
	for i := 0; i < min(topN, len(institutions)); i++ {
		result = append(result, map[string]any{
			"institution": institutions[i].name,
			"count":        institutions[i].count,
		})
	}

	return result
}

// getDominantRating returns the most common rating
func getDominantRating(ratingCount map[string]int) string {
	maxCount := 0
	dominantRating := "N/A"

	for rating, count := range ratingCount {
		if count > maxCount {
			maxCount = count
			dominantRating = rating
		}
	}

	return dominantantRating
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}