package backtest

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"

	"go-stock/backend/logger"
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// ParamRange defines one parameter's discrete values for grid search.
// Recognized names: "holdingDays", "stopLoss", "stopProfit".
type ParamRange struct {
	Name   string    `json:"name"`
	Values []float64 `json:"values"`
}

// ParamSpace is a collection of parameter ranges.
type ParamSpace struct {
	Ranges []ParamRange `json:"ranges"`
}

// ObjectiveConfig weights for the composite objective function.
// All weights are 0-1; the final score is a weighted sum of normalized metrics.
type ObjectiveConfig struct {
	SharpeWeight    float64 `json:"sharpeWeight"`
	WinRateWeight   float64 `json:"winRateWeight"`
	ReturnWeight    float64 `json:"returnWeight"`
	DrawdownPenalty float64 `json:"drawdownPenalty"`
}

// OptimizationInput is the request for parameter optimization.
type OptimizationInput struct {
	StockCode  string          `json:"stockCode"`
	StartDate  string          `json:"startDate"`
	EndDate    string          `json:"endDate"`
	Period     string          `json:"period"`
	Adjusted   bool            `json:"adjusted"`
	EntryPrice float64         `json:"entryPrice"`
	ParamSpace ParamSpace      `json:"paramSpace"`
	Objective  ObjectiveConfig `json:"objective"`
	TopN       int             `json:"topN"`
}

// OptimizationResult is one parameter combination's evaluation.
type OptimizationResult struct {
	Params         map[string]float64 `json:"params"`
	WinRate        float64            `json:"winRate"`
	AvgReturn      float64            `json:"avgReturn"`
	TotalReturn    float64            `json:"totalReturn"`
	SharpeRatio    float64            `json:"sharpeRatio"`
	MaxDrawdown    float64            `json:"maxDrawdown"`
	TotalTrades    int                `json:"totalTrades"`
	ObjectiveScore float64            `json:"objectiveScore"`
}

// ---------------------------------------------------------------------------
// Defaults
// ---------------------------------------------------------------------------

func DefaultObjective() ObjectiveConfig {
	return ObjectiveConfig{
		SharpeWeight:    0.3,
		WinRateWeight:   0.3,
		ReturnWeight:    0.3,
		DrawdownPenalty: 0.1,
	}
}

