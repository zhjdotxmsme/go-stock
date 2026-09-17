package handler

import (
	"context"

	"go-stock/backend/data"
	"go-stock/backend/emitter"
	"go-stock/backend/models"
)

// DailyPickHandler fronts data.DailyPickService for the Wails binding layer.
// The service itself is GUI-runtime-agnostic (S4): it emits progress through
// an injected emitter, and this handler bridges that emitter to the Wails
// event bus using the app context.
type DailyPickHandler struct {
	svc   *data.DailyPickService
	ctxFn func() context.Context
}

// NewDailyPickHandler wraps the service; emit forwards progress events to the
// frontend (desktop=EventsEmit adapter, mobile=v3 Event adapter).
func NewDailyPickHandler(svc *data.DailyPickService, emit emitter.Emitter) *DailyPickHandler {
	if emit == nil {
		emit = emitter.Discard
	}
	svc.WithEmitter(func(event string, payload map[string]any) {
		emit(event, payload)
	})
	return &DailyPickHandler{svc: svc}
}

// NewDefaultDailyPickHandler wires the production service with the emitter
// bridge. The wiring lives here because the main package cannot import
// backend/internal packages.
func NewDefaultDailyPickHandler(emit emitter.Emitter) *DailyPickHandler {
	return NewDailyPickHandler(data.InitDailyPickService(), emit)
}

func (h *DailyPickHandler) RunDailyPick(tradeDate string, topN int) ([]models.DailyPick, error) {
	return h.svc.RunDailyPick(tradeDate, topN)
}

func (h *DailyPickHandler) RunDailyPickAsync(tradeDate string, topN int) {
	h.svc.RunDailyPickAsync(tradeDate, topN)
}

func (h *DailyPickHandler) RunDailyReview(reviewDate string, pickDate string) int {
	return h.svc.RunDailyReview(reviewDate, pickDate)
}

func (h *DailyPickHandler) ReviewAllUnreviewed() int {
	return h.svc.ReviewAllUnreviewed()
}

func (h *DailyPickHandler) GetReviewSummary(tradeDate string) map[string]interface{} {
	return h.svc.GetReviewSummary(tradeDate)
}

func (h *DailyPickHandler) GetDailyPicks(query models.DailyPickQuery) models.DailyPickPageData {
	return h.svc.GetDailyPicks(query)
}

func (h *DailyPickHandler) GetLatestPicks(topN int) []models.DailyPick {
	return h.svc.GetLatestPicks(topN)
}

func (h *DailyPickHandler) DeleteDailyPick(id uint) error {
	return h.svc.DeleteDailyPick(id)
}

func (h *DailyPickHandler) UpdateDailyPickRemarks(id uint, remarks string) error {
	return h.svc.UpdateDailyPickRemarks(id, remarks)
}

func (h *DailyPickHandler) GetDailyPickStats() models.DailyPickStats {
	return h.svc.GetDailyPickStats()
}

func (h *DailyPickHandler) GetLatestUnreviewedPicks() []models.DailyPick {
	return h.svc.GetLatestUnreviewedPicks()
}

func (h *DailyPickHandler) GetDateRange() (string, string) {
	return h.svc.GetDateRange()
}

func (h *DailyPickHandler) GetReviewTrend(limit int) []map[string]interface{} {
	return h.svc.GetReviewTrend(limit)
}

// GetLLMRankingEnabled 返回 AI 增强选股（LLM 二次排序）开关状态；未设置时默认开启。
func (h *DailyPickHandler) GetLLMRankingEnabled() bool {
	sc := data.GetSettingConfig()
	if sc == nil || sc.EnableLLMRanking == nil {
		return true
	}
	return *sc.EnableLLMRanking
}

// SetLLMRankingEnabled 持久化 AI 增强选股开关。
func (h *DailyPickHandler) SetLLMRankingEnabled(enabled bool) string {
	sc := data.GetSettingConfig()
	if sc == nil || sc.Settings == nil {
		return "保存失败: 配置未就绪"
	}
	sc.EnableLLMRanking = &enabled
	return data.UpdateConfig(sc)
}
