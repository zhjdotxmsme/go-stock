package freestockdb

import "testing"

func dayBar(date int64, o, h, l, c, v, a float64) Bar {
	return Bar{Date: date, Open: o, High: h, Low: l, Close: c, Volume: v, Amount: a}
}

func TestAggregateWeekly(t *testing.T) {
	daily := []Bar{
		dayBar(20260622, 10, 11, 9.5, 10.5, 100, 1000), // 周一
		dayBar(20260623, 10.5, 11.2, 10.4, 11.0, 200, 2000),
		dayBar(20260629, 11, 11.5, 10.8, 11.3, 150, 1500), // 下周一
	}
	out := AggregatePeriod(daily, true)
	if len(out) != 2 {
		t.Fatalf("weeks = %d, want 2", len(out))
	}
	w1 := out[0]
	if w1.Open != 10 || w1.High != 11.2 || w1.Low != 9.5 || w1.Close != 11.0 || w1.Volume != 300 {
		t.Errorf("w1 = %+v", w1)
	}
	if w1.Date != 20260623 {
		t.Errorf("week date should be last trading day 20260623, got %d", w1.Date)
	}
	if out[1].PreClose != 11.0 {
		t.Errorf("w2 preClose should be prev week close 11.0, got %v", out[1].PreClose)
	}
}

func TestAggregateMonthly(t *testing.T) {
	daily := []Bar{
		dayBar(20260625, 10, 11, 9.5, 10.5, 100, 1000),
		dayBar(20260701, 10.5, 11.2, 10.4, 11.0, 200, 2000),
	}
	out := AggregatePeriod(daily, false)
	if len(out) != 2 || out[0].Date != 20260625 || out[1].Date != 20260701 {
		t.Fatalf("months = %+v", out)
	}
}

func minBar(hhmmss int64, o, h, l, c, v float64) Bar {
	return Bar{Date: 20260625*1000000 + hhmmss, Open: o, High: h, Low: l, Close: c, Volume: v}
}

func TestAggregateMinutes(t *testing.T) {
	mins := []Bar{
		minBar(93100, 10, 10.1, 9.9, 10.0, 100),  // 09:31
		minBar(93200, 10, 10.2, 9.95, 10.1, 120), // 09:32
		minBar(93300, 10.1, 10.3, 10.0, 10.2, 80),
		minBar(93400, 10.2, 10.4, 10.1, 10.3, 90),
		minBar(93500, 10.3, 10.5, 10.2, 10.4, 110), // 09:35 → 第一个 5m 结束
		minBar(93600, 10.4, 10.6, 10.3, 10.5, 100),
	}
	out := AggregateMinutes(mins, 5)
	if len(out) != 2 {
		t.Fatalf("5m bars = %d, want 2: %+v", len(out), out)
	}
	b1 := out[0]
	if b1.Open != 10 || b1.High != 10.5 || b1.Low != 9.9 || b1.Close != 10.4 || b1.Volume != 500 {
		t.Errorf("b1 = %+v", b1)
	}
	// 对齐结束时刻 09:35 → date = 20260625093500
	if b1.Date != 20260625093500 {
		t.Errorf("b1.Date = %d, want 20260625093500", b1.Date)
	}
}

func TestTradingElapsedBoundary(t *testing.T) {
	if e, ok := tradingElapsed(930); !ok || e != 0 {
		t.Errorf("09:30 → %d,%v", e, ok)
	}
	if e, ok := tradingElapsed(1130); !ok || e != 120 {
		t.Errorf("11:30 → %d,%v", e, ok)
	}
	if e, ok := tradingElapsed(1300); !ok || e != 121 {
		t.Errorf("13:00 → %d,%v", e, ok)
	}
	if _, ok := tradingElapsed(1200); ok {
		t.Error("12:00 should be outside trading hours")
	}
}