// PresetParamSpaces provides ready-to-use parameter grids.
func PresetParamSpaces() map[string]ParamSpace {
	return map[string]ParamSpace{
		"保守": {
			Ranges: []ParamRange{
				{Name: "holdingDays", Values: []float64{5, 10, 20}},
				{Name: "stopLoss", Values: []float64{0.03, 0.05, 0.08}},
				{Name: "stopProfit", Values: []float64{0.10, 0.15, 0.20}},
			},
		},
		"均衡": {
			Ranges: []ParamRange{
				{Name: "holdingDays", Values: []float64{5, 10, 20, 40, 60}},
				{Name: "stopLoss", Values: []float64{0.03, 0.05, 0.08, 0.12}},
				{Name: "stopProfit", Values: []float64{0.10, 0.15, 0.20, 0.30}},
			},
		},
		"激进": {
			Ranges: []ParamRange{
				{Name: "holdingDays", Values: []float64{3, 5, 10, 20}},
				{Name: "stopLoss", Values: []float64{0.05, 0.08, 0.12, 0.15}},
				{Name: "stopProfit", Values: []float64{0.15, 0.20, 0.30, 0.50}},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Core
// ---------------------------------------------------------------------------

// RunGridSearch sweeps the parameter space and returns ranked results.
// Each combination is evaluated by running batch backtests across all trading
// days in [startDate, endDate]. Results are sorted by ObjectiveScore descending.
func RunGridSearch(ctx context.Context, in OptimizationInput) ([]OptimizationResult, error) {
	if in.Period == "" {
		in.Period = "day"
	}
	if in.TopN <= 0 {
		in.TopN = 10
	}
	if in.Objective.SharpeWeight == 0 && in.Objective.WinRateWeight == 0 &&
		in.Objective.ReturnWeight == 0 && in.Objective.DrawdownPenalty == 0 {
		in.Objective = DefaultObjective()
	}

	combos := generateGrid(in.ParamSpace)
	if len(combos) == 0 {
		return nil, fmt.Errorf("no parameter combinations generated")
	}
	if len(combos) > 256 {
		return nil, fmt.Errorf("too many combinations (%d), max 256 — reduce parameter ranges", len(combos))
	}

	// Signal dates are shared across all combos
	dates, err := generateSignalDates(ctx, in.StockCode, in.Period, in.StartDate, in.EndDate, in.Adjusted)
	if err != nil {
		return nil, fmt.Errorf("failed to get trading dates: %w", err)
	}
	if len(dates) == 0 {
		return nil, fmt.Errorf("no trading dates found for %s [%s - %s]", in.StockCode, in.StartDate, in.EndDate)
	}

	logger.SugaredLogger.Infof("grid search: %d combos × %d dates = %d evaluations", len(combos), len(dates), len(combos)*len(dates))

	results := make([]OptimizationResult, len(combos))

	// Parallel evaluation: up to 4 concurrent combos (SQLite read-safe)
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)

	for i, combo := range combos {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		sem <- struct{}{}
		wg.Add(1)

		go func(idx int, c map[string]float64) {
			defer wg.Done()
			defer func() { <-sem }()

			r := evaluateCombo(ctx, in.StockCode, in.EntryPrice, in.Adjusted, c, dates)
			results[idx] = r
		}(i, combo)
	}

	wg.Wait()

	// Sort by objective score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].ObjectiveScore > results[j].ObjectiveScore
	})

	// Truncate to top N
	if len(results) > in.TopN {
		results = results[:in.TopN]
	}

	if len(results) > 0 {
		logger.SugaredLogger.Infof("grid search done: best score=%.3f params=%v", results[0].ObjectiveScore, results[0].Params)
	}

	return results, nil
}

// evaluateCombo runs batch backtest for one parameter combination (no DB persistence).
func evaluateCombo(ctx context.Context, stockCode string, entryPrice float64, adjusted bool, combo map[string]float64, dates []string) OptimizationResult {
	holdingDays := int(combo["holdingDays"])
	if holdingDays <= 0 {
		holdingDays = 20
	}
	stopLoss := combo["stopLoss"]
	if stopLoss <= 0 {
		stopLoss = 0.08
	}
	stopProfit := combo["stopProfit"]
	if stopProfit <= 0 {
		stopProfit = 0.20
	}

	engine := NewEngine()
	var batchResults []*Result

	for _, signalDate := range dates {
		r, err := engine.Run(ctx, Input{
			StockCode:   stockCode,
			SignalDate:  signalDate,
			EntryPrice:  entryPrice,
			HoldingDays: holdingDays,
			StopLoss:    stopLoss,
			StopProfit:  stopProfit,
			Adjusted:    adjusted,
		})
		if err != nil {
			continue
		}
		batchResults = append(batchResults, r)
	}

	if len(batchResults) == 0 {
		return OptimizationResult{
			Params:         combo,
			ObjectiveScore: -999,
		}
	}

	br := aggregateResults(batchResults)

	return OptimizationResult{
		Params:         combo,
		WinRate:        br.WinRate,
		AvgReturn:      br.AvgReturn,
		TotalReturn:    br.TotalReturn,
		SharpeRatio:    br.SharpeRatio,
		MaxDrawdown:    br.MaxDrawdown,
		TotalTrades:    br.TotalTrades,
		ObjectiveScore: computeObjective(br, ObjectiveConfig{}),
	}
}

// ---------------------------------------------------------------------------
// Grid generation
// ---------------------------------------------------------------------------

func generateGrid(ps ParamSpace) []map[string]float64 {
	if len(ps.Ranges) == 0 {
		return nil
	}

	results := []map[string]float64{{}}

	for _, r := range ps.Ranges {
		if len(r.Values) == 0 {
			continue
		}
		var next []map[string]float64
		for _, existing := range results {
			for _, v := range r.Values {
				combo := make(map[string]float64, len(existing)+1)
				for k, val := range existing {
					combo[k] = val
				}
				combo[r.Name] = v
				next = append(next, combo)
			}
		}
		results = next
	}

	return results
}

// ---------------------------------------------------------------------------
// Objective function
// ---------------------------------------------------------------------------

func computeObjective(br *BatchResult, cfg ObjectiveConfig) float64 {
	if cfg.SharpeWeight == 0 && cfg.WinRateWeight == 0 &&
		cfg.ReturnWeight == 0 && cfg.DrawdownPenalty == 0 {
		cfg = DefaultObjective()
	}

	score := 0.0
	score += cfg.SharpeWeight * clamp((br.SharpeRatio+1)/4, 0, 1) // Sharpe [-1,3] → [0,1]
	score += cfg.WinRateWeight * br.WinRate                        // [0,1]
	score += cfg.ReturnWeight * clamp((br.AvgReturn+0.1)/0.2, 0, 1) // [-10%,10%] → [0,1]
	score -= cfg.DrawdownPenalty * br.MaxDrawdown                   // penalty

	return math.Round(score*1000) / 1000
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
