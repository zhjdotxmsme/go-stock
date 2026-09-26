package data

import (
	"math"
	"strconv"
)

// strategy_parking_apron.go —— 停机坪策略（移植自 InStock parking_apron）。
//
// 形态定义：
//  1. 最近 15 日内出现涨幅 > 9.5% 的涨停日，且当日收盘创 15 日新高（强势确认）；
//  2. 次日高开（开盘价 > 涨停收盘价）、收涨，且 |收盘/开盘 - 1| < 3%（小实体）；
//  3. 第 2、3 日同样高开收涨小实体，且单日涨跌幅 |pct| < 5%。
//
// 评分：涨停日 40 分；其后每满足一日强势横盘 +20，完成 3 日横盘得 100。
// 横盘超过 3 日视为形态已过期（InStock 仅检查涨停后头 3 日），不再给分。
type ParkingApronStrategy struct{}

func (s *ParkingApronStrategy) Name() string { return "停机坪" }
func (s *ParkingApronStrategy) Code() string { return "parking_apron" }
func (s *ParkingApronStrategy) Description() string {
	return "涨停后高位缩量横盘 1-3 日，强势整理待突破"
}

func (s *ParkingApronStrategy) Score(ctx *StrategyContext) *StrategyResult {
	fail := &StrategyResult{Score: 0, Factors: map[string]float64{}, Signal: ""}
	n := len(ctx.CloseP)
	if n < 16 || len(ctx.KLines) < n {
		fail.Signal = "K线数据不足"
		return fail
	}

	// 涨停日窗口：最近 15 根
	winStart := n - 15
	limitIdx := -1
	for i := winStart; i < n; i++ {
		if ctx.CloseP[i-1] <= 0 {
			continue
		}
		pct := (ctx.CloseP[i]/ctx.CloseP[i-1] - 1) * 100
		if pct <= 9.5 {
			continue
		}
		// 当日收盘须创 15 日新高（含当日）
		hi := 0.0
		for j := max(0, i-14); j <= i; j++ {
			hi = math.Max(hi, ctx.CloseP[j])
		}
		if ctx.CloseP[i] >= hi {
			limitIdx = i // 取最近一个
		}
	}
	if limitIdx < 0 {
		fail.Signal = "近15日无涨停创新高"
		return fail
	}

	lp := ctx.CloseP[limitIdx]
	d := n - 1 - limitIdx // 涨停日后的横盘天数（0 = 今日涨停）
	factors := map[string]float64{"limit_up": 40}
	if d == 0 {
		return &StrategyResult{Score: 40, Factors: factors, Signal: "今日涨停创15日新高，观察次日是否强势横盘"}
	}
	if d > 3 {
		fail.Factors = factors
		fail.Signal = "涨停横盘已超3日，形态过期"
		return fail
	}

	// 校验涨停后每一横盘日
	for j := limitIdx + 1; j < n; j++ {
		o := parseFloat(ctx.KLines[j].Open)
		c := ctx.CloseP[j]
		if o <= 0 || ctx.CloseP[j-1] <= 0 {
			fail.Factors = factors
			fail.Signal = "横盘日开盘价缺失"
			return fail
		}
		pct := (c/ctx.CloseP[j-1] - 1) * 100
		body := c / o
		ok := o > lp && c > lp && body > 0.97 && body < 1.03
		if j > limitIdx+1 { // 第 2、3 日附加振幅约束
			ok = ok && pct > -5 && pct < 5
		}
		if !ok {
			fail.Factors = factors
			fail.Signal = "横盘日被跌破或实体过大，形态破坏"
			return fail
		}
		factors["consolidation"] = float64(d) * 20
	}

	score := 40 + float64(d)*20
	signal := "停机坪强势横盘第" + strconv.Itoa(d) + "日"
	if d == 3 {
		signal = "停机坪形态完成（涨停后3日高位横盘），临近方向选择"
	}
	return &StrategyResult{Score: score, Factors: factors, Signal: signal}
}
