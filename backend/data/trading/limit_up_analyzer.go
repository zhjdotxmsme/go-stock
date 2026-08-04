// backend/data/trading/limit_up_analyzer.go
package trading

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"go-stock/backend/logger"
)

// LimitUpAnalyzer analyzes limit-up stocks and market patterns
type LimitUpAnalyzer struct {
	config *LimitUpConfig
	cache  *LimitUpCache
}

// LimitUpConfig configures limit-up analysis behavior
type LimitUpConfig struct {
	RefreshInterval  time.Duration
	HistoryLookback  int
	PoolSize         int
	ExplosionRate    float64
	FirstBoardRatio  float64
	SectorThreshold  int
}

// LimitUpCache maintains limit-up data cache
type LimitUpCache struct {
	data      map[string]*LimitUpStockData
	mu        sync.RWMutex
	lastUpdate time.Time
}

// LimitUpStockData represents a stock's limit-up information
type LimitUpStockData struct {
	StockCode        string    `json:"stock_code"`
	StockName        string    `json:"stock_name"`
	LimitUpTime      time.Time `json:"limit_up_time"`
	LimitUpPrice     float64   `json:"limit_up_price"`
	CurrentPrice     float64   `json:"current_price"`
	ChangePercent    float64   `json:"change_percent"`
	Volume           float64   `json:"volume"`
	Amount           float64   `json:"amount"`
	Board            string    `json:"board"`           // 首板/连板/炸板
	ContinueDays     int       `json:"continue_days"`   // 连板天数
	FirstBoardCount  int       `json:"first_board_count"` // 首板数量
	Sector           string    `json:"sector"`
	IsExplosion      bool      `json:"is_explosion"`
	ExplosionRate    float64   `json:"explosion_rate"`
	TurnoverRate     float64   `json:"turnover_rate"`
	LimitUpOpenCount int       `json:"limit_up_open_count"` // 一字板开盘次数
}

// LimitUpPoolStatistics contains pool statistics
type LimitUpPoolStatistics struct {
	TotalCount       int                    `json:"total_count"`
	MainBoardCount   int                    `json:"main_board_count"`   // 主板
	SMEBoardCount    int                    `json:"sme_board_count"`    // 中小板
	CyberBoardCount  int                    `json:"cyber_board_count"`  // 创业板
	STBoardCount     int                    `json:"st_board_count"`     // ST板块
	SectorDistribution map[string]int       `json:"sector_distribution"`
	ExplosionCount   int                    `json:"explosion_count"`
	ExplosionRate    float64                `json:"explosion_rate"`
	FirstBoardCount  int                    `json:"first_board_count"`
	ContinueBoardCount int                  `json:"continue_board_count"`
	FirstBoardRatio  float64                `json:"first_board_ratio"`
	AverageTurnover  float64                `json:"average_turnover"`
	TopLimitUpStocks []*LimitUpStockData    `json:"top_limit_up_stocks"`
	AnalysisTime     time.Time              `json:"analysis_time"`
}

// LimitUpAnalysisResult contains comprehensive limit-up analysis
type LimitUpAnalysisResult struct {
	PoolStatistics   *LimitUpPoolStatistics `json:"pool_statistics"`
	SectorAnalysis   map[string]*SectorLimitUpAnalysis `json:"sector_analysis"`
	MarketTrend      string                 `json:"market_trend"`
	RiskAssessment   string                 `json:"risk_assessment"`
	Recommendations  []string               `json:"recommendations"`
}

// SectorLimitUpAnalysis contains sector-specific limit-up analysis
type SectorLimitUpAnalysis struct {
	SectorName       string   `json:"sector_name"`
	TotalCount       int      `json:"total_count"`
	ExplosionCount   int      `json:"explosion_count"`
	ExplosionRate    float64  `json:"explosion_rate"`
	TopStocks        []*LimitUpStockData `json:"top_stocks"`
	FirstBoardCount  int      `json:"first_board_count"`
	ContinueBoardCount int    `json:"continue_board_count"`
}

