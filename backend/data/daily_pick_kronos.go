package data

// 每日推荐 × Kronos 预测因子（P1）。
// 在策略打分完成后、LLM 二次排序前，对 TopM 候选做 Kronos 批量预测，
// 预测结果折算为因子分混入综合分；同时落库供复盘闭环（P2）验证有效性。
// 降级保护：开关关闭/服务离线/推理失败 → 结果原样返回，不影响选股主流程。

import (
	"context"
	"math"
	"sort"
	"time"

	"go-stock/backend/logger"
)

const (
	// kronosFactorTopM 参与批量预测的候选上限（CPU 推理成本约束）
	kronosFactorTopM = 30
	// kronosFactorBaseWeight 因子初始权重（混入综合分的比例）
	kronosFactorBaseWeight = 0.05
	// kronosFactorMaxWeight 自动调权允许的上限
	kronosFactorMaxWeight = 0.15
)

// kronosFactorScore 将预测摘要折算为 0~100 因子分：
// 预期涨幅按 ±10% 截断映射到 0~100（0%→50），再按一致度向中性 50 收缩。
func kronosFactorScore(pred *KronosPrediction) float64 {
	if pred == nil || pred.Summary == nil {
		return 0
	}
	chg := pred.Summary.ChangePct
	if chg > 10 {
		chg = 10
	} else if chg < -10 {
		chg = -10
	}
	base := (chg + 10) / 20 * 100 // 0~100，50 为中性
	conf := pred.Summary.Confidence / 100
	if conf < 0 {
		conf = 0
	} else if conf > 1 {
		conf = 1
	}
	return math.Round((base*conf+50*(1-conf))*10) / 10
}

// kronosFactorWeight 当前因子权重：基础权重 × 复盘驱动的调权系数（P2）。
func (e *DailyPickEngine) kronosFactorWeight(ctx context.Context) float64 {
	mult := e.kronosFactorMultiplier(ctx)
	w := kronosFactorBaseWeight * mult
	if w > kronosFactorMaxWeight {
		w = kronosFactorMaxWeight
	}
	if w < 0 {
		w = 0
	}
	return w
}

// applyKronosFactor 对 result 中得分 TopM 的候选做 Kronos 批量预测并混入因子分。
// 任何失败路径都返回原始 result（仅记日志）。
func (e *DailyPickEngine) applyKronosFactor(ctx context.Context, result []scored) []scored {
	cfg := GetSettingConfigSafe()
	if cfg == nil || cfg.Settings == nil ||
		!cfg.Settings.KronosEnable || !cfg.Settings.KronosPickEnable {
		return result
	}

	// 选取得分 TopM 的成功候选
	idx := make([]int, 0, len(result))
	for i, r := range result {
		if r.err == nil && r.pick.Score > 0 {
			idx = append(idx, i)
		}
	}
	if len(idx) == 0 {
		return result
	}
	sort.Slice(idx, func(a, b int) bool { return result[idx[a]].pick.Score > result[idx[b]].pick.Score })
	if len(idx) > kronosFactorTopM {
		idx = idx[:kronosFactorTopM]
	}

	codes := make([]string, 0, len(idx))
	codeOf := make(map[string]int, len(idx)) // apiCode -> result index
	for _, i := range idx {
		api := NormalizeKronosCode(result[i].pick.StockCode)
		if api == "" {
			continue
		}
		codes = append(codes, api)
		codeOf[api] = i
	}
	if len(codes) == 0 {
		return result
	}

	e.reportProgress("kronos", 0, len(codes))
	preds, perrs, err := KronosBatchPredict(ctx, codes, 0) // predLen=0 → 使用设置中的预测根数
	if err != nil {
		logger.SugaredLogger.Warnf("daily_pick: Kronos 因子批量预测失败（跳过因子）: %v", err)
		return result
	}
	e.reportProgress("kronos", len(codes), len(codes))

	w := e.kronosFactorWeight(ctx)
	if w <= 0 {
		return result
	}
	applied := 0
	for api, pred := range preds {
		i, ok := codeOf[api]
		if !ok || pred == nil || pred.Summary == nil {
			continue
		}
		p := &result[i].pick
		p.KronosDirection = pred.Summary.Direction
		p.KronosChangePct = pred.Summary.ChangePct
		p.KronosConfidence = pred.Summary.Confidence
		p.KronosScore = kronosFactorScore(pred)
		// 混入综合分
		p.Score = math.Round((p.Score*(1-w)+p.KronosScore*w)*100) / 100
		applied++
	}
	for api, msg := range perrs {
		logger.SugaredLogger.Debugf("daily_pick: Kronos 预测 %s 失败: %s", api, msg)
	}
	logger.SugaredLogger.Infof("daily_pick: Kronos 因子已应用 %d/%d 只（权重 %.2f）", applied, len(codes), w)
	return result
}

