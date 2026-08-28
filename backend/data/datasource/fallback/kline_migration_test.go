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

// TestToTencentCode_HKPassThrough pins the HK/US native-symbol mapping that
// lets TencentKLineProvider serve HK klines (group-3 migration enabler).
func TestToTencentCode_HKPassThrough(t *testing.T) {
	cases := map[string]string{
		"hk00700":  "hk00700",
		"HK00700":  "hk00700",
		"sh600000": "sh600000",
		"SZ000001": "sz000001",
		"600519":   "sh600519",
		"000001":   "sz000001",
	}
	for in, want := range cases {
		if got := toTencentCode(in); got != want {
			t.Errorf("toTencentCode(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestKLineMigration_HKLiveComparison is the group-3 before/after check for
// HK day klines: legacy GetHK_KLineData and the Router chain must agree on
// the latest bar. Both hit tencent fqkline upstream. Network-dependent.
func TestKLineMigration_HKLiveComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("network test skipped in short mode")
	}

	const code = "hk00700"
	legacy := data.NewStockDataApi().GetHK_KLineData(code, "day", 20)
	if legacy == nil || len(*legacy) == 0 {
		t.Fatal("legacy HK kline returned no data")
	}
	lastLegacy := (*legacy)[len(*legacy)-1]

	router := datasource.GetRouter()
	RegisterKLineChain(router)
	RegisterFreeDataSources(router)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	k, err := router.GetKLine(ctx, code, "day", 20)
	if err != nil || k == nil || len(k.Bars) == 0 {
		t.Fatalf("router HK kline failed: %v", err)
	}
	lastNew := k.Bars[len(k.Bars)-1]

	if got := lastNew.Time.Format("2006-01-02"); got != lastLegacy.Day {
		t.Errorf("latest bar date: router %q vs legacy %q", got, lastLegacy.Day)
	}
	diff := lastNew.Close - parseFloat64(lastLegacy.Close)
	if diff < -0.011 || diff > 0.011 {
		t.Errorf("latest close: router %.3f vs legacy %.3f", lastNew.Close, parseFloat64(lastLegacy.Close))
	}
	t.Logf("%s: router(%s) close=%.2f %s | legacy close=%s %s",
		code, k.Source, lastNew.Close, lastNew.Time.Format("2006-01-02"), lastLegacy.Close, lastLegacy.Day)
}
