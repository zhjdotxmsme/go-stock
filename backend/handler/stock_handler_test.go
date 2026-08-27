package handler

import (
	"testing"

	datasource "go-stock/backend/data/datasource"
)

// Behavior-contract tests for the StockHandler real-time price migration:
// the handler previously parsed data.StockInfo (string fields) inline; it now
// resolves from datasource.SnapshotData via Router.GetSnapshot. These tests
// pin the price fallback chain so the migration cannot silently change it.

func TestResolveSnapshotPrice_FallbackChain(t *testing.T) {
	cases := []struct {
		name string
		snap *datasource.SnapshotData
		want float64
	}{
		{"current price wins", &datasource.SnapshotData{Price: 12.34, A1P: 12.33, B1P: 12.32, PreClose: 12.00}, 12.34},
		{"suspended: ask1 fallback", &datasource.SnapshotData{Price: 0, A1P: 12.33, B1P: 12.32, PreClose: 12.00}, 12.33},
		{"no quotes: bid1 fallback", &datasource.SnapshotData{Price: 0, A1P: 0, B1P: 12.32, PreClose: 12.00}, 12.32},
		{"halted: preclose fallback", &datasource.SnapshotData{Price: 0, A1P: 0, B1P: 0, PreClose: 12.00}, 12.00},
		{"all zero", &datasource.SnapshotData{}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := resolveSnapshotPrice(tc.snap)
			if got != tc.want {
				t.Fatalf("price = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestResolveSnapshotPrice_NilSnapshot(t *testing.T) {
	price, name := resolveSnapshotPrice(nil)
	if price != 0 || name != "" {
		t.Fatalf("expected zero value, got price=%v name=%q", price, name)
	}
}

// TestResolveSnapshotPrice_MatchesLegacyContract replays the legacy inline
// logic (convertor.ToFloat over StockInfo string fields) on the same numbers
// and asserts the new resolver produces identical results — the observable
// contract of GetStockRealTimePrice is unchanged by the migration.
func TestResolveSnapshotPrice_MatchesLegacyContract(t *testing.T) {
	legacyResolve := func(price, a1p, b1p, preClose float64) float64 {
		p := price
		if p == 0 {
			p = a1p
		}
		if p == 0 {
			p = b1p
		}
		if p == 0 {
			p = preClose
		}
		return p
	}
	snap := &datasource.SnapshotData{Price: 0, A1P: 0, B1P: 9.87, PreClose: 9.90}
	got, _ := resolveSnapshotPrice(snap)
	if want := legacyResolve(snap.Price, snap.A1P, snap.B1P, snap.PreClose); got != want {
		t.Fatalf("resolver %v != legacy %v", got, want)
	}
}
