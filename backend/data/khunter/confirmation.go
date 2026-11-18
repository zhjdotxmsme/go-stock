package khunter

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"go-stock/backend/data/datasource"
	"go-stock/backend/data/khunter/risk"
	"go-stock/backend/data/trading"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// ConfirmationDecision 单日确认结论：Confirmed 为 true 时才允许生成真实交易。
type ConfirmationDecision struct {
	Confirmed   bool
	SkipDay     bool
	Reason      string
	EntryPrice  float64
	StopPrice   float64
	InitialRisk float64
}

// RawSignalLevel 把信号日从前复权口径的价格/ATR 转换为真实不复权口径。
// 复权因子 = rawClose / adjustedClose；结果按 0.01 取整。
func RawSignalLevel(level, adjustedClose, rawClose float64) (float64, error) {
	if level <= 0 || adjustedClose <= 0 || rawClose <= 0 {
		return 0, fmt.Errorf("invalid price conversion inputs")
	}
	return math.Round(level*rawClose/adjustedClose*100) / 100, nil
}

// EvaluateConfirmationDay 按 T+1~T+3 确认规则判断单日是否可入场。
// 输入的 signalHigh/signalLow/atr 必须为真实不复权口径；bar 为当日不复权 K 线。
func EvaluateConfirmationDay(
	signalHigh, signalLow, atr float64,
	bar models.KLineBar,
	limit trading.PriceLimit,
	risk RiskDecision,
	cfg RiskControlConfig,
) ConfirmationDecision {
	if !risk.AllowConfirm {
		return ConfirmationDecision{SkipDay: true, Reason: strings.Join(risk.Reasons, "；")}
	}
	if trading.OnePriceLimitUp(bar, limit) {
		return ConfirmationDecision{SkipDay: true, Reason: "一字涨停不可成交"}
	}
	if bar.Open > signalHigh*(1+cfg.MaxEntryGap) {
		return ConfirmationDecision{SkipDay: true, Reason: "开盘跳空超过2%"}
	}
	entry := 0.0
	switch {
	case bar.Open >= signalHigh:
		entry = bar.Open
	case bar.High >= signalHigh:
		entry = signalHigh
	default:
		return ConfirmationDecision{Reason: "未触及确认价"}
	}
	stop := math.Max(signalLow, entry-cfg.ATRStopMultiple*atr)
	if entry <= 0 || stop <= 0 || stop >= entry {
		return ConfirmationDecision{SkipDay: true, Reason: "失效价或风险距离无效"}
	}
	return ConfirmationDecision{Confirmed: true, EntryPrice: entry, StopPrice: stop, InitialRisk: entry - stop}
}

// ConfirmationWindowState 按真实交易日序列判断 checkDate 相对于 signalDate 的窗口位置。
// 返回 signal_day / open / expired / invalid 之一。
func ConfirmationWindowState(signalDate, checkDate string, tradeDates []string) string {
	signalIdx, checkIdx := -1, -1
	for i, d := range tradeDates {
		if d == signalDate {
			signalIdx = i
		}
		if d == checkDate {
			checkIdx = i
		}
	}
	if signalIdx == -1 || checkIdx == -1 {
		return "invalid"
	}
	offset := checkIdx - signalIdx
	switch {
	case offset == 0:
		return "signal_day"
	case offset >= 1 && offset <= cfgConfirmWindow:
		return "open"
	default:
		return "expired"
	}
}

// cfgConfirmWindow 默认确认窗口为 3 个交易日；由 RiskControlConfig.ConfirmTradingDays 控制。
const cfgConfirmWindow = 3

// RiskBudgetShares 按风险预算、仓位上限和 100 股整数手计算可买入股数。
// positionLimit 为仓位上限比例（如 1.0 表示 100%），usedExposure 为已用Exposure。
func RiskBudgetShares(equity, entry, stop float64, positionLimit float64, usedExposure float64) int {
	if equity <= 0 || entry <= 0 || stop <= 0 || stop >= entry {
		return 0
	}
	riskFraction := DefaultRiskControlConfig().AccountRiskFraction
	riskBudgetShares := int(math.Floor(equity * riskFraction / (entry - stop)))
	availableExposure := equity*positionLimit - usedExposure
	maxSharesByExposure := int(math.Floor(availableExposure / entry))
	shares := riskBudgetShares
	if shares > maxSharesByExposure {
		shares = maxSharesByExposure
	}
	if shares < 0 {
		shares = 0
	}
	return (shares / 100) * 100
}

// DailyReadinessChecker 日 K 同步完成度检查接口；默认实现检查实际 K 线存在性，
// 供 Task 11 后续替换为真实同步子系统信号。
type DailyReadinessChecker interface {
	Ready(ctx context.Context, tradeDate string) bool
}

