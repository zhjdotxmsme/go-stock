// Package timeutil 统一时间解析/格式化转换层。
//
// 项目约定：
//   - 业务日期（交易日、推荐日等）一律为本地时区语义的 "2006-01-02" 字符串，
//     字典序比较安全（禁止出现 "2006/01/02"、"20060102" 等变体进入比较链路）。
//   - 业务时间戳为本地时区 "2006-01-02 15:04:05"。
//   - 解析外部数据的时间字符串一律使用本包（ParseInLocation(time.Local)），
//     禁止裸 time.Parse（其结果为 UTC，与本地存储比较会产生最多 8h 边界偏移）。
//   - 外部时间戳（秒/毫秒）用 UnixToLocal / UnixMilliToLocal 转换。
package timeutil

import (
	"strings"
	"time"
)

const (
	// DateLayout 业务日期格式（YYYY-MM-DD）
	DateLayout = "2006-01-02"
	// DateTimeLayout 业务时间格式（YYYY-MM-DD HH:MM:SS）
	DateTimeLayout = "2006-01-02 15:04:05"
	// DateMinuteLayout K线分钟级格式（YYYY-MM-DD HH:MM）
	DateMinuteLayout = "2006-01-02 15:04"
	// CompactDateLayout tushare 等紧凑日期（YYYYMMDD）
	CompactDateLayout = "20060102"
)

// dateTimeLayouts 按尝试顺序排列的日期时间格式（均为本地时区解析）
var dateTimeLayouts = []string{
	DateTimeLayout,
	DateMinuteLayout,
	DateLayout,
	CompactDateLayout,
	time.RFC3339, // Wails 序列化的 time.Time / 外部 ISO 时间，自带时区
}

// normalizeInput 归一化常见变体："2026/09/21" → "2026-09-21"，去掉首尾空白与尾部 .000 毫秒。
func normalizeInput(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "/", "-")
	return s
}

// ParseDate 解析业务日期字符串（本地时区）。支持 "2006-01-02"、"2006/01/02"、"20060102"。
// 解析失败返回零值与 error。
func ParseDate(s string) (time.Time, error) {
	s = normalizeInput(s)
	if t, err := time.ParseInLocation(DateLayout, s, time.Local); err == nil {
		return t, nil
	}
	return time.ParseInLocation(CompactDateLayout, s, time.Local)
}

// ParseDateTime 解析业务时间字符串（本地时区），按序尝试
// "2006-01-02 15:04:05" / "2006-01-02 15:04" / "2006-01-02" / "20060102" / RFC3339。
// 全部失败返回零值与最后一个 error。
func ParseDateTime(s string) (time.Time, error) {
	s = normalizeInput(s)
	var lastErr error
	for _, layout := range dateTimeLayouts {
		var t time.Time
		var err error
		if layout == time.RFC3339 {
			t, err = time.Parse(layout, s) // RFC3339 自带时区，直接解析
		} else {
			t, err = time.ParseInLocation(layout, s, time.Local)
		}
		if err == nil {
			return t, nil
		}
		lastErr = err
	}
	return time.Time{}, lastErr
}

// MustParseDateTime 同 ParseDateTime，失败返回零值（适合"尽力解析"的容错场景）。
func MustParseDateTime(s string) time.Time {
	t, _ := ParseDateTime(s)
	return t
}

// DateStr 格式化为业务日期字符串（YYYY-MM-DD，本地时区）。
func DateStr(t time.Time) string {
	return t.In(time.Local).Format(DateLayout)
}

// DateTimeStr 格式化为业务时间字符串（YYYY-MM-DD HH:MM:SS，本地时区）。
func DateTimeStr(t time.Time) string {
	return t.In(time.Local).Format(DateTimeLayout)
}

// UnixToLocal 秒级时间戳 → 本地时区 time.Time。
func UnixToLocal(sec int64) time.Time {
	return time.Unix(sec, 0).Local()
}

// UnixMilliToLocal 毫秒级时间戳 → 本地时区 time.Time。
func UnixMilliToLocal(ms int64) time.Time {
	return time.UnixMilli(ms).Local()
}

// TodayStr 今日日期字符串（YYYY-MM-DD，本地时区）。
func TodayStr() string {
	return DateStr(time.Now())
}

// DaysAgoStr n 天前的日期字符串（YYYY-MM-DD，本地时区）。
func DaysAgoStr(n int) string {
	return DateStr(time.Now().AddDate(0, 0, -n))
}
