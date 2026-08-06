package data

import (
	"encoding/json"
	"math"
	"reflect"

	"go-stock/backend/agent/strategy/filter"
	"go-stock/backend/agent/strategy/risk"
	"go-stock/backend/agent/strategy/scoring"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// ===== 管线增强配置（方案 §8.1 A1：D7 硬过滤 + D1 九因子评分 + D3 风控叠加）=====

// PickEnhanceConfig 选股管线增强配置。三个增强步骤均可独立开关（默认全开），
// 任一步骤出错只记日志并回落原逻辑，绝不让选股整体失败（失败降级）。
// JSON 序列化，遵循仓库 models.StrategyConfig 的 JSON 配置惯例。
type PickEnhanceConfig struct {
	EnableFilter  bool `json:"enableFilter"`  // D7 候选池硬过滤（快照级）
	EnableScoring bool `json:"enableScoring"` // D1 九因子评分 → ScreenScore/FinalScore/FactorScores
	EnableRisk    bool `json:"enableRisk"`    // D3 风控叠加 → RiskScore/RiskLevel/RiskFlags

	// RiskExcludeEnabled 风控排除开关：false（默认）= 高风险只标记 ExcludedByRisk=true
	// 但保留在结果中（不改变现有输出数量）；true = 从结果中剔除高风险股票。
	RiskExcludeEnabled bool `json:"riskExcludeEnabled"`

	Filter      filter.HardFilterConfig `json:"filter"`
	Scorer      scoring.ScorerConfig    `json:"scorer"`
	RiskProfile risk.RiskProfile        `json:"riskProfile"`
}

// DefaultPickEnhanceConfig 返回默认增强配置：
//   - 过滤：DefaultHardFilterConfig，但 PE/PB/总市值规则禁用（现有数据源装配不出，
//     全零值会被 MinPE=0.01 误杀全部候选）；RejectMissingDaily=false（候选池阶段
//     无 K 线，日线级规则整体跳过并记录）。
//   - 评分：显式权重只启用 K 线可装配的因子（momentum/activity/liquidity/stability），
//     value/size/theme_heat/topic_alignment 因 PE/PB/市值/板块热度数据缺失不启用。
//   - 风控：DefaultRiskProfile，InvalidPEPenalty=0（PE 数据缺失，避免每只票误扣 3 分）。
func DefaultPickEnhanceConfig() PickEnhanceConfig {
	fc := filter.DefaultHardFilterConfig()
	fc.MinPE, fc.MaxPE = 0, 0
	fc.MinPB, fc.MaxPB = 0, 0
	fc.MinTotalMV, fc.MaxTotalMV = 0, 0
	fc.RejectMissingDaily = false

	rp := risk.DefaultRiskProfile()
	rp.InvalidPEPenalty = 0

	return PickEnhanceConfig{
		EnableFilter:  true,
		EnableScoring: true,
		EnableRisk:    true,
		Filter:        fc,
		Scorer: scoring.ScorerConfig{Weights: map[string]float64{
			"momentum":  0.30,
			"activity":  0.20,
			"liquidity": 0.25,
			"stability": 0.25,
		}},
		RiskProfile: rp,
	}
}

// normalize 归一化配置：三个开关全 false 且其余全零值时视为"未配置"，回落默认配置。
// 显式构造的 PickEnhanceConfig{}（想全关）请使用 WithEnhanceConfig 并自行承担语义。
func (c PickEnhanceConfig) normalize() PickEnhanceConfig {
	if reflect.DeepEqual(c, PickEnhanceConfig{}) {
		return DefaultPickEnhanceConfig()
	}
	return c
}

// ===== D7 候选池硬过滤（快照级）=====

// applyHardFilter 对候选池执行 D7 快照级硬过滤。
// 无快照数据的候选（东方财富 fallback 路径）直接放行；过滤结果为空时回落原始候选池。
// 任何异常/panic 都回落原始候选池（失败降级）。
func (e *DailyPickEngine) applyHardFilter(candidates []stockCandidate) []stockCandidate {
	cfg := e.enhanceCfg.normalize()
	if !cfg.EnableFilter || len(candidates) == 0 {
		return candidates
	}
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("daily_pick: hard filter panic: %v, fallback to original candidates", r)
		}
	}()

	var inputs []filter.FilterInput
	var withSnapshot []stockCandidate
	kept := make([]stockCandidate, 0, len(candidates))
	for _, c := range candidates {
		if !c.HasSnapshot {
			kept = append(kept, c) // 无快照数据，放行
			continue
		}
		withSnapshot = append(withSnapshot, c)
		inputs = append(inputs, filter.FilterInput{
			Code:          c.Code,
			Name:          c.Name,
			Price:         c.Price,
			ChangePercent: c.ChangePercent,
			Amount:        c.Amount,
			VolumeRatio:   c.VolumeRatio,
			TurnoverRate:  c.TurnoverRate,
			HasDailyData:  false, // 候选池阶段无 K 线，日线级规则跳过（RejectMissingDaily=false）
		})
	}
	if len(inputs) == 0 {
		return candidates
	}

	report := filter.NewPipeline(&cfg.Filter).Apply(inputs)
	logger.SugaredLogger.Infof("daily_pick: hard filter (snapshot-only, daily rules skipped):\n%s", report.Text())

	survived := make(map[string]bool, report.TotalPassed)
	for _, in := range report.Passed {
		survived[in.Code] = true
	}
	for _, c := range withSnapshot {
		if survived[c.Code] {
			kept = append(kept, c)
		}
	}
	if len(kept) == 0 {
		logger.SugaredLogger.Warn("daily_pick: hard filter rejected all candidates, fallback to original pool")
		return candidates
	}
	return kept
}

