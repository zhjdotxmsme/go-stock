package fallback

import (
	"context"
	"encoding/json"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/data/datasource"
	"go-stock/backend/logger"
	"strconv"
	"strings"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
)

func toTencentCode(code string) string {
	code = strings.TrimSpace(code)
	upper := strings.ToUpper(code)
	if strings.HasPrefix(upper, "SH") || strings.HasPrefix(upper, "SZ") {
		return strings.ToLower(upper[:2]) + upper[2:]
	}
	if strings.HasPrefix(code, "6") || strings.HasPrefix(code, "68") || strings.HasPrefix(code, "9") {
		return "sh" + code
	}
	return "sz" + code
}

func parseTencentQuoteResponse(text, code string) (*datasource.QuoteData, error) {
	if !strings.Contains(text, "~") {
		return nil, fmt.Errorf("tencent quote: invalid response for %s", code)
	}
	parts := strings.Split(text, "~")
	if len(parts) < 10 {
		return nil, fmt.Errorf("tencent quote: unexpected format for %s", code)
	}
	priceStr := strings.TrimSpace(parts[3])
	price, _ := strconv.ParseFloat(priceStr, 64)
	return &datasource.QuoteData{
		Code:  code,
		Price: price,
		Time:  time.Now(),
	}, nil
}

// TencentQuoteProvider provides real-time quotes from qt.gtimg.cn (free, no API key).
type TencentQuoteProvider struct{}

func (p *TencentQuoteProvider) Name() string                      { return "tencent" }
func (p *TencentQuoteProvider) Priority() int                     { return 10 }
func (p *TencentQuoteProvider) Available(ctx context.Context) bool { return true }

func (p *TencentQuoteProvider) GetQuote(ctx context.Context, code string) (*datasource.QuoteData, error) {
	tencentCode := toTencentCode(code)
	url := fmt.Sprintf("http://qt.gtimg.cn/q=%s", tencentCode)
	resp, err := data.SharedHTTPClient.SetTimeout(10*time.Second).R().
		SetHeader("Host", "qt.gtimg.cn").
		SetHeader("Referer", "https://gu.qq.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("tencent quote: %w", err)
	}
	quote, err := parseTencentQuoteResponse(string(resp.Body()), code)
	if err != nil {
		return nil, err
	}
	logger.SugaredLogger.Infof("datasource: quote %s from tencent: %.2f", code, quote.Price)
	return quote, nil
}

func parseTencentKLineResponse(body []byte, code, period, tencentCode string) (*datasource.KLineData, error) {
	var res map[string]any
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("tencent kline: parse error for %s: %w", code, err)
	}

	codeInt, _ := convertor.ToInt(res["code"])
	if codeInt != 0 {
		return nil, fmt.Errorf("tencent kline: api returned code %v for %s", res["code"], code)
	}

	dataMap, ok := res["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("tencent kline: missing data field for %s", code)
	}
	stockData, ok := dataMap[tencentCode].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("tencent kline: missing %s data", tencentCode)
	}

	var day []any
	if qfqday, ok := stockData["qfqday"].([]any); ok && len(qfqday) > 0 {
		day = qfqday
	} else if d, ok := stockData["day"].([]any); ok && len(d) > 0 {
		day = d
	}

	bars := make([]datasource.KLineBar, 0, len(day))
	for _, v := range day {
		vv, ok := v.([]any)
		if !ok || len(vv) < 6 {
			continue
		}
		bar := datasource.KLineBar{
			Time:   parseKLineTime(convertor.ToString(vv[0])),
			Open:   parseFloat64(convertor.ToString(vv[1])),
			Close:  parseFloat64(convertor.ToString(vv[2])),
			High:   parseFloat64(convertor.ToString(vv[3])),
			Low:    parseFloat64(convertor.ToString(vv[4])),
			Volume: parseInt64(convertor.ToString(vv[5])),
		}
		if bar.Time.IsZero() {
			continue
		}
		bars = append(bars, bar)
	}

	if len(bars) == 0 {
		return nil, fmt.Errorf("tencent kline: no bars for %s", code)
	}

	return &datasource.KLineData{
		Code:   code,
		Period: period,
		Bars:   bars,
	}, nil
}

// TencentKLineProvider provides K-line data from Tencent Finance.
type TencentKLineProvider struct{}

func (p *TencentKLineProvider) Name() string                      { return "tencent_kline" }
func (p *TencentKLineProvider) Priority() int                     { return 10 }
func (p *TencentKLineProvider) Available(ctx context.Context) bool { return true }

func (p *TencentKLineProvider) GetKLine(ctx context.Context, code string, period string, count int) (*datasource.KLineData, error) {
	tencentCode := toTencentCode(code)
	period = datasource.NormalizePeriod(period)
	url := fmt.Sprintf("http://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param=%s,%s,,,%d,qfq", tencentCode, period, count)
	resp, err := data.SharedHTTPClient.SetTimeout(15*time.Second).R().
		SetHeader("Host", "web.ifzq.gtimg.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("tencent kline: %w", err)
	}

	kline, err := parseTencentKLineResponse(resp.Body(), code, period, tencentCode)
	if err != nil {
		return nil, err
	}
	logger.SugaredLogger.Infof("datasource: kline %s from tencent (%d bars)", code, len(kline.Bars))
	return kline, nil
}

func parseKLineTime(s string) time.Time {
	if t, err := time.Parse("2006-01-02", strings.TrimSpace(s)); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02 15:04:05", strings.TrimSpace(s)); err == nil {
		return t
	}
	return time.Time{}
}

// MootdxQuoteProvider provides real-time quotes via existing TDX API (free, unlimited).
type MootdxQuoteProvider struct{}

