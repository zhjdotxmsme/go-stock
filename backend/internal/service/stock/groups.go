// groups.go 分组操作。
// 用户文案与原 handler 逐字一致(添加成功/添加失败等)。
package stock

import (
	"context"

	domainstock "go-stock/backend/internal/domain/stock"
)

// AddGroup 新增分组;保留传入 sort,sort 冲突时由实现把既有分组后移。
func (s *Service) AddGroup(ctx context.Context, group domainstock.Group) string {
	if err := s.repo.AddGroup(ctx, group); err != nil {
		return "添加失败"
	}
	return "添加成功"
}

// GetGroupList 获取全部分组(按 sort 升序)
func (s *Service) GetGroupList(ctx context.Context) ([]domainstock.Group, error) {
	return s.repo.GetGroupList(ctx)
}

// UpdateGroupSort 更新分组排序
func (s *Service) UpdateGroupSort(ctx context.Context, groupID, newSort int) bool {
	return s.repo.UpdateGroupSort(ctx, groupID, newSort) == nil
}

// InitializeGroupSort 按创建时间重排全部分组
func (s *Service) InitializeGroupSort(ctx context.Context) bool {
	return s.repo.InitializeGroupSort(ctx) == nil
}

// GetGroupStockList 获取分组内的股票关联(含分组信息)
func (s *Service) GetGroupStockList(ctx context.Context, groupID int) ([]domainstock.GroupStock, error) {
	return s.repo.GetGroupStockList(ctx, groupID)
}

// AddStockGroup 把股票加入分组
func (s *Service) AddStockGroup(ctx context.Context, groupID int, stockCode string) string {
	if err := s.repo.AddStockToGroup(ctx, groupID, stockCode); err != nil {
		return "添加失败"
	}
	return "添加成功"
}

// RemoveStockGroup 把股票移出分组
func (s *Service) RemoveStockGroup(ctx context.Context, stockCode, stockName string, groupID int) string {
	if err := s.repo.RemoveStockFromGroup(ctx, groupID, stockCode, stockName); err != nil {
		return "移除失败"
	}
	return "移除成功"
}

// RemoveGroup 删除分组(原版文案为"移除成功/移除失败")
func (s *Service) RemoveGroup(ctx context.Context, groupID int) string {
	if err := s.repo.RemoveGroup(ctx, groupID); err != nil {
		return "移除失败"
	}
	return "移除成功"
}
