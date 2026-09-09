package data

// strategy_technical_ext_test.go —— 6 个单指标扩展策略的合成数据测试。
//
// 合成 K 线全部字段为 string（对齐 KLineData 定义），Context 构建复刻引擎的
// ParseFloat 转换；断言：趋势数据下 adx_trend / donchian_breakout / supertrend_flip
// 得分为正、全策略无 panic、短数据/空数据全安全返回「K线数据不足」。

import (
	"fmt"
	"math"
	"strconv"
	"testing"
)

// extCloses 按逐根涨幅（百分数）合成收盘价序列。
func extCloses(start float64, pcts []float64) []float64 {
	closes := make([]float64, len(pcts))
	c := start
	for i, p := range pcts {
		c = c * (1 + p/100)
		closes[i] = c
	}
	return closes
}

// extKLines 由收盘价序列合成全 string 字段的 KLineData；最后一根成交量乘 lastVolMult。
func extKLines(closes []float64, lastVolMult float64) []KLineData {
	n := len(closes)
	klines := make([]KLineData, n)
	prev := closes[0] * 0.99
	for i := 0; i < n; i++ {
		c := closes[i]
		o := prev
		high := math.Max(o, c) * 1.008
		low := math.Min(o, c) * 0.992
		vol := 1_000_000 * (1 + 0.1*math.Sin(float64(i)))
		if i == n-1 {
			vol *= lastVolMult
		}
		klines[i] = KLineData{
			Day:           fmt.Sprintf("2024-01-%02d", i%28+1),
			Open:          strconv.FormatFloat(o, 'f', 2, 64),
			Close:         strconv.FormatFloat(c, 'f', 2, 64),
			High:          strconv.FormatFloat(high, 'f', 2, 64),
			Low:           strconv.FormatFloat(low, 'f', 2, 64),
			Volume:        strconv.FormatFloat(vol, 'f', 0, 64),
			Amount:        strconv.FormatFloat(vol*c, 'f', 0, 64),
			ChangePercent: strconv.FormatFloat((c/o-1)*100, 'f', 2, 64),
			ChangeValue:   strconv.FormatFloat(c-o, 'f', 2, 64),
			Amplitude:     strconv.FormatFloat((high-low)/o*100, 'f', 2, 64),
			TurnoverRate:  "1.50",
			VolumeRatio:   "1.00",
		}
		prev = c
	}
	return klines
}

// extCtx 复刻引擎的 KLineData → StrategyContext 转换。
func extCtx(klines []KLineData) *StrategyContext {
	n := len(klines)
	ctx := &StrategyContext{
		KLines:    klines,
		CloseP:    make([]float64, n),
		HighP:     make([]float64, n),
		LowP:      make([]float64, n),
		Volume:    make([]float64, n),
		StockCode: "000001",
		StockName: "平安银行",
	}
	for i, k := range klines {
		ctx.CloseP[i], _ = strconv.ParseFloat(k.Close, 64)
		ctx.HighP[i], _ = strconv.ParseFloat(k.High, 64)
		ctx.LowP[i], _ = strconv.ParseFloat(k.Low, 64)
		ctx.Volume[i], _ = strconv.ParseFloat(k.Volume, 64)
	}
	return ctx
}

// extAllExtStrategies 返回本文件的 6 个策略实例。
func extAllExtStrategies() []ScoringStrategy {
	return []ScoringStrategy{
		&ADXTrendStrategy{},
		&SuperTrendFlipStrategy{},
		&SARTrendStrategy{},
		&MFIFlowStrategy{},
		&OBVDivergenceStrategy{},
		&DonchianBreakoutStrategy{},
	}
}

// extSafeScore 带 recover 执行策略，panic 转为测试错误（绝不炸掉测试进程）。
func extSafeScore(t *testing.T, name string, s ScoringStrategy, ctx *StrategyContext) (res *StrategyResult) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("%s panicked: %v", name, r)
			res = &StrategyResult{Score: 0, Factors: map[string]float64{}, Signal: "panic"}
		}
	}()
	return s.Score(ctx)
}

