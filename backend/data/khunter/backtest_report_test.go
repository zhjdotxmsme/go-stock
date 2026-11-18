package khunter

import (
	"math"
	"testing"
)

func TestWorstDecileAndMaximumLoss(t *testing.T) {
	returns := []float64{-0.20, -0.10, -0.05, 0, 0.01, 0.02, 0.03, 0.04, 0.05, 0.10}
	m := SummarizeReturns(returns)
	if m.WorstDecileAvg != -0.20 || m.MaxSingleLoss != -0.20 {
		t.Fatalf("尾部风险指标错误: %+v", m)
	}
}

func TestFailureSamplesRemainInReport(t *testing.T) {
	report := NewBacktestReport(2)
	report.AddInvalid("缺成交额")
	report.AddTrade(BacktestTrade{NetReturn: 0.01})
	if report.SignalCount != 2 || report.InvalidCount != 1 || report.TradeCount != 1 {
		t.Fatalf("失败样本被静默丢弃: %+v", report)
	}
}

// 零收益交易单独计数，不混入平均亏损分母：10 笔里 1 笔零收益，
// AvgLoss 的分母只有 2 笔负收益，均值 = (0.10+0.05)/2 = 0.075。
func TestZeroReturnExcludedFromAvgLossDenominator(t *testing.T) {
	returns := []float64{-0.10, 0, -0.05, 0.02}
	m := SummarizeReturns(returns)
	if m.ZeroReturnCount != 1 {
		t.Fatalf("零收益计数错误: %+v", m)
	}
	if math.Abs(m.AvgLoss-0.075) > 1e-9 {
		t.Fatalf("平均亏损=%v, want=0.075（分母不含零收益）", m.AvgLoss)
	}
	if m.TradeCount != 4 {
		t.Fatalf("TradeCount=%d, want=4", m.TradeCount)
	}
}

func TestSummarizeReturnsEmpty(t *testing.T) {
	m := SummarizeReturns(nil)
	if m.TradeCount != 0 || m.WorstDecileAvg != 0 || m.MaxSingleLoss != 0 {
		t.Fatalf("空序列指标应为零值: %+v", m)
	}
	if !m.SampleInsufficient {
		t.Fatal("空序列应标记样本不足")
	}
}

// 最差十分位：k = max(1, len/10)，20 笔取最差 2 笔均值。
func TestWorstDecileAveragesWorstTenPercent(t *testing.T) {
	returns := make([]float64, 0, 20)
	for i := 0; i < 20; i++ {
		returns = append(returns, float64(i)/100.0-0.10) // -0.10 ~ +0.09
	}
	m := SummarizeReturns(returns)
	want := (-0.10 + -0.09) / 2
	if math.Abs(m.WorstDecileAvg-want) > 1e-9 {
		t.Fatalf("WorstDecileAvg=%v, want=%v", m.WorstDecileAvg, want)
	}
	if m.SampleInsufficient {
		t.Fatal("20 笔样本不应标记样本不足")
	}
}

// AddTrade 后指标应随净收益序列刷新，SignalCount 守恒（信号数 ≥ 失败 + 成交）。
func TestReportMetricsRefreshOnAddTrade(t *testing.T) {
	report := NewBacktestReport(3)
	report.AddInvalid("数据缺失")
	report.AddTrade(BacktestTrade{NetReturn: 0.01})
	report.AddTrade(BacktestTrade{NetReturn: -0.03})
	if report.TradeCount != 2 || report.InvalidCount != 1 {
		t.Fatalf("计数错误: %+v", report)
	}
	if math.Abs(report.Metrics.AvgNetReturn-(-0.01)) > 1e-9 {
		t.Fatalf("AvgNetReturn=%v, want=-0.01", report.Metrics.AvgNetReturn)
	}
	if math.Abs(report.Metrics.MaxSingleLoss-(-0.03)) > 1e-9 {
		t.Fatalf("MaxSingleLoss=%v, want=-0.03", report.Metrics.MaxSingleLoss)
	}
}
