package khunter

import (
	"fmt"

	"go-stock/backend/models"
)

// BuildTechProfile 技术画像：基于本地日 K（近 30 根）计算趋势/量能/位置/动能
// 四项客观事实，纯展示用（落库 details.technical.profile），
// 不改变技术面策略命中评分口径（与狩猎场阈值/回测保持一致）。
func BuildTechProfile(bars []models.KLineBar) map[string]any {
	n := len(bars)
	if n == 0 {
		return nil
	}
	last := bars[n-1]
	close := last.Close

	// 趋势：收盘价与 MA5/MA10/MA20 的相对位置
	ma := func(p int) float64 {
		if n < p {
			return 0
		}
		sum := 0.0
		for i := n - p; i < n; i++ {
			sum += bars[i].Close
		}
		return sum / float64(p)
	}
	ma5, ma10, ma20 := ma(5), ma(10), ma(20)
	trend := "均线数据不足"
	if ma20 > 0 {
		switch {
		case close > ma5 && ma5 >= ma10 && ma10 >= ma20:
			trend = "多头排列（收盘价>MA5≥MA10≥MA20）"
		case close < ma5 && ma5 <= ma10 && ma10 <= ma20:
			trend = "空头排列（收盘价<MA5≤MA10≤MA20）"
		default:
			trend = fmt.Sprintf("震荡（收盘 %.2f / MA5 %.2f / MA10 %.2f / MA20 %.2f）", close, ma5, ma10, ma20)
		}
	}

	// 量能：今日成交量 vs 前 5 日均量
	volDesc := "量数据不足"
	if n >= 6 {
		avg5 := 0.0
		for i := n - 6; i < n-1; i++ {
			avg5 += float64(bars[i].Volume)
		}
		avg5 /= 5
		if avg5 > 0 {
			ratio := float64(last.Volume) / avg5
			switch {
			case ratio >= 2:
				volDesc = fmt.Sprintf("显著放量（今日量为前5日均量 %.1f 倍）", ratio)
			case ratio >= 1:
				volDesc = fmt.Sprintf("温和放量（今日量为前5日均量 %.1f 倍）", ratio)
			default:
				volDesc = fmt.Sprintf("缩量（今日量为前5日均量 %.1f 倍）", ratio)
			}
		}
	}

	// 位置：收盘价在近 20 根高低区间的分位
	posDesc := "位置数据不足"
	if n >= 2 {
		lookback := n
		if lookback > 20 {
			lookback = 20
		}
		hi, lo := bars[n-lookback].High, bars[n-lookback].Low
		for i := n - lookback; i < n; i++ {
			if bars[i].High > hi {
				hi = bars[i].High
			}
			if bars[i].Low < lo {
				lo = bars[i].Low
			}
		}
		if hi > lo {
			pct := (close - lo) / (hi - lo) * 100
			posDesc = fmt.Sprintf("近%d日价格区间分位 %.0f%%（低 %.2f ~ 高 %.2f）", lookback, pct, lo, hi)
		}
	}

	// 动能：近 5 日涨跌幅
	momDesc := "动能数据不足"
	if n >= 6 && bars[n-6].Close > 0 {
		chg := (close/bars[n-6].Close - 1) * 100
		switch {
		case chg >= 10:
			momDesc = fmt.Sprintf("近5日大涨 %+.1f%%", chg)
		case chg >= 3:
			momDesc = fmt.Sprintf("近5日走强 %+.1f%%", chg)
		case chg > -3:
			momDesc = fmt.Sprintf("近5日横盘 %+.1f%%", chg)
		default:
			momDesc = fmt.Sprintf("近5日走弱 %+.1f%%", chg)
		}
	}

	return map[string]any{
		"trend":    trend,
		"volume":   volDesc,
		"position": posDesc,
		"momentum": momDesc,
	}
}