// klinePresenceReadinessChecker 当 sh000300 在 tradeDate 有不复权日 K 时认为已同步完成。
type klinePresenceReadinessChecker struct{}

func (klinePresenceReadinessChecker) Ready(ctx context.Context, tradeDate string) bool {
	bars, err := datasource.NewKLineStore().QueryKLines(ctx, "sh000300", "day", tradeDate, tradeDate, false)
	return err == nil && len(bars) > 0
}

// DefaultDailyReadinessChecker 返回基于 K 线存在性的默认 readiness 实现。
func DefaultDailyReadinessChecker() DailyReadinessChecker {
	return klinePresenceReadinessChecker{}
}

// ConfirmationEngine 负责 T+1~T+3 窗口内的触发价确认与真实交易生成。
type ConfirmationEngine struct {
	Repo      *Repo
	Readiness DailyReadinessChecker
}

// NewConfirmationEngine 创建确认引擎，使用默认 K 线存在性 readiness。
func NewConfirmationEngine(repo *Repo) *ConfirmationEngine {
	return &ConfirmationEngine{
		Repo:      repo,
		Readiness: DefaultDailyReadinessChecker(),
	}
}

// ConfirmForDate 对 tradeDate 当日的待确认观察执行确认窗口检查、数据/风险重评、
// 触发价确认与仓位计算；返回当日新确认生成的交易数。
func (e *ConfirmationEngine) ConfirmForDate(
	ctx context.Context,
	tradeDate string,
	tradeDates []string,
	marketRisk risk.Level,
	cfg RiskControlConfig,
	equity float64,
) (int, error) {
	if e.Readiness == nil {
		e.Readiness = DefaultDailyReadinessChecker()
	}
	if !e.Readiness.Ready(ctx, tradeDate) {
		return 0, nil
	}
	if marketRisk.PositionLimit <= 0 {
		return 0, nil
	}
	pending, err := e.Repo.GetPendingObservations(tradeDate)
	if err != nil {
		return 0, err
	}
	if len(pending) == 0 {
		return 0, nil
	}
	usedExposure, err := e.Repo.GetHoldingExposure()
	if err != nil {
		return 0, err
	}

	confirmedCount := 0
	for _, obs := range pending {
		state := ConfirmationWindowState(obs.SignalDate, tradeDate, tradeDates)
		if state == "signal_day" || state == "invalid" {
			continue
		}
		if state == "expired" {
			obs.Status = ObsStatusExpired
			obs.RejectReason = joinRejectReasons(obs.RejectReason, []string{"确认窗口已过期"})
			if err := e.Repo.UpsertObservation(&obs); err != nil {
				return confirmedCount, err
			}
			continue
		}

		// 加载足够长的不复权历史，保证风险画像有 20 根有效 K 线；确认日用最后一根。
		histStart := daysBefore(tradeDate, 60)
		rawBars, err := datasource.NewKLineStore().QueryKLines(ctx, obs.Code, "day", histStart, tradeDate, false)
		if err != nil || len(rawBars) == 0 {
			continue
		}

		dataDecision := EvaluateDataGate(rawBars, tradeDate, true)
		if !dataDecision.AllowConfirm {
			if err := e.recordRejection(&obs, tradeDate, dataDecision.Reasons); err != nil {
				return confirmedCount, err
			}
			continue
		}

		riskDecision := e.evaluateDayRisk(rawBars, dataDecision, marketRisk, cfg)
		if !riskDecision.AllowConfirm {
			if err := e.recordRejection(&obs, tradeDate, riskDecision.Reasons); err != nil {
				return confirmedCount, err
			}
			continue
		}

		rawHigh, rawLow, rawATR, ok := e.rawSignalLevels(ctx, obs)
		if !ok {
			continue
		}

		confirmBar, ok := findBarByDate(rawBars, tradeDate)
		if !ok {
			continue
		}

		limit := e.priceLimit(obs.Code, confirmBar)
		cd := EvaluateConfirmationDay(rawHigh, rawLow, rawATR, confirmBar, limit, riskDecision, cfg)
		if !cd.Confirmed {
			reason := cd.Reason
			if reason == "" {
				reason = "未确认"
			}
			if err := e.recordRejection(&obs, tradeDate, []string{reason}); err != nil {
				return confirmedCount, err
			}
			continue
		}

		shares := RiskBudgetShares(equity, cd.EntryPrice, cd.StopPrice, marketRisk.PositionLimit, usedExposure)
		if shares < 100 {
			if err := e.recordRejection(&obs, tradeDate, []string{"不足一手"}); err != nil {
				return confirmedCount, err
			}
			continue
		}

		support := e.rawSupportPrice(ctx, obs.Code, obs.SignalDate)
		trade := models.KhunterHunting{
			Code:             obs.Code,
			Name:             obs.Name,
			EnterDate:        tradeDate,
			EnterScore:       obs.Score,
			SupportPrice:     support,
			EntryPrice:       cd.EntryPrice,
			InitialStopPrice: cd.StopPrice,
			InitialRisk:      cd.InitialRisk,
			Shares:           shares,
		}
		if err := e.Repo.ConfirmHunting(obs.ID, &trade); err != nil {
			if errors.Is(err, ErrTradeAlreadyExists) || errors.Is(err, ErrObservationNotPending) {
				continue
			}
			logger.SugaredLogger.Warnf("khunter 确认入池事务失败 %s @ %s: %v", obs.Code, tradeDate, err)
			return confirmedCount, err
		}
		confirmedCount++
		usedExposure += cd.EntryPrice * float64(shares)
	}
	return confirmedCount, nil
}

