package data

import (
	"fmt"
	"strings"

	"go-stock/backend/models"
)

// DailyPickTechBrief 用推荐日已经落库的指标快照写技术分析。
// 不重新取数：历史记录打开详情时也能覆盖均线、MACD、RSI、KDJ、布林、量能和波动。
func DailyPickTechBrief(p models.DailyPick) []models.TechSection {
	var bulls, bears []string
	add := func(text string, b, r []string) string {
		bulls = append(bulls, b...)
		bears = append(bears, r...)
		return text
	}
	return []models.TechSection{
		{Title: "趋势与均线", Text: add(briefTrend(p))},
		{Title: "动量", Text: add(briefMomentum(p))},
		{Title: "价格位置", Text: add(briefLocation(p))},
		{Title: "量能", Text: add(briefVolume(p))},
		{Title: "波动与回撤", Text: add(briefRisk(p))},
		{Title: "综合", Text: briefVerdict(bulls, bears)},
	}
}

func briefTrend(p models.DailyPick) (string, []string, []string) {
	if p.ClosePrice <= 0 {
		return "缺少收盘价，无法判断趋势。", nil, nil
	}
	var b strings.Builder
	var bulls, bears []string
	fmt.Fprintf(&b, "收盘 %s", f2(p.ClosePrice))
	if p.ChangePercent != 0 {
		fmt.Fprintf(&b, "，当日 %s", signed1(p.ChangePercent))
	}
	b.WriteString("。")

	type ma struct {
		name string
		v    float64
	}
	var parts []string
	for _, m := range []ma{{"MA5", p.Ma5}, {"MA10", p.Ma10}, {"MA20", p.Ma20}, {"MA60", p.Ma60}} {
		if m.v <= 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %s（偏离 %s）", m.name, f2(m.v), signed1((p.ClosePrice-m.v)/m.v*100)))
	}
	if len(parts) == 0 {
		b.WriteString("均线样本不足。")
	} else {
		b.WriteString(strings.Join(parts, "，"))
		b.WriteString("。")
	}
	if p.Ma5 > 0 && p.Ma10 > 0 && p.Ma20 > 0 {
		switch {
		case p.ClosePrice >= p.Ma5 && p.Ma5 >= p.Ma10 && p.Ma10 >= p.Ma20:
			b.WriteString("短中期多头排列（收盘≥MA5≥MA10≥MA20）。")
			bulls = append(bulls, "均线多头排列")
		case p.ClosePrice <= p.Ma5 && p.Ma5 <= p.Ma10 && p.Ma10 <= p.Ma20:
			b.WriteString("短中期空头排列（收盘≤MA5≤MA10≤MA20）。")
			bears = append(bears, "均线空头排列")
		default:
			b.WriteString("均线没有单边排列，短中期处在缠绕或修复。")
		}
	}
	if p.Ma60 > 0 {
		if p.ClosePrice >= p.Ma60 && p.Ma20 >= p.Ma60 {
			b.WriteString("收盘和 MA20 都在 MA60 之上，中期方向与短线一致。")
			bulls = append(bulls, "中期均线向上")
		} else if p.ClosePrice < p.Ma60 {
			fmt.Fprintf(&b, "收盘仍在 MA60（%s）之下，中期压力还在。", f2(p.Ma60))
			bears = append(bears, "收盘在 MA60 下")
		}
	}
	if p.Ma20 > 0 {
		dev := (p.ClosePrice - p.Ma20) / p.Ma20 * 100
		switch {
		case p.PullbackMa20:
			fmt.Fprintf(&b, "价格贴着 MA20（偏离 %s），这是回踩，不是远离均线。", signed1(dev))
		case dev >= 8:
			fmt.Fprintf(&b, "价格偏离 MA20 达到 %s，短线离开均线较远。", signed1(dev))
			bears = append(bears, "远离 MA20")
		case dev <= -8:
			fmt.Fprintf(&b, "价格低于 MA20 达到 %s，反弹还没有回到均线。", signed1(dev))
			bears = append(bears, "远离 MA20")
		}
	}
	if p.Change60dPct != 0 {
		fmt.Fprintf(&b, "统计区间涨跌 %s。", signed1(p.Change60dPct))
		switch {
		case p.Change60dPct >= 30:
			bears = append(bears, "区间涨幅已经偏大")
		case p.Change60dPct <= -20:
			bears = append(bears, "区间跌幅已经偏深")
		}
	}
	return b.String(), bulls, bears
}

