// Package trading 交易记录服务
// @Author haipeng
// 该层只依赖 port 接口,不直接引用 data/db。
// 本切片承载交易记录域:金额计算、频繁交易风控、FIFO 持仓统计;
// 实时价格通过 PriceFunc 注入,避免反向依赖 data 层。
package trading

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/internal/domain/stock"
	"go-stock/backend/internal/port/repository"
	"go-stock/backend/logger"
)

// PriceFunc 获取股票实时价格的函数类型(入参为规范化后的代码,如 sh600519),
// 返回 0 或 error 表示暂无可用价格,统计时按无市值处理。
type PriceFunc func(stockCode string) (float64, error)

// Service 交易记录服务
type Service struct {
	repo    repository.StockRepository
	priceFn PriceFunc
}

// NewService 创建交易记录服务;priceFn 可为 nil(统计时持仓市值按 0 计)。
func NewService(repo repository.StockRepository, priceFn PriceFunc) *Service {
	return &Service{
		repo:    repo,
		priceFn: priceFn,
	}
}

// AddTradingRecord 添加交易记录,返回新记录ID。
// 金额计算与时间默认值在 service 完成;收盘价快照填充、落库由仓储层负责。
func (s *Service) AddTradingRecord(ctx context.Context, record *stock.TradingRecord) (uint, error) {
	// 检查频繁交易
	if record.Direction == "买入" {
		canTrade, msg := s.CheckFrequentTrading(ctx, record.StockCode)
		if !canTrade {
			return 0, fmt.Errorf("%s", msg)
		}
	}

	// 自动计算金额（价格 * 数量）
	record.Amount = record.Price * float64(record.Volume)

	// 设置交易时间为当前时间（如果未提供）
	if record.TradingTime.IsZero() {
		record.TradingTime = time.Now()
	}

	if err := s.repo.AddTradingRecord(ctx, record); err != nil {
		return 0, err
	}
	return record.ID, nil
}

// GetTradingRecordList 获取交易记录列表
func (s *Service) GetTradingRecordList(ctx context.Context, query stock.TradingRecordListQuery) (stock.TradingRecordPageData, error) {
	return s.repo.GetTradingRecordList(ctx, query)
}

// GetTradingRecordById 根据ID获取交易记录
func (s *Service) GetTradingRecordById(ctx context.Context, id uint) (*stock.TradingRecord, error) {
	return s.repo.GetTradingRecordById(ctx, id)
}

// UpdateTradingRecord 更新交易记录
func (s *Service) UpdateTradingRecord(ctx context.Context, record *stock.TradingRecord) error {
	if record.ID == 0 {
		return fmt.Errorf("记录ID不能为空")
	}
	// 自动计算金额（价格 * 数量）
	record.Amount = record.Price * float64(record.Volume)
	return s.repo.UpdateTradingRecord(ctx, record)
}

// DeleteTradingRecord 删除交易记录
func (s *Service) DeleteTradingRecord(ctx context.Context, id uint) error {
	return s.repo.DeleteTradingRecord(ctx, id)
}

// CheckFrequentTrading 检查是否频繁交易
// 返回值：(是否可以交易, 提示消息)
// 提示消息文案与原 data 层实现逐字一致。
func (s *Service) CheckFrequentTrading(ctx context.Context, stockCode string) (bool, string) {
	// 检查最近24小时内是否有同一只股票的买入交易日志
	cutoffTime := time.Now().Add(-24 * time.Hour)

	count, err := s.repo.CountBuyTradingRecords(ctx, stockCode, cutoffTime)
	if err != nil {
		logger.SugaredLogger.Errorf("检查频繁交易失败: %s", err.Error())
		return true, "检查频繁交易失败，默认允许交易"
	}

	if count > 0 {
		return false, "最近24小时内已对该股票进行过买入操作，为避免频繁交易，建议稍后再操作"
	}

	// 检查最近7天内的买入次数是否超过限制（5次）
	cutoffTime7Days := time.Now().Add(-7 * 24 * time.Hour)
	count, err = s.repo.CountBuyTradingRecords(ctx, "", cutoffTime7Days)
	if err != nil {
		logger.SugaredLogger.Errorf("检查频繁交易失败: %s", err.Error())
		return true, "检查频繁交易失败，默认允许交易"
	}

	if count >= 5 {
		return false, "最近7天内交易次数已达上限（5次），为避免频繁交易，建议稍后再操作"
	}

	return true, "可以交易"
}

