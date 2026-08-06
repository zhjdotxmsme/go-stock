package ranking

import (
	"fmt"
	"strings"
)

// FormatCandidates 将候选池格式化为 LLM 可读文本（方案 §8.1 D2：30+ 字段）。
// 零值/空值的可选字段不输出，减少 token 占用。
func FormatCandidates(candidates []Candidate) string {
	var sb strings.Builder
	for i, c := range candidates {
		fmt.Fprintf(&sb, "候选 %d: %s %s（综合分 %.2f）\n", i+1, c.Code, c.Name, c.ScreenScore)
		writeQuoteLine(&sb, c)
		writeSectorLine(&sb, c)
		writeHeatLine(&sb, c)
		writeTechLine(&sb, c)
		writeFactorLine(&sb, c)
		if len(c.NewsTitles) > 0 {
			sb.WriteString("新闻: ")
			for j, title := range c.NewsTitles {
				fmt.Fprintf(&sb, "%d) %s ", j+1, title)
			}
			sb.WriteString("\n")
		}
		if c.FundamentalsCovered {
			sb.WriteString("基本面: 已覆盖\n")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// writeQuoteLine 行情与估值行。
func writeQuoteLine(sb *strings.Builder, c Candidate) {
	var parts []string
	if c.Price > 0 {
		parts = append(parts, fmt.Sprintf("价格 %.2f", c.Price))
	}
	if c.ChangePercent != 0 {
		parts = append(parts, fmt.Sprintf("涨跌幅 %+.2f%%", c.ChangePercent))
	}
	if c.Amount > 0 {
		parts = append(parts, fmt.Sprintf("成交额 %.1f亿", c.Amount/1e8))
	}
	if c.TurnoverRate > 0 {
		parts = append(parts, fmt.Sprintf("换手率 %.2f%%", c.TurnoverRate))
	}
	if c.VolumeRatio > 0 {
		parts = append(parts, fmt.Sprintf("量比 %.2f", c.VolumeRatio))
	}
	if c.TotalCap > 0 {
		parts = append(parts, fmt.Sprintf("总市值 %.0f亿", c.TotalCap/1e8))
	}
	if c.PE != 0 {
		parts = append(parts, fmt.Sprintf("PE %.1f", c.PE))
	}
	if c.PB != 0 {
		parts = append(parts, fmt.Sprintf("PB %.1f", c.PB))
	}
	if len(parts) > 0 {
		sb.WriteString("行情: " + strings.Join(parts, " | ") + "\n")
	}
}

// writeSectorLine 行业与概念行。
func writeSectorLine(sb *strings.Builder, c Candidate) {
	var parts []string
	if c.Industry != "" {
		industry := c.Industry
		if c.IndustryRank > 0 {
			industry += fmt.Sprintf("(行业排名 %d", c.IndustryRank)
			if c.IndustryChangePct != 0 {
				industry += fmt.Sprintf(", 行业涨跌 %+.2f%%", c.IndustryChangePct)
			}
			industry += ")"
		}
		parts = append(parts, "行业 "+industry)
	}
	if len(c.Concepts) > 0 {
		parts = append(parts, "概念 "+strings.Join(c.Concepts, "、"))
	}
	if len(parts) > 0 {
		sb.WriteString(strings.Join(parts, " | ") + "\n")
	}
}

// writeHeatLine 板块热度 6 维行。
func writeHeatLine(sb *strings.Builder, c Candidate) {
	if c.HeatLatest == 0 && c.HeatState == "" && c.HeatSummary == "" {
		return
	}
	heat := fmt.Sprintf("板块热度: 最新 %.0f", c.HeatLatest)
	if c.HeatTrend != 0 {
		heat += fmt.Sprintf(" 趋势 %+.0f", c.HeatTrend)
	}
	if c.HeatPersistenceDays > 0 {
		heat += fmt.Sprintf(" 持续 %d 天", c.HeatPersistenceDays)
	}
	if c.HeatCooling != 0 {
		heat += fmt.Sprintf(" 降温 %.0f", c.HeatCooling)
	}
	if c.HeatWatchCount > 0 {
		heat += fmt.Sprintf(" 观察 %d", c.HeatWatchCount)
	}
	if c.HeatState != "" {
		heat += " 状态 " + c.HeatState
	}
	sb.WriteString(heat + "\n")
	if c.HeatSummary != "" {
		sb.WriteString("热度摘要: " + c.HeatSummary + "\n")
	}
}

// writeTechLine 技术面行。
func writeTechLine(sb *strings.Builder, c Candidate) {
	var parts []string
	if c.ChangePct60 != 0 {
		parts = append(parts, fmt.Sprintf("60日涨跌 %+.1f%%", c.ChangePct60))
	}
	if c.SignalScore > 0 {
		parts = append(parts, fmt.Sprintf("信号分 %.0f", c.SignalScore))
	}
	if c.MACDState != "" {
		parts = append(parts, "MACD "+c.MACDState)
	}
	if c.RSIState != "" {
		parts = append(parts, "RSI "+c.RSIState)
	}
	if c.BreakoutPct != 0 {
		parts = append(parts, fmt.Sprintf("突破 %+.1f%%", c.BreakoutPct))
	}
	if c.AmplitudePct != 0 {
		parts = append(parts, fmt.Sprintf("振幅 %.1f%%", c.AmplitudePct))
	}
	if c.VolumeRatio20 > 0 {
		parts = append(parts, fmt.Sprintf("20日量比 %.2f", c.VolumeRatio20))
	}
	if c.BodyPct != 0 {
		parts = append(parts, fmt.Sprintf("实体 %+.1f%%", c.BodyPct))
	}
	if c.PullbackMA20 {
		parts = append(parts, "回踩MA20")
	}
	if c.ConsolidationDays > 0 {
		parts = append(parts, fmt.Sprintf("盘整 %d 天", c.ConsolidationDays))
	}
	if c.Volatility > 0 {
		parts = append(parts, fmt.Sprintf("波动率 %.0f%%", c.Volatility))
	}
	if c.MaxDrawdown < 0 {
		parts = append(parts, fmt.Sprintf("回撤 %.1f%%", c.MaxDrawdown))
	}
	if c.ATR > 0 {
		parts = append(parts, fmt.Sprintf("ATR %.2f", c.ATR))
	}
	if len(parts) > 0 {
		sb.WriteString("技术面: " + strings.Join(parts, " ") + "\n")
	}
}

// writeFactorLine D1 九因子评分行。
func writeFactorLine(sb *strings.Builder, c Candidate) {
	if len(c.FactorScores) == 0 {
		return
	}
	parts := make([]string, 0, len(c.FactorScores))
	// 固定输出顺序，保证 prompt 稳定
	for _, name := range []string{"value", "liquidity", "momentum", "reversal", "activity", "stability", "size", "theme_heat", "topic_alignment"} {
		if score, ok := c.FactorScores[name]; ok {
			parts = append(parts, fmt.Sprintf("%s %.0f", name, score))
		}
	}
	for name, score := range c.FactorScores {
		found := false
		for _, known := range []string{"value", "liquidity", "momentum", "reversal", "activity", "stability", "size", "theme_heat", "topic_alignment"} {
			if name == known {
				found = true
				break
			}
		}
		if !found {
			parts = append(parts, fmt.Sprintf("%s %.0f", name, score))
		}
	}
	sb.WriteString("因子评分: " + strings.Join(parts, " ") + "\n")
}
