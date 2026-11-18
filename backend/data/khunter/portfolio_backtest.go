package khunter

import (
	"context"
	"math"
	"sort"

	"go-stock/backend/data/datasource"
	"go-stock/backend/models"
)

// PortfolioBenchmarkCode 组合回测基准固定为沪深300。
const PortfolioBenchmarkCode = "sh000300"

// PortfolioEntry 组合回测的一条确认入场：
// EntryPrice/StopPrice/SignalHigh 为真实不复权口径（Task 6 成本配置、
// Task 7 确认与股数、Task 8 退出轨迹的回放输入）。
type PortfolioEntry struct {
	Code        string
	SignalDate  string
	ConfirmDate string  // 入场日（确认日）
	EntryPrice  float64 // 确认成交价（原始价，滑点由引擎加入）
	StopPrice   float64 // 初始失效价（初始保护价）
	SignalHigh  float64 // 信号日最高价，突破失败判定用
	Shares      int     // RiskBudgetShares 输出的整手股数
	ConfirmSeq  int     // 同日确认顺序，入场排序键（先按它、再按代码）
}

// PortfolioBacktestInput 组合回测输入。
// 交易日历优先取 TradeDates，缺省从 BenchmarkBars（sh000300）推导；
// PositionLimits 为交易日 → 仓位上限（risk.Level.PositionLimit），
// 缺失日期回退 DefaultPositionLimit。
type PortfolioBacktestInput struct {
	TradeDates           []string
	BenchmarkBars        []models.KLineBar
	CodeBars             map[string][]models.KLineBar // 个股不复权日 K（升序）
	Entries              []PortfolioEntry
	PositionLimits       map[string]float64
	DefaultPositionLimit float64
	Cfg                  RiskControlConfig
}

// portfolioHolding 回放中的在途持仓。
type portfolioHolding struct {
	entry       PortfolioEntry
	shares      int
	entryFill   float64 // 含滑点买入成交价
	buyCost     float64 // 买入总现金支出（含佣金、过户费）
	protect     float64 // 当前保护价
	holdingDays int
	entryIdx    int     // 入场日在交易日历中的下标
	lastClose   float64 // 最近一次有效收盘价（停牌日用它估值）
}

// commissionOf 按成交金额计算佣金，不足最低佣金按最低收取。
func commissionOf(filledValue float64, cost CostConfig) float64 {
	if filledValue <= 0 {
		return 0
	}
	c := filledValue * cost.CommissionRate
	if c < cost.MinCommission {
		c = cost.MinCommission
	}
	return c
}

// buyTotalCost 买入总现金支出：成交金额（含滑点）+ 佣金 + 过户费。
func buyTotalCost(filledValue float64, cost CostConfig) float64 {
	return filledValue + commissionOf(filledValue, cost) + filledValue*cost.TransferFeeRate
}

// sellNetProceeds 卖出净入金：成交金额（减滑点）− 佣金 − 印花税 − 过户费。
func sellNetProceeds(filledValue float64, cost CostConfig) float64 {
	if filledValue <= 0 {
		return 0
	}
	return filledValue - commissionOf(filledValue, cost) -
		filledValue*cost.StampDutyRate - filledValue*cost.TransferFeeRate
}

// EquityMaxDrawdown 权益序列的最大回撤（峰到谷，相对峰值），空序列返回 0。
func EquityMaxDrawdown(equity []float64) float64 {
	if len(equity) == 0 {
		return 0
	}
	peak := equity[0]
	maxDD := 0.0
	for _, value := range equity {
		if value > peak {
			peak = value
		}
		if peak > 0 {
			maxDD = math.Max(maxDD, (peak-value)/peak)
		}
	}
	return maxDD
}

// LoadPortfolioBenchmarkBars 加载基准 sh000300 的不复权日 K，
// 组合回测的交易日历与基准比较都使用它。
func LoadPortfolioBenchmarkBars(ctx context.Context, startDate, endDate string) ([]models.KLineBar, error) {
	return datasource.NewKLineStore().QueryKLines(ctx, PortfolioBenchmarkCode, "day", startDate, endDate, false)
}

