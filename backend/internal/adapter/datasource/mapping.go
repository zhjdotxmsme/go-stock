package datasource

import (
	"strconv"
	"strings"
	"time"

	"go-stock/backend/data"
	portds "go-stock/backend/internal/port/datasource"
	"go-stock/backend/stockcode"
)

// data <-> port 显式映射（不反射），与 adapter/repository/sqlite 的模式一致。

// klineRowsToPort 把 data.KLineData（字符串数值）序列映射为 port KLineData。
// 空输入返回 errNoData（触发路由 fallback）；
// 单条字段解析失败按 0 处理（与 data 包 parseKLine 的容错风格一致）。
// PrevClose 由前一根 bar 的 Close 补算（data.KLineData 无该字段）。
func klineRowsToPort(code, period string, rows *[]data.KLineData) (*portds.KLineData, error) {
	if rows == nil || len(*rows) == 0 {
		return nil, errNoData
	}
	bars := make([]portds.KLineBar, 0, len(*rows))
	for _, r := range *rows {
		bars = append(bars, portds.KLineBar{
			Time:   parseKLineTime(r.Day),
			Open:   parseFloat(r.Open),
			High:   parseFloat(r.High),
			Low:    parseFloat(r.Low),
			Close:  parseFloat(r.Close),
			Volume: parseInt(r.Volume),
			Amount: parseFloat(r.Amount),
		})
	}
	for i := 1; i < len(bars); i++ {
		bars[i].PrevClose = bars[i-1].Close
	}
	return &portds.KLineData{
		Code:   code,
		Period: period,
		Bars:   bars,
	}, nil
}

// stockInfoToQuote 把 data.StockInfo（腾讯实时行情快照）映射为 port QuoteData。
// Change/ChangePct 为二次计算字段，源头未填充时按 Price/PreClose 补算；
// 买一/卖一价与五档之外的上下文放入 Extra。
func stockInfoToQuote(info *data.StockInfo) *portds.QuoteData {
	if info == nil {
		return nil
	}
	price := parseFloat(info.Price)
	prevClose := parseFloat(info.PreClose)
	change, changePct := info.ChangePrice, info.ChangePercent
	if change == 0 && price > 0 && prevClose > 0 {
		change = price - prevClose
		changePct = change / prevClose * 100
	}
	return &portds.QuoteData{
		Code:      stockcode.Normalize(info.Code),
		Name:      info.Name,
		Price:     price,
		Change:    change,
		ChangePct: changePct,
		Volume:    parseInt(info.Volume),
		Amount:    parseFloat(info.Amount),
		High:      parseFloat(info.High),
		Low:       parseFloat(info.Low),
		Open:      parseFloat(info.Open),
		PrevClose: prevClose,
		Time:      parseQuoteTime(info.Date, info.Time),
		Extra: map[string]interface{}{
			"bid":    parseFloat(info.Bid),
			"ask":    parseFloat(info.Ask),
			"ask1":   parseFloat(info.A1P),
			"bid1":   parseFloat(info.B1P),
			"market": info.Market,
		},
	}
}

// parseFloat 解析字符串数值；空串/非法值返回 0（与 data 包容错风格一致）。
func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

// parseInt 解析字符串整数；兼容带小数的成交量（截断取整）。
func parseInt(s string) int64 {
	s = strings.TrimSpace(s)
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		return v
	}
	v, _ := strconv.ParseFloat(s, 64)
	return int64(v)
}

// klineTimeLayouts K 线日期串的常见布局（日 K 为日期，分钟 K 带时分）。
var klineTimeLayouts = []string{
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"2006-01-02",
	"2006/01/02",
}

// parseKLineTime 解析 K 线日期串；全部布局不匹配时返回零值时间。
func parseKLineTime(day string) time.Time {
	day = strings.TrimSpace(day)
	for _, layout := range klineTimeLayouts {
		if t, err := time.ParseInLocation(layout, day, time.Local); err == nil {
			return t
		}
	}
	return time.Time{}
}

// parseQuoteTime 组合行情快照的 Date/Time 字段；失败时退化为零值时间。
func parseQuoteTime(date, tm string) time.Time {
	s := strings.TrimSpace(strings.TrimSpace(date) + " " + strings.TrimSpace(tm))
	if s == "" {
		return time.Time{}
	}
	return parseKLineTime(s)
}
