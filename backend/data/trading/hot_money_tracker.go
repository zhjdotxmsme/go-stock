// backend/data/trading/hot_money_tracker.go
package trading

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go-stock/backend/logger"
)

// HotMoneyTracker analyzes hot money seat patterns and trends
type HotMoneyTracker struct {
	config *HotMoneyConfig
	seatDB *SeatDatabase
}

// HotMoneyConfig configures hot money tracking behavior
type HotMoneyConfig struct {
	UpdateInterval time.Duration
	TrendLookback  int
	RankingThreshold float64
	ConsecutiveDays  int
}

// SeatDatabase maintains hot money seat information
type SeatDatabase struct {
	seats  map[string]*HotMoneySeat
	mu     sync.RWMutex
}

// HotMoneySeat represents a hot money trading seat
type HotMoneySeat struct {
	SeatName      string    `json:"seat_name"`
	TotalAmount   float64   `json:"total_amount"`
	SuccessRate   float64   `json:"success_rate"`
	Ranking       float64   `json:"ranking"`
	LastActive    time.Time `json:"last_active"`
	TopStocks     []string  `json:"top_stocks"`
	ConsecutiveDays int      `json:"consecutive_days"`
	MovementType  string    `json:"movement_type"`
}

// HotMoneyAnalysisResult contains hot money analysis results
type HotMoneyAnalysisResult struct {
	StockCode       string              `json:"stock_code"`
	AnalysisDate    time.Time           `json:"analysis_date"`
	TopSeats        []*HotMoneySeat     `json:"top_seats"`
	TotalHotAmount  float64             `json:"total_hot_amount"`
	SeatTrend       string              `json:"seat_trend"`
	RiskLevel       string              `json:"risk_level"`
	TrendDirection  string              `json:"trend_direction"`
	MovementPattern string              `json:"movement_pattern"`
	HistoricalData  []HotMoneySeatTrend `json:"historical_data"`
}

// HotMoneySeatTrend tracks seat performance over time
type HotMoneySeatTrend struct {
	Date     string  `json:"date"`
	SeatName string  `json:"seat_name"`
	Amount   float64 `json:"amount"`
	Ranking  float64 `json:"ranking"`
}

// NewHotMoneyTracker creates a new hot money tracker
func NewHotMoneyTracker(config *HotMoneyConfig) *HotMoneyTracker {
	if config == nil {
		config = &HotMoneyConfig{
			UpdateInterval: 24 * time.Hour,
			TrendLookback:  5,
			RankingThreshold: 0.8,
			ConsecutiveDays: 3,
		}
	}

	return &HotMoneyTracker{
		config: config,
		seatDB: &SeatDatabase{
			seats: make(map[string]*HotMoneySeat),
		},
	}
}

// AnalyzeHotMoney performs comprehensive hot money analysis
func (h *HotMoneyTracker) AnalyzeHotMoney(ctx context.Context, stockCode string, days int) (*HotMoneyAnalysisResult, error) {
	startTime := time.Now()

	// Fetch hot money data for the stock
	hotMoneyData, err := h.fetchHotMoneyData(ctx, stockCode, days)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch hot money data: %w", err)
	}

	// Identify top seats
	topSeats := h.identifyTopSeats(hotMoneyData)

	// Analyze seat trends
	trend := h.analyzeSeatTrend(topSeats)

	// Calculate risk level
	riskLevel := h.calculateRiskLevel(topSeats, trend)

	// Determine movement pattern
	movementPattern := h.determineMovementPattern(topSeats)

	// Get historical trend data
	historicalData := h.getHistoricalTrendData(stockCode, days)

	result := &HotMoneyAnalysisResult{
		StockCode:       stockCode,
		AnalysisDate:    time.Now(),
		TopSeats:        topSeats,
		TotalHotAmount:  h.calculateTotalHotAmount(topSeats),
		SeatTrend:       trend,
		RiskLevel:       riskLevel,
		TrendDirection:  h.determineTrendDirection(historicalData),
		MovementPattern: movementPattern,
		HistoricalData:  historicalData,
	}

	duration := time.Since(startTime)
	logger.SugaredLogger.Infof("Hot money analysis completed for %s in %v", stockCode, duration)

	return result, nil
}

