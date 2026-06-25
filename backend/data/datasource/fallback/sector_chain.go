package fallback

import (
	"context"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/data/datasource"
	"go-stock/backend/logger"
)

// TDXSectorProvider wraps TDX sector data source.
type TDXSectorProvider struct{}

func (p *TDXSectorProvider) Name() string                      { return "tdx_sector" }
func (p *TDXSectorProvider) Priority() int                     { return 10 }
func (p *TDXSectorProvider) Available(ctx context.Context) bool { return true }

func (p *TDXSectorProvider) GetSectorData(ctx context.Context, code string) (*datasource.SectorData, error) {
	stockApi := data.NewStockDataApi()
	conceptInfo := stockApi.GetStockConceptInfo(code)
	if conceptInfo.Result.Data != nil && len(conceptInfo.Result.Data) > 0 {
		first := conceptInfo.Result.Data[0]
		logger.SugaredLogger.Infof("datasource: sector %s from tdx: %s", code, first.BOARDNAME)
		return &datasource.SectorData{
			Code:   code,
			Sector: first.BOARDNAME,
		}, nil
	}
	return nil, fmt.Errorf("tdx sector: empty for %s", code)
}

// EastMoneySectorProvider wraps EastMoney sector data as fallback.
type EastMoneySectorProvider struct{}

func (p *EastMoneySectorProvider) Name() string                      { return "eastmoney_sector" }
func (p *EastMoneySectorProvider) Priority() int                     { return 20 }
func (p *EastMoneySectorProvider) Available(ctx context.Context) bool { return true }

func (p *EastMoneySectorProvider) GetSectorData(ctx context.Context, code string) (*datasource.SectorData, error) {
	stockApi := data.NewStockDataApi()
	conceptInfo := stockApi.GetStockConceptInfo(code)
	if conceptInfo.Result.Data != nil && len(conceptInfo.Result.Data) > 0 {
		first := conceptInfo.Result.Data[0]
		logger.SugaredLogger.Infof("datasource: sector %s from eastmoney: %s", code, first.BOARDNAME)
		return &datasource.SectorData{
			Code:   code,
			Sector: first.BOARDNAME,
		}, nil
	}
	return nil, fmt.Errorf("eastmoney sector: empty for %s", code)
}

func RegisterSectorChain(router *datasource.Router) {
	router.RegisterSectorProvider(&TDXSectorProvider{})
	router.RegisterSectorProvider(&EastMoneySectorProvider{})
}
