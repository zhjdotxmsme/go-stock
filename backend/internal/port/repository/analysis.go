package repository

import (
	"context"

	"go-stock/backend/internal/domain/analysis"
)

// AnalysisRepository abstracts persistence for analysis-related entities
// (AI 分析结果 / AI 推荐股票 / 自定义策略 / 提示词模板)。
// Implementations live in backend/internal/adapter/repository/sqlite.
type AnalysisRepository interface {
	// AI 分析结果
	SaveAIResponseResult(ctx context.Context, item *analysis.AIResponseResult) error
	// GetLatestAIResponseResult 按股票代码取最新一条（id desc）；不存在时返回 (nil, nil)。
	GetLatestAIResponseResult(ctx context.Context, stockCode string) (*analysis.AIResponseResult, error)
	GetAIResponseResultList(ctx context.Context, query analysis.AIResponseResultQuery) (*analysis.AIResponseResultPageData, error)
	DeleteAIResponseResult(ctx context.Context, id uint) error
	BatchDeleteAIResponseResult(ctx context.Context, ids []uint) error

	// AI 推荐股票
	// GetAiRecommendStocksList 纯 DB 分页（实时价补全由 service 层注入函数完成）。
	GetAiRecommendStocksList(ctx context.Context, query analysis.AiRecommendStocksQuery) (*analysis.AiRecommendStocksPageData, error)
	DeleteAiRecommendStocks(ctx context.Context, id uint) error
	UpdateAiRecommendStocksAlert(ctx context.Context, id uint, enableAlert bool) error
	// ListAllAiRecommendStocks 返回全部推荐记录（统计计算用）。
	ListAllAiRecommendStocks(ctx context.Context) ([]analysis.AiRecommendStocks, error)

	// 自定义策略
	GetCustomStrategyList(ctx context.Context, query analysis.CustomStrategyQuery) (*analysis.CustomStrategyPageData, error)
	ListAllCustomStrategies(ctx context.Context) ([]analysis.CustomStrategy, error)
	// GetCustomStrategyByID 不存在时返回 (nil, nil)。
	GetCustomStrategyByID(ctx context.Context, id uint) (*analysis.CustomStrategy, error)
	CreateCustomStrategy(ctx context.Context, strategy *analysis.CustomStrategy) error
	// UpdateCustomStrategy 按原 data 层语义只更新 name/query/description/sort_order 四列。
	UpdateCustomStrategy(ctx context.Context, strategy *analysis.CustomStrategy) error
	DeleteCustomStrategy(ctx context.Context, id uint) error

	// 提示词模板
	// GetPromptTemplates 按 name/type 精确匹配（空条件不参与筛选，与原 data 层四分支一致）。
	GetPromptTemplates(ctx context.Context, name, promptType string) ([]analysis.PromptTemplate, error)
	GetPromptTemplateList(ctx context.Context, query analysis.PromptTemplateQuery) (*analysis.PromptTemplatePageData, error)
	// GetPromptTemplateByID 不存在时返回 (nil, nil)。
	GetPromptTemplateByID(ctx context.Context, id uint) (*analysis.PromptTemplate, error)
	// CreatePromptTemplate 按原 data 层语义只写入 name/content/type 三列。
	CreatePromptTemplate(ctx context.Context, template *analysis.PromptTemplate) error
	UpdatePromptTemplate(ctx context.Context, template *analysis.PromptTemplate) error
	DeletePromptTemplate(ctx context.Context, id uint) error
	// ListMultiAgentPrompts 返回 type 为 multi_agent/single_agent 的全部模板。
	ListMultiAgentPrompts(ctx context.Context) ([]analysis.PromptTemplate, error)
	// UpsertPromptByRoleKey 按 role_key 创建（含 ptype）或更新 name/content。
	UpsertPromptByRoleKey(ctx context.Context, roleKey, name, content, ptype string) error
}