// NewLimitUpAnalyzer creates a new limit-up analyzer
func NewLimitUpAnalyzer(config *LimitUpConfig) *LimitUpAnalyzer {
	if config == nil {
		config = &LimitUpConfig{
			RefreshInterval: 1 * time.Hour,
			HistoryLookback: 10,
			PoolSize:        50,
			ExplosionRate:   0.8,
			FirstBoardRatio: 0.6,
			SectorThreshold: 3,
		}
	}

	return &LimitUpAnalyzer{
		config: config,
		cache: &LimitUpCache{
			data: make(map[string]*LimitUpStockData),
		},
	}
}

// AnalyzeLimitUpPool performs comprehensive limit-up pool analysis
func (l *LimitUpAnalyzer) AnalyzeLimitUpPool(ctx context.Context) (*LimitUpAnalysisResult, error) {
	startTime := time.Now()

	// Fetch current limit-up data
	limitUpData, err := l.fetchLimitUpData(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch limit-up data: %w", err)
	}

	// Update cache
	l.updateCache(limitUpData)

	// Generate pool statistics
	poolStats := l.generatePoolStatistics(limitUpData)

	// Perform sector analysis
	sectorAnalysis := l.performSectorAnalysis(limitUpData)

	// Determine market trend
	marketTrend := l.determineMarketTrend(poolStats)

	// Assess risk
	riskAssessment := l.assessRisk(poolStats, marketTrend)

	// Generate recommendations
	recommendations := l.generateRecommendations(poolStats, marketTrend, riskAssessment)

	result := &LimitUpAnalysisResult{
		PoolStatistics:  poolStats,
		SectorAnalysis:  sectorAnalysis,
		MarketTrend:     marketTrend,
		RiskAssessment:  riskAssessment,
		Recommendations: recommendations,
	}

	duration := time.Since(startTime)
	logger.SugaredLogger.Infof("Limit-up pool analysis completed in %v, found %d limit-up stocks", duration, poolStats.TotalCount)

	return result, nil
}

// fetchLimitUpData simulates fetching limit-up stock data
func (l *LimitUpAnalyzer) fetchLimitUpData(ctx context.Context) ([]*LimitUpStockData, error) {
	// In production, this would call actual limit-up APIs
	// Simulating data for now

	simulatedStocks := []*LimitUpStockData{
		{
			StockCode:        "sh600000",
			StockName:        "浦发银行",
			LimitUpTime:      time.Now().Add(-2 * time.Hour),
			LimitUpPrice:     10.50,
			CurrentPrice:     10.50,
			ChangePercent:    10.00,
			Volume:           50000000.00,
			Amount:           525000000.00,
			Board:            "首板",
			ContinueDays:     1,
			FirstBoardCount:  3,
			Sector:           "银行",
			IsExplosion:      true,
			ExplosionRate:    0.95,
			TurnoverRate:     12.5,
			LimitUpOpenCount: 2,
		},
		{
			StockCode:        "sh600519",
			StockName:        "贵州茅台",
			LimitUpTime:      time.Now().Add(-1 * time.Hour),
			LimitUpPrice:     1800.00,
			CurrentPrice:     1800.00,
			ChangePercent:    10.00,
			Volume:           1000000.00,
			Amount:           1800000000.00,
			Board:            "连板",
			ContinueDays:     3,
			FirstBoardCount:  2,
			Sector:           "白酒",
			IsExplosion:      true,
			ExplosionRate:    0.92,
			TurnoverRate:     8.3,
			LimitUpOpenCount: 1,
		},
		{
			StockCode:        "sz000001",
			StockName:        "平安银行",
			LimitUpTime:      time.Now().Add(-3 * time.Hour),
			LimitUpPrice:     12.30,
			CurrentPrice:     12.30,
			ChangePercent:    10.00,
			Volume:           80000000.00,
			Amount:           984000000.00,
			Board:            "首板",
			ContinueDays:     1,
			FirstBoardCount:  1,
			Sector:           "银行",
			IsExplosion:      false,
			ExplosionRate:    0.78,
			TurnoverRate:     15.2,
			LimitUpOpenCount: 1,
		},
		{
			StockCode:        "sz300750",
			StockName:        "宁德时代",
			LimitUpTime:      time.Now().Add(-4 * time.Hour),
			LimitUpPrice:     185.00,
			CurrentPrice:     185.00,
			ChangePercent:    10.00,
			Volume:           20000000.00,
			Amount:           3700000000.00,
			Board:            "连板",
			ContinueDays:     2,
			FirstBoardCount:  4,
			Sector:           "新能源",
			IsExplosion:      true,
			ExplosionRate:    0.88,
			TurnoverRate:     18.7,
			LimitUpOpenCount: 2,
		},
	}

	return simulatedStocks, nil
}

