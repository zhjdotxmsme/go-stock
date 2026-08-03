package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go-stock/backend/data/datasource"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

var (
	ErrCommodityNotFound      = errors.New("未找到品种")
	ErrSpotDataUnavailable    = errors.New("现货数据源全部不可用，请检查网络")
	ErrFuturesDataUnavailable = errors.New("期货数据源全部不可用，请检查网络")
	ErrETFDataUnavailable     = errors.New("ETF数据源全部不可用，请检查网络")
)

// CommodityApi 商品数据统一入口
type CommodityApi struct {
	wsClient        WallstreetcnApi      // 国际现货数据源（值接收器）
	emClient        *EastMoneyKLineApi   // ETF K 线数据源
	emFuturesClient EastMoneyFuturesApi  // 国内期货行情+K 线数据源
}

// NewCommodityApi 创建 CommodityApi 实例
func NewCommodityApi() *CommodityApi {
	return &CommodityApi{
		wsClient:        WallstreetcnApi{},
		emClient:        NewEastMoneyKLineApi(GetSettingConfig()),
		emFuturesClient: EastMoneyFuturesApi{},
	}
}

// GetQuote 获取实时行情（国内价格优先）
// 根据品种 AssetType 路由到对应数据源
func (c *CommodityApi) GetQuote(code string) (*datasource.QuoteData, error) {
	asset := FindCommodityByCode(code)
	if asset == nil {
		return nil, fmt.Errorf("%w: %s", ErrCommodityNotFound, code)
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

// GetQuoteIntl 获取国际参考行情（仅对期货/现货有意义）
func (c *CommodityApi) GetQuoteIntl(code string) (*datasource.QuoteData, error) {
	asset := FindCommodityByCode(code)
	if asset == nil {
		return nil, fmt.Errorf("%w: %s", ErrCommodityNotFound, code)
	}
	if asset.InternationalRef == "" {
		return nil, fmt.Errorf("%s 无国际参考代码", asset.Code)
	}

	yahoo := &YahooFinanceApi{}
	quote, err := yahoo.GetQuote(asset.Code)
	if err != nil {
		return nil, fmt.Errorf("国际参考行情获取失败: %w", err)
	}
	quote.Name = asset.Name + "(国际参考)"
	return quote, nil
}

func (c *CommodityApi) getSpotQuote(asset *models.CommodityAsset) (*datasource.QuoteData, error) {
	// 伦敦金银: XAU/XAG 优先使用 AURUM Rates（XAU=X/XAG=X 在 Yahoo 上有限制）
	if asset.Code == "XAU" || asset.Code == "XAG" {
		aurum := &AurumRatesApi{}
		quote, err := aurum.GetQuote(asset.Code)
		if err == nil {
			quote.Name = asset.Name
			return quote, nil
		}
		logger.SugaredLogger.Warnf("AURUM Rates quote failed for %s: %v, trying Yahoo", asset.Code, err)
	}

	// 其他现货: Yahoo 优先
	yahoo := &YahooFinanceApi{}
	quote, err := yahoo.GetQuote(asset.Code)
	if err == nil {
		quote.Name = asset.Name
		return quote, nil
	}
	logger.SugaredLogger.Warnf("Yahoo Finance spot quote failed for %s: %v, trying AURUM Rates", asset.Code, err)

	aurum := &AurumRatesApi{}
	quote, err = aurum.GetQuote(asset.Code)
	if err == nil {
		quote.Name = asset.Name
		return quote, nil
	}
	logger.SugaredLogger.Warnf("AURUM Rates spot quote failed for %s: %v, trying WallStreetCN", asset.Code, err)

	result := c.wsClient.GetMarketReal([]string{asset.Symbol}, nil)
	if result != nil && result.Code == 20000 && len(result.Data.Snapshot) > 0 {
		values := result.Data.Snapshot[asset.Symbol]
		if len(values) >= 4 {
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
	}

	logger.SugaredLogger.Errorf("All spot quote sources failed for %s", asset.Code)
	return nil, fmt.Errorf("%w: %s (%s)", ErrSpotDataUnavailable, asset.Name, asset.Symbol)
}

func (c *CommodityApi) getFuturesQuote(asset *models.CommodityAsset) (*datasource.QuoteData, error) {
	quote, err := c.emFuturesClient.GetQuote(asset)
	if err == nil {
		return quote, nil
	}
	logger.SugaredLogger.Warnf("EastMoney futures quote failed for %s: %v, trying Sina", asset.Code, err)

	quote, err = c.getFuturesQuoteFromSina(asset)
	if err == nil {
		return quote, nil
	}
	logger.SugaredLogger.Warnf("Sina futures quote failed for %s: %v, trying Yahoo international", asset.Code, err)

	quote, err = c.getFuturesQuoteFromYahoo(asset)
	if err == nil {
		return quote, nil
	}
	logger.SugaredLogger.Errorf("Yahoo international quote failed for %s: %v", asset.Code, err)
	return nil, fmt.Errorf("%w: %s (%s)", ErrFuturesDataUnavailable, asset.Name, asset.Symbol)
}

func (c *CommodityApi) getFuturesQuoteFromYahoo(asset *models.CommodityAsset) (*datasource.QuoteData, error) {
	yahoo := &YahooFinanceApi{}
	quote, err := yahoo.GetQuote(asset.Code)
	if err != nil {
		return nil, err
	}
	quote.Name = asset.Name + "(国际参考)"
	return quote, nil
}

func (c *CommodityApi) getFuturesQuoteFromSina(asset *models.CommodityAsset) (*datasource.QuoteData, error) {
	sinaSymbol := "nf_" + asset.Code + "0"
	url := fmt.Sprintf("http://hq.sinajs.cn/rn=%d&list=%s", time.Now().UnixMilli(), sinaSymbol)

	resp, err := SharedHTTPClient.SetTimeout(10*time.Second).R().
		SetHeader("Referer", "https://finance.sina.com.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("sina futures quote request: %w", err)
	}

	body := GB18030ToUTF8(resp.Body())
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
	if len(parts) < 10 {
		return nil, fmt.Errorf("sina futures quote: %d fields, need >=10 for %s", len(parts), asset.Code)
	}

	name := parts[0]
	// parts[1] = open interest/volume, not a price field — skip
	lastSettle, _ := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
	open, _ := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)
	high, _ := strconv.ParseFloat(strings.TrimSpace(parts[4]), 64)
	low, _ := strconv.ParseFloat(strings.TrimSpace(parts[5]), 64)
	current, _ := strconv.ParseFloat(strings.TrimSpace(parts[6]), 64)

	change := current - lastSettle
	var changePct float64
	if lastSettle > 0 {
		changePct = change / lastSettle * 100
	}

	// parts[17] = date string "2006-01-02"
	quoteDate, _ := time.Parse("2006-01-02", strings.TrimSpace(parts[17]))

	quote := &datasource.QuoteData{
		Code:      asset.Code,
		Name:      name,
		Price:     current,
		Change:    change,
		ChangePct: changePct,
		High:      high,
		Low:       low,
		Open:      open,
		Time:      time.Now(),
	}
	if !quoteDate.IsZero() && time.Since(quoteDate) > 72*time.Hour {
		quote.Extra = map[string]interface{}{"stale": true, "quoteDate": quoteDate.Format("2006-01-02")}
	}
	return quote, nil
}

func (c *CommodityApi) getETFQuote(asset *models.CommodityAsset) (*datasource.QuoteData, error) {
	stockDataApi := NewStockDataApi()
	infos, err := stockDataApi.GetStockCodeRealTimeData(asset.Symbol)
	if err == nil && infos != nil && len(*infos) > 0 {
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
	logger.SugaredLogger.Warnf("StockDataApi ETF quote failed for %s: %v, trying Tencent", asset.Symbol, err)

	quote, err := c.getETFQuoteFromTencent(asset.Symbol)
	if err == nil {
		quote.Code = asset.Code
		quote.Name = asset.Name
		quote.Time = time.Now()
		return quote, nil
	}
	logger.SugaredLogger.Warnf("Tencent ETF quote failed for %s: %v, trying Sina", asset.Symbol, err)

	quote, err = c.getETFQuoteFromSina(asset)
	if err == nil {
		quote.Code = asset.Code
		quote.Name = asset.Name
		quote.Time = time.Now()
		return quote, nil
	}
	logger.SugaredLogger.Errorf("Sina ETF quote failed for %s: %v", asset.Symbol, err)
	return nil, fmt.Errorf("%w: %s (%s)", ErrETFDataUnavailable, asset.Name, asset.Symbol)
}

func (c *CommodityApi) getETFQuoteFromSina(asset *models.CommodityAsset) (*datasource.QuoteData, error) {
	url := fmt.Sprintf("http://hq.sinajs.cn/rn=%d&list=%s", time.Now().UnixMilli(), asset.Symbol)
	resp, err := SharedHTTPClient.SetTimeout(10*time.Second).R().
		SetHeader("Referer", "https://finance.sina.com.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("sina etf quote request: %w", err)
	}

	body := GB18030ToUTF8(resp.Body())
	if !strings.HasPrefix(body, "var hq_str_") {
		return nil, fmt.Errorf("sina etf quote: unexpected response for %s: %s", asset.Symbol, truncateStr(body, 120))
	}
	startIdx := strings.Index(body, "\"")
	if startIdx < 0 {
		return nil, fmt.Errorf("sina etf quote: no quote data for %s", asset.Symbol)
	}
	endIdx := strings.LastIndex(body, "\"")
	if endIdx <= startIdx {
		return nil, fmt.Errorf("sina etf quote: malformed response for %s", asset.Symbol)
	}
	content := body[startIdx+1 : endIdx]
	if content == "" {
		return nil, fmt.Errorf("sina etf quote: empty data for %s", asset.Symbol)
	}
	parts := strings.Split(content, ",")
	if len(parts) < 4 {
		return nil, fmt.Errorf("sina etf quote: %d fields, need >=4 for %s", len(parts), asset.Symbol)
	}

	name := parts[0]
	open, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	prevClose, _ := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
	current, _ := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)
	high, _ := strconv.ParseFloat(strings.TrimSpace(parts[4]), 64)
	low, _ := strconv.ParseFloat(strings.TrimSpace(parts[5]), 64)
	volume, _ := strconv.ParseInt(strings.TrimSpace(parts[8]), 10, 64)

	change := current - prevClose
	var changePct float64
	if prevClose > 0 {
		changePct = change / prevClose * 100
	}

	quote := &datasource.QuoteData{
		Code:      asset.Code,
		Name:      name,
		Price:     current,
		Change:    change,
		ChangePct: changePct,
		High:      high,
		Low:       low,
		Open:      open,
		Volume:    volume,
		Time:      time.Now(),
	}
	return quote, nil
}

func (c *CommodityApi) getETFQuoteFromTencent(symbol string) (*datasource.QuoteData, error) {
	url := fmt.Sprintf("http://qt.gtimg.cn/?_=%d&q=%s", time.Now().Unix(), strings.ToLower(symbol))
	resp, err := SharedHTTPClient.SetTimeout(10*time.Second).R().
		SetHeader("Host", "qt.gtimg.cn").
		SetHeader("Referer", "https://gu.qq.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		Get(url)
	if err != nil {
		return nil, err
	}
	body := GB18030ToUTF8(resp.Body())
	for _, line := range strings.Split(body, ";") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		info, err := ParseTxStockData(line)
		if err != nil || info == nil {
			continue
		}
		price, _ := parseFloatToFloat(info.Price)
		return &datasource.QuoteData{
			Code:      info.Code,
			Name:      info.Name,
			Price:     price,
			Change:    info.ChangePrice,
			ChangePct: info.ChangePercent,
			Time:      time.Now(),
		}, nil
	}
	return nil, fmt.Errorf("tencent etf quote no valid data for %s", symbol)
}

// GetKLine 获取 K 线数据（国内价格优先）
func (c *CommodityApi) GetKLine(code string, period string, count int) ([]datasource.KLineBar, error) {
	asset := FindCommodityByCode(code)
	if asset == nil {
		return nil, fmt.Errorf("%w: %s", ErrCommodityNotFound, code)
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

// GetKLineIntl 获取国际参考 K 线（仅对期货/现货有意义）
func (c *CommodityApi) GetKLineIntl(code string, period string, count int) ([]datasource.KLineBar, error) {
	asset := FindCommodityByCode(code)
	if asset == nil {
		return nil, fmt.Errorf("%w: %s", ErrCommodityNotFound, code)
	}
	if asset.InternationalRef == "" {
		return nil, fmt.Errorf("%s 无国际参考代码", asset.Code)
	}

	yahoo := &YahooFinanceApi{}
	return yahoo.GetKLine(asset.Code, period, count)
}

func (c *CommodityApi) getSpotKLine(asset *models.CommodityAsset, period string, count int) ([]datasource.KLineBar, error) {
	yahoo := &YahooFinanceApi{}
	bars, err := yahoo.GetKLine(asset.Code, period, count)
	if err == nil {
		return bars, nil
	}
	logger.SugaredLogger.Warnf("Yahoo Finance spot K-line failed for %s: %v, trying WallStreetCN", asset.Code, err)

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
	if resp != nil && resp.Code == 20000 {
		candle, ok := resp.Data.Candle[asset.Symbol]
		if ok && len(candle.Lines) > 0 {
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
	}

	logger.SugaredLogger.Errorf("All spot K-line sources failed for %s", asset.Code)
	return nil, fmt.Errorf("%w: %s (%s)", ErrSpotDataUnavailable, asset.Name, asset.Symbol)
}

func (c *CommodityApi) getFuturesKLine(asset *models.CommodityAsset, period string, count int) ([]datasource.KLineBar, error) {
	bars, err := c.getFuturesKLineFromSina(asset, period, count)
	if err == nil {
		return bars, nil
	}
	logger.SugaredLogger.Warnf("Sina futures K-line failed for %s: %v, trying EastMoney push2his", asset.Code, err)

	bars, err = c.emFuturesClient.GetKLine(asset, period, count)
	if err == nil {
		return bars, nil
	}
	logger.SugaredLogger.Warnf("EastMoney futures K-line failed for %s: %v, trying Yahoo international", asset.Code, err)

	bars, err = c.getFuturesKLineFromYahoo(asset, period, count)
	if err == nil {
		return bars, nil
	}
	logger.SugaredLogger.Errorf("All futures K-line sources failed for %s: %v", asset.Code, err)
	return nil, fmt.Errorf("%w: %s (%s)", ErrFuturesDataUnavailable, asset.Name, asset.Symbol)
}

func (c *CommodityApi) getFuturesKLineFromYahoo(asset *models.CommodityAsset, period string, count int) ([]datasource.KLineBar, error) {
	yahoo := &YahooFinanceApi{}
	return yahoo.GetKLine(asset.Code, period, count)
}

func (c *CommodityApi) getFuturesKLineFromSina(asset *models.CommodityAsset, period string, count int) ([]datasource.KLineBar, error) {
	sinaSymbol := "nf_" + asset.Code + "0"
	url := fmt.Sprintf("http://stock.finance.sina.com.cn/futures/api/jsonp.php/InnerFuturesNewService.getDailyKLine?symbol=%s", sinaSymbol)

	resp, err := SharedHTTPClient.SetTimeout(15*time.Second).R().
		SetHeader("Referer", "https://finance.sina.com.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("sina futures kline: %w", err)
	}

	body := GB18030ToUTF8(resp.Body())
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

	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}

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
	if kLines != nil && len(*kLines) > 0 {
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
	logger.SugaredLogger.Warnf("EastMoney ETF K-line failed for %s, trying Sina stock K-line", asset.Symbol)

	sinaKline := NewSinaKLineApi(GetSettingConfig())
	sinaKLines := sinaKline.GetKLineData(asset.Symbol, klt, count)
	if sinaKLines != nil && len(*sinaKLines) > 0 {
		result := make([]datasource.KLineBar, 0, len(*sinaKLines))
		for _, k := range *sinaKLines {
			o, _ := parseFloatToFloat(k.Open)
			closeVal, _ := parseFloatToFloat(k.Close)
			h, _ := parseFloatToFloat(k.High)
			l, _ := parseFloatToFloat(k.Low)
			v, _ := parseFloatToFloat(k.Volume)
			result = append(result, datasource.KLineBar{
				Time:   parseEastMoneyDay(k.Day),
				Open:   o,
				Close:  closeVal,
				High:   h,
				Low:    l,
				Volume: int64(v),
			})
		}
		return result, nil
	}
	logger.SugaredLogger.Errorf("Sina ETF K-line empty for %s", asset.Symbol)
	return nil, fmt.Errorf("%w: %s (%s)", ErrETFDataUnavailable, asset.Name, asset.Symbol)
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

type MacroSnapshot struct {
	DXY        float64   `json:"dxy"`
	US2YR      float64   `json:"us2yr"`
	US10YR     float64   `json:"us10yr"`
	US30YR     float64   `json:"us30yr"`
	YieldCurve string    `json:"yieldCurve"`
	Timestamp  time.Time `json:"timestamp"`
}

// MacroSnapshotEnhanced 扩展宏观指标快照
type MacroSnapshotEnhanced struct {
	// 美元指数
	DXY float64 `json:"dxy"`

	// 美债收益率
	US2YR  float64 `json:"us2yr"`
	US5YR  float64 `json:"us5yr"`
	US7YR  float64 `json:"us7yr"`
	US10YR float64 `json:"us10yr"`
	US30YR float64 `json:"us30yr"`

	// 收益率曲线形态: normal / inverted / steep
	YieldCurve string `json:"yieldCurve"`

	// 美债 ETF
	TLTPrice     float64 `json:"tltPrice"`
	TLTChangePct float64 `json:"tltChangePct"`
	TIPPrice     float64 `json:"tipPrice"`
	TIPChangePct float64 `json:"tipChangePct"`

	// TIPS 实际利率 (多期限)
	TIPS5Y  float64 `json:"tips5y"`
	TIPS10Y float64 `json:"tips10y"`
	TIPS20Y float64 `json:"tips20y"`
	TIPS30Y float64 `json:"tips30y"`

	// 盈亏平衡通胀率
	BreakEven5Y  float64 `json:"breakEven5y"`
	BreakEven10Y float64 `json:"breakEven10y"`

	Timestamp time.Time `json:"timestamp"`
}

func (c *CommodityApi) GetMacroIndicators() (*MacroSnapshot, error) {
	ws := WallstreetcnApi{}
	resp := ws.GetMarketReal([]string{"DXY.OTC", "US2YR.OTC", "US10YR.OTC", "US30YR.OTC"}, nil)
	if resp == nil || len(resp.Data.Snapshot) == 0 {
		return nil, fmt.Errorf("wallstreetcn macro indicators: no data")
	}

	snap := resp.Data.Snapshot
	parsePx := func(code string) float64 {
		if row, ok := snap[code]; ok && len(row) > 1 {
			s := fmt.Sprintf("%v", row[1])
			v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
			return v
		}
		return 0
	}

	dxy := parsePx("DXY.OTC")
	us2yr := parsePx("US2YR.OTC")
	us10yr := parsePx("US10YR.OTC")
	us30yr := parsePx("US30YR.OTC")

	curve := "normal"
	if us2yr > us10yr && us10yr > 0 {
		curve = "inverted"
	} else if us30yr > 0 && us10yr > 0 && (us30yr-us2yr) > 0.02 {
		curve = "steep"
	}

	return &MacroSnapshot{
		DXY:        dxy,
		US2YR:      us2yr,
		US10YR:     us10yr,
		US30YR:     us30yr,
		YieldCurve: curve,
		Timestamp:  time.Now(),
	}, nil
}

// GetMacroIndicatorsEnhanced 获取扩展宏观指标（含多期限美债收益率 + TIPS + 盈亏平衡通胀 + TLT/TIP ETF）
func (c *CommodityApi) GetMacroIndicatorsEnhanced() (*MacroSnapshotEnhanced, error) {
	enhanced := &MacroSnapshotEnhanced{Timestamp: time.Now()}

	// 1. WallStreetCN: DXY + 多期限美债收益率
	ws := WallstreetcnApi{}
	resp := ws.GetMarketReal([]string{
		"DXY.OTC", "US2YR.OTC", "US5YR.OTC", "US7YR.OTC", "US10YR.OTC", "US30YR.OTC",
	}, nil)

	if resp != nil && len(resp.Data.Snapshot) > 0 {
		snap := resp.Data.Snapshot
		parsePx := func(code string) float64 {
			if row, ok := snap[code]; ok && len(row) > 1 {
				s := fmt.Sprintf("%v", row[1])
				v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
				return v
			}
			return 0
		}
		enhanced.DXY = parsePx("DXY.OTC")
		enhanced.US2YR = parsePx("US2YR.OTC")
		enhanced.US5YR = parsePx("US5YR.OTC")
		enhanced.US7YR = parsePx("US7YR.OTC")
		enhanced.US10YR = parsePx("US10YR.OTC")
		enhanced.US30YR = parsePx("US30YR.OTC")

		// 收益率曲线判断
		if enhanced.US2YR > enhanced.US10YR && enhanced.US10YR > 0 {
			enhanced.YieldCurve = "inverted"
		} else if enhanced.US30YR > 0 && enhanced.US10YR > 0 && (enhanced.US30YR-enhanced.US2YR) > 0.02 {
			enhanced.YieldCurve = "steep"
		} else {
			enhanced.YieldCurve = "normal"
		}
	}

	// 2. Yahoo Finance: TLT + TIP ETF 实时价格
	yahoo := &YahooFinanceApi{}
	if tltQuote, err := yahoo.GetQuote("TLT"); err == nil {
		enhanced.TLTPrice = tltQuote.Price
		enhanced.TLTChangePct = tltQuote.ChangePct
	} else {
		logger.SugaredLogger.Warnf("Yahoo TLT quote failed: %v", err)
	}
	if tipQuote, err := yahoo.GetQuote("TIP"); err == nil {
		enhanced.TIPPrice = tipQuote.Price
		enhanced.TIPChangePct = tipQuote.ChangePct
	} else {
		logger.SugaredLogger.Warnf("Yahoo TIP quote failed: %v", err)
	}

	// 3. FRED: TIPS 多期限 + 盈亏平衡通胀率
	fred := NewFredApi()
	if v, err := fred.GetTIPS5YRate(); err == nil {
		enhanced.TIPS5Y = v
	}
	if v, err := fred.GetTIPSRate(); err == nil {
		enhanced.TIPS10Y = v
	}
	if v, err := fred.GetTIPS20YRate(); err == nil {
		enhanced.TIPS20Y = v
	}
	if v, err := fred.GetTIPS30YRate(); err == nil {
		enhanced.TIPS30Y = v
	}
	if v, err := fred.GetBreakEvenInflation(); err == nil {
		enhanced.BreakEven5Y = v
	}
	if v, err := fred.GetBreakEvenInflation10Y(); err == nil {
		enhanced.BreakEven10Y = v
	}

	return enhanced, nil
}
