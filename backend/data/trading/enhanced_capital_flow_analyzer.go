// backend/data/trading/enhanced_capital_flow_analyzer.go
package trading

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"go-stock/backend/data/types"
	"go-stock/backend/logger"
)

// EnhancedCapitalFlowAnalyzer provides comprehensive capital flow analysis
type EnhancedCapitalFlowAnalyzer struct {
	config      *CapitalFlowConfig
	historical  map[string]*CapitalFlowHistory
	mu          sync.RWMutex
}

// CapitalFlowConfig configures capital flow analysis behavior
type CapitalFlowConfig struct {
	HistoryLength   int
	PercentileWindow int
	SuperLargeThreshold float64
	LargeThreshold    float64
	MediumThreshold   float64
	SmallThreshold    float64
}

// CapitalFlowHistory tracks historical capital flow data
type CapitalFlowHistory struct {
	StockCode     string
	DailyFlows    map[string]*DailyCapitalFlow
	AverageFlows  *AverageCapitalFlow
	Trend         string
	Volatility    float64
	LastUpdated   time.Time
}

// DailyCapitalFlow represents daily capital flow data
type DailyCapitalFlow struct {
	Date           string
	SuperLargeFlow float64
	LargeFlow      float64
	MediumFlow     float64
	SmallFlow      float64
	MainNetFlow    float64
	RetailNetFlow  float64
	TotalFlow      float64
	BuySellRatio   float64
	ClosePrice     float64
	Volume         float64
	Turnover       float64
}

// AverageCapitalFlow contains averaged capital flow metrics
type AverageCapitalFlow struct {
	SuperLargeAvg  float64
	LargeAvg       float64
	MediumAvg      float64
	SmallAvg       float64
	MainNetAvg     float64
	RetailNetAvg   float64
	TotalAvg       float64
	BuySellRatioAvg float64
}

// CapitalFlowAnalysisResult contains comprehensive analysis results
type CapitalFlowAnalysisResult struct {
	StockCode           string                         `json:"stock_code"`
	AnalysisDate        time.Time                      `json:"analysis_date"`
	CurrentFlow         *DailyCapitalFlow             `json:"current_flow"`
	HistoricalTrends    *HistoricalTrendAnalysis       `json:"historical_trends"`
	PercentileRank      *PercentileRanking            `json:"percentile_rank"`
	FeeStructure        *FeeStructureAnalysis          `json:"fee_structure"`
	PatternRecognition  *FlowPatternRecognition        `json:"pattern_recognition"`
	RiskAssessment      *FlowRiskAssessment            `json:"risk_assessment"`
	Predictions         *CapitalFlowPrediction         `json:"predictions"`
	Recommendations     []string                       `json:"recommendations"`
}

// HistoricalTrendAnalysis analyzes historical capital flow trends
type HistoricalTrendAnalysis struct {
	TrendDirection      string               `json:"trend_direction"`
	TrendStrength       float64              `json:"trend_strength"`
	Volatility          float64              `json:"volatility"`
	Momentum            float64              `json:"momentum"`
	ReversalSignal      bool                 `json:"reversal_signal"`
	TrendChanges        []TrendChangePoint   `json:"trend_changes"`
}

// TrendChangePoint identifies significant trend changes
type TrendChangePoint struct {
	Date         string
	PreviousTrend string
	NewTrend      string
	Significance  float64
}

// PercentileRanking compares current flow against historical data
type PercentileRanking struct {
	SuperLargePercentile float64
	LargePercentile      float64
	MediumPercentile     float64
	SmallPercentile      float64
	MainNetPercentile    float64
	RetailNetPercentile  float64
	OverallPercentile    float64
	Ranking              int
	TotalRanked          int
}

// FeeStructureAnalysis analyzes capital flow by fee structure
type FeeStructureAnalysis struct {
	SuperLargeContribution float64
	LargeContribution      float64
	MediumContribution     float64
	SmallContribution      float64
	MainCapitalShare       float64
	RetailCapitalShare     float64
	DominantFeeTier        string
	ConcentrationRatio     float64
}

// FlowPatternRecognition identifies flow patterns
type FlowPatternRecognition struct {
	PatternType          string
	Confidence           float64
	MatchedPatterns      []string
	PatternDuration      time.Duration
	NextMovePrediction   string
	Probability          float64
}

// FlowRiskAssessment evaluates capital flow risk
type FlowRiskAssessment struct {
	RiskLevel            string
	RiskScore            float64
	RiskFactors          []string
	LiquidityRisk        float64
	VolatilityRisk       float64
	ConcentrationRisk    float64
	TrendReversalRisk    float64
	StopLossLevel        float64
	TakeProfitLevel      float64
}

