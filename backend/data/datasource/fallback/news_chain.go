package fallback

import (
	"context"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/data/datasource"
	"go-stock/backend/logger"
	"time"
)

// WallStreetNewsProvider wraps WallStreetCN as news source (primary).
type WallStreetNewsProvider struct{}

func (p *WallStreetNewsProvider) Name() string                      { return "wallstreetcn" }
func (p *WallStreetNewsProvider) Priority() int                     { return 10 }
func (p *WallStreetNewsProvider) Available(ctx context.Context) bool { return true }

func (p *WallStreetNewsProvider) GetNews(ctx context.Context, code string, count int) ([]datasource.NewsItem, error) {
	ws := data.WallstreetcnApi{}
	markdown := ws.SearchNews(code, 1, count)
	if markdown == "" {
		return nil, fmt.Errorf("wallstreetcn: empty result for %s", code)
	}

	items := []datasource.NewsItem{
		{
			Title:   fmt.Sprintf("关于 %s 的资讯", code),
			Content: markdown,
			Source:  "华尔街见闻",
			Time:    time.Now(),
		},
	}
	logger.SugaredLogger.Infof("datasource: news %s from wallstreetcn (%d chars)", code, len(markdown))
	return items, nil
}

// EastMoneyNewsProvider wraps EastMoney news as first fallback.
type EastMoneyNewsProvider struct{}

func (p *EastMoneyNewsProvider) Name() string                      { return "eastmoney_news" }
func (p *EastMoneyNewsProvider) Priority() int                     { return 20 }
func (p *EastMoneyNewsProvider) Available(ctx context.Context) bool { return true }

func (p *EastMoneyNewsProvider) GetNews(ctx context.Context, code string, count int) ([]datasource.NewsItem, error) {
	em := data.NewEmAPI()
	if em == nil {
		return nil, fmt.Errorf("eastmoney news api not available")
	}

	result, err := em.FinanceSearch(code)
	if err != nil {
		return nil, fmt.Errorf("eastmoney news: %w", err)
	}

	if result == nil || result.Content == "" {
		return nil, fmt.Errorf("eastmoney news: empty result for %s", code)
	}

	items := []datasource.NewsItem{
		{
			Title:   fmt.Sprintf("关于 %s 的资讯", code),
			Content: result.Content,
			Source:  "东方财富",
			Time:    time.Now(),
		},
	}
	logger.SugaredLogger.Infof("datasource: news %s from eastmoney (%d chars)", code, len(result.Content))
	return items, nil
}

// CailiansheNewsProvider wraps local news DB as fallback.
type CailiansheNewsProvider struct{}

func (p *CailiansheNewsProvider) Name() string                      { return "cailianshe" }
func (p *CailiansheNewsProvider) Priority() int                     { return 30 }
func (p *CailiansheNewsProvider) Available(ctx context.Context) bool { return true }

func (p *CailiansheNewsProvider) GetNews(ctx context.Context, code string, count int) ([]datasource.NewsItem, error) {
	newsApi := data.MarketNewsApi{}
	telegraphs := newsApi.GetNewsList("", count)
	if telegraphs == nil || len(*telegraphs) == 0 {
		return nil, fmt.Errorf("cailianshe: empty result for %s", code)
	}

	items := make([]datasource.NewsItem, 0, len(*telegraphs))
	for _, t := range *telegraphs {
		if t == nil {
			continue
		}
		itemTime := time.Now()
		if t.DataTime != nil {
			itemTime = *t.DataTime
		}
		items = append(items, datasource.NewsItem{
			Title:   t.Title,
			Content: t.Content,
			Source:  t.Source,
			Time:    itemTime,
		})
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("cailianshe: empty result for %s", code)
	}
	logger.SugaredLogger.Infof("datasource: news %s from local db (%d items)", code, len(items))
	return items, nil
}

func RegisterNewsChain(router *datasource.Router) {
	router.RegisterNewsProvider(&WallStreetNewsProvider{})
	router.RegisterNewsProvider(&EastMoneyNewsProvider{})
	router.RegisterNewsProvider(&CailiansheNewsProvider{})
}
