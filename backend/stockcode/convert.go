// Package stockcode provides stock code normalization and conversion
// for all supported formats (Tushare, EastMoney, Sina, TDX, Tencent, Yahoo).
//
// Internal canonical format:
//   - A-share (Shanghai): sh600519
//   - A-share (Shenzhen): sz000001
//   - A-share (Beijing):  bj430047
//   - Hong Kong:          hk00700
//   - US:                 usAAPL
package stockcode

// ToTushare converts any recognized format to Tushare ts_code format: 600519.SH, 00700.HK, AAPL (US bare).
//
// Supported inputs: sh600519, SH600519, 600519.SH, 1.600519, 600519, hk00700, usAAPL, gb_AAPL, etc.
func ToTushare(code string) string {
	prefix, pure := splitCode(code)
	if prefix == "" {
		// No prefix — pure digit heuristic for A-share, otherwise return as-is (could be US ticker)
		prefix = guessAPrefix(pure)
		if prefix == "" {
			return code // bare US ticker like AAPL
		}
	}
	return toTushareFromInternal(prefix, pure)
}

// ToEastMoney converts any format to EastMoney K-line secid: 1.600519, 0.000001, 128.00700.
//
// For US stocks returns empty string (EastMoney doesn't support US stocks via push2his).
func ToEastMoney(code string) string {
	prefix, pure := splitCode(code)
	if prefix == "" {
		prefix = guessAPrefix(pure)
	}

	switch prefix {
	case "sh":
		return "1." + pure
	case "sz":
		return "0." + pure
	case "bj":
		return "0." + pure // Beijing uses SZ market in EastMoney K-line
	case "hk":
		return "128." + pure
	default:
		return "" // US stocks not supported on EastMoney K-line
	}
}

// ToSina converts any format to Sina K-line symbol: sh600519, sz000001, hk00700, usAAPL.
//
// For Beijing stocks, uses "bj" prefix (K-line API variant, not "sb" which is hq-only).
func ToSina(code string) string {
	return Normalize(code)
}

// ToTDX converts any format to (market_uint8, pure_code) for TDX protocol.
//
// Market IDs: SH=1, SZ=0, BJ=83, HK=3, US=2.
func ToTDX(code string) (market uint8, symbol string) {
	prefix, pure := splitCode(code)
	if prefix == "" {
		prefix = guessAPrefix(pure)
	}

	switch prefix {
	case "sh":
		return 1, pure
	case "sz":
		return 0, pure
	case "bj":
		return 83, pure
	case "hk":
		return 3, pure
	case "us":
		return 2, pure
	default:
		return 1, pure // fallback to SH
	}
}

// ToTencent converts any format to Tencent K-line symbol: sh600519, sz000001, hk00700.
//
// For US stocks, Tencent uses a different format (usAAPL.OQ for minute data).
// This function returns the basic symbol without the .OQ/.NQ suffix.
func ToTencent(code string) string {
	return Normalize(code)
}

// ToTusharePure strips prefix/suffix and returns the pure code for Tushare API calls.
//
// Examples: usAAPL → AAPL, gb_AAPL → AAPL, sh600519 → 600519.
func ToTusharePure(code string) string {
	prefix, pure := splitCode(code)
	if prefix == "us" || prefix == "gb" {
		return pure // US ticker is already pure
	}
	return pure
}

// --- internal helpers ---

func toTushareFromInternal(prefix, pure string) string {
	switch prefix {
	case "sh":
		return pure + ".SH"
	case "sz":
		return pure + ".SZ"
	case "bj":
		return pure + ".BJ"
	case "hk":
		return pure + ".HK"
	case "us", "gb":
		return pure // US stocks use bare ticker in Tushare
	default:
		return pure
	}
}
