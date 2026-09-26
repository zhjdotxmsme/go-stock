package handler

import (
	"context"

	"go-stock/backend/data/khunter"
	"go-stock/backend/emitter"
	"go-stock/backend/models"
)

// KhunterHandler fronts khunter.Service for the Wails binding layer.
// It follows the project handler convention: event push goes through an
// injected emitter.Emitter (desktop=EventsEmit adapter, mobile=v3 Event
// adapter), so the handler itself stays GUI-runtime-agnostic.
type KhunterHandler struct {
	svc  *khunter.Service
	emit emitter.Emitter
}

func NewKhunterHandler(emit emitter.Emitter) *KhunterHandler {
	if emit == nil {
		emit = emitter.Discard
	}
	return &KhunterHandler{svc: khunter.NewService(), emit: emit}
}

// RunPipelineAsync 异步执行狩猎场流水线，结果经 khunter:progress 事件推送
func (h *KhunterHandler) RunPipelineAsync(tradeDate string) map[string]any {
	go func() {
		res, err := h.svc.RunPipeline(tradeDate)
		if err != nil {
			h.emit("khunter:progress", map[string]any{"done": true, "error": err.Error()})
		} else {
			h.emit("khunter:progress", map[string]any{"done": true, "result": res})
		}
	}()
	return map[string]any{"started": true}
}

func (h *KhunterHandler) GetScores(date string) ([]models.KhunterScore, error) {
	return h.svc.GetScores(date)
}

func (h *KhunterHandler) GetSignals(date string) ([]models.KhunterSignal, error) {
	return h.svc.GetSignals(date)
}

func (h *KhunterHandler) GetHunting(status string) ([]models.KhunterHunting, error) {
	return h.svc.GetHunting(status)
}

// GetKellySuggestions 各策略半凯利建议仓位（配置缺失返回空列表）
func (h *KhunterHandler) GetKellySuggestions() []khunter.KellySuggestion {
	return h.svc.GetKellySuggestions()
}

func (h *KhunterHandler) GetRiskLevel() (*models.KhunterRiskLevel, error) {
	return h.svc.GetRiskLevel()
}

// RunBacktestAsync 异步回测：进度经 khunter:backtest_progress 推送，
// 完成（或失败）经 khunter:backtest_done 推送
func (h *KhunterHandler) RunBacktestAsync(codes []string, startDate, endDate string, holdingDays int) map[string]any {
	go func() {
		stats, err := khunter.BacktestStrategies(context.Background(), codes, startDate, endDate, holdingDays,
			func(done, total int) {
				h.emit("khunter:backtest_progress", map[string]any{"done": done, "total": total})
			})
		payload := map[string]any{"stats": stats}
		if err != nil {
			payload["error"] = err.Error()
		}
		h.emit("khunter:backtest_done", payload)
	}()
	return map[string]any{"started": true}
}
