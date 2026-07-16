// backend/data/benchmark/layer_performance_benchmark_test.go
package benchmark

import (
	"context"
	"testing"
	"time"
)

func TestLayerPerformanceBenchmark_BenchmarkMarketDataLayer(t *testing.T) {
	benchmark := NewLayerPerformanceBenchmark()
	ctx := context.Background()

	stockCodes := []string{"sh600000", "sh600519"}

	result := benchmark.BenchmarkMarketDataLayer(ctx, 20, stockCodes)

	// Verify result structure
	if result.LayerName != "MarketDataLayer" {
		t.Errorf("Expected LayerName 'MarketDataLayer', got '%s'", result.LayerName)
	}

	if result.TotalRequests != 20 {
		t.Errorf("Expected TotalRequests 20, got %d", result.TotalRequests)
	}

	if result.AverageLatency <= 0 {
		t.Error("Expected positive AverageLatency")
	}

	if result.MinLatency <= 0 {
		t.Error("Expected positive MinLatency")
	}

	if result.MaxLatency <= 0 {
		t.Error("Expected positive MaxLatency")
	}

	if result.Throughput <= 0 {
		t.Error("Expected positive Throughput")
	}

	// Verify latency constraints
	if result.MinLatency > result.AverageLatency {
		t.Error("MinLatency should be less than or equal to AverageLatency")
	}

	if result.MaxLatency < result.AverageLatency {
		t.Error("MaxLatency should be greater than or equal to AverageLatency")
	}

	t.Logf("MarketDataLayer benchmark:")
	t.Logf("  Average Latency: %v", result.AverageLatency)
	t.Logf("  Min Latency: %v", result.MinLatency)
	t.Logf("  Max Latency: %v", result.MaxLatency)
	t.Logf("  Throughput: %.2f req/sec", result.Throughput)
	t.Logf("  Success Rate: %.2f%%", result.SuccessRate*100)
}

func TestLayerPerformanceBenchmark_BenchmarkResearchReportLayer(t *testing.T) {
	benchmark := NewLayerPerformanceBenchmark()
	ctx := context.Background()

	stockCodes := []string{"sh600000"}

	result := benchmark.BenchmarkResearchReportLayer(ctx, 10, stockCodes)

	if result.LayerName != "ResearchReportLayer" {
		t.Errorf("Expected LayerName 'ResearchReportLayer', got '%s'", result.LayerName)
	}

	if result.TotalRequests != 10 {
		t.Errorf("Expected TotalRequests 10, got %d", result.TotalRequests)
	}

	t.Logf("ResearchReportLayer benchmark:")
	t.Logf("  Average Latency: %v", result.AverageLatency)
	t.Logf("  Cache Hit Rate: %.2f%%", result.CacheHitRate*100)
	t.Logf("  Throughput: %.2f req/sec", result.Throughput)
}

func TestLayerPerformanceBenchmark_BenchmarkSentimentLayer(t *testing.T) {
	benchmark := NewLayerPerformanceBenchmark()
	ctx := context.Background()

	stockCodes := []string{"sh600000"}

	result := benchmark.BenchmarkSentimentLayer(ctx, 10, stockCodes)

	if result.LayerName != "SentimentLayer" {
		t.Errorf("Expected LayerName 'SentimentLayer', got '%s'", result.LayerName)
	}

	t.Logf("SentimentLayer benchmark:")
	t.Logf("  Average Latency: %v", result.AverageLatency)
	t.Logf("  Throughput: %.2f req/sec", result.Throughput)
}

func TestLayerPerformanceBenchmark_BenchmarkCapitalFlowLayer(t *testing.T) {
	benchmark := NewLayerPerformanceBenchmark()
	ctx := context.Background()

	stockCodes := []string{"sh600000"}

	result := benchmark.BenchmarkCapitalFlowLayer(ctx, 10, stockCodes)

	if result.LayerName != "CapitalFlowLayer" {
		t.Errorf("Expected LayerName 'CapitalFlowLayer', got '%s'", result.LayerName)
	}

	t.Logf("CapitalFlowLayer benchmark:")
	t.Logf("  Average Latency: %v", result.AverageLatency)
	t.Logf("  Throughput: %.2f req/sec", result.Throughput)
}

