package khunter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go-stock/backend/data/khunter/scorer"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// ResearchRecordResult 研究记录 ResultJSON 的载荷：完整保留本次研究运行的
// 信号、评分与"假想入选"结果。研究模式不写任何生产表，这是唯一落库出口。
type ResearchRecordResult struct {
	Result          *PipelineResult         `json:"result"`
	Signals         []models.KhunterSignal  `json:"signals,omitempty"`
	Scores          []models.KhunterScore   `json:"scores,omitempty"`
	Hunted          []models.KhunterHunting `json:"hunted,omitempty"`
	NotBacktestable []string                `json:"notBacktestable,omitempty"`
	Error           string                  `json:"error,omitempty"`
}

func (r *Repo) UpsertResearchRecord(row *models.KhunterResearchRecord) error {
	return db.Dao.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "trade_date"}},
		UpdateAll: true,
	}).Create(row).Error
}

func (r *Repo) GetResearchRecord(tradeDate string) (*models.KhunterResearchRecord, error) {
	var row models.KhunterResearchRecord
	err := db.Dao.Where("trade_date = ?", tradeDate).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *Repo) GetResearchRecords(limit int) ([]models.KhunterResearchRecord, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	var rows []models.KhunterResearchRecord
	err := db.Dao.Order("trade_date DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

// runResearchPipeline 历史研究运行：与实时流水线同一套扫描和评分规则，但
//   - 只读时点输入：本地日K、资金流、事件均已带 asOfDate 上界；基本面/板块只读
//     available_date <= tradeDate 的快照，缺快照标记不可回测，绝不调最新接口补数；
//   - 不做资金流/事件同步（那会拉最新数据）；
//   - 只写 KhunterResearchRecord，不碰生产表和运行汇总。
func runResearchPipeline(ctx context.Context, repo *Repo, tradeDate string, now time.Time) (res *PipelineResult, err error) {
	res = &PipelineResult{TradeDate: tradeDate}
	// 历史股票清单无法完整重建：all_stock_info 只有仍在市股票，已退市标的缺失
	// 会让历史入选结果系统性偏乐观，研究结论必须带上幸存者偏差限制。
	res.Limitations = append(res.Limitations,
		"幸存者偏差：股票清单取自当前快照，已退市标的无法回补，历史结果可能偏乐观")

	stocks, err := loadStockList()
	if err != nil {
		logger.SugaredLogger.Warnf("khunter 研究运行读取股票清单失败: %v", err)
		res.Limitations = append(res.Limitations, "股票清单读取失败，本次研究基于空清单运行: "+err.Error())
		stocks = nil
		err = nil
	}

	// 1-2. 全市场扫描（纯本地时点日K），信号只留在内存
	signals, hits, lastBars, stat := scanStrategySignals(ctx, tradeDate, stocks)
	res.StockTotal = len(stocks)
	res.FreshStocks = len(lastBars)
	res.InsufficientHistory = stat.InsufficientHistory
	res.MissingTradeDate = stat.MissingTradeDate
	res.QueryFailed = stat.QueryFailed
	res.Signals = len(signals)
	logger.SugaredLogger.Infof("khunter 研究扫描完成: %s 当日有 bar %d/%d，信号 %d 条",
		tradeDate, len(lastBars), len(stocks), len(signals))

	names, candidates := buildCandidates(signals)
	res.Candidates = len(candidates)

	// 3. 风险档只算不落库：研究结果照样受当日风险阈值约束
	lv, _ := marketRisk(ctx, tradeDate, now)
	res.RiskLevel = lv.Name

	scores := make([]models.KhunterScore, 0, len(candidates))
	notBacktestable := []string{}
	for _, code := range candidates {
		flows, _ := repo.GetMoneyFlowAsOf(code, tradeDate, 5)
		events, evErr := repo.GetActiveEvents(code, tradeDate)
		// 只读时点快照：缺基本面或板块快照 → 标记不可回测，绝不用最新值替代
		fund, fundErr := loadFundamentalAsOf(repo, code, tradeDate)
		sectors, sectErr := loadSectorInputsAsOf(repo, code, tradeDate)
		if fundErr != nil || sectErr != nil {
			notBacktestable = append(notBacktestable, code)
		}
		in := scorer.FiveDimInput{
			Hits:      hits[code],
			MoneyFlow: flows,
			Finance:   fund,
			Sectors:   sectors,
			Events:    events,
			StockName: names[code],
		}
		r := scorer.Score(in)
		explain := r.Explain
		if explain == nil {
			explain = map[string]any{}
		}
		if tech, ok := explain["technical"].(map[string]any); ok {
			if bars := lastBars[code]; len(bars) > 0 {
				tech["profile"] = BuildTechProfile(bars)
			}
		}
		if evErr != nil {
			if ev, ok := explain["event"].(map[string]any); ok {
				ev["missing"] = true
				ev["reason"] = "事件查询失败: " + evErr.Error()
			}
		}
		if fundErr != nil {
			if f, ok := explain["fundamental"].(map[string]any); ok {
				f["missing"] = true
				f["notBacktestable"] = true
				f["reason"] = "缺 " + tradeDate + " 前的基本面时点快照，不可回测，禁止用最新数据替代"
			}
		}
		if sectErr != nil {
			if s, ok := explain["sector"].(map[string]any); ok {
				s["missing"] = true
				s["notBacktestable"] = true
				s["reason"] = "缺 " + tradeDate + " 前的板块时点快照，不可回测，禁止用最新数据替代"
			}
		}
		details, _ := json.Marshal(explain)
		scores = append(scores, models.KhunterScore{
			Code: code, Name: names[code], ScoreDate: tradeDate,
			Technical: r.Technical, Moneyflow: r.Moneyflow, Fundamental: r.Fundamental,
			Sector: r.Sector, Event: r.Event, Total: r.Total, Level: r.Level,
			VetoReason: r.VetoReason, Degraded: r.Degraded, Details: string(details),
		})
	}
	res.Scored = len(scores)
	res.NotBacktestable = notBacktestable

	// 7. 狩猎场筛选只在内存中标记"假想入选"，绝不写 KhunterHunting
	threshold := HuntingThreshold(lv)
	hunted := make([]models.KhunterHunting, 0)
	for _, sc := range scores {
		if sc.Total < threshold {
			continue
		}
		bars := lastBars[sc.Code]
		if len(bars) == 0 {
			continue
		}
		support := SupportPrice(bars)
		latest := bars[len(bars)-1].Close
		if latest < support*0.98 {
			continue
		}
		hunted = append(hunted, models.KhunterHunting{
			Code: sc.Code, Name: names[sc.Code], EnterDate: tradeDate,
			EnterScore: sc.Total, SupportPrice: support, Status: "追踪中",
		})
	}
	res.Hunted = len(hunted)

	payload := ResearchRecordResult{
		Result: res, Signals: signals, Scores: scores, Hunted: hunted,
		NotBacktestable: notBacktestable,
	}
	blob, mErr := json.Marshal(payload)
	if mErr != nil {
		return res, fmt.Errorf("研究记录序列化失败: %w", mErr)
	}
	row := &models.KhunterResearchRecord{
		TradeDate: tradeDate, Status: "success", ResultJSON: string(blob),
	}
	if err := repo.UpsertResearchRecord(row); err != nil {
		return res, fmt.Errorf("研究记录写入失败: %w", err)
	}
	logger.SugaredLogger.Infof("khunter 研究运行完成: %s 信号 %d 评分 %d 假想入选 %d 不可回测 %d",
		tradeDate, res.Signals, res.Scored, res.Hunted, len(notBacktestable))
	return res, nil
}
