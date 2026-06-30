package data

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/slice"
)

// Default scoring weights
const (
	WeightVolume = 0.20 // 成交量放大因子权重
	WeightMA     = 0.20 // 均线形态因子权重
	WeightRSI    = 0.15 // RSI 因子权重
	WeightMACD   = 0.15 // MACD 因子权重
	WeightPrice  = 0.15 // 价格位置因子权重
	WeightTurn   = 0.15 // 换手率因子权重
)

// DailyPickEngine handles the daily stock screening and scoring.
type DailyPickEngine struct {
	maxWorkers int
	strategies []ScoringStrategy

	// Pre-fetched data (populated once per RunDailyPick)
	macroScore        float64               // 0-1 macro environment score
	industryRankMap   map[string]float64    // industryName → score (0-1)
	stockIndustryMap  map[string]string     // stockCode → industryName
	researchReportMap map[string]int        // stockCode → report count (pre-fetched)
}

// NewDailyPickEngine creates a new engine instance with default strategies.
func NewDailyPickEngine() *DailyPickEngine {
	return &DailyPickEngine{
		maxWorkers: 10,
		strategies: []ScoringStrategy{
			&MATrendStrategy{},
			&OversoldReversalStrategy{},
			&MomentumStrategy{},
			&ChannelBreakoutStrategy{},
			&KDJShortStrategy{},
			&IndustryStrengthStrategy{},
			&ResearchReportStrategy{},
			&MacroEnvironmentStrategy{},
		},
	}
}

// WithStrategies replaces the default strategy list. Useful for testing.
func (e *DailyPickEngine) WithStrategies(s []ScoringStrategy) *DailyPickEngine {
	e.strategies = s
	return e
}

// RunDailyPick performs the full daily pick flow:
//  1. Get candidate stock pool
//  2. Parallel fetch K-line data and compute indicators
//  3. Multi-factor scoring
//  4. Take top N and persist to database
//
// The ctx.TradeDate is the date whose closing data to analyze (typically today).
func (e *DailyPickEngine) RunDailyPick(ctx context.Context, tradeDate string, topN int) ([]models.DailyPick, error) {
	if topN <= 0 {
		topN = 5
	}

	// Step 1: Get candidate stocks
	candidates := e.getCandidateStocks(ctx, tradeDate)
	if len(candidates) == 0 {
		logger.SugaredLogger.Warn("daily_pick: no candidates found, trying East Money pre-filter")
		candidates = e.getCandidatesFromEastMoney(ctx)
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("daily_pick: no candidate stocks found for %s", tradeDate)
	}
	logger.SugaredLogger.Infof("daily_pick: %d candidates to score", len(candidates))

	// Pre-fetch macro, industry, and research data
	e.prefetchMacroData()
	e.prefetchIndustryRankings(ctx)
	e.prefetchResearchData(ctx, candidates)

	// Step 2: Parallel score computation
	type scored struct {
		pick models.DailyPick
		err  error
	}

	var (
		mu     sync.Mutex
		wg     sync.WaitGroup
		sem    = make(chan struct{}, e.maxWorkers)
		result []scored
	)

	for _, c := range candidates {
		wg.Add(1)
		sem <- struct{}{}
		go func(candidate stockCandidate) {
			defer wg.Done()
			defer func() { <-sem }()

			p, err := e.scoreStock(ctx, candidate, tradeDate)
			mu.Lock()
			result = append(result, scored{pick: p, err: err})
			mu.Unlock()
		}(c)
	}
	wg.Wait()

	// Step 3: Filter successful scores and rank
	var picks []models.DailyPick
	for _, r := range result {
		if r.err != nil {
			logger.SugaredLogger.Debugf("daily_pick: skip %s: %v", r.pick.StockCode, r.err)
			continue
		}
		if r.pick.Score > 0 {
			picks = append(picks, r.pick)
		}
	}

	sort.Slice(picks, func(i, j int) bool {
		return picks[i].Score > picks[j].Score
	})

	if len(picks) > topN {
		picks = picks[:topN]
	}
	for i := range picks {
		picks[i].Rank = i + 1
	}

	logger.SugaredLogger.Infof("daily_pick: %d picks generated for %s", len(picks), tradeDate)

	// Step 4: Persist to database
	for i := range picks {
		if err := db.Dao.WithContext(ctx).Create(&picks[i]).Error; err != nil {
			logger.SugaredLogger.Errorf("daily_pick: failed to save pick %s: %v", picks[i].StockCode, err)
		}
	}

	return picks, nil
}

