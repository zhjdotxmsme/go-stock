package khunter

import (
	"encoding/json"
	"fmt"
	"strings"

	"go-stock/backend/data/khunter/scorer"
	"go-stock/backend/db"
	"go-stock/backend/logger"
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

// fetchAndSnapshotSectorInputs 实时模式专用：装配板块输入先落时点快照再返回评分输入。
// 落库失败只记日志，不阻断评分。
func fetchAndSnapshotSectorInputs(repo *Repo, code, availableDate string) []scorer.SectorInput {
	sectors := fetchSectorInputs(code)
	payload, err := json.Marshal(sectors)
	if err != nil {
		logger.SugaredLogger.Warnf("khunter 板块快照序列化失败 %s: %v", code, err)
		return sectors
	}
	row := models.KhunterSectorSnapshot{Code: code, AvailableDate: availableDate, Payload: string(payload)}
	if err := repo.UpsertSectorSnapshot(&row); err != nil {
		logger.SugaredLogger.Warnf("khunter 板块快照落库失败 %s@%s: %v", code, availableDate, err)
	}
	return sectors
}

// loadSectorInputsAsOf 研究模式读取时点板块快照并解码；
// 缺快照（或快照为空）返回 ErrSnapshotUnavailable，禁止调用最新接口补缺。
func loadSectorInputsAsOf(repo *Repo, code, asOfDate string) ([]scorer.SectorInput, error) {
	row, err := repo.GetSectorSnapshotAsOf(code, asOfDate)
	if err != nil {
		return nil, err
	}
	var sectors []scorer.SectorInput
	if err := json.Unmarshal([]byte(row.Payload), &sectors); err != nil {
		return nil, fmt.Errorf("板块快照解析失败 %s@%s: %w", code, row.AvailableDate, err)
	}
	if len(sectors) == 0 {
		return nil, fmt.Errorf("%w: %s@%s 板块快照为空", ErrSnapshotUnavailable, code, row.AvailableDate)
	}
	return sectors, nil
}
