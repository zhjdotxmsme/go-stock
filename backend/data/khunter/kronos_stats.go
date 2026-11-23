package khunter

import (
	"context"
	"encoding/json"
	"math"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// 因子统计与调权口径迁移自 daily_pick_kronos.go（同口径保证历史可比）：
// 统计窗 60 自然日、样本成熟 7 自然日（≈5 个交易日，保证预测窗口走完）、调权最小样本 20。
const (
	factorStatsHistoryDays = 60
	factorStatsMatureDays  = 7
	factorMinSamples       = 20
)

// KronosFactorStats 读取近窗已对账预测，输出命中率统计（khunter 数据源，替代 daily_pick 口径）。
func KronosFactorStats(ctx context.Context) data.KronosFactorStats {
	var stats data.KronosFactorStats
	if db.Dao == nil {
		return stats
	}
	cutoff := time.Now().AddDate(0, 0, -factorStatsHistoryDays).Format("2006-01-02")
	mature := time.Now().AddDate(0, 0, -factorStatsMatureDays).Format("2006-01-02")
	var rows []models.KhunterKronosPrediction
	err := db.Dao.WithContext(ctx).
		Where("hit IN ? AND signal_date >= ? AND signal_date <= ?",
			[]string{"hit", "miss"}, cutoff, mature).
		Find(&rows).Error
	if err != nil {
		logger.SugaredLogger.Warnf("khunter 因子统计查询失败: %v", err)
		return stats
	}
	var predSum, actualSum float64
	var actualN int
	for _, p := range rows {
		stats.Samples++
		if p.Hit == "hit" {
			stats.Hits++
		}
		if p.ForecastJSON != "" {
			var pred data.KronosPrediction
			if err := json.Unmarshal([]byte(p.ForecastJSON), &pred); err == nil && pred.Summary != nil {
				predSum += pred.Summary.ChangePct
			}
		}
		if p.ActualPct != 0 {
			actualSum += p.ActualPct
			actualN++
		}
	}
	if stats.Samples > 0 {
		stats.HitRate = math.Round(float64(stats.Hits)/float64(stats.Samples)*1000) / 1000
		stats.AvgPred = math.Round(predSum/float64(stats.Samples)*100) / 100
	}
	if actualN > 0 {
		stats.AvgT1 = math.Round(actualSum/float64(actualN)*100) / 100
	}
	return stats
}

// KronosFactorMultiplier 复盘驱动的因子调权系数：
// 命中率 ≥55% 上浮（最高 3 倍），[45%,55%) 不调，<45% 降权一半，<40% 归零；
// 样本 <20 或异常回落 1.0（等价于不调权）。
func KronosFactorMultiplier(ctx context.Context) float64 {
	stats := KronosFactorStats(ctx)
	if stats.Samples < factorMinSamples {
		return 1.0
	}
	switch {
	case stats.HitRate < 0.40:
		logger.SugaredLogger.Infof("khunter: Kronos 因子命中率 %.1f%% < 40%%，权重归零", stats.HitRate*100)
		return 0
	case stats.HitRate < 0.45:
		return 0.5
	case stats.HitRate >= 0.55:
		m := 1 + (stats.HitRate-0.5)*10 // 55%→1.5, 60%→2.0, 65%→2.5, ≥70%→3.0
		if m > 3 {
			m = 3
		}
		return math.Round(m*100) / 100
	default:
		return 1.0
	}
}

// AdjustedKronosWeight 应用调权系数后的因子权重（钳位 [0, MaxWeight]），
// 应用点在流水线因子挂接处；样本不足时系数为 1，行为与阶段 1 完全一致。
func AdjustedKronosWeight(ctx context.Context, cfg KronosFactorConfig) KronosFactorConfig {
	mult := KronosFactorMultiplier(ctx)
	if mult == 1 {
		return cfg
	}
	w := math.Round(cfg.Weight*mult*1000) / 1000
	if w > cfg.MaxWeight {
		w = cfg.MaxWeight
	}
	if w < 0 {
		w = 0
	}
	logger.SugaredLogger.Infof("khunter: Kronos 因子调权 命中率系数 %.2f → 权重 %.3f", mult, w)
	cfg.Weight = w
	return cfg
}
