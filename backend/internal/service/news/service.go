// Package news 新闻服务
// 该层只依赖 port 接口,不直接引用 data/db。
// 本切片承载新闻域的 DB 读路径:电报列表查询;
// 外部拉取(财联社/新浪/TradingView)保留在 handler 层直连 data。
package news

import (
	"context"

	"go-stock/backend/internal/domain/market"
	"go-stock/backend/internal/port/repository"
)

// telegraphListLimit 电报列表默认条数（与原 data 层 Limit(50) 一致）。
const telegraphListLimit = 50

// Service 新闻服务
type Service struct {
	repo repository.TelegraphRepository
}

// NewService 创建新闻服务。
func NewService(repo repository.TelegraphRepository) *Service {
	return &Service{repo: repo}
}

// GetTelegraphList 返回电报列表（source 空=全部，最多 50 条，含标签补全）。
func (s *Service) GetTelegraphList(ctx context.Context, source string) ([]market.Telegraph, error) {
	return s.repo.GetTelegraphList(ctx, source, telegraphListLimit)
}
