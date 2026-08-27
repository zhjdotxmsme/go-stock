package data

import (
	"encoding/json"
	"fmt"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/util"
	"strconv"
	"time"
)

type MarketStatisticApi struct {
}

func NewMarketStatisticApi() *MarketStatisticApi {
	return &MarketStatisticApi{}
}

// ---- EastMoney market data structures ----

type emUlistResponse struct {
	RC   int `json:"rc"`
	Data struct {
		Total int           `json:"total"`
		Diff  []emUlistItem `json:"diff"`
	} `json:"data"`
}

type emUlistItem struct {
	F2  interface{} `json:"f2"`  // price
	F3  interface{} `json:"f3"`  // change_pct
	F4  interface{} `json:"f4"`  // change
	F12 string      `json:"f12"` // code
	F14 string      `json:"f14"` // name
	F104 interface{} `json:"f104"` // up count
	F105 interface{} `json:"f105"` // down count
	F106 interface{} `json:"f106"` // flat count
}

type emClistResponse struct {
	RC   int `json:"rc"`
	Data struct {
		Total int            `json:"total"`
		Diff  []emClistItem  `json:"diff"`
	} `json:"data"`
}

type emClistItem struct {
	F3  interface{} `json:"f3"`  // change_pct
	F12 string      `json:"f12"` // code
}

// ---- Public market data fetcher (shared by stats cron + LLM tools) ----

// EMMarketSnapshot holds the full market overview from EastMoney.
type EMMarketSnapshot struct {
	IndexQuotes []EMIndexQuote
	UpDownDis   EMUpDownDis
}

type EMIndexQuote struct {
	Code     string
	Name     string
	Price    float64
	ChangePct float64
	Change   float64
	UpCount  int
	DownCount int
	FlatCount int
}

type EMUpDownDis struct {
	RiseCount   int     // 上涨家数
	FallCount   int     // 下跌家数
	FlatCount   int     // 平盘家数
	LimitUp     int     // 涨停家数 (>=9.8%)
	LimitDown   int     // 跌停家数 (<=-9.8%)
	AverageRise float64 // 平均涨幅
	Up2         int     // 0~2%
	Up4         int     // 2%~4%
	Up6         int     // 4%~6%
	Up8         int     // 6%~8%
	Up10        int     // 8%~10%
	Down2       int     // 0~-2%
	Down4       int     // -2%~-4%
	Down6       int     // -4%~-6%
	Down8       int     // -6%~-8%
	Down10      int     // -8%~-10%
}

// FetchMarketSnapshot fetches real-time market overview from EastMoney.
func FetchMarketSnapshot() (*EMMarketSnapshot, error) {
	snap := &EMMarketSnapshot{}

	// 1) Index quotes + per-exchange breadth (SH=000001, SZ=399001)
	ulistURL := "https://push2.eastmoney.com/api/qt/ulist.np/get?fltt=2&invt=2&fields=f2,f3,f4,f12,f14,f104,f105,f106&secids=1.000001,0.399001"
	var ulistResp emUlistResponse
	if err := fetchEastMoneyJSON(ulistURL, &ulistResp); err != nil {
		return nil, fmt.Errorf("index breadth: %w", err)
	}
	for _, item := range ulistResp.Data.Diff {
		snap.IndexQuotes = append(snap.IndexQuotes, EMIndexQuote{
			Code:      "sh" + item.F12,
			Name:      item.F14,
			Price:     toFloat64(item.F2),
			ChangePct: toFloat64(item.F3),
			Change:    toFloat64(item.F4),
			UpCount:   toInt(item.F104),
			DownCount: toInt(item.F105),
			FlatCount: toInt(item.F106),
		})
	}

	// 2) Whole-market stock list with pct change for distribution
	clistURL := "https://push2.eastmoney.com/api/qt/clist/get?pn=1&pz=6000&po=1&np=1&fltt=2&invt=2&fid=f3&fs=m:0+t:6,m:0+t:80,m:1+t:2,m:1+t:23&fields=f3"
	var clistResp emClistResponse
	if err := fetchEastMoneyJSON(clistURL, &clistResp); err != nil {
		return nil, fmt.Errorf("market breadth: %w", err)
	}

	// Bucket distribution from pct changes
	for _, item := range clistResp.Data.Diff {
		pct := toFloat64(item.F3)
		snap.UpDownDis.AverageRise += pct
		switch {
		case pct >= 9.8:
			snap.UpDownDis.LimitUp++
			snap.UpDownDis.RiseCount++
		case pct >= 8:
			snap.UpDownDis.Up10++
			snap.UpDownDis.RiseCount++
		case pct >= 6:
			snap.UpDownDis.Up8++
			snap.UpDownDis.RiseCount++
		case pct >= 4:
			snap.UpDownDis.Up6++
			snap.UpDownDis.RiseCount++
		case pct >= 2:
			snap.UpDownDis.Up4++
			snap.UpDownDis.RiseCount++
		case pct > 0:
			snap.UpDownDis.Up2++
			snap.UpDownDis.RiseCount++
		case pct == 0:
			snap.UpDownDis.FlatCount++
		case pct > -2:
			snap.UpDownDis.Down2++
			snap.UpDownDis.FallCount++
		case pct > -4:
			snap.UpDownDis.Down4++
			snap.UpDownDis.FallCount++
		case pct > -6:
			snap.UpDownDis.Down6++
			snap.UpDownDis.FallCount++
		case pct > -8:
			snap.UpDownDis.Down8++
			snap.UpDownDis.FallCount++
		case pct > -9.8:
			snap.UpDownDis.Down10++
			snap.UpDownDis.FallCount++
		default:
			snap.UpDownDis.LimitDown++
			snap.UpDownDis.FallCount++
		}
	}
	totalStocks := snap.UpDownDis.RiseCount + snap.UpDownDis.FallCount + snap.UpDownDis.FlatCount
	if totalStocks > 0 {
		snap.UpDownDis.AverageRise /= float64(totalStocks)
	}

	return snap, nil
}

