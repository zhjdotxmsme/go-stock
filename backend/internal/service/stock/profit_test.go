package stock

import "testing"

func TestCalcProfitFields_NormalTrading(t *testing.T) {
	r := CalcProfitFields(ProfitInput{
		Price: "10.50", AskPrice: "10.51", BidPrice: "10.49",
		PreClose: "10.00", High: "11.00", Low: "9.80", Open: "10.10",
		CostPrice: 9.00, Volume: 1000,
	})
	if r.EffectivePrice != 10.50 {
		t.Errorf("EffectivePrice = %v, want 10.5", r.EffectivePrice)
	}
	if r.ChangePrice != 0.5 {
		t.Errorf("ChangePrice = %v, want 0.5", r.ChangePrice)
	}
	if r.ChangePercent != 5 {
		t.Errorf("ChangePercent = %v, want 5", r.ChangePercent)
	}
	if r.HighRate != 10 {
		t.Errorf("HighRate = %v, want 10", r.HighRate)
	}
	if r.LowRate != -2 {
		t.Errorf("LowRate = %v, want -2", r.LowRate)
	}
	// 盈亏率 = (10.5-9)/9*100 = 16.667；盈亏额 = 1.5*1000 = 1500；今日盈亏 = 0.5*1000 = 500
	if r.Profit != 16.667 {
		t.Errorf("Profit = %v, want 16.667", r.Profit)
	}
	if r.ProfitAmount != 1500 {
		t.Errorf("ProfitAmount = %v, want 1500", r.ProfitAmount)
	}
	if r.ProfitAmountToday != 500 {
		t.Errorf("ProfitAmountToday = %v, want 500", r.ProfitAmountToday)
	}
}

func TestCalcProfitFields_PriceFallbackChain(t *testing.T) {
	// 最新价为空 → 卖一
	r := CalcProfitFields(ProfitInput{Price: "", AskPrice: "9.9", PreClose: "10.00"})
	if r.EffectivePrice != 9.9 {
		t.Errorf("fallback to ask: EffectivePrice = %v, want 9.9", r.EffectivePrice)
	}
	// 卖一也为空 → 买一
	r = CalcProfitFields(ProfitInput{Price: "", AskPrice: "", BidPrice: "9.8", PreClose: "10.00"})
	if r.EffectivePrice != 9.8 {
		t.Errorf("fallback to bid: EffectivePrice = %v, want 9.8", r.EffectivePrice)
	}
	// 全空 → 昨收
	r = CalcProfitFields(ProfitInput{PreClose: "10.00"})
	if r.EffectivePrice != 10.0 {
		t.Errorf("fallback to preclose: EffectivePrice = %v, want 10", r.EffectivePrice)
	}
}

func TestCalcProfitFields_HighLowFallbackToOpen(t *testing.T) {
	r := CalcProfitFields(ProfitInput{Price: "10.5", PreClose: "10.00", Open: "10.20"})
	if r.HighRate != 2 { // high 缺省 → open=10.2 → (10.2-10)/10*100
		t.Errorf("HighRate = %v, want 2", r.HighRate)
	}
	if r.LowRate != 2 {
		t.Errorf("LowRate = %v, want 2", r.LowRate)
	}
}

func TestCalcProfitFields_PreMarket(t *testing.T) {
	// 未开盘：价格全空 → 生效价=昨收，今日盈亏必须为 0
	r := CalcProfitFields(ProfitInput{PreClose: "10.00", CostPrice: 8.0, Volume: 100})
	if r.EffectivePrice != 10.0 {
		t.Errorf("EffectivePrice = %v, want 10", r.EffectivePrice)
	}
	if r.ProfitAmountToday != 0 {
		t.Errorf("premarket ProfitAmountToday = %v, want 0", r.ProfitAmountToday)
	}
	// 隔夜盈亏按昨收计算: (10-8)/8*100 = 25
	if r.Profit != 25 {
		t.Errorf("premarket Profit = %v, want 25", r.Profit)
	}
	if r.ProfitAmount != 200 { // (10-8)*100
		t.Errorf("premarket ProfitAmount = %v, want 200", r.ProfitAmount)
	}
}

func TestCalcProfitFields_NoPosition(t *testing.T) {
	// 无持仓（成本/数量为零）时不应计算盈亏字段
	r := CalcProfitFields(ProfitInput{Price: "10.5", PreClose: "10.00", CostPrice: 0, Volume: 0})
	if r.Profit != 0 || r.ProfitAmount != 0 || r.ProfitAmountToday != 0 {
		t.Errorf("no position should leave profit fields zero, got %+v", r)
	}
	// 涨跌幅仍应计算
	if r.ChangePercent != 5 {
		t.Errorf("ChangePercent = %v, want 5", r.ChangePercent)
	}
}

func TestCalcProfitFields_ZeroPrecloseNoDiv(t *testing.T) {
	// 昨收为 0 时不应计算涨跌幅（避免除零）
	r := CalcProfitFields(ProfitInput{Price: "10.5", PreClose: "0"})
	if r.ChangePrice != 0 || r.ChangePercent != 0 {
		t.Errorf("zero preclose should skip change calc, got %+v", r)
	}
}