// fetchHotMoneyData simulates fetching hot money seat data
func (h *HotMoneyTracker) fetchHotMoneyData(ctx context.Context, stockCode string, days int) (map[string][]*HotMoneySeat, error) {
	// In production, this would call actual hot money APIs
	// Simulating data for now
	simulatedData := make(map[string][]*HotMoneySeat)

	baseDate := time.Now()
	for i := 0; i < days; i++ {
		date := baseDate.AddDate(0, 0, -i)
		dateStr := date.Format("2006-01-02")

		// Simulate some hot money seats
		seats := []*HotMoneySeat{
			{
				SeatName:      "东方证券股份有限公司上海浦东新区源深路证券营业部",
				TotalAmount:   5000000.00,
				SuccessRate:   0.85,
				Ranking:       0.92,
				LastActive:    date,
				TopStocks:     []string{stockCode},
				ConsecutiveDays: 3,
				MovementType:  "流入",
			},
			{
				SeatName:      "中信证券股份有限公司上海淮海中路证券营业部",
				TotalAmount:   3500000.00,
				SuccessRate:   0.78,
				Ranking:       0.88,
				LastActive:    date,
				TopStocks:     []string{stockCode},
				ConsecutiveDays: 2,
				MovementType:  "流入",
			},
		}

		simulatedData[dateStr] = seats
	}

	return simulatedData, nil
}

// identifyTopSeats identifies the most significant hot money seats
func (h *HotMoneyTracker) identifyTopSeats(hotMoneyData map[string][]*HotMoneySeat) []*HotMoneySeat {
	seatAggregates := make(map[string]*HotMoneySeat)
	seatDates := make(map[string][]time.Time)

	// Aggregate data across all dates
	for date, seats := range hotMoneyData {
		parsedDate, _ := time.Parse("2006-01-02", date)

		for _, seat := range seats {
			if existing, exists := seatAggregates[seat.SeatName]; exists {
				existing.TotalAmount += seat.TotalAmount
				existing.SuccessRate = (existing.SuccessRate + seat.SuccessRate) / 2
				existing.Ranking = (existing.Ranking + seat.Ranking) / 2
				seatDates[seat.SeatName] = append(seatDates[seat.SeatName], parsedDate)
			} else {
				seatCopy := *seat
				seatAggregates[seat.SeatName] = &seatCopy
				seatDates[seat.SeatName] = []time.Time{parsedDate}
			}
		}
	}

	// Calculate consecutive days and filter by ranking threshold
	var topSeats []*HotMoneySeat
	for seatName, seat := range seatAggregates {
		dates := seatDates[seatName]
		seat.ConsecutiveDays = h.calculateConsecutiveDays(dates)
		seat.LastActive = dates[len(dates)-1]

		// Filter by ranking threshold
		if seat.Ranking >= h.config.RankingThreshold {
			topSeats = append(topSeats, seat)
		}
	}

	// Sort by ranking (highest first)
	h.sortSeatsByRanking(topSeats)

	// Return top 10 seats
	if len(topSeats) > 10 {
		return topSeats[:10]
	}

	return topSeats
}

// analyzeSeatTrend analyzes the trend of hot money seat activity
func (h *HotMoneyTracker) analyzeSeatTrend(seats []*HotMoneySeat) string {
	if len(seats) == 0 {
		return "无数据"
	}

	totalAmount := 0.0
	totalRanking := 0.0
	consecutiveSeats := 0

	for _, seat := range seats {
		totalAmount += seat.TotalAmount
		totalRanking += seat.Ranking

		if seat.ConsecutiveDays >= h.config.ConsecutiveDays {
			consecutiveSeats++
		}
	}

	avgAmount := totalAmount / float64(len(seats))
	avgRanking := totalRanking / float64(len(seats))

	// Determine trend based on multiple factors
	if avgRanking > 0.9 && consecutiveSeats > len(seats)/2 {
		return "强势流入"
	} else if avgRanking > 0.85 && consecutiveSeats > len(seats)/3 {
		return "持续流入"
	} else if avgRanking > 0.8 && avgAmount > 10000000 {
		return "大单流入"
	} else if avgRanking > 0.7 {
		return "中等流入"
	} else {
		return "资金分散"
	}
}

