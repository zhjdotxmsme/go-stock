package fallback

import (
	"context"
	"go-stock/backend/data/datasource"
)

// TDXQuoteProvider wraps the TDX data source as a QuoteProvider.
type TDXQuoteProvider struct{}

func (p *TDXQuoteProvider) Name() string                      { return "tdx" }
func (p *TDXQuoteProvider) Priority() int                     { return 10 }
func (p *TDXQuoteProvider) Available(ctx context.Context) bool { return true }

func (p *TDXQuoteProvider) GetQuote(ctx context.Context, code string) (*datasource.QuoteData, error) {
	// TODO: call existing data.TDX API and map to datasource.QuoteData
	return nil, datasource.ErrAllSourcesFailed
}

// EastMoneyQuoteProvider wraps the EastMoney data source.
type EastMoneyQuoteProvider struct{}

func (p *EastMoneyQuoteProvider) Name() string                      { return "eastmoney" }
func (p *EastMoneyQuoteProvider) Priority() int                     { return 20 }
func (p *EastMoneyQuoteProvider) Available(ctx context.Context) bool { return true }

func (p *EastMoneyQuoteProvider) GetQuote(ctx context.Context, code string) (*datasource.QuoteData, error) {
	return nil, datasource.ErrAllSourcesFailed
}

// SinaQuoteProvider wraps Sina Finance as a fallback quote source.
type SinaQuoteProvider struct{}

func (p *SinaQuoteProvider) Name() string                      { return "sina" }
func (p *SinaQuoteProvider) Priority() int                     { return 30 }
func (p *SinaQuoteProvider) Available(ctx context.Context) bool { return true }

func (p *SinaQuoteProvider) GetQuote(ctx context.Context, code string) (*datasource.QuoteData, error) {
	return nil, datasource.ErrAllSourcesFailed
}

// RegisterQuoteChain registers all quote providers with the router.
func RegisterQuoteChain(router *datasource.Router) {
	router.RegisterQuoteProvider(&TDXQuoteProvider{})
	router.RegisterQuoteProvider(&EastMoneyQuoteProvider{})
	router.RegisterQuoteProvider(&SinaQuoteProvider{})
}
