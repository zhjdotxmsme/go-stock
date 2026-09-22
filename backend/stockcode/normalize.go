package stockcode

import (
	"strings"
	"unicode"
)

// Normalize converts any supported stock code input to the internal canonical format.
//
// Internal standard:
//   - A-share (Shanghai): sh600519
//   - A-share (Shenzhen): sz000001
//   - A-share (Beijing):  bj430047
//   - Hong Kong:          hk00700
//   - US:                 usAAPL
//
// Supported input formats:
//   - Pure digits: 600519 → sh600519, 00700 → hk00700
//   - Tushare ts_code: 600519.SH → sh600519, 00700.HK → hk00700
//   - EastMoney secid: 1.600519 → sh600519, 0.430047 → bj430047（按首码分流）,
//     128.00700 / 116.00700 → hk00700, 105.AAPL → usAAPL
//   - Sina/Tencent prefix: sh600519 → sh600519 (no-op)
//   - US Sina variant: gb_AAPL → usAAPL
//   - US with Tushare suffix: AAPL.US → usAAPL
//   - Already normalized: sh600519 → sh600519 (no-op)
func Normalize(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}

	// Already in internal format (lowercase prefix + code, no dot, no underscore)
	if isInternalFormat(code) {
		return code
	}

	prefix, pure := splitCode(code)

	// If no prefix could be extracted, use heuristics
	if prefix == "" {
		prefix = guessAPrefix(pure)
		if prefix == "" {
			return code // Unknown format, return as-is
		}
	}

	return prefix + pure
}

// ParseStockCode decomposes any stock code into (market_prefix, pure_code).
//
// Market prefixes: "sh", "sz", "bj", "hk", "us".
// Pure code is the code portion without prefix or suffix.
//
// Examples:
//
//	ParseStockCode("sh600519")     → ("sh", "600519")
//	ParseStockCode("600519.SH")    → ("sh", "600519")
//	ParseStockCode("1.600519")     → ("sh", "600519")
//	ParseStockCode("00700.HK")     → ("hk", "00700")
//	ParseStockCode("hk00700")      → ("hk", "00700")
//	ParseStockCode("usAAPL")       → ("us", "AAPL")
//	ParseStockCode("gb_AAPL")      → ("us", "AAPL")
func ParseStockCode(code string) (prefix, pure string) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", ""
	}

	if isInternalFormat(code) {
		return extractPrefix(code), extractPureCode(code)
	}

	return splitCode(code)
}

// Market returns the market identifier: "SH", "SZ", "BJ", "HK", "US", or "".
func Market(code string) string {
	prefix, pure := ParseStockCode(code)
	if prefix == "" {
		prefix = guessAPrefix(pure)
	}
	switch prefix {
	case "sh":
		return "SH"
	case "sz":
		return "SZ"
	case "bj":
		return "BJ"
	case "hk":
		return "HK"
	case "us":
		return "US"
	default:
		return ""
	}
}

// IsA股 returns true for Shanghai, Shenzhen, and Beijing stock codes.
func IsA股(code string) bool {
	m := Market(code)
	return m == "SH" || m == "SZ" || m == "BJ"
}

// IsHK returns true for Hong Kong stock codes.
func IsHK(code string) bool {
	return Market(code) == "HK"
}

// IsUS returns true for US stock codes.
func IsUS(code string) bool {
	return Market(code) == "US"
}

// PureCode returns just the code portion (digits for A-share/HK, ticker for US).
func PureCode(code string) string {
	_, pure := ParseStockCode(code)
	return pure
}

// StockCodeCandidates returns the normalized code plus legacy format candidates
// for database queries where historical data may use mixed formats.
//
// Example: input "sh600519" → ["sh600519", "600519.SH", "600519"]
func StockCodeCandidates(code string) []string {
	normalized := Normalize(code)
	if normalized == "" {
		return nil
	}

	prefix, pure := ParseStockCode(normalized)
	candidates := make([]string, 0, 4)
	candidates = append(candidates, normalized)       // canonical: sh600519
	candidates = append(candidates, ToTushare(code))   // tushare:  600519.SH
	if prefix != "us" {
		candidates = append(candidates, pure)           // bare: 600519（种子脚本写入格式）
	} else {
		candidates = append(candidates, "gb_"+pure)     // legacy sina US variant
	}

	return candidates
}

// --- internal helpers ---

