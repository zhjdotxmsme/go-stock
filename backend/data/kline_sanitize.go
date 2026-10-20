package data

import (
	"go-stock/backend/logger"
	"go-stock/backend/util/timeutil"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	reKlineYearOnly  = regexp.MustCompile(`^\d{4}$`)
	reKlineYearMonth = regexp.MustCompile(`^(\d{4})-(\d{2})$`)
)

const klineMinDate = "1990-01-01"

// klineDayDateKey 将 K 线 Day 字段归一为可比较的 YYYY-MM-DD；无法解析或明显非法时 ok=false。
func klineDayDateKey(day string) (dateKey string, ok bool) {
	s := strings.TrimSpace(strings.ReplaceAll(day, "/", "-"))
	if s == "" {
		return "", false
	}
	if reKlineYearOnly.MatchString(s) {
		y, err := strconv.Atoi(s)
		if err != nil {
			return "", false
		}
		if y < 1990 || y > time.Now().Year()+1 {
			return "", false
		}
		return s + "-01-01", true
	}
	if m := reKlineYearMonth.FindStringSubmatch(s); len(m) == 3 {
		y, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		if y < 1990 || y > time.Now().Year()+1 || mo < 1 || mo > 12 {
			return "", false
		}
		return m[1] + "-" + m[2] + "-01", true
	}
	t, err := timeutil.ParseDateTime(s)
	if err != nil || t.IsZero() {
		return "", false
	}
	return timeutil.DateStr(t), true
}

// SanitizeKLineData 丢弃 Day 无法解析或明显超出合理区间的 K 线（指数 mootdx 脏数据等）。
func SanitizeKLineData(list []KLineData) []KLineData {
	if len(list) == 0 {
		return list
	}
	cutoff := timeutil.DateStr(time.Now().AddDate(0, 0, 1))
	out := make([]KLineData, 0, len(list))
	var firstBad KLineData
	dropped := 0
	for _, kd := range list {
		key, ok := klineDayDateKey(kd.Day)
		if !ok || key < klineMinDate || key > cutoff {
			if dropped == 0 {
				firstBad = kd
			}
			dropped++
			continue
		}
		out = append(out, kd)
	}
	if dropped > 0 {
		logger.SugaredLogger.Warnf("kline api sanitize: 丢弃 %d 条脏 K 线（日期非法或晚于 %s），首条 day=%q close=%s",
			dropped, cutoff, firstBad.Day, firstBad.Close)
	}
	return out
}
