package freestockdb

import (
	"context"
)

type Frequency string

const (
	Freq1d  Frequency = "1d"
	Freq1w  Frequency = "1w"
	Freq1M  Frequency = "1M"
	Freq1m  Frequency = "1m"
	Freq5m  Frequency = "5m"
	Freq15m Frequency = "15m"
	Freq30m Frequency = "30m"
	Freq60m Frequency = "60m"
)

type KLineService struct {
	c       *Client
	factors *FactorStore
}

func NewKLineService(c *Client, f *FactorStore) *KLineService {
	return &KLineService{c: c, factors: f}
}

func isMinuteFreq(f Frequency) bool {
	switch f {
	case Freq1m, Freq5m, Freq15m, Freq30m, Freq60m:
		return true
	}
	return false
}

func minuteInterval(f Frequency) int {
	switch f {
	case Freq5m:
		return 5
	case Freq15m:
		return 15
	case Freq30m:
		return 30
	case Freq60m:
		return 60
	}
	return 1
}

func rawFreq(f Frequency) Frequency {
	switch f {
	case Freq1w, Freq1M:
		return Freq1d
	case Freq5m, Freq15m, Freq30m, Freq60m:
		return Freq1m
	}
	return f
}

func rawMultiplier(f Frequency) int {
	switch f {
	case Freq1w:
		return 7
	case Freq1M:
		return 31
	case Freq5m:
		return 5
	case Freq15m:
		return 15
	case Freq30m:
		return 30
	case Freq60m:
		return 60
	}
	return 1
}

func dayToBar(d DayKBar) Bar {
	return Bar{Date: d.Date, Open: d.Open, High: d.High, Low: d.Low, Close: d.Close,
		PreClose: d.PreClose, Volume: d.Volume, Amount: d.Amount, Turnover: d.Turnover,
		VolRatio: d.VolRatio, PeTTM: d.PeTTM, Pb: d.Pb, TotalMv: d.TotalMv, FloatMv: d.FloatMv,
		IsST: d.IsST, Code: d.Code, Name: d.Name}
}

func minuteToBar(m MinuteKBar) Bar {
	return Bar{Date: m.Date, Open: m.Open, High: m.High, Low: m.Low, Close: m.Close,
		Volume: m.Volume, Amount: m.Amount, Code: m.Code}
}

func (s *KLineService) rawBars(ctx context.Context, code string, freq Frequency, start, end string, fq FQ, desc bool) ([]Bar, error) {
	if isMinuteFreq(freq) {
		mins, err := s.c.QueryMinuteK(ctx, code, start, end, desc)
		if err != nil {
			return nil, err
		}
		bars := make([]Bar, 0, len(mins))
		for _, m := range mins {
			bars = append(bars, minuteToBar(m))
		}
		return s.factors.AdjustBars(code, bars, fq), nil
	}
	days, err := s.c.QueryDayK(ctx, code, start, end, desc)
	if err != nil {
		return nil, err
	}
	bars := make([]Bar, 0, len(days))
	for _, d := range days {
		bars = append(bars, dayToBar(d))
	}
	return s.factors.AdjustBars(code, bars, fq), nil
}

func (s *KLineService) GetData(ctx context.Context, code string, freq Frequency, start, end string, fq FQ, desc bool, limit int) ([]Bar, error) {
	requiresAgg := freq == Freq1w || freq == Freq1M || minuteInterval(freq) > 1
	bars, err := s.rawBars(ctx, code, rawFreq(freq), start, end, fq, desc && !requiresAgg)
	if err != nil {
		return nil, err
	}
	switch {
	case freq == Freq1w:
		bars = AggregatePeriod(bars, true)
	case freq == Freq1M:
		bars = AggregatePeriod(bars, false)
	case minuteInterval(freq) > 1:
		bars = AggregateMinutes(bars, minuteInterval(freq))
	}
	if desc && requiresAgg {
		reverseBars(bars)
	}
	if limit > 0 && len(bars) > limit {
		bars = bars[:limit]
	}
	return bars, nil
}

func (s *KLineService) LastN(ctx context.Context, code string, freq Frequency, count int, fq FQ) ([]Bar, error) {
	if count <= 0 {
		count = 1
	}
	rf, mult := rawFreq(freq), rawMultiplier(freq)
	bars, err := s.rawBars(ctx, code, rf, "", "", fq, false)
	if err != nil {
		return nil, err
	}
	if need := count * mult; len(bars) > need {
		bars = bars[len(bars)-need:]
	}
	switch {
	case freq == Freq1w:
		bars = AggregatePeriod(bars, true)
	case freq == Freq1M:
		bars = AggregatePeriod(bars, false)
	case minuteInterval(freq) > 1:
		bars = AggregateMinutes(bars, minuteInterval(freq))
	}
	if len(bars) > count {
		bars = bars[len(bars)-count:]
	}
	return bars, nil
}

func reverseBars(b []Bar) {
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
}
