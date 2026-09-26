package data

// strategy_patterns_test.go —— 形态策略（停机坪/回踩年线/高窄旗形）合成数据测试。
// 形态规则移植自 InStock，测试按规则边界构造：成立、单条件破坏、数据不足。

import (
	"strconv"
	"testing"
	"time"
)

// patternKLines 由显式 OHLC+量 合成带连续自然日的 KLineData。
func patternKLines(o, h, l, c, v []float64, start time.Time) []KLineData {
	n := len(c)
	out := make([]KLineData, n)
	for i := range n {
		out[i] = KLineData{
			Day:    start.AddDate(0, 0, i).Format("2006-01-02"),
			Open:   strconv.FormatFloat(o[i], 'f', 2, 64),
			Close:  strconv.FormatFloat(c[i], 'f', 2, 64),
			High:   strconv.FormatFloat(h[i], 'f', 2, 64),
			Low:    strconv.FormatFloat(l[i], 'f', 2, 64),
			Volume: strconv.FormatFloat(v[i], 'f', 0, 64),
		}
	}
	return out
}

// fillBars 批量填充等值 K 线段。
func fillBars(o, h, l, c, v []float64, from, to int, price, vol float64) {
	for i := from; i <= to; i++ {
		o[i], h[i], l[i], c[i], v[i] = price, price*1.005, price*0.995, price, vol
	}
}

func TestParkingApron(t *testing.T) {
	mk := func() ([]float64, []float64, []float64, []float64, []float64) {
		o := make([]float64, 20)
		h := make([]float64, 20)
		l := make([]float64, 20)
		c := make([]float64, 20)
		v := make([]float64, 20)
		fillBars(o, h, l, c, v, 0, 19, 10.0, 1000)
		return o, h, l, c, v
	}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)

	// 成立：17 日涨停（10→11，+10%），18/19 日高开小实体横盘，d=2 → 80 分
	o, h, l, c, v := mk()
	c[17] = 11.0
	o[17] = 10.1
	h[17] = 11.0
	l[17] = 10.0
	v[17] = 5000
	o[18], c[18], h[18], l[18] = 11.10, 11.15, 11.20, 11.05
	o[19], c[19], h[19], l[19] = 11.20, 11.25, 11.30, 11.10
	res := (&ParkingApronStrategy{}).Score(extCtx(patternKLines(o, h, l, c, v, start)))
	if res.Score != 80 {
		t.Errorf("停机坪 d=2 应 80 分, got %v (%s)", res.Score, res.Signal)
	}

	// 破坏：18 日低开（open < 涨停价 11）→ 0 分
	o, h, l, c, v = mk()
	c[17] = 11.0
	o[17] = 10.1
	h[17] = 11.0
	l[17] = 10.0
	v[17] = 5000
	o[18], c[18], h[18], l[18] = 10.80, 10.90, 11.00, 10.70
	o[19], c[19], h[19], l[19] = 11.00, 11.10, 11.15, 10.95
	res = (&ParkingApronStrategy{}).Score(extCtx(patternKLines(o, h, l, c, v, start)))
	if res.Score != 0 {
		t.Errorf("低开破坏形态应 0 分, got %v (%s)", res.Score, res.Signal)
	}

	// 当日涨停（d=0）→ 40 分观察态
	o, h, l, c, v = mk()
	c[19] = 11.0
	o[19] = 10.1
	h[19] = 11.0
	l[19] = 10.0
	v[19] = 5000
	res = (&ParkingApronStrategy{}).Score(extCtx(patternKLines(o, h, l, c, v, start)))
	if res.Score != 40 {
		t.Errorf("当日涨停应 40 分, got %v (%s)", res.Score, res.Signal)
	}

	// 涨停超过 3 日 → 形态过期 0 分（涨停在 15 日，之后 4 日均横盘）
	o, h, l, c, v = mk()
	c[15] = 11.0
	o[15] = 10.1
	h[15] = 11.0
	l[15] = 10.0
	v[15] = 5000
	for i := 16; i <= 19; i++ {
		o[i], c[i], h[i], l[i] = 11.10, 11.15, 11.20, 11.05
	}
	res = (&ParkingApronStrategy{}).Score(extCtx(patternKLines(o, h, l, c, v, start)))
	if res.Score != 0 {
		t.Errorf("横盘超3日应 0 分, got %v (%s)", res.Score, res.Signal)
	}

	// 无涨停 → 0 分
	o, h, l, c, v = mk()
	res = (&ParkingApronStrategy{}).Score(extCtx(patternKLines(o, h, l, c, v, start)))
	if res.Score != 0 {
		t.Errorf("无涨停应 0 分, got %v (%s)", res.Score, res.Signal)
	}
}

