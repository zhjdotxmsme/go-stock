package freestockdb

import (
	"context"
	"encoding/json"
)

// DayKBar 对应服务端 "日k" 记录。字段名与上游一致。
type DayKBar struct {
	Date     int64   `json:"date"`
	Open     float64 `json:"open"`
	High     float64 `json:"high"`
	Low      float64 `json:"low"`
	Close    float64 `json:"close"`
	PreClose float64 `json:"pre_close"`
	Volume   float64 `json:"volume"`
	Amount   float64 `json:"amount"`
	PctChg   float64 `json:"pct_chg"`
	Turnover float64 `json:"turnover"`
	VolRatio float64 `json:"vol_ratio"`
	PeTTM    float64 `json:"pe_ttm"`
	Pb       float64 `json:"pb"`
	TotalMv  float64 `json:"total_mv"`
	FloatMv  float64 `json:"float_mv"`
	IsST     bool    `json:"is_st"`
	Code     string  `json:"code"`
	Name     string  `json:"name"`
}

// MinuteKBar 对应服务端 "分钟k" 记录（1 分钟粒度，date 为 14 位 YYYYMMDDHHMMSS）。
type MinuteKBar struct {
	Date   int64   `json:"date"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
	Amount float64 `json:"amount"`
	Code   string  `json:"code"`
}

// BuildTimeExpr 构造时间键表达式，对照 stock_sdk._build_time_query。
// "N" 表示最新记录。
func BuildTimeExpr(start, end string, desc bool) string {
	if start == "" && end == "" {
		return "*"
	}
	if start != "" && (start == end || (end == "" && !desc)) {
		return start
	}
	op := ">"
	if desc {
		op = "<"
	}
	if start == "" {
		start = "N"
	}
	if end == "" {
		end = "N"
	}
	return start + op + end
}

// PadMinuteTime 把 8 位日期补全为 14 位分钟K时间戳。
func PadMinuteTime(s string, isEnd bool) string {
	if len(s) != 8 {
		return s
	}
	if isEnd {
		return s + "235959"
	}
	return s + "000000"
}

// QueryDayK 查询日K，按服务端返回顺序（LevelDB 天然按日期升序）。
func (c *Client) QueryDayK(ctx context.Context, code, start, end string, desc bool) ([]DayKBar, error) {
	raw, err := c.Get(ctx, "日k:"+code+":"+BuildTimeExpr(start, end, desc))
	if err != nil {
		return nil, err
	}
	vals, err := decodeValues(raw)
	if err != nil {
		return nil, err
	}
	bars := make([]DayKBar, 0, len(vals))
	for _, v := range vals {
		var b DayKBar
		if err := json.Unmarshal(v, &b); err != nil || b.Date == 0 {
			continue
		}
		bars = append(bars, b)
	}
	return bars, nil
}

// QueryMinuteK 查询分钟K；8 位日期自动补全为当天完整交易时段。
func (c *Client) QueryMinuteK(ctx context.Context, code, start, end string, desc bool) ([]MinuteKBar, error) {
	raw, err := c.Get(ctx, "分钟k:"+code+":"+BuildTimeExpr(PadMinuteTime(start, false), PadMinuteTime(end, true), desc))
	if err != nil {
		return nil, err
	}
	vals, err := decodeValues(raw)
	if err != nil {
		return nil, err
	}
	bars := make([]MinuteKBar, 0, len(vals))
	for _, v := range vals {
		var b MinuteKBar
		if err := json.Unmarshal(v, &b); err != nil || b.Date == 0 {
			continue
		}
		bars = append(bars, b)
	}
	return bars, nil
}
