// Package stock 自选股/群组服务（方案 Step 5.1）
// 该层只依赖 port 接口与纯计算,不直接引用 data/db。
// profit.go 承载自选股行情叠加的涨跌幅/盈亏计算口径,
// 计算规则与原 handler.addStockFollowData 逐字一致,抽出以便单测。
package stock

import (
	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/mathutil"
)

// ProfitInput 行情快照的原始数值输入。
// Price/AskPrice/BidPrice 等允许为字符串字段的原值（内部再转换），
// 与 handler 侧 data.StockInfo 的字段一一对应。
type ProfitInput struct {
	Price    string // 最新价
	AskPrice string // 卖一价 A1P
	BidPrice string // 买一价 B1P
	PreClose string // 昨收
	High     string // 今日最高
	Low      string // 今日最低
	Open     string // 今开

	CostPrice float64 // 成本价
	Volume    int64   // 持仓量
}

// ProfitResult 涨跌幅/盈亏计算结果；零值字段表示条件不满足、未计算。
type ProfitResult struct {
	EffectivePrice float64 // 生效价格：最新价 → 卖一 → 买一 → 昨收 的降级链

	ChangePrice    float64
	ChangePercent  float64
	HighRate       float64
	LowRate        float64
	Profit         float64 // 持仓盈亏率 %
	ProfitAmount   float64 // 持仓盈亏额
	ProfitAmountToday float64 // 今日盈亏额
}

// CalcProfitFields 计算涨跌幅与盈亏字段。
// 规则要点（与原实现逐字一致）：
//   - 生效价格降级链：最新价 → 卖一价 → 买一价 → 昨收
//   - 最高/最低价缺省时回落到今开
//   - 未开盘（生效价=昨收）时今日盈亏为 0
func CalcProfitFields(in ProfitInput) ProfitResult {
	price, _ := convertor.ToFloat(in.Price)
	if price == 0 {
		price, _ = convertor.ToFloat(in.AskPrice)
	}
	if price == 0 {
		price, _ = convertor.ToFloat(in.BidPrice)
	}

	preClosePrice, _ := convertor.ToFloat(in.PreClose)
	if price == 0 {
		price = preClosePrice
	}

	highPrice, _ := convertor.ToFloat(in.High)
	if highPrice == 0 {
		highPrice, _ = convertor.ToFloat(in.Open)
	}

	lowPrice, _ := convertor.ToFloat(in.Low)
	if lowPrice == 0 {
		lowPrice, _ = convertor.ToFloat(in.Open)
	}

	var r ProfitResult
	r.EffectivePrice = price

	if price > 0 && preClosePrice > 0 {
		r.ChangePrice = mathutil.RoundToFloat(price-preClosePrice, 2)
		r.ChangePercent = mathutil.RoundToFloat(mathutil.Div(price-preClosePrice, preClosePrice)*100, 3)
	}
	if highPrice > 0 && preClosePrice > 0 {
		r.HighRate = mathutil.RoundToFloat(mathutil.Div(highPrice-preClosePrice, preClosePrice)*100, 3)
	}
	if lowPrice > 0 && preClosePrice > 0 {
		r.LowRate = mathutil.RoundToFloat(mathutil.Div(lowPrice-preClosePrice, preClosePrice)*100, 3)
	}
	if in.CostPrice > 0 && in.Volume > 0 {
		if price > 0 {
			r.Profit = mathutil.RoundToFloat(mathutil.Div(price-in.CostPrice, in.CostPrice)*100, 3)
			r.ProfitAmount = mathutil.RoundToFloat((price-in.CostPrice)*float64(in.Volume), 2)
			r.ProfitAmountToday = mathutil.RoundToFloat((price-preClosePrice)*float64(in.Volume), 2)
		} else {
			// 未开盘时当前价格为昨日收盘价，今日盈亏为 0
			r.Profit = mathutil.RoundToFloat(mathutil.Div(preClosePrice-in.CostPrice, in.CostPrice)*100, 3)
			r.ProfitAmount = mathutil.RoundToFloat((preClosePrice-in.CostPrice)*float64(in.Volume), 2)
			r.ProfitAmountToday = 0
		}
	}
	return r
}
