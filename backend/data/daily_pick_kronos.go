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
	"go-stock/backend/models"
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
		p.KronosPredLen = pred.Summary.PredLen
		if p.KronosPredLen <= 0 {
			p.KronosPredLen = 5
		}
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
// 依据近 60 天里已经走完预测窗口的方向命中率调权（对照预测期末，不是次日）：
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
	AvgT1    float64 `json:"avgT1"`    // 已到期样本在预测期末的平均实际涨幅 %
	CurrentW float64 `json:"currentW"` // 当前生效权重（调权后）
}

// kronosFactorStats 统计近 60 天的 Kronos 因子复盘成绩。
func (e *DailyPickEngine) kronosFactorStats(ctx context.Context) KronosFactorStats {
	var stats KronosFactorStats
	cutoff := time.Now().AddDate(0, 0, -weightHistoryDays).Format("2006-01-02")
	mature := time.Now().AddDate(0, 0, -weightMatureCalendarDays).Format("2006-01-02")
	picks := e.repo.ReviewedPicksSince(ctx, cutoff)
	var predSum, actualSum float64
	actualN := 0
	for _, p := range picks {
		if p.KronosDirection == "" || p.TradeDate > mature {
			continue
		}
		hit, actual, ok := judgeKronosPick(p)
		if !ok {
			continue
		}
		stats.Samples++
		predSum += p.KronosChangePct
		actualSum += actual
		actualN++
		if hit == "hit" {
			stats.Hits++
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

// kronosHorizon 预测对照的交易日数。历史记录没有 predLen 时按默认 5 日。
func kronosHorizon(predLen int) int {
	if predLen <= 0 {
		return 5
	}
	return predLen
}

// kronosHorizonHit 用预测期末涨跌对照方向。未到期返回空字符串。
// 涨跌绝对值不超过 0.3% 视为平盘，不记入命中率。
func kronosHorizonHit(direction string, horizonRet float64) string {
	const band = 0.3
	if math.Abs(horizonRet) <= band || direction == "flat" {
		if direction == "flat" && math.Abs(horizonRet) <= band {
			return "hit"
		}
		if direction == "flat" {
			return "miss"
		}
		return ""
	}
	up := horizonRet > 0
	switch direction {
	case "up":
		if up {
			return "hit"
		}
		return "miss"
	case "down":
		if !up {
			return "hit"
		}
		return "miss"
	default:
		return ""
	}
}

// applyKronosHorizonHit 按预测期末收盘回写命中标记。窗口未走完时清空，避免沿用次日对错。
func applyKronosHorizonHit(pick *models.DailyPick, klines []KLineData, t1 int, base float64) bool {
	if pick == nil || pick.KronosDirection == "" || base <= 0 {
		return false
	}
	want := ""
	horizon := kronosHorizon(pick.KronosPredLen)
	idx := t1 + horizon - 1
	if idx >= 0 && idx < len(klines) {
		closePx := parseFloat64(klines[idx].Close)
		if closePx > 0 {
			want = kronosHorizonHit(pick.KronosDirection, (closePx/base-1)*100)
		}
	}
	if pick.KronosHit == want {
		return false
	}
	pick.KronosHit = want
	return true
}

// judgeKronosPick 在预测窗口已经走完时给出命中和期末实际涨幅。
func judgeKronosPick(p models.DailyPick) (hit string, actual float64, ok bool) {
	if p.KronosDirection == "" {
		return "", 0, false
	}
	horizon := kronosHorizon(p.KronosPredLen)
	switch {
	case horizon <= 1:
		actual = p.NextReturnT1
		ok = p.Reviewed && (p.NextClose > 0 || p.NextReturnT1 != 0)
	case horizon <= 3:
		actual = p.Return3D
		ok = p.Return3D != 0
	default:
		actual = p.Return5D
		ok = p.Return5D != 0
	}
	if !ok {
		return "", 0, false
	}
	hit = kronosHorizonHit(p.KronosDirection, actual)
	if hit == "" {
		return "", actual, false
	}
	return hit, actual, true
}

// GetKronosFactorStats 前端查询：Kronos 因子复盘统计 + 当前权重。
func GetKronosFactorStats() KronosFactorStats {
	e := NewDailyPickEngine()
	ctx := context.Background()
	stats := e.kronosFactorStats(ctx)
	stats.CurrentW = math.Round(e.kronosFactorWeight(ctx)*1000) / 1000
	return stats
}