// GetTradingRecordStatistics 获取交易记录统计数据(FIFO 成本法)。
// 统计口径与原 data 层实现一致:FIFO 批次消耗、
// 盈亏率分母链 holdingsCost → costOfSoldShares → totalBuyAmount。
func (s *Service) GetTradingRecordStatistics(ctx context.Context) (*stock.TradingRecordStatistics, error) {
	type BuyRecord struct {
		Volume int64
		Price  float64
	}

	records, err := s.repo.ListAllTradingRecords(ctx)
	if err != nil {
		logger.SugaredLogger.Errorf("获取交易日志统计失败: %s", err.Error())
		return nil, err
	}

	stockMap := make(map[string][]BuyRecord)
	totalBuyAmount := 0.0
	totalSellAmount := 0.0
	holdingsCost := 0.0
	holdingsValue := 0.0
	costOfSoldShares := 0.0

	for _, r := range records {
		amount := r.Price * float64(r.Volume)
		if r.Direction == "买入" {
			totalBuyAmount += amount
			stockMap[r.StockCode] = append(stockMap[r.StockCode], BuyRecord{Volume: r.Volume, Price: r.Price})
		} else if r.Direction == "卖出" {
			totalSellAmount += amount
			remainingVolume := r.Volume
			for i := range stockMap[r.StockCode] {
				if remainingVolume == 0 {
					break
				}
				record := &stockMap[r.StockCode][i]
				if record.Volume <= remainingVolume {
					costOfSoldShares += float64(record.Volume) * record.Price
					remainingVolume -= record.Volume
					record.Volume = 0
				} else {
					costOfSoldShares += float64(remainingVolume) * record.Price
					record.Volume -= remainingVolume
					remainingVolume = 0
				}
			}
		}
	}

	var stockCount int64
	for code, buyRecords := range stockMap {
		currentVolume := int64(0)
		currentCost := 0.0
		for _, br := range buyRecords {
			if br.Volume > 0 {
				currentVolume += br.Volume
				currentCost += float64(br.Volume) * br.Price
			}
		}
		if currentVolume > 0 {
			stockCount++
			holdingsCost += currentCost

			if s.priceFn != nil {
				apiCode := normalizeAPICode(code)
				price, err := s.priceFn(apiCode)
				if err == nil && price > 0 {
					holdingsValue += price * float64(currentVolume)
				}
			}
		}
	}

	totalProfit := totalSellAmount - costOfSoldShares + (holdingsValue - holdingsCost)
	profitRate := 0.0
	denom := holdingsCost
	if denom <= 0 && costOfSoldShares > 0 {
		denom = costOfSoldShares
	}
	if denom <= 0 && totalBuyAmount > 0 {
		denom = totalBuyAmount
	}
	if denom > 0 {
		profitRate = (totalProfit / denom) * 100
	}

	return &stock.TradingRecordStatistics{
		TotalBuyAmount:  totalBuyAmount,
		TotalSellAmount: totalSellAmount,
		TotalProfit:     totalProfit,
		ProfitRate:      profitRate,
		HoldingsAmount:  holdingsCost,
		CurrentValue:    holdingsValue,
		StockCount:      stockCount,
	}, nil
}

// normalizeAPICode 将交易日志中的代码转为实时/K 线接口使用的代码。
// 规则与 data 层 normalizeTradingRecordAPI 逐字一致。
func normalizeAPICode(stockCode string) string {
	apiCode := stockCode
	if strings.Contains(apiCode, " - ") {
		apiCode = strings.Split(apiCode, " - ")[0]
	}
	apiCode = strings.ToLower(apiCode)
	if strings.HasSuffix(apiCode, ".sh") {
		apiCode = "sh" + strings.TrimSuffix(apiCode, ".sh")
	} else if strings.HasSuffix(apiCode, ".sz") {
		apiCode = "sz" + strings.TrimSuffix(apiCode, ".sz")
	} else if strings.HasSuffix(apiCode, ".bj") {
		apiCode = "bj" + strings.TrimSuffix(apiCode, ".bj")
	} else if strings.HasPrefix(apiCode, "6") || len(apiCode) == 6 {
		apiCode = "sh" + apiCode
	} else if strings.HasPrefix(apiCode, "0") || strings.HasPrefix(apiCode, "3") {
		apiCode = "sz" + apiCode
	} else if strings.HasPrefix(apiCode, "4") || strings.HasPrefix(apiCode, "8") {
		apiCode = "bj" + apiCode
	}
	return apiCode
}
