package scorer

// SectorInput 单个板块输入
type SectorInput struct {
	Name      string
	PctRank   int   // 当日涨幅全市场排名（0=未知，不给排名分）
	NetInflow int64 // 当日主力净流入（元）
}

const (
	sectorBase       = 50.0
	sectorRankTopN   = 20
	sectorFlowBigIn  = 1e8
	sectorFlowBigOut = -1e8
)

// ScoreSector 板块得分 = 50 + 排名分 + 资金流分；个股取所属板块 MAX
func ScoreSector(sectors []SectorInput) DimScore {
	if len(sectors) == 0 {
		return DimScore{Score: sectorBase, Degraded: true, Reason: "无板块归属数据"}
	}
	best := -1000.0
	bestName := ""
	for _, s := range sectors {
		sc := sectorBase
		if s.PctRank > 0 && s.PctRank <= sectorRankTopN {
			sc += 50
		}
		if s.NetInflow > sectorFlowBigIn {
			sc += 50
		} else if s.NetInflow <= sectorFlowBigOut {
			sc -= 50
		}
		if sc > best {
			best, bestName = sc, s.Name
		}
	}
	return DimScore{Score: best, Detail: map[string]any{"best_sector": bestName}}
}