// CapitalFlowPrediction provides capital flow predictions
type CapitalFlowPrediction struct {
	ShortTermPrediction    string
	MediumTermPrediction   string
	LongTermPrediction     string
	ConfidenceInterval     float64
	KeyInfluencingFactors  []string
	SensitivityAnalysis    map[string]float64
}

// NewEnhancedCapitalFlowAnalyzer creates a new enhanced capital flow analyzer
func NewEnhancedCapitalFlowAnalyzer(config *CapitalFlowConfig) *EnhancedCapitalFlowAnalyzer {
	if config == nil {
		config = &CapitalFlowConfig{
			HistoryLength:     30,
			PercentileWindow:  20,
			SuperLargeThreshold: 100000000.00,  // 超大单
			LargeThreshold:    50000000.00,   // 大单
			MediumThreshold:   20000000.00,   // 中单
			SmallThreshold:    5000000.00,    // 小单
		}
	}

	return &EnhancedCapitalFlowAnalyzer{
		config:     config,
		historical: make(map[string]*CapitalFlowHistory),
	}
}

// AnalyzeCapitalFlow performs comprehensive capital flow analysis
func (a *EnhancedCapitalFlowAnalyzer) AnalyzeCapitalFlow(ctx context.Context, stockCode, date string) (*CapitalFlowAnalysisResult, error) {
	startTime := time.Now()

	// Fetch current and historical capital flow data
	currentFlow, historicalData, err := a.fetchCapitalFlowData(ctx, stockCode, date)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch capital flow data: %w", err)
	}

	// Update historical data cache
	a.updateHistoricalData(stockCode, historicalData)

	// Perform comprehensive analysis
	result := &CapitalFlowAnalysisResult{
		StockCode:    stockCode,
		AnalysisDate: time.Now(),
		CurrentFlow:  currentFlow,
	}

	// Historical trend analysis
	result.HistoricalTrends = a.analyzeHistoricalTrends(stockCode)

	// Percentile ranking
	result.PercentileRank = a.calculatePercentileRanking(stockCode, currentFlow)

	// Fee structure analysis
	result.FeeStructure = a.analyzeFeeStructure(currentFlow)

	// Pattern recognition
	result.PatternRecognition = a.recognizeFlowPatterns(stockCode)

	// Risk assessment
	result.RiskAssessment = a.assessFlowRisk(result)

	// Predictions
	result.Predictions = a.generatePredictions(result)

	// Generate recommendations
	result.Recommendations = a.generateRecommendations(result)

	duration := time.Since(startTime)
	logger.SugaredLogger.Infof("Enhanced capital flow analysis completed for %s in %v", stockCode, duration)

	return result, nil
}

// fetchCapitalFlowData simulates fetching capital flow data
func (a *EnhancedCapitalFlowAnalyzer) fetchCapitalFlowData(ctx context.Context, stockCode, date string) (*DailyCapitalFlow, map[string]*DailyCapitalFlow, error) {
	// Simulate current flow data
	currentFlow := &DailyCapitalFlow{
		Date:            date,
		SuperLargeFlow:  15000000.00,
		LargeFlow:       8000000.00,
		MediumFlow:      4500000.00,
		SmallFlow:       -27500000.00,
		MainNetFlow:     12500000.00,
		RetailNetFlow:   -8500000.00,
		TotalFlow:       4000000.00,
		BuySellRatio:    1.45,
		ClosePrice:      10.25,
		Volume:          100000000.00,
		Turnover:        12.5,
	}

	// Simulate historical data
	historicalData := make(map[string]*DailyCapitalFlow)
	baseDate, _ := time.Parse("2006-01-02", date)

	for i := 0; i < a.config.HistoryLength; i++ {
		historicalDate := baseDate.AddDate(0, 0, -i)
		dateStr := historicalDate.Format("2006-01-02")

		// Generate synthetic historical data with some patterns
		superLarge := 10000000.00 + float64(i)*500000.00 + (float64(i%5)-2)*2000000.00
		large := 5000000.00 + float64(i)*300000.00
		medium := 3000000.00 + float64(i)*200000.00
		small := -(superLarge + large + medium + float64(i%3)*1000000.00)

		historicalFlow := &DailyCapitalFlow{
			Date:           dateStr,
			SuperLargeFlow: superLarge,
			LargeFlow:      large,
			MediumFlow:     medium,
			SmallFlow:      small,
			MainNetFlow:    superLarge + large,
			RetailNetFlow:  small,
			TotalFlow:      superLarge + large + medium + small,
			BuySellRatio:   1.2 + float64(i%7)*0.1,
			ClosePrice:     10.0 + float64(i)*0.01,
			Volume:         80000000.00 + float64(i)*2000000.00,
			Turnover:       10.0 + float64(i)*0.2,
		}

		historicalData[dateStr] = historicalFlow
	}

	return currentFlow, historicalData, nil
}

