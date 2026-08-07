package handler

import (
	"context"

	"go-stock/backend/data"
	"go-stock/backend/internal/adapter/repository/sqlite"
	newssvc "go-stock/backend/internal/service/news"
	"go-stock/backend/models"
)

// NewsHandler handles news-related Wails bindings.
// 电报列表 DB 读路径委托 news service（port/adapter 分层）;
// 外部拉取（财联社/新浪/TradingView）与板块新闻编排仍直连 data 层。
type NewsHandler struct {
	svc *newssvc.Service
}

// NewNewsHandler creates a new NewsHandler with the given service.
func NewNewsHandler(svc *newssvc.Service) *NewsHandler {
	return &NewsHandler{svc: svc}
}

// NewDefaultNewsHandler wires the production dependencies (sqlite repository)
// and returns the handler. The wiring lives here because backend/internal
// packages cannot be imported by the main package at the repository root.
func NewDefaultNewsHandler() *NewsHandler {
	return NewNewsHandler(newssvc.NewService(sqlite.NewTelegraphRepository()))
}

// GetTelegraphList returns telegraph (fast news) list for the given source.
func (h *NewsHandler) GetTelegraphList(source string) *[]*models.Telegraph {
	list, err := h.svc.GetTelegraphList(context.Background(), source)
	if err != nil {
		return &[]*models.Telegraph{}
	}
	out := sqlite.TelegraphPtrListFromDomain(list)
	return &out
}

// ReFleshTelegraphList refreshes telegraph/Sina/TradingView news and returns the latest telegraph list.
// 外部拉取留 handler 直连 data；最终 DB 读走 service。
func (h *NewsHandler) ReFleshTelegraphList(source string) *[]*models.Telegraph {
	//data.NewMarketNewsApi().GetNewTelegraph(30)
	go data.NewMarketNewsApi().TelegraphList(30)
	go data.NewMarketNewsApi().GetSinaNews(30)
	go data.NewMarketNewsApi().TradingViewNews()
	list, err := h.svc.GetTelegraphList(context.Background(), source)
	if err != nil {
		return &[]*models.Telegraph{}
	}
	out := sqlite.TelegraphPtrListFromDomain(list)
	return &out
}

// GetNewsBySector returns sector-related news.
// 混合编排（触发外部抓取 + DB 读 + 关键词过滤），保留直连 data。
func (h *NewsHandler) GetNewsBySector(sectorID string, limit int) (*data.SectorNewsResponse, error) {
	api := data.NewMarketNewsApi()
	return api.GetNewsBySector(sectorID, limit)
}

// GetStockRelatedNews returns news related to a specific stock.
// 混合编排（触发外部抓取 + DB 读 + 关键词过滤），保留直连 data。
func (h *NewsHandler) GetStockRelatedNews(code string, limit int) ([]data.SectorNewsItem, error) {
	api := data.NewMarketNewsApi()
	return api.GetStockRelatedNews(code, limit)
}

// GetSectors returns the predefined sector list used for news classification.
func (h *NewsHandler) GetSectors() []data.Sector {
	return data.NewsSectors
}