// kronosFactorMultiplier 复盘驱动的因子调权系数（P2）。
// 依据近 60 天复盘样本中 Kronos 预测方向的命中率（T+1 对照）调权：
// 命中率 ≥55% → 上浮（最高 3 倍，即权重上限 15%）；<45% → 降权；<40% → 归零。
// 护栏：样本 <20 恒为 1.0；任何异常回落 1.0（不调权）。
func (e *DailyPickEngine) kronosFactorMultiplier(ctx context.Context) float64 {
	stats := e.kronosFactorStats(ctx)
	if stats.Samples < kronosFactorMinSamples {
		return 1.0
	}
	switch {
	case stats.HitRate < 0.40:
		logger.SugaredLogger.Infof("daily_pick: Kronos 因子命中率 %.1f%% < 40%%，权重归零", stats.HitRate*100)
		return 0
	case stats.HitRate < 0.45:
		return 0.5
	case stats.HitRate >= 0.55:
		m := 1 + (stats.HitRate-0.5)*10 // 55%→1.5, 60%→2.0
		if m > 3 {
			m = 3
		}
		return math.Round(m*100) / 100
	default:
		return 1.0
	}
}

const kronosFactorMinSamples = 20

// KronosFactorStats Kronos 因子复盘统计（供调权与前端展示）
type KronosFactorStats struct {
	Samples  int     `json:"samples"`  // 有 Kronos 预测且已复盘的样本数
	Hits     int     `json:"hits"`     // 方向命中数
	HitRate  float64 `json:"hitRate"`  // 命中率 0~1
	AvgPred  float64 `json:"avgPred"`  // 平均预测涨幅 %
	AvgT1    float64 `json:"avgT1"`    // 命中样本的实际 T+1 平均收益 %
	CurrentW float64 `json:"currentW"` // 当前生效权重（调权后）
}

// kronosFactorStats 统计近 60 天的 Kronos 因子复盘成绩。
func (e *DailyPickEngine) kronosFactorStats(ctx context.Context) KronosFactorStats {
	var stats KronosFactorStats
	cutoff := time.Now().AddDate(0, 0, -weightHistoryDays).Format("2006-01-02")
	mature := time.Now().AddDate(0, 0, -weightMatureCalendarDays).Format("2006-01-02")
	picks := e.repo.ReviewedPicksSince(ctx, cutoff)
	var predSum, t1Sum float64
	for _, p := range picks {
		if p.KronosDirection == "" || p.TradeDate > mature {
			continue
		}
		stats.Samples++
		predSum += p.KronosChangePct
		if p.KronosHit == "hit" {
			stats.Hits++
			t1Sum += p.NextReturnT1
		}
	}
	if stats.Samples > 0 {
		stats.HitRate = math.Round(float64(stats.Hits)/float64(stats.Samples)*1000) / 1000
		stats.AvgPred = math.Round(predSum/float64(stats.Samples)*100) / 100
	}
	if stats.Hits > 0 {
		stats.AvgT1 = math.Round(t1Sum/float64(stats.Hits)*100) / 100
	}
	return stats
}

// GetKronosFactorStats 前端查询：Kronos 因子复盘统计 + 当前权重。
func GetKronosFactorStats() KronosFactorStats {
	e := NewDailyPickEngine()
	ctx := context.Background()
	stats := e.kronosFactorStats(ctx)
	stats.CurrentW = math.Round(e.kronosFactorWeight(ctx)*1000) / 1000
	return stats
}
