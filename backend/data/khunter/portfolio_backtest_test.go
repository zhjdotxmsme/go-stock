package khunter

import (
	"context"
	"math"
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

func TestEquityMaxDrawdownUsesDailyEquityPath(t *testing.T) {
	got := EquityMaxDrawdown([]float64{100000, 110000, 99000, 105000})
	want := (110000.0 - 99000.0) / 110000.0
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("组合最大回撤=%v, want=%v", got, want)
	}
	if EquityMaxDrawdown(nil) != 0 {
		t.Fatal("空权益序列回撤应为 0")
	}
	if EquityMaxDrawdown([]float64{100000}) != 0 {
		t.Fatal("单点权益序列回撤应为 0")
	}
}

type barRow struct {
	d                     string
	o, h, l, c, prevClose float64
}

type benchRow struct {
	d string
	c float64
}

func dayBars(code string, rows []barRow) []models.KLineBar {
	bars := make([]models.KLineBar, 0, len(rows))
	for _, r := range rows {
		bars = append(bars, models.KLineBar{
			StockCode: code, Period: "day", TradeDate: r.d,
			Open: r.o, High: r.h, Low: r.l, Close: r.c, PrevClose: r.prevClose,
			Volume: 10000, Amount: 100000,
		})
	}
	return bars
}

func benchBars(rows []benchRow) []models.KLineBar {
	bars := make([]models.KLineBar, 0, len(rows))
	for _, r := range rows {
		bars = append(bars, models.KLineBar{
			StockCode: PortfolioBenchmarkCode, Period: "day", TradeDate: r.d,
			Open: r.c, High: r.c, Low: r.c, Close: r.c,
		})
	}
	return bars
}

func defaultPortfolioCfg(cash float64) RiskControlConfig {
	cfg := DefaultRiskControlConfig()
	cfg.InitialCash = cash
	cfg.TransferFeeRate = 0.0001
	return cfg
}

// 单笔止损退出：买入加滑点、卖出减滑点、净收益扣双边佣金+印花税+过户费，
// 每日权益=现金+持仓市值，回撤从权益序列计算，成本可从成交记录复算。
func TestPortfolioBacktestReplaysCostsAndEquity(t *testing.T) {
	cfg := defaultPortfolioCfg(100000)

	stock := dayBars("sh600000", []barRow{
		{"2026-01-05", 10, 10.4, 9.9, 10.2, 10},
		{"2026-01-06", 10, 10.3, 9.95, 10.1, 10.2},
		{"2026-01-07", 9.5, 9.6, 8.9, 9, 10.1},
	})
	bench := benchBars([]benchRow{
		{"2026-01-05", 4000},
		{"2026-01-06", 4020},
		{"2026-01-07", 3990},
		{"2026-01-08", 4010},
	})

	report := RunPortfolioBacktest(PortfolioBacktestInput{
		BenchmarkBars: bench,
		CodeBars:      map[string][]models.KLineBar{"sh600000": stock},
		Entries: []PortfolioEntry{{
			Code: "sh600000", SignalDate: "2026-01-05", ConfirmDate: "2026-01-06",
			EntryPrice: 10, StopPrice: 9, SignalHigh: 10.2, Shares: 5000, ConfirmSeq: 0,
		}},
		DefaultPositionLimit: 1.0,
		Cfg:                  cfg,
	})

	if report.SignalCount != 1 || report.TradeCount != 1 || report.InvalidCount != 0 || report.UntradeableCount != 0 {
		t.Fatalf("样本守恒错误: %+v", report)
	}
	tr := report.Trades[0]
	if tr.Code != "sh600000" || tr.SignalDate != "2026-01-05" || tr.EntryDate != "2026-01-06" || tr.ExitDate != "2026-01-07" {
		t.Fatalf("日期错误: %+v", tr)
	}
	if tr.Shares != 5000 || tr.HoldingDays != 1 || tr.ExitReason != "失效位退出" {
		t.Fatalf("成交记录错误: %+v", tr)
	}
	if math.Abs(tr.EntryPrice-10*1.0005) > 1e-9 || math.Abs(tr.ExitPrice-9*0.9995) > 1e-9 {
		t.Fatalf("滑点未生效: Entry=%v Exit=%v", tr.EntryPrice, tr.ExitPrice)
	}
	// 成本可复算：从成交价与配置重算净收益应一致
	buyCost := buyTotalCost(float64(tr.Shares)*tr.EntryPrice, cfg.CostConfig)
	sellNet := sellNetProceeds(float64(tr.Shares)*tr.ExitPrice, cfg.CostConfig)
	recomputed := sellNet/buyCost - 1
	if math.Abs(recomputed-tr.NetReturn) > 1e-9 {
		t.Fatalf("净收益不可复算: %v vs %v", recomputed, tr.NetReturn)
	}
	if math.Abs(tr.GrossReturn-(tr.ExitPrice/tr.EntryPrice-1)) > 1e-9 {
		t.Fatalf("毛收益错误: %v", tr.GrossReturn)
	}

	equity := report.PortfolioEquity
	if len(equity) != 4 {
		t.Fatalf("权益点数=%d, want=4", len(equity))
	}
	if math.Abs(equity[0]-100000) > 1e-6 {
		t.Fatalf("起始权益=%v", equity[0])
	}
	// D2 买入后：成交额 50025 + 佣金 15.0075 + 过户费 5.0025 = 50045.01 现金支出，市值 5000*10.1
	wantD2 := 100000 - 50045.01 + 50500
	if math.Abs(equity[1]-wantD2) > 0.01 {
		t.Fatalf("D2 权益=%v, want≈%v", equity[1], wantD2)
	}
	// D3 止损退出后全部为现金
	wantD3 := equity[1] - 50500 + sellNet
	if math.Abs(equity[2]-wantD3) > 0.01 {
		t.Fatalf("D3 权益=%v, want≈%v", equity[2], wantD3)
	}
	if math.Abs(report.Metrics.PortfolioMaxDD-EquityMaxDrawdown(equity)) > 1e-9 {
		t.Fatalf("组合回撤应从权益序列计算: %v", report.Metrics.PortfolioMaxDD)
	}
	// 基准固定 sh000300：(4010-4000)/4000
	if math.Abs(report.Metrics.BenchmarkReturn-0.0025) > 1e-9 {
		t.Fatalf("基准收益=%v, want=0.0025", report.Metrics.BenchmarkReturn)
	}
	if math.Abs(report.Metrics.ExcessReturn-(report.Metrics.AvgNetReturn-0.0025)) > 1e-9 {
		t.Fatalf("超额收益=%v", report.Metrics.ExcessReturn)
	}
}

