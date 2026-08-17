package datasource

import (
	"context"

	"go-stock/backend/data/datasource/freestockdb"
	portds "go-stock/backend/internal/port/datasource"
)

// 编译期断言：FreestockdbProvider 实现 SectorProvider/QuoteProvider/KLineProvider。
// 注意 legacy data 包的数据类型（datasource.KLineData/QuoteData/SectorData）是
// portds 同名类型的别名，字段完全一致，故显式逐字段映射仅在层间解耦上有意义。
var (
	_ portds.SectorProvider = (*FreestockdbProvider)(nil)
	_ portds.QuoteProvider  = (*FreestockdbProvider)(nil)
	_ portds.KLineProvider  = (*FreestockdbProvider)(nil)
)

// FreestockdbProvider 适配器：包装 legacy freestockdb.Provider 实现端口接口。
// 供六边形装配（internal Router）使用；依赖 freestockdb.Manager 由装配方注入。
// 注意：生产装配 main.go 已把 freestockdb 注册进 data 层 Router，此处为
// internal 层可选注册，避免两套 Router 各自拉起 Manager 进程。
type FreestockdbProvider struct {
	p *freestockdb.Provider
}

func NewFreestockdbProvider(p *freestockdb.Provider) *FreestockdbProvider {
	return &FreestockdbProvider{p: p}
}

func (a *FreestockdbProvider) Name() string  { return a.p.Name() }
func (a *FreestockdbProvider) Priority() int { return a.p.Priority() }
func (a *FreestockdbProvider) Available(ctx context.Context) bool {
	return a.p.Available(ctx)
}

func (a *FreestockdbProvider) GetKLine(ctx context.Context, code, period string, count int) (*portds.KLineData, error) {
	kd, err := a.p.GetKLine(ctx, code, period, count)
	if err != nil {
		return nil, err
	}
	bars := make([]portds.KLineBar, len(kd.Bars))
	for i, b := range kd.Bars {
		bars[i] = portds.KLineBar{
			Time:      b.Time,
			Open:      b.Open,
			High:      b.High,
			Low:       b.Low,
			Close:     b.Close,
			PrevClose: b.PrevClose,
			Volume:    b.Volume,
			Amount:    b.Amount,
		}
	}
	return &portds.KLineData{Code: kd.Code, Period: kd.Period, Bars: bars}, nil
}

func (a *FreestockdbProvider) GetQuote(ctx context.Context, code string) (*portds.QuoteData, error) {
	q, err := a.p.GetQuote(ctx, code)
	if err != nil {
		return nil, err
	}
	return &portds.QuoteData{
		Code:      q.Code,
		Name:      q.Name,
		Price:     q.Price,
		Change:    q.Change,
		ChangePct: q.ChangePct,
		Volume:    q.Volume,
		Amount:    q.Amount,
		High:      q.High,
		Low:       q.Low,
		Open:      q.Open,
		PrevClose: q.PrevClose,
		Time:      q.Time,
	}, nil
}

func (a *FreestockdbProvider) GetSectorData(ctx context.Context, code string) (*portds.SectorData, error) {
	sd, err := a.p.GetSectorData(ctx, code)
	if err != nil {
		return nil, err
	}
	return &portds.SectorData{
		Code:       sd.Code,
		Sector:     sd.Sector,
		Rank:       sd.Rank,
		FlowAmount: sd.FlowAmount,
	}, nil
}
