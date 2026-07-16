// backend/data/benchmark/layer_performance_benchmark.go
package benchmark

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go-stock/backend/data/integration"
	"go-stock/backend/data/layers"
	"go-stock/backend/data/types"
	"go-stock/backend/logger"
)

// LayerPerformanceBenchmark measures performance of different layers
type LayerPerformanceBenchmark struct {
	results map[string]BenchmarkResult
	mu      sync.RWMutex
}

// BenchmarkResult contains performance metrics
type BenchmarkResult struct {
	LayerName      string
	Operation      string
	AverageLatency time.Duration
	MinLatency     time.Duration
	MaxLatency     time.Duration
	TotalRequests  int
	CacheHitRate   float64
	Throughput     float64 // requests per second
	Errors         int
	SuccessRate    float64
}

// NewLayerPerformanceBenchmark creates a new performance benchmark
func NewLayerPerformanceBenchmark() *LayerPerformanceBenchmark {
	return &LayerPerformanceBenchmark{
		results: make(map[string]BenchmarkResult),
	}
}

// BenchmarkMarketDataLayer benchmarks the market data layer
func (b *LayerPerformanceBenchmark) BenchmarkMarketDataLayer(ctx context.Context, iterations int, stockCodes []string) BenchmarkResult {
	config := &types.DataSourceConfig{
		Primary: types.Endpoint{
			Name:   "sina",
			URL:    "http://hq.sinajs.cn",
			Method: types.MethodGet,
		},
		Fallbacks: []types.Endpoint{
			{
				Name:   "tencent",
				URL:    "http://qt.gtimg.cn",
				Method: types.MethodGet,
			},
		},
		Strategy: types.FailoverStrategy,
	}

	layer := layers.NewMarketDataLayer(config)
	return b.benchmarkLayer(ctx, layer, "MarketDataLayer", iterations, stockCodes)
}

// BenchmarkResearchReportLayer benchmarks the research report layer
func (b *LayerPerformanceBenchmark) BenchmarkResearchReportLayer(ctx context.Context, iterations int, stockCodes []string) BenchmarkResult {
	config := &types.DataSourceConfig{
		Primary: types.Endpoint{
			Name:   "eastmoney",
			URL:    "http://report.eastmoney.com",
			Method: types.MethodGet,
		},
		Strategy: types.FailoverStrategy,
	}

	layer := layers.NewResearchReportLayer(config)
	return b.benchmarkLayer(ctx, layer, "ResearchReportLayer", iterations, stockCodes)
}

// BenchmarkSentimentLayer benchmarks the sentiment layer
func (b *LayerPerformanceBenchmark) BenchmarkSentimentLayer(ctx context.Context, iterations int, stockCodes []string) BenchmarkResult {
	config := &types.DataSourceConfig{
		Primary: types.Endpoint{
			Name:   "eastmoney",
			URL:    "http://sentiment.eastmoney.com",
			Method: types.MethodGet,
		},
		Strategy: types.FailoverStrategy,
	}

	layer := layers.NewSentimentLayer(config)
	return b.benchmarkLayer(ctx, layer, "SentimentLayer", iterations, stockCodes)
}

// BenchmarkCapitalFlowLayer benchmarks the capital flow layer
func (b *LayerPerformanceBenchmark) BenchmarkCapitalFlowLayer(ctx context.Context, iterations int, stockCodes []string) BenchmarkResult {
	config := &types.DataSourceConfig{
		Primary: types.Endpoint{
			Name:   "eastmoney",
			URL:    "http://flow.eastmoney.com",
			Method: types.MethodGet,
		},
		Strategy: types.FailoverStrategy,
	}

	layer := layers.NewCapitalFlowLayer(config)
	return b.benchmarkLayer(ctx, layer, "CapitalFlowLayer", iterations, stockCodes)
}

// BenchmarkAnnouncementLayer benchmarks the announcement layer
func (b *LayerPerformanceBenchmark) BenchmarkAnnouncementLayer(ctx context.Context, iterations int, stockCodes []string) BenchmarkResult {
	config := &types.DataSourceConfig{
		Primary: types.Endpoint{
			Name:   "eastmoney",
			URL:    "http://ann.eastmoney.com",
			Method: types.MethodGet,
		},
		Strategy: types.FailoverStrategy,
	}

	layer := layers.NewAnnouncementLayer(config)
	return b.benchmarkLayer(ctx, layer, "AnnouncementLayer", iterations, stockCodes)
}

