package history

import (
	"testing"

	"go-stock/backend/data/datasource"
)

func TestDecideSyncChunk(t *testing.T) {
	const start = "2020-01-01"
	const end = "2024-06-30"

	tests := []struct {
		name    string
		bars    []string
		count   int
		action  chunkAction
		nextEnd string
		empty   string
	}{
		{name: "空响应", count: 10, action: chunkUnproven},
		{
			name: "锚定未被遵守且窗口内没有 bar",
			bars: []string{"2024-07-02"}, count: 10, action: chunkUnanchored,
		},
		{
			name: "窗口起点已被覆盖",
			bars: []string{"2020-01-01", "2020-01-02"}, count: 2, action: chunkDone,
		},
		{
			name: "满批且只覆盖尾部，继续向前",
			bars: []string{"2024-06-28", "2024-06-29", "2024-06-30"}, count: 3,
			action: chunkContinue, nextEnd: "2024-06-27",
		},
		{
			name: "历史已经到头，起点之前是空窗",
			bars: []string{"2024-06-29", "2024-06-30"}, count: 5,
			action: chunkPrefix, empty: "2024-06-28",
		},
		{
			name: "窗口内没有任何交易日",
			bars: []string{"2019-01-02", "2019-06-03"}, count: 5,
			action: chunkEmpty, empty: end,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decideSyncChunk(start, end, tt.bars, tt.count)
			if got.Action != tt.action {
				t.Fatalf("action=%v, want %v", got.Action, tt.action)
			}
			if tt.nextEnd != "" && got.NextEnd != tt.nextEnd {
				t.Fatalf("next=%s, want %s", got.NextEnd, tt.nextEnd)
			}
			if tt.empty != "" && got.EmptyEnd != tt.empty {
				t.Fatalf("emptyEnd=%s, want %s", got.EmptyEnd, tt.empty)
			}
		})
	}
}

func TestBlockingGapsAndCheckpoint(t *testing.T) {
	short := []datasource.DateRange{{Start: "2024-10-01", End: "2024-10-07"}}
	if n := weekdaySpan(short[0].Start, short[0].End); n > maxNonSessionWeekdays {
		t.Fatalf("国庆样本应落在长假阈值内，实际 %d 个工作日", n)
	}
	if len(blockingGaps(short)) != 0 {
		t.Fatal("长假以内的空洞不应让任务失败")
	}

	long := []datasource.DateRange{{Start: "2024-01-02", End: "2024-03-01"}}
	if len(blockingGaps(long)) != 1 {
		t.Fatal("超过长假阈值的缺口应失败")
	}

	if got := checkpointFromGaps("2020-01-01", "2024-06-30", nil); got != "2024-06-30" {
		t.Fatalf("无缺口断点=%s", got)
	}
	tail := []datasource.DateRange{{Start: "2024-06-28", End: "2024-06-30"}}
	if got := checkpointFromGaps("2020-01-01", "2024-06-30", tail); got != "2024-06-27" {
		t.Fatalf("尾部缺口断点=%s，want 2024-06-27", got)
	}
	if got := checkpointFromGaps("2020-01-01", "2024-06-30", long); got != "" {
		t.Fatalf("前缀长缺口不应前移断点，实际 %q", got)
	}
}

func TestEmptySpanRoundTrip(t *testing.T) {
	merged := mergeEmptySpan("2020-01-02/2020-01-03", "2020-01-04", "2020-01-06")
	if merged != "2020-01-02/2020-01-06" {
		t.Fatalf("相邻空窗应合并，实际 %s", merged)
	}
	gaps := subtractEmptySpans(
		[]datasource.DateRange{{Start: "2020-01-01", End: "2020-01-10"}},
		merged,
	)
	if len(gaps) != 2 || gaps[0].End != "2020-01-01" || gaps[1].Start != "2020-01-07" {
		t.Fatalf("扣除空窗后=%v", gaps)
	}
}

func TestShouldSyncGapSkipsInteriorHoliday(t *testing.T) {
	gap := datasource.DateRange{Start: "2024-10-01", End: "2024-10-07"}
	if shouldSyncGap(gap, "2020-01-01", "2024-12-31") {
		t.Fatal("两侧都有数据的长假不应再打数据源")
	}
	edge := datasource.DateRange{Start: "2024-12-30", End: "2024-12-31"}
	if !shouldSyncGap(edge, "2020-01-01", "2024-12-31") {
		t.Fatal("贴着窗口末端的缺口必须同步")
	}
}
