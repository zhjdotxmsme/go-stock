package khunter

import (
	"sort"
)

// BacktestTrade 单笔回测成交记录。EntryPrice/ExitPrice 为含滑点的实际成交价
// （买入加滑点、卖出减滑点），配合 Shares 与 CostConfig 可完整复算净收益。
type BacktestTrade struct {
	Code        string  `json:"code"`
	SignalDate  string  `json:"signalDate"`
	EntryDate   string  `json:"entryDate"`
	ExitDate    string  `json:"exitDate"`
	EntryPrice  float64 `json:"entryPrice"`
	ExitPrice   float64 `json:"exitPrice"`
	Shares      int     `json:"shares"`
	GrossReturn float64 `json:"grossReturn"` // 价格收益（成交价口径，不含佣金/印花税/过户费）
	NetReturn   float64 `json:"netReturn"`   // 扣除全部交易成本后的收益
	HoldingDays int     `json:"holdingDays"`
	ExitReason  string  `json:"exitReason"`
}

// BacktestMetrics 回测尾部风险与汇总指标。
// 零收益交易单独计数（ZeroReturnCount），不混入平均亏损分母。
type BacktestMetrics struct {
	TradeCount         int     `json:"tradeCount"`
	InvalidCount       int     `json:"invalidCount"`
	UntradeableCount   int     `json:"untradeableCount"`
	ZeroReturnCount    int     `json:"zeroReturnCount"`
	WinRate            float64 `json:"winRate"`
	AvgNetReturn       float64 `json:"avgNetReturn"`
	AvgLoss            float64 `json:"avgLoss"`          // 平均亏损，分母只含负收益交易
	WorstDecileAvg     float64 `json:"worstDecileAvg"`   // 最差十分位平均（样本 <10 时标记 SampleInsufficient）
	MaxSingleLoss      float64 `json:"maxSingleLoss"`    // 最大单笔亏损（无亏损交易时为 0）
	PortfolioMaxDD     float64 `json:"portfolioMaxDD"`   // 组合每日权益序列的最大回撤
	FiveDayAvgReturn   float64 `json:"fiveDayAvgReturn"` // 持有 ≤5 个交易日的交易平均净收益
	ProfitLossRatio    float64 `json:"profitLossRatio"`
	BenchmarkReturn    float64 `json:"benchmarkReturn"` // 基准（sh000300）同期收益
	ExcessReturn       float64 `json:"excessReturn"`    // AvgNetReturn - BenchmarkReturn
	SampleInsufficient bool    `json:"sampleInsufficient"`
}

// BacktestReport 回测报告：失败样本计数守恒——
// SignalCount = ConfirmedCount + ExpiredCount + InvalidCount + UntradeableCount + StillInWindowCount，
// 任何信号要么成为确认成交、要么被计入 过期/无效/不可成交/仍在窗口，不允许静默丢弃。
// StillInWindowCount 为运行数据结束时仍在确认窗口内、未产生结局的信号数；
// 只要仍有窗口内记录，Unfinished=true（该段运行未完成，结论仅供参考）。
type BacktestReport struct {
	SignalCount          int             `json:"signalCount"`
	ConfirmedCount       int             `json:"confirmedCount"`
	InvalidCount         int             `json:"invalidCount"`
	UntradeableCount     int             `json:"untradeableCount"`
	ExpiredCount         int             `json:"expiredCount"`
	StillInWindowCount   int             `json:"stillInWindowCount"`
	StillInWindowReasons []string        `json:"stillInWindowReasons"`
	Unfinished           bool            `json:"unfinished"`
	TradeCount           int             `json:"tradeCount"`
	Trades               []BacktestTrade `json:"trades"`
	InvalidReasons       []string        `json:"invalidReasons"`
	UntradeableReasons   []string        `json:"untradeableReasons"`
	PortfolioEquity      []float64       `json:"portfolioEquity"` // 每日权益（现金 + 持仓市值）
	Limitations          []string        `json:"limitations"`
	Metrics              BacktestMetrics `json:"metrics"`
}

// NewBacktestReport 创建报告，SignalCount 为本阶段输入信号总数。
func NewBacktestReport(signalCount int) *BacktestReport {
	return &BacktestReport{SignalCount: signalCount}
}

// AddInvalid 记录一个失败样本及其原因。
func (r *BacktestReport) AddInvalid(reason string) {
	r.InvalidCount++
	r.InvalidReasons = append(r.InvalidReasons, reason)
}

// AddUntradeable 记录一个不可成交样本（如不足一手）及其原因。
func (r *BacktestReport) AddUntradeable(reason string) {
	r.UntradeableCount++
	r.UntradeableReasons = append(r.UntradeableReasons, reason)
}

// AddExpired 记录一个过期样本。
func (r *BacktestReport) AddExpired() {
	r.ExpiredCount++
}