func briefMomentum(p models.DailyPick) (string, []string, []string) {
	var b strings.Builder
	var bulls, bears []string
	if p.Rsi14 <= 0 {
		b.WriteString("RSI(14) 样本不足。")
	} else {
		fmt.Fprintf(&b, "RSI(14) %s，%s。", f1(p.Rsi14), rsiZone(p.Rsi14))
		switch {
		case p.Rsi14 >= 80:
			bears = append(bears, "RSI 严重超买")
		case p.Rsi14 >= 70:
			bears = append(bears, "RSI 超买")
		case p.Rsi14 >= 50:
			bulls = append(bulls, "RSI 处于强势区")
		case p.Rsi14 > 30:
			bears = append(bears, "RSI 偏弱")
		default:
			bulls = append(bulls, "RSI 超卖")
		}
	}
	if p.Macd == 0 && p.MacdSignal == 0 {
		b.WriteString("MACD 样本不足。")
	} else {
		hist := p.Macd - p.MacdSignal
		fmt.Fprintf(&b, "MACD DIF %s、DEA %s、柱 %s。", f2(p.Macd), f2(p.MacdSignal), signed2(hist))
		switch {
		case p.Macd > 0 && p.Macd > p.MacdSignal:
			b.WriteString("DIF 在零轴上方且高于 DEA，动量偏多。")
			bulls = append(bulls, "MACD 零轴上多头")
		case p.Macd > p.MacdSignal:
			b.WriteString("柱已转正，但 DIF 仍在零轴下，属于空头里的修复。")
		case p.Macd < 0 && p.Macd < p.MacdSignal:
			b.WriteString("DIF 在零轴下方且低于 DEA，动量偏空。")
			bears = append(bears, "MACD 零轴下空头")
		default:
			b.WriteString("DIF 低于 DEA，短线动量转弱。")
			bears = append(bears, "MACD 死叉")
		}
	}
	if p.KdjK == 0 && p.KdjD == 0 && p.KdjJ == 0 {
		b.WriteString("KDJ 样本不足。")
	} else {
		fmt.Fprintf(&b, "KDJ K %s / D %s / J %s。", f1(p.KdjK), f1(p.KdjD), f1(p.KdjJ))
		if p.KdjK >= p.KdjD {
			b.WriteString("K 在 D 之上，短线金叉方向。")
			if p.KdjJ < 100 && p.KdjK < 80 {
				bulls = append(bulls, "KDJ 金叉且未极端超买")
			}
		} else {
			b.WriteString("K 在 D 之下，短线死叉方向。")
			bears = append(bears, "KDJ 死叉")
		}
		switch {
		case p.KdjJ >= 100 || p.KdjK >= 80:
			b.WriteString("J 或 K 进入超买，追高性价比下降。")
			bears = append(bears, "KDJ 超买")
		case p.KdjJ <= 0 || p.KdjK <= 20:
			b.WriteString("J 或 K 进入超卖。")
			bulls = append(bulls, "KDJ 超卖")
		}
	}
	return b.String(), bulls, bears
}

func rsiZone(v float64) string {
	switch {
	case v >= 80:
		return "严重超买"
	case v >= 70:
		return "超买"
	case v >= 60:
		return "强势，尚未超买"
	case v >= 50:
		return "偏强"
	case v >= 40:
		return "偏弱"
	case v >= 30:
		return "弱势，尚未超卖"
	case v >= 20:
		return "超卖"
	default:
		return "严重超卖"
	}
}

func briefLocation(p models.DailyPick) (string, []string, []string) {
	var b strings.Builder
	var bulls, bears []string
	if p.BollUp > p.BollDown && p.BollMid > 0 && p.ClosePrice > 0 {
		width := (p.BollUp - p.BollDown) / p.BollMid * 100
		pos := (p.ClosePrice - p.BollDown) / (p.BollUp - p.BollDown) * 100
		fmt.Fprintf(&b, "布林中轨 %s、上轨 %s、下轨 %s，带宽 %s，收盘在轨道的 %s 位置。",
			f2(p.BollMid), f2(p.BollUp), f2(p.BollDown), signed1(width), f0(pos)+"%")
		switch {
		case p.ClosePrice > p.BollUp:
			b.WriteString("收盘已经站上上轨，短线延伸过度。")
			bears = append(bears, "收盘在布林上轨外")
		case p.ClosePrice >= p.BollMid:
			b.WriteString("收盘在中轨与上轨之间。")
			if pos < 90 {
				bulls = append(bulls, "价格在布林中上轨")
			}
		case p.ClosePrice >= p.BollDown:
			b.WriteString("收盘落到中轨下方，强度转弱。")
			bears = append(bears, "价格在布林中轨下")
		default:
			b.WriteString("收盘跌破下轨。")
			bears = append(bears, "跌破布林下轨")
		}
	} else {
		b.WriteString("布林带样本不足。")
	}
	if p.Breakout20dPct != 0 {
		fmt.Fprintf(&b, "相对前 20 日高点 %s。", signed1(p.Breakout20dPct))
		if p.Breakout20dPct > 0 {
			bulls = append(bulls, "突破近 20 日高点")
		} else if p.Breakout20dPct <= -8 {
			bears = append(bears, "远离近 20 日高点")
		}
	}
	if p.AmplitudePct > 0 {
		fmt.Fprintf(&b, "当日振幅 %s。", signed1(p.AmplitudePct))
	}
	if p.BodyPct > 0 {
		fmt.Fprintf(&b, "K 线实体占振幅 %s。", signed1(p.BodyPct*100))
		switch {
		case p.BodyPct >= 0.6 && p.ClosePrice >= p.OpenPrice:
			b.WriteString("阳线实体主导。")
			bulls = append(bulls, "阳线实体主导")
		case p.BodyPct >= 0.6 && p.ClosePrice < p.OpenPrice:
			b.WriteString("阴线实体主导。")
			bears = append(bears, "阴线实体主导")
		case p.BodyPct < 0.3:
			b.WriteString("实体短，影线长，方向还没定。")
		}
	}
	if b.Len() == 0 {
		return "价格位置指标不足。", nil, nil
	}
	return b.String(), bulls, bears
}

