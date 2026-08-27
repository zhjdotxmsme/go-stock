package fallback

import (
	"context"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/data/datasource"
	"go-stock/backend/logger"
	"strconv"
	"strings"
	"time"
)

// The snapshot chain serves rich real-time snapshots (price + level-1 fields
// + pre-close) used by "real-time price with fallback" style reads:
//
//	tencent(10)   — qt.gtimg.cn full snapshot via data.StockDataApi
//	eastmoney(20) — EastMoney real-time page scrape, price/time only
//
// coversAShareSnapshot reports whether the upstreams behind this chain can
// serve the code (A-shares and HK only).
func coversAShareSnapshot(code string) bool {
	lc := strings.ToLower(code)
	return strings.HasPrefix(lc, "sh") || strings.HasPrefix(lc, "sz") || strings.HasPrefix(lc, "hk")
}

// TencentSnapshotProvider wraps the qt.gtimg.cn snapshot (five-level bid/ask,
// pre-close, OHLC) parsed by data.StockDataApi.GetStockCodeRealTimeData.
type TencentSnapshotProvider struct{}

func (p *TencentSnapshotProvider) Name() string                      { return "tencent_snapshot" }
func (p *TencentSnapshotProvider) Priority() int                     { return 10 }
func (p *TencentSnapshotProvider) Available(ctx context.Context) bool { return true }

func (p *TencentSnapshotProvider) GetSnapshot(ctx context.Context, code string) (*datasource.SnapshotData, error) {
	if !coversAShareSnapshot(code) {
		return nil, fmt.Errorf("%w: tencent snapshot does not cover %s", datasource.ErrUnsupported, code)
	}
	stockDatas, err := data.NewStockDataApi().GetStockCodeRealTimeData(code)
	if err != nil {
		return nil, fmt.Errorf("tencent snapshot %s: %w", code, err)
	}
	if stockDatas == nil || len(*stockDatas) == 0 {
		return nil, fmt.Errorf("tencent snapshot: empty for %s", code)
	}
	s := (*stockDatas)[0]
	snap := &datasource.SnapshotData{
		Code:     code,
		Name:     s.Name,
		Price:    toFloat64(s.Price),
		Open:     toFloat64(s.Open),
		PreClose: toFloat64(s.PreClose),
		High:     toFloat64(s.High),
		Low:      toFloat64(s.Low),
		A1P:      toFloat64(s.A1P),
		B1P:      toFloat64(s.B1P),
	}
	if t, err := time.Parse("2006-01-02 15:04:05", strings.TrimSpace(s.Date+" "+s.Time)); err == nil {
		snap.Time = t
	}
	logger.SugaredLogger.Infof("datasource: snapshot %s from tencent (%s)", code, snap.Name)
	return snap, nil
}

// EastMoneySnapshotProvider wraps the EastMoney real-time page scrape. It can
// only fill price and time; downstream consumers must apply their own
// pre-close/level-1 fallbacks, which arrive as zeros here.
type EastMoneySnapshotProvider struct{}

func (p *EastMoneySnapshotProvider) Name() string                      { return "eastmoney_snapshot" }
func (p *EastMoneySnapshotProvider) Priority() int                     { return 20 }
func (p *EastMoneySnapshotProvider) Available(ctx context.Context) bool { return true }

func (p *EastMoneySnapshotProvider) GetSnapshot(ctx context.Context, code string) (*datasource.SnapshotData, error) {
	price, priceTime := data.GetRealTimeStockPriceInfo(ctx, code)
	if price == "" {
		return nil, fmt.Errorf("eastmoney snapshot: empty price for %s", code)
	}
	priceVal, err := strconv.ParseFloat(strings.TrimSpace(price), 64)
	if err != nil {
		return nil, fmt.Errorf("eastmoney snapshot: parse price %q: %w", price, err)
	}
	var t time.Time
	if priceTime != "" {
		t, _ = time.Parse("2006-01-02 15:04:05", strings.TrimSpace(priceTime))
	}
	if t.IsZero() {
		t = time.Now()
	}
	logger.SugaredLogger.Infof("datasource: snapshot %s from eastmoney: %.2f", code, priceVal)
	return &datasource.SnapshotData{Code: code, Price: priceVal, Time: t}, nil
}

// RegisterSnapshotChain registers all snapshot providers with the router.
func RegisterSnapshotChain(router *datasource.Router) {
	router.RegisterSnapshotProvider(&TencentSnapshotProvider{})
	router.RegisterSnapshotProvider(&EastMoneySnapshotProvider{})
}
