package datasource

import "strings"

// CanonicalKLinePeriod maps API/router period aliases to the SQLite storage key.
func CanonicalKLinePeriod(period string) string {
	switch strings.ToLower(strings.TrimSpace(period)) {
	case "101", "daily", "d":
		return "day"
	case "102", "weekly", "w":
		return "week"
	case "103", "monthly", "m":
		return "month"
	default:
		return strings.TrimSpace(period)
	}
}

// KLinePeriodCandidates returns DB period values to match for a logical period.
func KLinePeriodCandidates(period string) []string {
	p := CanonicalKLinePeriod(period)
	switch p {
	case "day":
		return []string{"day", "101"}
	case "week":
		return []string{"week", "102"}
	case "month":
		return []string{"month", "103"}
	default:
		if p == "" {
			return []string{"day", "101"}
		}
		return []string{p}
	}
}