// splitCode extracts the prefix and pure code from any known format.
func splitCode(code string) (prefix, pure string) {
	// Check for dot
	dotIdx := strings.Index(code, ".")
	if dotIdx >= 0 {
		beforeDot := code[:dotIdx]
		afterDot := code[dotIdx+1:]

		// EastMoney secid: 1.600519, 0.000001, 128.00700
		// Characteristic: beforeDot is a short number (1, 0, 128), afterDot is the 6-digit
		if isAllDigits(beforeDot) && len(beforeDot) <= 3 {
			if isAllDigits(afterDot) {
				return emSecidToPrefix(beforeDot, afterDot), afterDot
			}
			// 数字市场号 + 字母代码：东财美股 secid（105.AAPL / 106.TSLA / 107 等）
			switch beforeDot {
			case "105", "106", "107", "110", "111", "112", "113", "114", "115":
				return "us", afterDot
			}
		}

		// Tushare format: 600519.SH, 00700.HK, AAPL.US
		// afterDot is 2-3 uppercase letters
		prefix := tushareSuffixToInternal(afterDot)
		if prefix != "" {
			return prefix, beforeDot
		}
	}

	// gb_AAPL format (Sina US variant)
	if len(code) >= 6 && strings.EqualFold(code[:3], "gb_") {
		return "us", code[3:]
	}

	// Prefix format: sh600519, hk00700, usAAPL (case-insensitive)
	if len(code) >= 3 {
		prefixLower := strings.ToLower(code[:2])
		for _, p := range []string{"sh", "sz", "bj", "hk", "us"} {
			if prefixLower == p {
				return p, code[2:]
			}
		}
	}

	// No prefix — pure code
	return "", code
}

// isAllDigits returns true if all characters in s are digits.
func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

// isInternalFormat checks if the code is already in canonical form:
// lowercase prefix (2 chars) + code, no dots, no underscores.
func isInternalFormat(code string) bool {
	if len(code) < 3 {
		return false
	}

	// Check for gb_ prefix — not internal (should be normalized to us)
	if len(code) >= 3 && strings.EqualFold(code[:3], "gb_") {
		return false
	}

	// Must start with known 2-char lowercase prefix (exact match)
	prefix := code[:2]
	for _, p := range []string{"sh", "sz", "bj", "hk", "us"} {
		if prefix == p {
			// After prefix, must have at least 1 char
			rest := code[2:]
			if len(rest) == 0 {
				return false
			}
			// Ensure rest starts with alphanumeric
			r := rune(rest[0])
			return unicode.IsLetter(r) || unicode.IsDigit(r)
		}
	}

	return false
}

// extractPrefix returns the lowercase 2-char prefix from an internal format code.
func extractPrefix(code string) string {
	return strings.ToLower(code[:2])
}

// extractPureCode returns the code portion after the 2-char prefix.
func extractPureCode(code string) string {
	return code[2:]
}

// guessAPrefix uses first-digit heuristic to determine the market for a pure digit code.
func guessAPrefix(code string) string {
	if len(code) == 0 {
		return ""
	}

	// First check if it looks like a US ticker (contains letters)
	for _, r := range code {
		if unicode.IsLetter(r) {
			return "us"
		}
	}

	// 港股为 5 位数字（含前导零，如 00700）；A 股/基金均为 6 位
	if len(code) == 5 {
		return "hk"
	}
	if len(code) != 6 {
		return "" // too short or unknown length, don't guess
	}

	switch code[0:1] {
	case "6":
		return "sh"
	case "5": // 沪市基金：50/51/52/56/58（含 588 科创ETF）
		return "sh"
	case "0", "3":
		return "sz"
	case "2": // 深市 B 股 200xxx
		return "sz"
	case "1": // 深市基金：15x ETF / 16x LOF / 18x
		if len(code) >= 2 && (code[1] == '5' || code[1] == '6' || code[1] == '8') {
			return "sz"
		}
		return "" // Unknown, don't guess
	case "4", "8": // 43xxx, 83xxx are Beijing Stock Exchange
		return "bj"
	case "9":
		// 920xxx 为北交所新号段；900xxx 为沪市 B 股
		if strings.HasPrefix(code, "920") {
			return "bj"
		}
		return "sh"
	default:
		return "" // Unknown, don't guess
	}
}

// tushareSuffixToInternal maps Tushare exchange suffixes to internal lowercase prefix.
func tushareSuffixToInternal(suffix string) string {
	switch strings.ToUpper(suffix) {
	case "SH":
		return "sh"
	case "SZ":
		return "sz"
	case "BJ":
		return "bj"
	case "HK":
		return "hk"
	case "US":
		return "us"
	default:
		return ""
	}
}

// emSecidToPrefix maps EastMoney secid market IDs to internal prefix.
// pure 用于 "0" 市场的深/北分流（东财 push2 体系中北交所与深市共用 market 0）。
func emSecidToPrefix(marketID, pure string) string {
	switch marketID {
	case "1":
		return "sh" // Shanghai
	case "0":
		// 深市与北交所共用 market 0，按代码首部分流：4/8/920 开头为北交所
		if strings.HasPrefix(pure, "4") || strings.HasPrefix(pure, "8") || strings.HasPrefix(pure, "920") {
			return "bj"
		}
		return "sz"
	case "100", "116", "124", "128":
		return "hk" // Hong Kong（100=指数，116/124/128 为港股行情）
	case "90":
		return "" // Sector (板块), not a stock
	default:
		return ""
	}
}