func TestAggregateMinutesBoundary(t *testing.T) {
	// ① 09:30（elapsed=0）与 09:31-09:35 同组：第一根 5m 含 6 根 1m
	mins := []Bar{
		minBar(93000, 10, 10, 10, 10, 1),
		minBar(93100, 10, 10, 10, 10, 2),
		minBar(93200, 10, 10, 10, 10, 3),
		minBar(93300, 10, 10, 10, 10, 4),
		minBar(93400, 10, 10, 10, 10, 5),
		minBar(93500, 10, 10, 10, 10, 6),
		// ② 午休交界：11:30 与 13:00 不得同组
		minBar(113000, 10, 10, 10, 10, 10),
		minBar(130000, 10, 10, 10, 10, 100),
		// ③ 15:00 收盘 bar 归入最后一组
		minBar(150000, 10, 10, 10, 10, 1000),
	}
	out := AggregateMinutes(mins, 5)
	if len(out) != 4 {
		t.Fatalf("5m groups = %d, want 4: %+v", len(out), out)
	}
	if out[0].Volume != 21 || out[0].Date != 20260625093500 {
		t.Errorf("first 5m bar (6 根 1m) = %+v", out[0])
	}
	if out[1].Date != 20260625113000 {
		t.Errorf("11:30 group date = %d, want 20260625113000", out[1].Date)
	}
	if out[2].Date != 20260625130500 { // 13:00 → end=125 → 对齐结束时刻 13:05
		t.Errorf("13:00 group date = %d, want 20260625130500", out[2].Date)
	}
	if out[3].Date != 20260625150000 {
		t.Errorf("15:00 group date = %d, want 20260625150000", out[3].Date)
	}

	// ④ 固定输入：上午 4 个时段窗口各 6 根（24 根）+ 13:00 一根
	var day25 []Bar
	for _, start := range []int64{93000, 100100, 103100, 110100} {
		for i := int64(0); i < 6; i++ {
			day25 = append(day25, minBar(start+i*100, 10, 10, 10, 10, 1))
		}
	}
	day25 = append(day25, minBar(130000, 10, 10, 10, 10, 1))
	if got := len(AggregateMinutes(day25, 30)); got != 5 {
		t.Errorf("30m groups = %d, want 5", got)
	}
	if got := len(AggregateMinutes(day25, 60)); got != 3 {
		t.Errorf("60m groups = %d, want 3", got)
	}
}

func TestAggregateWeeklyFields(t *testing.T) {
	mk := func(date int64, turnover, volRatio, pe, pb, tmv, fmv float64, st bool) Bar {
		return Bar{Date: date, Open: 10, High: 11, Low: 9, Close: 10.5, Volume: 100,
			Turnover: turnover, VolRatio: volRatio, PeTTM: pe, Pb: pb,
			TotalMv: tmv, FloatMv: fmv, IsST: st}
	}
	daily := []Bar{
		mk(20260622, 1.5, 0.5, 10, 1, 100, 80, false),
		mk(20260623, 2.5, 1.5, 20, 2, 200, 160, true), // 周期末截面
		mk(20260629, 3.25, 2.5, 30, 3, 300, 240, false),
	}
	out := AggregatePeriod(daily, true)
	if len(out) != 2 {
		t.Fatalf("weeks = %d, want 2", len(out))
	}
	w1 := out[0]
	if w1.Turnover != 4.0 {
		t.Errorf("w1.Turnover = %v, want 4.0 (求和)", w1.Turnover)
	}
	if w1.VolRatio != 1.0 {
		t.Errorf("w1.VolRatio = %v, want 1.0 (均值 round 3)", w1.VolRatio)
	}
	if w1.PeTTM != 20 || w1.Pb != 2 || w1.TotalMv != 200 || w1.FloatMv != 160 || !w1.IsST {
		t.Errorf("w1 截面字段应取周期末: %+v", w1)
	}
	if out[1].Turnover != 3.25 {
		t.Errorf("w2.Turnover = %v, want 3.25", out[1].Turnover)
	}

	// 门控移除回归：PreClose 恒为 0（首周期、无 PreClose、Open=0）时仍须聚合
	zero := []Bar{
		{Date: 20260622, Open: 0, High: 1, Low: 0, Close: 1, Volume: 1, Turnover: 0.5, VolRatio: 0.5},
		{Date: 20260623, Open: 1, High: 2, Low: 1, Close: 2, Volume: 1, Turnover: 1.5, VolRatio: 2.5},
	}
	z := AggregatePeriod(zero, true)
	if len(z) != 1 {
		t.Fatalf("weeks = %d, want 1", len(z))
	}
	if z[0].PreClose != 0 {
		t.Fatalf("precondition: PreClose = %v, want 0", z[0].PreClose)
	}
	if z[0].Turnover != 2.0 || z[0].VolRatio != 1.5 {
		t.Errorf("PreClose==0 时 Turnover/VolRatio 仍应聚合: %+v", z[0])
	}
}

func TestAggregateWeeklyCrossYear(t *testing.T) {
	daily := []Bar{
		dayBar(20251229, 10, 11, 9.5, 10.5, 100, 1000),     // 2026-W01 周一
		dayBar(20260102, 10.5, 11.2, 10.4, 11.0, 200, 2000), // 2026-W01 周五
		dayBar(20260105, 11, 11.5, 10.8, 11.3, 150, 1500),   // 2026-W02 周一
	}
	out := AggregatePeriod(daily, true)
	if len(out) != 2 {
		t.Fatalf("weeks = %d, want 2 (跨年同一 ISO 周应合并): %+v", len(out), out)
	}
	w1 := out[0]
	if w1.Open != 10 || w1.Close != 11.0 || w1.Date != 20260102 || w1.Volume != 300 {
		t.Errorf("cross-year w1 = %+v", w1)
	}
}