// ===== D1 九因子评分 + D3 风控叠加（stage-2 评分后的后处理，无额外网络调用）=====

// stockTechData scoreStock 计算过程中已获取的 K 线与价格数组，
// 供增强后处理复用（避免为评分重复拉取 K 线）。
type stockTechData struct {
	KLines []KLineData
	CloseP []float64
	HighP  []float64
	LowP   []float64
	Volume []float64
}

// enhanceResults 对 stage-2 评分结果执行 D1 九因子评分与 D3 风控叠加，
// 结果写入 D12 新字段（ScreenScore/FinalScore/FactorScores/RiskScore 等）。
// 旧字段（Score 等）完全不动。任何步骤失败只记日志跳过（失败降级）。
func (e *DailyPickEngine) enhanceResults(results []scored) []scored {
	cfg := e.enhanceCfg.normalize()
	if !cfg.EnableScoring && !cfg.EnableRisk {
		return results
	}
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("daily_pick: enhance panic: %v, picks keep baseline fields only", r)
		}
	}()

	// 候选池百分位上下文（仅成交额可装配；PE/PB/市值缺失，PercentileRanks 全等值返回 0.5 中性）
	amounts := make([]float64, len(results))
	for i := range results {
		amounts[i] = results[i].pick.Amount
	}
	amountRanks := scoring.PercentileRanks(amounts)

	scorer := scoring.NewScorer(cfg.Scorer)
	overlay := risk.NewRiskOverlay(cfg.RiskProfile)

	for i := range results {
		if results[i].err != nil {
			continue
		}
		pick := &results[i].pick
		tech := results[i].tech

		// D12 技术面派生字段（K 线可用的填，不可用的留零值）
		if tech != nil {
			fillTechDerivedFields(pick, tech)
		}
		if pick.Industry == "" {
			pick.Industry = e.stockIndustryMap[pick.StockCode]
		}

		if cfg.EnableScoring {
			e.applyNineFactorScore(pick, tech, amountRanks[i], scorer)
		}
		if cfg.EnableRisk {
			applyRiskOverlay(pick, overlay)
		}
	}
	return results
}

// applyNineFactorScore 装配 FactorInput 并执行 D1 九因子评分。
// FinalScore 初版与 ScreenScore 相同（D2 LLM 排序是 A2 任务）。
func (e *DailyPickEngine) applyNineFactorScore(pick *models.DailyPick, tech *stockTechData, amountPct float64, scorer *scoring.Scorer) {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Warnf("daily_pick: nine-factor score failed for %s: %v", pick.StockCode, r)
		}
	}()

	input := &scoring.FactorInput{
		Code:             pick.StockCode,
		Name:             pick.StockName,
		Price:            pick.ClosePrice,
		ChangePercent:    pick.ChangePercent,
		Amount:           pick.Amount,
		TurnoverRate:     pick.TurnoverRate,
		VolumeRatio:      pick.VolumeRatio,
		Industry:         pick.Industry,
		PEPercentile:     0.5, // PE 数据缺失，中性
		PBPercentile:     0.5,
		AmountPercentile: amountPct,
		CapPercentile:    0.5,
	}
	if tech != nil {
		input.KLine = klineToBars(tech.KLines)
	}

	result := scorer.Score(input)
	pick.ScreenScore = math.Round(result.Total*100) / 100
	pick.FinalScore = pick.ScreenScore // D2 LLM 排序前，FinalScore=ScreenScore

	factorScores := make(map[string]float64, len(result.Factors))
	for name, fr := range result.Factors {
		factorScores[name] = math.Round(fr.Score*100) / 100
	}
	if data, err := json.Marshal(factorScores); err == nil {
		pick.FactorScores = string(data)
	}
}

