package fallback

import (
	"context"
	"errors"
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/data/datasource"
)

// TestSnapshotProvider_LiveComparison is the migration before/after check for
// the StockHandler real-time price migration: it fetches the same stock
// through the legacy direct API and through the new provider, then asserts
// the observable fields (name, price) agree. Network-dependent; skipped in
// -short mode.
func TestSnapshotProvider_LiveComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("network test skipped in short mode")
	}

	codes := []string{"sh600000", "sz000001"}
	for _, code := range codes {
		t.Run(code, func(t *testing.T) {
			// Legacy path: direct data-layer snapshot.
			legacy, err := data.NewStockDataApi().GetStockCodeRealTimeData(code)
			if err != nil || legacy == nil || len(*legacy) == 0 {
				t.Fatalf("legacy path failed: %v", err)
			}
			old := (*legacy)[0]

			// New path: provider backing Router.GetSnapshot.
			p := &TencentSnapshotProvider{}
			if !p.Available(context.Background()) {
				t.Fatal("provider unexpectedly unavailable")
			}
			snap, err := p.GetSnapshot(context.Background(), code)
			if err != nil {
				t.Fatalf("provider failed: %v", err)
			}

			// Name must match exactly (same upstream field).
			if snap.Name != old.Name {
				t.Errorf("name mismatch: provider %q vs legacy %q", snap.Name, old.Name)
			}

			// Price: provider parses to float64, legacy keeps strings; the
			// values come from the same upstream response seconds apart, so
			// they should agree within a small tolerance.
			diff := snap.Price - toFloat64(old.Price)
			if diff < -0.011 || diff > 0.011 {
				t.Errorf("price mismatch: provider %.4f vs legacy %.4f (diff %.4f)", snap.Price, toFloat64(old.Price), diff)
			}
			t.Logf("%s: name=%s provider_price=%.2f legacy_price=%.2f preclose=%.2f a1p=%.2f", code, snap.Name, snap.Price, toFloat64(old.Price), snap.PreClose, snap.A1P)
		})
	}
}

// TestSnapshotProvider_UnsupportedMarket pins the ErrUnsupported contract for
// markets the snapshot chain does not cover.
func TestSnapshotProvider_UnsupportedMarket(t *testing.T) {
	p := &TencentSnapshotProvider{}
	_, err := p.GetSnapshot(context.Background(), "usAAPL")
	if err == nil {
		t.Fatal("expected error for unsupported market")
	}
	if !errors.Is(err, datasource.ErrUnsupported) {
		t.Fatalf("expected ErrUnsupported, got %v", err)
	}
}
