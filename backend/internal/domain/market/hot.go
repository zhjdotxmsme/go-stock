package market

import "gorm.io/gorm"

// XUEQIUHot 雪球热门股票
type XUEQIUHot struct {
	Data struct {
		Items     []HotItem `json:"items"`
		ItemsSize int       `json:"items_size"`
	} `json:"data"`
	ErrorCode        int    `json:"error_code"`
	ErrorDescription string `json:"error_description"`
}

// HotItem 热门股票项
type HotItem struct {
	Code       string  `json:"code" md:"股票代码"`
	Name       string  `json:"name" md:"股票名称"`
	Value      float64 `json:"value" md:"热度"`
	Increment  int     `json:"increment" md:"热度变化"`
	RankChange int     `json:"rank_change" md:"排名变化"`
	Percent    float64 `json:"percent" md:"涨跌幅(%)"`
	Current    float64 `json:"current" md:"股价"`
	Chg        float64 `json:"chg" md:"股价变化"`
	Exchange   string  `json:"exchange" md:"交易所代码"`
}

// HotEvent 热门事件
type HotEvent struct {
	PicSize     interface{} `json:"pic_size"`
	Tag         string      `json:"tag"`
	Id          int         `json:"id"`
	Pic         string      `json:"pic"`
	Hot         int         `json:"hot"`
	StatusCount int         `json:"status_count"`
	Content     string      `json:"content"`
}

// SinaStockInfo 新浪股票信息
type SinaStockInfo struct {
	Symbol        string `json:"symbol"`
	Name          string `json:"name"`
	Engname       string `json:"engname"`
	Tradetype     string `json:"tradetype"`
	Lasttrade     string `json:"lasttrade"`
	Prevclose     string `json:"prevclose"`
	Open          string `json:"open"`
	High          string `json:"high"`
	Low           string `json:"low"`
	Volume        string `json:"volume"`
	Currentvolume string `json:"currentvolume"`
	Amount        string `json:"amount"`
	Ticktime      string `json:"ticktime"`
	Buy           string `json:"buy"`
	Sell          string `json:"sell"`
	High52Week    string `json:"high_52week"`
	Low52Week     string `json:"low_52week"`
	Eps           string `json:"eps"`
	Dividend      string `json:"dividend"`
	StocksSum     string `json:"stocks_sum"`
	Pricechange   string `json:"pricechange"`
	Changepercent string `json:"changepercent"`
	MarketValue   string `json:"market_value"`
	PeRatio       string `json:"pe_ratio"`
}

// LongTigerRankData 龙虎榜数据
type LongTigerRankData struct {
	ACCUMAMOUNT      float64 `json:"ACCUM_AMOUNT"`
	BILLBOARDBUYAMT  float64 `json:"BILLBOARD_BUY_AMT"`
	BILLBOARDDEALAMT float64 `json:"BILLBOARD_DEAL_AMT"`
	BILLBOARDNETAMT  float64 `json:"BILLBOARD_NET_AMT"`
	BILLBOARDSELLAMT float64 `json:"BILLBOARD_SELL_AMT"`
	CHANGERATE       float64 `json:"CHANGE_RATE"`
	CLOSEPRICE       float64 `json:"CLOSE_PRICE"`
	DEALAMOUNTRATIO  float64 `json:"DEAL_AMOUNT_RATIO"`
	DEALNETRATIO     float64 `json:"DEAL_NET_RATIO"`
	EXPLAIN          string  `json:"EXPLAIN"`
	EXPLANATION      string  `json:"EXPLANATION"`
	FREEMARKETCAP    float64 `json:"FREE_MARKET_CAP"`
	SECUCODE         string  `json:"SECUCODE" gorm:"index"`
	SECURITYCODE     string  `json:"SECURITY_CODE"`
	SECURITYNAMEABBR string  `json:"SECURITY_NAME_ABBR"`
	SECURITYTYPECODE string  `json:"SECURITY_TYPE_CODE"`
	TRADEDATE        string  `json:"TRADE_DATE" gorm:"index"`
	TURNOVERRATE     float64 `json:"TURNOVERRATE"`
}

func (LongTigerRankData) TableName() string {
	return "long_tiger_rank"
}

// BKDict 板块字典
type BKDict struct {
	gorm.Model
	BKCode string `json:"bk_code" gorm:"uniqueIndex:idx_bk_code"`
	BKName string `json:"bk_name" gorm:"index"`
	BKType string `json:"bk_type" gorm:"index"` // 概念/行业/地域
	Source string `json:"source"`               // 数据源
}

func (BKDict) TableName() string {
	return "bk_dict"
}
