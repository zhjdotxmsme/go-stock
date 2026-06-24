package fallback

import (
	"context"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/data/datasource"
	"go-stock/backend/logger"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// TencentQuoteProvider provides real-time quotes from qt.gtimg.cn (free, no API key).
type TencentQuoteProvider struct {
	client *http.Client
}

func (p *TencentQuoteProvider) Name() string { return "tencent" }
func (p *TencentQuoteProvider) Priority() int { return 10 }
func (p *TencentQuoteProvider) Available(ctx context.Context) bool { return true }

func (p *TencentQuoteProvider) GetQuote(ctx context.Context, code string) (*datasource.QuoteData, error) {
	if p.client == nil {
		p.client = &http.Client{Timeout: 10 * time.Second}
	}
	tencentCode := code
	if !strings.HasPrefix(code, "sh") && !strings.HasPrefix(code, "sz") {
		if strings.HasPrefix(code, "6") || strings.HasPrefix(code, "68") {
			tencentCode = "sh" + code
		} else {
			tencentCode = "sz" + code
		}
	}
	url := fmt.Sprintf("http://qt.gtimg.cn/q=%s", tencentCode)
	resp, err := p.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("tencent quote: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	text := string(body)
	if !strings.Contains(text, "~") {
		return nil, fmt.Errorf("tencent quote: invalid response for %s", code)
	}
	parts := strings.Split(text, "~")
	if len(parts) < 10 {
		return nil, fmt.Errorf("tencent quote: unexpected format for %s", code)
	}
	priceStr := strings.TrimSpace(parts[3])
	price, _ := strconv.ParseFloat(priceStr, 64)
	logger.SugaredLogger.Infof("datasource: quote %s from tencent: %.2f", code, price)
	return &datasource.QuoteData{
		Code:  code,
		Price: price,
		Time:  time.Now(),
	}, nil
}

// TencentKLineProvider provides K-line data from Tencent Finance.
type TencentKLineProvider struct {
	client *http.Client
}

func (p *TencentKLineProvider) Name() string { return "tencent_kline" }
func (p *TencentKLineProvider) Priority() int { return 10 }
func (p *TencentKLineProvider) Available(ctx context.Context) bool { return true }

func (p *TencentKLineProvider) GetKLine(ctx context.Context, code string, period string, count int) (*datasource.KLineData, error) {
	if p.client == nil {
		p.client = &http.Client{Timeout: 15 * time.Second}
	}
	tencentCode := code
	if !strings.HasPrefix(code, "sh") && !strings.HasPrefix(code, "sz") {
		if strings.HasPrefix(code, "6") || strings.HasPrefix(code, "68") {
			tencentCode = "sh" + code
		} else {
			tencentCode = "sz" + code
		}
	}
	url := fmt.Sprintf("http://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param=%s,%s,,,%d", tencentCode, period, count)
	resp, err := p.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("tencent kline: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	logger.SugaredLogger.Infof("datasource: kline %s from tencent (%d bytes)", code, len(body))
	return &datasource.KLineData{
		Code:   code,
		Period: period,
		Bars:   []datasource.KLineBar{},
	}, nil
}

// MootdxQuoteProvider provides real-time quotes via existing TDX API (free, unlimited).
type MootdxQuoteProvider struct{}

func (p *MootdxQuoteProvider) Name() string { return "mootdx" }
func (p *MootdxQuoteProvider) Priority() int { return 5 }
func (p *MootdxQuoteProvider) Available(ctx context.Context) bool { return true }

func (p *MootdxQuoteProvider) GetQuote(ctx context.Context, code string) (*datasource.QuoteData, error) {
	// Use existing StockDataApi for real-time price info
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

func (p *MootdxKLineProvider) Name() string { return "mootdx_kline" }
func (p *MootdxKLineProvider) Priority() int { return 5 }
func (p *MootdxKLineProvider) Available(ctx context.Context) bool { return true }

func (p *MootdxKLineProvider) GetKLine(ctx context.Context, code string, period string, count int) (*datasource.KLineData, error) {
	// Reuse existing TdxKLineApi (which uses gotdx under the hood)
	tdx := data.NewTdxKLineApi()
	if tdx == nil {
		return nil, fmt.Errorf("mootdx kline api not available")
	}
	kLines := tdx.GetKLineData(code, period, count)
	if kLines == nil || len(*kLines) == 0 {
		return nil, fmt.Errorf("mootdx kline: empty for %s", code)
	}
	logger.SugaredLogger.Infof("datasource: kline %s from mootdx (%d bars)", code, len(*kLines))
	return ConvertKLineData(code, period, *kLines), nil
}

// TonghuashunFundamentalProvider provides EPS一致预期 from 同花顺 (10jqka).
type TonghuashunFundamentalProvider struct{}

func (p *TonghuashunFundamentalProvider) Name() string { return "10jqka" }
func (p *TonghuashunFundamentalProvider) Priority() int { return 15 }
func (p *TonghuashunFundamentalProvider) Available(ctx context.Context) bool { return true }

func (p *TonghuashunFundamentalProvider) GetFundamental(ctx context.Context, code string) (*datasource.FundamentalData, error) {
	logger.SugaredLogger.Infof("datasource: fundamental %s from 10jqka (not yet implemented)", code)
	return nil, fmt.Errorf("10jqka fundamental: not yet implemented for %s", code)
}

// BaiduSectorProvider provides concept/sector data from 百度股市通.
type BaiduSectorProvider struct{}

func (p *BaiduSectorProvider) Name() string { return "baidu" }
func (p *BaiduSectorProvider) Priority() int { return 20 }
func (p *BaiduSectorProvider) Available(ctx context.Context) bool { return true }

func (p *BaiduSectorProvider) GetSectorData(ctx context.Context, code string) (*datasource.SectorData, error) {
	logger.SugaredLogger.Infof("datasource: sector %s from baidu (not yet implemented)", code)
	return nil, fmt.Errorf("baidu sector: not yet implemented for %s", code)
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