// updateHistoricalData updates the historical data cache
func (a *EnhancedCapitalFlowAnalyzer) updateHistoricalData(stockCode string, historicalData map[string]*DailyCapitalFlow) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.historical[stockCode] == nil {
		a.historical[stockCode] = &CapitalFlowHistory{
			StockCode:  stockCode,
			DailyFlows: make(map[string]*DailyCapitalFlow),
		}
	}

	// Update with new data
	for date, flow := range historicalData {
		a.historical[stockCode].DailyFlows[date] = flow
	}

	// Update averages
	a.calculateAverageFlows(stockCode)

	a.historical[stockCode].LastUpdated = time.Now()
}

// calculateAverageFlows calculates average capital flow metrics
func (a *EnhancedCapitalFlowAnalyzer) calculateAverageFlows(stockCode string) {
	history := a.historical[stockCode]
	if history == nil || len(history.DailyFlows) == 0 {
		return
	}

	var superLargeSum, largeSum, mediumSum, smallSum, mainNetSum, retailNetSum, totalSum, buySellRatioSum float64
	count := 0

	for _, flow := range history.DailyFlows {
		superLargeSum += flow.SuperLargeFlow
		largeSum += flow.LargeFlow
		mediumSum += flow.MediumFlow
		smallSum += flow.SmallFlow
		mainNetSum += flow.MainNetFlow
		retailNetSum += flow.RetailNetFlow
		totalSum += flow.TotalFlow
		buySellRatioSum += flow.BuySellRatio
		count++
	}

	if count > 0 {
		history.AverageFlows = &AverageCapitalFlow{
			SuperLargeAvg:   superLargeSum / float64(count),
			LargeAvg:        largeSum / float64(count),
			MediumAvg:       mediumSum / float64(count),
			SmallAvg:        smallSum / float64(count),
			MainNetAvg:      mainNetSum / float64(count),
			RetailNetAvg:    retailNetSum / float64(count),
			TotalAvg:        totalSum / float64(count),
			BuySellRatioAvg: buySellRatioSum / float64(count),
		}
	}

	// Calculate trend and volatility
	history.Trend = a.calculateTrend(history.DailyFlows)
	history.Volatility = a.calculateVolatility(history.DailyFlows)
}

// calculateTrend determines the overall capital flow trend
func (a *EnhancedCapitalFlowAnalyzer) calculateTrend(dailyFlows map[string]*DailyCapitalFlow) string {
	if len(dailyFlows) < 3 {
		return "数据不足"
	}

	// Sort flows by date
	var dates []string
	for date := range dailyFlows {
		dates = append(dates, date)
	}
	sort.Strings(dates)

	// Calculate recent trend (last 5 days)
	recentNetFlows := make([]float64, 0, 5)
	startIdx := max(0, len(dates)-5)

	for i := startIdx; i < len(dates); i++ {
		recentNetFlows = append(recentNetFlows, dailyFlows[dates[i]].MainNetFlow)
	}

	// Calculate trend direction
	var totalChange float64
	for i := 1; i < len(recentNetFlows); i++ {
		totalChange += recentNetFlows[i] - recentNetFlows[i-1]
	}

	avgChange := totalChange / float64(len(recentNetFlows)-1)

	if avgChange > 1000000 {
		return "强劲流入"
	} else if avgChange > 0 {
		return "持续流入"
	} else if avgChange > -1000000 {
		return "资金平衡"
	} else if avgChange > -5000000 {
		return "持续流出"
	} else {
		return "大幅流出"
	}
}

// calculateVolatility calculates capital flow volatility
func (a *EnhancedCapitalFlowAnalyzer) calculateVolatility(dailyFlows map[string]*DailyCapitalFlow) float64 {
	if len(dailyFlows) < 2 {
		return 0
	}

	var netFlows []float64
	for _, flow := range dailyFlows {
		netFlows = append(netFlows, flow.MainNetFlow)
	}

	mean := calculateMean(netFlows)
	var variance float64

	for _, flow := range netFlows {
		variance += math.Pow(flow-mean, 2)
	}

	variance /= float64(len(netFlows))
	return math.Sqrt(variance)
}

