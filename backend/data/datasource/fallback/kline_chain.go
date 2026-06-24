package fallback

import (
	"context"
	"go-stock/backend/data/datasource"
)

// TDXKLineProvider wraps TDX K-line data source.
type TDXKLineProvider struct{}

func (p *TDXKLineProvider) Name() string                      { return "tdx_kline" }
func (p *TDXKLineProvider) Priority() int                     { return 10 }
func (p *TDXKLineProvider) Available(ctx context.Context) bool { return true }

func (p *TDXKLineProvider) GetKLine(ctx context.Context, code string, period string, count int) (*datasource.KLineData, error) {
	return nil, datasource.ErrAllSourcesFailed
}

// EastMoneyKLineProvider wraps EastMoney K-line data source.
type EastMoneyKLineProvider struct{}

func (p *EastMoneyKLineProvider) Name() string                      { return "eastmoney_kline" }
func (p *EastMoneyKLineProvider) Priority() int                     { return 20 }
func (p *EastMoneyKLineProvider) Available(ctx context.Context) bool { return true }

func (p *EastMoneyKLineProvider) GetKLine(ctx context.Context, code string, period string, count int) (*datasource.KLineData, error) {
	return nil, datasource.ErrAllSourcesFailed
}

// RegisterKLineChain registers all K-line providers with the router.
func RegisterKLineChain(router *datasource.Router) {
	router.RegisterKLineProvider(&TDXKLineProvider{})
	router.RegisterKLineProvider(&EastMoneyKLineProvider{})
}
