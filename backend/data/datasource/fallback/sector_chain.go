package fallback

import (
	"context"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/data/datasource"
	"go-stock/backend/logger"
)

// The sector chain registers a single remote provider: EastMoney concept
// info. Earlier TDXSectorProvider/BaiduSectorProvider variants queried the
// exact same upstream (data.StockDataApi.GetStockConceptInfo), so keeping
// one avoids guaranteed triple failure on outage. The local freestockdb
// engine also registers a sector provider (priority 1) in main.go.

// EastMoneySectorProvider wraps EastMoney concept data.
type EastMoneySectorProvider struct{}

func (p *EastMoneySectorProvider) Name() string                      { return "eastmoney_sector" }
func (p *EastMoneySectorProvider) Priority() int                     { return 10 }
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

// RegisterSectorChain registers all sector providers with the router.
func RegisterSectorChain(router *datasource.Router) {
	router.RegisterSectorProvider(&EastMoneySectorProvider{})
}
