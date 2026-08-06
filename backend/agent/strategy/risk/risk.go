// Package risk 实现风控叠加层（方案 §8.1 D3）。
// 17 项独立风险检查逐项扣分累加，按累计分判定风险等级（high/medium/low）。
// 检查为纯函数，只依赖 RiskInput 输入结构，不直连任何数据源；
// MACD/RSI 等技术指标状态由调用方预计算后以枚举传入。
package risk

// MACDState MACD 中期趋势状态（由调用方根据指标计算结果传入）。
type MACDState string

const (
	MACDBullish MACDState = "bullish"
	MACDBearish MACDState = "bearish"
	MACDNeutral MACDState = "neutral"
)

// RSIState RSI 短期超买超卖状态（由调用方根据指标计算结果传入）。
type RSIState string

const (
	RSIOverbought RSIState = "overbought"
	RSIOversold   RSIState = "oversold"
	RSINeutral    RSIState = "neutral"
)

// RiskLevel 风险等级。
type RiskLevel string

const (
	RiskHigh   RiskLevel = "high"
	RiskMedium RiskLevel = "medium"
	RiskLow    RiskLevel = "low"
)

// RiskInput 风控检查输入（纯数据，参照 D1 scoring.FactorInput 模式）。
// 零值结构不触发任何检查（SignalScore/KLineQuality 零值视为"无数据"，不做弱信号/低质量判定）。
type RiskInput struct {
	Code          string
	ChangePercent float64 // 当日涨跌幅 %
	VolumeRatio   float64 // 量比
	TurnoverRate  float64 // 换手率 %
	PE            float64 // 市盈率（<=0 视为亏损股）
	PB            float64 // 市净率

	SignalScore float64   // 日线信号分 0-100（HasSignalScore=false 时不检查）
	MACDState   MACDState // MACD 趋势状态
	RSIState    RSIState  // RSI 超买超卖状态

	LLMConfidence float64  // LLM 置信度 0-1（HasLLMConfidence=false 时不检查）
	LLMRiskFlags  []string // LLM 识别的风险标记
	DeepRiskFlags []string // 多 Agent 深度分析识别的风险

	KLineQuality     float64  // 日线质量分 0-100（HasKLineQuality=false 时不检查）
	KLineFetchFailed bool     // 日线获取失败
	StaleCache       bool     // 日线缓存过期
	FallbackErrors   bool     // 数据源降级
	InvalidDataFlags []string // 异常数据标记（invalid_ohlc 等）

	// 数据存在性标记：区分"零值"与"无数据"，无数据时不做对应检查
	HasSignalScore   bool
	HasLLMConfidence bool
	HasKLineQuality  bool
}

// RiskCheck 单项触发的风险检查。
type RiskCheck struct {
	Name   string  // 检查项名
	Points float64 // 扣分
	Detail string  // 触发说明
}

// RiskResult 一只股票的风控评估结果。
type RiskResult struct {
	Code   string      // 股票代码
	Points float64     // 累计扣分
	Level  RiskLevel   // 风险等级
	Checks []RiskCheck // 触发的检查项（按评估顺序）
}

// RiskOverlay 风控叠加层：对单只股票执行全部风险检查并分级。
type RiskOverlay struct {
	Profile RiskProfile
}

// NewRiskOverlay 按配置构造风控叠加层。
func NewRiskOverlay(profile RiskProfile) *RiskOverlay {
	return &RiskOverlay{Profile: profile}
}

// Evaluate 执行 17 项独立风险检查，累加扣分并判定风险等级。
func (o *RiskOverlay) Evaluate(input *RiskInput) RiskResult {
	p := o.Profile
	result := RiskResult{Code: input.Code, Level: RiskLow}
	add := func(name string, points float64, detail string) {
		result.Points += points
		result.Checks = append(result.Checks, RiskCheck{Name: name, Points: points, Detail: detail})
	}

	// 1. 单日追高
	if input.ChangePercent >= p.ChaseThreshold {
		add("chase_high", p.ChasePenalty, "单日涨幅过大，追高被套风险")
	}
	// 2. 单日破位
	if input.ChangePercent <= p.BreakThreshold {
		add("break_down", p.BreakPenalty, "单日跌幅过大，趋势破坏信号")
	}
	// 3. 异常量比
	if input.VolumeRatio >= p.VolumeRatioThreshold {
		add("abnormal_volume_ratio", p.VolumeRatioPenalty, "量比异常，对倒/异常交易嫌疑")
	}
	// 4. 高换手
	if input.TurnoverRate >= p.TurnoverThreshold {
		add("high_turnover", p.TurnoverPenalty, "换手率过高，短期资金博弈")
	}
	// 5. 无效 PE（亏损）
	if input.PE <= 0 {
		add("invalid_pe", p.InvalidPEPenalty, "PE 非正，亏损企业")
	}
	// 6. 高 PB
	if input.PB >= p.PBThreshold {
		add("high_pb", p.PBPenalty, "PB 过高，估值风险")
	}
	// 7. 弱日线信号
	if input.HasSignalScore && input.SignalScore < p.WeakSignalThreshold {
		add("weak_signal", p.WeakSignalPenalty, "日线信号分偏弱，技术面弱势")
	}
	// 8. MACD 空头
	if input.MACDState == MACDBearish {
		add("macd_bearish", p.MACDBearishPenalty, "MACD 空头，中期趋势向下")
	}
	// 9. RSI 超买
	if input.RSIState == RSIOverbought {
		add("rsi_overbought", p.RSIOverboughtPenalty, "RSI 超买，短期回调风险")
	}
	// 10. 低 LLM 置信度
	if input.HasLLMConfidence && input.LLMConfidence < p.LLMConfidenceThreshold {
		add("low_llm_confidence", p.LLMConfidencePenalty, "LLM 置信度低，结论不确定")
	}
	// 11. LLM 风险标记（每条扣分，封顶）
	if n := len(input.LLMRiskFlags); n > 0 {
		points := float64(n) * p.LLMFlagPenalty
		if points > p.LLMFlagCap {
			points = p.LLMFlagCap
		}
		add("llm_risk_flags", points, "LLM 识别的风险标记")
	}
	// 12. 深度分析风险（每条扣分，封顶）
	if n := len(input.DeepRiskFlags); n > 0 {
		points := float64(n) * p.DeepFlagPenalty
		if points > p.DeepFlagCap {
			points = p.DeepFlagCap
		}
		add("deep_analysis_risks", points, "多 Agent 深度分析识别的风险")
	}
	// 13. 低日线质量
	if input.HasKLineQuality && input.KLineQuality < p.LowQualityThreshold {
		add("low_kline_quality", p.LowQualityPenalty, "日线数据质量差")
	}
	// 14. 日线获取失败
	if input.KLineFetchFailed {
		add("kline_fetch_failed", p.FetchFailedPenalty, "日线获取失败，严重数据问题")
	}
	// 15. 日线缓存过期
	if input.StaleCache {
		add("stale_cache", p.StaleCachePenalty, "日线缓存过期，数据时效性差")
	}
	// 16. 数据源降级
	if input.FallbackErrors {
		add("fallback_errors", p.FallbackPenalty, "数据源降级，可靠性下降")
	}
	// 17. 异常数据标记
	if len(input.InvalidDataFlags) > 0 {
		add("invalid_data", p.InvalidDataPenalty, "异常数据标记，数据完整性问题")
	}

	result.Level = p.LevelForPoints(result.Points)
	return result
}