// stockCandidate holds minimal info about a candidate stock.
type stockCandidate struct {
	Code string
	Name string
}

// getCandidateStocks queries the local all_stock_info table for A-share candidates.
func (e *DailyPickEngine) getCandidateStocks(ctx context.Context, tradeDate string) []stockCandidate {
	var infos []models.AllStockInfo

	// Query A-share stocks: SH/SZ exchanges, non-ST, active
	err := db.Dao.WithContext(ctx).
		Where("(secucode LIKE ? OR secucode LIKE ?)", "%.SH", "%.SZ").
		Where("secucode NOT LIKE ?", "688%").      // exclude 科创板
		Where("sec_uri_tynameabbr NOT LIKE ?", "%ST%").
		Where("sec_uri_tynameabbr NOT LIKE ?", "%退%").
		Find(&infos).Error

	if err != nil {
		logger.SugaredLogger.Errorf("daily_pick: query all_stock_info error: %v", err)
		return nil
	}

	return slice.Map(infos, func(_ int, info models.AllStockInfo) stockCandidate {
		return stockCandidate{
			Code: info.SECUCODE,
			Name: info.SECURITYNAMEABBR,
		}
	})
}

// getCandidatesFromEastMoney uses the East Money search API as an alternative pre-filter.
func (e *DailyPickEngine) getCandidatesFromEastMoney(ctx context.Context) []stockCandidate {
	// Use the existing SearchStockApi with a broad condition
	searchWords := "沪深主板,创业板;换手率大于1%,量比大于1,涨幅大于0%小于8%,股价大于5小于200;不要ST股,不要退市股,非北交所,非科创板"
	api := NewSearchStockApi(searchWords)
	res := api.SearchStock(200)
	if convertor.ToString(res["code"]) != "100" {
		return nil
	}
	resData, ok := res["data"].(map[string]any)
	if !ok {
		return nil
	}
	result, ok := resData["result"].(map[string]any)
	if !ok {
		return nil
	}
	dataList, ok := result["dataList"].([]any)
	if !ok || len(dataList) == 0 {
		return nil
	}

	columns, ok := result["columns"].([]any)
	if !ok {
		return nil
	}

	// Find code and name column indices
	codeIdx, nameIdx := -1, -1
	for i, col := range columns {
		c := col.(map[string]any)
		if c["key"] == "SECUCODE" || c["key"] == "CODE" || c["key"] == "stockCode" {
			codeIdx = i
		}
		if c["key"] == "SECURITY_NAME_ABBR" || c["key"] == "NAME" || c["key"] == "stockName" {
			nameIdx = i
		}
	}
	if codeIdx < 0 && nameIdx < 0 {
		// Fallback: try common indices
		codeIdx = 1
		nameIdx = 2
	}
	if codeIdx < 0 {
		return nil
	}

	var candidates []stockCandidate
	for _, item := range dataList {
		d := item.(map[string]any)
		code := fmt.Sprintf("%v", d[columns[codeIdx].(map[string]any)["key"].(string)])
		name := ""
		if nameIdx >= 0 {
			name = fmt.Sprintf("%v", d[columns[nameIdx].(map[string]any)["key"].(string)])
		}
		if code != "" && code != "<nil>" {
			candidates = append(candidates, stockCandidate{Code: code, Name: name})
		}
	}
	return candidates
}

