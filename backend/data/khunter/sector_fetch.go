package khunter

import (
	"encoding/json"
	"strings"

	"go-stock/backend/data/khunter/scorer"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/stockcode"
)

// fetchSectorInputs 装配个股板块输入：行业+概念归属 × 对应板块最新资金流快照
func fetchSectorInputs(code string) []scorer.SectorInput {
	// all_stock_info.sec_uri_tycode 存 6 位裸代码（gorm 对 SECURITYCODE 的列命名），
	// 入参是内部格式（sz300780），需取纯代码，否则永远查空导致板块维度恒退化 50 分
	pure := stockcode.PureCode(code)
	if pure == "" {
		pure = code
	}
	var info models.AllStockInfo
	if err := db.Dao.Where("sec_uri_tycode = ?", pure).First(&info).Error; err != nil {
		return nil
	}
	names := []string{}
	if info.INDUSTRY != "" {
		names = append(names, info.INDUSTRY)
	}
	// CONCEPT 列存 JSON 数组字符串（如 ["工业母机","机器人概念"]），
	// 先按 JSON 解析；非 JSON 时回退逗号分隔，避免板块名带引号括号
	var concepts []string
	if err := json.Unmarshal([]byte(info.CONCEPT), &concepts); err != nil {
		concepts = strings.Split(info.CONCEPT, ",")
	}
	for _, c := range concepts {
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
