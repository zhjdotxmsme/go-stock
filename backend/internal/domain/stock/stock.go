package stock

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/plugin/soft_delete"
)

// StockChangeHistory 股票异动历史记录
type StockChangeHistory struct {
	ID         uint      `json:"id" gorm:"primarykey" md:"-"`
	ChangeTime string    `json:"changeTime" gorm:"uniqueIndex:idx_unique_change;size:10" md:"异动时间"`       // 异动时间 HH:MM:SS
	ChangeDate string    `json:"changeDate" gorm:"uniqueIndex:idx_unique_change;index;size:10" md:"异动日期"` // 异动日期 YYYY-MM-DD
	StockCode  string    `json:"stockCode" gorm:"uniqueIndex:idx_unique_change;index;size:20" md:"股票代码"`  // 股票代码
	StockName  string    `json:"stockName" gorm:"size:50" md:"股票名称"`                                      // 股票名称
	Market     int       `json:"market" md:"-"`                                                           // 市场
	ChangeType int       `json:"changeType" gorm:"uniqueIndex:idx_unique_change;index" md:"-"`            // 异动类型代码
	TypeName   string    `json:"typeName" gorm:"size:20" md:"异动类型名称"`                                     // 异动类型名称
	Volume     int64     `json:"volume" gorm:"uniqueIndex:idx_unique_change" md:"成交量(股)"`                 // 成交量(股)
	Price      float64   `json:"price" gorm:"uniqueIndex:idx_unique_change" md:"价格"`                      // 价格
	ChangeRate float64   `json:"changeRate" gorm:"uniqueIndex:idx_unique_change" md:" 涨跌幅(%)"`            // 涨跌幅(%)
	Amount     float64   `json:"amount" gorm:"uniqueIndex:idx_unique_change" md:"金额"`                     // 金额
	Industry   string    `json:"industry" gorm:"size:100" md:"所属行业"`                                      // 所属行业
	Concept    string    `json:"concept" gorm:"size:500" md:"所属概念"`                                       // 所属概念
	CreatedAt  time.Time `json:"createdAt" gorm:"autoCreateTime" md:"-"`
}

func (StockChangeHistory) TableName() string {
	return "stock_change_history"
}

// StockChangeHistoryQuery 异动查询参数
type StockChangeHistoryQuery struct {
	StockCode     string  `json:"stockCode"`
	StockName     string  `json:"stockName"`
	ChangeType    int     `json:"changeType"`
	ChangeTypes   []int   `json:"changeTypes"`
	TypeName      string  `json:"typeName"`
	StartDate     string  `json:"startDate"`
	EndDate       string  `json:"endDate"`
	StartTime     string  `json:"startTime"`
	EndTime       string  `json:"endTime"`
	MinVolume     int64   `json:"minVolume"`
	MinAmount     float64 `json:"minAmount"`
	MinChangeRate float64 `json:"minChangeRate"`
	MaxChangeRate float64 `json:"maxChangeRate"`
	Industry      string  `json:"industry"`
	Concept       string  `json:"concept"`
	Page          int     `json:"page"`
	PageSize      int     `json:"pageSize"`
}

// StockChangeHistoryPageData 异动分页数据
type StockChangeHistoryPageData struct {
	List       []StockChangeHistory `json:"list"`
	Total      int64                `json:"total"`
	Page       int                  `json:"page"`
	PageSize   int                  `json:"pageSize"`
	TotalPages int                  `json:"totalPages"`
}

// StockInfoHK 港股信息
type StockInfoHK struct {
	gorm.Model
	Code     string                `json:"code"`
	Name     string                `json:"name"`
	FullName string                `json:"fullName"`
	EName    string                `json:"eName"`
	IsDel    soft_delete.DeletedAt `gorm:"softDelete:flag"`
	BKName   string                `json:"bk_name"`
	BKCode   string                `json:"bk_code"`
}

func (StockInfoHK) TableName() string {
	return "stock_base_info_hk"
}