// scoreStock computes all indicators and returns a scored DailyPick record.
func (e *DailyPickEngine) scoreStock(ctx context.Context, candidate stockCandidate, tradeDate string) (models.DailyPick, error) {
	pick := models.DailyPick{
		StockCode: candidate.Code,
		StockName: candidate.Name,
		TradeDate: tradeDate,
	}

	// Normalize stock code for API call
	apiCode := normalizeCode(candidate.Code)

	// Fetch K-line data via Sina API (scale=240 = daily).
	// Historical date path (EastMoney K-line) is removed because
	// EastMoney push API is blocked from this network.
	klineData := NewStockDataApi().GetKLineData(apiCode, "240", 60)
	if klineData == nil || len(*klineData) < 20 {
		return pick, fmt.Errorf("insufficient kline data: %d bars", lenPtr(klineData))
	}

	klines := *klineData
	n := len(klines)

	// Extract price arrays
	closeP := make([]float64, n)
	highP := make([]float64, n)
	lowP := make([]float64, n)
	volume := make([]float64, n)

	for i, k := range klines {
		closeP[i] = parseFloat64(k.Close)
		highP[i] = parseFloat64(k.High)
		lowP[i] = parseFloat64(k.Low)
		volume[i] = parseFloat64(k.Volume)
	}

	// Today's data
	today := klines[n-1]
	pick.ClosePrice = parseFloat64(today.Close)
	pick.OpenPrice = parseFloat64(today.Open)
	pick.HighPrice = parseFloat64(today.High)
	pick.LowPrice = parseFloat64(today.Low)
	pick.Volume = int64(parseFloat64(today.Volume))
	pick.TurnoverRate = parseFloat64(today.TurnoverRate)
	pick.ChangePercent = parseFloat64(today.ChangePercent)

	// Check basic filters
	if pick.ClosePrice < 5 || pick.ClosePrice > 200 {
		return pick, fmt.Errorf("price %.2f out of range [5,200]", pick.ClosePrice)
	}
	if pick.ChangePercent > 9.5 || pick.ChangePercent < -5 {
		return pick, fmt.Errorf("change %.2f%% too extreme", pick.ChangePercent)
	}

	// Compute technical indicators
	if n >= 5 {
		pick.Ma5 = calcSMA(closeP, 5)
	}
	if n >= 10 {
		pick.Ma10 = calcSMA(closeP, 10)
	}
	if n >= 20 {
		pick.Ma20 = calcSMA(closeP, 20)
	}
	if n >= 60 {
		pick.Ma60 = calcSMA(closeP, 60)
	}
	if n >= 26 {
		macd := calcMACD(closeP, 12, 26, 9)
		pick.Macd = macd["MACD"]
		pick.MacdSignal = macd["Signal"]
	}
	if n >= 14 {
		pick.Rsi14 = calcRSI(closeP, 14)
	}
	if n >= 9 {
		kdj := calcKDJ(highP, lowP, closeP, 9, 3)
		pick.KdjK = kdj["K"]
		pick.KdjD = kdj["D"]
		pick.KdjJ = kdj["J"]
	}
	if n >= 20 {
		boll := calcBOLL(closeP, 20, 2.0)
		pick.BollMid = boll["Mid"]
		pick.BollUp = boll["Up"]
		pick.BollDown = boll["Down"]
	}

	// Multi-factor baseline scoring
	pick.VolumeFactor = scoreVolumeFactor(volume, closeP)
	pick.MaFactor = scoreMAFactor(pick.Ma5, pick.Ma10, pick.Ma20)
	pick.RsiFactor = scoreRSIFactor(pick.Rsi14)
	pick.MacdFactor = scoreMACDFactor(pick.Macd, pick.MacdSignal)
	pick.PriceFactor = scorePriceFactor(pick.ClosePrice, pick.BollMid, pick.BollUp, pick.BollDown)
	pick.TurnoverFactor = scoreTurnoverFactor(pick.TurnoverRate)
	baselineScore := math.Round(
		(pick.VolumeFactor*WeightVolume+
			pick.MaFactor*WeightMA+
			pick.RsiFactor*WeightRSI+
			pick.MacdFactor*WeightMACD+
			pick.PriceFactor*WeightPrice+
			pick.TurnoverFactor*WeightTurn)*100*100) / 100

	// Multi-strategy scoring: run all registered strategies and pick the best
	// Look up industry and research data from pre-fetched maps
	industryName := e.stockIndustryMap[candidate.Code]
	industryRankScore := e.lookupIndustryRankScore(industryName)
	researchCount := e.researchReportMap[candidate.Code]

	strategyCtx := &StrategyContext{
		KLines:              klines,
		CloseP:              closeP,
		HighP:               highP,
		LowP:                lowP,
		Volume:              volume,
		StockCode:           candidate.Code,
		StockName:           candidate.Name,
		TradeDate:           tradeDate,
		IndustryCode:        industryName,
		IndustryRankScore:   industryRankScore,
		MacroScore:          e.macroScore,
		ResearchReportCount: researchCount,
	}

	pick.Score = baselineScore
	pick.Reason = buildReason(pick)
	pick.IndustryScore = industryRankScore
	pick.ResearchScore = float64(researchCount)
	pick.MacroScore = e.macroScore

	// Fundamental/industry/macro strategies add bonus on top of technical score,
	// while existing technical strategies compete for highest score.
	for _, s := range e.strategies {
		r := s.Score(strategyCtx)
		switch s.Code() {
		case "industry_strength", "research_report", "macro_environment":
			// Bonus overlay: add 0-20% of the fundamental score to the technical baseline
			bonus := r.Score * 0.15
			newScore := pick.Score + bonus
			if newScore > pick.Score {
				pick.Score = newScore
				pick.Reason = fmt.Sprintf("%s +%s加分", pick.Reason, s.Name())
			}
		default:
			// Technical strategies compete for highest score (existing behavior)
			if r.Score > pick.Score {
				pick.Score = r.Score
				pick.StrategyCode = s.Code()
				pick.StrategyName = s.Name()
				pick.Reason = fmt.Sprintf("[%s] %s", s.Name(), r.Signal)
			}
		}
	}

	return pick, nil
}