// AddStillInWindow 记录一个运行结束时仍在确认窗口内、未产生结局的样本，
// 并把本次运行标记为未完成（结论仅供展示，不得作为替换默认策略的依据）。
func (r *BacktestReport) AddStillInWindow(reason string) {
	r.StillInWindowCount++
	r.Unfinished = true
	r.StillInWindowReasons = append(r.StillInWindowReasons, reason)
}

// Conserves 样本守恒校验：
// SignalCount = Confirmed + Expired + Invalid + Untradeable + StillInWindow。
func (r *BacktestReport) Conserves() bool {
	return r.SignalCount == r.ConfirmedCount+r.ExpiredCount+r.InvalidCount+r.UntradeableCount+r.StillInWindowCount
}

// AddConfirmed 记录一个成功建仓。
func (r *BacktestReport) AddConfirmed() {
	r.ConfirmedCount++
}

// AddTrade 记录一笔成交并刷新指标。
func (r *BacktestReport) AddTrade(trade BacktestTrade) {
	r.Trades = append(r.Trades, trade)
	r.TradeCount++
	r.refreshMetrics()
}

// AppendEquity 追加一个每日权益点并刷新组合回撤。
func (r *BacktestReport) AppendEquity(value float64) {
	r.PortfolioEquity = append(r.PortfolioEquity, value)
	r.refreshMetrics()
}

// SetBenchmarkReturn 设置基准同期收益并刷新超额收益。
func (r *BacktestReport) SetBenchmarkReturn(benchmarkReturn float64) {
	r.Metrics.BenchmarkReturn = benchmarkReturn
	r.Metrics.ExcessReturn = r.Metrics.AvgNetReturn - benchmarkReturn
}

// AddLimitation 追加一条已知局限说明。
func (r *BacktestReport) AddLimitation(text string) {
	r.Limitations = append(r.Limitations, text)
}

// refreshMetrics 从成交与权益序列重建汇总指标。
func (r *BacktestReport) refreshMetrics() {
	netReturns := make([]float64, 0, len(r.Trades))
	for _, tr := range r.Trades {
		netReturns = append(netReturns, tr.NetReturn)
	}
	m := SummarizeReturns(netReturns)
	m.InvalidCount = r.InvalidCount
	m.UntradeableCount = r.UntradeableCount
	m.PortfolioMaxDD = EquityMaxDrawdown(r.PortfolioEquity)

	// FiveDayAvgReturn：持有 ≤5 个交易日（短线默认周期）的交易平均净收益
	var fiveSum float64
	fiveCount := 0
	for _, tr := range r.Trades {
		if tr.HoldingDays <= 5 {
			fiveSum += tr.NetReturn
			fiveCount++
		}
	}
	if fiveCount > 0 {
		m.FiveDayAvgReturn = fiveSum / float64(fiveCount)
	}
	m.BenchmarkReturn = r.Metrics.BenchmarkReturn
	m.ExcessReturn = m.AvgNetReturn - m.BenchmarkReturn
	r.Metrics = m
}

// SummarizeReturns 汇总净收益序列：
//   - WorstDecileAvg：最差十分位均值，k = max(1, len/10)；
//   - MaxSingleLoss：最小收益（全为正收益时记 0）；
//   - 零收益交易单独计数，不进入盈亏均值与平均亏损分母。
func SummarizeReturns(returns []float64) BacktestMetrics {
	m := BacktestMetrics{TradeCount: len(returns)}
	if len(returns) == 0 {
		m.SampleInsufficient = true
		return m
	}
	var sum, winSum, lossSum float64
	wins := 0
	for _, ret := range returns {
		sum += ret
		switch {
		case ret > 0:
			wins++
			winSum += ret
		case ret < 0:
			lossSum += -ret
		default:
			m.ZeroReturnCount++
		}
	}
	m.WinRate = float64(wins) / float64(len(returns))
	m.AvgNetReturn = sum / float64(len(returns))
	losses := len(returns) - wins - m.ZeroReturnCount
	if losses > 0 {
		m.AvgLoss = lossSum / float64(losses)
	}
	if wins > 0 && losses > 0 {
		m.ProfitLossRatio = (winSum / float64(wins)) / m.AvgLoss
	}

	sorted := append([]float64(nil), returns...)
	sort.Float64s(sorted)
	k := len(sorted) / 10
	if k < 1 {
		k = 1
	}
	var tailSum float64
	for i := 0; i < k; i++ {
		tailSum += sorted[i]
	}
	m.WorstDecileAvg = tailSum / float64(k)
	m.MaxSingleLoss = sorted[0]
	if m.MaxSingleLoss > 0 {
		m.MaxSingleLoss = 0
	}
	// 十分位统计至少需要 10 个样本才有意义
	if len(returns) < 10 {
		m.SampleInsufficient = true
	}
	return m
}
