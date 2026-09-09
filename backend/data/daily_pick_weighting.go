package data

import (
	"context"
	"math"
	"time"

	"go-stock/backend/logger"
)

// ===== 策略自动调权（§AI选股技术指标增强 §10）=====
//
// 依据复盘 T+1 可执行口径（尾盘买入→T+1 收盘卖出）的历史成绩，为每个策略
// 生成竞争系数：表现优于全体均值的策略加权，落后的减权。
//
// 防过拟合护栏（设计规格 §10.2）：
//   - 最小样本：策略成熟样本 < weightMinSamples → 系数恒为 1.0
//   - 硬限幅：系数永远在 [weightFloor, weightCeil]
//   - 单一口径：只用 T+1（3/5 日窗口仅展示，不参与调权）
//   - 零和基线：减去全体均值后再缩放，普涨行情不会全体加权
//
// 失败降级：任何查询/计算异常都回落全 1.0（等价于不调权），绝不让选股失败。

const (
	// weightMinSamples 参与调权的最小成熟样本数。
	weightMinSamples = 10
	// weightFloor / weightCeil 系数硬限幅。
	weightFloor = 0.6
	weightCeil  = 1.5
	// weightSensitivity 每 1 个百分点超额收益对应的系数变化。
	weightSensitivity = 0.1
	// weightHistoryDays 复盘历史回看天数（自然日）。
	weightHistoryDays = 60
	// weightMatureCalendarDays 成熟样本的 TradeDate 距今最小自然日数
	// （≈5 个交易日，保证 3/5 日窗口均已到期回填）。
	weightMatureCalendarDays = 7
)

// loadStrategyMultipliers 从复盘历史计算每个策略的调权系数。
// 每次选股运行（RunDailyPick / RunWithConfig）开始时调用一次。
func (e *DailyPickEngine) loadStrategyMultipliers(ctx context.Context) map[string]float64 {
	mults := make(map[string]float64)
	if !e.enhanceCfg.EnableAutoWeights {
		return mults
	}

	cutoff := time.Now().AddDate(0, 0, -weightHistoryDays).Format("2006-01-02")
	mature := time.Now().AddDate(0, 0, -weightMatureCalendarDays).Format("2006-01-02")

	picks := e.repo.ReviewedPicksSince(ctx, cutoff)
	if len(picks) == 0 {
		return mults
	}

	// 按策略聚合成熟样本；同时累计全体 T+1 均值（零和基线）。
	var (
		overallSum float64
		overallCnt int
		sums       = make(map[string]float64)
		cnts       = make(map[string]int)
	)
	for _, p := range picks {
		if p.NextReturnT1 == 0 {
			continue // 未回填
		}
		overallSum += p.NextReturnT1
		overallCnt++
		if p.StrategyCode == "" || p.TradeDate > mature {
			continue // 未成熟或无策略归属（ bonus 层不参与）
		}
		sums[p.StrategyCode] += p.NextReturnT1
		cnts[p.StrategyCode]++
	}
	if overallCnt == 0 {
		return mults
	}
	overallAvg := overallSum / float64(overallCnt)

	for code, sum := range sums {
		n := cnts[code]
		if n < weightMinSamples {
			continue // 护栏：样本不足不调权
		}
		avg := sum / float64(n)
		m := 1 + weightSensitivity*(avg-overallAvg)
		if m < weightFloor {
			m = weightFloor
		}
		if m > weightCeil {
			m = weightCeil
		}
		mults[code] = math.Round(m*100) / 100
	}

	if len(mults) > 0 {
		logger.SugaredLogger.Infof("daily_pick: strategy multipliers loaded: %v", mults)
	}
	return mults
}

// strategyMultFor 返回策略的竞争系数；未加载/无记录返回 1.0（不调权）。
func (e *DailyPickEngine) strategyMultFor(code string) float64 {
	if m, ok := e.strategyMults[code]; ok && m > 0 {
		return m
	}
	return 1.0
}
