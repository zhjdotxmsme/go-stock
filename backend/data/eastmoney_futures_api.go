package data

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go-stock/backend/data/datasource"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// EastMoneyFuturesApi 东方财富期货行情 API
type EastMoneyFuturesApi struct{}

// GetQuote 获取国内期货实时行情
// 使用东方财富 push2 接口，secid 格式为 market.code，例如 113.AU0
func (e *EastMoneyFuturesApi) GetQuote(asset *models.CommodityAsset) (*datasource.QuoteData, error) {
	secid := asset.Symbol
	if secid == "" {
		return nil, fmt.Errorf("empty symbol for %s", asset.Code)
	}

	url := fmt.Sprintf(
		"https://push2.eastmoney.com/api/qt/stock/get?secid=%s&fields=f43,f44,f45,f46,f57,f58,f60,f169,f170,f86",
		secid,
	)

	resp, err := SharedHTTPClient.SetTimeout(15 * time.Second).R().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		SetHeader("Referer", "https://quote.eastmoney.com/").
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("eastmoney futures quote request: %w", err)
	}

	body := resp.Body()
	var result struct {
		RC   int    `json:"rc"`
		RT   int    `json:"rt"`
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("eastmoney futures quote parse: %w", err)
	}
	if result.RC != 0 || len(result.Data) == 0 {
		return nil, fmt.Errorf("eastmoney futures quote empty: rc=%d", result.RC)
	}

	parseFloat := func(key string) float64 {
		v, ok := result.Data[key]
		if !ok {
			return 0
		}
		s := fmt.Sprintf("%v", v)
		if s == "" || s == "-" {
			return 0
		}
		f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
		return f
	}

	price := parseFloat("f43")
	if price == 0 {
		return nil, fmt.Errorf("eastmoney futures quote invalid price for %s", asset.Code)
	}

	high := parseFloat("f44")
	low := parseFloat("f45")
	open := parseFloat("f46")
	preClose := parseFloat("f60")
	change := parseFloat("f169")
	changePct := parseFloat("f170")

	name := asset.Name
	if n, ok := result.Data["f58"].(string); ok && n != "" {
		name = n
	}

	logger.SugaredLogger.Infof("EastMoney futures quote %s: price=%.2f change=%.2f pct=%.2f", asset.Code, price, change, changePct)
	return &datasource.QuoteData{
		Code:      asset.Code,
		Name:      name,
		Price:     price,
		Change:    change,
		ChangePct: changePct,
		High:      high,
		Low:       low,
		Open:      open,
		PrevClose: preClose,
		Time:      time.Now(),
	}, nil
}

// GetKLine 获取国内期货 K 线数据
// 使用东方财富 push2his 接口
func (e *EastMoneyFuturesApi) GetKLine(asset *models.CommodityAsset, period string, count int) ([]datasource.KLineBar, error) {
	secid := asset.Symbol
	if secid == "" {
		return nil, fmt.Errorf("empty symbol for %s", asset.Code)
	}

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

	url := fmt.Sprintf(
		"https://push2his.eastmoney.com/api/qt/stock/kline/get?secid=%s&klt=%s&fqt=1&lmt=%d",
		secid, klt, count,
	)

	resp, err := SharedHTTPClient.SetTimeout(15 * time.Second).R().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		SetHeader("Referer", "https://quote.eastmoney.com/").
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("eastmoney futures kline request: %w", err)
	}

	var result struct {
		RC   int `json:"rc"`
		Data *struct {
			Klines []string `json:"klines"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("eastmoney futures kline parse: %w", err)
	}
	if result.RC != 0 || result.Data == nil || len(result.Data.Klines) == 0 {
		return nil, fmt.Errorf("eastmoney futures kline empty: rc=%d", result.RC)
	}

	bars := make([]datasource.KLineBar, 0, len(result.Data.Klines))
	for _, line := range result.Data.Klines {
		parts := strings.Split(line, ",")
		if len(parts) < 6 {
			continue
		}
		t, _ := time.Parse("2006-01-02", strings.TrimSpace(parts[0]))
		o, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		c, _ := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
		h, _ := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)
		l, _ := strconv.ParseFloat(strings.TrimSpace(parts[4]), 64)
		v, _ := strconv.ParseFloat(strings.TrimSpace(parts[5]), 64)
		bars = append(bars, datasource.KLineBar{
			Time:   t,
			Open:   o,
			Close:  c,
			High:   h,
			Low:    l,
			Volume: int64(v),
		})
	}

	if len(bars) == 0 {
		return nil, fmt.Errorf("eastmoney futures kline no valid bars for %s", asset.Code)
	}
	return bars, nil
}