// 敞口上限、不足一手不可成交、同日入场按（确认顺序,代码）确定性排序、T+1 当日不退出。
func TestPortfolioBacktestExposureCapSubLotAndEntryOrder(t *testing.T) {
	cfg := defaultPortfolioCfg(100000)

	bench := benchBars([]benchRow{
		{"2026-01-05", 4000},
		{"2026-01-06", 4010},
		{"2026-01-07", 4000},
		{"2026-01-08", 4010},
	})
	codeBars := map[string][]models.KLineBar{
		"sz000001": dayBars("sz000001", []barRow{
			{"2026-01-06", 10, 10.3, 9.95, 10.1, 10.2},
			{"2026-01-07", 10.2, 10.5, 10.0, 10.3, 10.1},
			{"2026-01-08", 10.3, 10.6, 10.2, 10.4, 10.3},
		}),
		// sh600000 信号高点 20.0，收盘均高于它，持仓到窗口结束被强平
		"sh600000": dayBars("sh600000", []barRow{
			{"2026-01-06", 20, 20.4, 19.9, 20.1, 20},
			{"2026-01-07", 20.2, 20.6, 20.0, 20.3, 20.1},
			{"2026-01-08", 20.3, 20.7, 20.2, 20.4, 20.3},
		}),
	}
	// 输入故意乱序：B(seq=1) 在 A(seq=0) 前面，引擎必须按确认顺序入场
	entries := []PortfolioEntry{
		{Code: "sh600000", SignalDate: "2026-01-05", ConfirmDate: "2026-01-06",
			EntryPrice: 20, StopPrice: 18, SignalHigh: 20.0, Shares: 4000, ConfirmSeq: 1},
		{Code: "sz000001", SignalDate: "2026-01-05", ConfirmDate: "2026-01-06",
			EntryPrice: 10, StopPrice: 9, SignalHigh: 10.2, Shares: 5000, ConfirmSeq: 0},
		{Code: "sz000002", SignalDate: "2026-01-05", ConfirmDate: "2026-01-06",
			EntryPrice: 5, StopPrice: 4.5, SignalHigh: 5.1, Shares: 50, ConfirmSeq: 2},
	}

	report := RunPortfolioBacktest(PortfolioBacktestInput{
		BenchmarkBars: bench,
		CodeBars:      codeBars,
		Entries:       entries,
		PositionLimits: map[string]float64{
			"2026-01-05": 0.6, "2026-01-06": 0.6, "2026-01-07": 0.6, "2026-01-08": 0.6,
		},
		DefaultPositionLimit: 1.0,
		Cfg:                  cfg,
	})

	if report.SignalCount != 3 || report.TradeCount != 2 || report.UntradeableCount != 1 {
		t.Fatalf("样本守恒错误: Signal=%d Trade=%d Untradeable=%d", report.SignalCount, report.TradeCount, report.UntradeableCount)
	}
	if len(report.UntradeableReasons) != 1 || report.UntradeableReasons[0] != "不足一手" {
		t.Fatalf("不足一手未记录: %v", report.UntradeableReasons)
	}
	// sz000001（seq=0）先入场拿到满额 5000 股；sh600000 受 60% 敞口限制被压缩到 400 股
	if report.Trades[0].Code != "sz000001" || report.Trades[0].Shares != 5000 {
		t.Fatalf("确认顺序入场失败: %+v", report.Trades[0])
	}
	if report.Trades[1].Code != "sh600000" || report.Trades[1].Shares != 400 {
		t.Fatalf("敞口上限未生效: %+v", report.Trades[1])
	}
	// T+1：sz000001 入场日收盘 10.1 < 信号高点 10.2，若当日可退出即违规；实际应持有到强平
	if report.Trades[0].ExitDate != "2026-01-08" || report.Trades[0].ExitReason != "回测结束强平" {
		t.Fatalf("T+1 约束或强平缺失: %+v", report.Trades[0])
	}
	for _, v := range report.PortfolioEquity {
		if v < 0 {
			t.Fatalf("权益出现负值（透支）: %v", v)
		}
	}
	final := report.PortfolioEquity[len(report.PortfolioEquity)-1]
	if final <= 0 {
		t.Fatalf("现金透支: %v", final)
	}
}

