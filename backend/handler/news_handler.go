package handler

import (
	"context"
	"sync"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/internal/adapter/repository/sqlite"
	newssvc "go-stock/backend/internal/service/news"
	"go-stock/backend/models"
)

// NewsHandler handles news-related Wails bindings.
// 电报列表 DB 读路径委托 news service（port/adapter 分层）;
// 外部拉取（财联社/新浪/TradingView）与板块新闻编排仍直连 data 层。
type NewsHandler struct {
	ctxFn func() context.Context
	svc *newssvc.Service
}

// NewNewsHandler creates a new NewsHandler with the given service.
func NewNewsHandler(ctxFn func() context.Context, svc *newssvc.Service) *NewsHandler {
	return &NewsHandler{ctxFn: ctxFn, svc: svc}
}

// NewDefaultNewsHandler wires the production dependencies (sqlite repository)
// and returns the handler. The wiring lives here because backend/internal
// packages cannot be imported by the main package at the repository root.
func NewDefaultNewsHandler(ctxFn func() context.Context) *NewsHandler {
	return NewNewsHandler(ctxFn, newssvc.NewService(sqlite.NewTelegraphRepository()))
}

// GetTelegraphList returns telegraph (fast news) list for the given source.
func (h *NewsHandler) GetTelegraphList(source string) *[]*models.Telegraph {
	list, err := h.svc.GetTelegraphList(h.currentCtx(), source)
	if err != nil {
		return &[]*models.Telegraph{}
	}
	out := sqlite.TelegraphPtrListFromDomain(list)
	return &out
}

// telegraphCrawlThrottle 快讯外部抓取节流：市场页交易时段每 10 秒对 3 个源各调一次刷新，
// 每次都会起协程抓外部 API 并去重写入（约 18 次抓取/分钟）。同一源 60 秒内只放行一次抓取，
// 之后的刷新仅重新读库返回。
var telegraphCrawlLast = sync.Map{} // source -> time.Time

func shouldCrawlTelegraph(source string, now time.Time) bool {
	if v, ok := telegraphCrawlLast.Load(source); ok {
		if last, ok := v.(time.Time); ok && now.Sub(last) < 60*time.Second {
			return false
		}
	}
	telegraphCrawlLast.Store(source, now)
	return true
}

// ReFleshTelegraphList refreshes telegraph/Sina/TradingView news and returns the latest telegraph list.
// 外部拉取留 handler 直连 data；最终 DB 读走 service。
func (h *NewsHandler) ReFleshTelegraphList(source string) *[]*models.Telegraph {
	if shouldCrawlTelegraph(source, time.Now()) {
		go data.NewMarketNewsApi().TelegraphList(30)
		go data.NewMarketNewsApi().GetSinaNews(30)
		go data.NewMarketNewsApi().TradingViewNews()
	}
	list, err := h.svc.GetTelegraphList(h.currentCtx(), source)
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

// currentCtx returns the Wails app context (set after startup), falling back
// to context.Background when not wired — so in-flight service calls observe
// app shutdown instead of running detached.
func (h *NewsHandler) currentCtx() context.Context {
	if h.ctxFn != nil {
		return h.ctxFn()
	}
	return context.Background()
}
