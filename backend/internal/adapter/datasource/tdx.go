package datasource

import (
	"context"
	"time"

	"go-stock/backend/data"
	portds "go-stock/backend/internal/port/datasource"
	"go-stock/backend/stockcode"
)

// 通达信数据源（包装 data.TdxKLineApi），对应 FetchKLineWithFallback 链的
// 两端：MAC 扩展行情为第 1 顺位，普通通达信为最后兜底（第 5 顺位）。

// defaultMACTimeout MAC 行情超时上限（对齐 data.fetchFromMACWithTimeout 的 5s）。
const defaultMACTimeout = 5 * time.Second

// TdxMACKLineProvider 通达信 MAC 扩展行情 K 线（priority 10，全市场含港美股）。
// MAC 连接可能阻塞，调用带超时保护，超时视为失败触发路由 fallback。
type TdxMACKLineProvider struct {
	timeout time.Duration
}

// NewTdxMACKLineProvider 创建 MAC 行情适配器；timeout<=0 时使用默认 5s。
func NewTdxMACKLineProvider(timeout time.Duration) *TdxMACKLineProvider {
	if timeout <= 0 {
		timeout = defaultMACTimeout
	}
	return &TdxMACKLineProvider{timeout: timeout}
}

func (p *TdxMACKLineProvider) Name() string { return "tdx-mac" }

func (p *TdxMACKLineProvider) Priority() int { return 10 }

func (p *TdxMACKLineProvider) Available(ctx context.Context) bool { return true }

func (p *TdxMACKLineProvider) GetKLine(ctx context.Context, code, period string, count int) (*portds.KLineData, error) {
	klt := normalizePeriod(period)

	// 超时保护（与 data.fetchFromMACWithTimeout 同模式）：MAC 客户端阻塞时
	// 不拖住整条 fallback 链。
	type result struct {
		rows *[]data.KLineData
	}
	ch := make(chan result, 1)
	go func() {
		ch <- result{rows: data.NewTdxKLineApi().GetMACKLineData(code, klt, count)}
	}()

	select {
	case r := <-ch:
		return klineRowsToPort(code, period, r.rows)
	case <-time.After(p.timeout):
		return nil, errMACTimeout
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// TdxKLineProvider 普通通达信 K 线（priority 50，兜底；仅 A 股——
// FetchKLineWithFallback 中港美股在新浪/腾讯/通达信之前已提前返回）。
type TdxKLineProvider struct{}

// NewTdxKLineProvider 创建普通通达信 K 线适配器。
func NewTdxKLineProvider() *TdxKLineProvider {
	return &TdxKLineProvider{}
}

func (p *TdxKLineProvider) Name() string { return "tdx" }

func (p *TdxKLineProvider) Priority() int { return 50 }

func (p *TdxKLineProvider) Available(ctx context.Context) bool { return true }

func (p *TdxKLineProvider) GetKLine(ctx context.Context, code, period string, count int) (*portds.KLineData, error) {
	if !stockcode.IsA股(code) {
		return nil, errOnlyAShare
	}
	rows := data.NewTdxKLineApi().GetKLineData(code, normalizePeriod(period), count)
	return klineRowsToPort(code, period, rows)
}