func (p *MootdxQuoteProvider) Name() string                      { return "mootdx" }
func (p *MootdxQuoteProvider) Priority() int                     { return 5 }
func (p *MootdxQuoteProvider) Available(ctx context.Context) bool { return true }

func (p *MootdxQuoteProvider) GetQuote(ctx context.Context, code string) (*datasource.QuoteData, error) {
	price, priceTime := data.GetRealTimeStockPriceInfo(ctx, code)
	if price == "" {
		return nil, fmt.Errorf("mootdx quote: empty price for %s", code)
	}
	priceVal, _ := strconv.ParseFloat(strings.TrimSpace(price), 64)
	var t time.Time
	if priceTime != "" {
		t, _ = time.Parse("2006-01-02 15:04:05", strings.TrimSpace(priceTime))
	}
	if t.IsZero() {
		t = time.Now()
	}
	logger.SugaredLogger.Infof("datasource: quote %s from mootdx: %.2f", code, priceVal)
	return &datasource.QuoteData{Code: code, Price: priceVal, Time: t}, nil
}

// MootdxKLineProvider provides K-line data via existing TDX API (free, unlimited).
type MootdxKLineProvider struct{}

func (p *MootdxKLineProvider) Name() string                      { return "mootdx_kline" }
func (p *MootdxKLineProvider) Priority() int                     { return 5 }
func (p *MootdxKLineProvider) Available(ctx context.Context) bool { return true }

func (p *MootdxKLineProvider) GetKLine(ctx context.Context, code string, period string, count int) (*datasource.KLineData, error) {
		tdx := data.NewTdxKLineApi()
		if tdx == nil {
			return nil, fmt.Errorf("mootdx kline api not available")
		}
		period = datasource.NormalizePeriod(period)
		kLines := tdx.GetKLineData(code, period, count)
	if kLines == nil || len(*kLines) == 0 {
		return nil, fmt.Errorf("mootdx kline: empty for %s", code)
	}
	logger.SugaredLogger.Infof("datasource: kline %s from mootdx (%d bars)", code, len(*kLines))
	return ConvertKLineData(code, period, *kLines), nil
}

// TonghuashunFundamentalProvider provides fundamental data for A-share stocks.
type TonghuashunFundamentalProvider struct{}

func (p *TonghuashunFundamentalProvider) Name() string                      { return "10jqka" }
func (p *TonghuashunFundamentalProvider) Priority() int                     { return 15 }
func (p *TonghuashunFundamentalProvider) Available(ctx context.Context) bool { return true }

func (p *TonghuashunFundamentalProvider) GetFundamental(ctx context.Context, code string) (*datasource.FundamentalData, error) {
	stockApi := data.NewStockDataApi()

	latest, err := stockApi.GetStockLatestFinance(code)
	if err != nil {
		return nil, fmt.Errorf("10jqka fundamental: latest finance failed for %s: %w", code, err)
	}
	if latest == nil || latest.Result == nil || len(latest.Result.Data) == 0 {
		return nil, fmt.Errorf("10jqka fundamental: no latest finance data for %s", code)
	}

	predict, _ := stockApi.GetStockPredictSummary(code)

	fd := &datasource.FundamentalData{}
	d := latest.Result.Data[0]
	fd.ROE = toFloat64(d["ROEJQ"])
	fd.DebtRatio = toFloat64(d["ZCFZL"])
	fd.Revenue = toFloat64(d["TOTAL_OPERATEINCOME"])
	fd.NetProfit = toFloat64(d["PARENT_NETPROFIT"])

	if predict != nil && predict.Result != nil && len(predict.Result.Data) > 0 {
		pd := predict.Result.Data[0]
		if fd.PE == 0 {
			fd.PE = toFloat64(pd["PE"])
		}
	}

	logger.SugaredLogger.Infof("datasource: fundamental %s from 10jqka (PE:%.2f PB:%.2f ROE:%.2f)", code, fd.PE, fd.PB, fd.ROE)
	return fd, nil
}

// BaiduSectorProvider provides sector and concept data for A-share stocks.
type BaiduSectorProvider struct{}

func (p *BaiduSectorProvider) Name() string                      { return "baidu" }
func (p *BaiduSectorProvider) Priority() int                     { return 20 }
func (p *BaiduSectorProvider) Available(ctx context.Context) bool { return true }

func (p *BaiduSectorProvider) GetSectorData(ctx context.Context, code string) (*datasource.SectorData, error) {
	stockApi := data.NewStockDataApi()
	info := stockApi.GetStockConceptInfo(code)
	if info.Result.Data == nil || len(info.Result.Data) == 0 {
		return nil, fmt.Errorf("baidu sector: empty for %s", code)
	}
	first := info.Result.Data[0]
	logger.SugaredLogger.Infof("datasource: sector %s from baidu: %s", code, first.BOARDNAME)
	return &datasource.SectorData{
		Code:   code,
		Sector: first.BOARDNAME,
	}, nil
}

func toFloat64(v any) float64 {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(n), 64)
		return f
	}
	return 0
}

func RegisterFreeDataSources(router *datasource.Router) {
	router.RegisterQuoteProvider(&MootdxQuoteProvider{})
	router.RegisterQuoteProvider(&TencentQuoteProvider{})
	router.RegisterKLineProvider(&MootdxKLineProvider{})
	router.RegisterKLineProvider(&TencentKLineProvider{})
	router.RegisterFundamentalProvider(&TonghuashunFundamentalProvider{})
	router.RegisterSectorProvider(&BaiduSectorProvider{})
	logger.SugaredLogger.Info("free data sources registered: mootdx, tencent, 10jqka, baidu")
}
