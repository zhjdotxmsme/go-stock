package strategy

import (
	"fmt"
	"testing"
	"time"

	"go-stock/backend/models"
)

// mkBars 按正序生成 K 线；o/h/l/c/v 等长
func mkBars(dates []string, o, h, l, c []float64, v []int64) []models.KLineBar {
	bars := make([]models.KLineBar, len(dates))
	for i := range dates {
		prev := 0.0
		if i > 0 {
			prev = c[i-1]
		}
		bars[i] = models.KLineBar{StockCode: "600000", Period: "day", TradeDate: dates[i],
			Adjusted: true, Open: o[i], High: h[i], Low: l[i], Close: c[i], PrevClose: prev, Volume: v[i]}
	}
	return bars
}

// genDates 生成 n 个连续日期字符串（不需要是真实交易日，策略只看顺序）
func genDates(n int) []string {
	dates := make([]string, n)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		dates[i] = base.AddDate(0, 0, i).Format("2006-01-02")
	}
	return dates
}

// mkConst 生成等长常量序列的快捷方式（构造测试数据用）
func mkConst(n int, val float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = val
	}
	return out
}

// mkVol 生成等长常量成交量
func mkVol(n int, val int64) []int64 {
	out := make([]int64, n)
	for i := range out {
		out[i] = val
	}
	return out
}

var _ = fmt.Sprintf // 保留 fmt 供各策略测试打印调试

func TestAvgVolumeBefore(t *testing.T) {
	vols := []float64{10, 20, 30, 40, 50, 60}
	// idx=5，前5日均量 = (10+20+30+40+50)/5 = 30，不含 idx=5 的 60
	if got := AvgVolumeBefore(vols, 5, 5); got != 30 {
		t.Fatalf("expect 30, got %v", got)
	}
	// 数据不足返回 0
	if got := AvgVolumeBefore(vols, 3, 5); got != 0 {
		t.Fatalf("expect 0, got %v", got)
	}
}

func TestIsInvalidName(t *testing.T) {
	for _, n := range []string{"ST中安", "*ST左江", "退市博天", "未知"} {
		if !IsInvalidName(n) {
			t.Fatalf("%s 应被过滤", n)
		}
	}
	if IsInvalidName("贵州茅台") {
		t.Fatal("正常名称不应被过滤")
	}
}

func TestPctChange(t *testing.T) {
	bars := mkBars(genDates(3),
		[]float64{10, 10, 10}, []float64{10, 10, 11}, []float64{10, 10, 10},
		[]float64{10, 10, 11}, []int64{1, 1, 1})
	if got := PctChange(bars, 2); got < 0.099 || got > 0.101 {
		t.Fatalf("expect ~0.10, got %v", got)
	}
	if got := PctChange(bars, 0); got != 0 {
		t.Fatalf("首根无前收，expect 0, got %v", got)
	}
}

func TestSwingHighs(t *testing.T) {
	highs := []float64{1, 3, 2, 5, 4, 6, 1, 2, 1}
	// leftRight=2：i=3(5) 左右两根均小于它 → 是；i=5(6) 右侧只有 3 根且 1<6,2<6 但右窗需满 2 → i=5 右侧有 [1,2] 满足
	got := SwingHighs(highs, 2)
	if len(got) != 2 || got[0] != 3 || got[1] != 5 {
		t.Fatalf("expect [3 5], got %v", got)
	}
	// 最右侧 2 根不参与（右窗不足）
	highs2 := []float64{1, 3, 2, 5, 4, 6, 1, 9, 1}
	got2 := SwingHighs(highs2, 2)
	for _, i := range got2 {
		if i == 7 {
			t.Fatalf("i=7 右窗不足 2，不应成为 swing high")
		}
	}
}
