package data

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"go-stock/backend/data/datasource"
	"go-stock/backend/models"
)

// CommodityApi 商品数据统一入口
type CommodityApi struct {
	wsClient WallstreetcnApi   // 国际现货数据源（值接收器）
	emClient *EastMoneyKLineApi // 国内期货/ETF K线数据源
}

// NewCommodityApi 创建 CommodityApi 实例
func NewCommodityApi() *CommodityApi {
	return &CommodityApi{
		wsClient: WallstreetcnApi{},
		emClient: NewEastMoneyKLineApi(GetSettingConfig()),
	}
}

// GetQuote 获取实时行情
// 根据品种 AssetType 路由到对应数据源
func (c *CommodityApi) GetQuote(code string) (*datasource.QuoteData, error) {
	asset := FindCommodityByCode(code)
	if asset == nil {
		return nil, fmt.Errorf("未找到品种: %s", code)
	}

	switch asset.AssetType {
	case models.AssetSpot:
		return c.getSpotQuote(asset)
	case models.AssetFutures:
		return c.getFuturesQuote(asset)
	case models.AssetETF:
		return c.getETFQuote(asset)
	}

	return nil, fmt.Errorf("不支持的资产类型: %s", asset.AssetType)
}

func (c *CommodityApi) getSpotQuote(asset *models.CommodityAsset) (*datasource.QuoteData, error) {
	result := c.wsClient.GetMarketReal([]string{asset.Symbol}, nil)
	if result == nil || result.Code != 20000 || len(result.Data.Snapshot) == 0 {
		return nil, fmt.Errorf("获取 %s 行情失败", asset.Symbol)
	}

	values := result.Data.Snapshot[asset.Symbol]
	if len(values) < 4 {
		return nil, fmt.Errorf("%s 行情字段不完整", asset.Symbol)
	}

	lastPx, _ := strconv.ParseFloat(fmt.Sprintf("%v", values[1]), 64)
	pxChange, _ := strconv.ParseFloat(fmt.Sprintf("%v", values[2]), 64)
	pxChgRate, _ := strconv.ParseFloat(fmt.Sprintf("%v", values[3]), 64)

	return &datasource.QuoteData{
		Code:      asset.Code,
		Name:      asset.Name,
		Price:     lastPx,
		Change:    pxChange,
		ChangePct: pxChgRate,
		Time:      time.Now(),
	}, nil
}

func (c *CommodityApi) getFuturesQuote(asset *models.CommodityAsset) (*datasource.QuoteData, error) {
	kLines := c.emClient.GetKLineData(asset.Symbol, "101", "", 1)
	if kLines == nil || len(*kLines) == 0 {
		return nil, fmt.Errorf("获取 %s 行情失败", asset.Symbol)
	}
	k := (*kLines)[0]
	closeVal, _ := parseFloatToFloat(k.Close)
	openVal, _ := parseFloatToFloat(k.Open)
	highVal, _ := parseFloatToFloat(k.High)
	lowVal, _ := parseFloatToFloat(k.Low)
	changeVal, _ := parseFloatToFloat(k.ChangeValue)
	changePctVal, _ := parseFloatToFloat(k.ChangePercent)

	return &datasource.QuoteData{
		Code:      asset.Code,
		Name:      asset.Name,
		Price:     closeVal,
		Change:    changeVal,
		ChangePct: changePctVal,
		High:      highVal,
		Low:       lowVal,
		Open:      openVal,
		Time:      time.Now(),
	}, nil
}

func (c *CommodityApi) getETFQuote(asset *models.CommodityAsset) (*datasource.QuoteData, error) {
	stockDataApi := NewStockDataApi()
	infos, err := stockDataApi.GetStockCodeRealTimeData(asset.Symbol)
	if err != nil || infos == nil || len(*infos) == 0 {
		return nil, fmt.Errorf("获取 %s 行情失败", asset.Symbol)
	}
	info := (*infos)[0]
	price, _ := parseFloatToFloat(info.Price)
	high, _ := parseFloatToFloat(info.High)
	low, _ := parseFloatToFloat(info.Low)
	open, _ := parseFloatToFloat(info.Open)
	return &datasource.QuoteData{
		Code:      asset.Code,
		Name:      info.Name,
		Price:     price,
		Change:    info.ChangePrice,
		ChangePct: info.ChangePercent,
		High:      high,
		Low:       low,
		Open:      open,
		Time:      time.Now(),
	}, nil
}

