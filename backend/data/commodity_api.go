package data

import (
	"encoding/json"
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
		return nil, fmt.Errorf("华尔街见闻数据源未配置或网络不可达：获取 %s 行情失败", asset.Symbol)
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
	// Sina Finance is the only supported source for domestic futures quotes.
	// EastMoney push2his K-line API does NOT support futures secids (113.AU0 etc).
	return c.getFuturesQuoteFromSina(asset)
}

func (c *CommodityApi) getFuturesQuoteFromSina(asset *models.CommodityAsset) (*datasource.QuoteData, error) {
	// Sina real-time futures quote: hq.sinajs.cn/list=NF_AU0 (上海期货黄金主力连续)
	sinaSymbol := "NF_" + asset.Code + "0"
	url := fmt.Sprintf("http://hq.sinajs.cn/list=%s", sinaSymbol)

	resp, err := SharedHTTPClient.SetTimeout(10*time.Second).R().
		SetHeader("Referer", "https://finance.sina.com.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("sina futures quote request: %w", err)
	}

	body := GB18030ToUTF8(resp.Body())
	// Response format: var hq_str_NF_AU0="沪金0,571.98,572.78,571.52,572.90,570.26,2024-01-15,0,...";
	if !strings.HasPrefix(body, "var hq_str_") {
		return nil, fmt.Errorf("sina futures quote: unexpected response format for %s: %s", asset.Code, truncateStr(body, 120))
	}
	startIdx := strings.Index(body, "\"")
	if startIdx < 0 {
		return nil, fmt.Errorf("sina futures quote: no quote data for %s", asset.Code)
	}
	endIdx := strings.LastIndex(body, "\"")
	if endIdx <= startIdx {
		return nil, fmt.Errorf("sina futures quote: malformed response for %s", asset.Code)
	}
	content := body[startIdx+1 : endIdx]
	if content == "" {
		return nil, fmt.Errorf("sina futures quote: empty data for %s (symbol may be delisted)", asset.Code)
	}
	parts := strings.Split(content, ",")
	// Fields: 0=name,1=open,2=lastSettle,3=current,4=high,5=low,6=date,...
	if len(parts) < 10 {
		return nil, fmt.Errorf("sina futures quote: %d fields, need >=10 for %s", len(parts), asset.Code)
	}

	name := parts[0]
	open, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	lastSettle, _ := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
	current, _ := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)
	high, _ := strconv.ParseFloat(strings.TrimSpace(parts[4]), 64)
	low, _ := strconv.ParseFloat(strings.TrimSpace(parts[5]), 64)

	change := current - lastSettle
	var changePct float64
	if lastSettle > 0 {
		changePct = change / lastSettle * 100
	}

	return &datasource.QuoteData{
		Code:      asset.Code,
		Name:      name,
		Price:     current,
		Change:    change,
		ChangePct: changePct,
		High:      high,
		Low:       low,
		Open:      open,
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
	// Use Sina Finance for futures K-lines (EastMoney stock endpoint doesn't support futures secids)
	sinaSymbol := asset.Code + "0"
	url := fmt.Sprintf("http://stock.finance.sina.com.cn/futures/api/jsonp.php/InnerFuturesNewService.getDailyKLine?symbol=%s", sinaSymbol)

	resp, err := SharedHTTPClient.SetTimeout(15*time.Second).R().
		SetHeader("Referer", "https://finance.sina.com.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("sina futures kline: %w", err)
	}

	body := GB18030ToUTF8(resp.Body())
	// Response: InnerFuturesNewService.getDailyKLine([["2024-01-15","473.28","476.46","472.98","474.50","150220","0"],...]);
	idx := strings.Index(body, "[")
	if idx < 0 {
		return nil, fmt.Errorf("sina futures kline: unexpected response for %s", asset.Symbol)
	}
	endIdx := strings.LastIndex(body, "]")
	if endIdx <= idx {
		return nil, fmt.Errorf("sina futures kline: unexpected response for %s", asset.Symbol)
	}

	var rows [][]string
	jsonStr := body[idx : endIdx+1]
	if err := json.Unmarshal([]byte(jsonStr), &rows); err != nil {
		return nil, fmt.Errorf("sina futures kline: parse error for %s: %w", asset.Symbol, err)
	}

	// Sina returns newest first, reverse to oldest-first
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}

	// Take the most recent `count` entries
	if count <= 0 {
		count = 120
	}
	if len(rows) > count {
		rows = rows[len(rows)-count:]
	}

	result := make([]datasource.KLineBar, 0, len(rows))
	for _, row := range rows {
		if len(row) < 6 {
			continue
		}
		t, _ := time.Parse("2006-01-02", row[0])
		o, _ := strconv.ParseFloat(row[1], 64)
		h, _ := strconv.ParseFloat(row[2], 64)
		l, _ := strconv.ParseFloat(row[3], 64)
		c, _ := strconv.ParseFloat(row[4], 64)
		v, _ := strconv.ParseFloat(row[5], 64)
		result = append(result, datasource.KLineBar{
			Time:   t,
			Open:   o,
			Close:  c,
			High:   h,
			Low:    l,
			Volume: int64(v),
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
