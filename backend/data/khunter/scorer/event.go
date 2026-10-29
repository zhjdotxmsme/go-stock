package scorer

import (
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
			return DimScore{Score: -100, Veto: true, Reason: e.EventType}
		}
		summary := e.EventType
		if e.EventDate != "" {
			summary += "(" + e.EventDate + ")"
		}
		eventSummaries = append(eventSummaries, summary)
	}
	detail := map[string]any{"event_count": len(events), "events": eventSummaries}
	reason := "核查期内无活跃事件"
	if len(events) > 0 {
		reason = "存在活跃事件"
	}
	return DimScore{Score: math.Max(-100, math.Min(100, score)), Reason: reason, Detail: detail}
}