// ---- Multi-factor scoring functions (each returns 0-1) ----

// scoreVolumeFactor: compare today's volume to 20-day average volume
func scoreVolumeFactor(volume, closeP []float64) float64 {
	n := len(volume)
	if n < 21 {
		return 0
	}
	todayVol := volume[n-1]
	var sum float64
	for i := n - 21; i < n-1; i++ {
		sum += volume[i]
	}
	avgVol := sum / 20
	if avgVol <= 0 {
		return 0
	}
	ratio := todayVol / avgVol
	switch {
	case ratio >= 2.5:
		return 1.0
	case ratio >= 1.8:
		return 0.9
	case ratio >= 1.3:
		return 0.7
	case ratio >= 1.0:
		return 0.4
	case ratio >= 0.7:
		return 0.2
	default:
		return 0
	}
}

// scoreMAFactor: MA alignment score
func scoreMAFactor(ma5, ma10, ma20 float64) float64 {
	if ma5 <= 0 || ma10 <= 0 || ma20 <= 0 {
		return 0
	}
	// Multi-head alignment
	if ma5 > ma10 && ma10 > ma20 {
		return 1.0
	}
	// MA5 > MA20 but MA10 between them
	if ma5 > ma20 && ma10 > ma20 {
		return 0.6
	}
	// Only MA5 > MA10
	if ma5 > ma10 {
		return 0.3
	}
	return 0
}

// scoreRSIFactor: RSI in ideal range (40-70)
func scoreRSIFactor(rsi float64) float64 {
	if rsi <= 0 {
		return 0
	}
	switch {
	case rsi >= 45 && rsi <= 65:
		return 1.0
	case rsi >= 35 && rsi < 45:
		return 0.7
	case rsi > 65 && rsi <= 75:
		return 0.6
	case rsi >= 25 && rsi < 35:
		return 0.3
	default:
		return 0
	}
}

// scoreMACDFactor: MACD status score
func scoreMACDFactor(macd, signal float64) float64 {
	switch {
	case macd > 0 && macd > signal:
		return 1.0 // golden cross above zero
	case macd > 0:
		return 0.6 // above zero
	case macd > signal:
		return 0.4 // golden cross below zero
	default:
		return 0
	}
}

