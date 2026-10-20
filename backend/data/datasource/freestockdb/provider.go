package freestockdb

import (
	"context"
	"fmt"

	"go-stock/backend/data/datasource"
	"go-stock/backend/logger"
	"go-stock/backend/stockcode"
)

type Provider struct {
	m      *Manager
	svc    *KLineService
	boards *BoardIndex
}

func NewProvider(m *Manager, svc *KLineService, bi *BoardIndex) *Provider {
	return &Provider{m: m, svc: svc, boards: bi}
}

func (p *Provider) Name() string                      { return "freestockdb" }
func (p *Provider) Priority() int                     { return 5 }
func (p *Provider) Available(ctx context.Context) bool { return p.m.Available(ctx) }

var periodFreq = map[string]Frequency{
	"101": Freq1d, "102": Freq1w, "103": Freq1M,
	"1": Freq1m, "5": Freq5m, "15": Freq15m, "30": Freq30m, "60": Freq60m,
}

func (p *Provider) GetKLine(ctx context.Context, code, period string, count int) (*datasource.KLineData, error) {
	bare, err := EngineCode(code)
	if err != nil {
		return nil, err
	}
	freq, ok := periodFreq[datasource.NormalizePeriod(period)]
	if !ok {
		return nil, fmt.Errorf("freestockdb: unsupported period %q", period)
	}
	bars, err := p.svc.LastN(ctx, bare, freq, count, FQQFQ)
	if err != nil {
		return nil, err
	}
	if len(bars) == 0 {
		return nil, fmt.Errorf("freestockdb: empty result for %s", code)
	}
	outCode := stockcode.Normalize(code)
	logger.SugaredLogger.Infof("datasource: kline %s from freestockdb (%d bars)", outCode, len(bars))
	return toKLineData(outCode, period, bars), nil
}

func (p *Provider) GetQuote(ctx context.Context, code string) (*datasource.QuoteData, error) {
	bare, err := EngineCode(code)
	if err != nil {
		return nil, err
	}
	bars, err := p.svc.LastN(ctx, bare, Freq1d, 1, FQNone)
	if err != nil {
		return nil, err
	}
	if len(bars) == 0 {
		return nil, fmt.Errorf("freestockdb: no quote for %s", code)
	}
	b := bars[0]
	outCode := stockcode.Normalize(code)
	return &datasource.QuoteData{
		Code:      outCode,
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

func (p *Provider) GetSectorData(ctx context.Context, code string) (*datasource.SectorData, error) {
	items := p.boards.OfStock(code, CategoryConcept)
	if len(items) == 0 {
		return nil, fmt.Errorf("freestockdb: no board for %s", code)
	}
	return &datasource.SectorData{Code: stockcode.Normalize(code), Sector: items[0].Name}, nil
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
