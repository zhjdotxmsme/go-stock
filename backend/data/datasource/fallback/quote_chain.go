package fallback

import (
	"context"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/data/datasource"
	"go-stock/backend/logger"
	"strconv"
	"strings"
	"time"
)

// TDXQuoteProvider wraps TDX as quote source (primary).
type TDXQuoteProvider struct{}

func (p *TDXQuoteProvider) Name() string                      { return "tdx" }
func (p *TDXQuoteProvider) Priority() int                     { return 10 }
func (p *TDXQuoteProvider) Available(ctx context.Context) bool { return true }

func (p *TDXQuoteProvider) GetQuote(ctx context.Context, code string) (*datasource.QuoteData, error) {
	// TDX doesn't have a direct real-time quote function yet
	return nil, fmt.Errorf("tdx quote: not available for %s", code)
}

// EastMoneyQuoteProvider wraps EastMoney as quote source (recommended primary).
type EastMoneyQuoteProvider struct{}

func (p *EastMoneyQuoteProvider) Name() string                      { return "eastmoney" }
func (p *EastMoneyQuoteProvider) Priority() int                     { return 20 }
func (p *EastMoneyQuoteProvider) Available(ctx context.Context) bool { return true }

func (p *EastMoneyQuoteProvider) GetQuote(ctx context.Context, code string) (*datasource.QuoteData, error) {
	price, priceTime := data.GetRealTimeStockPriceInfo(ctx, code)
	if price == "" {
		return nil, fmt.Errorf("eastmoney quote: empty price for %s", code)
	}

	priceVal, err := strconv.ParseFloat(strings.TrimSpace(price), 64)
	if err != nil {
		return nil, fmt.Errorf("eastmoney quote: parse price %q: %w", price, err)
	}

	var t time.Time
	if priceTime != "" {
		t, _ = time.Parse("2006-01-02 15:04:05", strings.TrimSpace(priceTime))
	}
	if t.IsZero() {
		t = time.Now()
	}

	logger.SugaredLogger.Infof("datasource: quote %s from eastmoney: %.2f", code, priceVal)
	return &datasource.QuoteData{
		Code:  code,
		Price: priceVal,
		Time:  t,
	}, nil
}

// SinaQuoteProvider wraps Sina Finance as fallback.
type SinaQuoteProvider struct{}

func (p *SinaQuoteProvider) Name() string                      { return "sina" }
func (p *SinaQuoteProvider) Priority() int                     { return 30 }
func (p *SinaQuoteProvider) Available(ctx context.Context) bool { return true }

func (p *SinaQuoteProvider) GetQuote(ctx context.Context, code string) (*datasource.QuoteData, error) {
	return nil, fmt.Errorf("sina quote: not available for %s", code)
}

// RegisterQuoteChain registers all quote providers with the router.
func RegisterQuoteChain(router *datasource.Router) {
	router.RegisterQuoteProvider(&TDXQuoteProvider{})
	router.RegisterQuoteProvider(&EastMoneyQuoteProvider{})
	router.RegisterQuoteProvider(NewYahooQuoteProvider()) // Yahoo Finance: global stocks, indices, commodities
	router.RegisterQuoteProvider(&SinaQuoteProvider{})
}
