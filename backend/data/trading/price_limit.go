// backend/data/trading/price_limit.go
package trading

import (
	"math"
	"strings"

	"go-stock/backend/models"
)

// priceLimitTolerance 涨跌停价比较容差：限价已按 0.01 取整，0.001 覆盖浮点误差。
const priceLimitTolerance = 0.001

// PriceLimit 按 board 规则计算的一日真实涨跌停价。
type PriceLimit struct {
	UpPrice   float64
	DownPrice float64
	Rate      float64
}

// DailyPriceLimit 按股票板块、上一交易日有效收盘价和最小报价单位（0.01 元）
// 计算当日真实涨跌停价。规则：主板 10%；创业板 300/301 与科创板 688 为 20%
// （与是否 ST 无关）；北交所 4/8/920 号段 30%；主板 ST 5%。
// 代码无法识别（非 6 位数字，如指数/ETF/测试 mock）或 prevClose <= 0 时
// 返回 ok=false，调用方必须跳过涨跌停判定，不得按近似幅度命名涨停。
func DailyPriceLimit(stockCode string, prevClose float64, isST bool) (PriceLimit, bool) {
	code := strings.ToLower(strings.TrimSpace(stockCode))
	if i := strings.LastIndex(code, "."); i >= 0 {
		code = code[:i]
	}
	code = strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(code, "sh"), "sz"), "bj")
	if len(code) != 6 || prevClose <= 0 {
		return PriceLimit{}, false
	}
	rate := 0.10
	switch {
	case strings.HasPrefix(code, "300"), strings.HasPrefix(code, "301"), strings.HasPrefix(code, "688"):
		rate = 0.20
	case code[0] == '4', code[0] == '8', strings.HasPrefix(code, "920"):
		rate = 0.30
	case isST:
		rate = 0.05
	}
	return PriceLimit{
		Rate:      rate,
		UpPrice:   math.Round(prevClose*(1+rate)*100) / 100,
		DownPrice: math.Round(prevClose*(1-rate)*100) / 100,
	}, true
}

// atLimitPrice 报告价格是否等于限价（0.001 容差）。
func atLimitPrice(px, limit float64) bool {
	return math.Abs(px-limit) <= priceLimitTolerance
}

// ClosedAtLimitUp 报告 bar 收盘是否收于真实涨停价（0.001 容差）。
// 日线口径下这只说明「收在涨停价」，是否可买还需 OnePriceLimitUp 判断一字板。
func ClosedAtLimitUp(bar models.KLineBar, limit PriceLimit) bool {
	return atLimitPrice(bar.Close, limit.UpPrice)
}

// OnePriceLimitUp 一字涨停：开盘=最高=最低=收盘=涨停价，全天封死、实际买不进。
func OnePriceLimitUp(bar models.KLineBar, limit PriceLimit) bool {
	return atLimitPrice(bar.Open, limit.UpPrice) &&
		atLimitPrice(bar.High, limit.UpPrice) &&
		atLimitPrice(bar.Low, limit.UpPrice) &&
		atLimitPrice(bar.Close, limit.UpPrice)
}

// OnePriceLimitDown 一字跌停：开盘=最高=最低=收盘=跌停价，全天封死、实际卖不出。
func OnePriceLimitDown(bar models.KLineBar, limit PriceLimit) bool {
	return atLimitPrice(bar.Open, limit.DownPrice) &&
		atLimitPrice(bar.High, limit.DownPrice) &&
		atLimitPrice(bar.Low, limit.DownPrice) &&
		atLimitPrice(bar.Close, limit.DownPrice)
}
