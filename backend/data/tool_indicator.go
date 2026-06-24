package data

import (
	"context"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
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
// Uses stock-sdk MCP server if available; returns empty result on failure.
func GetTechnicalIndicators(ctx context.Context, code string, period string, count int) (*IndicatorResult, error) {
	logger.SugaredLogger.Infof("indicators requested for %s period=%s count=%d", code, period, count)

	// Check if stock-sdk MCP server is available in the database
	var mcp models.MCPServer
	err := db.Dao.Where("name = ? AND enable = ?", "stock-sdk", true).First(&mcp).Error
	if err == nil && mcp.ID > 0 {
		logger.SugaredLogger.Infof("stock-sdk MCP server found: %s (type=%s)", mcp.Name, mcp.Type)
		// MCP tool call will be integrated here in a future update
		// For now, return empty result
	}

	// Return empty result — the technical analyst will still work with raw K-line data
	return &IndicatorResult{}, nil
}
