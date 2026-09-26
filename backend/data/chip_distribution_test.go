package data

import (
	"fmt"
	"testing"
)

func TestChipPriceAtQuantile(t *testing.T) {
	// dist 覆盖价格 0.5/1.5/2.5/3.5（minP=0, width=1），总量 4
	dist := []float64{0, 1, 2, 1, 0}
	cases := []struct {
		q    float64
		want float64
	}{
		{0.0, 1.5},  // q=0 命中第一个非空 bin
		{0.25, 1.5}, // 累计 1 到达 25%
		{0.5, 2.5},  // 累计 3 跨过 50%
		{0.75, 2.5}, // 累计 3 跨过 75%（=3）
		{1.0, 3.5},  // 100% 落在最后一个非空 bin
	}
	for _, c := range cases {
		got := chipPriceAtQuantile(dist, 0, 1, 4, c.q)
		if got != c.want {
			t.Errorf("q=%v: got %v, want %v", c.q, got, c.want)
		}
	}
	// 空分布 / 越界
	if got := chipPriceAtQuantile([]float64{0, 0}, 0, 1, 0, 0.5); got != 0 {
		t.Errorf("empty dist: got %v, want 0", got)
	}
	if got := chipPriceAtQuantile(dist, 0, 1, 4, 1.5); got != 0 {
		t.Errorf("q>1: got %v, want 0", got)
	}
}

func TestChipConcentration(t *testing.T) {
	if got := chipConcentration([2]float64{10, 20}); got <= 0 || got >= 1 {
		t.Errorf("normal range: got %v, want in (0,1)", got)
	}
	// 区间越窄越集中
	narrow := chipConcentration([2]float64{10, 11})
	wide := chipConcentration([2]float64{10, 20})
	if narrow >= wide {
		t.Errorf("narrow %v should be < wide %v", narrow, wide)
	}
	// 无效输入
	for _, r := range [][2]float64{{0, 10}, {20, 10}, {0, 0}} {
		if got := chipConcentration(r); got != 0 {
			t.Errorf("range %v: got %v, want 0", r, got)
		}
	}
}

// 一字板（low==high）+ 换手率 0：筹码不衰减、单档堆积，分布完全可预期。
func TestCalculate_CostRangeAndConcentration(t *testing.T) {
	kLines := make([]KLineData, 0, 10)
	for i := range 10 {
		p := fmt.Sprintf("%d", 10+i) // 价格 10..19，每日单档
		kLines = append(kLines, KLineData{
			Day:          fmt.Sprintf("2026-09-%02d", i+1),
			Open:         p,
			Close:        p,
			High:         p,
			Low:          p,
			Volume:       "100",
			Amount:       fmt.Sprintf("%d00", (10+i)*100),
			TurnoverRate: "0",
		})
	}

	res, err := NewChipDistributionCalculator().Calculate("sh600000", kLines, 90)
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}

	// 分位单调性：P5 <= P15 <= 中位 <= P85 <= P95
	if !(res.CostRange90[0] <= res.CostRange70[0] &&
		res.CostRange70[0] <= res.MedianCost &&
		res.MedianCost <= res.CostRange70[1] &&
		res.CostRange70[1] <= res.CostRange90[1]) {
		t.Errorf("quantile order violated: 90=%v 70=%v median=%v",
			res.CostRange90, res.CostRange70, res.MedianCost)
	}
	// 均匀分布下中位成本应落在价格中枢附近 [13.5, 15.5]
	if res.MedianCost < 13.5 || res.MedianCost > 15.5 {
		t.Errorf("MedianCost %v out of expected central band", res.MedianCost)
	}
	// 90% 区间应覆盖几乎全部价格带 [10, 19]
	if res.CostRange90[0] > 11 || res.CostRange90[1] < 18 {
		t.Errorf("CostRange90 %v should span near-full band", res.CostRange90)
	}
	// 集中度：90% 区间更宽 → 集中度更高
	if !(res.Concentration90 > res.Concentration70 && res.Concentration70 > 0) {
		t.Errorf("concentration ordering violated: 90=%v 70=%v",
			res.Concentration90, res.Concentration70)
	}
	// 现价 19 为最高档，获利比例应为 1
	if res.ProfitRatio != 1 {
		t.Errorf("ProfitRatio got %v, want 1", res.ProfitRatio)
	}
}
