package data

import (
	"os"
	"testing"
)

// TestSignalBoard_Live 全品种信号板（live）：所有可交易品种都有行，综合分与结论一致。
func TestSignalBoard_Live(t *testing.T) {
	if os.Getenv("GOSTOCK_LIVE") == "off" {
		t.Skip("live off")
	}
	board, err := GetCommoditySignalBoard()
	if err != nil {
		t.Fatalf("board error: %v", err)
	}
	if board == nil || len(board.Rows) == 0 {
		t.Fatalf("empty board")
	}

	tradable := TradableCommodities()
	if len(board.Rows) != len(tradable) {
		t.Errorf("rows=%d, expected %d tradable", len(board.Rows), len(tradable))
	}

	seen := map[string]bool{}
	bull, neutral, bear := 0, 0, 0
	for _, r := range board.Rows {
		if seen[r.Code] {
			t.Errorf("dup %s", r.Code)
		}
		seen[r.Code] = true
		switch r.Verdict {
		case "偏多":
			bull++
			if r.Score < 0.25 {
				t.Errorf("%s verdict 偏多 but score %.3f", r.Code, r.Score)
			}
		case "偏空":
			bear++
			if r.Score > -0.25 {
				t.Errorf("%s verdict 偏空 but score %.3f", r.Code, r.Score)
			}
		default:
			neutral++
			if !r.Failed && (r.Score < -0.25 || r.Score > 0.25) {
				t.Errorf("%s verdict %s but score %.3f", r.Code, r.Verdict, r.Score)
			}
		}
		t.Logf("%s %-22s 价=%-10.2f score=%+.3f %s T=%s M=%s B=%s C=%s O=%s",
			r.Market, r.Code, r.Price, r.Score, r.Verdict, r.Trend, r.Momentum, r.Breakout, r.Carry, r.OI)
	}
	t.Logf("stats: bull=%d neutral=%d bear=%d  fetchedAt=%s", board.Stats.Bull, board.Stats.Neutral, board.Stats.Bear, board.FetchedAt)

	// 期货标的必须有 trend+momentum+breakout（K 线可得时）
	for _, r := range board.Rows {
		if r.Code == "AU" || r.Code == "AG" || r.Code == "SC" {
			if r.Trend == "" || r.Momentum == "" || r.Breakout == "" {
				t.Errorf("%s missing trend/momentum/breakout (T=%q M=%q B=%q failed=%v)", r.Code, r.Trend, r.Momentum, r.Breakout, r.Failed)
			}
		}
	}
}
