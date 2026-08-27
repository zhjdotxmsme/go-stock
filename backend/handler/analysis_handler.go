package handler

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"go-stock/backend/data"
	"go-stock/backend/internal/adapter/repository/sqlite"
	domanalysis "go-stock/backend/internal/domain/analysis"
	analysissvc "go-stock/backend/internal/service/analysis"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// AnalysisHandler handles analysis-related Wails bindings.
// DB 密集路径委托 analysis service（port/adapter 分层）;
// 外部 API/计算/文件导出部分仍直连 data 层。
type AnalysisHandler struct {
	ctxFn func() context.Context
	svc   *analysissvc.Service
}

// NewAnalysisHandler creates a new AnalysisHandler with the given service.
// ctxFn should return the current App context (set after Wails startup).
func NewAnalysisHandler(svc *analysissvc.Service, ctxFn func() context.Context) *AnalysisHandler {
	return &AnalysisHandler{ctxFn: ctxFn, svc: svc}
}

// NewDefaultAnalysisHandler wires the production dependencies
// (sqlite repository + 实时价补全) and returns the handler.
// The wiring lives here because backend/internal packages cannot be
// imported by the main package at the repository root.
func NewDefaultAnalysisHandler(ctxFn func() context.Context) *AnalysisHandler {
	// 推荐列表实时价补全：复刻原 data 层逻辑（tushare 代码转换 + 腾讯实时快照）。
	enrichFn := func(items []domanalysis.AiRecommendStocks) {
		stockCodes := make([]string, 0, len(items))
		for _, item := range items {
			stockCodes = append(stockCodes, data.ConvertTushareCodeToStockCode(item.StockCode))
		}
		stockData, _ := data.NewStockDataApi().GetStockCodeRealTimeData(stockCodes...)
		if stockData == nil {
			return
		}
		for _, info := range *stockData {
			for idx, item := range items {
				if data.ConvertTushareCodeToStockCode(item.StockCode) == data.ConvertTushareCodeToStockCode(info.Code) {
					items[idx].StockCurrentPrice = info.Price
					items[idx].StockPrePrice = info.PreClose
					items[idx].StockCurrentPriceTime = info.Date + " " + info.Time
				}
			}
		}
	}
	return NewAnalysisHandler(analysissvc.NewService(sqlite.NewAnalysisRepository(), enrichFn), ctxFn)
}

func (h *AnalysisHandler) currentCtx() context.Context {
	if h.ctxFn != nil {
		return h.ctxFn()
	}
	return context.Background()
}

func (h *AnalysisHandler) SearchStock(words string) map[string]any {
	return data.NewSearchStockApi(words).SearchStock(5000)
}

func (h *AnalysisHandler) GetHotStrategy() map[string]any {
	return data.NewSearchStockApi("").HotStrategy()
}

func (h *AnalysisHandler) AIConfiguredStockPick(query string, topN int) ([]models.DailyPick, error) {
	if topN <= 0 {
		topN = 10
	}

	config, err := data.CallLLMForConfig(query)
	if err != nil {
		logger.SugaredLogger.Warnf("AIConfiguredStockPick: LLM config failed, fallback to default: %v", err)
		return data.NewDailyPickEngine().RunDailyPick(h.currentCtx(), time.Now().Format("2006-01-02"), topN)
	}
	if config.TopN <= 0 {
		config.TopN = topN
	}

	return data.NewDailyPickEngine().RunWithConfig(h.currentCtx(), time.Now().Format("2006-01-02"), config)
}

func (h *AnalysisHandler) GetCustomStrategyList(query models.CustomStrategyQuery) *models.CustomStrategyPageData {
	page, err := h.svc.GetCustomStrategyList(h.currentCtx(), *sqlite.CustomStrategyQueryToDomain(query))
	if err != nil {
		return &models.CustomStrategyPageData{}
	}
	return sqlite.CustomStrategyPageDataFromDomain(page)
}

