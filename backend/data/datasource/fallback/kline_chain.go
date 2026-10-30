package fallback

import (
	"context"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/data/datasource"
	"go-stock/backend/logger"
	"go-stock/backend/util/timeutil"
	"strconv"
	"strings"
	"sync"
	"time"
)

// TDXKLineProvider was removed: MootdxKLineProvider (free_data.go, priority 5)
// wraps the exact same upstream (data.NewTdxKLineApi().GetKLineData), so the
// duplicate only added a guaranteed second TDX attempt on failure.
//
// Live K-line chain: mootdx_kline(5, TDX) → tencent_kline(10) →
// eastmoney_kline(20) → yahoo_kline(25, non-A-share, circuit gated).

// emKlineThrottleMu/emKlineLastFetch 东财 kline 域粘性节流：最小请求间隔 300ms。
var (
	emKlineThrottleMu sync.Mutex
	emKlineLastFetch  time.Time
)

func emKlineThrottle(ctx context.Context) {
	emKlineThrottleMu.Lock()
	wait := 300*time.Millisecond - time.Since(emKlineLastFetch)
	if wait > 0 {
		emKlineLastFetch = time.Now().Add(wait)
		emKlineThrottleMu.Unlock()
		select {
		case <-ctx.Done():
		case <-time.After(wait):
		}
		return
	}
	emKlineLastFetch = time.Now()
	emKlineThrottleMu.Unlock()
}

// EastMoneyKLineProvider wraps EastMoney K-line data source (fallback).
type EastMoneyKLineProvider struct{}

func (p *EastMoneyKLineProvider) Name() string                      { return "eastmoney_kline" }
func (p *EastMoneyKLineProvider) Priority() int                     { return 20 }
func (p *EastMoneyKLineProvider) Available(ctx context.Context) bool { return true }

func (p *EastMoneyKLineProvider) GetKLine(ctx context.Context, code string, period string, count int) (*datasource.KLineData, error) {
	em := data.NewEastMoneyKLineApi(data.GetSettingConfig())
	if em == nil {
		return nil, fmt.Errorf("eastmoney kline api not available")
	}
	period = datasource.NormalizePeriod(period)
	adjustFlag := "1" // 前复权
	kLines := em.GetKLineData(code, period, adjustFlag, count)
	if kLines == nil || len(*kLines) == 0 {
		// Try without adjust
		kLines = em.GetKLineData(code, period, "", count)
	}
	if kLines == nil || len(*kLines) == 0 {
		return nil, fmt.Errorf("eastmoney kline: empty result for %s", code)
	}
	logger.SugaredLogger.Infof("datasource: kline %s from eastmoney (%d bars)", code, len(*kLines))
	return ConvertKLineData(code, period, *kLines), nil
}

// GetKLineBefore 取 endDate（YYYY-MM-DD）当日及之前的 count 根 K 线。
// 历史缺口回补用：东财 kline 接口原生支持 end+lmt 锚定语义。
// 粘性节流：批量回补时 6 个 worker 并发打同一域名，高频会触发限流掐连接
//（CLAUDE.md 已知噪音），最小间隔 300ms 串行化对东财 kline 域的请求。
func (p *EastMoneyKLineProvider) GetKLineBefore(ctx context.Context, code string, period string, count int, endDate string) (*datasource.KLineData, error) {
	emKlineThrottle(ctx)
	em := data.NewEastMoneyKLineApi(data.GetSettingConfig())
	if em == nil {
		return nil, fmt.Errorf("eastmoney kline api not available")
	}
	period = datasource.NormalizePeriod(period)
	// 东财 end 参数格式 YYYYMMDD；锚定语义为"取 end 之前的 limit 根"（含 end 当日）
	end := strings.ReplaceAll(endDate, "-", "")
	adjustFlag := "1" // 前复权
	kLines := em.GetKLineDataBefore(code, period, adjustFlag, count, end)
	if kLines == nil || len(*kLines) == 0 {
		kLines = em.GetKLineDataBefore(code, period, "", count, end)
	}
	if kLines == nil || len(*kLines) == 0 {
		return nil, fmt.Errorf("eastmoney kline before %s: empty result for %s", endDate, code)
	}
	out := ConvertKLineData(code, period, *kLines)
	// 统一接口语义：只保留 endDate 当日及之前的 bars
	kept := make([]datasource.KLineBar, 0, len(out.Bars))
	for _, b := range out.Bars {
		if b.Time.Format("2006-01-02") <= endDate {
			kept = append(kept, b)
		}
	}
	out.Bars = kept
	if len(out.Bars) == 0 {
		return nil, fmt.Errorf("eastmoney kline before %s: no bars on/before anchor for %s", endDate, code)
	}
	logger.SugaredLogger.Infof("datasource: kline %s before %s from eastmoney (%d bars)", code, endDate, len(out.Bars))
	return out, nil
}

// RegisterKLineChain registers all K-line providers with the router.
func RegisterKLineChain(router *datasource.Router) {
	router.RegisterKLineProvider(&EastMoneyKLineProvider{})
	router.RegisterKLineProvider(NewYahooKLineProvider()) // Yahoo Finance: global stocks, indices
}

// ConvertKLineData converts legacy data.KLineData (string fields) to datasource.KLineData (float64 fields).
func ConvertKLineData(code, period string, src []data.KLineData) *datasource.KLineData {
	dst := &datasource.KLineData{
		Code:   code,
		Period: period,
		Bars:   make([]datasource.KLineBar, 0, len(src)),
	}
	for _, k := range src {
		bar := datasource.KLineBar{
			Open:   parseFloat64(k.Open),
			High:   parseFloat64(k.High),
			Low:    parseFloat64(k.Low),
			Close:  parseFloat64(k.Close),
			Volume: parseInt64(k.Volume),
		}
		// 统一走 timeutil：本地时区解析（time.Parse 会给 UTC，与本地语义数据比较会偏 8h）
		bar.Time = timeutil.MustParseDateTime(k.Day)
		dst.Bars = append(dst.Bars, bar)
	}
	return dst
}

func parseFloat64(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func parseInt64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}