// RunPortfolioBacktest 逐交易日回放组合：
// 每日先处理退出（Task 8 EvaluateExitDay 状态机，含一字跌停不可成交、T+1 约束），
// 再按（确认顺序,代码）确定性排序处理入场；现金被持仓占用，
// 总敞口不得超过当日 PositionLimit；买入价加滑点、卖出价减滑点；
// 净收益扣双边佣金（最低 MinCommission 元）、卖出印花税和过户费。
// 每日权益 = 现金 + 全部持仓当日市值（停牌日用最近收盘估值），
// 组合最大回撤从该权益序列计算；窗口结束时未退出的持仓按最后收盘价强平
// （ExitReason=回测结束强平），最后一日的入场记为不可成交（无法建模 T+1 退出）；
// 确认日不在交易日历内的入场（窗口外或缺失）记为过期（Expired），
// 保证样本计数守恒。报告注明非一字板成交不含真实排队数据；
// 基准日 K 缺失时注明 BenchmarkReturn 不可用（记 0）。
func RunPortfolioBacktest(input PortfolioBacktestInput) *BacktestReport {
	cfg := input.Cfg
	cash := cfg.InitialCash
	if cash <= 0 {
		cash = DefaultRiskControlConfig().InitialCash
	}

	dates := input.TradeDates
	if len(dates) == 0 {
		for _, b := range input.BenchmarkBars {
			dates = append(dates, b.TradeDate)
		}
	}
	dateIdx := make(map[string]int, len(dates))
	for i, d := range dates {
		dateIdx[d] = i
	}

	barByDate := make(map[string]map[string]models.KLineBar, len(input.CodeBars))
	for code, bars := range input.CodeBars {
		m := make(map[string]models.KLineBar, len(bars))
		for _, b := range bars {
			m[b.TradeDate] = b
		}
		barByDate[code] = m
	}
	benchByDate := make(map[string]float64, len(input.BenchmarkBars))
	for _, b := range input.BenchmarkBars {
		if b.Close > 0 {
			benchByDate[b.TradeDate] = b.Close
		}
	}

	entriesByDate := make(map[string][]PortfolioEntry)
	for _, e := range input.Entries {
		entriesByDate[e.ConfirmDate] = append(entriesByDate[e.ConfirmDate], e)
	}
	for _, list := range entriesByDate {
		sort.Slice(list, func(i, j int) bool {
			if list[i].ConfirmSeq != list[j].ConfirmSeq {
				return list[i].ConfirmSeq < list[j].ConfirmSeq
			}
			return list[i].Code < list[j].Code
		})
	}

	positionLimit := func(date string) float64 {
		if v, ok := input.PositionLimits[date]; ok {
			return v
		}
		return input.DefaultPositionLimit
	}

	report := NewBacktestReport(len(input.Entries))
	// 全局约束：日线非一字板为模拟撮合，报告必须注明不包含真实排队数据
	report.AddLimitation("日线非一字板成交为按触发价撮合的近似，不包含真实排队/委托数据")
	holdings := make([]*portfolioHolding, 0, 8)

	marketValue := func(date string) float64 {
		total := 0.0
		for _, h := range holdings {
			price := h.lastClose
			if bar, ok := barByDate[h.entry.Code][date]; ok && bar.Close > 0 {
				price = bar.Close
			}
			total += price * float64(h.shares)
		}
		return total
	}

	// closeHolding 按退出价结算：卖出减滑点，扣佣金/印花税/过户费后入账。
	closeHolding := func(h *portfolioHolding, exitDate string, exitPrice float64, reason string) {
		sellFill := exitPrice * (1 - cfg.SlippageRate)
		net := sellNetProceeds(sellFill*float64(h.shares), cfg.CostConfig)
		cash += net
		gross := 0.0
		if h.entryFill > 0 {
			gross = (sellFill - h.entryFill) / h.entryFill
		}
		netReturn := 0.0
		if h.buyCost > 0 {
			netReturn = net/h.buyCost - 1
		}
		report.AddTrade(BacktestTrade{
			Code:        h.entry.Code,
			SignalDate:  h.entry.SignalDate,
			EntryDate:   h.entry.ConfirmDate,
			ExitDate:    exitDate,
			EntryPrice:  h.entryFill,
			ExitPrice:   sellFill,
			Shares:      h.shares,
			GrossReturn: gross,
			NetReturn:   netReturn,
			HoldingDays: h.holdingDays,
			ExitReason:  reason,
		})
	}

	lastBenchClose := 0.0

	for idx, date := range dates {
		isLast := idx == len(dates)-1

		// 1) 先退出：复用 Task 8 单日退出状态机
		kept := holdings[:0:0]
		for _, h := range holdings {
			bar, ok := barByDate[h.entry.Code][date]
			if !ok {
				kept = append(kept, h)
				continue
			}
			p := models.KhunterHunting{
				Code:         h.entry.Code,
				EntryPrice:   h.entry.EntryPrice,
				ProtectPrice: h.protect,
				InitialRisk:  h.entry.EntryPrice - h.entry.StopPrice,
			}
			decision := EvaluateExitDay(p, h.entry.SignalHigh, bar, dailyPriceLimit(h.entry.Code, bar),
				idx == h.entryIdx, idx-h.entryIdx, cfg)
			if decision.Exit {
				h.holdingDays = decision.HoldingDays
				closeHolding(h, date, decision.ExitPrice, decision.Reason)
				continue
			}
			h.protect = decision.NewProtectPrice
			h.holdingDays = decision.HoldingDays
			if bar.Close > 0 {
				h.lastClose = bar.Close
			}
			kept = append(kept, h)
		}
		holdings = kept

		// 2) 再入场：按（确认顺序,代码）确定性排序
		for _, e := range entriesByDate[date] {
			if isLast {
				// 最后一日建仓无法在窗口内建模 T+1 退出，记为不可成交而非丢弃
				report.AddUntradeable("回测结束无法建仓")
				continue
			}
			if e.Shares < 100 {
				report.AddUntradeable("不足一手")
				continue
			}
			fill := e.EntryPrice * (1 + cfg.SlippageRate)
			if fill <= 0 {
				report.AddInvalid("入场价无效")
				continue
			}
			equityNow := cash + marketValue(date)
			capValue := equityNow*positionLimit(date) - marketValue(date)
			affordable := floorLot(cash / (fill * (1 + cfg.CommissionRate + cfg.TransferFeeRate)))
			for affordable > 0 && buyTotalCost(float64(affordable)*fill, cfg.CostConfig) > cash {
				affordable -= 100
			}
			byExposure := 0
			if capValue > 0 {
				byExposure = floorLot(capValue / fill)
			}
			shares := min(e.Shares, affordable, byExposure)
			if shares < 100 {
				report.AddUntradeable("仓位或资金不足一手")
				continue
			}
			totalBuy := buyTotalCost(float64(shares)*fill, cfg.CostConfig)
			cash -= totalBuy
			holdings = append(holdings, &portfolioHolding{
				entry:     e,
				shares:    shares,
				entryFill: fill,
				buyCost:   totalBuy,
				protect:   e.StopPrice,
				entryIdx:  idx,
				lastClose: e.EntryPrice,
			})
			report.AddConfirmed()
		}

		// 3) 窗口结束：剩余持仓按最后收盘价强平（在记当日权益之前，保证最终权益=现金）
		if isLast {
			for _, h := range holdings {
				closeHolding(h, date, h.lastClose, "回测结束强平")
			}
			holdings = holdings[:0:0]
		}

		// 4) 每日权益 = 现金 + 全部持仓当日市值
		report.AppendEquity(cash + marketValue(date))
		if close, ok := benchByDate[date]; ok {
			lastBenchClose = close
		}
		delete(entriesByDate, date)
	}

	// 交易日历之外的确认信号（ConfirmDate 在窗口外或缺失）无法建仓，
	// 记为过期而非静默丢弃，保证 SignalCount = Trade + Invalid + Untradeable + Expired 守恒。
	for _, list := range entriesByDate {
		for range list {
			report.AddExpired()
		}
	}

	// 基准（sh000300）同期收益：首末有效收盘价
	if lastBenchClose > 0 {
		firstBench := 0.0
		for _, d := range dates {
			if c, ok := benchByDate[d]; ok && c > 0 {
				firstBench = c
				break
			}
		}
		if firstBench > 0 {
			report.SetBenchmarkReturn(lastBenchClose/firstBench - 1)
		}
	} else {
		report.AddLimitation("基准 sh000300 日 K 缺失，基准同期收益不可用（BenchmarkReturn 记 0）")
	}
	return report
}

// floorLot 向下取整到 100 股整数手。
func floorLot(shares float64) int {
	if shares <= 0 {
		return 0
	}
	return int(math.Floor(shares/100)) * 100
}
