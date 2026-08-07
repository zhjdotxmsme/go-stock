package datasource

import (
	"context"
	"errors"

	"go-stock/backend/data"
	"go-stock/backend/db"
	portds "go-stock/backend/internal/port/datasource"
	"go-stock/backend/stockcode"
)

// 适配器层共享错误（router.go 的 errNoData/errConfigUnavailable 之外）。

// errOnlyAShare 该数据源仅支持 A 股（对齐 FetchKLineWithFallback 的港美股提前返回）。
var errOnlyAShare = errors.New("data source supports A-share only")

// errMACTimeout MAC 行情调用超时。
var errMACTimeout = errors.New("tdx-mac request timeout")

// SinaKLineProvider 新浪 K 线数据源（包装 data.SinaKLineApi，
// FetchKLineWithFallback 链第 3 顺位，priority 30；仅 A 股）。
//
// 注：data 包没有新浪个股实时行情 API（hq.sinajs 仅用于基金/期货/ETF），
// 故本适配器只实现 KLineProvider，不实现 QuoteProvider。
type SinaKLineProvider struct{}

// NewSinaKLineProvider 创建新浪 K 线数据源适配器。
func NewSinaKLineProvider() *SinaKLineProvider {
	return &SinaKLineProvider{}
}

func (p *SinaKLineProvider) Name() string { return "sina" }

func (p *SinaKLineProvider) Priority() int { return 30 }

func (p *SinaKLineProvider) Available(ctx context.Context) bool { return true }

func (p *SinaKLineProvider) GetKLine(ctx context.Context, code, period string, count int) (*portds.KLineData, error) {
	if !stockcode.IsA股(code) {
		return nil, errOnlyAShare
	}
	if db.Dao == nil {
		return nil, errConfigUnavailable
	}
	rows := data.NewSinaKLineApi(data.GetSettingConfig()).GetKLineData(code, normalizePeriod(period), count)
	return klineRowsToPort(code, period, rows)
}