// StockInfoUS 美股信息
type StockInfoUS struct {
	gorm.Model
	Code     string                `json:"code"`
	Name     string                `json:"name"`
	FullName string                `json:"fullName"`
	EName    string                `json:"eName"`
	Exchange string                `json:"exchange"`
	Type     string                `json:"type"`
	IsDel    soft_delete.DeletedAt `gorm:"softDelete:flag"`
	BKName   string                `json:"bk_name"`
	BKCode   string                `json:"bk_code"`
}

func (StockInfoUS) TableName() string {
	return "stock_base_info_us"
}

// FollowedStock 自选股
type FollowedStock struct {
	StockCode          string
	Name               string
	Volume             int64
	CostPrice          float64
	Price              float64
	PriceChange        float64
	ChangePercent      float64
	AlarmChangePercent float64
	AlarmPrice         float64
	Time               time.Time
	Sort               int64
	Cron               *string
	IsDel              soft_delete.DeletedAt `gorm:"softDelete:flag"`
	Groups             []GroupStock          `gorm:"foreignKey:StockCode;references:StockCode"`
	AiConfigId         int
	EntryPrice         float64
	TakeProfitPrice    float64
	StopLossPrice      float64
}

func (FollowedStock) TableName() string {
	return "followed_stock"
}

// Group 股票分组
type Group struct {
	gorm.Model
	Name string `json:"name" gorm:"index"`
	Sort int    `json:"sort"`
}

func (Group) TableName() string {
	return "stock_groups"
}

// GroupStock 分组股票关联
type GroupStock struct {
	gorm.Model
	StockCode string `json:"stockCode" gorm:"index"`
	GroupId   int    `json:"groupId" gorm:"index"`
	GroupInfo Group  `json:"groupInfo" gorm:"foreignKey:GroupId;references:ID"`
}

func (GroupStock) TableName() string {
	return "group_stock_info"
}

// TradingRecord 交易日志
type TradingRecord struct {
	ID                 uint      `gorm:"primaryKey"`
	StockCode          string    `gorm:"index"`
	StockName          string
	Direction          string    `gorm:"index"` // 买入/卖出
	Price              float64
	Volume             int64
	Amount             float64   `gorm:"-"` // 计算字段: Price * Volume
	TradingTime        time.Time `gorm:"index"`
	Reason             string    `gorm:"type:text"`
	StopLossPrice      float64
	TakeProfitPrice    float64
	Fee                float64
	MarketValue        float64
	Mindset            string    `gorm:"type:text"`
	RecordedClosePrice float64   `json:"recordedClosePrice" gorm:"column:recorded_close_price"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (TradingRecord) TableName() string {
	return "trading_records"
}

// TradingRecordListQuery 交易日志列表查询
type TradingRecordListQuery struct {
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
	Keyword   string `json:"keyword"`   // 股票代码或名称模糊匹配
	Direction string `json:"direction"` // 买入 / 卖出，空表示全部
	StartDate string `json:"startDate"` // yyyy-MM-dd，交易时间起始（含当日 0 点）
	EndDate   string `json:"endDate"`   // yyyy-MM-dd，交易时间结束（含当日）
}

// TradingRecordPageData 交易日志分页结果
type TradingRecordPageData struct {
	List       []TradingRecordItem `json:"list"`
	Total      int64               `json:"total"`
	Page       int                 `json:"page"`
	PageSize   int                 `json:"pageSize"`
	TotalPages int                 `json:"totalPages"`
}

// TradingRecordItem 交易日志项（包含盈亏信息）
type TradingRecordItem struct {
	TradingRecord
	ClosePrice    float64 `json:"closePrice"`    // 收盘价或最新价
	ProfitAmount  float64 `json:"profitAmount"`  // 盈亏金额
	ProfitPercent float64 `json:"profitPercent"` // 盈亏收益率
}

// TradingRecordStatistics 交易统计
type TradingRecordStatistics struct {
	TotalBuyAmount  float64 `json:"totalBuyAmount"`
	TotalSellAmount float64 `json:"totalSellAmount"`
	TotalProfit     float64 `json:"totalProfit"`
	ProfitRate      float64 `json:"profitRate"`
	HoldingsAmount  float64 `json:"holdingsAmount"`
	CurrentValue    float64 `json:"currentValue"`
	StockCount      int64   `json:"stockCount"`
}
