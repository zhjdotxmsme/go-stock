package scorer

import "go-stock/backend/models"

// 五维权重（KHunter 代码实际生效值）
const (
	WTechnical   = 0.35
	WMoneyflow   = 0.35
	WFundamental = 0.10
	WSector      = 0.10
	WEvent       = 0.10
)

// FiveDimInput 五维评分输入
type FiveDimInput struct {
	Hits      []Hit
	MoneyFlow []models.KhunterMoneyFlowDaily
	Finance   FundamentalInput
	Sectors   []SectorInput
	Events    []models.KhunterEvent
	StockName string
}

// Result 综合评分结果
type Result struct {
	Technical   float64
	Moneyflow   float64
	Fundamental float64
	Sector      float64
	Event       float64
	Total       float64
	Level       string
	VetoReason  string
	Degraded    bool
	// Explain 逐维解释：score/missing/reason/detail，落库 details 供前端展示
	Explain map[string]any
}

// dimExplain 单维解释快照：得分/缺失标记/原因/明细
func dimExplain(d DimScore) map[string]any {
	out := map[string]any{
		"score":    d.Score,
		"missing":  d.Missing,
		"degraded": d.Degraded,
	}
	if d.Reason != "" {
		out["reason"] = d.Reason
	}
	for k, v := range d.Detail {
		if k == "score" {
			continue
		}
		out[k] = v
	}
	return out
}

// LevelOf 六档评级
func LevelOf(total float64) string {
	switch {
	case total == -100:
		return "淘汰"
	case total >= 80:
		return "强烈推荐"
	case total >= 60:
		return "推荐"
	case total >= 40:
		return "中性"
	case total >= 20:
		return "谨慎"
	default:
		return "回避"
	}
}

// Score 综合评分：任一维度否决 → Total=-100（淘汰）
func Score(in FiveDimInput) Result {
	tech := TechnicalScorer{}.Score(in.Hits)
	mf := ScoreMoneyFlow(in.MoneyFlow)
	fund := ScoreFundamental(in.Finance)
	sector := ScoreSector(in.Sectors)
	event := ScoreEvent(in.Events, in.StockName)

	r := Result{
		Technical: tech.Score, Moneyflow: mf.Score, Fundamental: fund.Score,
		Sector: sector.Score, Event: event.Score,
		Degraded: tech.Degraded || mf.Degraded || fund.Degraded || sector.Degraded || event.Degraded,
	}
	r.Explain = map[string]any{
		"technical":   dimExplain(tech),
		"moneyflow":   dimExplain(mf),
		"fundamental": dimExplain(fund),
		"sector":      dimExplain(sector),
		"event":       dimExplain(event),
	}
	for _, d := range []DimScore{tech, mf, fund, sector, event} {
		if d.Veto {
			r.Total = -100
			r.Level = "淘汰"
			r.VetoReason = d.Reason
			return r
		}
	}
	r.Total = tech.Score*WTechnical + mf.Score*WMoneyflow + fund.Score*WFundamental +
		sector.Score*WSector + event.Score*WEvent
	r.Level = LevelOf(r.Total)
	return r
}
