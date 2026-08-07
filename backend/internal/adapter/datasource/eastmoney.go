package datasource

import (
	"context"

	"go-stock/backend/data"
	"go-stock/backend/db"
	portds "go-stock/backend/internal/port/datasource"
)

// EastMoneyKLineProvider 东方财富 K 线数据源（包装 data.EastMoneyKLineApi）。
// 对应 data.FetchKLineWithFallback 链中的第 2 顺位（priority 20）。
// 支持 A 股/港股/美股（东财 secid 覆盖）。
//
// 注：data 包没有东财个股实时行情 API（push2 quote 仅用于期货），
// 故本适配器只实现 KLineProvider，不实现 QuoteProvider。
type EastMoneyKLineProvider struct{}

// NewEastMoneyKLineProvider 创建东方财富 K 线数据源适配器。
func NewEastMoneyKLineProvider() *EastMoneyKLineProvider {
	return &EastMoneyKLineProvider{}
}

func (p *EastMoneyKLineProvider) Name() string { return "eastmoney" }

func (p *EastMoneyKLineProvider) Priority() int { return 20 }

// Available 惰性可用性声明：不主动发探测请求，真实可用性由 GetKLine 结果体现。
func (p *EastMoneyKLineProvider) Available(ctx context.Context) bool { return true }

func (p *EastMoneyKLineProvider) GetKLine(ctx context.Context, code, period string, count int) (*portds.KLineData, error) {
	if db.Dao == nil {
		return nil, errConfigUnavailable
	}
	klt := normalizePeriod(period)
	rows := data.NewEastMoneyKLineApi(data.GetSettingConfig()).GetKLineData2(code, klt, "", count)
	return klineRowsToPort(code, period, rows)
}