func (h *AnalysisHandler) GetAllCustomStrategies() *[]models.CustomStrategy {
	list, err := h.svc.GetAllCustomStrategies(h.currentCtx())
	if err != nil {
		return &[]models.CustomStrategy{}
	}
	out := make([]models.CustomStrategy, 0, len(list))
	for i := range list {
		out = append(out, *sqlite.CustomStrategyFromDomain(&list[i]))
	}
	return &out
}

func (h *AnalysisHandler) SaveCustomStrategy(strategy models.CustomStrategy) string {
	return h.svc.SaveCustomStrategy(h.currentCtx(), *sqlite.CustomStrategyToDomain(&strategy))
}

func (h *AnalysisHandler) DeleteCustomStrategy(id uint) string {
	return h.svc.DeleteCustomStrategy(h.currentCtx(), id)
}

func (h *AnalysisHandler) GetAllStocks(page int, pageSize int, name string, technicalIndicators models.TechnicalIndicators) *models.AllStocksResp {
	return data.NewStockDataApi().GetAllStocks(page, pageSize, name, technicalIndicators)
}

func (h *AnalysisHandler) AnalyzeSentiment(text string) models.SentimentResult {
	return data.AnalyzeSentiment(text)
}

func (h *AnalysisHandler) AnalyzeSentimentWithFreqWeight(text string) map[string]any {
	result, cleanFrequencies := data.NewsAnalyze(text, false)
	return map[string]any{
		"result":      result,
		"frequencies": cleanFrequencies,
	}
}

func (h *AnalysisHandler) GetAIResponseResultList(query models.AIResponseResultQuery) *models.AIResponseResultPageData {
	page, err := h.svc.GetAIResponseResultList(h.currentCtx(), *sqlite.AIResponseResultQueryToDomain(query))
	if err != nil {
		return &models.AIResponseResultPageData{}
	}
	return sqlite.AIResponseResultPageDataFromDomain(page)
}

func (h *AnalysisHandler) DeleteAIResponseResult(id uint) string {
	return h.svc.DeleteAIResponseResult(h.currentCtx(), id)
}

func (h *AnalysisHandler) BatchDeleteAIResponseResult(ids []uint) string {
	return h.svc.BatchDeleteAIResponseResult(h.currentCtx(), ids)
}

func (h *AnalysisHandler) SaveAIResponseResult(stockCode, stockName, result, chatId, question string, aiConfigId int) {
	// ModelName 仍从 AI 配置解析（外部配置耦合留 handler），落库走 service
	modelName := data.NewDeepSeekOpenAi(h.currentCtx(), aiConfigId).GetModel()
	if err := h.svc.SaveAIResponseResult(h.currentCtx(), stockCode, stockName, result, chatId, question, modelName); err != nil {
		logger.SugaredLogger.Errorf("failed to save ai response result: %v", err)
	}
}

func (h *AnalysisHandler) GetAIResponseResult(stock string) *models.AIResponseResult {
	item, err := h.svc.GetAIResponseResult(h.currentCtx(), stock)
	if err != nil || item == nil {
		// 原实现记录不存在时返回空结构体指针，保持契约
		return &models.AIResponseResult{}
	}
	return sqlite.AIResponseResultFromDomain(item)
}

func (h *AnalysisHandler) GetAiRecommendStocksList(query models.AiRecommendStocksQuery) *models.AiRecommendStocksPageData {
	page, err := h.svc.GetAiRecommendStocksList(h.currentCtx(), *sqlite.AiRecommendStocksQueryToDomain(query))
	if err != nil {
		return &models.AiRecommendStocksPageData{}
	}
	return sqlite.AiRecommendStocksPageDataFromDomain(page)
}

func (h *AnalysisHandler) DeleteAiRecommendStocks(id uint) string {
	return h.svc.DeleteAiRecommendStocks(h.currentCtx(), id)
}

func (h *AnalysisHandler) UpdateAiRecommendStocksAlert(id uint, enableAlert bool) string {
	return h.svc.UpdateAiRecommendStocksAlert(h.currentCtx(), id, enableAlert)
}

