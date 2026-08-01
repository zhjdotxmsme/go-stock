package freestockdb

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"go-stock/backend/data/datasource"
	"go-stock/backend/logger"
)

// Provider 同时实现 KLineProvider / QuoteProvider / SectorProvider。
type Provider struct {
	m      *Manager
	svc    *KLineService
	boards *BoardIndex
	// ready 在 Setup 后台 goroutine 完成复权因子与板块索引预载后置位；
	// 未就绪时 Available=false，避免 qfq 请求拿到未复权透传数据并被 Router 缓存/落盘。
	ready atomic.Bool
}

func NewProvider(m *Manager, svc *KLineService, bi *BoardIndex) *Provider {
	return &Provider{m: m, svc: svc, boards: bi}
}

func (p *Provider) Name() string { return "freestockdb" }

// Priority 取 1：free_data 链上已有 priority=5 的 provider，并列时排序稳定性无保障。
func (p *Provider) Priority() int { return 1 }

// Available 要求引擎可用且因子/板块索引预载完成（见 ready 注释）。
func (p *Provider) Available(ctx context.Context) bool { return p.m.Available(ctx) && p.ready.Load() }

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

// sharesPerLot 股/手换算系数。实证口径：freestockdb 服务端 volume 单位为股，
// 而现有 K 线链（东财 push2his，见 eastmoney_kline_api.go parseKLine 注释
// "成交量 (手)"；TDX 协议 K 线 Vol 同为手）对外统一为手，故此处换算对齐。
const sharesPerLot = 100.0

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
		Volume:    int64(b.Volume / sharesPerLot), // 股 → 手，与东财/腾讯 quote 链口径一致
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

// toKLineData 转换 Bar 为链上 KLineData。Volume 由股换算为手（见 sharesPerLot 注释）。
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
			Volume:    int64(b.Volume / sharesPerLot),
			Amount:    b.Amount,
		})
	}
	return dst
}

// Setup 同步把 Provider 注册进 kline/sector 两条链（Router 链立刻存在，引擎未就绪时
// Available=false 自然降级到 TDX → 东财），拉起引擎与预载因子/板块索引
// 在后台 goroutine 中异步完成，避免阻塞启动路径。
// 日内实时报价仍由东财/腾讯链承担，故不注册 quote 链（规格 §5.5）。
func Setup(router *datasource.Router, cfg Config) *Manager {
	m := NewManager(cfg)
	client := m.Client()
	factors := NewFactorStore()
	bi := NewBoardIndex()
	p := NewProvider(m, NewKLineService(client, factors), bi)
	router.RegisterKLineProvider(p)
	router.RegisterSectorProvider(p)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		if err := m.Start(ctx); err != nil {
			logger.SugaredLogger.Warnf("freestockdb: %v（降级使用远程数据源）", err)
		}
		if m.Available(ctx) {
			if err := factors.Load(ctx, client); err != nil {
				logger.SugaredLogger.Warnf("freestockdb: 复权因子加载失败: %v", err)
			}
			if err := bi.Load(ctx, client); err != nil {
				logger.SugaredLogger.Warnf("freestockdb: 板块索引加载失败: %v", err)
			}
			// 因子与板块索引预载完成后才对外可用，防止 qfq 未复权透传被 Router 缓存/落盘。
			p.ready.Store(true)
		}
	}()
	return m
}
