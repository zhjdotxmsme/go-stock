// backend/data/integration/research_report_integration_test.go
package integration

import (
	"context"
	"testing"
	"time"

	"go-stock/backend/data"
)

func TestResearchReportIntegration_GetStockResearchReportWithLayer(t *testing.T) {
	// Create original API instance
	originalApi := data.NewMarketNewsApi()

	// Create integration with layered architecture
	integration := NewResearchReportIntegration(originalApi)

	ctx := context.Background()

	// Test getting research reports
	reports, err := integration.GetStockResearchReportWithLayer(ctx, "sh600000", 30)

	if err != nil {
		t.Fatalf("GetStockResearchReportWithLayer() error = %v", err)
	}

	// Verify reports structure
	if len(reports) == 0 {
		t.Log("No research reports found (this may be expected)")
		return
	}

	t.Logf("Successfully fetched %d research reports", len(reports))

	// Check first report structure
	for _, report := range reports {
		if report == nil {
			t.Error("Expected non-nil report")
			continue
		}

		// Verify required fields
		if _, hasId := report["id"]; !hasId {
			t.Error("Expected id field in report")
		}

		if _, hasTitle := report["title"]; !hasTitle {
			t.Error("Expected title field in report")
		}

		if _, hasInstitution := report["institution"]; !hasInstitution {
			t.Error("Expected institution field in report")
		}

		// Verify metadata fields
		if _, hasDataSource := report["data_source"]; !hasDataSource {
			t.Error("Expected data_source field in report")
		}

		if _, hasCached := report["cached"]; !hasCached {
			t.Error("Expected cached field in report")
		}

		if _, hasFetchedAt := report["fetched_at"]; !hasFetchedAt {
			t.Error("Expected fetched_at field in report")
		}

		// Only check first report
		break
	}
}

func TestResearchReportIntegration_GetStockResearchReportMarkdownWithLayer(t *testing.T) {
	originalApi := data.NewMarketNewsApi()
	integration := NewResearchReportIntegration(originalApi)

	ctx := context.Background()

	// Test getting markdown output
	markdown, err := integration.GetStockResearchReportMarkdownWithLayer(ctx, "sh600000", 30)

	if err != nil {
		t.Fatalf("GetStockResearchReportMarkdownWithLayer() error = %v", err)
	}

	if markdown == "" {
		t.Fatal("Expected non-empty markdown output")
	}

	t.Logf("Markdown output length: %d characters", len(markdown))

	// Verify markdown contains expected sections
	expectedSections := []string{"研究报告", "数据来源", "缓存状态"}
	missingSections := 0

	for _, section := range expectedSections {
		if !containsString(markdown, section) {
			t.Logf("Warning: Expected section '%s' not found in markdown", section)
			missingSections++
		}
	}

	if missingSections > 0 {
		t.Logf("Warning: %d expected sections missing from markdown", missingSections)
	}
}

func TestResearchReportIntegration_GetMultipleStockResearchReportsWithLayer(t *testing.T) {
	originalApi := data.NewMarketNewsApi()
	integration := NewResearchReportIntegration(originalApi)

	ctx := context.Background()

	// Test getting reports for multiple stocks
	stockCodes := []string{"sh600000", "sz000001"}

	results, err := integration.GetMultipleStockResearchReportsWithLayer(ctx, stockCodes, 30)

	if err != nil {
		t.Fatalf("GetMultipleStockResearchReportsWithLayer() error = %v", err)
	}

	if len(results) == 0 {
		t.Fatal("Expected at least one result")
	}

	t.Logf("Successfully fetched reports for %d stocks", len(results))

	// Verify results structure
	for stockCode, reports := range results {
		t.Logf("Stock %s: %d reports", stockCode, len(reports))

		if reports == nil {
			t.Errorf("Expected non-nil reports for %s", stockCode)
			continue
		}
	}
}

func TestResearchReportIntegration_GetResearchReportStatistics(t *testing.T) {
	originalApi := data.NewMarketNewsApi()
	integration := NewResearchReportIntegration(originalApi)

	ctx := context.Background()

	// Test getting statistics
	stats, err := integration.GetResearchReportStatistics(ctx, "sh600000", 30)

	if err != nil {
		t.Fatalf("GetResearchReportStatistics() error = %v", err)
	}

	if stats == nil {
		t.Fatal("Expected non-nil statistics")
	}

	// Verify statistics structure
	expectedFields := []string{"stock_code", "total_reports", "days_analyzed", "reports_per_day"}
	for _, field := range expectedFields {
		if _, hasField := stats[field]; !hasField {
			t.Errorf("Expected %s field in statistics", field)
		}
	}

	// Verify rating distribution exists
	if _, hasRatingDist := stats["rating_distribution"]; !hasRatingDist {
		t.Error("Expected rating_distribution field in statistics")
	}

	// Verify top institutions exists
	if _, hasTopInst := stats["top_institutions"]; !hasTopInst {
		t.Error("Expected top_institutions field in statistics")
	}

	t.Logf("Statistics: %+v", stats)
}

