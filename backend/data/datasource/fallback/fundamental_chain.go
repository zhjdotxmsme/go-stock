package fallback

import (
	"context"
	"go-stock/backend/data/datasource"
)

type TushareFundamentalProvider struct{}

func (p *TushareFundamentalProvider) Name() string                      { return "tushare" }
func (p *TushareFundamentalProvider) Priority() int                     { return 10 }
func (p *TushareFundamentalProvider) Available(ctx context.Context) bool { return true }

func (p *TushareFundamentalProvider) GetFundamental(ctx context.Context, code string) (*datasource.FundamentalData, error) {
	return nil, datasource.ErrAllSourcesFailed
}

type EastMoneyFundamentalProvider struct{}

func (p *EastMoneyFundamentalProvider) Name() string                      { return "eastmoney_fund" }
func (p *EastMoneyFundamentalProvider) Priority() int                     { return 20 }
func (p *EastMoneyFundamentalProvider) Available(ctx context.Context) bool { return true }

func (p *EastMoneyFundamentalProvider) GetFundamental(ctx context.Context, code string) (*datasource.FundamentalData, error) {
	return nil, datasource.ErrAllSourcesFailed
}

func RegisterFundamentalChain(router *datasource.Router) {
	router.RegisterFundamentalProvider(&TushareFundamentalProvider{})
	router.RegisterFundamentalProvider(&EastMoneyFundamentalProvider{})
}
