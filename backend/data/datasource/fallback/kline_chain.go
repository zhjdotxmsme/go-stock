package fallback

import (
	"context"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/data/datasource"
	"go-stock/backend/logger"
	"strconv"
	"time"
)

// TDXKLineProvider wraps TDX K-line data source (primary).
type TDXKLineProvider struct{}

func (p *TDXKLineProvider) Name() string                      { return "tdx_kline" }
func (p *TDXKLineProvider) Priority() int                     { return 10 }
func (p *TDXKLineProvider) Available(ctx context.Context) bool { return true }

func (p *TDXKLineProvider) GetKLine(ctx context.Context, code string, period string, count int) (*datasource.KLineData, error) {
	tdx := data.NewTdxKLineApi()
	if tdx == nil {
		return nil, fmt.Errorf("tdx kline api not available")
	}
	kLines := tdx.GetKLineData(code, period, count)
	if kLines == nil || len(*kLines) == 0 {
		return nil, fmt.Errorf("tdx kline: empty result for %s", code)
	}
	logger.SugaredLogger.Infof("datasource: kline %s from tdx (%d bars)", code, len(*kLines))
	return convertKLineData(code, period, *kLines), nil
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
	// Map period: "101"=日K, "102"=周K, "103"=月K
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
	return convertKLineData(code, period, *kLines), nil
}

// RegisterKLineChain registers all K-line providers with the router.
func RegisterKLineChain(router *datasource.Router) {
	router.RegisterKLineProvider(&TDXKLineProvider{})
	router.RegisterKLineProvider(&EastMoneyKLineProvider{})
}

// convertKLineData converts legacy data.KLineData (string fields) to datasource.KLineData (float64 fields).
func convertKLineData(code, period string, src []data.KLineData) *datasource.KLineData {
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
		if t, err := time.Parse("2006-01-02 15:04:05", k.Day); err == nil {
			bar.Time = t
		} else if t, err := time.Parse("2006-01-02", k.Day); err == nil {
			bar.Time = t
		}
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