// calculateRiskLevel assesses the risk level based on hot money activity
func (h *HotMoneyTracker) calculateRiskLevel(seats []*HotMoneySeat, trend string) string {
	if len(seats) == 0 {
		return "低风险"
	}

	totalAmount := h.calculateTotalHotAmount(seats)
	highRankingSeats := 0

	for _, seat := range seats {
		if seat.Ranking > 0.9 {
			highRankingSeats++
		}
	}

	// Risk assessment logic
	if trend == "强势流入" && highRankingSeats > len(seats)/2 {
		if totalAmount > 50000000 {
			return "高风险"
		} else {
			return "中风险"
		}
	} else if trend == "持续流入" {
		return "中风险"
	} else if totalAmount > 30000000 {
		return "中风险"
	} else {
		return "低风险"
	}
}

// determineMovementPattern identifies the movement pattern
func (h *HotMoneyTracker) determineMovementPattern(seats []*HotMoneySeat) string {
	if len(seats) == 0 {
		return "无明显模式"
	}

	inflowCount := 0
	outflowCount := 0

	for _, seat := range seats {
		if seat.MovementType == "流入" {
			inflowCount++
		} else if seat.MovementType == "流出" {
			outflowCount++
		}
	}

	if inflowCount > outflowCount*2 {
		return "集中流入"
	} else if outflowCount > inflowCount*2 {
		return "集中流出"
	} else if inflowCount > outflowCount {
		return "净流入"
	} else if outflowCount > inflowCount {
		return "净流出"
	} else {
		return "资金平衡"
	}
}

// getHistoricalTrendData retrieves historical hot money trend data
func (h *HotMoneyTracker) getHistoricalTrendData(stockCode string, days int) []HotMoneySeatTrend {
	var trends []HotMoneySeatTrend

	baseDate := time.Now()
	for i := 0; i < days && i < h.config.TrendLookback; i++ {
		date := baseDate.AddDate(0, 0, -i)

		// Simulate historical trend data
		trends = append(trends, HotMoneySeatTrend{
			Date:     date.Format("2006-01-02"),
			SeatName: "东方证券上海浦东新区源深路",
			Amount:   5000000.00 - float64(i)*100000.00,
			Ranking:  0.92 - float64(i)*0.02,
		})
	}

	return trends
}

// determineTrendDirection determines the overall trend direction
func (h *HotMoneyTracker) determineTrendDirection(historicalData []HotMoneySeatTrend) string {
	if len(historicalData) < 2 {
		return "无法判断"
	}

	latest := historicalData[0]
	oldest := historicalData[len(historicalData)-1]

	amountChange := latest.Amount - oldest.Amount
	rankingChange := latest.Ranking - oldest.Ranking

	if amountChange > 0 && rankingChange > 0 {
		return "上升趋势"
	} else if amountChange < 0 && rankingChange < 0 {
		return "下降趋势"
	} else if amountChange > 0 {
		return "资金增长"
	} else if amountChange < 0 {
		return "资金减少"
	} else {
		return "平稳趋势"
	}
}

// calculateTotalHotAmount calculates total hot money amount
func (h *HotMoneyTracker) calculateTotalHotAmount(seats []*HotMoneySeat) float64 {
	total := 0.0
	for _, seat := range seats {
		total += seat.TotalAmount
	}
	return total
}

// calculateConsecutiveDays calculates consecutive active days
func (h *HotMoneyTracker) calculateConsecutiveDays(dates []time.Time) int {
	if len(dates) == 0 {
		return 0
	}

	// Sort dates in descending order
	for i := 0; i < len(dates); i++ {
		for j := i + 1; j < len(dates); j++ {
			if dates[i].Before(dates[j]) {
				dates[i], dates[j] = dates[j], dates[i]
			}
		}
	}

	consecutive := 1
	for i := 1; i < len(dates); i++ {
		if dates[i-1].Sub(dates[i]) <= 24*time.Hour {
			consecutive++
		} else {
			break
		}
	}

	return consecutive
}

