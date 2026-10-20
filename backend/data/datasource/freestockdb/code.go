package freestockdb

import (
	"fmt"
	"strconv"
	"time"

	"go-stock/backend/stockcode"
	"go-stock/backend/util/timeutil"
)

// EngineCode 将 go-stock 内部代码转为 free-stockdb 键里的裸代码（6 位 A 股/ETF）。
func EngineCode(code string) (string, error) {
	norm := stockcode.Normalize(code)
	if norm == "" {
		return "", fmt.Errorf("freestockdb: empty stock code")
	}
	if !stockcode.IsA股(norm) {
		return "", fmt.Errorf("freestockdb: A-share only, got %q", code)
	}
	pure := stockcode.PureCode(norm)
	if pure == "" {
		return "", fmt.Errorf("freestockdb: cannot parse code %q", code)
	}
	return pure, nil
}

// InternalCode 把引擎返回的裸代码还原为 go-stock 内部格式（保留原输入的市场前缀）。
func InternalCode(originalInput, bareFromEngine string) string {
	norm := stockcode.Normalize(originalInput)
	prefix, _ := stockcode.ParseStockCode(norm)
	if prefix != "" && bareFromEngine != "" {
		return prefix + bareFromEngine
	}
	return stockcode.Normalize(bareFromEngine)
}

// barTime 将引擎 date 字段转为本地时区 time.Time。
// 日 K：8 位 YYYYMMDD；分钟 K：14 位 YYYYMMDDHHMMSS。
func barTime(date int64) time.Time {
	s := strconv.FormatInt(date, 10)
	if len(s) == 8 {
		return timeutil.MustParseDateTime(s)
	}
	if len(s) >= 14 {
		t, err := time.ParseInLocation("20060102150405", s[:14], time.Local)
		if err == nil {
			return t
		}
	}
	return time.Time{}
}
