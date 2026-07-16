// backend/data/trading/limit_up_analyzer_test.go
package trading

import (
	"context"
	"testing"
	"time"
)

func TestLimitUpAnalyzer_AnalyzeLimitUpPool(t *testing.T) {
	config := &LimitUpConfig{
		RefreshInterval: 1 * time.Hour,
		HistoryLookback: 10,
		PoolSize:        50,
		ExplosionRate:   0.8,
		FirstBoardRatio: 0.6,
		SectorThreshold: 3,
	}

	analyzer := NewLimitUpAnalyzer(config)
	ctx := context.Background()

	result, err := analyzer.AnalyzeLimitUpPool(ctx)

	if err != nil {
		t.Fatalf("AnalyzeLimitUpPool() error = %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Verify pool statistics
	poolStats := result.PoolStatistics
	if poolStats == nil {
		t.Fatal("Expected non-nil PoolStatistics")
	}

	if poolStats.TotalCount <= 0 {
		t.Errorf("Expected positive TotalCount, got %d", poolStats.TotalCount)
	}

	if poolStats.ExplosionRate < 0 || poolStats.ExplosionRate > 1 {
		t.Errorf("Expected ExplosionRate between 0 and 1, got %f", poolStats.ExplosionRate)
	}

	if poolStats.FirstBoardRatio < 0 || poolStats.FirstBoardRatio > 1 {
		t.Errorf("Expected FirstBoardRatio between 0 and 1, got %f", poolStats.FirstBoardRatio)
	}

	if len(poolStats.TopLimitUpStocks) == 0 {
		t.Error("Expected at least one top limit-up stock")
	}

	// Verify sector analysis
	if len(result.SectorAnalysis) == 0 {
		t.Log("Warning: No sector analysis results")
	}

	// Verify market trend
	validTrends := map[string]bool{
		"强势市场": true,
		"活跃市场": true,
		"轮动市场": true,
		"分化市场": true,
		"弱市场": true,
		"低迷市场": true,
		"无涨停板": true,
	}

	if !validTrends[result.MarketTrend] {
		t.Errorf("Expected valid market trend, got '%s'", result.MarketTrend)
	}

	// Verify risk assessment
	validRisks := map[string]bool{
		"高风险": true,
		"中风险": true,
		"低风险": true,
	}

	if !validRisks[result.RiskAssessment] {
		t.Errorf("Expected valid risk assessment, got '%s'", result.RiskAssessment)
	}

	if len(result.Recommendations) == 0 {
		t.Error("Expected at least one recommendation")
	}

	t.Logf("Limit-up pool analysis completed:")
	t.Logf("  Total Count: %d", poolStats.TotalCount)
	t.Logf("  Explosion Rate: %.2f%%", poolStats.ExplosionRate*100)
	t.Logf("  First Board Ratio: %.2f%%", poolStats.FirstBoardRatio*100)
	t.Logf("  Market Trend: %s", result.MarketTrend)
	t.Logf("  Risk Assessment: %s", result.RiskAssessment)
}

func TestLimitUpAnalyzer_GetStockLimitUpInfo(t *testing.T) {
	analyzer := NewLimitUpAnalyzer(nil)
	ctx := context.Background()

	// First analyze to populate cache
	_, err := analyzer.AnalyzeLimitUpPool(ctx)
	if err != nil {
		t.Fatalf("AnalyzeLimitUpPool() error = %v", err)
	}

	// Test getting specific stock info
	stockInfo, err := analyzer.GetStockLimitUpInfo("sh600000")

	if err != nil {
		t.Fatalf("GetStockLimitUpInfo() error = %v", err)
	}

	if stockInfo == nil {
		t.Fatal("Expected non-nil stock info")
	}

	// Verify stock info structure
	if stockInfo.StockCode != "sh600000" {
		t.Errorf("Expected StockCode 'sh600000', got '%s'", stockInfo.StockCode)
	}

	if stockInfo.StockName == "" {
		t.Error("Expected non-empty StockName")
	}

	if stockInfo.LimitUpPrice <= 0 {
		t.Errorf("Expected positive LimitUpPrice, got %f", stockInfo.LimitUpPrice)
	}

	if stockInfo.ChangePercent != 10.00 {
		t.Errorf("Expected ChangePercent 10.00, got %f", stockInfo.ChangePercent)
	}

	if stockInfo.Board == "" {
		t.Error("Expected non-empty Board")
	}

	if stockInfo.Sector == "" {
		t.Error("Expected non-empty Sector")
	}

	t.Logf("Stock limit-up info verified:")
	t.Logf("  Code: %s", stockInfo.StockCode)
	t.Logf("  Name: %s", stockInfo.StockName)
	t.Logf("  Price: %.2f", stockInfo.LimitUpPrice)
	t.Logf("  Board: %s", stockInfo.Board)
	t.Logf("  Sector: %s", stockInfo.Sector)
}

func TestLimitUpAnalyzer_GetSectorLimitUpAnalysis(t *testing.T) {
	analyzer := NewLimitUpAnalyzer(nil)
	ctx := context.Background()

	// Test bank sector analysis
	analysis, err := analyzer.GetSectorLimitUpAnalysis(ctx, "银行")

	if err != nil {
		t.Fatalf("GetSectorLimitUpAnalysis() error = %v", err)
	}

	if analysis == nil {
		t.Fatal("Expected non-nil sector analysis")
	}

	// Verify sector analysis structure
	if analysis.SectorName != "银行" {
		t.Errorf("Expected SectorName '银行', got '%s'", analysis.SectorName)
	}

	if analysis.TotalCount <= 0 {
		t.Errorf("Expected positive TotalCount, got %d", analysis.TotalCount)
	}

	if analysis.ExplosionRate < 0 || analysis.ExplosionRate > 1 {
		t.Errorf("Expected ExplosionRate between 0 and 1, got %f", analysis.ExplosionRate)
	}

	if len(analysis.TopStocks) == 0 {
		t.Error("Expected at least one top stock")
	}

	// Verify explosion count doesn't exceed total
	if analysis.ExplosionCount > analysis.TotalCount {
		t.Errorf("Expected ExplosionCount <= TotalCount, got %d > %d", analysis.ExplosionCount, analysis.TotalCount)
	}

	t.Logf("Sector analysis verified:")
	t.Logf("  Sector: %s", analysis.SectorName)
	t.Logf("  Total Count: %d", analysis.TotalCount)
	t.Logf("  Explosion Rate: %.2f%%", analysis.ExplosionRate*100)
}

func TestLimitUpAnalyzer_GetExplosionRankings(t *testing.T) {
	analyzer := NewLimitUpAnalyzer(nil)
	ctx := context.Background()

	rankings, err := analyzer.GetExplosionRankings(ctx)

	if err != nil {
		t.Fatalf("GetExplosionRankings() error = %v", err)
	}

	if len(rankings) == 0 {
		t.Fatal("Expected at least one explosion ranking")
	}

	// Verify rankings are sorted by score
	for i := 1; i < len(rankings); i++ {
		if rankings[i].Score > rankings[i-1].Score {
			t.Errorf("Rankings should be sorted by Score descending, but found higher score at position %d", i)
		}
	}

	// Verify ranking assignment
	for i, ranking := range rankings {
		if ranking.Ranking != i+1 {
			t.Errorf("Expected Ranking %d, got %d", i+1, ranking.Ranking)
		}

		if ranking.StockCode == "" {
			t.Errorf("Expected non-empty StockCode at position %d", i)
		}

		if ranking.Score < 0 || ranking.Score > 100 {
			t.Errorf("Expected Score between 0 and 100 at position %d, got %f", i, ranking.Score)
		}

		t.Logf("Ranking %d: %s (%.2f)", ranking.Ranking, ranking.StockName, ranking.Score)
	}

	t.Logf("Successfully generated %d explosion rankings", len(rankings))
}

func TestLimitUpAnalyzer_MonitorLimitUpPool(t *testing.T) {
	analyzer := NewLimitUpAnalyzer(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultChan, err := analyzer.MonitorLimitUpPool(ctx, 1*time.Second)

	if err != nil {
		t.Fatalf("MonitorLimitUpPool() error = %v", err)
	}

	if resultChan == nil {
		t.Fatal("Expected non-nil result channel")
	}

	// Receive first result
	select {
	case result := <-resultChan:
		if result == nil {
			t.Fatal("Expected non-nil result from monitoring channel")
		}

		t.Logf("Received monitoring result:")
		t.Logf("  Total Count: %d", result.PoolStatistics.TotalCount)
		t.Logf("  Market Trend: %s", result.MarketTrend)
		t.Logf("  Risk Assessment: %s", result.RiskAssessment)

	case <-ctx.Done():
		t.Fatal("Timeout waiting for monitoring result")
	}

	// Verify channel is still receiving
	select {
	case result := <-resultChan:
		if result == nil {
			t.Fatal("Expected non-nil result from second monitoring")
		}

		t.Logf("Received second monitoring result successfully")

	case <-time.After(2 * time.Second):
		t.Log("Warning: No second result received within timeout")
	}
}

func TestLimitUpAnalyzer_PoolStatisticsCalculation(t *testing.T) {
	analyzer := NewLimitUpAnalyzer(nil)
	ctx := context.Background()

	result, err := analyzer.AnalyzeLimitUpPool(ctx)

	if err != nil {
		t.Fatalf("AnalyzeLimitUpPool() error = %v", err)
	}

	poolStats := result.PoolStatistics

	// Verify statistics consistency
	if poolStats.FirstBoardCount + poolStats.ContinueBoardCount != poolStats.TotalCount {
		t.Logf("Warning: Board count mismatch: %d + %d != %d",
			poolStats.FirstBoardCount, poolStats.ContinueBoardCount, poolStats.TotalCount)
	}

	// Verify sector distribution sum
	sectorSum := 0
	for _, count := range poolStats.SectorDistribution {
		sectorSum += count
	}

	if sectorSum > poolStats.TotalCount {
		t.Errorf("Sector distribution sum (%d) exceeds total count (%d)", sectorSum, poolStats.TotalCount)
	}

	// Verify average turnover is reasonable
	if poolStats.AverageTurnover < 0 || poolStats.AverageTurnover > 100 {
		t.Errorf("Expected AverageTurnover between 0 and 100, got %f", poolStats.AverageTurnover)
	}

	// Verify top stocks are within total count
	if len(poolStats.TopLimitUpStocks) > poolStats.TotalCount {
		t.Errorf("Top stocks count (%d) exceeds total count (%d)",
			len(poolStats.TopLimitUpStocks), poolStats.TotalCount)
	}

	t.Logf("Pool statistics verified:")
	t.Logf("  Total Count: %d", poolStats.TotalCount)
	t.Logf("  First Board: %d", poolStats.FirstBoardCount)
	t.Logf("  Continue Board: %d", poolStats.ContinueBoardCount)
	t.Logf("  Average Turnover: %.2f%%", poolStats.AverageTurnover)
}

func TestLimitUpAnalyzer_SectorAnalysisLogic(t *testing.T) {
	config := &LimitUpConfig{
		SectorThreshold: 1, // Lower threshold for testing
	}
	analyzer := NewLimitUpAnalyzer(config)
	ctx := context.Background()

	result, err := analyzer.AnalyzeLimitUpPool(ctx)

	if err != nil {
		t.Fatalf("AnalyzeLimitUpPool() error = %v", err)
	}

	// Verify sector analysis structure
	for sectorName, analysis := range result.SectorAnalysis {
		t.Logf("Sector: %s", sectorName)

		if analysis.SectorName != sectorName {
			t.Errorf("Expected SectorName to match key, got '%s' vs '%s'", analysis.SectorName, sectorName)
		}

		if analysis.TotalCount <= 0 {
			t.Errorf("Expected positive TotalCount for sector %s, got %d", sectorName, analysis.TotalCount)
		}

		if analysis.ExplosionRate < 0 || analysis.ExplosionRate > 1 {
			t.Errorf("Expected ExplosionRate between 0 and 1 for sector %s, got %f", sectorName, analysis.ExplosionRate)
		}

		if len(analysis.TopStocks) == 0 {
			t.Errorf("Expected at least one top stock for sector %s", sectorName)
		}

		// Verify sector counts consistency
		if analysis.FirstBoardCount + analysis.ContinueBoardCount > analysis.TotalCount {
			t.Errorf("Board counts exceed total for sector %s", sectorName)
		}
	}
}

func TestLimitUpAnalyzer_MarketTrendDetermination(t *testing.T) {
	analyzer := NewLimitUpAnalyzer(nil)
	ctx := context.Background()

	result, err := analyzer.AnalyzeLimitUpPool(ctx)

	if err != nil {
		t.Fatalf("AnalyzeLimitUpPool() error = %v", err)
	}

	poolStats := result.PoolStatistics

	// Verify market trend logic
	t.Logf("Market Trend: %s", result.MarketTrend)
	t.Logf("  Explosion Rate: %.2f%%", poolStats.ExplosionRate*100)
	t.Logf("  First Board Ratio: %.2f%%", poolStats.FirstBoardRatio*100)

	// Verify trend matches expectations based on statistics
	if result.MarketTrend == "强势市场" {
		if poolStats.ExplosionRate < 0.8 || poolStats.FirstBoardRatio < 0.6 {
			t.Logf("Warning: '强势市场' with lower stats - Explosion: %.2f, FirstBoard: %.2f",
				poolStats.ExplosionRate, poolStats.FirstBoardRatio)
		}
	}

	if result.MarketTrend == "低迷市场" {
		if poolStats.TotalCount > 10 {
			t.Logf("Warning: '低迷市场' with significant count (%d)", poolStats.TotalCount)
		}
	}
}

func TestLimitUpAnalyzer_RiskAssessment(t *testing.T) {
	analyzer := NewLimitUpAnalyzer(nil)
	ctx := context.Background()

	result, err := analyzer.AnalyzeLimitUpPool(ctx)

	if err != nil {
		t.Fatalf("AnalyzeLimitUpPool() error = %v", err)
	}

	// Verify risk assessment is consistent with market trend
	t.Logf("Risk Assessment: %s", result.RiskAssessment)
	t.Logf("  Market Trend: %s", result.MarketTrend)

	if result.MarketTrend == "强势市场" && result.RiskAssessment == "低风险" {
		// This is possible but notable
		t.Logf("Note: 强势市场 with low risk assessment")
	}

	if result.MarketTrend == "低迷市场" && result.RiskAssessment == "高风险" {
		t.Logf("Note: 低迷市场 with high risk assessment")
	}

	// Verify high continue board count increases risk
	highContinueCount := 0
	for _, stock := range result.PoolStatistics.TopLimitUpStocks {
		if stock.Board == "连板" && stock.ContinueDays >= 3 {
			highContinueCount++
		}
	}

	if highContinueCount > 3 && result.RiskAssessment != "高风险" {
		t.Logf("Warning: High continue board count (%d) but risk is '%s'", highContinueCount, result.RiskAssessment)
	}
}

func TestLimitUpAnalyzer_Recommendations(t *testing.T) {
	analyzer := NewLimitUpAnalyzer(nil)
	ctx := context.Background()

	result, err := analyzer.AnalyzeLimitUpPool(ctx)

	if err != nil {
		t.Fatalf("AnalyzeLimitUpPool() error = %v", err)
	}

	// Verify recommendations exist and are relevant
	if len(result.Recommendations) == 0 {
		t.Fatal("Expected at least one recommendation")
	}

	t.Logf("Recommendations:")
	for i, rec := range result.Recommendations {
		if rec == "" {
			t.Errorf("Expected non-empty recommendation at position %d", i)
		}

		t.Logf("  %d. %s", i+1, rec)
	}

	// Verify recommendations are contextually relevant
	if result.RiskAssessment == "高风险" {
		hasRiskWarning := false
		for _, rec := range result.Recommendations {
			if containsSubstring(rec, "风险") || containsSubstring(rec, "仓位") {
				hasRiskWarning = true
				break
			}
		}

		if !hasRiskWarning {
			t.Log("Warning: High risk assessment but no risk management recommendations")
		}
	}
}

func TestLimitUpAnalyzer_ExplosionScoreCalculation(t *testing.T) {
	analyzer := NewLimitUpAnalyzer(nil)
	ctx := context.Background()

	rankings, err := analyzer.GetExplosionRankings(ctx)

	if err != nil {
		t.Fatalf("GetExplosionRankings() error = %v", err)
	}

	for _, ranking := range rankings {
		// Verify score components are reflected in final score
		expectedMinScore := ranking.ExplosionRate * 30 // Minimum from explosion rate

		if ranking.Score < expectedMinScore {
			t.Errorf("Expected Score >= %.2f (from explosion rate), got %.2f",
				expectedMinScore, ranking.Score)
		}

		// High turnover stocks should have higher scores
		if ranking.TurnoverRate > 15 && ranking.Score < 40 {
			t.Logf("Warning: High turnover (%.2f%%) but low score (%.2f)",
				ranking.TurnoverRate, ranking.Score)
		}

		// Continue board stocks should have bonus points
		if ranking.Board == "连板" && ranking.Score < 50 {
			t.Logf("Warning: Continue board but score (%.2f) seems low", ranking.Score)
		}

		t.Logf("Score breakdown for %s: %.2f", ranking.StockName, ranking.Score)
	}
}

func TestLimitUpAnalyzer_Configuration(t *testing.T) {
	// Test custom configuration
	config := &LimitUpConfig{
		RefreshInterval:  30 * time.Minute,
		HistoryLookback:  5,
		PoolSize:         30,
		ExplosionRate:    0.9,
		FirstBoardRatio:  0.7,
		SectorThreshold:  5,
	}

	analyzer := NewLimitUpAnalyzer(config)
	ctx := context.Background()

	result, err := analyzer.AnalyzeLimitUpPool(ctx)

	if err != nil {
		t.Fatalf("AnalyzeLimitUpPool() error = %v", err)
	}

	// Verify configuration is applied
	if result.PoolStatistics.ExplosionRate < 0.9 && result.MarketTrend == "强势市场" {
		t.Logf("Warning: High threshold (0.9) but marked as 强势市场 with rate %.2f",
			result.PoolStatistics.ExplosionRate)
	}

	// Lower sector threshold should produce more sector analysis
	t.Logf("Sector analysis count with threshold %d: %d",
		config.SectorThreshold, len(result.SectorAnalysis))

	// Test default configuration
	defaultAnalyzer := NewLimitUpAnalyzer(nil)
	defaultResult, err := defaultAnalyzer.AnalyzeLimitUpPool(ctx)

	if err != nil {
		t.Fatalf("Default analyzer AnalyzeLimitUpPool() error = %v", err)
	}

	t.Logf("Default configuration sector analysis count: %d", len(defaultResult.SectorAnalysis))
}

func TestLimitUpAnalyzer_EdgeCases(t *testing.T) {
	ctx := context.Background()

	// Test with non-existent sector
	analyzer := NewLimitUpAnalyzer(nil)
	_, err := analyzer.GetSectorLimitUpAnalysis(ctx, "不存在的板块")

	if err == nil {
		t.Error("Expected error for non-existent sector")
	}

	// Test with non-existent stock
	_, err = analyzer.GetStockLimitUpInfo("INVALID_CODE")

	if err == nil {
		t.Error("Expected error for non-existent stock code")
	}

	// Test monitoring with very short interval
	analyzer = NewLimitUpAnalyzer(nil)
	shortCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = analyzer.MonitorLimitUpPool(shortCtx, 50*time.Millisecond)

	if err != nil {
		t.Errorf("Monitoring with short interval failed: %v", err)
	}

	t.Log("Edge cases handled correctly")
}

// Helper function
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}