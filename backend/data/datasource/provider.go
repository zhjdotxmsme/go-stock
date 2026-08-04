// Package datasource provides a unified data access layer with priority-based fallback chains.
//
// Deprecated: the interfaces and data types are being migrated to
// go-stock/backend/internal/port/datasource. This package now re-exports those
// definitions for backward compatibility during the transition.
package datasource

import (
	"errors"

	"go-stock/backend/internal/port/datasource"
)

// Re-export data types and interfaces from the port layer.
type (
	DataType            = datasource.DataType
	DataSourceProvider  = datasource.DataSourceProvider
	QuoteProvider       = datasource.QuoteProvider
	KLineProvider       = datasource.KLineProvider
	NewsProvider        = datasource.NewsProvider
	FundamentalProvider = datasource.FundamentalProvider
	SectorProvider      = datasource.SectorProvider

	QuoteData       = datasource.QuoteData
	KLineData       = datasource.KLineData
	KLineBar        = datasource.KLineBar
	NewsItem        = datasource.NewsItem
	FundamentalData = datasource.FundamentalData
	SectorData      = datasource.SectorData
)

// Re-export constants.
const (
	DataTypeQuote       = datasource.DataTypeQuote
	DataTypeKLine       = datasource.DataTypeKLine
	DataTypeNews        = datasource.DataTypeNews
	DataTypeFundamental = datasource.DataTypeFundamental
	DataTypeSector      = datasource.DataTypeSector
)

// NormalizePeriod maps common named periods to numeric codes used by data sources.
// Passes through numeric codes unchanged.
func NormalizePeriod(period string) string {
	switch period {
	case "day":
		return "101"
	case "week":
		return "102"
	case "month":
		return "103"
	case "quarter":
		return "104"
	case "year", "annual":
		return "105"
	case "1min", "1m":
		return "1"
	case "5min", "5m":
		return "5"
	case "15min", "15m":
		return "15"
	case "30min", "30m":
		return "30"
	case "60min", "60m":
		return "60"
	default:
		return period
	}
}

// --- Error types ---

var (
	ErrAllSourcesFailed = errors.New("all data sources failed")
	ErrNoProvider       = errors.New("no provider registered for data type")
)