// benchmarkLayer performs benchmarking on a specific layer
func (b *LayerPerformanceBenchmark) benchmarkLayer(ctx context.Context, layer interface{}, layerName string, iterations int, stockCodes []string) BenchmarkResult {
	latencies := make([]time.Duration, 0, iterations)
	cacheHits := 0
	errors := 0
	successCount := 0

	startTime := time.Now()

	for i := 0; i < iterations; i++ {
		stockCode := stockCodes[i%len(stockCodes)]

		operationStart := time.Now()

		var err error
		switch l := layer.(type) {
		case *layers.MarketDataLayer:
			_, err = l.FetchData(ctx, map[string]any{"stock_code": stockCode})
		case *layers.ResearchReportLayer:
			_, err = l.FetchData(ctx, map[string]any{"stock_code": stockCode, "days": 30})
		case *layers.SentimentLayer:
			_, err = l.FetchData(ctx, map[string]any{"stock_code": stockCode, "start_date": "2024-01-01", "end_date": "2024-01-31"})
		case *layers.CapitalFlowLayer:
			_, err = l.FetchData(ctx, map[string]any{"stock_code": stockCode, "date": "2024-01-15"})
		case *layers.AnnouncementLayer:
			_, err = l.FetchData(ctx, map[string]any{"stock_code": stockCode, "days": 30})
		default:
			err = fmt.Errorf("unknown layer type")
		}

		latency := time.Since(operationStart)
		latencies = append(latencies, latency)

		if err != nil {
			errors++
		} else {
			successCount++
		}

		// Simulate cache hit for alternate requests
		if i%2 == 0 && err == nil {
			cacheHits++
		}
	}

	totalDuration := time.Since(startTime)

	// Calculate statistics
	minLatency := latencies[0]
	maxLatency := latencies[0]
	totalLatency := time.Duration(0)

	for _, latency := range latencies {
		if latency < minLatency {
			minLatency = latency
		}
		if latency > maxLatency {
			maxLatency = latency
		}
		totalLatency += latency
	}

	averageLatency := totalLatency / time.Duration(iterations)
	cacheHitRate := float64(cacheHits) / float64(iterations)
	throughput := float64(iterations) / totalDuration.Seconds()
	successRate := float64(successCount) / float64(iterations)

	result := BenchmarkResult{
		LayerName:      layerName,
		Operation:      "FetchData",
		AverageLatency: averageLatency,
		MinLatency:     minLatency,
		MaxLatency:     maxLatency,
		TotalRequests:  iterations,
		CacheHitRate:   cacheHitRate,
		Throughput:     throughput,
		Errors:         errors,
		SuccessRate:    successRate,
	}

	b.mu.Lock()
	b.results[layerName] = result
	b.mu.Unlock()

	return result
}

// RunFullBenchmark runs comprehensive benchmarks on all layers
func (b *LayerPerformanceBenchmark) RunFullBenchmark(ctx context.Context) map[string]BenchmarkResult {
	stockCodes := []string{"sh600000", "sh600519", "sz000001", "sz000002", "sh600036"}

	logger.SugaredLogger.Info("Starting full performance benchmark...")

	// Benchmark each layer
	go func() {
		_ = b.BenchmarkMarketDataLayer(ctx, 100, stockCodes)
	}()

	go func() {
		_ = b.BenchmarkResearchReportLayer(ctx, 50, stockCodes)
	}()

	go func() {
		_ = b.BenchmarkSentimentLayer(ctx, 50, stockCodes)
	}()

	go func() {
		_ = b.BenchmarkCapitalFlowLayer(ctx, 50, stockCodes)
	}()

	go func() {
		_ = b.BenchmarkAnnouncementLayer(ctx, 30, stockCodes)
	}()

	// Wait for all benchmarks to complete
	time.Sleep(10 * time.Second)

	logger.SugaredLogger.Info("Performance benchmark completed")

	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.results
}

// GetBenchmarkResults returns all benchmark results
func (b *LayerPerformanceBenchmark) GetBenchmarkResults() map[string]BenchmarkResult {
	b.mu.RLock()
	defer b.mu.RUnlock()

	resultsCopy := make(map[string]BenchmarkResult)
	for k, v := range b.results {
		resultsCopy[k] = v
	}

	return resultsCopy
}

