package analysis

import (
	"time"

	"gorm.io/gorm"
)

// AiRecommendStocks AI 推荐股票记录（与 models.AiRecommendStocks 逐字段一致）。
type AiRecommendStocks struct {
	gorm.Model
	DataTime                    *time.Time `json:"dataTime" gorm:"index;autoCreateTime"`
	ModelName                   string     `json:"modelName"`
	Rating                      string     `json:"rating"`
	StockCode                   string     `json:"stockCode"`
	StockName                   string     `json:"stockName"`
	BkCode                      string     `json:"bkCode"`
	BkName                      string     `json:"bkName"`
	StockPrice                  string     `json:"stockPrice"`
	StockCurrentPrice           string     `json:"stockCurrentPrice"`
	StockCurrentPriceTime       string     `json:"stockCurrentPriceTime"`
	StockClosePrice             string     `json:"stockClosePrice"`
	StockPrePrice               string     `json:"stockPrePrice"`
	RecommendReason             string     `json:"recommendReason"`
	RecommendBuyPrice           string     `json:"recommendBuyPrice"`
	RecommendBuyPriceMin        float64    `json:"recommendBuyPriceMin"`
	RecommendBuyPriceMax        float64    `json:"recommendBuyPriceMax"`
	RecommendStopProfitPrice    string     `json:"recommendStopProfitPrice"`
	RecommendStopProfitPriceMin float64    `json:"recommendStopProfitPriceMin"`
	RecommendStopProfitPriceMax float64    `json:"recommendStopProfitPriceMax"`
	RecommendStopLossPrice      string     `json:"recommendStopLossPrice"`
	RiskRemarks                 string     `json:"riskRemarks"`
	Remarks                     string     `json:"remarks"`
	EnableAlert                 bool       `json:"enableAlert" gorm:"default:false"`
}

func (AiRecommendStocks) TableName() string { return "ai_recommend_stocks" }

// AiRecommendStocksQuery 分页查询参数
type AiRecommendStocksQuery struct {
	Page        int    `form:"page" json:"page"`
	PageSize    int    `form:"pageSize" json:"pageSize"`
	ModelName   string `form:"modelName" json:"modelName"`
	StockCode   string `form:"stockCode" json:"stockCode"`
	StockName   string `form:"stockName" json:"stockName"`
	BkCode      string `form:"bkCode" json:"bkCode"`
	BkName      string `form:"bkName" json:"bkName"`
	StartDate   string `form:"startDate" json:"startDate"`
	EndDate     string `form:"endDate" json:"endDate"`
	EnableAlert *bool  `form:"enableAlert" json:"enableAlert"`
}

// AiRecommendStocksPageData 分页数据
type AiRecommendStocksPageData struct {
	List       []AiRecommendStocks `json:"list"`
	Total      int64               `json:"total"`
	Page       int                 `json:"page"`
	PageSize   int                 `json:"pageSize"`
	TotalPages int                 `json:"totalPages"`
}

// ModelStat 按模型的胜率统计
type ModelStat struct {
	ModelName string  `json:"modelName"`
	WinRate   float64 `json:"winRate"`
	AvgReturn float64 `json:"avgReturn"`
	Count     int     `json:"count"`
}

// SectorStat 按板块的推荐数量统计
type SectorStat struct {
	BkName string `json:"bkName"`
	Count  int    `json:"count"`
}

// DailyCount 按日的推荐数量统计
type DailyCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// AiRecommendStats AI 推荐统计
type AiRecommendStats struct {
	ByModel    []ModelStat  `json:"byModel"`
	BySector   []SectorStat `json:"bySector"`
	DailyCount []DailyCount `json:"dailyCount"`
}