// updateCache updates the limit-up cache
func (l *LimitUpAnalyzer) updateCache(data []*LimitUpStockData) {
	l.cache.mu.Lock()
	defer l.cache.mu.Unlock()

	l.cache.data = make(map[string]*LimitUpStockData)
	for _, stock := range data {
		l.cache.data[stock.StockCode] = stock
	}

	l.cache.lastUpdate = time.Now()
}

// generatePoolStatistics generates comprehensive pool statistics
func (l *LimitUpAnalyzer) generatePoolStatistics(limitUpData []*LimitUpStockData) *LimitUpPoolStatistics {
	stats := &LimitUpPoolStatistics{
		SectorDistribution: make(map[string]int),
		AnalysisTime:       time.Now(),
	}

	for _, stock := range limitUpData {
		stats.TotalCount++

		// Board classification
		switch {
		case stock.Board == "首板":
			stats.FirstBoardCount++
		case stock.Board == "连板":
			stats.ContinueBoardCount++
		}

		// Sector distribution
		if stock.Sector != "" {
			stats.SectorDistribution[stock.Sector]++
		}

		// Explosion tracking
		if stock.IsExplosion {
			stats.ExplosionCount++
		}

		// Total turnover calculation
		stats.AverageTurnover += stock.TurnoverRate
	}

	// Calculate derived statistics
	if stats.TotalCount > 0 {
		stats.ExplosionRate = float64(stats.ExplosionCount) / float64(stats.TotalCount)
		stats.FirstBoardRatio = float64(stats.FirstBoardCount) / float64(stats.TotalCount)
		stats.AverageTurnover = stats.AverageTurnover / float64(stats.TotalCount)
	}

	// Get top limit-up stocks by explosion rate
	stats.TopLimitUpStocks = l.getTopLimitUpStocks(limitUpData, 10)

	return stats
}

// getTopLimitUpStocks returns top N limit-up stocks
func (l *LimitUpAnalyzer) getTopLimitUpStocks(stocks []*LimitUpStockData, topN int) []*LimitUpStockData {
	// Sort by explosion rate and turnover rate
	sort.Slice(stocks, func(i, j int) bool {
		if stocks[i].IsExplosion && !stocks[j].IsExplosion {
			return true
		}
		if !stocks[i].IsExplosion && stocks[j].IsExplosion {
			return false
		}
		return stocks[i].ExplosionRate > stocks[j].ExplosionRate
	})

	if len(stocks) > topN {
		return stocks[:topN]
	}

	return stocks
}

// performSectorAnalysis performs sector-specific analysis
func (l *LimitUpAnalyzer) performSectorAnalysis(limitUpData []*LimitUpStockData) map[string]*SectorLimitUpAnalysis {
	sectorMap := make(map[string][]*LimitUpStockData)

	// Group stocks by sector
	for _, stock := range limitUpData {
		if stock.Sector != "" {
			sectorMap[stock.Sector] = append(sectorMap[stock.Sector], stock)
		}
	}

	// Analyze each sector
	sectorAnalysis := make(map[string]*SectorLimitUpAnalysis)
	for sectorName, stocks := range sectorMap {
		if len(stocks) < l.config.SectorThreshold {
			continue // Skip sectors with too few limit-up stocks
		}

		analysis := &SectorLimitUpAnalysis{
			SectorName:  sectorName,
			TotalCount:  len(stocks),
			TopStocks:   l.getTopLimitUpStocks(stocks, 3),
		}

		for _, stock := range stocks {
			if stock.IsExplosion {
				analysis.ExplosionCount++
			}

			if stock.Board == "首板" {
				analysis.FirstBoardCount++
			} else if stock.Board == "连板" {
				analysis.ContinueBoardCount++
			}
		}

		analysis.ExplosionRate = float64(analysis.ExplosionCount) / float64(analysis.TotalCount)

		sectorAnalysis[sectorName] = analysis
	}

	return sectorAnalysis
}

