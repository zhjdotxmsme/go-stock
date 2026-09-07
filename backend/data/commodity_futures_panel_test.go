package data

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go-stock/backend/internal/port/datasource"
)

// 实时接口测试：不可达时 Skip（沿用仓库既有 skip 语义，断网不挂）。
// 数据源映射已由 2026-09-07 本机采样交叉验证：
//   东财 f108(持仓) == Sina parts[13]；东财 f5(量) == Sina parts[14]（AU/AG 双品种一致）。

func TestFuturesPanel_Live(t *testing.T) {
	// 重置缓存，强制实拉
	panelCache.Lock()
	panelCache.v = nil
	panelCache.Unlock()

	panel, err := GetCommodityFuturesPanel()
	if err != nil {
		t.Fatalf("panel: %v", err)
	}
	if len(panel.Contracts) != 3 {
		t.Fatalf("want 3 contracts, got %d", len(panel.Contracts))
	}
	oiOK := 0
	for _, c := range panel.Contracts {
		if c.OpenInterest > 0 {
			oiOK++
		}
		t.Logf("%s 价=%v 涨跌%v 持仓=%v 增仓=%v carry%v=%v ATR%v=%v 季节%v=%v",
			c.Code, c.Price, c.ChangePct, c.OpenInterest, c.OIChange, c.CarryPct, c.CarryOK, c.ATRPct, c.ATROK, c.Season5y, c.SeasonOK)
	}
	if oiOK < 1 {
		t.Fatalf("at least one contract must have OI (data sources all down?)")
	}
	if panel.GoldSilverRatio == nil {
		hasFailed := false
		for _, f := range panel.Failed {
			if f == "gold_silver_ratio" {
				hasFailed = true
			}
		}
		if !hasFailed {
			t.Fatalf("goldSilverRatio nil but not marked failed")
		}
		t.Logf("gold/silver ratio unavailable (marked failed): %v", panel.Failed)
	} else {
		t.Logf("金银比=%v 分位=%v", *panel.GoldSilverRatio, panel.GoldSilverPercentile)
	}
	if panel.COT == nil {
		t.Logf("COT unavailable (expected when cftc.gov unreachable): failed=%v", panel.Failed)
	} else {
		for _, c := range panel.COT {
			t.Logf("COT %s 净=%v 占比%v date=%s", c.Product, c.Net, c.Percent, c.Date)
		}
	}
}

func TestSinaFuturesOIVsBoard_Live(t *testing.T) {
	oi, volume, found := fetchSinaFuturesOI("AU")
	if !found {
		t.Skipf("Sina nf_AU0 unreachable — skip cross-check")
	}
	_, _, boardOK := findBoardContract("m:113", "au", 3)
	if !boardOK {
		t.Skipf("EastMoney board unreachable — skip cross-check")
	}
	// 东财主力持仓（aum）
	near, _, ok := findBoardContract("m:113", "au", 3)
	if !ok {
		t.Skipf("EastMoney board has no au rows — skip")
	}
	if near.OpenInt <= 0 {
		t.Fatalf("board au OI empty")
	}
	diff := (oi - near.OpenInt) / near.OpenInt
	if diff < -0.05 || diff > 0.05 {
		t.Fatalf("Sina OI=%v vs board OI=%v diff=%.2f%% (should agree within 5%%)", oi, near.OpenInt, diff*100)
	}
	t.Logf("Sina OI=%v 量=%v | board OI=%v 价=%v — 一致", oi, volume, near.OpenInt, near.Price)
}

func TestUpdateOISnapshot_DeltaSemantics(t *testing.T) {
	dir := t.TempDir()
	old := oiSnapshotFile
	oiSnapshotFile = filepath.Join(dir, "oi_snapshots.json")
	defer func() { oiSnapshotFile = old }()

	// 首日：无快照 → 增仓 nil
	d1 := updateOISnapshot("AU", "2026-09-06", 150000)
	if d1 != nil {
		t.Fatalf("first day must have nil delta, got %v", *d1)
	}

	// 次日：与"上一交易日最后快照"差值
	d2 := updateOISnapshot("AU", "2026-09-07", 154549)
	if d2 == nil {
		t.Fatalf("second day must have delta")
	}
	if *d2 != 4549 {
		t.Fatalf("want +4549, got %v", *d2)
	}

	// 同日第二次抓取：与当首次快照差值
	d3 := updateOISnapshot("AU", "2026-09-07", 155000)
	if d3 == nil {
		t.Fatalf("same-day second fetch should have delta vs first capture")
	}
	if *d3 != 451 {
		t.Fatalf("want +451 (vs first capture 154549), got %v", *d3)
	}

	// 文件确实落盘
	if _, err := os.Stat(oiSnapshotFile); err != nil {
		t.Fatalf("snapshot file missing: %v", err)
	}
}

func TestAtrPct_Stub(t *testing.T) {
	// 10 根定值 K：high=105, low=95, close=100 → TR=10, ATR=10, ATR%=10
	bars := make([]HighLowClose, 10)
	for i := range bars {
		bars[i] = HighLowClose{High: 105, Low: 95, Close: 100}
	}
	v, ok := atrPct(bars, 3)
	if !ok || v != 10 {
		t.Fatalf("want ATR%%=10, got %v ok=%v", v, ok)
	}
	if _, ok := atrPct(bars[:2], 3); ok {
		t.Fatalf("2 bars insufficient for ATR(3)")
	}
}

func TestMonthAvgs_Stable(t *testing.T) {
	// 构造 6 年"每月固定 +1% 月末收盘"的日 K：每月取 12 个交易日。
	now := time.Now()
	start := now.AddDate(-6, 0, 0)
	var bars []datasource.KLineBar
	// 简化：逐月加一根"月初 100 → 月末 101"的两根 K 不足以成月收益……
	// 用连续上涨序列即可验证函数不 panic、分月可得（具体数值依赖采样）
	base := 100.0
	for i := 0; i < 1400; i++ {
		d := start.AddDate(0, 0, i)
		base *= 1.0005
		bars = append(bars, datasource.KLineBar{
			Time:  d,
			Open:  base, High: base * 1.001, Low: base * 0.999, Close: base,
		})
	}
	avgs, ok := monthAvgs(bars, 5)
	if !ok {
		t.Fatalf("monthAvgs should resolve from 6y data")
	}
	if len(avgs) < 6 {
		t.Fatalf("want ≥6 month buckets, got %d", len(avgs))
	}
	for m, v := range avgs {
		if v <= 0 {
			t.Fatalf("month %d: uptrend sample must yield positive avg %v", m, v)
		}
	}
}
