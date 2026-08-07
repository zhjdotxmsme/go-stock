package repository

import (
	"context"

	"go-stock/backend/internal/domain/market"
)

// TelegraphRepository abstracts persistence for telegraph (fast news) entities.
// Implementations live in backend/internal/adapter/repository/sqlite.
type TelegraphRepository interface {
	// GetTelegraphList 按 source 过滤（空=全部），按 data_time desc,time desc 排序，
	// 最多返回 limit 条；SubjectTags 经 telegraph_tags/tags 关联补全（与原 data 层一致）。
	GetTelegraphList(ctx context.Context, source string, limit int) ([]market.Telegraph, error)
}