// scoreTurnoverFactor: turnover rate score — ideal range 3%-15%
func scoreTurnoverFactor(turnoverRate float64) float64 {
	if turnoverRate <= 0 {
		return 0
	}
	switch {
	case turnoverRate >= 3 && turnoverRate <= 10:
		return 1.0
	case turnoverRate >= 1.5 && turnoverRate < 3:
		return 0.7
	case turnoverRate > 10 && turnoverRate <= 20:
		return 0.6
	case turnoverRate > 20 && turnoverRate <= 30:
		return 0.3
	case turnoverRate < 1.5:
		return 0.1
	default:
		return 0
	}
}

// scorePriceFactor: price position relative to BOLL bands
func scorePriceFactor(price, bollMid, bollUp, bollDown float64) float64 {
	if bollMid <= 0 || price <= 0 {
		return 0
	}
	switch {
	case price >= bollMid && price <= bollUp:
		return 1.0 // 中轨到上轨之间，理想
	case price < bollMid && price >= bollMid*0.95:
		return 0.6 // 中轨附近偏下
	case price > bollUp && price <= bollUp*1.05:
		return 0.4 // 轻微突破上轨
	default:
		return 0
	}
}

// ---- Helpers ----

// normalizeCode converts stock codes to Sina API format.
// "600519.SH" → "sh600519", "000001.SZ" → "sz000001", "sh600519" → "sh600519"
func normalizeCode(code string) string {
	code = strings.ToLower(code)
	if strings.HasPrefix(code, "sh") || strings.HasPrefix(code, "sz") {
		return code
	}
	if strings.Contains(code, ".") {
		parts := strings.SplitN(code, ".", 2)
		if parts[1] == "sh" || parts[1] == "sz" {
			return parts[1] + parts[0]
		}
		return parts[0]
	}
	return code
}

func lenPtr(p *[]KLineData) int {
	if p == nil {
		return 0
	}
	return len(*p)
}

// buildReason generates a human-readable recommendation reason.
func buildReason(pick models.DailyPick) string {
	parts := []string{}
	if pick.MaFactor >= 0.8 {
		parts = append(parts, "均线多头排列")
	}
	if pick.VolumeFactor >= 0.7 {
		parts = append(parts, "成交量显著放大")
	} else if pick.VolumeFactor >= 0.4 {
		parts = append(parts, "成交量温和放大")
	}
	if pick.MacdFactor >= 0.8 {
		parts = append(parts, "MACD金叉且位于零轴上方")
	} else if pick.MacdFactor >= 0.5 {
		parts = append(parts, "MACD处于零轴上方")
	}
	if pick.RsiFactor >= 0.8 {
		parts = append(parts, "RSI处于强势区")
	}
	if pick.PriceFactor >= 0.8 {
		parts = append(parts, "价格位于BOLL中上轨之间")
	}
	if len(parts) == 0 {
		parts = append(parts, "综合技术面评分入选")
	}
	return strings.Join(parts, "，")
}

// prefetchMacroData fetches PMI/CPI/GDP data and computes a macro environment score (0-1).
// Called once per RunDailyPick before parallel scoring.
func (e *DailyPickEngine) prefetchMacroData() {
	api := NewMarketNewsApi()
	score := 0.5 // neutral baseline

	defer func() {
		if score < 0 {
			score = 0
		}
		if score > 1 {
			score = 1
		}
		e.macroScore = score
		logger.SugaredLogger.Infof("daily_pick: macro score computed = %.2f", score)
	}()

	pmiResp := api.GetPMI()
	if pmiResp != nil && len(pmiResp.PMIResult.Data) > 0 {
		pmi := pmiResp.PMIResult.Data[0].MAKEINDEX
		switch {
		case pmi > 52:
			score += 0.3
		case pmi > 50:
			score += 0.2
		case pmi < 48:
			score -= 0.2
		}
	}

	cpiResp := api.GetCPI()
	if cpiResp != nil && len(cpiResp.CPIResult.Data) > 0 {
		cpi := cpiResp.CPIResult.Data[0].NATIONALSAME
		if cpi >= 1 && cpi <= 3 {
			score += 0.15
		} else if cpi > 4 {
			score -= 0.1
		}
	}

	gdpResp := api.GetGDP()
	if gdpResp != nil && len(gdpResp.GDPResult.Data) > 0 {
		gdp := gdpResp.GDPResult.Data[0].SUMSAME
		switch {
		case gdp >= 5:
			score += 0.2
		case gdp >= 4:
			score += 0.1
		case gdp < 3:
			score -= 0.1
		}
	}

	logger.SugaredLogger.Infof("daily_pick: macro score computed = %.2f", e.macroScore)
}