func briefVolume(p models.DailyPick) (string, []string, []string) {
	var b strings.Builder
	var bulls, bears []string
	if p.TurnoverRate > 0 {
		fmt.Fprintf(&b, "换手率 %s。", signed1(p.TurnoverRate))
		switch {
		case p.TurnoverRate >= 15:
			bears = append(bears, "换手过热")
		case p.TurnoverRate >= 2:
			bulls = append(bulls, "换手适中")
		default:
			b.WriteString("换手偏低，交易不活跃。")
		}
	}
	switch {
	case p.VolumeRatio20d > 0:
		fmt.Fprintf(&b, "成交量是前 20 日均量的 %s 倍。", f1(p.VolumeRatio20d))
		switch {
		case p.VolumeRatio20d >= 1.5:
			bulls = append(bulls, "相对 20 日均量放量")
		case p.VolumeRatio20d < 0.7:
			bears = append(bears, "相对 20 日均量缩量")
		}
	case p.VolumeRatio > 0:
		fmt.Fprintf(&b, "量比 %s。", f1(p.VolumeRatio))
		if p.VolumeRatio >= 1.5 {
			bulls = append(bulls, "量比放大")
		} else if p.VolumeRatio < 0.7 {
			bears = append(bears, "量比偏低")
		}
	}
	if p.VolumeFactor >= 0.7 {
		b.WriteString("成交量因子处于显著放大区间。")
	} else if p.VolumeFactor >= 0.4 {
		b.WriteString("成交量因子温和放大。")
	}
	if b.Len() == 0 {
		return "量能指标不足。", nil, nil
	}
	return b.String(), bulls, bears
}

func briefRisk(p models.DailyPick) (string, []string, []string) {
	var b strings.Builder
	var bulls, bears []string
	if p.Atr14 > 0 {
		fmt.Fprintf(&b, "ATR(14) %s", f2(p.Atr14))
		if p.ClosePrice > 0 {
			atrPct := p.Atr14 / p.ClosePrice * 100
			fmt.Fprintf(&b, "，约占收盘价 %s", signed1(atrPct))
			if atrPct >= 6 {
				bears = append(bears, "ATR 波动偏大")
			}
		}
		b.WriteString("。")
	}
	if p.Volatility20dPct > 0 {
		fmt.Fprintf(&b, "20 日年化波动率 %s。", signed1(p.Volatility20dPct))
		if p.Volatility20dPct >= 50 {
			bears = append(bears, "年化波动偏高")
		}
	}
	if p.MaxDrawdownPct < 0 {
		fmt.Fprintf(&b, "统计窗口最大回撤 %s。", signed1(p.MaxDrawdownPct))
		if p.MaxDrawdownPct <= -25 {
			bears = append(bears, "历史回撤偏深")
		} else if p.MaxDrawdownPct >= -12 {
			bulls = append(bulls, "回撤可控")
		}
	}
	if p.ConsolidationDays > 0 {
		fmt.Fprintf(&b, "已连续 %d 个交易日振幅不超过 4%% 且涨跌不超过 2%%。", p.ConsolidationDays)
		if p.ConsolidationDays >= 5 {
			bulls = append(bulls, "窄幅整理")
		}
	}
	if b.Len() == 0 {
		return "波动与回撤指标不足。", nil, nil
	}
	return b.String(), bulls, bears
}

func briefVerdict(bulls, bears []string) string {
	nb, nr := len(bulls), len(bears)
	label := "中性"
	switch {
	case nb == 0 && nr == 0:
		return "可用技术指标不足，暂不给方向。"
	case nb >= nr+2 && nr <= 1:
		label = "偏多"
	case nb > nr:
		label = "中性偏多"
	case nr >= nb+2 && nb <= 1:
		label = "偏空"
	case nr > nb:
		label = "中性偏空"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "技术面%s（支持 %d 项，制约 %d 项）。", label, nb, nr)
	if nb > 0 {
		b.WriteString("支持：" + strings.Join(bulls, "、") + "。")
	}
	if nr > 0 {
		b.WriteString("制约：" + strings.Join(bears, "、") + "。")
	}
	return b.String()
}

func f0(v float64) string { return fmt.Sprintf("%.0f", v) }
func f1(v float64) string { return fmt.Sprintf("%.1f", v) }
func f2(v float64) string { return fmt.Sprintf("%.2f", v) }

func signed1(v float64) string { return fmt.Sprintf("%+.1f%%", v) }
func signed2(v float64) string { return fmt.Sprintf("%+.2f", v) }
