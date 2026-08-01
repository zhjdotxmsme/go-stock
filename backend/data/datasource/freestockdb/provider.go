package freestockdb

import (
	"context"
	"fmt"
	"time"

	"go-stock/backend/data/datasource"
	"go-stock/backend/logger"
)

// Provider 同时实现 KLineProvider / QuoteProvider / SectorProvider。
type Provider struct {
	m      *Manager
	svc    *KLineService
	boards *BoardIndex
}

func NewProvider(m *Manager, svc *KLineService, bi *BoardIndex) *Provider {
	return &Provider{m: m, svc: svc, boards: bi}
}

func (p *Provider) Name() string                       { return "freestockdb" }
func (p *Provider) Priority() int                      { return 5 }
func (p *Provider) Available(ctx context.Context) bool { return p.m.Available(ctx) }

var periodFreq = map[string]Frequency{
	"101": Freq1d, "102": Freq1w, "103": Freq1M,
	"1": Freq1m, "5": Freq5m, "15": Freq15m, "30": Freq30m, "60": Freq60m,
}

// GetKLine period 为 datasource 周期码（"day"/"101"/"5m"/"5" 均可）。
func (p *Provider) GetKLine(ctx context.Context, code, period string, count int) (*datasource.KLineData, error) {
	freq, ok := periodFreq[datasource.NormalizePeriod(period)]
	if !ok {
		return nil, fmt.Errorf("freestockdb: unsupported period %q", period)
	}
	bars, err := p.svc.LastN(ctx, code, freq, count, FQQFQ)
	if err != nil {
		return nil, err
	}
	if len(bars) == 0 {
		return nil, fmt.Errorf("freestockdb: empty result for %s", code)
	}
	logger.SugaredLogger.Infof("datasource: kline %s from freestockdb (%d bars)", code, len(bars))
	return toKLineData(code, period, bars), nil
}

// GetQuote 用最新一根日K（不复权）实现报价。
func (p *Provider) GetQuote(ctx context.Context, code string) (*datasource.QuoteData, error) {
	bars, err := p.svc.LastN(ctx, code, Freq1d, 1, FQNone)
	if err != nil {
		return nil, err
	}
	if len(bars) == 0 {
		return nil, fmt.Errorf("freestockdb: no quote for %s", code)
	}
	b := bars[0]
	return &datasource.QuoteData{
		Code:      code,
		Name:      b.Name,
		Price:     b.Close,
		Change:    round(b.Close-b.PreClose, 2),
		ChangePct: round(pctChg(b), 2),
		Volume:    int64(b.Volume),
		Amount:    b.Amount,
		High:      b.High,
		Low:       b.Low,
		Open:      b.Open,
		PrevClose: b.PreClose,
		Time:      barTime(b.Date),
	}, nil
}

func pctChg(b Bar) float64 {
	if b.PreClose == 0 {
		return 0
	}
	return (b.Close - b.PreClose) / b.PreClose * 100
}

// GetSectorData 返回股票所属的第一个概念板块（与现有 sector chain 语义一致）。
func (p *Provider) GetSectorData(ctx context.Context, code string) (*datasource.SectorData, error) {
	items := p.boards.OfStock(code, CategoryConcept)
	if len(items) == 0 {
		return nil, fmt.Errorf("freestockdb: no board for %s", code)
	}
	return &datasource.SectorData{Code: code, Sector: items[0].Name}, nil
}

func barTime(date int64) time.Time {
	if date > 1e12 {
		if t, err := time.Parse("20060102150405", fmt.Sprintf("%d", date)); err == nil {
			return t
		}
	}
	if t, err := time.Parse("20060102", fmt.Sprintf("%d", date)); err == nil {
		return t
	}
	return time.Time{}
}

func toKLineData(code, period string, bars []Bar) *datasource.KLineData {
	dst := &datasource.KLineData{Code: code, Period: period, Bars: make([]datasource.KLineBar, 0, len(bars))}
	for _, b := range bars {
		dst.Bars = append(dst.Bars, datasource.KLineBar{
			Time:      barTime(b.Date),
			Open:      b.Open,
			High:      b.High,
			Low:       b.Low,
			Close:     b.Close,
			PrevClose: b.PreClose,
			Volume:    int64(b.Volume),
			Amount:    b.Amount,
		})
	}
	return dst
}

// Setup 拉起引擎、预载因子与板块索引，并把 Provider 注册进三条链。
// 引擎不可用时仅记录日志，Router 会自然降级到 TDX → 东财。
func Setup(router *datasource.Router, cfg Config) *Manager {
	m := NewManager(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := m.Start(ctx); err != nil {
		logger.SugaredLogger.Warnf("freestockdb: %v（降级使用远程数据源）", err)
	}
	client := m.Client()
	factors := NewFactorStore()
	bi := NewBoardIndex()
	if m.Available(ctx) {
		if err := factors.Load(ctx, client); err != nil {
			logger.SugaredLogger.Warnf("freestockdb: 复权因子加载失败: %v", err)
		}
		if err := bi.Load(ctx, client); err != nil {
			logger.SugaredLogger.Warnf("freestockdb: 板块索引加载失败: %v", err)
		}
	}
	p := NewProvider(m, NewKLineService(client, factors), bi)
	router.RegisterKLineProvider(p)
	router.RegisterQuoteProvider(p)
	router.RegisterSectorProvider(p)
	return m
}
