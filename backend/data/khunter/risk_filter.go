package khunter

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"go-stock/backend/data/indicator"
	"go-stock/backend/data/khunter/risk"
	"go-stock/backend/models"
)

// RiskMetrics 事前风险画像：信号日与每个拟入场日都重新计算，只读不写形态/评分。
type RiskMetrics struct {
	FiveDayReturn  float64 `json:"fiveDayReturn"`  // 近5日涨幅 = T 收盘 / T-5 收盘 - 1
	MA20Deviation  float64 `json:"ma20Deviation"`  // MA20 乖离 = 收盘 / MA20 - 1
	ATR14          float64 `json:"atr14"`          // Wilder ATR(指标包口径，周期取 cfg.ATRPeriod)
	ATRRatio       float64 `json:"atrRatio"`       // ATR / 收盘价
	MedianAmount20 float64 `json:"medianAmount20"` // 最近 20 根有效 K 线成交额中位数（元）
}

// RiskDecision 风险过滤结论：只输出否决原因，不修改形态、不修改评分权重。
type RiskDecision struct {
	AllowConfirm bool        `json:"allowConfirm"`
	Reasons      []string    `json:"reasons,omitempty"`
	Metrics      RiskMetrics `json:"metrics"`
}

// RiskHistoryEntry 单日风险结论（信号日 + 每个拟入场日各一条，追加进 RiskHistoryJSON）。
type RiskHistoryEntry struct {
	Date     string       `json:"date"`
	Decision RiskDecision `json:"decision"`
}

// medianValidBars 近20日成交额中位数与 MA20 的样本窗口（有效 K 线根数）。
const medianValidBars = 20

// BuildRiskMetrics 由截至当日的本地日 K 计算风险画像。
// 只统计通过 ValidateDailyBar 的有效 K 线（单位齐全、量额为正、OHLC 正常）：
//   - FiveDayReturn = T 收盘 / T-5 收盘 - 1（T 为最后一根有效 K 线）；
//   - MA20Deviation = 收盘 / MA20 - 1；
//   - ATR14/ATRRatio 用现有指标包的 Wilder ATR(cfg.ATRPeriod)；
//   - MedianAmount20 取最近 20 根有效 K 线成交额中位数。
//
// 有效 K 线不足 20 根返回错误，调用方必须禁止确认。
func BuildRiskMetrics(bars []models.KLineBar, cfg RiskControlConfig) (RiskMetrics, error) {
	valid := make([]models.KLineBar, 0, len(bars))
	for _, b := range bars {
		if len(ValidateDailyBar(b)) == 0 {
			valid = append(valid, b)
		}
	}
	if len(valid) < medianValidBars {
		return RiskMetrics{}, fmt.Errorf("有效K线不足 %d 根（实际 %d 根），禁止确认", medianValidBars, len(valid))
	}
	n := len(valid)
	last := valid[n-1]
	closes := make([]float64, n)
	highs := make([]float64, n)
	lows := make([]float64, n)
	amounts := make([]float64, 0, medianValidBars)
	for i, b := range valid {
		closes[i], highs[i], lows[i] = b.Close, b.High, b.Low
		if i >= n-medianValidBars {
			amounts = append(amounts, b.Amount)
		}
	}
	m := RiskMetrics{
		FiveDayReturn:  last.Close/closes[n-1-5] - 1,
		MedianAmount20: medianFloat64(amounts),
	}
	var ma20 float64
	for _, c := range closes[n-medianValidBars:] {
		ma20 += c
	}
	ma20 /= float64(medianValidBars)
	if ma20 > 0 {
		m.MA20Deviation = last.Close/ma20 - 1
	}
	if v, ok := indicator.LastValid(indicator.ATR(highs, lows, closes, cfg.ATRPeriod)); ok {
		m.ATR14 = v
	}
	if last.Close > 0 {
		m.ATRRatio = m.ATR14 / last.Close
	}
	return m, nil
}