// analyzeHistoricalTrends performs historical trend analysis
func (a *EnhancedCapitalFlowAnalyzer) analyzeHistoricalTrends(stockCode string) *HistoricalTrendAnalysis {
	a.mu.RLock()
	history := a.historical[stockCode]
	a.mu.RUnlock()

	if history == nil || len(history.DailyFlows) < 3 {
		return &HistoricalTrendAnalysis{
			TrendDirection: "数据不足",
			TrendStrength:  0,
		}
	}

	// Calculate momentum
	momentum := a.calculateMomentum(history.DailyFlows)

	// Identify trend changes
	trendChanges := a.identifyTrendChanges(history.DailyFlows)

	// Detect reversal signals
	reversalSignal := a.detectReversalSignal(history.DailyFlows)

	return &HistoricalTrendAnalysis{
		TrendDirection: history.Trend,
		TrendStrength:  a.calculateTrendStrength(history.DailyFlows),
		Volatility:     history.Volatility,
		Momentum:       momentum,
		ReversalSignal: reversalSignal,
		TrendChanges:   trendChanges,
	}
}

// calculateMomentum calculates capital flow momentum
func (a *EnhancedCapitalFlowAnalyzer) calculateMomentum(dailyFlows map[string]*DailyCapitalFlow) float64 {
	if len(dailyFlows) < 5 {
		return 0
	}

	var dates []string
	for date := range dailyFlows {
		dates = append(dates, date)
	}
	sort.Strings(dates)

	// Calculate 5-day momentum
	recent := dailyFlows[dates[0]].MainNetFlow
	previous := dailyFlows[dates[4]].MainNetFlow

	return (recent - previous) / math.Abs(previous) * 100
}

// identifyTrendChanges identifies significant trend changes
func (a *EnhancedCapitalFlowAnalyzer) identifyTrendChanges(dailyFlows map[string]*DailyCapitalFlow) []TrendChangePoint {
	var changes []TrendChangePoint

	if len(dailyFlows) < 5 {
		return changes
	}

	var dates []string
	for date := range dailyFlows {
		dates = append(dates, date)
	}
	sort.Strings(dates)

	// Detect trend changes
	currentTrend := a.calculateTrend(dailyFlows)

	for i := 0; i < len(dates)-3; i++ {
		subset := make(map[string]*DailyCapitalFlow)
		for j := i; j < i+3; j++ {
			subset[dates[j]] = dailyFlows[dates[j]]
		}

		subsetTrend := a.calculateTrend(subset)

		if subsetTrend != currentTrend {
			change := TrendChangePoint{
				Date:          dates[i+2],
				PreviousTrend: subsetTrend,
				NewTrend:      currentTrend,
				Significance:  float64(len(dates)-i) / float64(len(dates)),
			}
			changes = append(changes, change)
		}
	}

	return changes
}

// detectReversalSignal detects potential trend reversals
func (a *EnhancedCapitalFlowAnalyzer) detectReversalSignal(dailyFlows map[string]*DailyCapitalFlow) bool {
	if len(dailyFlows) < 7 {
		return false
	}

	var dates []string
	for date := range dailyFlows {
		dates = append(dates, date)
	}
	sort.Strings(dates)

	// Check for divergence pattern
	recentAvg := calculateMovingAverage(dailyFlows, dates, 3)
	historicalAvg := calculateMovingAverage(dailyFlows, dates, 7)

	// Reversal signal: recent average moving opposite to historical trend
	if recentAvg > 0 && historicalAvg < 0 {
		return true
	}
	if recentAvg < 0 && historicalAvg > 0 {
		return true
	}

	return false
}

// calculateTrendStrength calculates the strength of the current trend
func (a *EnhancedCapitalFlowAnalyzer) calculateTrendStrength(dailyFlows map[string]*DailyCapitalFlow) float64 {
	if len(dailyFlows) < 3 {
		return 0
	}

	var dates []string
	for date := range dailyFlows {
		dates = append(dates, date)
	}
	sort.Strings(dates)

	// Calculate trend consistency
	trendSign := 0
	trendConsistentDays := 0

	for i := 0; i < len(dates)-1; i++ {
		current := dailyFlows[dates[i]].MainNetFlow
		next := dailyFlows[dates[i+1]].MainNetFlow

		if i == 0 {
			if current > 0 {
				trendSign = 1
			} else if current < 0 {
				trendSign = -1
			}
		}

		if trendSign == 1 && next > current {
			trendConsistentDays++
		} else if trendSign == -1 && next < current {
			trendConsistentDays++
		} else if trendSign == 0 && (next > 0 || next < 0) {
			// Set trend direction
			if next > 0 {
				trendSign = 1
			} else {
				trendSign = -1
			}
			trendConsistentDays = 1
		}
	}

	return float64(trendConsistentDays) / float64(len(dates))
}

