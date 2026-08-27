// backend/data/integration/stock_api_integration_test.go
package integration

import (
	"context"
	"testing"
	"time"

	"go-stock/backend/data"
)

func TestStockApiIntegration_GetStockCodeRealTimeDataWithFallback(t *testing.T) {
	// Create original API instance
	originalApi := &data.StockDataApi{}

	// Create integration with layered architecture
	integration := NewStockApiIntegration(originalApi)

	ctx := context.Background()

	// Test getting stock data with fallback
	result, err := integration.GetStockCodeRealTimeDataWithFallback(ctx, "sh600000")

	if err != nil {
		t.Fatalf("GetStockCodeRealTimeDataWithFallback() error = %v", err)
	}

	if len(result) == 0 {
		t.Fatal("Expected at least one stock result")
	}

	stockInfo, exists := result["sh600000"]
	if !exists {
		t.Fatal("Expected stock info for sh600000")
	}

	if stockInfo == nil {
		t.Fatal("Expected non-nil stock info")
	}

	// Verify required fields
	if stockInfo.TSCode == "" {
		t.Error("Expected TSCode to be set")
	}

	if stockInfo.Date == "" {
		t.Error("Expected Date to be set")
	}

	if stockInfo.Time == "" {
		t.Error("Expected Time to be set")
	}

	// Check if metadata is populated
	if stockInfo.Latency <= 0 {
		t.Logf("Warning: Latency not populated, got %d", stockInfo.Latency)
	}

	if stockInfo.DataSource == "" {
		t.Logf("Warning: DataSource not populated, got %s", stockInfo.DataSource)
	}

	// Test ToMap conversion
	dataMap := stockInfo.ToMap()
	if _, hasCode := dataMap["code"]; !hasCode {
		t.Error("Expected code field in ToMap output")
	}

	if _, hasCurrent := dataMap["current"]; !hasCurrent {
		t.Error("Expected current field in ToMap output")
	}

	if _, hasCached := dataMap["cached"]; !hasCached {
		t.Error("Expected cached field in ToMap output")
	}
}

func TestStockApiIntegration_GetStockSentiment(t *testing.T) {
	originalApi := &data.StockDataApi{}
	integration := NewStockApiIntegration(originalApi)

	ctx := context.Background()

	sentimentData, err := integration.GetStockSentiment(ctx, "sh600000", "2024-01-01", "2024-01-31")

	if err != nil {
		t.Fatalf("GetStockSentiment() error = %v", err)
	}

	if len(sentimentData) == 0 {
		t.Error("Expected at least one sentiment record")
	}

	// Verify sentiment data structure
	for _, record := range sentimentData {
		if _, hasDate := record["date"]; !hasDate {
			t.Error("Expected date field in sentiment data")
		}

		if _, hasSentiment := record["sentiment"]; !hasSentiment {
			t.Error("Expected sentiment field in sentiment data")
		}

		if _, hasScore := record["sentiment_score"]; !hasScore {
			t.Error("Expected sentiment_score field in sentiment data")
		}
	}
}

func TestStockApiIntegration_GetStockCapitalFlow(t *testing.T) {
	originalApi := &data.StockDataApi{}
	integration := NewStockApiIntegration(originalApi)

	ctx := context.Background()

	capitalFlowData, err := integration.GetStockCapitalFlow(ctx, "sh600000", "2024-01-15")

	if err != nil {
		t.Fatalf("GetStockCapitalFlow() error = %v", err)
	}

	// Verify capital flow data structure
	if _, hasMainFlow := capitalFlowData["main_flow"]; !hasMainFlow {
		t.Error("Expected main_flow field in capital flow data")
	}

	if _, hasStockCode := capitalFlowData["stock_code"]; !hasStockCode {
		t.Error("Expected stock_code field in capital flow data")
	}

	if _, hasDate := capitalFlowData["date"]; !hasDate {
		t.Error("Expected date field in capital flow data")
	}
}

