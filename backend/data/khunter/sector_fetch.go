package khunter

import (
	"strings"

	"go-stock/backend/data/khunter/scorer"
	"go-stock/backend/db"
	"go-stock/backend/models"
)

// fetchSectorInputs 装配个股板块输入：行业+概念归属 × 对应板块最新资金流快照
func fetchSectorInputs(code string) []scorer.SectorInput {
	var info models.AllStockInfo
	if err := db.Dao.Where("sec_uri_tycode = ?", code).First(&info).Error; err != nil {
		return nil
	}
	names := []string{}
	if info.INDUSTRY != "" {
		names = append(names, info.INDUSTRY)
	}
	for _, c := range strings.Split(info.CONCEPT, ",") {
		if c = strings.TrimSpace(c); c != "" {
			names = append(names, c)
		}
	}
	var out []scorer.SectorInput
	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] {
			continue
		}
		seen[name] = true
		in := scorer.SectorInput{Name: name, PctRank: 0}
		var bk models.BKFundFlow
		if err := db.Dao.Where("name = ?", name).Order("snap_time DESC").First(&bk).Error; err == nil {
			in.NetInflow = bk.NetInflow
		} else {
			var cf models.ConceptFundFlow
			if err := db.Dao.Where("name = ?", name).Order("snap_time DESC").First(&cf).Error; err == nil {
				in.NetInflow = cf.NetInflow
			}
		}
		out = append(out, in)
	}
	return out
}
