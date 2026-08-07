package datasource

import (
	"context"

	"go-stock/backend/data"
	"go-stock/backend/db"
	portds "go-stock/backend/internal/port/datasource"
	"go-stock/backend/stockcode"
)

// TencentKLineProvider 腾讯 K 线数据源（包装 data.TencentKLineApi，
// FetchKLineWithFallback 链第 4 顺位，priority 40；仅 A 股）。
type TencentKLineProvider struct{}

// NewTencentKLineProvider 创建腾讯 K 线数据源适配器。
func NewTencentKLineProvider() *TencentKLineProvider {
	return &TencentKLineProvider{}
}

func (p *TencentKLineProvider) Name() string { return "tencent" }

func (p *TencentKLineProvider) Priority() int { return 40 }

func (p *TencentKLineProvider) Available(ctx context.Context) bool { return true }

func (p *TencentKLineProvider) GetKLine(ctx context.Context, code, period string, count int) (*portds.KLineData, error) {
	if !stockcode.IsA股(code) {
		return nil, errOnlyAShare
	}
	if db.Dao == nil {
		return nil, errConfigUnavailable
	}
	rows := data.NewTencentKLineApi(data.GetSettingConfig()).GetKLineData(code, normalizePeriod(period), count)
	return klineRowsToPort(code, period, rows)
}

// TencentQuoteProvider 腾讯实时行情数据源（包装 data.StockDataApi 的
// qt.gtimg 实时快照——data 包内个股实时行情的唯一实现，priority 10）。
// 支持 A 股/港股（StockDataApi.GetStockCodeRealTimeData 覆盖 sh/sz/hk）。
type TencentQuoteProvider struct{}

// NewTencentQuoteProvider 创建腾讯实时行情适配器。
func NewTencentQuoteProvider() *TencentQuoteProvider {
	return &TencentQuoteProvider{}
}

func (p *TencentQuoteProvider) Name() string { return "tencent" }

func (p *TencentQuoteProvider) Priority() int { return 10 }

func (p *TencentQuoteProvider) Available(ctx context.Context) bool { return true }

func (p *TencentQuoteProvider) GetQuote(ctx context.Context, code string) (*portds.QuoteData, error) {
	if db.Dao == nil {
		return nil, errConfigUnavailable
	}
	infos, err := data.NewStockDataApi().GetStockCodeRealTimeData(code)
	if err != nil {
		return nil, err
	}
	if infos == nil || len(*infos) == 0 {
		return nil, errNoData
	}
	return stockInfoToQuote(&(*infos)[0]), nil
}
