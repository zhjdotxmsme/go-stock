package history

import (
	"strings"
	"time"

	"go-stock/backend/data/datasource"
	"go-stock/backend/util/timeutil"
)

// maxNonSessionWeekdays 工作日跨度不超过该值的内部空洞视为休市（春节/国庆约 7～8 个工作日），
// 不作为同步失败或重新排队的依据。更长的停牌必须向数据源核实后再写入 EmptySpans。
const maxNonSessionWeekdays = 12

// syncChunkMax 单段锚定拉取上限。超过则从窗口末端向前分块，避免只写下尾部就把断点推到末尾。
const syncChunkMax = 2000

type chunkAction int

const (
	chunkUnproven chunkAction = iota
	chunkUnanchored
	chunkEmpty
	chunkDone
	chunkPrefix
	chunkContinue
)

type chunkPlan struct {
	Action   chunkAction
	NextEnd  string
	EmptyEnd string
}

// decideSyncChunk 解释一次锚定拉取的结果。
// barDates 是数据源返回的交易日；countRequested 是本次请求的根数。
// 锚定被遵守的前提是最晚一根不晚于 cursorEnd。返回根数少于请求量表示历史已经到头
// （上市前/停牌），这时 cursor 之前、最早一根之前的工作日是核实过的空窗，不是假完成。
func decideSyncChunk(start, cursorEnd string, barDates []string, countRequested int) chunkPlan {
	if len(barDates) == 0 {
		return chunkPlan{Action: chunkUnproven}
	}
	minAll, maxAll := barDates[0], barDates[0]
	minIn := ""
	for _, d := range barDates {
		if d < minAll {
			minAll = d
		}
		if d > maxAll {
			maxAll = d
		}
		if d >= start && d <= cursorEnd && (minIn == "" || d < minIn) {
			minIn = d
		}
	}
	if maxAll > cursorEnd {
		if minIn == "" || minIn > start {
			return chunkPlan{Action: chunkUnanchored}
		}
		return chunkPlan{Action: chunkDone}
	}
	if minIn == "" {
		return chunkPlan{Action: chunkEmpty, EmptyEnd: cursorEnd}
	}
	exhausted := len(barDates) < countRequested
	if minIn <= start || exhausted {
		if minIn > start {
			prev, err := dayBefore(minIn)
			if err != nil || prev < start {
				return chunkPlan{Action: chunkDone}
			}
			return chunkPlan{Action: chunkPrefix, EmptyEnd: prev}
		}
		return chunkPlan{Action: chunkDone}
	}
	next, err := dayBefore(minIn)
	if err != nil || next >= cursorEnd || next < start {
		return chunkPlan{Action: chunkDone}
	}
	return chunkPlan{Action: chunkContinue, NextEnd: next}
}

func chunkCount(period, startDate, endDate string) int {
	startT, err1 := timeutil.ParseDate(startDate)
	endT, err2 := timeutil.ParseDate(endDate)
	if err1 != nil || err2 != nil || endT.Before(startT) {
		return 50
	}
	days := int(endT.Sub(startT).Hours()/24) + 1
	count := days * 5 / 7
	if period != "day" {
		count = days/7 + 2
		if period == "month" {
			count = days/28 + 2
		}
	}
	if count < 50 {
		count = 50
	}
	if count > syncChunkMax {
		count = syncChunkMax
	}
	return count
}

func dayBefore(date string) (string, error) {
	t, err := timeutil.ParseDate(date)
	if err != nil {
		return "", err
	}
	return timeutil.DateStr(t.AddDate(0, 0, -1)), nil
}

func dayAfter(date string) (string, error) {
	t, err := timeutil.ParseDate(date)
	if err != nil {
		return "", err
	}
	return timeutil.DateStr(t.AddDate(0, 0, 1)), nil
}

func weekdaySpan(start, end string) int {
	s, err1 := timeutil.ParseDate(start)
	e, err2 := timeutil.ParseDate(end)
	if err1 != nil || err2 != nil || e.Before(s) {
		return 0
	}
	n := 0
	for d := s; !d.After(e); d = d.AddDate(0, 0, 1) {
		if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
			n++
		}
	}
	return n
}

type dateSpan struct {
	Start string
	End   string
}

func parseEmptySpans(raw string) []dateSpan {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var out []dateSpan
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		i := strings.IndexByte(part, '/')
		if i <= 0 || i >= len(part)-1 {
			continue
		}
		s, e := part[:i], part[i+1:]
		if s > e {
			continue
		}
		out = mergeSpanList(out, dateSpan{Start: s, End: e})
	}
	return out
}