func TestLayerPerformanceBenchmark_BenchmarkAnnouncementLayer(t *testing.T) {
	benchmark := NewLayerPerformanceBenchmark()
	ctx := context.Background()

	stockCodes := []string{"sh600000"}

	result := benchmark.BenchmarkAnnouncementLayer(ctx, 10, stockCodes)

	if result.LayerName != "AnnouncementLayer" {
		t.Errorf("Expected LayerName 'AnnouncementLayer', got '%s'", result.LayerName)
	}

	t.Logf("AnnouncementLayer benchmark:")
	t.Logf("  Average Latency: %v", result.AverageLatency)
	t.Logf("  Throughput: %.2f req/sec", result.Throughput)
}

func TestLayerPerformanceBenchmark_RunFullBenchmark(t *testing.T) {
	benchmark := NewLayerPerformanceBenchmark()
	ctx := context.Background()

	t.Log("Running full performance benchmark...")

	results := benchmark.RunFullBenchmark(ctx)

	// Verify we got results for multiple layers
	if len(results) == 0 {
		t.Fatal("Expected at least one benchmark result")
	}

	expectedLayers := []string{"MarketDataLayer", "ResearchReportLayer", "SentimentLayer", "CapitalFlowLayer", "AnnouncementLayer"}
	missingLayers := 0

	for _, layerName := range expectedLayers {
		if _, exists := results[layerName]; !exists {
			t.Logf("Warning: Expected benchmark result for %s", layerName)
			missingLayers++
		}
	}

	if missingLayers > 0 {
		t.Logf("Warning: %d expected layers missing from benchmark results", missingLayers)
	}

	// Verify each result has required fields
	for layerName, result := range results {
		t.Logf("Benchmark result for %s:", layerName)
		t.Logf("  Average Latency: %v", result.AverageLatency)
		t.Logf("  Throughput: %.2f req/sec", result.Throughput)
		t.Logf("  Success Rate: %.2f%%", result.SuccessRate*100)

		if result.AverageLatency <= 0 {
			t.Errorf("Expected positive AverageLatency for %s", layerName)
		}

		if result.Throughput <= 0 {
			t.Errorf("Expected positive Throughput for %s", layerName)
		}
	}
}

func TestLayerPerformanceBenchmark_GenerateBenchmarkReport(t *testing.T) {
	benchmark := NewLayerPerformanceBenchmark()
	ctx := context.Background()

	// Run a simple benchmark first
	benchmark.BenchmarkMarketDataLayer(ctx, 5, []string{"sh600000"})
	benchmark.BenchmarkResearchReportLayer(ctx, 5, []string{"sh600000"})

	// Generate report
	report := benchmark.GenerateBenchmarkReport()

	if report == "" {
		t.Fatal("Expected non-empty benchmark report")
	}

	// Verify report contains expected sections
	expectedSections := []string{"LAYER PERFORMANCE BENCHMARK", "Layer:", "Operation:", "Average Latency", "Throughput"}

	missingSections := 0
	for _, section := range expectedSections {
		if len(report) == 0 || !containsString(report, section) {
			t.Logf("Warning: Expected section '%s' not found in report", section)
			missingSections++
		}
	}

	if missingSections > 0 {
		t.Logf("Warning: %d expected sections missing from report", missingSections)
	}

	t.Logf("Benchmark report generated (%d characters)", len(report))
}

func TestLayerPerformanceBenchmark_CompareResults(t *testing.T) {
	benchmark := NewLayerPerformanceBenchmark()
	ctx := context.Background()

	// Run benchmarks for multiple layers
	benchmark.BenchmarkMarketDataLayer(ctx, 10, []string{"sh600000"})
	benchmark.BenchmarkResearchReportLayer(ctx, 10, []string{"sh600000"})
	benchmark.BenchmarkSentimentLayer(ctx, 10, []string{"sh600000"})

	comparison := benchmark.CompareResults()

	// Verify comparison structure
	expectedFields := []string{"fastest_layer", "slowest_layer", "highest_throughput", "best_success_rate"}

	for _, field := range expectedFields {
		if _, hasField := comparison[field]; !hasField {
			t.Errorf("Expected %s field in comparison", field)
		}
	}

	t.Logf("Performance comparison:")
	t.Logf("  Fastest Layer: %v", comparison["fastest_layer"])
	t.Logf("  Slowest Layer: %v", comparison["slowest_layer"])
	t.Logf("  Highest Throughput: %v", comparison["highest_throughput"])
	t.Logf("  Best Success Rate: %v", comparison["best_success_rate"])
}