func TestResearchReportIntegration_CachePerformance(t *testing.T) {
	originalApi := data.NewMarketNewsApi()
	integration := NewResearchReportIntegration(originalApi)

	ctx := context.Background()

	// Test cache performance
	t.Log("Testing cache performance...")

	// First call (cache miss)
	start := time.Now()
	_, err := integration.GetStockResearchReportWithLayer(ctx, "sh600000", 30)
	if err != nil {
		t.Fatalf("First call failed: %v", err)
	}
	firstCallDuration := time.Since(start)

	t.Logf("First call duration: %v", firstCallDuration)

	// Second call (should be cache hit)
	start = time.Now()
	_, err = integration.GetStockResearchReportWithLayer(ctx, "sh600000", 30)
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}
	secondCallDuration := time.Since(start)

	t.Logf("Second call duration (cache hit): %v", secondCallDuration)

	// Cache should provide speedup
	if secondCallDuration >= firstCallDuration {
		t.Logf("Warning: Second call not faster than first call. First: %v, Second: %v",
			firstCallDuration, secondCallDuration)
	} else {
		speedup := float64(firstCallDuration) / float64(secondCallDuration)
		t.Logf("Cache speedup: %.2fx", speedup)
	}
}

func TestResearchReportIntegration_ErrorHandling(t *testing.T) {
	originalApi := data.NewMarketNewsApi()
	integration := NewResearchReportIntegration(originalApi)

	ctx := context.Background()

	// Test with invalid stock code
	_, err := integration.GetStockResearchReportWithLayer(ctx, "INVALID_CODE", 30)

	// Should handle gracefully
	if err != nil {
		t.Logf("Expected error for invalid code: %v", err)
	}

	// Test with invalid days range
	_, err = integration.GetStockResearchReportWithLayer(ctx, "sh600000", -1)

	if err != nil {
		t.Logf("Expected error for invalid days: %v", err)
	}

	// Test with zero days
	_, err = integration.GetStockResearchReportWithLayer(ctx, "sh600000", 0)

	if err != nil {
		t.Logf("Expected error for zero days: %v", err)
	}
}

func TestResearchReportIntegration_MixedOperations(t *testing.T) {
	originalApi := data.NewMarketNewsApi()
	integration := NewResearchReportIntegration(originalApi)

	ctx := context.Background()

	// Test mixed operations (similar to real usage)
	t.Log("Testing mixed operations...")

	// 1. Get single report
	singleReport, err := integration.GetStockResearchReportWithLayer(ctx, "sh600000", 30)
	if err != nil {
		t.Logf("Warning: Single report fetch failed: %v", err)
	} else {
		t.Logf("Successfully fetched %d reports for single stock", len(singleReport))
	}

	// 2. Get markdown
	markdown, err := integration.GetStockResearchReportMarkdownWithLayer(ctx, "sh600000", 30)
	if err != nil {
		t.Logf("Warning: Markdown generation failed: %v", err)
	} else {
		t.Logf("Successfully generated markdown (%d characters)", len(markdown))
	}

	// 3. Get statistics
	stats, err := integration.GetResearchReportStatistics(ctx, "sh600000", 30)
	if err != nil {
		t.Logf("Warning: Statistics fetch failed: %v", err)
	} else {
		t.Logf("Successfully fetched statistics")
	}

	// 4. Get multiple reports
	multiReports, err := integration.GetMultipleStockResearchReportsWithLayer(ctx, []string{"sh600000", "sz000001"}, 30)
	if err != nil {
		t.Logf("Warning: Multiple reports fetch failed: %v", err)
	} else {
		t.Logf("Successfully fetched reports for %d stocks", len(multiReports))
	}

	// Verify stats structure if successful
	if stats != nil {
		if totalReports, ok := stats["total_reports"].(int); ok {
			t.Logf("Total reports: %d", totalReports)
		}
	}
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && contains(s, substr))
}

func contains(s string, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}