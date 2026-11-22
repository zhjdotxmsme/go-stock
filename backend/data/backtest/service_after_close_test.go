package backtest

import (
	"testing"
	"time"
)

// 盘后补尾触发点：当日 15:10 前触发当日，已过（含恰好在 15:10 整）触发明天。
// 狩猎场确认依赖收盘后同步把当日 bar 补齐，该触发点错一天就会多等一个交易日。
func TestNextAfterCloseTrigger(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	day := func(h, m int) time.Time {
		return time.Date(2026, 11, 19, h, m, 0, 0, loc) // 周四
	}
	tomorrow := func(h, m int) time.Time {
		return time.Date(2026, 11, 20, h, m, 0, 0, loc) // 周五
	}
	at := func(want time.Time, got time.Time) {
		t.Helper()
		if !got.Equal(want) {
			t.Fatalf("触发点=%s, want=%s", got, want)
		}
	}

	// 盘中 14:54 → 当日 15:10
	at(day(15, 10), nextAfterCloseTrigger(day(14, 54)))
	// 收盘后 15:32（当日同步刚结束）→ 明天 15:10
	at(tomorrow(15, 10), nextAfterCloseTrigger(day(15, 32)))
	// 恰好 15:10 整 → 明天（再触发当日是无害空转，直接排明天）
	at(tomorrow(15, 10), nextAfterCloseTrigger(day(15, 10)))
	// 凌晨 00:30 → 当日 15:10
	at(day(15, 10), nextAfterCloseTrigger(day(0, 30)))
}