func fetchEastMoneyJSON(url string, target interface{}) error {
	resp, err := SharedHTTPClient.R().
		SetHeader("User-Agent", util.GetUserAgent()).
		SetHeader("Referer", "https://quote.eastmoney.com/").
		SetHeader("Accept", "application/json").
		Get(url)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	if resp.StatusCode() != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode())
	}
	if err := json.Unmarshal(resp.Body(), target); err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	return nil
}

func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	case json.Number:
		f, _ := val.Float64()
		return f
	default:
		return 0
	}
}

func toInt(v interface{}) int {
	return int(toFloat64(v))
}

func (a *MarketStatisticApi) FetchAndSave() error {
	snap, err := FetchMarketSnapshot()
	if err != nil {
		logger.SugaredLogger.Errorf("获取市场数据失败: %v", err)
		return err
	}

	now := time.Now()
	dataDate := now.Format("2006-01-02")
	dataTime := now.Format("15:04")

	// Build MarketStatistic (same schema as before, new data source)
	totalUp := snap.UpDownDis.RiseCount
	totalDown := snap.UpDownDis.FallCount
	limitUp := snap.UpDownDis.LimitUp
	limitDown := snap.UpDownDis.LimitDown

	var shUp, shDown, szUp, szDown int
	for _, idx := range snap.IndexQuotes {
		switch idx.Code {
		case "sh000001":
			shUp, shDown = idx.UpCount, idx.DownCount
		case "sh399001":
			szUp, szDown = idx.UpCount, idx.DownCount
		}
	}

	var upRatio, upDownRatio, limitRatio float64
	total := totalUp + totalDown
	if total > 0 {
		upRatio = float64(totalUp) / float64(total) * 100
	}
	if totalDown > 0 {
		upDownRatio = float64(totalUp) / float64(totalDown)
	} else if totalUp > 0 {
		upDownRatio = float64(totalUp)
	}
	sentimentDesc := getSentimentDesc(upDownRatio)
	if limitDown > 0 {
		limitRatio = float64(limitUp) / float64(limitDown)
	} else if limitUp > 0 {
		limitRatio = float64(limitUp)
	}

	stat := models.MarketStatistic{
		DataDate:      dataDate,
		DataTime:      dataTime,
		UpCount:       totalUp,
		DownCount:     totalDown,
		UpRatio:       upRatio,
		UpDownRatio:   upDownRatio,
		SentimentDesc: sentimentDesc,
		LimitUp:       limitUp,
		LimitDown:     limitDown,
		LimitRatio:    limitRatio,
		ShUpCount:     shUp,
		ShDownCount:   shDown,
		SzUpCount:     szUp,
		SzDownCount:   szDown,
	}

	var existing models.MarketStatistic
	result := db.Dao.Where("data_date = ? AND data_time = ?", dataDate, dataTime).First(&existing)
	if result.Error == nil {
		db.Dao.Model(&existing).Updates(stat)
		logger.SugaredLogger.Infof("更新市场统计数据(东方财富): %s %s 涨跌家数(%d/%d) 涨跌停(%d/%d)",
			dataDate, dataTime, totalUp, totalDown, limitUp, limitDown)
	} else {
		db.Dao.Create(&stat)
		logger.SugaredLogger.Infof("保存市场统计数据(东方财富): %s %s 涨跌家数(%d/%d) 涨跌停(%d/%d)",
			dataDate, dataTime, totalUp, totalDown, limitUp, limitDown)
	}

	return nil
}

func (a *MarketStatisticApi) GetTodayData() []models.MarketStatistic {
	today := time.Now().Format("2006-01-02")
	var data []models.MarketStatistic
	db.Dao.Where("data_date = ?", today).Order("data_time ASC").Find(&data)
	if len(data) > 0 {
		return data
	}
	var latest models.MarketStatistic
	if err := db.Dao.Order("data_date DESC, data_time DESC").First(&latest).Error; err == nil {
		db.Dao.Where("data_date = ?", latest.DataDate).Order("data_time ASC").Find(&data)
	}
	return data
}

func (a *MarketStatisticApi) GetRecentDaysData(days int) []models.MarketStatistic {
	startDate := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	var data []models.MarketStatistic
	db.Dao.Where("data_date >= ?", startDate).Order("data_date ASC, data_time ASC").Find(&data)
	return data
}

func (a *MarketStatisticApi) GetByDate(date string) []models.MarketStatistic {
	var data []models.MarketStatistic
	db.Dao.Where("data_date = ?", date).Order("data_time ASC").Find(&data)
	return data
}

func getSentimentDesc(upDownRatio float64) string {
	switch {
	case upDownRatio >= 2:
		return "普涨(极强)"
	case upDownRatio >= 1.5:
		return "偏强"
	case upDownRatio > 1:
		return "稍强"
	case upDownRatio == 1:
		return "中性"
	case upDownRatio > 0.5:
		return "稍弱"
	case upDownRatio > 0:
		return "偏弱"
	default:
		return "普跌(冰点)"
	}
}