// determineMarketTrend determines the overall market trend
func (l *LimitUpAnalyzer) determineMarketTrend(poolStats *LimitUpPoolStatistics) string {
	if poolStats.TotalCount == 0 {
		return "无涨停板"
	}

	explosionRate := poolStats.ExplosionRate
	firstBoardRatio := poolStats.FirstBoardRatio

	// Determine trend based on multiple factors
	if explosionRate > l.config.ExplosionRate && firstBoardRatio > l.config.FirstBoardRatio {
		return "强势市场"
	} else if explosionRate > l.config.ExplosionRate {
		return "活跃市场"
	} else if firstBoardRatio > l.config.FirstBoardRatio {
		return "轮动市场"
	} else if poolStats.TotalCount > 20 {
		return "分化市场"
	} else if poolStats.TotalCount > 10 {
		return "弱市场"
	} else {
		return "低迷市场"
	}
}

// assessRisk assesses overall market risk
func (l *LimitUpAnalyzer) assessRisk(poolStats *LimitUpPoolStatistics, marketTrend string) string {
	highContinueBoardCount := 0

	for _, stock := range poolStats.TopLimitUpStocks {
		if stock.Board == "连板" && stock.ContinueDays >= 3 {
			highContinueBoardCount++
		}
	}

	// Risk assessment logic
	switch marketTrend {
	case "强势市场":
		if highContinueBoardCount > 3 {
			return "高风险"
		} else if poolStats.AverageTurnover > 15 {
			return "中风险"
		} else {
			return "低风险"
		}
	case "活跃市场":
		if poolStats.AverageTurnover > 20 {
			return "中风险"
		} else {
			return "低风险"
		}
	case "分化市场":
		if poolStats.ExplosionRate < 0.3 {
			return "中风险"
		} else {
			return "低风险"
		}
	default:
		return "低风险"
	}
}

// generateRecommendations generates actionable recommendations
func (l *LimitUpAnalyzer) generateRecommendations(poolStats *LimitUpPoolStatistics, marketTrend, riskAssessment string) []string {
	var recommendations []string

	switch marketTrend {
	case "强势市场":
		recommendations = append(recommendations, "市场强势，可关注领涨板块")
		recommendations = append(recommendations, "建议追高龙头股票，注意控制仓位")
		if poolStats.ExplosionRate > 0.8 {
			recommendations = append(recommendations, "爆板率过高，注意市场回调风险")
		}
	case "活跃市场":
		recommendations = append(recommendations, "市场活跃，板块轮动明显")
		recommendations = append(recommendations, "建议寻找低位启动的潜力股票")
	case "分化市场":
		recommendations = append(recommendations, "市场分化，需精选个股")
		recommendations = append(recommendations, "建议关注热点题材和基本面")
	case "弱市场", "低迷市场":
		recommendations = append(recommendations, "市场疲软，建议观望为主")
		recommendations = append(recommendations, "可考虑低吸优质个股，不宜追高")
	}

	switch riskAssessment {
	case "高风险":
		recommendations = append(recommendations, "市场风险较高，建议降低仓位")
		recommendations = append(recommendations, "注意设置止盈止损")
	case "中风险":
		recommendations = append(recommendations, "市场风险适中，注意控制风险")
		recommendations = append(recommendations, "建议分散投资，避免重仓单一股票")
	}

	return recommendations
}

// GetStockLimitUpInfo gets specific stock limit-up information
func (l *LimitUpAnalyzer) GetStockLimitUpInfo(stockCode string) (*LimitUpStockData, error) {
	l.cache.mu.RLock()
	defer l.cache.mu.RUnlock()

	stock, exists := l.cache.data[stockCode]
	if !exists {
		return nil, fmt.Errorf("stock %s not found in limit-up data", stockCode)
	}

	return stock, nil
}

