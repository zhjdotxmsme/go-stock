package freestockdb

import (
	"sort"
	"time"

	"go-stock/backend/util/timeutil"
)

func parseDate8(v int64) (time.Time, error) {
	return timeutil.ParseDate(itoa8(v))
}

func itoa8(v int64) string {
	buf := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf)
}

func AggregatePeriod(daily []Bar, weekly bool) []Bar {
	if len(daily) == 0 {
		return nil
	}
	type gkey struct{ y, n int }
	var order []gkey
	byKey := map[gkey][]Bar{}
	for _, b := range daily {
		t, err := parseDate8(b.Date)
		if err != nil {
			continue
		}
		var k gkey
		if weekly {
			y, w := t.ISOWeek()
			k = gkey{y, w}
		} else {
			k = gkey{t.Year(), int(t.Month())}
		}
		if _, ok := byKey[k]; !ok {
			order = append(order, k)
		}
		byKey[k] = append(byKey[k], b)
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i].y != order[j].y {
			return order[i].y < order[j].y
		}
		return order[i].n < order[j].n
	})
	out := make([]Bar, 0, len(order))
	for _, k := range order {
		items := byKey[k]
		m, ok := mergeBars(items)
		if !ok {
			continue
		}
		if len(out) > 0 {
			m.PreClose = out[len(out)-1].Close
		}
		if m.PreClose == 0 {
			m.PreClose = items[0].PreClose
		}
		if m.PreClose == 0 {
			m.PreClose = m.Open
		}
		if m.PreClose != 0 {
			m.Turnover = round(sumField(items, func(b Bar) float64 { return b.Turnover }), 3)
			m.VolRatio = round(avgField(items, func(b Bar) float64 { return b.VolRatio }), 3)
		}
		out = append(out, m)
	}
	return out
}

func mergeBars(items []Bar) (Bar, bool) {
	if len(items) == 0 {
		return Bar{}, false
	}
	m := items[0]
	for _, b := range items[1:] {
		if b.High > m.High {
			m.High = b.High
		}
		if b.Low < m.Low {
			m.Low = b.Low
		}
		m.Volume += b.Volume
		m.Amount += b.Amount
	}
	last := items[len(items)-1]
	m.Date, m.Close = last.Date, last.Close
	m.Code, m.Name = last.Code, last.Name
	m.PeTTM, m.Pb = last.PeTTM, last.Pb
	m.TotalMv, m.FloatMv, m.IsST = last.TotalMv, last.FloatMv, last.IsST
	m.PreClose = 0
	return m, true
}

func sumField(items []Bar, f func(Bar) float64) float64 {
	var s float64
	for _, b := range items {
		s += f(b)
	}
	return s
}

func avgField(items []Bar, f func(Bar) float64) float64 {
	if len(items) == 0 {
		return 0
	}
	return sumField(items, f) / float64(len(items))
}

func tradingElapsed(hhmm int) (int, bool) {
	m := hhmm/100*60 + hhmm%100
	switch {
	case m >= 570 && m <= 690:
		return m - 570, true
	case m >= 780 && m <= 900:
		if m == 780 {
			return 121, true
		}
		return 120 + (m - 780), true
	}
	return 0, false
}

func elapsedToMinuteOfDay(e int) int {
	if e > 240 {
		e = 240
	}
	if e <= 120 {
		m := 570 + e
		return m/60*100 + m%60
	}
	m := 780 + (e - 120)
	return m/60*100 + m%60
}

func AggregateMinutes(mins []Bar, interval int) []Bar {
	if len(mins) == 0 || interval <= 1 {
		return mins
	}
	type gkey struct {
		ymd int64
		end int
	}
	var order []gkey
	byKey := map[gkey][]Bar{}
	for _, b := range mins {
		ymd := b.Date / 1000000
		hhmm := int(b.Date/10000)%100*100 + int(b.Date/100)%100
		elapsed, ok := tradingElapsed(hhmm)
		if !ok {
			continue
		}
		end := interval
		if elapsed > 0 {
			end = ((elapsed-1)/interval + 1) * interval
		}
		k := gkey{ymd, end}
		if _, ok := byKey[k]; !ok {
			order = append(order, k)
		}
		byKey[k] = append(byKey[k], b)
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i].ymd != order[j].ymd {
			return order[i].ymd < order[j].ymd
		}
		return order[i].end < order[j].end
	})
	out := make([]Bar, 0, len(order))
	for _, k := range order {
		m, ok := mergeBars(byKey[k])
		if !ok {
			continue
		}
		hhmm := elapsedToMinuteOfDay(k.end)
		if hhmm/100 >= 24 {
			hhmm = 2359
		}
		m.Date = k.ymd*1000000 + int64(hhmm)*100
		if len(out) > 0 {
			m.PreClose = out[len(out)-1].Close
		} else if m.PreClose == 0 {
			m.PreClose = m.Open
		}
		out = append(out, m)
	}
	return out
}
