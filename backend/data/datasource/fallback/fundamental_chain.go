package fallback

import (
	"context"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/data/datasource"
	"go-stock/backend/logger"
	"strings"
)

// isAShareCode reports whether a normalized go-stock code refers to an
// A-share security (the only market the EastMoney DuPont endpoint covers).
func isAShareCode(code string) bool {
	lc := strings.ToLower(code)
	return strings.HasPrefix(lc, "sh") || strings.HasPrefix(lc, "sz") || strings.HasPrefix(lc, "bj")
}

// EastMoneyDuPontFundamentalProvider serves A-share fundamentals from the
// EastMoney F10 DuPont analysis endpoint (structured ROE/net-profit/revenue/
// debt-ratio data), replacing earlier providers that wrote report counts
// into the Revenue field.
type EastMoneyDuPontFundamentalProvider struct{}

func (p *EastMoneyDuPontFundamentalProvider) Name() string                      { return "eastmoney_fund" }
func (p *EastMoneyDuPontFundamentalProvider) Priority() int                     { return 10 }
func (p *EastMoneyDuPontFundamentalProvider) Available(ctx context.Context) bool { return true }

func (p *EastMoneyDuPontFundamentalProvider) GetFundamental(ctx context.Context, code string) (*datasource.FundamentalData, error) {
	if !isAShareCode(code) {
		return nil, fmt.Errorf("%w: eastmoney_fundamental only covers A-shares, skipping %s", datasource.ErrUnsupported, code)
	}

	resp := data.NewStockDataApi().GetStockFinancialInfo(code)
	if resp == nil || !resp.Success || len(resp.Result.Data) == 0 {
		return nil, fmt.Errorf("eastmoney_fundamental: empty duPont data for %s", code)
	}

	// Rows are sorted by REPORT_DATE desc; the first entry is the latest report.
	latest := resp.Result.Data[0]
	logger.SugaredLogger.Infof("datasource: fundamental %s from eastmoney duPont (report %s)", code, latest.REPORTDATE)
	return &datasource.FundamentalData{
		ROE:       latest.ROE,
		Revenue:   latest.TOTALOPERATEINCOME,
		NetProfit: latest.NETPROFIT,
		DebtRatio: latest.DEBTASSETRATIO,
	}, nil
}

// RegisterFundamentalChain registers all fundamental providers with the router.
func RegisterFundamentalChain(router *datasource.Router) {
	router.RegisterFundamentalProvider(&EastMoneyDuPontFundamentalProvider{})
	router.RegisterFundamentalProvider(NewYahooFundamentalProvider()) // Yahoo Finance: global stocks fundamentals
}
