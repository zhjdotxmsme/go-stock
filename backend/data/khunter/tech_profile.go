package khunter

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"go-stock/backend/data"
	"go-stock/backend/data/indicator"
	"go-stock/backend/data/khunter/strategy"
	"go-stock/backend/db"
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
		// 与每日选股同一套口径：均线、MACD、RSI、KDJ、布林、量能、波动、回撤。
		"sections": data.DailyPickTechBrief(snapshotFromBars(bars)),
	}
}

// snapshotFromBars 把本地日 K 收成推荐日指标快照，供技术结论复用。
func snapshotFromBars(bars []models.KLineBar) models.DailyPick {
	n := len(bars)
	last := bars[n-1]
	closes := make([]float64, n)
	highs := make([]float64, n)
	lows := make([]float64, n)
	vols := make([]float64, n)
	for i, bar := range bars {
		closes[i] = bar.Close
		highs[i] = bar.High
		lows[i] = bar.Low
		vols[i] = float64(bar.Volume)
	}
	p := models.DailyPick{
		ClosePrice: last.Close,
		OpenPrice:  last.Open,
		HighPrice:  last.High,
		LowPrice:   last.Low,
		Volume:     last.Volume,
	}
	prev := last.PrevClose
	if prev <= 0 && n >= 2 {
		prev = bars[n-2].Close
	}
	if prev > 0 {
		p.ChangePercent = (last.Close/prev - 1) * 100
	}
	p.Ma5 = smaLast(closes, 5)
	p.Ma10 = smaLast(closes, 10)
	p.Ma20 = smaLast(closes, 20)
	p.Ma60 = smaLast(closes, 60)
	dif, dea, _ := indicator.MACD(closes, 12, 26, 9)
	if v, ok := indicator.LastValid(dif); ok {
		p.Macd = round2(v)
	}
	if v, ok := indicator.LastValid(dea); ok {
		p.MacdSignal = round2(v)
	}
	if v, ok := indicator.LastValid(indicator.RSI(closes, 14)); ok {
		p.Rsi14 = round2(v)
	}
	k, d, j := indicator.KDJ(highs, lows, closes, 9, 3, 3)
	if v, ok := indicator.LastValid(k); ok {
		p.KdjK = round2(v)
	}
	if v, ok := indicator.LastValid(d); ok {
		p.KdjD = round2(v)
	}
	if v, ok := indicator.LastValid(j); ok {
		p.KdjJ = round2(v)
	}
	mid, up, low := indicator.BOLL(closes, 20, 2)
	if v, ok := indicator.LastValid(mid); ok {
		p.BollMid = round2(v)
	}
	if v, ok := indicator.LastValid(up); ok {
		p.BollUp = round2(v)
	}
	if v, ok := indicator.LastValid(low); ok {
		p.BollDown = round2(v)
	}
	if v, ok := indicator.LastValid(indicator.ATR(highs, lows, closes, 14)); ok {
		p.Atr14 = round2(v)
	}
	if n >= 21 {
		base := highs[n-21]
		for _, h := range highs[n-21 : n-1] {
			if h > base {
				base = h
			}
		}
		if base > 0 {
			p.Breakout20dPct = round2((closes[n-1] - base) / base * 100)
		}
		sum := 0.0
		for _, v := range vols[n-21 : n-1] {
			sum += v
		}
		if avg := sum / 20; avg > 0 {
			p.VolumeRatio20d = round2(vols[n-1] / avg)
		}
	}
	if n >= 2 && closes[0] > 0 {
		look := n - 1
		if look > 60 {
			look = 60
		}
		base := closes[n-1-look]
		if base > 0 {
			p.Change60dPct = round2((closes[n-1]/base - 1) * 100)
		}
	}
	if prev > 0 {
		p.AmplitudePct = round2((last.High - last.Low) / prev * 100)
	}
	if rng := last.High - last.Low; rng > 0 {
		p.BodyPct = round2(math.Abs(last.Close-last.Open) / rng)
	}
	if p.Ma20 > 0 {
		dev := (p.ClosePrice - p.Ma20) / p.Ma20 * 100
		p.PullbackMa20 = dev >= -3 && dev <= 3
	}
	for i := n - 1; i >= 1; i-- {
		if closes[i-1] <= 0 {
			break
		}
		amp := (highs[i] - lows[i]) / closes[i-1] * 100
		chg := math.Abs(closes[i]-closes[i-1]) / closes[i-1] * 100
		if amp <= 4 && chg <= 2 {
			p.ConsolidationDays++
		} else {
			break
		}
	}
	p.Volatility20dPct = round2(annVolPct(closes))
	p.MaxDrawdownPct = round2(windowMaxDD(closes))
	return p
}

func smaLast(xs []float64, period int) float64 {
	if period <= 0 || len(xs) < period {
		return 0
	}
	sum := 0.0
	for _, v := range xs[len(xs)-period:] {
		sum += v
	}
	return round2(sum / float64(period))
}

func annVolPct(closes []float64) float64 {
	if len(closes) < 3 {
		return 0
	}
	returns := make([]float64, 0, len(closes)-1)
	for i := 1; i < len(closes); i++ {
		if closes[i-1] <= 0 {
			continue
		}
		returns = append(returns, (closes[i]-closes[i-1])/closes[i-1])
	}
	if len(returns) < 2 {
		return 0
	}
	mean := 0.0
	for _, r := range returns {
		mean += r
	}
	mean /= float64(len(returns))
	varSq := 0.0
	for _, r := range returns {
		varSq += (r - mean) * (r - mean)
	}
	return math.Sqrt(varSq/float64(len(returns)-1)) * math.Sqrt(252) * 100
}

func windowMaxDD(closes []float64) float64 {
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

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// enrichScoreTechProfiles 给还没有分段结论的历史评分补上技术分析。
// 只读本地日 K，不重跑狩猎场流水线。
func enrichScoreTechProfiles(ctx context.Context, rows []models.KhunterScore) {
	if db.Dao == nil {
		return
	}
	for i := range rows {
		if rows[i].Code == "" || profileHasSections(rows[i].Details) {
			continue
		}
		bars, err := strategy.LoadDailyBars(ctx, rows[i].Code, 30)
		if err != nil || len(bars) == 0 {
			continue
		}
		rows[i].Details = mergeTechProfile(rows[i].Details, BuildTechProfile(bars))
	}
}

func profileHasSections(details string) bool {
	var root map[string]any
	if json.Unmarshal([]byte(details), &root) != nil {
		return false
	}
	tech, _ := root["technical"].(map[string]any)
	prof, _ := tech["profile"].(map[string]any)
	secs, _ := prof["sections"].([]any)
	return len(secs) > 0
}

func mergeTechProfile(details string, profile map[string]any) string {
	root := map[string]any{}
	if details != "" {
		_ = json.Unmarshal([]byte(details), &root)
	}
	tech, _ := root["technical"].(map[string]any)
	if tech == nil {
		tech = map[string]any{}
		root["technical"] = tech
	}
	tech["profile"] = profile
	b, err := json.Marshal(root)
	if err != nil {
		return details
	}
	return string(b)
}