func (h *AnalysisHandler) GetAiRecommendStats() *data.AiRecommendStats {
	stats, err := h.svc.GetAiRecommendStats(h.currentCtx())
	if err != nil {
		return &data.AiRecommendStats{}
	}
	return sqlite.AiRecommendStatsFromDomain(stats)
}

func (h *AnalysisHandler) GetPromptTemplates(name, promptType string) *[]models.PromptTemplate {
	list, err := h.svc.GetPromptTemplates(h.currentCtx(), name, promptType)
	if err != nil {
		return &[]models.PromptTemplate{}
	}
	out := sqlite.PromptTemplateListFromDomain(list)
	return &out
}

func (h *AnalysisHandler) AddPrompt(prompt models.Prompt) string {
	promptTemplate := models.PromptTemplate{
		ID:      prompt.ID,
		Content: prompt.Content,
		Name:    prompt.Name,
		Type:    prompt.Type,
	}
	return h.svc.SavePromptTemplate(h.currentCtx(), *sqlite.PromptTemplateToDomain(&promptTemplate))
}

func (h *AnalysisHandler) DelPrompt(id uint) string {
	return h.svc.DeletePromptTemplate(h.currentCtx(), id)
}

func (h *AnalysisHandler) GetPromptTemplateList(query models.PromptTemplateQuery) *models.PromptTemplatePageData {
	page, err := h.svc.GetPromptTemplateList(h.currentCtx(), *sqlite.PromptTemplateQueryToDomain(query))
	if err != nil {
		return &models.PromptTemplatePageData{}
	}
	return sqlite.PromptTemplatePageDataFromDomain(page)
}

func (h *AnalysisHandler) AddPromptTemplate(template models.PromptTemplate) string {
	return h.svc.SavePromptTemplate(h.currentCtx(), *sqlite.PromptTemplateToDomain(&template))
}

func (h *AnalysisHandler) UpdatePromptTemplate(template models.PromptTemplate) string {
	return h.svc.SavePromptTemplate(h.currentCtx(), *sqlite.PromptTemplateToDomain(&template))
}

func (h *AnalysisHandler) DeletePromptTemplate(id uint) string {
	return h.svc.DeletePromptTemplate(h.currentCtx(), id)
}

func (h *AnalysisHandler) GetMultiAgentPrompts() []models.PromptTemplate {
	list, err := h.svc.GetMultiAgentPrompts(h.currentCtx())
	if err != nil {
		return []models.PromptTemplate{}
	}
	return sqlite.PromptTemplateListFromDomain(list)
}

func (h *AnalysisHandler) UpdateMultiAgentPrompt(roleKey, name, content string) string {
	return h.svc.UpdateMultiAgentPrompt(h.currentCtx(), roleKey, name, content)
}

func (h *AnalysisHandler) ShareAnalysis(stockCode, stockName string) string {
	res := data.NewDeepSeekOpenAi(h.currentCtx(), 0).GetAIResponseResult(stockCode)
	if res != nil && len(res.Content) > 100 {
		analysisTime := res.CreatedAt.Format("2006/01/02")
		response, err := data.SharedHTTPClient.R().SetHeader("ua-x", "go-stock").SetFormData(map[string]string{
			"text":         res.Content,
			"stockCode":    stockCode,
			"stockName":    stockName,
			"analysisTime": analysisTime,
		}).Post("http://go-stock.sparkmemory.top:16688/upload")
		if err != nil {
			return err.Error()
		}
		return response.String()
	}
	return "分析结果异常"
}

func (h *AnalysisHandler) ShareText(text, title string) string {
	text = strings.TrimSpace(text)
	title = strings.TrimSpace(title)
	if text == "" {
		return "内容为空"
	}
	if title == "" {
		title = "AI助手"
	}
	analysisTime := time.Now().Format("2006/01/02")
	response, err := data.SharedHTTPClient.R().SetHeader("ua-x", "go-stock").SetFormData(map[string]string{
		"text":         text,
		"stockCode":    title,
		"stockName":    title,
		"analysisTime": analysisTime,
	}).Post("http://go-stock.sparkmemory.top:16688/upload")
	if err != nil {
		return err.Error()
	}
	return response.String()
}

