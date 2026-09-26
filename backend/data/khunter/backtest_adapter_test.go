package khunter

import "testing"

func TestAggregateStats(t *testing.T) {
	// 3 笔：+10%、-5%、+20% → 胜率 2/3，AvgWin=0.15，AvgLoss=0.05，盈亏比=3
	results := []float64{0.10, -0.05, 0.20}
	st := aggregateReturns("仙人指路", results)
	if st.Total != 3 || st.Wins != 2 {
		t.Fatalf("expect 3/2, got %+v", st)
	}
	if st.WinRate < 0.666 || st.WinRate > 0.667 {
		t.Fatalf("WinRate: %v", st.WinRate)
	}
	if st.ProfitLossRatio < 2.99 || st.ProfitLossRatio > 3.01 {
		t.Fatalf("PLRatio expect ~3, got %v", st.ProfitLossRatio)
	}
}
