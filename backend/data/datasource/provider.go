// Package datasource provides a unified data access layer with priority-based fallback chains.
package datasource

import (
	"context"
	"errors"
	"time"
)

// DataType enumerates the types of financial data.
type DataType string

const (
	DataTypeQuote       DataType = "quote"
	DataTypeKLine       DataType = "kline"
	DataTypeNews        DataType = "news"
	DataTypeFundamental DataType = "fundamental"
	DataTypeSector      DataType = "sector"
)

// DataSourceProvider is the interface every data source must implement.
type DataSourceProvider interface {
	// Name returns a human-readable name (e.g. "tdx", "eastmoney").
	Name() string
	// Priority returns the priority order (lower = higher priority).
	Priority() int
	// Available checks if this data source is currently reachable.
	Available(ctx context.Context) bool
}

// QuoteProvider provides real-time quote data.
type QuoteProvider interface {
	DataSourceProvider
	GetQuote(ctx context.Context, code string) (*QuoteData, error)
}

// KLineProvider provides K-line data.
type KLineProvider interface {
	DataSourceProvider
	GetKLine(ctx context.Context, code string, period string, count int) (*KLineData, error)
}

// NewsProvider provides news data.
type NewsProvider interface {
	DataSourceProvider
	GetNews(ctx context.Context, code string, count int) ([]NewsItem, error)
}

// FundamentalProvider provides fundamental/financial data.
type FundamentalProvider interface {
	DataSourceProvider
	GetFundamental(ctx context.Context, code string) (*FundamentalData, error)
}

// SectorProvider provides sector/industry data.
type SectorProvider interface {
	DataSourceProvider
	GetSectorData(ctx context.Context, code string) (*SectorData, error)
}

// --- Data types ---

type QuoteData struct {
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Price     float64   `json:"price"`
	Change    float64   `json:"change"`
	ChangePct float64   `json:"changePct"`
	Volume    int64     `json:"volume"`
	Amount    float64   `json:"amount"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Open      float64   `json:"open"`
	PrevClose float64   `json:"prevClose"`
	Time      time.Time `json:"time"`
}

type KLineData struct {
	Code   string     `json:"code"`
	Period string     `json:"period"`
	Bars   []KLineBar `json:"bars"`
}

type KLineBar struct {
	Time   time.Time `json:"time"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	PrevClose float64   `json:"prevClose"` // 前一交易日收盘价
	Volume    int64     `json:"volume"`
	Amount float64   `json:"amount"`
}

type NewsItem struct {
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Source    string    `json:"source"`
	Time      time.Time `json:"time"`
	Sentiment float64   `json:"sentiment"` // -1 to 1
}

type FundamentalData struct {
	PE        float64 `json:"pe"`
	PB        float64 `json:"pb"`
	ROE       float64 `json:"roe"`
	Revenue   float64 `json:"revenue"`
	NetProfit float64 `json:"netProfit"`
	DebtRatio float64 `json:"debtRatio"`
}

type SectorData struct {
	Code       string  `json:"code"`
	Sector     string  `json:"sector"`
	Rank       int     `json:"rank"`
	FlowAmount float64 `json:"flowAmount"`
}

// --- Error types ---

var (
	ErrAllSourcesFailed = errors.New("all data sources failed")
	ErrNoProvider       = errors.New("no provider registered for data type")
)