func (h *AnalysisHandler) SaveAsMarkdown(stockCode, stockName string) string {
	res := data.NewDeepSeekOpenAi(h.currentCtx(), 0).GetAIResponseResult(stockCode)
	if res != nil && len(res.Content) > 100 {
		analysisTime := res.CreatedAt.Format("2006-01-02_15_04_05")
		file, err := runtime.SaveFileDialog(h.currentCtx(), runtime.SaveDialogOptions{
			Title:           "保存为Markdown",
			DefaultFilename: fmt.Sprintf("%s[%s]AI分析结果_%s.md", stockName, stockCode, analysisTime),
			Filters: []runtime.FileFilter{
				{
					DisplayName: "Markdown",
					Pattern:     "*.md;*.markdown",
				},
			},
		})
		if err != nil {
			return err.Error()
		}
		err = os.WriteFile(file, []byte(res.Content), 0644)
		return "已保存至：" + file
	}
	return "分析结果异常,无法保存。"
}

func (h *AnalysisHandler) SaveImage(name, base64Data string) string {
	filePath, err := runtime.SaveFileDialog(h.currentCtx(), runtime.SaveDialogOptions{
		Title:           "保存图片",
		DefaultFilename: name + "AI分析.png",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "PNG 图片",
				Pattern:     "*.png",
			},
		},
	})
	if err != nil || filePath == "" {
		return "文件路径,无法保存。"
	}

	base64Data = strings.ReplaceAll(base64Data, " ", "+")
	base64Data = strings.ReplaceAll(base64Data, "\n", "")
	base64Data = strings.ReplaceAll(base64Data, "\r", "")
	if idx := strings.Index(base64Data, ";base64,"); idx != -1 {
		base64Data = base64Data[idx+8:]
	} else if idx := strings.Index(base64Data, "base64,"); idx != -1 {
		base64Data = base64Data[idx+7:]
	} else if strings.HasPrefix(base64Data, "data:") {
		if commaIdx := strings.Index(base64Data, ","); commaIdx != -1 {
			base64Data = base64Data[commaIdx+1:]
		}
	}
	decodeString, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		decodeString, err = base64.RawStdEncoding.DecodeString(base64Data)
	}
	if err != nil {
		return "文件内容异常,无法保存。" + err.Error()
	}

	err = os.WriteFile(filepath.Clean(filePath), decodeString, os.ModePerm)
	if err != nil {
		return "保存结果异常,无法保存。"
	}
	return filePath
}

func (h *AnalysisHandler) SaveWordFile(filename string, base64Data string) string {
	filePath, err := runtime.SaveFileDialog(h.currentCtx(), runtime.SaveDialogOptions{
		Title:           "保存 Word 文件",
		DefaultFilename: filename,
		Filters: []runtime.FileFilter{
			{DisplayName: "Word 文件", Pattern: "*.docx"},
		},
	})
	if err != nil || filePath == "" {
		return "文件路径,无法保存。"
	}

	base64Data = strings.ReplaceAll(base64Data, " ", "+")
	base64Data = strings.ReplaceAll(base64Data, "\n", "")
	base64Data = strings.ReplaceAll(base64Data, "\r", "")
	if idx := strings.Index(base64Data, ";base64,"); idx != -1 {
		base64Data = base64Data[idx+8:]
	} else if idx := strings.Index(base64Data, "base64,"); idx != -1 {
		base64Data = base64Data[idx+7:]
	} else if strings.HasPrefix(base64Data, "data:") {
		if commaIdx := strings.Index(base64Data, ","); commaIdx != -1 {
			base64Data = base64Data[commaIdx+1:]
		}
	}
	decodeString, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		decodeString, err = base64.RawStdEncoding.DecodeString(base64Data)
	}
	if err != nil {
		return "文件内容异常,无法保存。" + err.Error()
	}
	err = os.WriteFile(filepath.Clean(filePath), decodeString, 0777)
	if err != nil {
		return "保存结果异常,无法保存。"
	}
	return filePath
}
