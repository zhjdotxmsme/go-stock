// backend/data/models/stock_info_extended.go
package models

import (
	"time"
)

// StockInfoExtended extends the original StockInfo with new fields from the layered architecture
type StockInfoExtended struct {
	// Original StockInfo fields (embedded for compatibility)
	StockInfo

	// New fields from layered architecture
	TSCode      string `json:"ts_code" gorm:"index"`
	Current     float64 `json:"current"`
	Change      float64 `json:"change"`
	PctChg      float64 `json:"pct_chg"`

	// Metadata from layered architecture
	Latency    int64  `json:"latency"`     // Request latency in milliseconds
	DataSource  string `json:"data_source"` // Which data source was used
	Cached      bool   `json:"cached"`       // Whether data came from cache
	Version     string `json:"version"`      // API version used
	RequestTime time.Time `json:"request_time"` // When the request was made
}

// TableName specifies the table name for StockInfoExtended
func (StockInfoExtended) TableName() string {
	return "stock_info_extended"
}

// ToMap converts StockInfoExtended to a map for JSON serialization
func (s *StockInfoExtended) ToMap() map[string]any {
	return map[string]any{
		"ts_code":        s.TSCode,
		"code":           s.Code,
		"name":           s.Name,
		"current":        s.Current,
		"change":         s.Change,
		"pct_chg":        s.PctChg,
		"latency":        s.Latency,
		"data_source":    s.DataSource,
		"cached":         s.Cached,
		"version":        s.Version,
		"request_time":   s.RequestTime,
		"date":           s.Date,
		"time":           s.Time,
		"pre_price":      s.PrePrice,
		"price":          s.Price,
		"volume":         s.Volume,
		"amount":         s.Amount,
		"open":           s.Open,
		"pre_close":      s.PreClose,
		"high":           s.High,
		"low":            s.Low,
		"bid":            s.Bid,
		"ask":            s.Ask,
		"change_percent": s.ChangePercent,
		"change_price":   s.ChangePrice,
	}
}