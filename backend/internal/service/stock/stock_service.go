// Package stock 自选股/群组服务（方案 Step 5.1）
// 该层只依赖 port 接口,不直接引用 data/db。
// 本切片承载自选股(关注列表)与分组域的编排与用户文案;
// 行情叠加数值口径见 profit.go。
package stock

import (
	"go-stock/backend/internal/port/repository"
)

// Service 自选股/群组服务
type Service struct {
	repo repository.StockRepository
}

// NewService 创建自选股/群组服务
func NewService(repo repository.StockRepository) *Service {
	return &Service{repo: repo}
}
