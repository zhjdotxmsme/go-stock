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
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// AnalysisHandler handles analysis-related Wails bindings.
type AnalysisHandler struct {
	ctxFn func() context.Context
}

// NewAnalysisHandler creates a new AnalysisHandler.
// ctxFn should return the current App context (set after Wails startup).
func NewAnalysisHandler(ctxFn func() context.Context) *AnalysisHandler {
	return &AnalysisHandler{ctxFn: ctxFn}
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
		return data.NewDailyPickEngine().RunDailyPick(context.Background(), time.Now().Format("2006-01-02"), topN)
	}
	if config.TopN <= 0 {
		config.TopN = topN
	}

	return data.NewDailyPickEngine().RunWithConfig(context.Background(), time.Now().Format("2006-01-02"), config)
}

func (h *AnalysisHandler) GetCustomStrategyList(query models.CustomStrategyQuery) *models.CustomStrategyPageData {
	page, err := data.NewCustomStrategyApi().GetCustomStrategyList(&query)
	if err != nil {
		return &models.CustomStrategyPageData{}
	}
	return page
}

func (h *AnalysisHandler) GetAllCustomStrategies() *[]models.CustomStrategy {
	return data.NewCustomStrategyApi().GetAllCustomStrategies()
}

func (h *AnalysisHandler) SaveCustomStrategy(strategy models.CustomStrategy) string {
	return data.NewCustomStrategyApi().SaveCustomStrategy(strategy)
}

func (h *AnalysisHandler) DeleteCustomStrategy(id uint) string {
	return data.NewCustomStrategyApi().DeleteCustomStrategy(id)
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
	page, err := data.NewAIResponseResultService().GetAIResponseResultList(query)
	if err != nil {
		return &models.AIResponseResultPageData{}
	}
	return page
}

func (h *AnalysisHandler) DeleteAIResponseResult(id uint) string {
	err := data.NewAIResponseResultService().DeleteAIResponseResult(id)
	if err != nil {
		return "删除失败"
	}
	return "删除成功"
}

func (h *AnalysisHandler) BatchDeleteAIResponseResult(ids []uint) string {
	err := data.NewAIResponseResultService().BatchDeleteAIResponseResult(ids)
	if err != nil {
		return "删除失败"
	}
	return "删除成功"
}

func (h *AnalysisHandler) SaveAIResponseResult(stockCode, stockName, result, chatId, question string, aiConfigId int) {
	data.NewDeepSeekOpenAi(h.currentCtx(), aiConfigId).SaveAIResponseResult(stockCode, stockName, result, chatId, question)
}

func (h *AnalysisHandler) GetAIResponseResult(stock string) *models.AIResponseResult {
	return data.NewDeepSeekOpenAi(h.currentCtx(), 0).GetAIResponseResult(stock)
}

func (h *AnalysisHandler) GetAiRecommendStocksList(query models.AiRecommendStocksQuery) *models.AiRecommendStocksPageData {
	page, err := data.NewAiRecommendStocksService().GetAiRecommendStocksList(&query)
	if err != nil {
		return &models.AiRecommendStocksPageData{}
	}
	return page
}

func (h *AnalysisHandler) DeleteAiRecommendStocks(id uint) string {
	err := data.NewAiRecommendStocksService().DeleteAiRecommendStocks(id)
	if err != nil {
		return "删除失败"
	}
	return "删除成功"
}

func (h *AnalysisHandler) UpdateAiRecommendStocksAlert(id uint, enableAlert bool) string {
	err := data.NewAiRecommendStocksService().UpdateAiRecommendStocksAlert(id, enableAlert)
	if err != nil {
		return "更新预警状态失败"
	}
	return "更新预警状态成功"
}

func (h *AnalysisHandler) GetAiRecommendStats() *data.AiRecommendStats {
	stats, err := data.NewAiRecommendStocksService().GetAiRecommendStats()
	if err != nil {
		return &data.AiRecommendStats{}
	}
	return stats
}

func (h *AnalysisHandler) GetPromptTemplates(name, promptType string) *[]models.PromptTemplate {
	return data.NewPromptTemplateApi().GetPromptTemplates(name, promptType)
}

func (h *AnalysisHandler) AddPrompt(prompt models.Prompt) string {
	promptTemplate := models.PromptTemplate{
		ID:      prompt.ID,
		Content: prompt.Content,
		Name:    prompt.Name,
		Type:    prompt.Type,
	}
	return data.NewPromptTemplateApi().AddPrompt(promptTemplate)
}

func (h *AnalysisHandler) DelPrompt(id uint) string {
	return data.NewPromptTemplateApi().DelPrompt(id)
}

func (h *AnalysisHandler) GetPromptTemplateList(query models.PromptTemplateQuery) *models.PromptTemplatePageData {
	page, err := data.NewPromptTemplateApi().GetPromptTemplateList(&query)
	if err != nil {
		return &models.PromptTemplatePageData{}
	}
	return page
}

func (h *AnalysisHandler) AddPromptTemplate(template models.PromptTemplate) string {
	return data.NewPromptTemplateApi().AddPrompt(template)
}

func (h *AnalysisHandler) UpdatePromptTemplate(template models.PromptTemplate) string {
	return data.NewPromptTemplateApi().AddPrompt(template)
}

func (h *AnalysisHandler) DeletePromptTemplate(id uint) string {
	return data.NewPromptTemplateApi().DelPrompt(id)
}

func (h *AnalysisHandler) GetMultiAgentPrompts() []models.PromptTemplate {
	return data.GetAllMultiAgentPrompts()
}

func (h *AnalysisHandler) UpdateMultiAgentPrompt(roleKey, name, content string) string {
	err := data.UpsertPromptByRoleKey(roleKey, name, content, "multi_agent")
	if err != nil {
		return "更新失败: " + err.Error()
	}
	return "更新成功"
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
