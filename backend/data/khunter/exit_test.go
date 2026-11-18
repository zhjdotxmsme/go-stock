package khunter

import (
	"math"
	"testing"

	"go-stock/backend/data/trading"
	"go-stock/backend/models"
)

func TestExitGapThroughStopUsesOpen(t *testing.T) {
	p := models.KhunterHunting{EntryPrice: 100, InitialStopPrice: 90, ProtectPrice: 90, InitialRisk: 10, Status: "持仓中"}
	d := EvaluateExitDay(p, 100,
		models.KLineBar{Open: 85, High: 88, Low: 84, Close: 86},
		trading.PriceLimit{DownPrice: 81}, false, 1, DefaultRiskControlConfig())
	if !d.Exit || d.ExitPrice != 85 || d.Reason != "失效位跳空退出" {
		t.Fatalf("跳空退出错误: %+v", d)
	}
}

func TestOneRDoesNotUseOptimisticIntradayOrder(t *testing.T) {
	p := models.KhunterHunting{EntryPrice: 100, InitialStopPrice: 90, ProtectPrice: 90, InitialRisk: 10}
	d := EvaluateExitDay(p, 100,
		models.KLineBar{Open: 100, High: 111, Low: 89, Close: 105},
		trading.PriceLimit{DownPrice: 80}, false, 1, DefaultRiskControlConfig())
	if !d.Exit || d.ExitPrice != 90 {
		t.Fatalf("不得假定先到1R再保本退出: %+v", d)
	}
}

func TestEntryDayStopWaitsForTPlusOne(t *testing.T) {
	p := models.KhunterHunting{EntryPrice: 100, InitialStopPrice: 90, ProtectPrice: 90, InitialRisk: 10}
	entryDay := EvaluateExitDay(p, 100,
		models.KLineBar{Open: 100, High: 101, Low: 85, Close: 88},
		trading.PriceLimit{DownPrice: 80}, true, 0, DefaultRiskControlConfig())
	if entryDay.Exit || !entryDay.Pending {
		t.Fatalf("A股入场日不得假设卖出: %+v", entryDay)
	}
	nextDay := EvaluateExitDay(p, 100,
		models.KLineBar{Open: 80, High: 82, Low: 78, Close: 79},
		trading.PriceLimit{DownPrice: 79.2}, false, 1, DefaultRiskControlConfig())
	if !nextDay.Exit || nextDay.ExitPrice != 80 {
		t.Fatalf("下一交易日应按实际开盘价退出: %+v", nextDay)
	}
}

func TestFiveDayTimeExit(t *testing.T) {
	p := models.KhunterHunting{EntryPrice: 100, InitialStopPrice: 90, ProtectPrice: 90, InitialRisk: 10}
	d := EvaluateExitDay(p, 100,
		models.KLineBar{Open: 101, High: 102, Low: 100, Close: 101},
		trading.PriceLimit{DownPrice: 90}, false, 5, DefaultRiskControlConfig())
	if !d.Exit || d.ExitPrice != 101 || d.Reason != "到期退出" {
		t.Fatalf("五日到期退出错误: %+v", d)
	}
}

func TestOnePriceLimitDownHoldsPosition(t *testing.T) {
	p := models.KhunterHunting{EntryPrice: 100, InitialStopPrice: 90, ProtectPrice: 95, InitialRisk: 10}
	d := EvaluateExitDay(p, 100,
		models.KLineBar{Open: 90, High: 90, Low: 90, Close: 90},
		trading.PriceLimit{DownPrice: 90}, false, 2, DefaultRiskControlConfig())
	if d.Exit || !d.Pending {
		t.Fatalf("一字跌停应继续持有并标记待退出: %+v", d)
	}
}

func TestOneRRaisesProtectToEntry(t *testing.T) {
	p := models.KhunterHunting{EntryPrice: 100, InitialStopPrice: 90, ProtectPrice: 90, InitialRisk: 10}
	d := EvaluateExitDay(p, 100,
		models.KLineBar{Open: 100, High: 111, Low: 100, Close: 105},
		trading.PriceLimit{DownPrice: 80}, false, 1, DefaultRiskControlConfig())
	if d.Exit || d.NewProtectPrice != 100 {
		t.Fatalf("1R 锁盈应把保护位提到成本价: %+v", d)
	}
}

func TestBreakoutFailureExitsAtClose(t *testing.T) {
	p := models.KhunterHunting{EntryPrice: 100, InitialStopPrice: 90, ProtectPrice: 90, InitialRisk: 10}
	d := EvaluateExitDay(p, 100,
		models.KLineBar{Open: 101, High: 102, Low: 99, Close: 99.5},
		trading.PriceLimit{DownPrice: 80}, false, 1, DefaultRiskControlConfig())
	if !d.Exit || math.Abs(d.ExitPrice-99.5) > 1e-9 || d.Reason != "突破失败" {
		t.Fatalf("突破失败应按收盘价退出: %+v", d)
	}
}