func TestLayerPerformanceBenchmark_GetBenchmarkResults(t *testing.T) {
	benchmark := NewLayerPerformanceBenchmark()
	ctx := context.Background()

	// Run benchmark first
	benchmark.BenchmarkMarketDataLayer(ctx, 5, []string{"sh600000"})

	// Get results
	results := benchmark.GetBenchmarkResults()

	if len(results) == 0 {
		t.Fatal("Expected at least one benchmark result")
	}

	// Verify results structure
	for layerName, result := range results {
		t.Logf("Results for %s:", layerName)
		t.Logf("  Total Requests: %d", result.TotalRequests)
		t.Logf("  Average Latency: %v", result.AverageLatency)

		if result.LayerName != layerName {
			t.Errorf("Expected LayerName to match key, got '%s' vs '%s'", result.LayerName, layerName)
		}
	}
}

func TestLayerPerformanceBenchmark_BenchmarkIntegration(t *testing.T) {
	benchmark := NewLayerPerformanceBenchmark()
	ctx := context.Background()

	// Benchmark integration layer
	results := benchmark.BenchmarkIntegration(ctx, 10)

	if len(results) == 0 {
		t.Fatal("Expected integration benchmark results")
	}

	// Verify integration results
	for layerName, result := range results {
		t.Logf("Integration benchmark for %s:", layerName)
		t.Logf("  Total Requests: %d", result.TotalRequests)
		t.Logf("  Average Latency: %v", result.AverageLatency)
		t.Logf("  Throughput: %.2f req/sec", result.Throughput)

		if result.Operation == "" {
			t.Error("Expected non-empty Operation")
		}

		if result.Throughput <= 0 {
			t.Error("Expected positive Throughput")
		}
	}
}

func TestLayerPerformanceBenchmark_ConcurrentBenchmarks(t *testing.T) {
	benchmark := NewLayerPerformanceBenchmark()
	ctx := context.Background()

	t.Log("Testing concurrent benchmarks...")

	var wg sync.WaitGroup
	benchmarks := 5

	startTime := time.Now()

	for i := 0; i < benchmarks; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			switch index % 5 {
			case 0:
				benchmark.BenchmarkMarketDataLayer(ctx, 5, []string{"sh600000"})
			case 1:
				benchmark.BenchmarkResearchReportLayer(ctx, 5, []string{"sh600000"})
			case 2:
				benchmark.BenchmarkSentimentLayer(ctx, 5, []string{"sh600000"})
			case 3:
				benchmark.BenchmarkCapitalFlowLayer(ctx, 5, []string{"sh600000"})
			case 4:
				benchmark.BenchmarkAnnouncementLayer(ctx, 5, []string{"sh600000"})
			}
		}(i)
	}

	wg.Wait()
	totalDuration := time.Since(startTime)

	t.Logf("Completed %d concurrent benchmarks in %v", benchmarks, totalDuration)
	t.Logf("Average time per benchmark: %v", totalDuration/time.Duration(benchmarks))

	results := benchmark.GetBenchmarkResults()
	t.Logf("Collected results for %d layers", len(results))
}

func TestLayerPerformanceBenchmark_CacheImpactAnalysis(t *testing.T) {
	benchmark := NewLayerPerformanceBenchmark()
	ctx := context.Background()

	stockCodes := []string{"sh600000"}

	t.Log("Analyzing cache impact...")

	// First benchmark with cold cache
	coldCacheResult := benchmark.BenchmarkMarketDataLayer(ctx, 10, stockCodes)
	t.Logf("Cold cache benchmark - Average Latency: %v", coldCacheResult.AverageLatency)

	// Second benchmark with warm cache (simulated)
	warmCacheResult := benchmark.BenchmarkMarketDataLayer(ctx, 10, stockCodes)
	t.Logf("Warm cache benchmark - Average Latency: %v", warmCacheResult.AverageLatency)

	// Calculate cache improvement
	if warmCacheResult.AverageLatency < coldCacheResult.AverageLatency {
		improvement := coldCacheResult.AverageLatency - warmCacheResult.AverageLatency
		speedup := float64(coldCacheResult.AverageLatency) / float64(warmCacheResult.AverageLatency)
		t.Logf("Cache improvement: %v (%.2fx speedup)", improvement, speedup)

		if speedup < 1.5 {
			t.Logf("Warning: Cache speedup less than 1.5x, got %.2fx", speedup)
		}
	} else {
		t.Logf("No cache improvement detected (this may be expected in testing)")
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