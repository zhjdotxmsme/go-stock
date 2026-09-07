// watchlist.go 自选股(关注列表)操作。
// 用户文案与原 handler/data 层逐字一致,保证前端提示不变:
// 适配层 resultErr 把成功文案折叠为 nil、失败文案原样作为 error 透传。
package stock

import (
	"context"

	domainstock "go-stock/backend/internal/domain/stock"
)

// Follow 关注股票。实现侧内含实时行情校验、63 只上限、去重与默认报警值,
// 失败文案(关注失败/最多只能关注63只股票/已经关注了)由实现透传。
func (s *Service) Follow(ctx context.Context, stockCode string) string {
	if err := s.repo.AddFollow(ctx, stockCode, ""); err != nil {
		return err.Error()
	}
	return "关注成功"
}

// UnFollow 取消关注
func (s *Service) UnFollow(ctx context.Context, stockCode string) string {
	if err := s.repo.RemoveFollow(ctx, stockCode); err != nil {
		return err.Error()
	}
	return "取消关注成功"
}

// GetFollowList 获取分组的自选股列表;groupID<=0 表示全部。
func (s *Service) GetFollowList(ctx context.Context, groupID int) ([]domainstock.FollowedStock, error) {
	return s.repo.GetFollowList(ctx, groupID)
}

// SetCostPriceAndVolume 设置成本价与持仓量
func (s *Service) SetCostPriceAndVolume(ctx context.Context, stockCode string, price float64, volume int64) string {
	if err := s.repo.SetCostPriceAndVolume(ctx, stockCode, price, volume); err != nil {
		return err.Error()
	}
	return "设置成功"
}

// SetTradingPrice 设置开仓/止盈/止损/成本价
func (s *Service) SetTradingPrice(ctx context.Context, stockCode string, entryPrice, takeProfitPrice, stopLossPrice, costPrice float64) string {
	if err := s.repo.SetTradingPrice(ctx, stockCode, entryPrice, takeProfitPrice, stopLossPrice, costPrice); err != nil {
		return err.Error()
	}
	return "设置成功"
}

// SetAlarmChangePercent 设置涨跌幅报警阈值与价格报警线
func (s *Service) SetAlarmChangePercent(ctx context.Context, val, alarmPrice float64, stockCode string) string {
	if err := s.repo.SetAlarmChangePercent(ctx, stockCode, val, alarmPrice); err != nil {
		return err.Error()
	}
	return "设置成功"
}

// SetStockSort 设置自选股排序(原实现无返回值语义,静默失败)
func (s *Service) SetStockSort(ctx context.Context, stockCode string, sort int64) {
	_ = s.repo.SetStockSort(ctx, stockCode, sort)
}