func TestStockApiIntegration_GetStockAnnouncements(t *testing.T) {
	originalApi := &data.StockDataApi{}
	integration := NewStockApiIntegration(originalApi)

	ctx := context.Background()

	announcementData, err := integration.GetStockAnnouncements(ctx, "sh600000", 30)

	if err != nil {
		t.Fatalf("GetStockAnnouncements() error = %v", err)
	}

	if len(announcementData) == 0 {
		t.Error("Expected at least one announcement")
	}

	// Verify announcement data structure
	for _, announcement := range announcementData {
		if _, hasId := announcement["id"]; !hasId {
			t.Error("Expected id field in announcement data")
		}

		if _, hasTitle := announcement["title"]; !hasTitle {
			t.Error("Expected title field in announcement data")
		}

		if _, hasType := announcement["type"]; !hasType {
			t.Error("Expected type field in announcement data")
		}
	}
}

func TestStockApiIntegration_GetLayerStatus(t *testing.T) {
	originalApi := &data.StockDataApi{}
	integration := NewStockApiIntegration(originalApi)

	ctx := context.Background()

	layerStatus := integration.GetLayerStatus(ctx)

	// Verify layer status structure
	expectedLayers := []string{"market", "sentiment", "capital_flow", "announcement"}

	for _, layerName := range expectedLayers {
		status, exists := layerStatus[layerName]
		if !exists {
			t.Errorf("Expected layer status for %s", layerName)
			continue
		}

		if _, hasName := status["name"]; !hasName {
			t.Errorf("Expected name field in %s status", layerName)
		}

		if _, hasVersion := status["version"]; !hasVersion {
			t.Errorf("Expected version field in %s status", layerName)
		}

		if _, hasCacheStatus := status["cache_status"]; !hasCacheStatus {
			t.Errorf("Expected cache_status field in %s status", layerName)
		}
	}
}

func TestStockApiIntegration_MixedDataFetching(t *testing.T) {
	originalApi := &data.StockDataApi{}
	integration := NewStockApiIntegration(originalApi)

	ctx := context.Background()

	// Test fetching different types of data in sequence
	// This simulates real-world usage patterns

	// 1. Get real-time stock data
	stockData, err := integration.GetStockCodeRealTimeDataWithFallback(ctx, "sh600000", "sz000001")
	if err != nil {
		t.Logf("Warning: Stock data fetch failed (might be expected): %v", err)
	}

	if len(stockData) > 0 {
		t.Logf("Successfully fetched %d stock records", len(stockData))
	}

	// 2. Get sentiment data
	sentimentData, err := integration.GetStockSentiment(ctx, "sh600000", "2024-01-01", "2024-01-31")
	if err != nil {
		t.Fatalf("GetStockSentiment() failed: %v", err)
	}

	t.Logf("Successfully fetched %d sentiment records", len(sentimentData))

	// 3. Get capital flow data
	_, err = integration.GetStockCapitalFlow(ctx, "sh600000", "2024-01-15")
	if err != nil {
		t.Fatalf("GetStockCapitalFlow() failed: %v", err)
	}

	t.Logf("Successfully fetched capital flow data")

	// 4. Get announcements
	announcementData, err := integration.GetStockAnnouncements(ctx, "sh600000", 30)
	if err != nil {
		t.Fatalf("GetStockAnnouncements() failed: %v", err)
	}

	t.Logf("Successfully fetched %d announcements", len(announcementData))
}

func TestStockApiIntegration_Performance(t *testing.T) {
	originalApi := &data.StockDataApi{}
	integration := NewStockApiIntegration(originalApi)

	ctx := context.Background()

	// Test performance with cache warming
	t.Log("Testing cache performance...")

	// First call (cache miss)
	start := time.Now()
	_, err := integration.GetStockSentiment(ctx, "sh600000", "2024-01-01", "2024-01-31")
	if err != nil {
		t.Fatalf("First call failed: %v", err)
	}
	firstCallDuration := time.Since(start)

	t.Logf("First call duration: %v", firstCallDuration)

	// Second call (should be cache hit)
	start = time.Now()
	_, err = integration.GetStockSentiment(ctx, "sh600000", "2024-01-01", "2024-01-31")
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}
	secondCallDuration := time.Since(start)

	t.Logf("Second call duration (cache hit): %v", secondCallDuration)

	// Cache should provide significant speedup
	if secondCallDuration >= firstCallDuration {
		t.Logf("Warning: Second call not faster than first call. First: %v, Second: %v",
			firstCallDuration, secondCallDuration)
	} else {
		speedup := float64(firstCallDuration) / float64(secondCallDuration)
		t.Logf("Cache speedup: %.2fx", speedup)

		if speedup < 2.0 {
			t.Logf("Warning: Cache speedup less than 2x, got %.2fx", speedup)
		}
	}
}