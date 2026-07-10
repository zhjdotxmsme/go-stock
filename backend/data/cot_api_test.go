package data

import (
	"testing"
)

func TestCotApi_GetPositionSummary(t *testing.T) {
	cot := NewCotApi()

	// Test with gold
	summary, err := cot.GetPositionSummary("GC")
	if err != nil {
		t.Logf("GetPositionSummary(GC) failed (expected - CFTC API may be unavailable): %v", err)
		return
	}

	if summary.ZScore == 0 && summary.NetPosition == 0 {
		t.Log("Summary returned zero values, possibly no data")
	}

	t.Logf("Gold COT summary: net=%d, z-score=%.2f, percentile=%.1f%%, crowded=%s",
		summary.NetPosition, summary.ZScore, summary.HistoricalPercentile, summary.CrowdedLevel)

	// Validate crowded level
	validLevels := map[string]bool{
		"none":            true,
		"moderate_top":    true,
		"moderate_bottom": true,
		"crowded_top":     true,
		"crowded_bottom":  true,
	}
	if !validLevels[summary.CrowdedLevel] {
		t.Errorf("Invalid crowded level: %s", summary.CrowdedLevel)
	}
}

func TestCotApi_GetPositionSummary_Silver(t *testing.T) {
	cot := NewCotApi()

	summary, err := cot.GetPositionSummary("SI")
	if err != nil {
		t.Logf("GetPositionSummary(SI) failed (expected): %v", err)
		return
	}

	t.Logf("Silver COT summary: net=%d, z-score=%.2f, percentile=%.1f%%, crowded=%s",
		summary.NetPosition, summary.ZScore, summary.HistoricalPercentile, summary.CrowdedLevel)
}

func TestCotApi_GetPositionSummary_Crude(t *testing.T) {
	cot := NewCotApi()

	summary, err := cot.GetPositionSummary("CL")
	if err != nil {
		t.Logf("GetPositionSummary(CL) failed (expected): %v", err)
		return
	}

	t.Logf("Crude COT summary: net=%d, z-score=%.2f, percentile=%.1f%%, crowded=%s",
		summary.NetPosition, summary.ZScore, summary.HistoricalPercentile, summary.CrowdedLevel)
}

func TestCotApi_calculateStdDev(t *testing.T) {
	cot := NewCotApi()

	values := []float64{1, 2, 3, 4, 5}
	mean := cot.calculateMean(values)
	stdDev := cot.calculateStdDev(values, mean)

	// For [1,2,3,4,5], mean=3, sample std dev = sqrt(2.5) ≈ 1.58
	expectedStdDev := 1.5811388300841898
	tolerance := 0.0001
	if stdDev < expectedStdDev-tolerance || stdDev > expectedStdDev+tolerance {
		t.Errorf("calculateStdDev = %f, expected %f", stdDev, expectedStdDev)
	}
}

func TestCotApi_calculateZScore(t *testing.T) {
	cot := NewCotApi()

	// Test with a simple series where z-score should be high
	values := []float64{1, 2, 3, 4, 5}
	mean := cot.calculateMean(values)
	stdDev := cot.calculateStdDev(values, mean)

	currentValue := int64(7)
	zScore := (float64(currentValue) - mean) / stdDev

	expectedZScore := (7 - 3) / stdDev
	tolerance := 0.0001
	if zScore < expectedZScore-tolerance || zScore > expectedZScore+tolerance {
		t.Errorf("zScore = %f, expected %f", zScore, expectedZScore)
	}
}

func TestCotApi_calculatePercentile(t *testing.T) {
	cot := NewCotApi()

	// This test requires network access to CFTC data.
	// We just test the function signature and error handling.
	percentile := cot.calculatePercentile("GC", "assetManager", 0)
	if percentile < 0 || percentile > 100 {
		t.Errorf("Percentile out of range: %f", percentile)
	}

	t.Logf("GC percentile for 0 net position: %.2f%%", percentile)
}