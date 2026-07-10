package data

import (
	"testing"
)

func TestFredApi_GetTIPSRate(t *testing.T) {
	fred := NewFredApi()

	// Test getting latest TIPS rate
	tipsRate, err := fred.GetTIPSRate()
	if err != nil {
		t.Logf("GetTIPSRate failed (expected - no API key): %v", err)
		return
	}

	// Validate TIPS rate range
	if tipsRate < -5 || tipsRate > 10 {
		t.Errorf("TIPS rate out of reasonable range: %.4f%%", tipsRate)
	}

	t.Logf("TIPS rate: %.4f%%", tipsRate)
}

func TestFredApi_GetBreakEvenInflation(t *testing.T) {
	fred := NewFredApi()

	// Test getting break-even inflation
	beInflation, err := fred.GetBreakEvenInflation()
	if err != nil {
		t.Logf("GetBreakEvenInflation failed (expected - no API key): %v", err)
		return
	}

	// Validate break-even inflation range
	if beInflation < 0 || beInflation > 15 {
		t.Errorf("Break-even inflation out of reasonable range: %.4f%%", beInflation)
	}

	t.Logf("Break-even inflation: %.4f%%", beInflation)
}

func TestFredApi_CalculateRealRate(t *testing.T) {
	tests := []struct {
		name       string
		nominal    float64
		tips       float64
		expected   float64
	}{
		{"Positive real rate", 4.5, 2.0, 2.5},
		{"Negative real rate", 3.0, 3.5, -0.5},
		{"Zero real rate", 3.0, 3.0, 0.0},
		{"Zero TIPS", 3.0, 0.0, 3.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			realRate := CalculateRealRate(tt.nominal, tt.tips)
			if realRate != tt.expected {
				t.Errorf("CalculateRealRate(%.2f, %.2f) = %.2f, expected %.2f",
					tt.nominal, tt.tips, realRate, tt.expected)
			}
		})
	}
}

func TestFredApi_GetLatestValue(t *testing.T) {
	fred := NewFredApi()

	// Test with a known series (DGS10 - 10-Year Treasury Constant Maturity Rate)
	value, err := fred.GetLatestValue("DGS10")
	if err != nil {
		t.Logf("GetLatestValue(DGS10) failed (expected - no API key): %v", err)
		return
	}

	// Validate treasury yield range
	if value < 0 || value > 20 {
		t.Errorf("Treasury yield out of reasonable range: %.4f%%", value)
	}

	t.Logf("10-year Treasury yield: %.4f%%", value)
}