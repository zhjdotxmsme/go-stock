package fallback

import (
	"context"
	"go-stock/backend/data/datasource"
)

type WallStreetNewsProvider struct{}

func (p *WallStreetNewsProvider) Name() string                      { return "wallstreetcn" }
func (p *WallStreetNewsProvider) Priority() int                     { return 10 }
func (p *WallStreetNewsProvider) Available(ctx context.Context) bool { return true }

func (p *WallStreetNewsProvider) GetNews(ctx context.Context, code string, count int) ([]datasource.NewsItem, error) {
	return nil, datasource.ErrAllSourcesFailed
}

type EastMoneyNewsProvider struct{}

func (p *EastMoneyNewsProvider) Name() string                      { return "eastmoney_news" }
func (p *EastMoneyNewsProvider) Priority() int                     { return 20 }
func (p *EastMoneyNewsProvider) Available(ctx context.Context) bool { return true }

func (p *EastMoneyNewsProvider) GetNews(ctx context.Context, code string, count int) ([]datasource.NewsItem, error) {
	return nil, datasource.ErrAllSourcesFailed
}

type CailiansheNewsProvider struct{}

func (p *CailiansheNewsProvider) Name() string                      { return "cailianshe" }
func (p *CailiansheNewsProvider) Priority() int                     { return 30 }
func (p *CailiansheNewsProvider) Available(ctx context.Context) bool { return true }

func (p *CailiansheNewsProvider) GetNews(ctx context.Context, code string, count int) ([]datasource.NewsItem, error) {
	return nil, datasource.ErrAllSourcesFailed
}

func RegisterNewsChain(router *datasource.Router) {
	router.RegisterNewsProvider(&WallStreetNewsProvider{})
	router.RegisterNewsProvider(&EastMoneyNewsProvider{})
	router.RegisterNewsProvider(&CailiansheNewsProvider{})
}