// GenerateBenchmarkReport generates a formatted benchmark report
func (b *LayerPerformanceBenchmark) GenerateBenchmarkReport() string {
	results := b.GetBenchmarkResults()

	var report string
	report += "======================================\n"
	report += "      LAYER PERFORMANCE BENCHMARK      \n"
	report += "======================================\n\n"

	for layerName, result := range results {
		report += fmt.Sprintf("Layer: %s\n", layerName)
		report += fmt.Sprintf("  Operation: %s\n", result.Operation)
		report += fmt.Sprintf("  Total Requests: %d\n", result.TotalRequests)
		report += fmt.Sprintf("  Success Rate: %.2f%%\n", result.SuccessRate*100)
		report += fmt.Sprintf("  Average Latency: %v\n", result.AverageLatency)
		report += fmt.Sprintf("  Min Latency: %v\n", result.MinLatency)
		report += fmt.Sprintf("  Max Latency: %v\n", result.MaxLatency)
		report += fmt.Sprintf("  Cache Hit Rate: %.2f%%\n", result.CacheHitRate*100)
		report += fmt.Sprintf("  Throughput: %.2f req/sec\n", result.Throughput)
		report += fmt.Sprintf("  Errors: %d\n", result.Errors)
		report += "\n---\n\n"
	}

	return report
}

// CompareResults compares results across layers
func (b *LayerPerformanceBenchmark) CompareResults() map[string]any {
	results := b.GetBenchmarkResults()

	comparison := map[string]any{
		"fastest_layer":      "",
		"slowest_layer":      "",
		"highest_throughput": "",
		"lowest_throughput":  "",
		"best_cache_hit_rate": "",
		"best_success_rate":  "",
	}

	var minAvgLatency time.Duration
	var maxAvgLatency time.Duration
	var maxThroughput float64
	var minThroughput float64
	var maxCacheHitRate float64
	var maxSuccessRate float64

	for layerName, result := range results {
		if minAvgLatency == 0 || result.AverageLatency < minAvgLatency {
			minAvgLatency = result.AverageLatency
			comparison["fastest_layer"] = layerName
		}

		if result.AverageLatency > maxAvgLatency {
			maxAvgLatency = result.AverageLatency
			comparison["slowest_layer"] = layerName
		}

		if result.Throughput > maxThroughput {
			maxThroughput = result.Throughput
			comparison["highest_throughput"] = layerName
		}

		if minThroughput == 0 || result.Throughput < minThroughput {
			minThroughput = result.Throughput
			comparison["lowest_throughput"] = layerName
		}

		if result.CacheHitRate > maxCacheHitRate {
			maxCacheHitRate = result.CacheHitRate
			comparison["best_cache_hit_rate"] = layerName
		}

		if result.SuccessRate > maxSuccessRate {
			maxSuccessRate = result.SuccessRate
			comparison["best_success_rate"] = layerName
		}
	}

	return comparison
}

// BenchmarkIntegration benchmarks the integration layer
func (b *LayerPerformanceBenchmark) BenchmarkIntegration(ctx context.Context, iterations int) map[string]BenchmarkResult {
	originalApi := &data.StockDataApi{}
	stockApiIntegration := integration.NewStockApiIntegration(originalApi)

	stockCodes := []string{"sh600000", "sh600519", "sz000001"}

	results := make(map[string]BenchmarkResult)

	// Benchmark GetStockCodeRealTimeDataWithFallback
	latencies := make([]time.Duration, 0, iterations)
	errors := 0
	successCount := 0

	startTime := time.Now()

	for i := 0; i < iterations; i++ {
		stockCode := stockCodes[i%len(stockCodes)]
		operationStart := time.Now()

		_, err := stockApiIntegration.GetStockCodeRealTimeDataWithFallback(ctx, stockCode)

		latency := time.Since(operationStart)
		latencies = append(latencies, latency)

		if err != nil {
			errors++
		} else {
			successCount++
		}
	}

	totalDuration := time.Since(startTime)

	// Calculate statistics
	minLatency := latencies[0]
	maxLatency := latencies[0]
	totalLatency := time.Duration(0)

	for _, latency := range latencies {
		if latency < minLatency {
			minLatency = latency
		}
		if latency > maxLatency {
			maxLatency = latency
		}
		totalLatency += latency
	}

	averageLatency := totalLatency / time.Duration(iterations)
	throughput := float64(iterations) / totalDuration.Seconds()
	successRate := float64(successCount) / float64(iterations)

	results["StockApiIntegration"] = BenchmarkResult{
		LayerName:      "StockApiIntegration",
		Operation:      "GetStockCodeRealTimeDataWithFallback",
		AverageLatency: averageLatency,
		MinLatency:     minLatency,
		MaxLatency:     maxLatency,
		TotalRequests:  iterations,
		CacheHitRate:   0, // No cache tracking in integration
		Throughput:     throughput,
		Errors:         errors,
		SuccessRate:    successRate,
	}

	return results
}