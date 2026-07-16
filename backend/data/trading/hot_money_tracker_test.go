// backend/data/trading/hot_money_tracker_test.go
package trading

import (
	"context"
	"testing"
	"time"
)

func TestHotMoneyTracker_AnalyzeHotMoney(t *testing.T) {
	config := &HotMoneyConfig{
		UpdateInterval:    24 * time.Hour,
		TrendLookback:     5,
		RankingThreshold:  0.8,
		ConsecutiveDays:   3,
	}

	tracker := NewHotMoneyTracker(config)
	ctx := context.Background()

	result, err := tracker.AnalyzeHotMoney(ctx, "sh600000", 5)

	if err != nil {
		t.Fatalf("AnalyzeHotMoney() error = %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Verify result structure
	if result.StockCode != "sh600000" {
		t.Errorf("Expected StockCode 'sh600000', got '%s'", result.StockCode)
	}

	if len(result.TopSeats) == 0 {
		t.Error("Expected at least one top seat")
	}

	if result.TotalHotAmount <= 0 {
		t.Errorf("Expected positive TotalHotAmount, got %f", result.TotalHotAmount)
	}

	if result.SeatTrend == "" {
		t.Error("Expected non-empty SeatTrend")
	}

	if result.RiskLevel == "" {
		t.Error("Expected non-empty RiskLevel")
	}

	t.Logf("Hot money analysis completed:")
	t.Logf("  Stock: %s", result.StockCode)
	t.Logf("  Total Amount: %.2f", result.TotalHotAmount)
	t.Logf("  Trend: %s", result.SeatTrend)
	t.Logf("  Risk Level: %s", result.RiskLevel)
	t.Logf("  Top Seats: %d", len(result.TopSeats))
}

func TestHotMoneyTracker_TrackHotMoneyInRealTime(t *testing.T) {
	config := &HotMoneyConfig{}
	tracker := NewHotMoneyTracker(config)
	ctx := context.Background()

	stockCodes := []string{"sh600000", "sh600519", "sz000001"}

	results, err := tracker.TrackHotMoneyInRealTime(ctx, stockCodes)

	if err != nil {
		t.Fatalf("TrackHotMoneyInRealTime() error = %v", err)
	}

	if len(results) == 0 {
		t.Fatal("Expected at least one result")
	}

	// Verify each result
	for stockCode, result := range results {
		t.Logf("Stock %s analysis:", stockCode)
		t.Logf("  Total Amount: %.2f", result.TotalHotAmount)
		t.Logf("  Trend: %s", result.SeatTrend)
		t.Logf("  Risk Level: %s", result.RiskLevel)

		if result.StockCode != stockCode {
			t.Errorf("Expected StockCode '%s', got '%s'", stockCode, result.StockCode)
		}
	}

	t.Logf("Successfully tracked %d stocks in real-time", len(results))
}

func TestHotMoneyTracker_GetHotMoneyRankings(t *testing.T) {
	config := &HotMoneyConfig{}
	tracker := NewHotMoneyTracker(config)
	ctx := context.Background()

	stockCodes := []string{"sh600000", "sh600519", "sz000001", "sz000002"}

	rankings, err := tracker.GetHotMoneyRankings(ctx, stockCodes)

	if err != nil {
		t.Fatalf("GetHotMoneyRankings() error = %v", err)
	}

	if len(rankings) == 0 {
		t.Fatal("Expected at least one ranking")
	}

	// Verify ranking structure
	for i, ranking := range rankings {
		if ranking.StockCode == "" {
			t.Errorf("Expected non-empty StockCode at position %d", i)
		}

		if ranking.TotalAmount < 0 {
			t.Errorf("Expected non-negative TotalAmount at position %d", i)
		}

		if ranking.Trend == "" {
			t.Errorf("Expected non-empty Trend at position %d", i)
		}

		if ranking.RiskLevel == "" {
			t.Errorf("Expected non-empty RiskLevel at position %d", i)
		}

		if ranking.AnalysisScore < 0 || ranking.AnalysisScore > 100 {
			t.Errorf("Expected AnalysisScore between 0 and 100 at position %d, got %f", i, ranking.AnalysisScore)
		}

		// Verify ranking assignment
		if ranking.Ranking != i+1 {
			t.Logf("Warning: Expected Ranking %d, got %d", i+1, ranking.Ranking)
		}
	}

	// Verify rankings are sorted by analysis score (descending)
	for i := 1; i < len(rankings); i++ {
		if rankings[i].AnalysisScore > rankings[i-1].AnalysisScore {
			t.Errorf("Rankings should be sorted by AnalysisScore descending, but found higher score at position %d", i)
		}
	}

	t.Logf("Successfully generated rankings for %d stocks", len(rankings))
}

func TestHotMoneyTracker_SeatIdentification(t *testing.T) {
	config := &HotMoneyConfig{
		RankingThreshold: 0.8,
	}
	tracker := NewHotMoneyTracker(config)
	ctx := context.Background()

	result, err := tracker.AnalyzeHotMoney(ctx, "sh600000", 5)

	if err != nil {
		t.Fatalf("AnalyzeHotMoney() error = %v", err)
	}

	// Verify seat identification
	for _, seat := range result.TopSeats {
		if seat.SeatName == "" {
			t.Error("Expected non-empty SeatName")
		}

		if seat.TotalAmount <= 0 {
			t.Errorf("Expected positive TotalAmount, got %f", seat.TotalAmount)
		}

		if seat.SuccessRate < 0 || seat.SuccessRate > 1 {
			t.Errorf("Expected SuccessRate between 0 and 1, got %f", seat.SuccessRate)
		}

		if seat.Ranking < 0 || seat.Ranking > 1 {
			t.Errorf("Expected Ranking between 0 and 1, got %f", seat.Ranking)
		}

		// Verify ranking threshold
		if seat.Ranking < config.RankingThreshold {
			t.Errorf("Expected Ranking >= threshold %f, got %f", config.RankingThreshold, seat.Ranking)
		}

		if len(seat.TopStocks) == 0 {
			t.Error("Expected at least one TopStock")
		}

		if seat.MovementType == "" {
			t.Error("Expected non-empty MovementType")
		}

		t.Logf("Verified seat: %s", seat.SeatName)
		t.Logf("  Amount: %.2f", seat.TotalAmount)
		t.Logf("  Success Rate: %.2f", seat.SuccessRate)
		t.Logf("  Ranking: %.2f", seat.Ranking)
	}
}

func TestHotMoneyTracker_TrendAnalysis(t *testing.T) {
	config := &HotMoneyConfig{}
	tracker := NewHotMoneyTracker(config)
	ctx := context.Background()

	result, err := tracker.AnalyzeHotMoney(ctx, "sh600000", 5)

	if err != nil {
		t.Fatalf("AnalyzeHotMoney() error = %v", err)
	}

	// Verify trend analysis
	validTrends := map[string]bool{
		"强势流入":  true,
		"持续流入":  true,
		"大单流入":  true,
		"中等流入":  true,
		"资金分散":  true,
	}

	if !validTrends[result.SeatTrend] {
		t.Errorf("Expected valid trend, got '%s'", result.SeatTrend)
	}

	t.Logf("Trend Analysis:")
	t.Logf("  Trend: %s", result.SeatTrend)
	t.Logf("  Direction: %s", result.TrendDirection)
	t.Logf("  Pattern: %s", result.MovementPattern)

	// Verify trend direction
	validDirections := map[string]bool{
		"上升趋势": true,
		"下降趋势": true,
		"资金增长": true,
		"资金减少": true,
		"平稳趋势": true,
		"无法判断": true,
	}

	if !validDirections[result.TrendDirection] {
		t.Errorf("Expected valid direction, got '%s'", result.TrendDirection)
	}

	// Verify movement pattern
	validPatterns := map[string]bool{
		"集中流入":   true,
		"集中流出":   true,
		"净流入":    true,
		"净流出":    true,
		"资金平衡":   true,
		"无明显模式": true,
	}

	if !validPatterns[result.MovementPattern] {
		t.Errorf("Expected valid pattern, got '%s'", result.MovementPattern)
	}
}

func TestHotMoneyTracker_RiskAssessment(t *testing.T) {
	config := &HotMoneyConfig{}
	tracker := NewHotMoneyTracker(config)
	ctx := context.Background()

	result, err := tracker.AnalyzeHotMoney(ctx, "sh600000", 5)

	if err != nil {
		t.Fatalf("AnalyzeHotMoney() error = %v", err)
	}

	// Verify risk assessment
	validRiskLevels := map[string]bool{
		"低风险":  true,
		"中风险": true,
		"高风险":  true,
	}

	if !validRiskLevels[result.RiskLevel] {
		t.Errorf("Expected valid risk level, got '%s'", result.RiskLevel)
	}

	t.Logf("Risk Assessment:")
	t.Logf("  Risk Level: %s", result.RiskLevel)
	t.Logf("  Total Amount: %.2f", result.TotalHotAmount)
	t.Logf("  Top Seats: %d", len(result.TopSeats))

	// Test risk level logic
	if result.SeatTrend == "强势流入" && result.TotalHotAmount > 50000000 {
		if result.RiskLevel != "高风险" && result.RiskLevel != "中风险" {
			t.Errorf("Expected high or medium risk for strong inflow with large amount, got '%s'", result.RiskLevel)
		}
	}
}

func TestHotMoneyTracker_HistoricalTrends(t *testing.T) {
	config := &HotMoneyConfig{
		TrendLookback: 5,
	}
	tracker := NewHotMoneyTracker(config)
	ctx := context.Background()

	result, err := tracker.AnalyzeHotMoney(ctx, "sh600000", 5)

	if err != nil {
		t.Fatalf("AnalyzeHotMoney() error = %v", err)
	}

	// Verify historical trend data
	if len(result.HistoricalData) == 0 {
		t.Error("Expected historical trend data")
	}

	if len(result.HistoricalData) > config.TrendLookback {
		t.Errorf("Expected at most %d historical data points, got %d", config.TrendLookback, len(result.HistoricalData))
	}

	// Verify historical data structure
	for _, trend := range result.HistoricalData {
		if trend.Date == "" {
			t.Error("Expected non-empty Date in historical data")
		}

		if trend.SeatName == "" {
			t.Error("Expected non-empty SeatName in historical data")
		}

		if trend.Amount < 0 {
			t.Errorf("Expected non-negative Amount in historical data, got %f", trend.Amount)
		}

		if trend.Ranking < 0 || trend.Ranking > 1 {
			t.Errorf("Expected Ranking between 0 and 1 in historical data, got %f", trend.Ranking)
		}
	}

	t.Logf("Historical Trends:")
	for i, trend := range result.HistoricalData {
		t.Logf("  [%d] %s: %.2f (Ranking: %.2f)", i, trend.Date, trend.Amount, trend.Ranking)
	}
}

func TestHotMoneyTracker_AnalysisScore(t *testing.T) {
	config := &HotMoneyConfig{}
	tracker := NewHotMoneyTracker(config)
	ctx := context.Background()

	// Test multiple stocks
	stockCodes := []string{"sh600000", "sh600519", "sz000001"}

	rankings, err := tracker.GetHotMoneyRankings(ctx, stockCodes)

	if err != nil {
		t.Fatalf("GetHotMoneyRankings() error = %v", err)
	}

	// Verify analysis scores
	for _, ranking := range rankings {
		if ranking.AnalysisScore < 0 || ranking.AnalysisScore > 100 {
			t.Errorf("Expected AnalysisScore between 0 and 100, got %f", ranking.AnalysisScore)
		}

		// Verify score correlates with amount and trend
		if ranking.TotalAmount > 30000000 && ranking.AnalysisScore < 40 {
			t.Logf("Warning: Low analysis score (%.2f) for large amount (%.2f)", ranking.AnalysisScore, ranking.TotalAmount)
		}

		if ranking.Trend == "强势流入" && ranking.AnalysisScore < 50 {
			t.Logf("Warning: Low analysis score (%.2f) for strong trend", ranking.AnalysisScore)
		}

		t.Logf("Stock %s: Score %.2f (Amount: %.2f, Trend: %s)", ranking.StockCode, ranking.AnalysisScore, ranking.TotalAmount, ranking.Trend)
	}
}

func TestHotMoneyTracker_ConcurrentOperations(t *testing.T) {
	config := &HotMoneyConfig{}
	tracker := NewHotMoneyTracker(config)
	ctx := context.Background()

	stockCodes := []string{"sh600000", "sh600519", "sz000001", "sz000002", "sh600036"}

	// Test concurrent real-time tracking
	startTime := time.Now()
	results, err := tracker.TrackHotMoneyInRealTime(ctx, stockCodes)
	concurrentDuration := time.Since(startTime)

	if err != nil {
		t.Fatalf("TrackHotMoneyInRealTime() error = %v", err)
	}

	if len(results) != len(stockCodes) {
		t.Errorf("Expected %d results, got %d", len(stockCodes), len(results))
	}

	t.Logf("Concurrent analysis completed in %v", concurrentDuration)
	t.Logf("Analyzed %d stocks concurrently", len(results))

	// Verify all results are valid
	for stockCode, result := range results {
		if result == nil {
			t.Errorf("Expected non-nil result for %s", stockCode)
		}

		if result.StockCode != stockCode {
			t.Errorf("Expected StockCode '%s', got '%s'", stockCode, result.StockCode)
		}
	}
}

func TestHotMoneyTracker_Configuration(t *testing.T) {
	// Test custom configuration
	config := &HotMoneyConfig{
		UpdateInterval:    12 * time.Hour,
		TrendLookback:     3,
		RankingThreshold:  0.9,
		ConsecutiveDays:   5,
	}

	tracker := NewHotMoneyTracker(config)
	ctx := context.Background()

	result, err := tracker.AnalyzeHotMoney(ctx, "sh600000", 5)

	if err != nil {
		t.Fatalf("AnalyzeHotMoney() error = %v", err)
	}

	// Verify configuration is applied
	for _, seat := range result.TopSeats {
		if seat.Ranking < config.RankingThreshold {
			t.Errorf("Expected Ranking >= custom threshold %f, got %f", config.RankingThreshold, seat.Ranking)
		}
	}

	t.Logf("Custom configuration verified:")
	t.Logf("  Ranking Threshold: %.2f", config.RankingThreshold)
	t.Logf("  Consecutive Days: %d", config.ConsecutiveDays)
	t.Logf("  Trend Lookback: %d", config.TrendLookback)

	// Test default configuration
	defaultTracker := NewHotMoneyTracker(nil)
	defaultResult, err := defaultTracker.AnalyzeHotMoney(ctx, "sh600000", 5)

	if err != nil {
		t.Fatalf("Default tracker AnalyzeHotMoney() error = %v", err)
	}

	t.Logf("Default configuration produced %d top seats", len(defaultResult.TopSeats))
}

func TestHotMoneyTracker_EdgeCases(t *testing.T) {
	config := &HotMoneyConfig{
		RankingThreshold: 1.0, // Very high threshold
	}
	tracker := NewHotMoneyTracker(config)
	ctx := context.Background()

	// Test with very high threshold
	result, err := tracker.AnalyzeHotMoney(ctx, "sh600000", 5)

	if err != nil {
		t.Fatalf("AnalyzeHotMoney() error = %v", err)
	}

	// With very high threshold, we should have fewer or no top seats
	t.Logf("High threshold test: %d top seats found", len(result.TopSeats))

	// Test with very low threshold
	lowThresholdConfig := &HotMoneyConfig{
		RankingThreshold: 0.0, // Very low threshold
	}
	lowThresholdTracker := NewHotMoneyTracker(lowThresholdConfig)

	lowThresholdResult, err := lowThresholdTracker.AnalyzeHotMoney(ctx, "sh600000", 5)

	if err != nil {
		t.Fatalf("Low threshold AnalyzeHotMoney() error = %v", err)
	}

	t.Logf("Low threshold test: %d top seats found", len(lowThresholdResult.TopSeats))

	// Test with minimal days
	minimalResult, err := tracker.AnalyzeHotMoney(ctx, "sh600000", 1)

	if err != nil {
		t.Fatalf("Minimal days AnalyzeHotMoney() error = %v", err)
	}

	t.Logf("Minimal days test: %d top seats found", len(minimalResult.TopSeats))
	if len(minimalResult.TopSeats) > 0 {
		for _, seat := range minimalResult.TopSeats {
			if seat.ConsecutiveDays > 1 {
				t.Errorf("Expected ConsecutiveDays <= 1 for 1-day analysis, got %d", seat.ConsecutiveDays)
			}
		}
	}
}