// 先退出后入场：同一交易日内止损释放的现金可用于当日新入场。
func TestPortfolioBacktestExitFreesCashForSameDayEntry(t *testing.T) {
	cfg := defaultPortfolioCfg(100000)

	bench := benchBars([]benchRow{
		{"2026-01-05", 4000},
		{"2026-01-06", 4010},
		{"2026-01-07", 4000},
		{"2026-01-08", 4010},
	})
	codeBars := map[string][]models.KLineBar{
		"sh600000": dayBars("sh600000", []barRow{
			{"2026-01-06", 10, 10.3, 9.95, 10.1, 10.2},
			{"2026-01-07", 9.5, 9.6, 8.9, 9, 10.1},
		}),
		"sz000001": dayBars("sz000001", []barRow{
			{"2026-01-07", 40, 41, 39.5, 40.5, 40},
			{"2026-01-08", 40.4, 40.8, 40.2, 40.6, 40.5},
		}),
	}
	report := RunPortfolioBacktest(PortfolioBacktestInput{
		BenchmarkBars: bench,
		CodeBars:      codeBars,
		Entries: []PortfolioEntry{
			{Code: "sh600000", SignalDate: "2026-01-05", ConfirmDate: "2026-01-06",
				EntryPrice: 10, StopPrice: 9, SignalHigh: 10.2, Shares: 5000, ConfirmSeq: 0},
			{Code: "sz000001", SignalDate: "2026-01-06", ConfirmDate: "2026-01-07",
				EntryPrice: 40, StopPrice: 38, SignalHigh: 39, Shares: 2000, ConfirmSeq: 0},
		},
		DefaultPositionLimit: 1.0,
		Cfg:                  cfg,
	})

	if report.TradeCount != 2 {
		t.Fatalf("TradeCount=%d, want=2", report.TradeCount)
	}
	// 第一笔：sh600000 于 D3 止损退出（先退出）
	if report.Trades[0].Code != "sh600000" || report.Trades[0].ExitDate != "2026-01-07" {
		t.Fatalf("先退出失败: %+v", report.Trades[0])
	}
	// 第二笔：sz000001 当日入场。若退出资金不可用，现金只够约 1200 股；满额 2000 说明现金已释放
	if report.Trades[1].Code != "sz000001" || report.Trades[1].EntryDate != "2026-01-07" || report.Trades[1].Shares != 2000 {
		t.Fatalf("退出资金未用于同日入场: %+v", report.Trades[1])
	}
	if report.Trades[1].ExitReason != "回测结束强平" || report.Trades[1].ExitDate != "2026-01-08" {
		t.Fatalf("强平缺失: %+v", report.Trades[1])
	}
}

