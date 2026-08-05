// Package stockchange 异动监控服务
// 该层只依赖 port 接口,不直接引用 data/db。
// 本切片承载异动历史域:实时异动项落库(去重)、历史分页查询、纯 DB 统计聚合;
// 实时外部数据由 handler 拉取后以 domain 类型传入,避免反向依赖 data 层。
package stockchange

import (
	"context"
	"fmt"
	"time"

	"go-stock/backend/internal/domain/stock"
	"go-stock/backend/internal/port/repository"
)

// Service 异动监控服务
type Service struct {
	repo repository.StockRepository
}

// NewService 创建异动监控服务。
func NewService(repo repository.StockRepository) *Service {
	return &Service{repo: repo}
}

// GetStockChangeHistory 获取异动历史分页数据;Page<=0 时按第 1 页处理(与原 data 层一致)。
func (s *Service) GetStockChangeHistory(ctx context.Context, query stock.StockChangeHistoryQuery) (stock.StockChangeHistoryPageData, error) {
	if query.Page <= 0 {
		query.Page = 1
	}
	return s.repo.GetStockChangeHistory(ctx, query)
}

// SaveStockChangesToHistory 保存实时异动项到历史(三列去重)。
// ChangeDate 统一填当日;文案与原 data 层逐字一致。
func (s *Service) SaveStockChangesToHistory(ctx context.Context, items []stock.StockChangeItem) string {
	if len(items) == 0 {
		return "没有获取到异动数据"
	}
	histories := itemsToHistories(items, false)
	if err := s.repo.SaveStockChangesToHistory(ctx, histories); err != nil {
		return "保存失败: " + err.Error()
	}
	return fmt.Sprintf("成功保存 %d 条异动数据", len(items))
}

// SaveStockChangesWithDedup 保存实时异动项到历史(全字段去重),返回实际新增条数。
func (s *Service) SaveStockChangesWithDedup(ctx context.Context, items []stock.StockChangeItem) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}
	return s.repo.SaveStockChangesToHistoryWithDedup(ctx, itemsToHistories(items, true))
}

// DeleteStockChangeHistory 删除 days 天前的历史数据;文案与原 data 层逐字一致。
func (s *Service) DeleteStockChangeHistory(ctx context.Context, days int) string {
	cutoffDate := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	if err := s.repo.DeleteStockChangeHistoryBefore(ctx, cutoffDate); err != nil {
		return "删除失败: " + err.Error()
	}
	return fmt.Sprintf("已删除 %d 天前的历史数据", days)
}

// GetDailyChangeStats 每日异动统计(涨/跌/涨停/跌停)。
func (s *Service) GetDailyChangeStats(ctx context.Context, days int) ([]stock.DailyChangeStats, error) {
	return s.repo.GetDailyChangeStats(ctx, startDateOf(days))
}

// GetChangeTypeDailyStats 异动类型每日统计。
func (s *Service) GetChangeTypeDailyStats(ctx context.Context, days int) ([]stock.ChangeTypeDailyStats, error) {
	return s.repo.GetChangeTypeDailyStats(ctx, startDateOf(days))
}

// GetChangeRank 异动榜单(个股/行业/概念);topN<=0 时默认 20(与原 data 层一致)。
func (s *Service) GetChangeRank(ctx context.Context, days int, topN int) (*stock.ChangeRankResult, error) {
	if topN <= 0 {
		topN = 20
	}
	return s.repo.GetChangeRank(ctx, startDateOf(days), topN)
}

// GetDailyDimensionStats 按维度(个股/行业/概念/类型)的每日涨跌统计。
func (s *Service) GetDailyDimensionStats(ctx context.Context, dimension string, name string, days int) ([]stock.DailyDimensionStats, error) {
	switch dimension {
	case "stock", "industry", "concept", "type":
	default:
		return nil, fmt.Errorf("unsupported dimension: %s", dimension)
	}
	return s.repo.GetDailyDimensionStats(ctx, dimension, name, startDateOf(days))
}

// GetTypeStatsByDate 指定日期的异动类型统计。
func (s *Service) GetTypeStatsByDate(ctx context.Context, date string) ([]stock.TypeCountStats, error) {
	return s.repo.GetTypeStatsByDate(ctx, date)
}

// startDateOf 计算 days 天前的日期(YYYY-MM-DD),与原 data 层一致。
func startDateOf(days int) string {
	return time.Now().AddDate(0, 0, -days).Format("2006-01-02")
}

// itemsToHistories 实时异动项 -> 历史记录,ChangeDate 统一填当日。
// withDimensions 为 true 时带 Industry/Concept(对应全字段去重口径)。
func itemsToHistories(items []stock.StockChangeItem, withDimensions bool) []stock.StockChangeHistory {
	today := time.Now().Format("2006-01-02")
	histories := make([]stock.StockChangeHistory, 0, len(items))
	for _, item := range items {
		history := stock.StockChangeHistory{
			ChangeTime: item.Time,
			ChangeDate: today,
			StockCode:  item.Code,
			StockName:  item.Name,
			Market:     item.Market,
			ChangeType: item.ChangeType,
			TypeName:   item.TypeName,
			Volume:     item.Volume,
			Price:      item.Price,
			ChangeRate: item.ChangeRate,
			Amount:     item.Amount,
		}
		if withDimensions {
			history.Industry = item.Industry
			history.Concept = item.Concept
		}
		histories = append(histories, history)
	}
	return histories
}