// prefetchIndustryRankings fetches industry money-flow rankings and builds:
//   - industryRankMap: industryName → score (0-1)
//   - stockIndustryMap: stockCode → industryName
//
// Called once per RunDailyPick before parallel scoring.
func (e *DailyPickEngine) prefetchIndustryRankings(ctx context.Context) {
	// Build stockIndustryMap from database
	var infos []struct {
		SECUCODE  string
		INDUSTRY  string
	}
	err := db.Dao.WithContext(ctx).
		Model(&models.AllStockInfo{}).
		Where("secucode IS NOT NULL AND industry IS NOT NULL AND industry != ''").
		Select("secucode, industry").
		Find(&infos).Error
	if err != nil {
		logger.SugaredLogger.Errorf("daily_pick: query stock industry error: %v", err)
		e.stockIndustryMap = make(map[string]string)
		e.industryRankMap = make(map[string]float64)
		return
	}
	e.stockIndustryMap = make(map[string]string, len(infos))
	for _, row := range infos {
		e.stockIndustryMap[row.SECUCODE] = row.INDUSTRY
	}

	// Fetch industry rankings from Sina
	ranks := NewMarketNewsApi().GetIndustryMoneyRankSina("0", "netamount")
	e.industryRankMap = make(map[string]float64, len(ranks))
	for i, r := range ranks {
		name := convertor.ToString(r["name"])
		if name == "" {
			continue
		}
		score := 1.0 - float64(i)/float64(len(ranks))
		e.industryRankMap[name] = score
	}
	logger.SugaredLogger.Infof("daily_pick: %d industry rankings loaded", len(e.industryRankMap))
}

// prefetchResearchData batches research report fetching for all candidates.
// Called once per RunDailyPick before parallel scoring.
func (e *DailyPickEngine) prefetchResearchData(ctx context.Context, candidates []stockCandidate) {
	e.researchReportMap = make(map[string]int, len(candidates))

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, e.maxWorkers)

	for _, c := range candidates {
		wg.Add(1)
		sem <- struct{}{}
		go func(code string) {
			defer wg.Done()
			defer func() { <-sem }()

			done := make(chan int, 1)
			go func() {
				reports := NewMarketNewsApi().StockResearchReport(code, 30)
				if reports != nil {
					done <- len(reports)
				} else {
					done <- 0
				}
			}()

			var count int
			select {
			case count = <-done:
			case <-time.After(12 * time.Second):
				logger.SugaredLogger.Warnf("daily_pick: research report timeout for %s", code)
				count = 0
			case <-ctx.Done():
				count = 0
			}

			mu.Lock()
			e.researchReportMap[code] = count
			mu.Unlock()
		}(c.Code)
	}
	wg.Wait()
	logger.SugaredLogger.Infof("daily_pick: research reports pre-fetched for %d stocks", len(e.researchReportMap))
}

// lookupIndustryRankScore finds the rank score for an industry name.
// Tries exact match first, then substring fallback.
func (e *DailyPickEngine) lookupIndustryRankScore(industryName string) float64 {
	if industryName == "" || len(e.industryRankMap) == 0 {
		return 0
	}
	if score, ok := e.industryRankMap[industryName]; ok {
		return score
	}
	// Fallback: check if any key contains or is contained by the target name
	for key, score := range e.industryRankMap {
		if strings.Contains(industryName, key) || strings.Contains(key, industryName) {
			return score
		}
	}
	return 0
}