// 最大持有交易日到期退出（Task 8 退出轨迹）。
func TestPortfolioBacktestMaxHoldingExit(t *testing.T) {
	cfg := defaultPortfolioCfg(100000)

	dates := []string{"2026-01-05", "2026-01-06", "2026-01-07", "2026-01-08", "2026-01-09", "2026-01-12"}
	bench := make([]models.KLineBar, 0, len(dates))
	stock := make([]models.KLineBar, 0, len(dates))
	for i, d := range dates {
		bench = append(bench, models.KLineBar{
			StockCode: PortfolioBenchmarkCode, Period: "day", TradeDate: d,
			Open: 4000, High: 4000, Low: 4000, Close: float64(4000 + i),
		})
		stock = append(stock, models.KLineBar{
			StockCode: "sh600000", Period: "day", TradeDate: d,
			Open: 10, High: 10.2, Low: 9.8, Close: 10, PrevClose: 10,
		})
	}
	report := RunPortfolioBacktest(PortfolioBacktestInput{
		BenchmarkBars: bench,
		CodeBars:      map[string][]models.KLineBar{"sh600000": stock},
		Entries: []PortfolioEntry{{
			Code: "sh600000", SignalDate: "2026-01-04", ConfirmDate: "2026-01-05",
			EntryPrice: 10, StopPrice: 9, SignalHigh: 9.5, Shares: 5000, ConfirmSeq: 0,
		}},
		DefaultPositionLimit: 1.0,
		Cfg:                  cfg,
	})

	if report.TradeCount != 1 {
		t.Fatalf("TradeCount=%d", report.TradeCount)
	}
	tr := report.Trades[0]
	// D1 入场（days=0），D6 holdingDays=5 >= MaxHoldingTradingDays(5) → 到期退出
	if tr.ExitDate != "2026-01-12" || tr.ExitReason != "到期退出" || tr.HoldingDays != 5 {
		t.Fatalf("到期退出失败: %+v", tr)
	}
}

func TestCostHelpersRecomputeBothSides(t *testing.T) {
	cost := DefaultRiskControlConfig().CostConfig
	cost.TransferFeeRate = 0.0001
	// 最低佣金 5 元在小额成交上生效
	if v := commissionOf(1000, cost); v != 5 {
		t.Fatalf("最低佣金=%v, want=5", v)
	}
	if v := commissionOf(50000, cost); math.Abs(v-15) > 1e-9 {
		t.Fatalf("比例佣金=%v, want=15", v)
	}
	// 买入总支出 = 成交额 + 佣金 + 过户费；卖出净入金 = 成交额 - 佣金 - 印花税 - 过户费
	buy := buyTotalCost(100000, cost)
	wantBuy := 100000 + 30 + 10.0
	if math.Abs(buy-wantBuy) > 1e-9 {
		t.Fatalf("买入总支出=%v, want=%v", buy, wantBuy)
	}
	sell := sellNetProceeds(100000, cost)
	wantSell := 100000 - 30 - 50 - 10.0
	if math.Abs(sell-wantSell) > 1e-9 {
		t.Fatalf("卖出净入金=%v, want=%v", sell, wantSell)
	}
}

func TestPortfolioBenchmarkCodeIsFixed(t *testing.T) {
	if PortfolioBenchmarkCode != "sh000300" {
		t.Fatalf("组合回测基准必须固定为 sh000300, got %s", PortfolioBenchmarkCode)
	}
	setupTestDB(t)
	if err := db.Dao.AutoMigrate(&models.KLineBar{}); err != nil {
		t.Fatal(err)
	}
	bar := models.KLineBar{
		StockCode: "sh000300", Period: "day", TradeDate: "2026-01-06",
		Open: 4000, High: 4010, Low: 3990, Close: 4005, Volume: 1000,
	}
	if err := db.Dao.Create(&bar).Error; err != nil {
		t.Fatal(err)
	}
	bars, err := LoadPortfolioBenchmarkBars(context.Background(), "2026-01-01", "2026-01-31")
	if err != nil {
		t.Fatalf("基准加载失败: %v", err)
	}
	if len(bars) != 1 || bars[0].StockCode != "sh000300" || bars[0].Close != 4005 {
		t.Fatalf("基准加载结果错误: %+v", bars)
	}
}