// GetKLine 获取 K 线数据
func (c *CommodityApi) GetKLine(code string, period string, count int) ([]datasource.KLineBar, error) {
	asset := FindCommodityByCode(code)
	if asset == nil {
		return nil, fmt.Errorf("未找到品种: %s", code)
	}

	switch asset.AssetType {
	case models.AssetSpot:
		return c.getSpotKLine(asset, period, count)
	case models.AssetFutures:
		return c.getFuturesKLine(asset, period, count)
	case models.AssetETF:
		return c.getETFKLine(asset, period, count)
	}

	return nil, fmt.Errorf("不支持的资产类型: %s", asset.AssetType)
}

func (c *CommodityApi) getSpotKLine(asset *models.CommodityAsset, period string, count int) ([]datasource.KLineBar, error) {
	periodMap := map[string]int{
		"day":   86400,
		"week":  604800,
		"month": 2592000,
	}

	wsPeriod := periodMap[period]
	if wsPeriod == 0 {
		wsPeriod = 86400
	}
	if count <= 0 {
		count = 120
	}

	fields := []string{"tick_at", "open_px", "close_px", "high_px", "low_px", "turnover_volume"}
	resp := c.wsClient.GetKline(asset.Symbol, wsPeriod, count, fields)
	if resp == nil || resp.Code != 20000 {
		return nil, fmt.Errorf("获取 %s K 线失败", asset.Symbol)
	}

	candle, ok := resp.Data.Candle[asset.Symbol]
	if !ok {
		return nil, fmt.Errorf("%s K 线数据为空", asset.Symbol)
	}

	result := make([]datasource.KLineBar, 0, len(candle.Lines))
	for _, line := range candle.Lines {
		if len(line) < 5 {
			continue
		}
		timestamp := int64(line[0])
		var volume int64
		if len(line) > 5 {
			volume = int64(line[5])
		}
		result = append(result, datasource.KLineBar{
			Time:   time.Unix(timestamp, 0),
			Open:   line[1],
			Close:  line[2],
			High:   line[3],
			Low:    line[4],
			Volume: volume,
		})
	}
	return result, nil
}

func (c *CommodityApi) getFuturesKLine(asset *models.CommodityAsset, period string, count int) ([]datasource.KLineBar, error) {
	periodMap := map[string]string{
		"day":   "101",
		"week":  "102",
		"month": "103",
	}

	klt := periodMap[period]
	if klt == "" {
		klt = "101"
	}
	if count <= 0 {
		count = 120
	}

	kLines := c.emClient.GetKLineData(asset.Symbol, klt, "", count)
	if kLines == nil || len(*kLines) == 0 {
		return nil, fmt.Errorf("获取 %s K 线失败", asset.Symbol)
	}

	result := make([]datasource.KLineBar, 0, len(*kLines))
	for _, k := range *kLines {
		o, _ := parseFloatToFloat(k.Open)
		closeVal, _ := parseFloatToFloat(k.Close)
		h, _ := parseFloatToFloat(k.High)
		l, _ := parseFloatToFloat(k.Low)
		v, _ := parseFloatToFloat(k.Volume)
		a, _ := parseFloatToFloat(k.Amount)
		result = append(result, datasource.KLineBar{
			Time:   parseEastMoneyDay(k.Day),
			Open:   o,
			Close:  closeVal,
			High:   h,
			Low:    l,
			Volume: int64(v),
			Amount: a,
		})
	}
	return result, nil
}

func (c *CommodityApi) getETFKLine(asset *models.CommodityAsset, period string, count int) ([]datasource.KLineBar, error) {
	periodMap := map[string]string{
		"day":   "101",
		"week":  "102",
		"month": "103",
	}

	klt := periodMap[period]
	if klt == "" {
		klt = "101"
	}
	if count <= 0 {
		count = 120
	}

	kLines := c.emClient.GetKLineData(asset.Symbol, klt, "", count)
	if kLines == nil || len(*kLines) == 0 {
		return nil, fmt.Errorf("获取 %s K 线失败", asset.Symbol)
	}

	result := make([]datasource.KLineBar, 0, len(*kLines))
	for _, k := range *kLines {
		o, _ := parseFloatToFloat(k.Open)
		closeVal, _ := parseFloatToFloat(k.Close)
		h, _ := parseFloatToFloat(k.High)
		l, _ := parseFloatToFloat(k.Low)
		v, _ := parseFloatToFloat(k.Volume)
		a, _ := parseFloatToFloat(k.Amount)
		result = append(result, datasource.KLineBar{
			Time:   parseEastMoneyDay(k.Day),
			Open:   o,
			Close:  closeVal,
			High:   h,
			Low:    l,
			Volume: int64(v),
			Amount: a,
		})
	}
	return result, nil
}

func parseEastMoneyDay(day string) time.Time {
	day = strings.TrimSpace(day)
	if len(day) >= 10 {
		t, err := time.Parse("2006-01-02", day[:10])
		if err == nil {
			return t
		}
	}
	return time.Time{}
}