// applyRiskOverlay 装配 RiskInput 并执行 D3 风控叠加。
// 默认只标记不剔除：高风险票 ExcludedByRisk=true 但仍在结果中。
func applyRiskOverlay(pick *models.DailyPick, overlay *risk.RiskOverlay) {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Warnf("daily_pick: risk overlay failed for %s: %v", pick.StockCode, r)
		}
	}()

	input := &risk.RiskInput{
		Code:          pick.StockCode,
		ChangePercent: pick.ChangePercent,
		VolumeRatio:   pick.VolumeRatio,
		TurnoverRate:  pick.TurnoverRate,
		// PE/PB 数据缺失：PE=0（InvalidPEPenalty 已在引擎默认配置置 0），PB=0 不触发高 PB
		SignalScore:    pick.SignalScore,
		HasSignalScore: pick.SignalScore > 0,
		MACDState:      risk.MACDState(pick.MacdStatus),
		RSIState:       risk.RSIState(pick.RsiStatus),
	}
	result := overlay.Evaluate(input)

	pick.RiskScore = math.Round(result.Points*100) / 100
	pick.RiskPenalty = pick.RiskScore
	pick.RiskLevel = string(result.Level)
	pick.ExcludedByRisk = result.Level == risk.RiskHigh // 只标记；剔除由 RiskExcludeEnabled 控制

	if len(result.Checks) > 0 {
		names := make([]string, 0, len(result.Checks))
		for _, c := range result.Checks {
			names = append(names, c.Name)
		}
		if data, err := json.Marshal(names); err == nil {
			pick.RiskFlags = string(data)
		}
		if data, err := json.Marshal(result.Checks); err == nil {
			pick.RiskChecks = string(data)
		}
	}
}

// riskExcluded 判断该票是否应被风控从结果中剔除（仅 RiskExcludeEnabled=true 时生效）。
func (e *DailyPickEngine) riskExcluded(p models.DailyPick) bool {
	return e.enhanceCfg.normalize().RiskExcludeEnabled && p.ExcludedByRisk
}

// ===== D12 技术面派生字段装配（全部来自已获取的 K 线，无额外网络调用）=====

