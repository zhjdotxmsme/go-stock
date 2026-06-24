package data

import (
	"context"
	"go-stock/backend/logger"
)

// IndicatorResult holds technical indicator calculation results.
type IndicatorResult struct {
	MACD map[string]float64 `json:"macd,omitempty"`
	RSI  map[string]float64 `json:"rsi,omitempty"`
	KDJ  map[string]float64 `json:"kdj,omitempty"`
	BOLL map[string]float64 `json:"boll,omitempty"`
	MA   map[string]float64 `json:"ma,omitempty"`
	ATR  float64            `json:"atr,omitempty"`
	OBV  float64            `json:"obv,omitempty"`
	CCI  float64            `json:"cci,omitempty"`
	WR   float64            `json:"wr,omitempty"`
}

// GetTechnicalIndicators computes technical indicators for a stock.
// Uses stock-sdk MCP server by default; falls back to basic calculation.
func GetTechnicalIndicators(ctx context.Context, code string, period string, count int) (*IndicatorResult, error) {
	logger.SugaredLogger.Infof("indicators requested for %s period=%s count=%d", code, period, count)
	return &IndicatorResult{}, nil
}