// GetSectorLimitUpAnalysis gets sector-specific limit-up analysis
func (l *LimitUpAnalyzer) GetSectorLimitUpAnalysis(ctx context.Context, sectorName string) (*SectorLimitUpAnalysis, error) {
	// Fetch current limit-up data
	limitUpData, err := l.fetchLimitUpData(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch limit-up data: %w", err)
	}

	// Filter stocks by sector
	var sectorStocks []*LimitUpStockData
	for _, stock := range limitUpData {
		if stock.Sector == sectorName {
			sectorStocks = append(sectorStocks, stock)
		}
	}

	if len(sectorStocks) == 0 {
		return nil, fmt.Errorf("no limit-up stocks found in sector %s", sectorName)
	}

	// Generate sector analysis
	analysis := &SectorLimitUpAnalysis{
		SectorName:  sectorName,
		TotalCount:  len(sectorStocks),
		TopStocks:   l.getTopLimitUpStocks(sectorStocks, 5),
	}

	for _, stock := range sectorStocks {
		if stock.IsExplosion {
			analysis.ExplosionCount++
		}

		if stock.Board == "首板" {
			analysis.FirstBoardCount++
		} else if stock.Board == "连板" {
			analysis.ContinueBoardCount++
		}
	}

	analysis.ExplosionRate = float64(analysis.ExplosionCount) / float64(analysis.TotalCount)

	return analysis, nil
}

// MonitorLimitUpPool continuously monitors limit-up pool
func (l *LimitUpAnalyzer) MonitorLimitUpPool(ctx context.Context, interval time.Duration) (<-chan *LimitUpAnalysisResult, error) {
	resultChan := make(chan *LimitUpAnalysisResult, 1)

	go func() {
		defer close(resultChan)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				result, err := l.AnalyzeLimitUpPool(ctx)
				if err != nil {
					logger.SugaredLogger.Warnw("Failed to analyze limit-up pool", "error", err)
					continue
				}

				select {
				case resultChan <- result:
				default:
					// Channel full, skip this update
				}
			}
		}
	}()

	return resultChan, nil
}

// GetExplosionRankings gets rankings of explosion stocks
func (l *LimitUpAnalyzer) GetExplosionRankings(ctx context.Context) ([]*LimitUpRanking, error) {
	limitUpData, err := l.fetchLimitUpData(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch limit-up data: %w", err)
	}

	var rankings []*LimitUpRanking
	for _, stock := range limitUpData {
		if stock.IsExplosion {
			ranking := &LimitUpRanking{
				StockCode:     stock.StockCode,
				StockName:     stock.StockName,
				Sector:        stock.Sector,
				ExplosionRate: stock.ExplosionRate,
				TurnoverRate:  stock.TurnoverRate,
				Board:         stock.Board,
				ContinueDays:  stock.ContinueDays,
				Score:         l.calculateExplosionScore(stock),
			}
			rankings = append(rankings, ranking)
		}
	}

	// Sort by score
	sort.Slice(rankings, func(i, j int) bool {
		return rankings[i].Score > rankings[j].Score
	})

	// Add rankings
	for i := range rankings {
		rankings[i].Ranking = i + 1
	}

	return rankings, nil
}

// LimitUpRanking represents a stock's limit-up ranking
type LimitUpRanking struct {
	StockCode     string  `json:"stock_code"`
	StockName     string  `json:"stock_name"`
	Sector        string  `json:"sector"`
	ExplosionRate float64 `json:"explosion_rate"`
	TurnoverRate  float64 `json:"turnover_rate"`
	Board         string  `json:"board"`
	ContinueDays  int     `json:"continue_days"`
	Score         float64 `json:"score"`
	Ranking       int     `json:"ranking"`
}

// calculateExplosionScore calculates comprehensive explosion score
func (l *LimitUpAnalyzer) calculateExplosionScore(stock *LimitUpStockData) float64 {
	score := 0.0

	// Explosion rate score (0-30 points)
	score += stock.ExplosionRate * 30

	// Turnover rate score (0-25 points)
	if stock.TurnoverRate > 20 {
		score += 25
	} else if stock.TurnoverRate > 15 {
		score += 20
	} else if stock.TurnoverRate > 10 {
		score += 15
	} else {
		score += 10
	}

	// Continue days score (0-25 points)
	if stock.ContinueDays >= 5 {
		score += 25
	} else if stock.ContinueDays >= 3 {
		score += 20
	} else if stock.ContinueDays >= 2 {
		score += 15
	} else {
		score += 10
	}

	// Board bonus (0-20 points)
	if stock.Board == "连板" {
		score += 20
	} else if stock.Board == "首板" {
		score += 15
	}

	// Ensure score is within bounds
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}