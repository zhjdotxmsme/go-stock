package fallback

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"go-stock/backend/data/datasource"
)

// 手动核对生产失败样本。KLINE_LIVE=1 才打外网。
func TestLiveAnchorProviders(t *testing.T) {
	if os.Getenv("KLINE_LIVE") != "1" {
		t.Skip("set KLINE_LIVE=1")
	}
	ctx := context.Background()
	tx := &TencentKLineProvider{}
	em := &EastMoneyKLineProvider{}

	start := time.Now()
	k, err := tx.GetKLineBefore(ctx, "sh600519", "day", 5, "2022-04-19")
	if err != nil || k == nil || len(k.Bars) == 0 {
		t.Fatalf("tencent 600519: %v bars=%v elapsed=%s", err, k, time.Since(start))
	}
	last := k.Bars[len(k.Bars)-1].Time.Format("2006-01-02")
	if last > "2022-04-19" {
		t.Fatalf("tencent 600519 last bar %s is after anchor", last)
	}
	t.Logf("tencent 600519 bars=%d last=%s elapsed=%s", len(k.Bars), last, time.Since(start))

	start = time.Now()
	_, err = tx.GetKLineBefore(ctx, "sz301248", "day", 30, "2022-04-19")
	if !errors.Is(err, datasource.ErrNoBarsBefore) {
		t.Fatalf("tencent 301248 before listing: %v elapsed=%s", err, time.Since(start))
	}
	t.Logf("tencent 301248 empty elapsed=%s", time.Since(start))

	start = time.Now()
	k, err = em.GetKLineBefore(ctx, "sh600519", "day", 5, "2022-04-19")
	if err != nil || k == nil || len(k.Bars) == 0 {
		t.Fatalf("eastmoney 600519: %v elapsed=%s", err, time.Since(start))
	}
	last = k.Bars[len(k.Bars)-1].Time.Format("2006-01-02")
	if last > "2022-04-19" {
		t.Fatalf("eastmoney 600519 last bar %s is after anchor", last)
	}
	t.Logf("eastmoney 600519 bars=%d last=%s elapsed=%s", len(k.Bars), last, time.Since(start))

	start = time.Now()
	_, err = em.GetKLineBefore(ctx, "sz301248", "day", 30, "2022-04-19")
	if !errors.Is(err, datasource.ErrNoBarsBefore) {
		t.Fatalf("eastmoney 301248 before listing: %v elapsed=%s", err, time.Since(start))
	}
	if time.Since(start) > 8*time.Second {
		t.Fatalf("pre-listing anchor took %s, tail fallback is still in the path", time.Since(start))
	}
	t.Logf("eastmoney 301248 empty elapsed=%s", time.Since(start))
}
