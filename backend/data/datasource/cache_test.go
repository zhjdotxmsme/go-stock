package datasource

import (
	"context"
	"testing"
	"time"
)

// TestCacheLayerTypedRoundTrip verifies the concrete-type cache contract:
// Set stores a typed value and GetInto decodes it back into the same type.
// This is the regression test for the earlier bug where Get decoded JSON into
// interface{} (maps/slices) and the router's type assertions never matched,
// silently disabling every cache TTL.
func TestCacheLayerTypedRoundTrip(t *testing.T) {
	cache := NewCacheLayer(1) // 1MB is enough
	ctx := context.Background()

	t.Run("pointer value round trip", func(t *testing.T) {
		in := &QuoteData{Code: "sh600000", Price: 12.34}
		if err := cache.Set(ctx, "quote:sh600000", string(DataTypeQuote), in, 60*time.Second); err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		out := &QuoteData{}
		if !cache.GetInto(ctx, "quote:sh600000", out) {
			t.Fatal("expected cache hit")
		}
		if out.Code != "sh600000" || out.Price != 12.34 {
			t.Fatalf("round trip mismatch: got %+v", out)
		}
	})

	t.Run("slice value round trip", func(t *testing.T) {
		in := []NewsItem{{Title: "a"}, {Title: "b"}}
		if err := cache.Set(ctx, "news:all:2", string(DataTypeNews), in, 60*time.Second); err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		out := []NewsItem{}
		if !cache.GetInto(ctx, "news:all:2", &out) {
			t.Fatal("expected cache hit")
		}
		if len(out) != 2 || out[0].Title != "a" || out[1].Title != "b" {
			t.Fatalf("round trip mismatch: got %+v", out)
		}
	})

	t.Run("miss returns false", func(t *testing.T) {
		out := &QuoteData{}
		if cache.GetInto(ctx, "quote:missing", out) {
			t.Fatal("expected cache miss")
		}
	})

	t.Run("expired L2 entry is not served", func(t *testing.T) {
		// Without a DB (db.Dao == nil) only L1 serves; TTL expiry in freecache
		// is covered implicitly by the 1-second TTL below.
		in := &SectorData{Code: "bk0001"}
		if err := cache.Set(ctx, "sector:bk0001", string(DataTypeSector), in, time.Second); err != nil {
			t.Fatalf("Set failed: %v", err)
		}
		time.Sleep(1100 * time.Millisecond)

		out := &SectorData{}
		if cache.GetInto(ctx, "sector:bk0001", out) {
			t.Fatal("expected expired entry to miss")
		}
	})
}
