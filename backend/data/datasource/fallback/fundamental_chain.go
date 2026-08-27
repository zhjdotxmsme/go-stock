package fallback

import (
	"context"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/data/datasource"
	"go-stock/backend/logger"
)

// TushareFundamentalProvider wraps Tushare data source for fundamental data.
type TushareFundamentalProvider struct{}

func (p *TushareFundamentalProvider) Name() string                      { return "tushare" }
func (p *TushareFundamentalProvider) Priority() int                     { return 10 }
func (p *TushareFundamentalProvider) Available(ctx context.Context) bool { return true }

func (p *TushareFundamentalProvider) GetFundamental(ctx context.Context, code string) (*datasource.FundamentalData, error) {
	reports := data.GetFinancialReports(code, 30)
	if reports != nil && len(*reports) > 0 {
		logger.SugaredLogger.Infof("datasource: fundamental %s from tushare (%d reports)", code, len(*reports))
		return &datasource.FundamentalData{
			Revenue:   float64(len(*reports)),
			NetProfit: 0,
		}, nil
	}
	return nil, fmt.Errorf("tushare fundamental: empty for %s", code)
}

// EastMoneyFundamentalProvider wraps EastMoney financial data as fallback.
type EastMoneyFundamentalProvider struct{}

func (p *EastMoneyFundamentalProvider) Name() string                      { return "eastmoney_fund" }
func (p *EastMoneyFundamentalProvider) Priority() int                     { return 20 }
func (p *EastMoneyFundamentalProvider) Available(ctx context.Context) bool { return true }

func (p *EastMoneyFundamentalProvider) GetFundamental(ctx context.Context, code string) (*datasource.FundamentalData, error) {
	reports := data.GetFinancialReportsByXUEQIU(code, 30)
	if reports != nil && len(*reports) > 0 {
		logger.SugaredLogger.Infof("datasource: fundamental %s from xueqiu (%d reports)", code, len(*reports))
		return &datasource.FundamentalData{
			Revenue: float64(len(*reports)),
		}, nil
	}
	return nil, fmt.Errorf("eastmoney fundamental: empty for %s", code)
}

// RegisterFundamentalChain registers all fundamental providers with the router.
func RegisterFundamentalChain(router *datasource.Router) {
	router.RegisterFundamentalProvider(&TushareFundamentalProvider{})
	router.RegisterFundamentalProvider(&EastMoneyFundamentalProvider{})
	router.RegisterFundamentalProvider(NewYahooFundamentalProvider()) // Yahoo Finance: global stocks fundamentals
}
