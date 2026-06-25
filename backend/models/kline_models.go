package models

import (
	"time"

	"gorm.io/gorm"
)

// KLineBar 持久化 K 线（日线/周线/月线）
type KLineBar struct {
	ID        uint      `gorm:"primarykey"`
	StockCode string    `gorm:"index:idx_kline_code_period_date_adj,unique;size:20"`
	Period    string    `gorm:"index:idx_kline_code_period_date_adj,unique;size:10"` // day / week / month
	TradeDate string    `gorm:"index:idx_kline_code_period_date_adj,unique;size:10"` // YYYY-MM-DD
	Adjusted  bool      `gorm:"index:idx_kline_code_period_date_adj,unique"`         // 是否前复权
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    int64
	Amount    float64
	Source    string    `gorm:"size:20"` // tdx / tencent / eastmoney / seed
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (KLineBar) TableName() string { return "kline_bars" }

// KLineSyncLog 记录每只股票每个周期的同步进度
type KLineSyncLog struct {
	ID            uint      `gorm:"primarykey"`
	StockCode     string    `gorm:"index;size:20"`
	Period        string    `gorm:"size:10"`
	Adjusted      bool
	StartDate     string    `gorm:"size:10"`
	EndDate       string    `gorm:"size:10"`
	SyncedCount   int
	ExpectedCount int
	Status        string    `gorm:"size:20"` // pending / running / done / failed
	ErrorMsg      string    `gorm:"type:text"`
	UpdatedAt     time.Time
}

func (KLineSyncLog) TableName() string { return "kline_sync_log" }

// AiRecommendBacktest AI 推荐回测结果
type AiRecommendBacktest struct {
	gorm.Model
	AiRecommendID uint    `gorm:"index"`
	StockCode     string  `gorm:"index;size:20"`
	StockName     string  `gorm:"size:50"`
	SignalDate    string  `gorm:"index;size:10"`
	SignalRating  string  `gorm:"size:10"`
	EntryPrice    float64
	ExitPrice     float64
	ExitDate      string  `gorm:"size:10"`
	HoldingDays   int
	TotalReturn   float64
	MaxDrawdown   float64
	Csi300Return  float64
	Alpha         float64
	Win           bool
	Source        string  `gorm:"size:20"`
}

func (AiRecommendBacktest) TableName() string { return "ai_recommend_backtests" }

// AiRecommendBacktestPageData 分页响应
type AiRecommendBacktestPageData struct {
	List       []AiRecommendBacktest `json:"list"`
	Total      int64                 `json:"total"`
	Page       int                   `json:"page"`
	PageSize   int                   `json:"pageSize"`
	TotalPages int                   `json:"totalPages"`
}