// Package analysis 分析服务
// 该层只依赖 port 接口,不直接引用 data/db。
// 本切片承载分析域:AI 分析结果/AI 推荐股票/自定义策略/提示词模板的
// 编排与文案;推荐列表的实时价补全通过 RecommendEnrichFunc 注入,
// 避免反向依赖 data 层。
package analysis

import (
	"context"
	"fmt"
	"math"

	"go-stock/backend/internal/domain/analysis"
	"go-stock/backend/internal/port/repository"
)

// RecommendEnrichFunc 推荐列表实时价补全函数类型:
// 原地填充 StockCurrentPrice/StockPrePrice/StockCurrentPriceTime;可为 nil(不补全)。
type RecommendEnrichFunc func(items []analysis.AiRecommendStocks)

// Service 分析服务
type Service struct {
	repo     repository.AnalysisRepository
	enrichFn RecommendEnrichFunc
}

// NewService 创建分析服务;enrichFn 可为 nil(推荐列表不做实时价补全)。
func NewService(repo repository.AnalysisRepository, enrichFn RecommendEnrichFunc) *Service {
	return &Service{repo: repo, enrichFn: enrichFn}
}

// ---------------------------------------------------------------------------
// AI 分析结果
// ---------------------------------------------------------------------------

// SaveAIResponseResult 保存 AI 分析结果（ModelName 由调用方从 AI 配置解析）。
func (s *Service) SaveAIResponseResult(ctx context.Context, stockCode, stockName, result, chatId, question, modelName string) error {
	return s.repo.SaveAIResponseResult(ctx, &analysis.AIResponseResult{
		StockCode: stockCode,
		StockName: stockName,
		ModelName: modelName,
		Content:   result,
		ChatId:    chatId,
		Question:  question,
	})
}

// GetAIResponseResult 取该股票最新一条分析结果；不存在时返回 (nil, nil)。
func (s *Service) GetAIResponseResult(ctx context.Context, stock string) (*analysis.AIResponseResult, error) {
	return s.repo.GetLatestAIResponseResult(ctx, stock)
}

// GetAIResponseResultList 分页查询 AI 分析结果。
func (s *Service) GetAIResponseResultList(ctx context.Context, query analysis.AIResponseResultQuery) (*analysis.AIResponseResultPageData, error) {
	return s.repo.GetAIResponseResultList(ctx, query)
}

// DeleteAIResponseResult 删除 AI 分析结果;文案与原 handler 逐字一致。
func (s *Service) DeleteAIResponseResult(ctx context.Context, id uint) string {
	if err := s.repo.DeleteAIResponseResult(ctx, id); err != nil {
		return "删除失败"
	}
	return "删除成功"
}

// BatchDeleteAIResponseResult 批量删除 AI 分析结果;文案与原 handler 逐字一致。
func (s *Service) BatchDeleteAIResponseResult(ctx context.Context, ids []uint) string {
	if err := s.repo.BatchDeleteAIResponseResult(ctx, ids); err != nil {
		return "删除失败"
	}
	return "删除成功"
}

// ---------------------------------------------------------------------------
// AI 推荐股票
// ---------------------------------------------------------------------------

// GetAiRecommendStocksList 分页查询推荐记录；返回前做实时价补全（enrichFn 非 nil 时）。
func (s *Service) GetAiRecommendStocksList(ctx context.Context, query analysis.AiRecommendStocksQuery) (*analysis.AiRecommendStocksPageData, error) {
	page, err := s.repo.GetAiRecommendStocksList(ctx, query)
	if err != nil {
		return nil, err
	}
	if s.enrichFn != nil && page != nil {
		s.enrichFn(page.List)
	}
	return page, nil
}

// DeleteAiRecommendStocks 删除推荐记录;文案与原 handler 逐字一致。
func (s *Service) DeleteAiRecommendStocks(ctx context.Context, id uint) string {
	if err := s.repo.DeleteAiRecommendStocks(ctx, id); err != nil {
		return "删除失败"
	}
	return "删除成功"
}

// UpdateAiRecommendStocksAlert 更新预警状态;文案与原 handler 逐字一致。
func (s *Service) UpdateAiRecommendStocksAlert(ctx context.Context, id uint, enableAlert bool) string {
	if err := s.repo.UpdateAiRecommendStocksAlert(ctx, id, enableAlert); err != nil {
		return "更新预警状态失败"
	}
	return "更新预警状态成功"
}

// GetAiRecommendStats 推荐统计（纯计算,逻辑复刻原 data 层）。
func (s *Service) GetAiRecommendStats(ctx context.Context) (*analysis.AiRecommendStats, error) {
	all, err := s.repo.ListAllAiRecommendStocks(ctx)
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return &analysis.AiRecommendStats{}, nil
	}

	// ByModel
	modelMap := make(map[string]struct {
		wins   int
		total  int
		retSum float64
	})
	for _, r := range all {
		key := r.ModelName
		if key == "" {
			key = "unknown"
		}
		entry := modelMap[key]
		entry.total++
		current := parsePrice(r.StockCurrentPrice)
		orig := parsePrice(r.StockPrice)
		if orig > 0 && current > 0 {
			if current > orig {
				entry.wins++
			}
			entry.retSum += (current - orig) / orig
		}
		modelMap[key] = entry
	}
	var byModel []analysis.ModelStat
	for name, m := range modelMap {
		wr := 0.0
		if m.total > 0 {
			wr = float64(m.wins) / float64(m.total) * 100
		}
		avgRet := 0.0
		if m.total > 0 {
			avgRet = m.retSum / float64(m.total) * 100
		}
		byModel = append(byModel, analysis.ModelStat{
			ModelName: name,
			WinRate:   math.Round(wr*10) / 10,
			AvgReturn: math.Round(avgRet*10) / 10,
			Count:     m.total,
		})
	}

	// BySector
	sectorMap := make(map[string]int)
	for _, r := range all {
		key := r.BkName
		if key == "" {
			key = "未知"
		}
		sectorMap[key]++
	}
	var bySector []analysis.SectorStat
	for name, cnt := range sectorMap {
		bySector = append(bySector, analysis.SectorStat{BkName: name, Count: cnt})
	}

	// DailyCount
	dayMap := make(map[string]int)
	for _, r := range all {
		if r.DataTime != nil {
			day := r.DataTime.Format("2006-01-02")
			dayMap[day]++
		}
	}
	var dailyCount []analysis.DailyCount
	for d, cnt := range dayMap {
		dailyCount = append(dailyCount, analysis.DailyCount{Date: d, Count: cnt})
	}

	return &analysis.AiRecommendStats{
		ByModel:    byModel,
		BySector:   bySector,
		DailyCount: dailyCount,
	}, nil
}