// calculatePercentileRanking calculates percentile rankings
func (a *EnhancedCapitalFlowAnalyzer) calculatePercentileRanking(stockCode string, currentFlow *DailyCapitalFlow) *PercentileRanking {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Collect all historical values for comparison
	var allSuperLargeFlows, allLargeFlows, allMediumFlows, allSmallFlows, allMainNetFlows, allRetailNetFlows []float64

	for _, history := range a.historical {
		for _, flow := range history.DailyFlows {
			allSuperLargeFlows = append(allSuperLargeFlows, flow.SuperLargeFlow)
			allLargeFlows = append(allLargeFlows, flow.LargeFlow)
			allMediumFlows = append(allMediumFlows, flow.MediumFlow)
			allSmallFlows = append(allSmallFlows, flow.SmallFlow)
			allMainNetFlows = append(allMainNetFlows, flow.MainNetFlow)
			allRetailNetFlows = append(allRetailNetFlows, flow.RetailNetFlow)
		}
	}

	// Calculate percentiles
	superLargePercentile := calculatePercentile(currentFlow.SuperLargeFlow, allSuperLargeFlows)
	largePercentile := calculatePercentile(currentFlow.LargeFlow, allLargeFlows)
	mediumPercentile := calculatePercentile(currentFlow.MediumFlow, allMediumFlows)
	smallPercentile := calculatePercentile(currentFlow.SmallFlow, allSmallFlows)
	mainNetPercentile := calculatePercentile(currentFlow.MainNetFlow, allMainNetFlows)
	retailNetPercentile := calculatePercentile(currentFlow.RetailNetFlow, allRetailNetFlows)

	// Calculate overall percentile
	overallPercentile := (superLargePercentile + largePercentile + mediumPercentile + smallPercentile + mainNetPercentile + retailNetPercentile) / 6

	// Calculate ranking based on main net flow
	ranking := 1
	for _, flow := range allMainNetFlows {
		if flow > currentFlow.MainNetFlow {
			ranking++
		}
	}

	return &PercentileRanking{
		SuperLargePercentile: superLargePercentile,
		LargePercentile:      largePercentile,
		MediumPercentile:     mediumPercentile,
		SmallPercentile:      smallPercentile,
		MainNetPercentile:    mainNetPercentile,
		RetailNetPercentile:  retailNetPercentile,
		OverallPercentile:   overallPercentile,
		Ranking:              ranking,
		TotalRanked:          len(allMainNetFlows),
	}
}

// analyzeFeeStructure analyzes capital flow by fee structure
func (a *EnhancedCapitalFlowAnalyzer) analyzeFeeStructure(flow *DailyCapitalFlow) *FeeStructureAnalysis {
	totalMainFlow := flow.SuperLargeFlow + flow.LargeFlow + flow.MediumFlow
	totalRetailFlow := flow.SmallFlow
	totalFlow := math.Abs(totalMainFlow) + math.Abs(totalRetailFlow)

	if totalFlow == 0 {
		return &FeeStructureAnalysis{
			SuperLargeContribution: 0,
			LargeContribution:      0,
			MediumContribution:     0,
			SmallContribution:      0,
			MainCapitalShare:       0,
			RetailCapitalShare:     0,
			DominantFeeTier:        "无数据",
			ConcentrationRatio:     0,
		}
	}

	superLargeContribution := math.Abs(flow.SuperLargeFlow) / totalFlow * 100
	largeContribution := math.Abs(flow.LargeFlow) / totalFlow * 100
	mediumContribution := math.Abs(flow.MediumFlow) / totalFlow * 100
	smallContribution := math.Abs(flow.SmallFlow) / totalFlow * 100
	mainCapitalShare := math.Abs(totalMainFlow) / totalFlow * 100
	retailCapitalShare := math.Abs(totalRetailFlow) / totalFlow * 100

	// Determine dominant fee tier
	maxContribution := superLargeContribution
	dominantTier := "超大单"

	if largeContribution > maxContribution {
		maxContribution = largeContribution
		dominantTier = "大单"
	}
	if mediumContribution > maxContribution {
		maxContribution = mediumContribution
		dominantTier = "中单"
	}
	if smallContribution > maxContribution {
		maxContribution = smallContribution
		dominantTier = "小单"
	}

	// Calculate concentration ratio
	topTwoContributions := []float64{superLargeContribution, largeContribution, mediumContribution, smallContribution}
	sort.Float64s(topTwoContributions)
	concentrationRatio := topTwoContributions[2] + topTwoContributions[3] // Top 2

	return &FeeStructureAnalysis{
		SuperLargeContribution: superLargeContribution,
		LargeContribution:      largeContribution,
		MediumContribution:     mediumContribution,
		SmallContribution:      smallContribution,
		MainCapitalShare:       mainCapitalShare,
		RetailCapitalShare:     retailCapitalShare,
		DominantFeeTier:        dominantTier,
		ConcentrationRatio:     concentrationRatio,
	}
}

