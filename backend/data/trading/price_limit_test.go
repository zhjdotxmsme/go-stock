package trading

import (
	"math"
	"testing"

	"go-stock/backend/models"
)

// TestDailyPriceLimitByBoardAndTick 按板块验证真实涨停价（四舍五入到 0.01）：
// 主板 10%、创业板 300/301 与科创板 688 20%（与 ST 无关）、北交所 30%、主板 ST 5%。
func TestDailyPriceLimitByBoardAndTick(t *testing.T) {
	tests := []struct {
		code string
		st   bool
		up   float64
	}{
		{"600000", false, 11.00},
		{"300780", false, 12.00},
		{"688322", false, 12.00},
		{"920001", false, 13.00},
		{"600000", true, 10.50},
		{"300780", true, 12.00},
	}
	for _, tt := range tests {
		limit, ok := DailyPriceLimit(tt.code, 10, tt.st)
		if !ok || math.Abs(limit.UpPrice-tt.up) > 1e-9 {
			t.Fatalf("%s: limit=%+v ok=%v", tt.code, limit, ok)
		}
	}
}

// TestDailyPriceLimitFormats 裸码/前缀/ts_code 三种格式都应得到同一板块规则。
func TestDailyPriceLimitFormats(t *testing.T) {
	for _, code := range []string{"300780", "sz300780", "SZ300780", "300780.SZ"} {
		limit, ok := DailyPriceLimit(code, 10, false)
		if !ok || math.Abs(limit.UpPrice-12.00) > 1e-9 {
			t.Fatalf("%s: limit=%+v ok=%v", code, limit, ok)
		}
	}
}

// TestDailyPriceLimitInvalid 代码无法识别或前收无效时不给规则，调用方必须跳过判定。
func TestDailyPriceLimitInvalid(t *testing.T) {
	for _, tc := range []struct {
		code      string
		prevClose float64
	}{
		{"mock_win", 10},
		{"test_tplus1", 10},
		{"60051", 10},
		{"", 10},
		{"600000", 0},
		{"600000", -1},
	} {
		if limit, ok := DailyPriceLimit(tc.code, tc.prevClose, false); ok {
			t.Fatalf("%q prev=%v: 应返回 ok=false, got %+v", tc.code, tc.prevClose, limit)
		}
	}
}

// TestDailyPriceLimitDownPrice 跌停价同样按 0.01 取整，且主板 ST 幅度 5%。
func TestDailyPriceLimitDownPrice(t *testing.T) {
	limit, ok := DailyPriceLimit("600000", 9.97, false)
	if !ok || math.Abs(limit.DownPrice-8.97) > 1e-9 || math.Abs(limit.UpPrice-10.97) > 1e-9 {
		t.Fatalf("down=%v up=%v ok=%v", limit.DownPrice, limit.UpPrice, ok)
	}
	st, ok := DailyPriceLimit("000001", 10.03, true)
	if !ok || math.Abs(st.UpPrice-10.53) > 1e-9 || math.Abs(st.DownPrice-9.53) > 1e-9 {
		t.Fatalf("ST down=%v up=%v ok=%v", st.DownPrice, st.UpPrice, ok)
	}
}

// TestFifteenPercentRiseOnChiNextIsNotLimitUp 创业板 15% 上涨不得命名为涨停，20% 收盘才是。
func TestFifteenPercentRiseOnChiNextIsNotLimitUp(t *testing.T) {
	limit, ok := DailyPriceLimit("300780", 10, false)
	if !ok {
		t.Fatal("expected limit rule")
	}
	bar := models.KLineBar{Open: 10, High: 11.5, Low: 10, Close: 11.5, PrevClose: 10}
	if ClosedAtLimitUp(bar, limit) {
		t.Fatal("创业板 15% 上涨不能命名为涨停")
	}
	bar.High, bar.Close = 12, 12
	if !ClosedAtLimitUp(bar, limit) {
		t.Fatal("创业板 20% 收盘应识别为涨停")
	}
}

// TestOnePriceLimit 一字板判定：四价齐平于限价才可判定不可买/不可卖。
func TestOnePriceLimit(t *testing.T) {
	limit, _ := DailyPriceLimit("600000", 10, false) // 涨停 11 / 跌停 9

	luBar := models.KLineBar{Open: 11, High: 11, Low: 11, Close: 11}
	if !OnePriceLimitUp(luBar, limit) {
		t.Fatal("四价齐平于涨停价应判一字涨停")
	}
	// 盘中开板：最低价低于涨停价，不再是一字板
	openedBar := models.KLineBar{Open: 11, High: 11, Low: 10.98, Close: 11}
	if OnePriceLimitUp(openedBar, limit) {
		t.Fatal("最低价低于涨停价不是一字涨停")
	}
	// 收盘涨停但开盘不等于涨停价，也不是一字板
	closeOnly := models.KLineBar{Open: 10.5, High: 11, Low: 10.5, Close: 11}
	if OnePriceLimitUp(closeOnly, limit) || !ClosedAtLimitUp(closeOnly, limit) {
		t.Fatal("收盘涨停但非一字板判定错误")
	}

	ldBar := models.KLineBar{Open: 9, High: 9, Low: 9, Close: 9}
	if !OnePriceLimitDown(ldBar, limit) {
		t.Fatal("四价齐平于跌停价应判一字跌停")
	}
	if OnePriceLimitUp(ldBar, limit) {
		t.Fatal("跌停价不得判为一字涨停")
	}
	// 跌停价附近但不等于限价（>0.001 容差）不得判跌停
	near := models.KLineBar{Open: 9.01, High: 9.01, Low: 9.01, Close: 9.01}
	if ClosedAtLimitUp(near, limit) || OnePriceLimitDown(near, limit) {
		t.Fatal("9.01 不等于跌停价 9.00，不应判定")
	}
}