// sortSeatsByRanking sorts seats by ranking in descending order
func (h *HotMoneyTracker) sortSeatsByRanking(seats []*HotMoneySeat) {
	for i := 0; i < len(seats); i++ {
		for j := i + 1; j < len(seats); j++ {
			if seats[j].Ranking > seats[i].Ranking {
				seats[i], seats[j] = seats[j], seats[i]
			}
		}
	}
}

// TrackHotMoneyInRealTime tracks hot money activity in real-time
func (h *HotMoneyTracker) TrackHotMoneyInRealTime(ctx context.Context, stockCodes []string) (map[string]*HotMoneyAnalysisResult, error) {
	results := make(map[string]*HotMoneyAnalysisResult)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, stockCode := range stockCodes {
		wg.Add(1)
		go func(code string) {
			defer wg.Done()

			result, err := h.AnalyzeHotMoney(ctx, code, 5)
			if err != nil {
				logger.SugaredLogger.Warnw("Failed to analyze hot money for stock",
					"stock_code", code, "error", err)
				return
			}

			mu.Lock()
			results[code] = result
			mu.Unlock()
		}(stockCode)
	}

	wg.Wait()
	return results, nil
}

// GetHotMoneyRankings returns hot money rankings for multiple stocks
func (h *HotMoneyTracker) GetHotMoneyRankings(ctx context.Context, stockCodes []string) ([]*HotMoneyRanking, error) {
	results, err := h.TrackHotMoneyInRealTime(ctx, stockCodes)
	if err != nil {
		return nil, fmt.Errorf("failed to track hot money: %w", err)
	}

	var rankings []*HotMoneyRanking
	for stockCode, analysis := range results {
		ranking := &HotMoneyRanking{
			StockCode:      stockCode,
			TotalAmount:    analysis.TotalHotAmount,
			Trend:          analysis.SeatTrend,
			RiskLevel:      analysis.RiskLevel,
			TopSeatCount:   len(analysis.TopSeats),
			AnalysisScore:  h.calculateAnalysisScore(analysis),
		}
		rankings = append(rankings, ranking)
	}

	// Sort by analysis score
	for i := 0; i < len(rankings); i++ {
		for j := i + 1; j < len(rankings); j++ {
			if rankings[j].AnalysisScore > rankings[i].AnalysisScore {
				rankings[i], rankings[j] = rankings[j], rankings[i]
			}
		}
	}

	return rankings, nil
}

// HotMoneyRanking represents a stock's hot money ranking
type HotMoneyRanking struct {
	StockCode      string  `json:"stock_code"`
	TotalAmount    float64 `json:"total_amount"`
	Trend          string  `json:"trend"`
	RiskLevel      string  `json:"risk_level"`
	TopSeatCount   int     `json:"top_seat_count"`
	AnalysisScore  float64 `json:"analysis_score"`
	Ranking        int     `json:"ranking"`
}

// calculateAnalysisScore calculates a comprehensive analysis score
func (h *HotMoneyTracker) calculateAnalysisScore(analysis *HotMoneyAnalysisResult) float64 {
	score := 0.0

	// Amount score (0-40 points)
	if analysis.TotalHotAmount > 50000000 {
		score += 40
	} else if analysis.TotalHotAmount > 30000000 {
		score += 30
	} else if analysis.TotalHotAmount > 10000000 {
		score += 20
	} else {
		score += 10
	}

	// Trend score (0-30 points)
	switch analysis.SeatTrend {
	case "强势流入":
		score += 30
	case "持续流入":
		score += 25
	case "大单流入":
		score += 20
	case "中等流入":
		score += 15
	default:
		score += 5
	}

	// Seat count score (0-20 points)
	topSeatCount := len(analysis.TopSeats)
	if topSeatCount > 5 {
		score += 20
	} else if topSeatCount > 3 {
		score += 15
	} else if topSeatCount > 1 {
		score += 10
	} else {
		score += 5
	}

	// Risk penalty (0-10 points)
	switch analysis.RiskLevel {
	case "高风险":
		score -= 10
	case "中风险":
		score -= 5
	default:
		// No penalty for low risk
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