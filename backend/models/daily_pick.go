package models

import (
	"time"

	"gorm.io/gorm"
)

// DailyPick 短线荐股 — 每日收盘后技术面选股推荐
type DailyPick struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	StockCode string `json:"stockCode" gorm:"uniqueIndex:idx_daily_pick_date_code;size:20"`
	StockName string `json:"stockName" gorm:"size:50"`
	TradeDate string `json:"tradeDate" gorm:"uniqueIndex:idx_daily_pick_date_code;size:10"` // YYYY-MM-DD 推荐基准日

	// 评分
	Score float64 `json:"score" gorm:"index"` // 综合评分 0-100
	Rank  int     `json:"rank"`               // 当日排名

	StrategyCode string `json:"strategyCode" gorm:"size:50;index"`
	StrategyName string `json:"strategyName" gorm:"size:50"`

	// 因子得分明细
	VolumeFactor   float64 `json:"volumeFactor"`   // 成交量放大因子 (0-1)
	MaFactor       float64 `json:"maFactor"`       // 均线形态因子 (0-1)
	RsiFactor      float64 `json:"rsiFactor"`      // RSI 因子 (0-1)
	MacdFactor     float64 `json:"macdFactor"`     // MACD 因子 (0-1)
	PriceFactor    float64 `json:"priceFactor"`    // 价格位置因子 (0-1)
	TurnoverFactor float64 `json:"turnoverFactor"` // 换手率因子 (0-1)

	// 推荐日行情快照
	ClosePrice    float64 `json:"closePrice"`
	OpenPrice     float64 `json:"openPrice"`
	HighPrice     float64 `json:"highPrice"`
	LowPrice      float64 `json:"lowPrice"`
	Volume        int64   `json:"volume"`
	TurnoverRate  float64 `json:"turnoverRate"`
	ChangePercent float64 `json:"changePercent"` // 当日涨跌幅

	// 技术指标快照
	Ma5        float64 `json:"ma5"`
	Ma10       float64 `json:"ma10"`
	Ma20       float64 `json:"ma20"`
	Ma60       float64 `json:"ma60"`
	Macd       float64 `json:"macd"`
	MacdSignal float64 `json:"macdSignal"`
	Rsi14      float64 `json:"rsi14"`
	KdjK       float64 `json:"kdjK"`
	KdjD       float64 `json:"kdjD"`
	KdjJ       float64 `json:"kdjJ"`
	BollMid    float64 `json:"bollMid"`
	BollUp     float64 `json:"bollUp"`
	BollDown   float64 `json:"bollDown"`

	// 次日复盘（复盘时填充）
	NextOpen       float64 `json:"nextOpen"`
	NextHigh       float64 `json:"nextHigh"`
	NextLow        float64 `json:"nextLow"`
	NextClose      float64 `json:"nextClose"`
	NextReturn     float64 `json:"nextReturn"`     // 次日收益率（开盘买入→收盘卖出）
	NextMaxReturn  float64 `json:"nextMaxReturn"`  // 次日最大收益率
	NextMaxDrawdown float64 `json:"nextMaxDrawdown"` // 次日最大回撤
	Reviewed       bool    `json:"reviewed" gorm:"default:false;index"` // 是否已复盘

	// 备注
	Reason  string `json:"reason" gorm:"size:500"`
	Remarks string `json:"remarks" gorm:"size:500"`
}

func (DailyPick) TableName() string {
	return "daily_picks"
}

// DailyPickQuery 分页查询参数
type DailyPickQuery struct {
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
	TradeDate string `json:"tradeDate"` // 按日期筛选 YYYY-MM-DD
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	Reviewed  *bool  `json:"reviewed"` // nil=全部, true=已复盘, false=未复盘
}

// DailyPickPageData 分页响应
type DailyPickPageData struct {
	List       []DailyPick `json:"list"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"pageSize"`
	TotalPages int         `json:"totalPages"`
}

// DailyPickStats 统计汇总
type DailyPickStats struct {
	TotalPicks      int     `json:"totalPicks"`      // 总推荐次数
	ReviewedPicks   int     `json:"reviewedPicks"`   // 已复盘次数
	WinCount        int     `json:"winCount"`        // 盈利次数
	LossCount       int     `json:"lossCount"`       // 亏损次数
	WinRate         float64 `json:"winRate"`         // 胜率
	AvgReturn       float64 `json:"avgReturn"`       // 平均收益率
	TotalReturn     float64 `json:"totalReturn"`     // 累计收益率（复利）
	MaxReturn       float64 `json:"maxReturn"`       // 单日最高收益
	MaxDrawdown     float64 `json:"maxDrawdown"`     // 单日最大回撤
	AvgMaxReturn    float64 `json:"avgMaxReturn"`    // 平均最大收益
	AvgMaxDrawdown  float64 `json:"avgMaxDrawdown"`  // 平均最大回撤
}
