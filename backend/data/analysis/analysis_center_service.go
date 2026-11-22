package analysis

// 统一分析中心门面：选股预测族的唯一后端入口。
// 读 khunter 的观察/运行汇总表与 Kronos 因子明细表，写出预测任务；
// 不修改 khunter 引擎语义，不参与五维评分。
//
// 独立成 analysis 子包而非放 data 根包：khunter 已 import data（预测任务
// 注册表依赖 data.PredictKLine），门面再 import khunter 会成环。

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"gorm.io/gorm"

	"go-stock/backend/data"
	"go-stock/backend/data/khunter"
	"go-stock/backend/db"
	"go-stock/backend/models"
)

type RecommendRow struct {
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	SignalDate     string  `json:"signalDate"`
	Status         string  `json:"status"`
	DataStatus     string  `json:"dataStatus"`
	RejectReason   string  `json:"rejectReason,omitempty"`
	Score          float64 `json:"score"`
	FactorScore    float64 `json:"factorScore,omitempty"`
	FactorWeight   float64 `json:"factorWeight,omitempty"`
	BlendedScore   float64 `json:"blendedScore"`
	HasFactor      bool    `json:"hasFactor"`
	Degraded       bool    `json:"degraded"`
	DegradedReason string  `json:"degradedReason,omitempty"`
}

type AnalysisCenterStatus struct {
	KronosStatus        string                    `json:"kronosStatus"`
	LastRun             *models.KhunterRunSummary `json:"lastRun,omitempty"`
	PendingObservations int                       `json:"pendingObservations"`
	FactorEnabled       bool                      `json:"factorEnabled"`
}

type AnalysisCenterService struct {
	Jobs *khunter.PredictionJobRegistry
}

func NewAnalysisCenterService() *AnalysisCenterService {
	return &AnalysisCenterService{Jobs: khunter.NewPredictionJobRegistry()}
}

// errNoDB 门面在无数据库（移动端/启动早期）时的统一错误，禁止用空列表冒充。
var errNoDB = errors.New("分析中心：数据库未初始化")

// GetRecommendations 推荐视图：观察中/已确认候选，按混分降序。
// 只有未降级的因子明细参与混分；降级只透出原因（HasFactor=false）。
func (s *AnalysisCenterService) GetRecommendations(tradeDate string, topN int) ([]RecommendRow, error) {
	if db.Dao == nil {
		return nil, errNoDB
	}
	if topN <= 0 || topN > 100 {
		topN = 30
	}
	q := db.Dao.Model(&models.KhunterObservation{}).
		Where("status IN ?", []string{khunter.ObsStatusPending, khunter.ObsStatusConfirmed})
	if strings.TrimSpace(tradeDate) != "" {
		q = q.Where("signal_date = ?", tradeDate)
	} else if latest, err := latestObservationDate(); err != nil {
		return nil, err
	} else if latest != "" {
		q = q.Where("signal_date = ?", latest)
	}
	var obs []models.KhunterObservation
	if err := q.Order("score DESC, code").Limit(topN * 3).Find(&obs).Error; err != nil {
		return nil, err
	}
	if len(obs) == 0 {
		return []RecommendRow{}, nil
	}

	// 因子明细：只取与本批观察同 (code, signal_date) 的记录
	keys := make([]string, 0, len(obs))
	for _, o := range obs {
		keys = append(keys, o.Code+"|"+o.SignalDate)
	}
	var preds []models.KhunterKronosPrediction
	if err := db.Dao.Where("code || '|' || signal_date IN ?", keys).Find(&preds).Error; err != nil {
		// 因子表查询失败不阻断推荐主视图，按无因子降级透出
		preds = nil
	}
	byKey := make(map[string]models.KhunterKronosPrediction, len(preds))
	for _, p := range preds {
		byKey[p.Code+"|"+p.SignalDate] = p
	}

	rows := make([]RecommendRow, 0, len(obs))
	for _, o := range obs {
		row := RecommendRow{
			Code: o.Code, Name: o.Name, SignalDate: o.SignalDate, Status: o.Status,
			DataStatus: o.DataStatus, RejectReason: o.RejectReason,
			Score: o.Score, BlendedScore: o.Score,
		}
		if p, ok := byKey[o.Code+"|"+o.SignalDate]; ok {
			if p.DegradedReason != "" {
				row.Degraded = true
				row.DegradedReason = p.DegradedReason
			} else if p.Weight > 0 {
				row.HasFactor = true
				row.FactorScore = p.FactorScore
				row.FactorWeight = p.Weight
				row.BlendedScore = round2(o.Score*(1-p.Weight) + p.FactorScore*p.Weight)
			}
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].BlendedScore != rows[j].BlendedScore {
			return rows[i].BlendedScore > rows[j].BlendedScore
		}
		return rows[i].Code < rows[j].Code
	})
	if len(rows) > topN {
		rows = rows[:topN]
	}
	return rows, nil
}

// GetForecast 返回候选落库的完整预测明细（推荐视图展开曲线）；无明细返回空串。
func (s *AnalysisCenterService) GetForecast(code, signalDate string) (string, error) {
	if db.Dao == nil {
		return "", errNoDB
	}
	var p models.KhunterKronosPrediction
	err := db.Dao.Where("code = ? AND signal_date = ?", code, signalDate).First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return p.ForecastJSON, nil
}

func (s *AnalysisCenterService) GetAnalysisStatus() (AnalysisCenterStatus, error) {
	if db.Dao == nil {
		return AnalysisCenterStatus{}, errNoDB
	}
	st := AnalysisCenterStatus{
		KronosStatus:  data.KronosServiceStatus(),
		FactorEnabled: khunter.LoadKronosFactorConfig("").Enable,
	}
	var sum models.KhunterRunSummary
	if err := db.Dao.Order("trade_date DESC").First(&sum).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return st, err
		}
	} else {
		cp := sum
		st.LastRun = &cp
	}
	var n int64
	if err := db.Dao.Model(&models.KhunterObservation{}).
		Where("status = ?", khunter.ObsStatusPending).Count(&n).Error; err != nil {
		return st, err
	}
	st.PendingObservations = int(n)
	return st, nil
}

func (s *AnalysisCenterService) GetPredictJob(id string) (khunter.PredictJob, error) {
	return s.Jobs.Get(id)
}

func (s *AnalysisCenterService) SubmitPredictJob(code string, predLen int, asOfDate string) (khunter.PredictJob, error) {
	if strings.TrimSpace(code) == "" {
		return khunter.PredictJob{}, fmt.Errorf("股票代码不能为空")
	}
	return s.Jobs.Submit(context.Background(), code, predLen, asOfDate, data.PredictKLine)
}

// latestObservationDate 最新有观察的信号日（推荐视图默认日期）。
// MAX() 在空表返回 NULL，必须用 sql.NullString 接收，否则 Scan 到 string 会报错。
func latestObservationDate() (string, error) {
	var d sql.NullString
	err := db.Dao.Model(&models.KhunterObservation{}).
		Select("MAX(signal_date)").Scan(&d).Error
	if err != nil {
		return "", err
	}
	if !d.Valid {
		return "", nil
	}
	return d.String, nil
}

// round2 与前端 blendedScore 展示同口径（两位小数）。
func round2(v float64) float64 { return math.Round(v*100) / 100 }
