package scorer

import "fmt"

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
		return DimScore{Score: sectorBase, Degraded: true, Missing: true, Reason: "无板块归属数据"}
	}
	best := -1000.0
	bestName := ""
	var bestIn SectorInput
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
			best, bestName, bestIn = sc, s.Name, s
		}
	}
	return DimScore{Score: best, Reason: sectorReason(sectors, bestName, bestIn), Detail: map[string]any{"best_sector": bestName}}
}

func sectorReason(sectors []SectorInput, bestName string, best SectorInput) string {
	rank := "涨幅排名未知，不给排名分"
	if best.PctRank > 0 && best.PctRank <= sectorRankTopN {
		rank = fmt.Sprintf("涨幅排名第 %d（前 %d，加分）", best.PctRank, sectorRankTopN)
	} else if best.PctRank > sectorRankTopN {
		rank = fmt.Sprintf("涨幅排名第 %d，未进前 %d", best.PctRank, sectorRankTopN)
	}
	flow := "主力净流入未达到 ±1 亿的加减分阈值"
	switch {
	case best.NetInflow > sectorFlowBigIn:
		flow = "主力净流入超过 1 亿"
	case best.NetInflow <= sectorFlowBigOut:
		flow = "主力净流出超过 1 亿"
	}
	text := fmt.Sprintf("所属 %d 个板块，按最高分取「%s」。该板块%s，%s（净流入 %s）。",
		len(sectors), bestName, rank, flow, yuanText(float64(best.NetInflow)))
	if len(sectors) > 1 {
		names := make([]string, 0, len(sectors)-1)
		for _, s := range sectors {
			if s.Name == bestName {
				continue
			}
			names = append(names, s.Name)
		}
		if len(names) > 0 {
			text += "其余板块不计入：" + joinNames(names) + "。"
		}
	}
	return text
}

func joinNames(names []string) string {
	out := ""
	for i, n := range names {
		if i > 0 {
			out += "、"
		}
		out += n
	}
	return out
}