// recognizeFlowPatterns identifies capital flow patterns
func (a *EnhancedCapitalFlowAnalyzer) recognizeFlowPatterns(stockCode string) *FlowPatternRecognition {
	a.mu.RLock()
	history := a.historical[stockCode]
	a.mu.RUnlock()

	if history == nil || len(history.DailyFlows) < 7 {
		return &FlowPatternRecognition{
			PatternType:       "数据不足",
			Confidence:        0,
			MatchedPatterns:   []string{},
			NextMovePrediction: "无法预测",
			Probability:       0,
		}
	}

	// Identify common patterns
	matchedPatterns := a.identifyPatterns(history.DailyFlows)

	// Determine dominant pattern
	patternType := "无显著模式"
	confidence := 0.0

	if len(matchedPatterns) > 0 {
		patternType = matchedPatterns[0]
		confidence = 0.7
		if len(matchedPatterns) >= 2 {
			confidence = 0.85
		}
		if len(matchedPatterns) >= 3 {
			confidence = 0.95
		}
	}

	// Predict next move
	prediction, probability := a.predictNextMove(history.DailyFlows, matchedPatterns)

	return &FlowPatternRecognition{
		PatternType:        patternType,
		Confidence:         confidence,
		MatchedPatterns:    matchedPatterns,
		NextMovePrediction: prediction,
		Probability:        probability,
	}
}

// identifyPatterns identifies common capital flow patterns
func (a *EnhancedCapitalFlowAnalyzer) identifyPatterns(dailyFlows map[string]*DailyCapitalFlow) []string {
	var patterns []string

	if len(dailyFlows) < 5 {
		return patterns
	}

	var dates []string
	for date := range dailyFlows {
		dates = append(dates, date)
	}
	sort.Strings(dates)

	// Pattern 1: Sustained inflow
	sustainedInflow := true
	for i := 0; i < min(5, len(dates)); i++ {
		if dailyFlows[dates[i]].MainNetFlow <= 0 {
			sustainedInflow = false
			break
		}
	}
	if sustainedInflow {
		patterns = append(patterns, "持续流入")
	}

	// Pattern 2: Accelerating inflow
	acceleratingInflow := true
	for i := 1; i < min(5, len(dates)); i++ {
		if dailyFlows[dates[i]].MainNetFlow <= dailyFlows[dates[i-1]].MainNetFlow {
			acceleratingInflow = false
			break
		}
	}
	if acceleratingInflow {
		patterns = append(patterns, "加速流入")
	}

	// Pattern 3: Large cap dominance
	largeCapDominance := true
	for i := 0; i < min(5, len(dates)); i++ {
		flow := dailyFlows[dates[i]]
		if flow.SuperLargeFlow+flow.LargeFlow < math.Abs(flow.SmallFlow) {
			largeCapDominance = false
			break
		}
	}
	if largeCapDominance {
		patterns = append(patterns, "大单主导")
	}

	// Pattern 4: Retail reversal
	retailReversal := false
	if len(dates) >= 7 {
		recentRetail := dailyFlows[dates[0]].RetailNetFlow
		historicalRetail := dailyFlows[dates[6]].RetailNetFlow

		if (recentRetail > 0 && historicalRetail < 0) || (recentRetail < 0 && historicalRetail > 0) {
			retailReversal = true
		}
	}
	if retailReversal {
		patterns = append(patterns, "散户反转")
	}

	return patterns
}

// predictNextMove predicts the next capital flow move
func (a *EnhancedCapitalFlowAnalyzer) predictNextMove(dailyFlows map[string]*DailyCapitalFlow, matchedPatterns []string) (string, float64) {
	if len(matchedPatterns) == 0 {
		return "不确定", 0.3
	}

	// Simple prediction based on patterns
	hasInflowPattern := containsString(matchedPatterns, "持续流入") || containsString(matchedPatterns, "加速流入")
	hasLargeCapPattern := containsString(matchedPatterns, "大单主导")
	hasReversalPattern := containsString(matchedPatterns, "散户反转")

	if hasInflowPattern && hasLargeCapPattern {
		return "强势流入", 0.85
	} else if hasInflowPattern {
		return "持续流入", 0.75
	} else if hasLargeCapPattern {
		return "大单流入", 0.7
	} else if hasReversalPattern {
		return "趋势反转", 0.6
	} else {
		return "观望", 0.5
	}
}

