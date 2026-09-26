package khunter

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/duke-git/lancet/v2/convertor"

	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// 事件规则：类型 → (分值, 有效天数)
var eventRules = map[string]struct {
	Score float64
	Days  int
}{
	"业绩预增":  {20, 20},
	"业绩略增":  {10, 20},
	"股东增持":  {20, 50},
	"大股东增持": {20, 50},
	"股票回购":  {10, 50},
	"业绩预减":  {-20, 20},
	"业绩略减":  {-10, 20},
	"股东减持":  {-30, 50},
	"大股东减持": {-30, 50},
	"业绩暴雷":  {-20, 20},
	"异常波动":  {-15, 10},
}

var forecastGrowthPctRe = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*%`)

// ClassifyAnnouncement 按标题关键词分类公告；不匹配返回 nil。
// 顺序敏感：先判暴雷/预减/略减，再判增持/减持的股东层级。
func ClassifyAnnouncement(title, noticeDate, code string) *models.KhunterEvent {
	if title == "" || noticeDate == "" {
		return nil
	}
	typ := ""
	switch {
	case strings.Contains(title, "异常波动"):
		typ = "异常波动"
	case strings.Contains(title, "首亏") || strings.Contains(title, "预亏") ||
		(strings.Contains(title, "预减") && strings.Contains(title, "80%")):
		typ = "业绩暴雷"
	case strings.Contains(title, "预减"):
		typ = "业绩预减"
	case strings.Contains(title, "略减"):
		typ = "业绩略减"
	case strings.Contains(title, "减持"):
		if strings.Contains(title, "控股股东") || strings.Contains(title, "实际控制人") ||
			strings.Contains(title, "5%以上") || strings.Contains(title, "大股东") {
			typ = "大股东减持"
		} else {
			typ = "股东减持"
		}
	case strings.Contains(title, "增持"):
		if strings.Contains(title, "控股股东") || strings.Contains(title, "实际控制人") {
			typ = "大股东增持"
		} else {
			typ = "股东增持"
		}
	case strings.Contains(title, "回购"):
		typ = "股票回购"
	case strings.Contains(title, "预增"):
		typ = "业绩预增"
	case strings.Contains(title, "略增"):
		typ = "业绩略增"
	case (strings.Contains(title, "业绩预告") || strings.Contains(title, "预告")) &&
		(strings.Contains(title, "增长") || strings.Contains(title, "上升")):
		typ = classifyForecastGrowth(title)
	}
	if typ == "" {
		return nil
	}
	rule := eventRules[typ]
	if len(noticeDate) < 10 {
		return nil
	}
	d, err := time.Parse("2006-01-02", noticeDate[:10])
	if err != nil {
		return nil
	}
	return &models.KhunterEvent{
		Code: code, EventType: typ, EventDate: d.Format("2006-01-02"),
		Score: rule.Score, ExpireDate: d.AddDate(0, 0, rule.Days).Format("2006-01-02"),
		Source: "announcement",
	}
}

// classifyForecastGrowth 业绩预告增长数值兜底：≥50% 业绩预增，<50% 业绩略增
func classifyForecastGrowth(title string) string {
	m := forecastGrowthPctRe.FindStringSubmatch(title)
	if len(m) < 2 {
		return ""
	}
	pct, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return ""
	}
	if pct >= 50 {
		return "业绩预增"
	}
	return "业绩略增"
}

// SyncEvents 拉取候选股公告并分类落库。300ms 间隔限流。
func SyncEvents(ctx context.Context, codes []string) (int, error) {
	api := data.NewMarketNewsApi()
	repo := NewRepo()
	total := 0
	for _, code := range codes {
		select {
		case <-ctx.Done():
			return total, ctx.Err()
		default:
		}
		notices := api.StockNotice(code)
		var events []models.KhunterEvent
		for _, item := range notices {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			title := convertor.ToString(m["title"])
			date := convertor.ToString(m["notice_date"])
			if ev := ClassifyAnnouncement(title, date, code); ev != nil {
				events = append(events, *ev)
			}
		}
		if len(events) > 0 {
			if err := repo.SaveEvents(events); err != nil {
				logger.SugaredLogger.Warnf("khunter 事件落库失败 %s: %v", code, err)
			} else {
				total += len(events)
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	return total, nil
}
