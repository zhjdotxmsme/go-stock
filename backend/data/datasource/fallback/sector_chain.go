package fallback

import (
	"context"
	"go-stock/backend/data/datasource"
)

type TDXSectorProvider struct{}

func (p *TDXSectorProvider) Name() string                      { return "tdx_sector" }
func (p *TDXSectorProvider) Priority() int                     { return 10 }
func (p *TDXSectorProvider) Available(ctx context.Context) bool { return true }

func (p *TDXSectorProvider) GetSectorData(ctx context.Context, code string) (*datasource.SectorData, error) {
	return nil, datasource.ErrAllSourcesFailed
}

type EastMoneySectorProvider struct{}

func (p *EastMoneySectorProvider) Name() string                      { return "eastmoney_sector" }
func (p *EastMoneySectorProvider) Priority() int                     { return 20 }
func (p *EastMoneySectorProvider) Available(ctx context.Context) bool { return true }

func (p *EastMoneySectorProvider) GetSectorData(ctx context.Context, code string) (*datasource.SectorData, error) {
	return nil, datasource.ErrAllSourcesFailed
}

func RegisterSectorChain(router *datasource.Router) {
	router.RegisterSectorProvider(&TDXSectorProvider{})
	router.RegisterSectorProvider(&EastMoneySectorProvider{})
}