// assessFlowRisk assesses capital flow risk
func (a *EnhancedCapitalFlowAnalyzer) assessFlowRisk(result *CapitalFlowAnalysisResult) *FlowRiskAssessment {
	riskScore := 0.0
	var riskFactors []string

	// Assess volatility risk
	volatilityRisk := min(result.HistoricalTrends.Volatility/10000000.0, 1.0)
	riskScore += volatilityRisk * 25

	if volatilityRisk > 0.5 {
		riskFactors = append(riskFactors, "波动性较高")
	}

	// Assess concentration risk
	concentrationRisk := result.FeeStructure.ConcentrationRatio / 100.0
	riskScore += concentrationRisk * 20

	if concentrationRisk > 0.6 {
		riskFactors = append(riskFactors, "资金集中度过高")
	}

	// Assess trend reversal risk
	reversalRisk := 0.0
	if result.HistoricalTrends.ReversalSignal {
		reversalRisk = 0.8
		riskFactors = append(riskFactors, "趋势反转信号")
	}
	riskScore += reversalRisk * 30

	// Assess liquidity risk
	liquidityRisk := 1.0 - min(result.CurrentFlow.Turnover/20.0, 1.0)
	riskScore += liquidityRisk * 25

	if liquidityRisk > 0.5 {
		riskFactors = append(riskFactors, "流动性不足")
	}

	// Determine risk level
	riskLevel := "低风险"
	if riskScore > 75 {
		riskLevel = "高风险"
	} else if riskScore > 50 {
		riskLevel = "中风险"
	}

	// Calculate stop loss and take profit levels
	currentPrice := result.CurrentFlow.ClosePrice
	volatility := result.HistoricalTrends.Volatility / currentPrice

	stopLossLevel := currentPrice * (1 - volatility*2)  // 2倍波动率作为止损
	takeProfitLevel := currentPrice * (1 + volatility*3) // 3倍波动率作为止盈

	return &FlowRiskAssessment{
		RiskLevel:         riskLevel,
		RiskScore:         riskScore,
		RiskFactors:       riskFactors,
		LiquidityRisk:     liquidityRisk * 100,
		VolatilityRisk:    volatilityRisk * 100,
		ConcentrationRisk: concentrationRisk * 100,
		TrendReversalRisk: reversalRisk * 100,
		StopLossLevel:     stopLossLevel,
		TakeProfitLevel:   takeProfitLevel,
	}
}

// generatePredictions generates capital flow predictions
func (a *EnhancedCapitalFlowAnalyzer) generatePredictions(result *CapitalFlowAnalysisResult) *CapitalFlowPrediction {
	// Short-term prediction (1-3 days)
	shortTermPrediction := result.PatternRecognition.NextMovePrediction
	shortTermConfidence := result.PatternRecognition.Probability

	// Medium-term prediction (1-2 weeks)
	mediumTermPrediction := a.predictMediumTerm(result)
	mediumTermConfidence := 0.6

	// Long-term prediction (1+ months)
	longTermPrediction := a.predictLongTerm(result)
	longTermConfidence := 0.4

	// Identify key influencing factors
	keyFactors := a.identifyKeyFactors(result)

	// Sensitivity analysis
	sensitivity := map[string]float64{
		"市场情绪":  0.3,
		"板块轮动":  0.25,
		"资金面":   0.2,
		"基本面":   0.15,
		"技术面":   0.1,
	}

	return &CapitalFlowPrediction{
		ShortTermPrediction:  shortTermPrediction,
		MediumTermPrediction: mediumTermPrediction,
		LongTermPrediction:   longTermPrediction,
		ConfidenceInterval:   min(shortTermConfidence, 0.95),
		KeyInfluencingFactors: keyFactors,
		SensitivityAnalysis:   sensitivity,
	}
}

// predictMediumTerm predicts medium-term capital flow
func (a *EnhancedCapitalFlowAnalyzer) predictMediumTerm(result *CapitalFlowAnalysisResult) string {
	trend := result.HistoricalTrends.TrendDirection
	momentum := result.HistoricalTrends.Momentum

	if momentum > 5 && (trend == "强劲流入" || trend == "持续流入") {
		return "持续看好"
	} else if momentum < -5 && (trend == "大幅流出" || trend == "持续流出") {
		return "持续看淡"
	} else if result.PatternRecognition.NextMovePrediction == "趋势反转" {
		return "趋势转换"
	} else {
		return "区间震荡"
	}
}