func (e *ConfirmationEngine) recordRejection(obs *models.KhunterObservation, date string, reasons []string) error {
	decision := RiskDecision{AllowConfirm: false, Reasons: reasons}
	obs.RiskHistoryJSON = AppendRiskHistory(obs.RiskHistoryJSON, date, decision)
	obs.RejectReason = joinRejectReasons(obs.RejectReason, reasons)
	return e.Repo.UpsertObservation(obs)
}

func (e *ConfirmationEngine) evaluateDayRisk(bars []models.KLineBar, data DataDecision, marketRisk risk.Level, cfg RiskControlConfig) RiskDecision {
	metrics, err := BuildRiskMetrics(bars, cfg)
	if err != nil {
		return RiskDecision{AllowConfirm: false, Reasons: []string{err.Error()}}
	}
	return EvaluateRisk(metrics, data, marketRisk, cfg)
}

func (e *ConfirmationEngine) rawSignalLevels(ctx context.Context, obs models.KhunterObservation) (high, low, atr float64, ok bool) {
	adjBars, err := datasource.NewKLineStore().QueryKLines(ctx, obs.Code, "day", obs.SignalDate, obs.SignalDate, true)
	if err != nil || len(adjBars) == 0 {
		return 0, 0, 0, false
	}
	rawBars, err := datasource.NewKLineStore().QueryKLines(ctx, obs.Code, "day", obs.SignalDate, obs.SignalDate, false)
	if err != nil || len(rawBars) == 0 {
		return 0, 0, 0, false
	}
	adjClose := adjBars[len(adjBars)-1].Close
	rawClose := rawBars[len(rawBars)-1].Close
	high, err = RawSignalLevel(obs.TriggerPrice, adjClose, rawClose)
	if err != nil {
		return 0, 0, 0, false
	}
	low, err = RawSignalLevel(obs.SignalLow, adjClose, rawClose)
	if err != nil {
		return 0, 0, 0, false
	}
	atr, err = RawSignalLevel(obs.ATR14, adjClose, rawClose)
	if err != nil {
		return 0, 0, 0, false
	}
	return high, low, atr, true
}

func (e *ConfirmationEngine) rawSupportPrice(ctx context.Context, code, signalDate string) float64 {
	// 取信号日前约 60 个交易日的不复权 K 线计算支撑位，确保窗口足够 20 根。
	startDate := daysBefore(signalDate, 60)
	bars, err := datasource.NewKLineStore().QueryKLines(ctx, code, "day", startDate, signalDate, false)
	if err != nil || len(bars) == 0 {
		return 0
	}
	return SupportPrice(bars)
}

func daysBefore(date string, n int) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return ""
	}
	return t.AddDate(0, 0, -n).Format("2006-01-02")
}

func (e *ConfirmationEngine) priceLimit(code string, bar models.KLineBar) trading.PriceLimit {
	limit, ok := trading.DailyPriceLimit(code, bar.PrevClose, false)
	if ok {
		return limit
	}
	// PrevClose 缺失时退回到 10% 幅度（不因此跳过涨跌停判定）。
	if bar.Close > 0 {
		return trading.PriceLimit{UpPrice: math.Round(bar.Close*1.1*100) / 100, DownPrice: math.Round(bar.Close*0.9*100) / 100}
	}
	return trading.PriceLimit{}
}

func findBarByDate(bars []models.KLineBar, date string) (models.KLineBar, bool) {
	for _, b := range bars {
		if b.TradeDate == date {
			return b, true
		}
	}
	return models.KLineBar{}, false
}
