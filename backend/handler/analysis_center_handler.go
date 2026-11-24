package handler

import (
	"go-stock/backend/data/analysis"
	"go-stock/backend/data/khunter"
)

// AnalysisCenterHandler 统一分析中心 Wails 绑定（选股预测族唯一新入口）。
type AnalysisCenterHandler struct {
	svc *analysis.AnalysisCenterService
}

func NewAnalysisCenterHandler() *AnalysisCenterHandler {
	return &AnalysisCenterHandler{svc: analysis.NewAnalysisCenterService()}
}

func (h *AnalysisCenterHandler) GetRecommendations(tradeDate string, topN int) ([]analysis.RecommendRow, error) {
	return h.svc.GetRecommendations(tradeDate, topN)
}

func (h *AnalysisCenterHandler) GetAnalysisStatus() (analysis.AnalysisCenterStatus, error) {
	return h.svc.GetAnalysisStatus()
}

func (h *AnalysisCenterHandler) GetForecast(code string, signalDate string) (string, error) {
	return h.svc.GetForecast(code, signalDate)
}

func (h *AnalysisCenterHandler) SubmitPredictJob(code string, predLen int, asOfDate string) (khunter.PredictJob, error) {
	return h.svc.SubmitPredictJob(code, predLen, asOfDate)
}

func (h *AnalysisCenterHandler) GetPredictJob(id string) (khunter.PredictJob, error) {
	return h.svc.GetPredictJob(id)
}

// RunPipeline 触发智能流水线
func (h *AnalysisCenterHandler) RunPipeline(tradeDate string) error {
	return h.svc.RunPipeline(tradeDate)
}

// GetPipelineStatus 查询流水线状态
func (h *AnalysisCenterHandler) GetPipelineStatus() (analysis.PipelineStatus, error) {
	return h.svc.GetPipelineStatus()
}

// GetPipelineResult 获取流水线结果
func (h *AnalysisCenterHandler) GetPipelineResult(tradeDate string, topN int) (analysis.PipelineResult, error) {
	return h.svc.GetPipelineResult(tradeDate, topN)
}

// GetPipelineDetail 获取单只股票详情
func (h *AnalysisCenterHandler) GetPipelineDetail(code, signalDate string) (analysis.PipelineDetail, error) {
	return h.svc.GetPipelineDetail(code, signalDate)
}
