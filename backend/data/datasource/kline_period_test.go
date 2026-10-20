package datasource

import "testing"

func TestCanonicalKLinePeriod(t *testing.T) {
	for in, want := range map[string]string{
		"101": "day", "day": "day", "102": "week", "103": "month", "hour": "hour",
	} {
		if got := CanonicalKLinePeriod(in); got != want {
			t.Fatalf("CanonicalKLinePeriod(%q)=%q want %q", in, got, want)
		}
	}
}

func TestKLinePeriodCandidates(t *testing.T) {
	got := KLinePeriodCandidates("day")
	if len(got) != 2 || got[0] != "day" || got[1] != "101" {
		t.Fatalf("day candidates: %v", got)
	}
}
