package data

import (
	"testing"

	"go-stock/backend/models"
)

func TestApplyFilters(t *testing.T) {
	picks := []models.DailyPick{
		{Score: 80, ClosePrice: 15.0, Volume: 1000000, Rsi14: 65, TurnoverFactor: 0.5},
		{Score: 60, ClosePrice: 10.0, Volume: 500000, Rsi14: 55, TurnoverFactor: 0.7},
		{Score: 40, ClosePrice: 5.0, Volume: 200000, Rsi14: 30, TurnoverFactor: 0.3},
	}

	t.Run("filter score > 50", func(t *testing.T) {
		result := applyFilters(picks, []models.FilterCondition{
			{Field: "score", Op: ">", Value: 50},
		})
		if len(result) != 2 {
			t.Fatalf("expected 2, got %d", len(result))
		}
		if result[0].Score != 80 || result[1].Score != 60 {
			t.Fatalf("unexpected scores: %v", result)
		}
	})

	t.Run("filter score < 70", func(t *testing.T) {
		result := applyFilters(picks, []models.FilterCondition{
			{Field: "score", Op: "<", Value: 70},
		})
		if len(result) != 2 {
			t.Fatalf("expected 2, got %d", len(result))
		}
	})

	t.Run("filter score >= 60", func(t *testing.T) {
		result := applyFilters(picks, []models.FilterCondition{
			{Field: "score", Op: ">=", Value: 60},
		})
		if len(result) != 2 {
			t.Fatalf("expected 2, got %d", len(result))
		}
	})

	t.Run("multiple filters: score>50 AND price<12", func(t *testing.T) {
		result := applyFilters(picks, []models.FilterCondition{
			{Field: "score", Op: ">", Value: 50},
			{Field: "price", Op: "<", Value: 12},
		})
		if len(result) != 1 {
			t.Fatalf("expected 1, got %d", len(result))
		}
		if result[0].ClosePrice != 10.0 {
			t.Fatalf("expected price 10.0, got %.1f", result[0].ClosePrice)
		}
	})

	t.Run("filter rsi14 < 40", func(t *testing.T) {
		result := applyFilters(picks, []models.FilterCondition{
			{Field: "rsi14", Op: "<", Value: 40},
		})
		if len(result) != 1 {
			t.Fatalf("expected 1, got %d", len(result))
		}
	})

	t.Run("empty result", func(t *testing.T) {
		result := applyFilters(picks, []models.FilterCondition{
			{Field: "score", Op: ">", Value: 100},
		})
		if len(result) != 0 {
			t.Fatalf("expected 0, got %d", len(result))
		}
	})

	t.Run("no filters returns original", func(t *testing.T) {
		result := applyFilters(picks, nil)
		if len(result) != 3 {
			t.Fatalf("expected 3, got %d", len(result))
		}
	})
}

func TestGetFilterFieldValue(t *testing.T) {
	p := models.DailyPick{
		Score:         75,
		ClosePrice:    20.5,
		Volume:        2000000,
		Rsi14:         45,
		Macd:          0.5,
		TurnoverFactor: 0.8,
	}

	tests := []struct {
		field string
		want  float64
	}{
		{"score", 75},
		{"price", 20.5},
		{"volume", 2000000},
		{"rsi14", 45},
		{"macd", 0.5},
		{"turnover", 0.8},
		{"unknown", 0},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			got := getFilterFieldValue(p, tt.field)
			if got != tt.want {
				t.Fatalf("getFilterFieldValue(%q) = %v, want %v", tt.field, got, tt.want)
			}
		})
	}
}

func TestCompareValues(t *testing.T) {
	tests := []struct {
		val    float64
		op     string
		target float64
		want   bool
	}{
		{10, ">", 5, true},
		{10, ">", 20, false},
		{10, "<", 20, true},
		{10, "<", 5, false},
		{10, ">=", 10, true},
		{10, ">=", 9, true},
		{10, "<=", 10, true},
		{10, "<=", 11, true},
		{10, "==", 10, true},
		{10, "==", 10.001, false},
		{0, "unknown", 10, true}, // default returns true
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := compareValues(tt.val, tt.op, tt.target)
			if got != tt.want {
				t.Fatalf("compareValues(%v, %q, %v) = %v, want %v", tt.val, tt.op, tt.target, got, tt.want)
			}
		})
	}
}