// parsePrice 与原 data 层一致：fmt.Sscanf 解析，失败/空串为 0。
func parsePrice(s string) float64 {
	if s == "" {
		return 0
	}
	var v float64
	if _, err := fmt.Sscanf(s, "%f", &v); err == nil {
		return v
	}
	return 0
}

// ---------------------------------------------------------------------------
// 自定义策略
// ---------------------------------------------------------------------------

// GetCustomStrategyList 分页查询自定义策略。
func (s *Service) GetCustomStrategyList(ctx context.Context, query analysis.CustomStrategyQuery) (*analysis.CustomStrategyPageData, error) {
	return s.repo.GetCustomStrategyList(ctx, query)
}

// GetAllCustomStrategies 返回全部自定义策略。
func (s *Service) GetAllCustomStrategies(ctx context.Context) ([]analysis.CustomStrategy, error) {
	return s.repo.ListAllCustomStrategies(ctx)
}

// SaveCustomStrategy 新增/更新策略;文案与原 data 层逐字一致。
func (s *Service) SaveCustomStrategy(ctx context.Context, strategy analysis.CustomStrategy) string {
	if strategy.ID == 0 {
		if err := s.repo.CreateCustomStrategy(ctx, &strategy); err != nil {
			return "添加失败"
		}
		return "添加成功"
	}
	existing, _ := s.repo.GetCustomStrategyByID(ctx, strategy.ID)
	if existing == nil {
		return "策略不存在"
	}
	if err := s.repo.UpdateCustomStrategy(ctx, &strategy); err != nil {
		return "更新失败"
	}
	return "更新成功"
}

// DeleteCustomStrategy 删除策略;文案与原 data 层逐字一致。
func (s *Service) DeleteCustomStrategy(ctx context.Context, id uint) string {
	existing, _ := s.repo.GetCustomStrategyByID(ctx, id)
	if existing == nil {
		return "策略不存在"
	}
	if err := s.repo.DeleteCustomStrategy(ctx, id); err != nil {
		return "删除失败"
	}
	return "删除成功"
}

// ---------------------------------------------------------------------------
// 提示词模板
// ---------------------------------------------------------------------------

// GetPromptTemplates 按 name/type 精确匹配查询模板。
func (s *Service) GetPromptTemplates(ctx context.Context, name, promptType string) ([]analysis.PromptTemplate, error) {
	return s.repo.GetPromptTemplates(ctx, name, promptType)
}

// SavePromptTemplate 新增/更新模板（按 ID 存在性 upsert）;文案与原 data 层逐字一致。
func (s *Service) SavePromptTemplate(ctx context.Context, template analysis.PromptTemplate) string {
	existing, _ := s.repo.GetPromptTemplateByID(ctx, uint(template.ID))
	if existing == nil {
		if err := s.repo.CreatePromptTemplate(ctx, &template); err != nil {
			return "添加失败"
		}
		return "添加成功"
	}
	if err := s.repo.UpdatePromptTemplate(ctx, &template); err != nil {
		return "更新失败"
	}
	return "更新成功"
}

// DeletePromptTemplate 删除模板;文案与原 data 层逐字一致。
func (s *Service) DeletePromptTemplate(ctx context.Context, id uint) string {
	existing, _ := s.repo.GetPromptTemplateByID(ctx, id)
	if existing == nil {
		return "模板信息不存在"
	}
	if err := s.repo.DeletePromptTemplate(ctx, id); err != nil {
		return "删除失败"
	}
	return "删除成功"
}

// GetPromptTemplateList 分页查询模板。
func (s *Service) GetPromptTemplateList(ctx context.Context, query analysis.PromptTemplateQuery) (*analysis.PromptTemplatePageData, error) {
	return s.repo.GetPromptTemplateList(ctx, query)
}

// GetMultiAgentPrompts 返回全部多智能体提示词。
func (s *Service) GetMultiAgentPrompts(ctx context.Context) ([]analysis.PromptTemplate, error) {
	return s.repo.ListMultiAgentPrompts(ctx)
}

// UpdateMultiAgentPrompt 按 roleKey 更新多智能体提示词;文案与原 handler 逐字一致。
func (s *Service) UpdateMultiAgentPrompt(ctx context.Context, roleKey, name, content string) string {
	if err := s.repo.UpsertPromptByRoleKey(ctx, roleKey, name, content, "multi_agent"); err != nil {
		return "更新失败: " + err.Error()
	}
	return "更新成功"
}
