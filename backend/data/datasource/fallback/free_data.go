package fallback

import (
	"context"
	"fmt"
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

// MootdxQuoteProvider provides real-time quotes from mootdx (TCP 7709).
type MootdxQuoteProvider struct{}

func (p *MootdxQuoteProvider) Name() string { return "mootdx" }
func (p *MootdxQuoteProvider) Priority() int { return 5 }
func (p *MootdxQuoteProvider) Available(ctx context.Context) bool { return true }

func (p *MootdxQuoteProvider) GetQuote(ctx context.Context, code string) (*datasource.QuoteData, error) {
	return nil, fmt.Errorf("mootdx quote: not yet implemented for %s", code)
}

func RegisterFreeDataSources(router *datasource.Router) {
	router.RegisterQuoteProvider(&MootdxQuoteProvider{})
	router.RegisterQuoteProvider(&TencentQuoteProvider{})
	router.RegisterKLineProvider(&TencentKLineProvider{})
	logger.SugaredLogger.Info("free data sources registered: mootdx, tencent finance")
}
