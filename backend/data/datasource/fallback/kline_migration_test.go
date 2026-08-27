package fallback

import (
	"context"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/data/datasource"
)

// TestKLineMigration_LiveComparison is the group-2 before/after check: the
// same stock's day kline fetched through the legacy serial chain and through
// the Router must agree on the latest bar's date and close. Upstreams differ
// (legacy starts at tdx-mac, the Router at mootdx_kline/tencent), so values
// are compared with a small tolerance. Network-dependent; skipped in -short.
func TestKLineMigration_LiveComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("network test skipped in short mode")
	}

	const code = "sh600000"
	legacy := data.FetchKLineWithFallback(code, "", "101", 30, "")
	if legacy == nil || legacy.Data == nil || len(*legacy.Data) == 0 {
		t.Fatal("legacy chain returned no data")
	}
	lastLegacy := (*legacy.Data)[len(*legacy.Data)-1]

	// Assemble the same chain main.go registers; the test binary starts with
	// a bare Router singleton.
	router := datasource.GetRouter()
	RegisterKLineChain(router)
	RegisterFreeDataSources(router)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	k, err := router.GetKLine(ctx, code, "day", 30)
	if err != nil || k == nil || len(k.Bars) == 0 {
		t.Fatalf("router returned no data: %v", err)
	}
	lastNew := k.Bars[len(k.Bars)-1]

	if got := lastNew.Time.Format("2006-01-02"); got != lastLegacy.Day {
		t.Errorf("latest bar date: router %q vs legacy %q", got, lastLegacy.Day)
	}
	legacyClose := parseFloat64(lastLegacy.Close)
	diff := lastNew.Close - legacyClose
	if diff < -0.011 || diff > 0.011 {
		t.Errorf("latest close: router %.3f vs legacy %.3f", lastNew.Close, legacyClose)
	}
	t.Logf("%s: router source=%s close=%.2f (%s), legacy source=%s close=%s (%s)",
		code, k.Source, lastNew.Close, lastNew.Time.Format("2006-01-02"), legacy.Source, lastLegacy.Close, lastLegacy.Day)
}
