// Package datasource defines the outbound data-source ports for the application layer.
// Adapters in backend/internal/adapter/datasource implement these interfaces.
package datasource

import (
	"context"
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
	DataTypeSnapshot    DataType = "snapshot"
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

// QuoteData represents a real-time quote snapshot.
type QuoteData struct {
	Code      string                 `json:"code"`
	Name      string                 `json:"name"`
	Price     float64                `json:"price"`
	Change    float64                `json:"change"`
	ChangePct float64                `json:"changePct"`
	Volume    int64                  `json:"volume"`
	Amount    float64                `json:"amount"`
	High      float64                `json:"high"`
	Low       float64                `json:"low"`
	Open      float64                `json:"open"`
	PrevClose float64                `json:"prevClose"`
	Time      time.Time              `json:"time"`
	Extra     map[string]interface{} `json:"extra,omitempty"`
}

// KLineData represents a series of K-line bars.
type KLineData struct {
	Code   string     `json:"code"`
	Period string     `json:"period"`
	Bars   []KLineBar `json:"bars"`
}

// KLineBar represents a single K-line bar.
type KLineBar struct {
	Time      time.Time `json:"time"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	PrevClose float64   `json:"prevClose"` // 前一交易日收盘价
	Volume    int64     `json:"volume"`
	Amount    float64   `json:"amount"`
}

// NewsItem represents a single news article.
type NewsItem struct {
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Source    string    `json:"source"`
	Time      time.Time `json:"time"`
	Sentiment float64   `json:"sentiment"` // -1 to 1
}

// FundamentalData represents fundamental/financial metrics.
type FundamentalData struct {
	PE        float64 `json:"pe"`
	PB        float64 `json:"pb"`
	ROE       float64 `json:"roe"`
	Revenue   float64 `json:"revenue"`
	NetProfit float64 `json:"netProfit"`
	DebtRatio float64 `json:"debtRatio"`
}

// SectorData represents sector/industry data.
type SectorData struct {
	Code       string  `json:"code"`
	Sector     string  `json:"sector"`
	Rank       int     `json:"rank"`
	FlowAmount float64 `json:"flowAmount"`
}

// SnapshotData is a rich real-time quote snapshot (level-1 market fields).
// Values are numeric; providers parse from their upstream representations.
type SnapshotData struct {
	Code     string    `json:"code"`
	Name     string    `json:"name"`
	Price    float64   `json:"price"`
	Open     float64   `json:"open"`
	PreClose float64   `json:"preClose"`
	High     float64   `json:"high"`
	Low      float64   `json:"low"`
	A1P      float64   `json:"a1p"` // 卖一报价：停牌/无成交时作为价格回退
	B1P      float64   `json:"b1p"` // 买一报价：价格回退链在 PreClose 之前
	Time     time.Time `json:"time"`
}

// SnapshotProvider provides rich real-time snapshots.
type SnapshotProvider interface {
	DataSourceProvider
	GetSnapshot(ctx context.Context, code string) (*SnapshotData, error)
}
