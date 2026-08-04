package handler

import (
	"go-stock/backend/data"
	"go-stock/backend/models"
)

// NewsHandler handles news-related Wails bindings.
type NewsHandler struct{}

// NewNewsHandler creates a new NewsHandler.
func NewNewsHandler() *NewsHandler {
	return &NewsHandler{}
}

// GetTelegraphList returns telegraph (fast news) list for the given source.
func (h *NewsHandler) GetTelegraphList(source string) *[]*models.Telegraph {
	telegraphs := data.NewMarketNewsApi().GetTelegraphList(source)
	return telegraphs
}

// ReFleshTelegraphList refreshes telegraph/Sina/TradingView news and returns the latest telegraph list.
func (h *NewsHandler) ReFleshTelegraphList(source string) *[]*models.Telegraph {
	//data.NewMarketNewsApi().GetNewTelegraph(30)
	go data.NewMarketNewsApi().TelegraphList(30)
	go data.NewMarketNewsApi().GetSinaNews(30)
	go data.NewMarketNewsApi().TradingViewNews()
	telegraphs := data.NewMarketNewsApi().GetTelegraphList(source)
	return telegraphs
}

// GetNewsBySector returns sector-related news.
func (h *NewsHandler) GetNewsBySector(sectorID string, limit int) (*data.SectorNewsResponse, error) {
	api := data.NewMarketNewsApi()
	return api.GetNewsBySector(sectorID, limit)
}

// GetStockRelatedNews returns news related to a specific stock.
func (h *NewsHandler) GetStockRelatedNews(code string, limit int) ([]data.SectorNewsItem, error) {
	api := data.NewMarketNewsApi()
	return api.GetStockRelatedNews(code, limit)
}

// GetSectors returns the predefined sector list used for news classification.
func (h *NewsHandler) GetSectors() []data.Sector {
	return data.NewsSectors
}