// fillTechDerivedFields 填充 D12 技术面字段：60 日涨跌幅、信号分、MACD/RSI 状态、
// 20 日突破、振幅、20 日量比、K 线实体、回踩 MA20、盘整天数、波动率、最大回撤、ATR。
func fillTechDerivedFields(pick *models.DailyPick, tech *stockTechData) {
	n := len(tech.KLines)
	if n == 0 {
		return
	}
	closes, highs, lows, vols := tech.CloseP, tech.HighP, tech.LowP, tech.Volume
	today := tech.KLines[n-1]

	// 60 日涨跌幅（数据不足 60 根时用全部历史）
	if closes[0] > 0 {
		pick.Change60dPct = math.Round((closes[n-1]-closes[0])/closes[0]*10000) / 100
	}

	// 信号分：以现有综合分为代理（0-100 截断），供 D7 日线规则与 D3 弱信号检查使用
	pick.SignalScore = math.Min(100, math.Max(0, math.Round(pick.Score*100)/100))

	// MACD/RSI 状态枚举（与 D1 momentum 因子口径一致）
	pick.MacdStatus = mapMACDStatus(pick.Macd, pick.MacdSignal)
	pick.RsiStatus = mapRSIStatus(pick.Rsi14)

	// 20 日突破幅度：(收盘 - 前 20 日最高价) / 前 20 日最高价 × 100
	if n >= 21 {
		base := 0.0
		for _, h := range highs[n-21 : n-1] {
			if h > base {
				base = h
			}
		}
		if base > 0 {
			pick.Breakout20dPct = math.Round((closes[n-1]-base)/base*10000) / 100
		}
	}

	// 振幅 %：K 线自带字段优先，缺则用 (高-低)/昨收 计算
	pick.AmplitudePct = parseFloat64(today.Amplitude)
	if pick.AmplitudePct == 0 && n >= 2 && closes[n-2] > 0 {
		pick.AmplitudePct = math.Round((highs[n-1]-lows[n-1])/closes[n-2]*10000) / 100
	}

	// 20 日量比：当日成交量 / 前 20 日均量
	if n >= 21 {
		sum := 0.0
		for _, v := range vols[n-21 : n-1] {
			sum += v
		}
		if avg := sum / 20; avg > 0 {
			pick.VolumeRatio20d = math.Round(vols[n-1]/avg*100) / 100
		}
	}

	// K 线实体比例 0-1：|收-开| / (高-低)
	if rng := highs[n-1] - lows[n-1]; rng > 0 {
		pick.BodyPct = math.Round(math.Abs(closes[n-1]-parseFloat64(today.Open))/rng*100) / 100
	}

	// 回踩 MA20：价格相对 MA20 偏离在 ±3% 以内
	if pick.Ma20 > 0 {
		dev := (pick.ClosePrice - pick.Ma20) / pick.Ma20 * 100
		pick.PullbackMa20 = dev >= -3 && dev <= 3
	}

	// 盘整天数：从最新一根向前，连续满足 振幅<=4% 且 |涨跌|<=2% 的天数
	for i := n - 1; i >= 1; i-- {
		prev := closes[i-1]
		if prev <= 0 {
			break
		}
		amp := (highs[i] - lows[i]) / prev * 100
		chg := math.Abs(closes[i]-prev) / prev * 100
		if amp <= 4 && chg <= 2 {
			pick.ConsolidationDays++
		} else {
			break
		}
	}

	pick.Volatility20dPct = math.Round(volatilityPct(closes)*100) / 100
	pick.MaxDrawdownPct = math.Round(maxDrawdownPct(closes)*100) / 100
	if n >= 15 {
		pick.Atr14 = math.Round(calcATR(highs, lows, closes, 14)*100) / 100
	}
}

// mapMACDStatus DIF/DEA → bullish/bearish/neutral（与 D1 momentum 因子口径一致）。
func mapMACDStatus(dif, dea float64) string {
	switch {
	case dif > 0 && dif > dea:
		return "bullish"
	case dif < 0 && dif < dea:
		return "bearish"
	default:
		return "neutral"
	}
}

// mapRSIStatus RSI14 → overbought/oversold/neutral。
func mapRSIStatus(rsi float64) string {
	switch {
	case rsi >= 70:
		return "overbought"
	case rsi <= 30 && rsi > 0:
		return "oversold"
	default:
		return "neutral"
	}
}

// klineToBars 将 data.KLineData 转为 scoring.KLineBar（时间升序）。
func klineToBars(klines []KLineData) []scoring.KLineBar {
	bars := make([]scoring.KLineBar, len(klines))
	for i, k := range klines {
		bars[i] = scoring.KLineBar{
			Open:   parseFloat64(k.Open),
			High:   parseFloat64(k.High),
			Low:    parseFloat64(k.Low),
			Close:  parseFloat64(k.Close),
			Volume: parseFloat64(k.Volume),
			Amount: parseFloat64(k.Amount),
		}
	}
	return bars
}

// volatilityPct 日收益率年化波动率（%，std × sqrt(252) × 100），与 scoring 内部实现同口径。
func volatilityPct(closes []float64) float64 {
	n := len(closes)
	if n < 3 {
		return 0
	}
	returns := make([]float64, 0, n-1)
	for i := 1; i < n; i++ {
		if closes[i-1] <= 0 {
			continue
		}
		returns = append(returns, (closes[i]-closes[i-1])/closes[i-1])
	}
	m := len(returns)
	if m < 2 {
		return 0
	}
	mean := 0.0
	for _, r := range returns {
		mean += r
	}
	mean /= float64(m)
	varSq := 0.0
	for _, r := range returns {
		varSq += (r - mean) * (r - mean)
	}
	return math.Sqrt(varSq/float64(m-1)) * math.Sqrt(252) * 100
}

// maxDrawdownPct 最大回撤（%，负数），与 scoring 内部实现同口径。
func maxDrawdownPct(closes []float64) float64 {
	if len(closes) < 2 {
		return 0
	}
	peak := closes[0]
	maxDD := 0.0
	for _, c := range closes {
		if c > peak {
			peak = c
		}
		if peak > 0 {
			if dd := (c - peak) / peak * 100; dd < maxDD {
				maxDD = dd
			}
		}
	}
	return maxDD
}