// EvaluateRisk 纯风险判断：数据降级、四类阈值越界、市场风险档禁止新增任一命中即否决。
// 阈值全部来自配置，不修改形态、不修改五维权重。
func EvaluateRisk(m RiskMetrics, data DataDecision, market risk.Level, cfg RiskControlConfig) RiskDecision {
	d := RiskDecision{Metrics: m}
	if !data.AllowConfirm {
		d.Reasons = append(d.Reasons, "数据不允许确认")
	}
	if m.FiveDayReturn > cfg.MaxFiveDayReturn {
		d.Reasons = append(d.Reasons, "近5日涨幅过热")
	}
	if m.MA20Deviation > cfg.MaxMA20Deviation {
		d.Reasons = append(d.Reasons, "MA20正向乖离过大")
	}
	if m.ATRRatio > cfg.MaxATRRatio {
		d.Reasons = append(d.Reasons, "ATR波动过高")
	}
	if m.MedianAmount20 < cfg.MinMedianAmount20 {
		d.Reasons = append(d.Reasons, "流动性不足")
	}
	if market.PositionLimit <= 0 {
		d.Reasons = append(d.Reasons, "市场风险档禁止新增")
	}
	d.AllowConfirm = len(d.Reasons) == 0
	return d
}

// AppendRiskHistory 把单日风险结论合并进 RiskHistoryJSON：
// 逐日追加、同日重跑覆盖该日记录不重复；信号日失败、入场日恢复及中间尝试都留痕。
func AppendRiskHistory(historyJSON, date string, d RiskDecision) string {
	var rows []RiskHistoryEntry
	if strings.TrimSpace(historyJSON) != "" {
		if json.Unmarshal([]byte(historyJSON), &rows) != nil {
			rows = nil
		}
	}
	replaced := false
	for i := range rows {
		if rows[i].Date == date {
			rows[i].Decision = d
			replaced = true
			break
		}
	}
	if !replaced {
		rows = append(rows, RiskHistoryEntry{Date: date, Decision: d})
	}
	b, err := json.Marshal(rows)
	if err != nil {
		return historyJSON
	}
	return string(b)
}

// AttachSignalRisk 计算信号日风险结论并写入观察记录：
// SignalRiskJSON 存当日结论、RiskHistoryJSON 追加当日记录、否决原因并入 RejectReason。
// 有效 K 线不足 20 根时按错误否决确认（不再评估阈值）；确认与否决由确认引擎消费，
// 本函数不修改评分、形态或触发价。
func AttachSignalRisk(obs *models.KhunterObservation, bars []models.KLineBar, decision DataDecision, market risk.Level, cfg RiskControlConfig) RiskDecision {
	d := RiskDecision{}
	if m, err := BuildRiskMetrics(bars, cfg); err != nil {
		d.Reasons = append(d.Reasons, err.Error())
	} else {
		d = EvaluateRisk(m, decision, market, cfg)
	}
	if b, err := json.Marshal(d); err == nil {
		obs.SignalRiskJSON = string(b)
	}
	obs.RiskHistoryJSON = AppendRiskHistory(obs.RiskHistoryJSON, obs.SignalDate, d)
	obs.RejectReason = joinRejectReasons(obs.RejectReason, d.Reasons)
	return d
}

// joinRejectReasons 合并数据闸门与风险否决的拒绝原因并去重，保持"讲清楚"的完整原因列表。
func joinRejectReasons(existing string, add []string) string {
	seen := map[string]bool{}
	parts := make([]string, 0, len(add)+1)
	for _, r := range strings.Split(existing, "；") {
		if r = strings.TrimSpace(r); r == "" || seen[r] {
			continue
		}
		seen[r] = true
		parts = append(parts, r)
	}
	for _, r := range add {
		if seen[r] {
			continue
		}
		seen[r] = true
		parts = append(parts, r)
	}
	return strings.Join(parts, "；")
}

// medianFloat64 中位数：奇数取中间值，偶数取中间两根均值。
func medianFloat64(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}