// predictLongTerm predicts long-term capital flow
func (a *EnhancedCapitalFlowAnalyzer) predictLongTerm(result *CapitalFlowAnalysisResult) string {
	// Long-term predictions are more conservative
	if result.FeeStructure.MainCapitalShare > 60 {
		return "机构主导"
	} else if result.FeeStructure.RetailCapitalShare > 60 {
		return "散户主导"
	} else {
		return "资金平衡"
	}
}

// identifyKeyFactors identifies key factors influencing capital flow
func (a *EnhancedCapitalFlowAnalyzer) identifyKeyFactors(result *CapitalFlowAnalysisResult) []string {
	var factors []string

	// Analyze fee structure
	if result.FeeStructure.DominantFeeTier == "超大单" {
		factors = append(factors, "机构资金活跃")
	} else if result.FeeStructure.DominantFeeTier == "小单" {
		factors = append(factors, "散户情绪主导")
	}

	// Analyze trend
	if result.HistoricalTrends.TrendDirection == "强劲流入" {
		factors = append(factors, "资金流入强劲")
	} else if result.HistoricalTrends.TrendDirection == "大幅流出" {
		factors = append(factors, "资金流出明显")
	}

	// Analyze patterns
	if len(result.PatternRecognition.MatchedPatterns) > 0 {
		factors = append(factors, "趋势模式明确")
	}

	// Analyze risk
	if result.RiskAssessment.RiskLevel != "低风险" {
		factors = append(factors, "风险因素需要关注")
	}

	return factors
}

// generateRecommendations generates actionable recommendations
func (a *EnhancedCapitalFlowAnalyzer) generateRecommendations(result *CapitalFlowAnalysisResult) []string {
	var recommendations []string

	// Trend-based recommendations
	switch result.HistoricalTrends.TrendDirection {
	case "强劲流入", "持续流入":
		recommendations = append(recommendations, "资金流入积极，可考虑逢低布局")
		if result.FeeStructure.MainCapitalShare > 60 {
			recommendations = append(recommendations, "机构资金主导，适合中长期投资")
		}
	case "大幅流出", "持续流出":
		recommendations = append(recommendations, "资金流出明显，建议谨慎观望")
		recommendations = append(recommendations, "可等待企稳信号再入场")
	case "资金平衡":
		recommendations = append(recommendations, "资金面平衡，适合区间操作")
	}

	// Risk-based recommendations
	switch result.RiskAssessment.RiskLevel {
	case "高风险":
		recommendations = append(recommendations, "风险较高，建议控制仓位")
		recommendations = append(recommendations, fmt.Sprintf("建议止损价位: %.2f", result.RiskAssessment.StopLossLevel))
		recommendations = append(recommendations, fmt.Sprintf("建议止盈价位: %.2f", result.RiskAssessment.TakeProfitLevel))
	case "中风险":
		recommendations = append(recommendations, "风险适中，注意仓位管理")
		recommendations = append(recommendations, "建议分散投资降低风险")
	}

	// Pattern-based recommendations
	if result.PatternRecognition.NextMovePrediction == "趋势反转" {
		recommendations = append(recommendations, "趋势可能反转，注意观察确认信号")
	}

	if result.PatternRecognition.NextMovePrediction == "强势流入" {
		recommendations = append(recommendations, "预测强势流入，可考虑追涨")
	}

	// Fee structure-based recommendations
	if result.FeeStructure.ConcentrationRatio > 60 {
		recommendations = append(recommendations, "资金集中度较高，注意主力动向")
	}

	// Ensure we have some recommendations
	if len(recommendations) == 0 {
		recommendations = append(recommendations, "当前资金面平稳，保持正常监控")
	}

	return recommendations
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	var sum float64
	for _, v := range values {
		sum += v
	}

	return sum / float64(len(values))
}

func calculateMovingAverage(dailyFlows map[string]*DailyCapitalFlow, dates []string, window int) float64 {
	if len(dates) < window {
		return 0
	}

	var sum float64
	for i := 0; i < window; i++ {
		sum += dailyFlows[dates[i]].MainNetFlow
	}

	return sum / float64(window)
}

func calculatePercentile(value float64, values []float64) float64 {
	if len(values) == 0 {
		return 50.0 // Default to 50th percentile
	}

	sort.Float64s(values)

	// Count values less than or equal to the target
	lessOrEqual := 0
	for _, v := range values {
		if v <= value {
			lessOrEqual++
		}
	}

	return float64(lessOrEqual) / float64(len(values)) * 100
}

func containsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}