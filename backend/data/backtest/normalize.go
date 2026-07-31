package backtest

import (
	"strings"
)

// A 股代码在系统内存在多种格式：
//   - ts_code 格式（前端自动补全传入）：600519.SH / 000001.SZ
//   - 前缀格式（引擎与测试使用）：sh600519 / sz000001 / bj430047
//   - 裸码（种子脚本 scripts/history_seed/baostock_seed.py 写入 kline_bars）：600519
//
// 内部标准格式统一为小写交易所前缀 + 6 位数字，如 sh600519。
// 选择该格式的原因：与引擎、测试的既有用法一致；交易所信息显式无歧义；
// 小写避免大小写重复键。

// isSixDigits 报告 s 是否恰为 6 位数字。
func isSixDigits(s string) bool {
	if len(s) != 6 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// isExchange 报告 s 是否为合法交易所标识（不区分大小写）。
func isExchange(s string) bool {
	switch strings.ToLower(s) {
	case "sh", "sz", "bj":
		return true
	}
	return false
}

// inferExchange 按 6 位数字代码推断交易所（输入未显式给出交易所时使用）。
// 返回 "" 表示无法推断。
func inferExchange(digits string) string {
	if digits == "" {
		return ""
	}
	switch digits[0] {
	case '6':
		return "sh"
	case '9':
		// 920xxx 为北交所新号段，其余 9xxxxx（如 900 B 股）属上交所
		if strings.HasPrefix(digits, "920") {
			return "bj"
		}
		return "sh"
	case '4', '8':
		return "bj"
	case '5':
		return "sh" // 51xxxx/58xxxx 等沪市基金、ETF
	case '0', '1', '2', '3':
		return "sz"
	}
	return ""
}

// ParseStockCode 从任意支持的格式中提取 6 位数字代码与交易所。
// 支持：600519.SH、sh.600519、SH600519、sh600519、600519、BJ430047 等。
// exchange 为小写 sh/sz/bj；输入未显式给出交易所时按数字推断。
// ok=false 表示输入不含可识别的 6 位代码（如测试 mock 代码），调用方应原样使用输入。
func ParseStockCode(input string) (digits, exchange string, ok bool) {
	s := strings.TrimSpace(input)
	if s == "" {
		return "", "", false
	}

	var exchPart string
	if i := strings.Index(s, "."); i >= 0 {
		left, right := s[:i], s[i+1:]
		switch {
		case isSixDigits(left) && isExchange(right): // 600519.SH
			digits, exchPart = left, right
		case isSixDigits(right) && isExchange(left): // sh.600519
			digits, exchPart = right, left
		default:
			return "", "", false
		}
	} else {
		// 字母前缀 + 数字（sh600519 / SH600519），或纯裸码（600519）
		i := 0
		for i < len(s) && (s[i] >= 'a' && s[i] <= 'z' || s[i] >= 'A' && s[i] <= 'Z') {
			i++
		}
		prefix, rest := s[:i], s[i:]
		if !isSixDigits(rest) {
			return "", "", false
		}
		if prefix != "" && !isExchange(prefix) {
			return "", "", false
		}
		digits, exchPart = rest, prefix
	}

	exchange = strings.ToLower(exchPart)
	if exchange == "" {
		exchange = inferExchange(digits)
	}
	return digits, exchange, true
}

// NormalizeStockCode 将任意格式规范为内部标准格式（小写前缀 + 6 位数字，如 sh600519）。
// 无法识别的输入原样返回。
func NormalizeStockCode(input string) string {
	digits, exchange, ok := ParseStockCode(input)
	if !ok || exchange == "" {
		return input
	}
	return exchange + digits
}

// StockCodeCandidates 返回该代码在 kline_bars 中可能出现的存储格式候选列表（去重、按优先级排序）。
// 用于缓存容错查询：历史数据可能同时存在裸码与前缀格式两种存量记录，
// 逐个候选做精确等值查询（仍可命中 idx_kline_code_period_date_adj 复合索引）。
func StockCodeCandidates(input string) []string {
	var candidates []string
	seen := make(map[string]struct{})
	add := func(c string) {
		if c == "" {
			return
		}
		if _, dup := seen[c]; dup {
			return
		}
		seen[c] = struct{}{}
		candidates = append(candidates, c)
	}

	add(input) // 原始输入优先，保持既有行为
	digits, exchange, ok := ParseStockCode(input)
	if ok {
		if exchange != "" {
			add(exchange + digits)                      // 标准前缀格式 sh600519
			add(digits + "." + strings.ToUpper(exchange)) // ts_code 格式 600519.SH
		}
		add(digits) // 裸码 600519（种子数据格式）
	}
	return candidates
}