// TestExtStrategiesNoPanicOnUptrend 120 根加速上升趋势：全策略无 panic、
// 分数在 [0,100]、Signal 非空；adx_trend / donchian_breakout 得分 > 0。
func TestExtStrategiesNoPanicOnUptrend(t *testing.T) {
	pcts := make([]float64, 120)
	for i := range pcts {
		switch {
		case i >= 115:
			pcts[i] = 2.0 + 0.4*float64(i-115) // 末端 5 根持续加速，保证 ADX 较 3 根前上行
		case i%7 == 6:
			pcts[i] = -2.0 // 周期性回踩（低点须击穿前一根低点才会产生 Wilder -DM），避免 -DI 归零使 ADX 饱和在 100
		default:
			pcts[i] = 1.2 + 0.25*math.Sin(float64(i)*0.7)
		}
	}
	ctx := extCtx(extKLines(extCloses(10, pcts), 2.0)) // 末日放量 2 倍

	got := map[string]float64{}
	for _, s := range extAllExtStrategies() {
		r := extSafeScore(t, s.Code(), s, ctx)
		if r == nil {
			t.Fatalf("%s returned nil result", s.Code())
		}
		if r.Score < 0 || r.Score > 100 {
			t.Errorf("%s Score=%.0f out of [0,100]", s.Code(), r.Score)
		}
		if r.Signal == "" {
			t.Errorf("%s Signal is empty", s.Code())
		}
		if r.Factors == nil {
			t.Errorf("%s Factors is nil", s.Code())
		}
		got[s.Code()] = r.Score
		t.Logf("%s(%s): score=%.0f signal=%s", s.Name(), s.Code(), r.Score, r.Signal)
	}
	for _, code := range []string{"adx_trend", "donchian_breakout"} {
		if got[code] <= 0 {
			t.Errorf("%s Score=%.0f, want >0 on uptrend data", code, got[code])
		}
	}
}

// TestExtStrategiesSuperTrendFlipOnReversal 先涨后挖坑、末日两根大阳：
// SuperTrend 应在最近 3 根内翻多且收盘站上线，得分 > 0。
// （从第 0 根起单边上行的序列不会在最近 3 根内产生 false→true 翻转，
// 故翻转断言使用「回撤后反转」的趋势序列。）
func TestExtStrategiesSuperTrendFlipOnReversal(t *testing.T) {
	pcts := make([]float64, 120)
	for i := range pcts {
		switch {
		case i < 112:
			pcts[i] = 0.5 + 0.2*math.Sin(float64(i)*0.9)
		case i < 118:
			pcts[i] = -1.8 // 6 根回调，把 SuperTrend 打翻为空头
		default:
			pcts[i] = 6.5 // 末日两根大阳，翻回多头
		}
	}
	ctx := extCtx(extKLines(extCloses(10, pcts), 1.5))

	r := extSafeScore(t, "supertrend_flip", &SuperTrendFlipStrategy{}, ctx)
	if r == nil {
		t.Fatal("supertrend_flip returned nil result")
	}
	if r.Score <= 0 {
		t.Errorf("supertrend_flip Score=%.0f, want >0 after dip-then-rally reversal (signal=%s)", r.Score, r.Signal)
	}
	t.Logf("supertrend_flip: score=%.0f signal=%s", r.Score, r.Signal)
}

// TestExtStrategiesShortDataSafe 3 根短数据 / 空数据 / nil Context 全安全返回 0 + 「K线数据不足」。
func TestExtStrategiesShortDataSafe(t *testing.T) {
	ctx3 := extCtx(extKLines(extCloses(10, []float64{1, 0.5, 2}), 1))
	empty := &StrategyContext{}
	for _, tc := range []struct {
		name string
		ctx  *StrategyContext
		bars int
	}{{"3bars", ctx3, 3}, {"empty", empty, 0}, {"nil", nil, 0}} {
		for _, s := range extAllExtStrategies() {
			r := extSafeScore(t, s.Code(), s, tc.ctx)
			if r == nil {
				t.Fatalf("%s(%s) returned nil result", tc.name, s.Code())
			}
			if r.Score != 0 {
				t.Errorf("%s(%s) Score=%.0f, want 0", tc.name, s.Code(), r.Score)
			}
			if r.Signal != "K线数据不足" {
				t.Errorf("%s(%s) Signal=%q, want K线数据不足", tc.name, s.Code(), r.Signal)
			}
			if r.Factors == nil {
				t.Errorf("%s(%s) Factors is nil", tc.name, s.Code())
			}
		}
	}
}
