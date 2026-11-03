package scorer

import (
	"fmt"
	"math"
	"strings"

	"go-stock/backend/models"
)

// 触发一票否决的事件类型
var eventVetoTypes = map[string]bool{"大股东减持": true, "业绩暴雷": true}

// ScoreEvent 事件得分 = 50 + Σ 事件分，截断 [-100,100]
func ScoreEvent(events []models.KhunterEvent, stockName string) DimScore {
	if strings.HasPrefix(stockName, "ST") || strings.HasPrefix(stockName, "*ST") {
		return DimScore{Score: -100, Veto: true, Reason: "ST/*ST 股票"}
	}
	score := 50.0
	eventSummaries := make([]string, 0, len(events))
	for _, e := range events {
		score += e.Score
		if eventVetoTypes[e.EventType] {
			when := ""
			if e.EventDate != "" {
				when = e.EventDate + "，"
			}
			return DimScore{Score: -100, Veto: true, Reason: fmt.Sprintf("%s（%s%+.0f分），一票否决。", e.EventType, when, e.Score)}
		}
		summary := fmt.Sprintf("%s %+.0f分", e.EventType, e.Score)
		if e.EventDate != "" {
			summary += "(" + e.EventDate + ")"
		}
		if e.ExpireDate != "" {
			summary += "，有效至" + e.ExpireDate
		}
		eventSummaries = append(eventSummaries, summary)
	}
	score = math.Max(-100, math.Min(100, score))
	detail := map[string]any{"event_count": len(events), "events": eventSummaries}
	reason := "核查期内没有活跃事件，事件分维持基准 50。"
	if len(events) > 0 {
		reason = fmt.Sprintf("活跃事件 %d 条：%s。基准 50 分后合计 %.0f。", len(events), strings.Join(eventSummaries, "、"), score)
	}
	return DimScore{Score: score, Reason: reason, Detail: detail}
}
