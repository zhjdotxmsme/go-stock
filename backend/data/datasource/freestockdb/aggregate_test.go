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