// ma250Ctx 构造 320 根回踩年线形态（或破坏变体）的 Context。
// 0-200: 12.0 平台；201-260: 下跌至 9.5（跌破年线）；261-300: 拉升至 20（放量突破）；
// 301-318: 缩量回踩至 15.5（不破年线）；319: 平收。
func ma250Ctx(breakHold bool) *StrategyContext {
	n := 320
	o := make([]float64, n)
	h := make([]float64, n)
	l := make([]float64, n)
	c := make([]float64, n)
	v := make([]float64, n)
	fillBars(o, h, l, c, v, 0, 200, 12.0, 1000)
	for i := 201; i <= 260; i++ {
		p := 12.0 - float64(i-200)*(2.5/60.0) // 12 → 9.5
		o[i], h[i], l[i], c[i], v[i] = p, p*1.005, p*0.995, p, 1000
	}
	for i := 261; i <= 300; i++ {
		p := 9.5 + float64(i-260)*(10.5/40.0) // 9.5 → 20
		o[i], h[i], l[i], c[i], v[i] = p, p*1.005, p*0.995, p, 1000
	}
	v[300] = 3000 // 最高日放量
	for i := 301; i <= 318; i++ {
		var p float64
		if breakHold {
			p = 20.0 - float64(i-300)*(9.0/18.0) // 回踩 20 → 11，跌破年线
		} else {
			p = 20.0 - float64(i-300)*(4.5/18.0) // 回踩 20 → 15.5
		}
		o[i], h[i], l[i], c[i], v[i] = p, p*1.005, p*0.995, p, 1000
	}
	fillBars(o, h, l, c, v, 319, 319, c[318], 1000)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local)
	return extCtx(patternKLines(o, h, l, c, v, start))
}

func TestBacktraceMA250(t *testing.T) {
	res := (&BacktraceMA250Strategy{}).Score(ma250Ctx(false))
	if res.Score != 100 {
		t.Errorf("完整回踩年线应 100 分, got %v (%s) factors=%v", res.Score, res.Signal, res.Factors)
	}

	res = (&BacktraceMA250Strategy{}).Score(ma250Ctx(true))
	if res.Factors["hold_above_ma250"] != 0 || res.Score >= 100 {
		t.Errorf("跌破年线不应满分, got %v (%s) factors=%v", res.Score, res.Signal, res.Factors)
	}

	// 数据不足
	short := extCtx(extKLines(extCloses(10, []float64{1, 1, 1}), 1))
	res = (&BacktraceMA250Strategy{}).Score(short)
	if res.Score != 0 || res.Signal == "" {
		t.Errorf("短数据应安全返回 0 分, got %v", res.Score)
	}
}

func TestHighTightFlag(t *testing.T) {
	mk := func(consec bool) *StrategyContext {
		n := 60
		o := make([]float64, n)
		h := make([]float64, n)
		l := make([]float64, n)
		c := make([]float64, n)
		v := make([]float64, n)
		fillBars(o, h, l, c, v, 0, 35, 10.0, 1000)
		// 观察窗 [36,49]：急涨旗杆 10 → 19.5
		for i := 36; i <= 49; i++ {
			var p float64
			if consec && i == 36 {
				p = 11.0 // +10%
			} else if consec && i == 37 {
				p = 12.1 // 再 +10%，连续两日
			} else {
				p = 10.0 + float64(i-35)*(9.5/14.0) // 平滑爬升到 19.5
			}
			o[i], h[i], l[i], c[i], v[i] = p*0.998, p*1.002, p*0.995, p, 1000
		}
		l[36] = 9.98                               // 旗杆起涨日下探，锚定观察窗最低价
		fillBars(o, h, l, c, v, 50, 59, 19.0, 800) // 高位窄旗整理
		start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)
		return extCtx(patternKLines(o, h, l, c, v, start))
	}

	res := (&HighTightFlagStrategy{}).Score(mk(true))
	if res.Score != 100 {
		t.Errorf("完整高窄旗形应 100 分, got %v (%s) factors=%v", res.Score, res.Signal, res.Factors)
	}

	res = (&HighTightFlagStrategy{}).Score(mk(false))
	if res.Factors["consecutive_limit_up"] != 0 {
		t.Errorf("无连续涨停不应给该项分, factors=%v", res.Factors)
	}

	short := extCtx(extKLines(extCloses(10, []float64{1, 1, 1}), 1))
	res = (&HighTightFlagStrategy{}).Score(short)
	if res.Score != 0 || res.Signal == "" {
		t.Errorf("短数据应安全返回 0 分, got %v", res.Score)
	}
}
