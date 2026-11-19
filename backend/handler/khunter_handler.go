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
func (h *KhunterHandler) GetMeta() khunter.KhunterMeta {
	return h.svc.GetMeta()
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

func (h *KhunterHandler) GetRunSummaries(limit int) ([]models.KhunterRunSummary, error) {
	return h.svc.GetRunSummaries(limit)
}

// GetTechAnalysis 任意个股的狩猎场同口径技术结论（四项事实 + 六段分析），
// 不依赖是否已关注。tradeDate 为空取今天；传入历史日期只使用该日及之前的 K 线。
func (h *KhunterHandler) GetTechAnalysis(stockCode string, tradeDate string) (map[string]any, error) {
	return khunter.GetTechProfile(context.Background(), stockCode, tradeDate)
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

// RunResearchPipelineAsync 异步历史研究运行（新"历史"按钮入口）：只写研究记录、
// 不写生产表；结果经 khunter:research_progress 推送（result.readiness 携带同步诊断）。
func (h *KhunterHandler) RunResearchPipelineAsync(tradeDate string) map[string]any {
	go func() {
		res, err := h.svc.RunResearchPipeline(tradeDate)
		if err != nil {
			h.emit("khunter:research_progress", map[string]any{"done": true, "error": err.Error()})
		} else {
			h.emit("khunter:research_progress", map[string]any{"done": true, "result": res})
		}
	}()
	return map[string]any{"started": true}
}

// GetObservations 观察列表（status 为空返回全部）；观察不是买入推荐。
func (h *KhunterHandler) GetObservations(status string) ([]models.KhunterObservation, error) {
	return h.svc.GetObservations(status)
}

// RunFullBacktestAsync 异步全链路四组对照回测（新回测按钮入口）：推送 Task 10
// 完整报告（四组共享输入凭证 + 区间/样本外结论 + 案例回放），绝不写生产狩猎池。
func (h *KhunterHandler) RunFullBacktestAsync(codes []string, startDate, endDate string) map[string]any {
	go func() {
		res, err := h.svc.RunFullBacktest(context.Background(), khunter.BacktestRequest{
			Codes: codes, StartDate: startDate, EndDate: endDate,
		})
		payload := map[string]any{"result": res}
		if err != nil {
			payload = map[string]any{"result": nil, "error": err.Error()}
		}
		h.emit("khunter:full_backtest_done", payload)
	}()
	return map[string]any{"started": true}
}