func formatEmptySpans(spans []dateSpan) string {
	if len(spans) == 0 {
		return ""
	}
	parts := make([]string, len(spans))
	for i, sp := range spans {
		parts[i] = sp.Start + "/" + sp.End
	}
	return strings.Join(parts, ",")
}

func mergeEmptySpan(raw, start, end string) string {
	if start == "" || end == "" || end < start {
		return raw
	}
	return formatEmptySpans(mergeSpanList(parseEmptySpans(raw), dateSpan{Start: start, End: end}))
}

func mergeSpanList(spans []dateSpan, add dateSpan) []dateSpan {
	spans = append(spans, add)
	if len(spans) == 0 {
		return nil
	}
	// 插入排序，跨度数量是休市段，很小
	for i := 1; i < len(spans); i++ {
		j := i
		for j > 0 && spans[j].Start < spans[j-1].Start {
			spans[j], spans[j-1] = spans[j-1], spans[j]
			j--
		}
	}
	out := []dateSpan{spans[0]}
	for _, sp := range spans[1:] {
		last := &out[len(out)-1]
		next, err := dayAfter(last.End)
		if err != nil {
			next = last.End
		}
		if sp.Start <= next {
			if sp.End > last.End {
				last.End = sp.End
			}
			continue
		}
		out = append(out, sp)
	}
	return out
}

func subtractEmptySpans(gaps []datasource.DateRange, raw string) []datasource.DateRange {
	spans := parseEmptySpans(raw)
	if len(spans) == 0 {
		return gaps
	}
	var out []datasource.DateRange
	for _, g := range gaps {
		parts := []dateSpan{{Start: g.Start, End: g.End}}
		for _, sp := range spans {
			var next []dateSpan
			for _, p := range parts {
				next = append(next, subtractOne(p, sp)...)
			}
			parts = next
		}
		for _, p := range parts {
			if p.Start != "" && p.End != "" && p.Start <= p.End {
				out = append(out, datasource.DateRange{Start: p.Start, End: p.End})
			}
		}
	}
	return out
}

func subtractOne(p, sp dateSpan) []dateSpan {
	if sp.End < p.Start || sp.Start > p.End {
		return []dateSpan{p}
	}
	if sp.Start <= p.Start && sp.End >= p.End {
		return nil
	}
	var out []dateSpan
	if sp.Start > p.Start {
		prev, err := dayBefore(sp.Start)
		if err == nil && prev >= p.Start {
			out = append(out, dateSpan{Start: p.Start, End: prev})
		}
	}
	if sp.End < p.End {
		next, err := dayAfter(sp.End)
		if err == nil && next <= p.End {
			out = append(out, dateSpan{Start: next, End: p.End})
		}
	}
	return out
}

// shouldSyncGap 跳过两侧都已有 K 线、且短于长假阈值的内部空洞。
// 窗口两端的短缺口仍然同步（尾部增量、起始日休市都要核实）。
func shouldSyncGap(gap datasource.DateRange, taskStart, taskEnd string) bool {
	if gap.Start <= taskStart || gap.End >= taskEnd {
		return true
	}
	return weekdaySpan(gap.Start, gap.End) > maxNonSessionWeekdays
}

// blockingGaps 返回仍会让任务失败的缺口。
// 不超过长假阈值的空洞（内部休市，或同步日正好停在休市）不失败。
// 更长的尾部或缺段仍然失败。贴着结束日的短缺口由 checkpointFromGaps 留在断点之前，下次再扫。
func blockingGaps(gaps []datasource.DateRange) []datasource.DateRange {
	var out []datasource.DateRange
	for _, g := range gaps {
		if weekdaySpan(g.Start, g.End) > maxNonSessionWeekdays {
			out = append(out, g)
		}
	}
	return out
}

// checkpointFromGaps 给出可以安全续跑的日期。
// 尾部仍缺数据时停在缺口前一天，下次从这里重扫；前缀有未关闭的长缺口时不前移。
func checkpointFromGaps(taskStart, taskEnd string, gaps []datasource.DateRange) string {
	for _, g := range gaps {
		if weekdaySpan(g.Start, g.End) > maxNonSessionWeekdays && g.End < taskEnd {
			return ""
		}
	}
	for _, g := range gaps {
		if g.End >= taskEnd {
			prev, err := dayBefore(g.Start)
			if err != nil || prev < taskStart {
				return ""
			}
			return prev
		}
	}
	return taskEnd
}
