package khunter

import (
	"context"
	"errors"
	"fmt"
	"math"

	"go-stock/backend/data/datasource"
	"go-stock/backend/data/trading"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// ExitDecision 单日退出结论。
type ExitDecision struct {
	Exit            bool
	Pending         bool
	ExitPrice       float64
	Reason          string
	NewProtectPrice float64
	HoldingDays     int
}

// EvaluateExitDay 按固定顺序评估持仓单日退出状态机：
//  1. 入场日 T+1 不卖出，仅记录待退出；
//  2. 一字跌停无法成交，继续持有；
//  3. 开盘<=保护价：跳空退出；
//  4. 最低价触及保护价：保护价退出；
//  5. 收盘价<signalHigh：突破失败；
//  6. 未退出且最高价达到 Entry+InitialRisk：保护价提到成本价；
//  7. 持有交易日>=MaxHoldingTradingDays：到期退出。
func EvaluateExitDay(
	p models.KhunterHunting,
	signalHigh float64,
	bar models.KLineBar,
	limit trading.PriceLimit,
	isEntryDay bool,
	holdingDays int,
	cfg RiskControlConfig,
) ExitDecision {
	if isEntryDay {
		pending := bar.Low <= p.ProtectPrice || bar.Close < signalHigh
		return ExitDecision{Pending: pending, NewProtectPrice: p.ProtectPrice, HoldingDays: 0}
	}
	if trading.OnePriceLimitDown(bar, limit) {
		return ExitDecision{Pending: true, NewProtectPrice: p.ProtectPrice, HoldingDays: holdingDays}
	}
	if bar.Open <= p.ProtectPrice {
		return ExitDecision{Exit: true, ExitPrice: bar.Open, Reason: "失效位跳空退出", NewProtectPrice: p.ProtectPrice, HoldingDays: holdingDays}
	}
	if bar.Low <= p.ProtectPrice {
		return ExitDecision{Exit: true, ExitPrice: p.ProtectPrice, Reason: "失效位退出", NewProtectPrice: p.ProtectPrice, HoldingDays: holdingDays}
	}
	if bar.Close < signalHigh {
		return ExitDecision{Exit: true, ExitPrice: bar.Close, Reason: "突破失败", NewProtectPrice: p.ProtectPrice, HoldingDays: holdingDays}
	}
	protect := p.ProtectPrice
	if bar.High >= p.EntryPrice+p.InitialRisk && protect < p.EntryPrice {
		protect = p.EntryPrice
	}
	if holdingDays >= cfg.MaxHoldingTradingDays {
		return ExitDecision{Exit: true, ExitPrice: bar.Close, Reason: "到期退出", NewProtectPrice: protect, HoldingDays: holdingDays}
	}
	return ExitDecision{NewProtectPrice: protect, HoldingDays: holdingDays}
}

// ProcessHoldingsForDate 对 tradeDate 当日的所有持仓执行退出评估。
func ProcessHoldingsForDate(
	ctx context.Context,
	repo *Repo,
	tradeDate string,
	tradeDates []string,
	cfg RiskControlConfig,
	holdings []models.KhunterHunting,
) {
	if len(holdings) == 0 {
		return
	}
	logger.SugaredLogger.Infof("khunter %s 评估 %d 笔持仓退出", tradeDate, len(holdings))
	for _, h := range holdings {
		days, isEntry, ok := holdingDaysBetween(h.EnterDate, tradeDate, tradeDates)
		if !ok {
			logger.SugaredLogger.Warnf("khunter %s 持仓 %s 无法定位交易日序列（EnterDate=%s），本日未评估退出，请检查交易日历数据",
				tradeDate, h.Code, h.EnterDate)
			continue
		}
		signalHigh, err := resolveSignalHigh(ctx, repo, h)
		if err != nil {
			logger.SugaredLogger.Warnf("khunter %s 持仓 %s 信号高点解析失败: %v", tradeDate, h.Code, err)
			continue
		}
		bar, ok := loadRawBar(ctx, h.Code, tradeDate)
		if !ok {
			continue
		}
		limit := dailyPriceLimit(h.Code, bar)
		decision := EvaluateExitDay(h, signalHigh, bar, limit, isEntry, days, cfg)
		if decision.Exit {
			if err := repo.ExitHunting(h.ID, tradeDate, decision.ExitPrice, decision.Reason); err != nil {
				logger.SugaredLogger.Warnf("khunter %s 持仓 %s 退出失败: %v", tradeDate, h.Code, err)
			}
			continue
		}
		if err := repo.UpdateHoldingProtectAndTrack(h.ID, decision.NewProtectPrice, decision.HoldingDays); err != nil {
			logger.SugaredLogger.Warnf("khunter %s 持仓 %s 保护位/追踪天数更新失败: %v", tradeDate, h.Code, err)
		}
	}
}

func holdingDaysBetween(enterDate, tradeDate string, tradeDates []string) (days int, isEntryDay bool, ok bool) {
	enterIdx, tradeIdx := -1, -1
	for i, d := range tradeDates {
		if d == enterDate {
			enterIdx = i
		}
		if d == tradeDate {
			tradeIdx = i
		}
	}
	if enterIdx == -1 || tradeIdx == -1 || tradeIdx < enterIdx {
		return 0, false, false
	}
	days = tradeIdx - enterIdx
	return days, days == 0, true
}

func resolveSignalHigh(ctx context.Context, repo *Repo, h models.KhunterHunting) (float64, error) {
	obs, err := repo.GetObservationBySignal(h.Code, h.SignalDate)
	if err != nil {
		return 0, err
	}
	if obs == nil {
		return 0, errors.New("observation not found for signal")
	}
	adjBars, err := datasource.NewKLineStore().QueryKLines(ctx, h.Code, "day", h.SignalDate, h.SignalDate, true)
	if err != nil {
		return 0, fmt.Errorf("query adjusted kline for signal: %w", err)
	}
	if len(adjBars) == 0 {
		return 0, errors.New("adjusted kline not found for signal")
	}
	rawBars, err := datasource.NewKLineStore().QueryKLines(ctx, h.Code, "day", h.SignalDate, h.SignalDate, false)
	if err != nil {
		return 0, fmt.Errorf("query raw kline for signal: %w", err)
	}
	if len(rawBars) == 0 {
		return 0, errors.New("raw kline not found for signal")
	}
	high, err := RawSignalLevel(obs.TriggerPrice, adjBars[0].Close, rawBars[0].Close)
	if err != nil {
		return 0, err
	}
	return high, nil
}

func loadRawBar(ctx context.Context, code, tradeDate string) (models.KLineBar, bool) {
	bars, err := datasource.NewKLineStore().QueryKLines(ctx, code, "day", tradeDate, tradeDate, false)
	if err != nil || len(bars) == 0 {
		return models.KLineBar{}, false
	}
	return bars[0], true
}

func dailyPriceLimit(code string, bar models.KLineBar) trading.PriceLimit {
	limit, ok := trading.DailyPriceLimit(code, bar.PrevClose, false)
	if ok {
		return limit
	}
	base := bar.PrevClose
	if base <= 0 {
		base = bar.Close
	}
	if base > 0 {
		return trading.PriceLimit{
			UpPrice:   math.Round(base*1.1*100) / 100,
			DownPrice: math.Round(base*0.9*100) / 100,
		}
	}
	return trading.PriceLimit{}
}
