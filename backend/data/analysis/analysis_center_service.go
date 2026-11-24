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
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"gorm.io/gorm"

	"go-stock/backend/data"
	"go-stock/backend/data/datasource"
	"go-stock/backend/data/khunter"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/util/timeutil"
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

// PipelineStep 流水线步骤定义
type PipelineStep struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// PipelineStatus 流水线运行状态
type PipelineStatus struct {
	Running bool   `json:"running"`
	Step    string `json:"step"`
	Message string `json:"message"`
	Error   string `json:"error"`
}

// PipelineKlineBar K线数据点
type PipelineKlineBar struct {
	Date   string  `json:"date"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume int64   `json:"volume"`
}

// PipelineForecast AI预测摘要
type PipelineForecast struct {
	Direction  string  `json:"direction"`
	ChangePct  float64 `json:"changePct"`
	PredEnd    float64 `json:"predEnd"`
	PredHigh   float64 `json:"predHigh"`
	PredLow    float64 `json:"predLow"`
	Confidence float64 `json:"confidence"`
	PredLen    int     `json:"predLen"`
}

// PipelineTechDetail 技术面详情
type PipelineTechDetail struct {
	Trend    string `json:"trend"`
	Volume   string `json:"volume"`
	Position string `json:"position"`
	Momentum string `json:"momentum"`
	Sections []struct {
		Title string `json:"title"`
		Text  string `json:"text"`
	} `json:"sections"`
}

// PipelineFactorDetail 因子面详情
type PipelineFactorDetail struct {
	FactorScore    float64 `json:"factorScore"`
	Weight         float64 `json:"weight"`
	Contribution   float64 `json:"contribution"`
	Rank           int     `json:"rank"`
	Total          int     `json:"total"`
	DegradedReason string  `json:"degradedReason,omitempty"`
}

// PipelineAIDetail AI面详情
type PipelineAIDetail struct {
	Forecast      *PipelineForecast `json:"forecast,omitempty"`
	FactorScore   float64           `json:"factorScore"`
	Degraded      bool              `json:"degraded"`
	DegradedReason string           `json:"degradedReason,omitempty"`
}

// PipelineDetail 行内展开详情
type PipelineDetail struct {
	Code     string                `json:"code"`
	Name     string                `json:"name"`
	Tech     *PipelineTechDetail   `json:"tech,omitempty"`
	Factor   *PipelineFactorDetail `json:"factor,omitempty"`
	AI       *PipelineAIDetail     `json:"ai,omitempty"`
	KlineBars []PipelineKlineBar   `json:"klineBars,omitempty"`
}

// PipelineResult 流水线完整结果
type PipelineResult struct {
	TradeDate string         `json:"tradeDate"`
	Steps     []PipelineStep `json:"steps"`
	Rows      []RecommendRow `json:"rows"`
	Status    PipelineStatus `json:"status"`
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

// ===== 智能流水线 =====

var pipelineSteps = []PipelineStep{
	{Key: "data", Label: "数据获取", Description: "获取行情数据"},
	{Key: "scan", Label: "候选筛选", Description: "策略扫描与候选筛选"},
	{Key: "kronos", Label: "Kronos评分", Description: "Kronos 因子评分"},
	{Key: "predict", Label: "AI预测", Description: "AI 预测与排序"},
}

// GetPipelineSteps 返回流水线步骤定义
func (s *AnalysisCenterService) GetPipelineSteps() []PipelineStep {
	return pipelineSteps
}

var pipelineBusy atomic.Bool

// RunPipeline 触发流水线（异步）：立即返回，进度经 GetPipelineStatus 轮询。
// 后台执行 khunter 流水线（含第 8 步 Kronos 批量预测异步旁路），运行结果
// 落 KhunterRunSummary；失败信息随汇总透出，前端据此展示。
func (s *AnalysisCenterService) RunPipeline(tradeDate string) error {
	if db.Dao == nil {
		return errNoDB
	}
	if !pipelineBusy.CompareAndSwap(false, true) {
		return errors.New("流水线正在运行，请等待当前运行结束")
	}
	go func() {
		defer pipelineBusy.Store(false)
		// 失败已由 khunter 落 failed 运行汇总并打日志，错误经 GetPipelineStatus 透出
		_, _ = khunter.RunPipeline(context.Background(), tradeDate)
	}()
	return nil
}

// GetPipelineStatus 查询流水线状态：运行中返回 khunter 实时阶段（含 Kronos
// 异步旁路）；空闲时回读最近一次运行汇总。
func (s *AnalysisCenterService) GetPipelineStatus() (PipelineStatus, error) {
	if db.Dao == nil {
		return PipelineStatus{}, errNoDB
	}
	if khunter.IsRunning() {
		return PipelineStatus{
			Running: true,
			Step:    mapPipelineStep(khunter.CurrentStep()),
			Message: "流水线运行中",
		}, nil
	}
	var sum models.KhunterRunSummary
	err := db.Dao.Order("trade_date DESC").First(&sum).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return PipelineStatus{Running: false, Step: "", Message: "尚无运行记录"}, nil
		}
		return PipelineStatus{}, err
	}
	status := PipelineStatus{
		Running: false,
		Step:    "done",
		Message: fmt.Sprintf("最近运行: %s", sum.TradeDate),
	}
	if sum.Status == "failed" {
		status.Error = sum.Message
	}
	return status, nil
}

// mapPipelineStep khunter 内部阶段 → 展示步骤 key（数据获取/候选筛选/Kronos/AI预测）
func mapPipelineStep(khStep string) string {
	switch khStep {
	case "data":
		return "data"
	case "scan", "score", "observe":
		return "scan"
	case "kronos":
		return "kronos"
	default:
		return ""
	}
}

// GetPipelineResult 获取流水线结果（推荐列表 + 状态）
func (s *AnalysisCenterService) GetPipelineResult(tradeDate string, topN int) (PipelineResult, error) {
	if db.Dao == nil {
		return PipelineResult{}, errNoDB
	}
	rows, err := s.GetRecommendations(tradeDate, topN)
	if err != nil {
		return PipelineResult{}, err
	}
	status, err := s.GetPipelineStatus()
	if err != nil {
		return PipelineResult{}, err
	}
	td := tradeDate
	if td == "" {
		td = latestObservationDateOrEmpty()
	}
	return PipelineResult{
		TradeDate: td,
		Steps:     pipelineSteps,
		Rows:      rows,
		Status:    status,
	}, nil
}

// GetPipelineDetail 获取单只股票详情（懒加载）
func (s *AnalysisCenterService) GetPipelineDetail(code, signalDate string) (PipelineDetail, error) {
	if db.Dao == nil {
		return PipelineDetail{}, errNoDB
	}
	detail := PipelineDetail{Code: code}
	
	// 1. 技术画像
	techProfile, err := s.getTechProfile(code)
	if err == nil && techProfile != nil {
		detail.Tech = techProfile
	}
	
	// 2. Kronos 因子
	var pred models.KhunterKronosPrediction
	err = db.Dao.Where("code = ? AND signal_date = ?", code, signalDate).First(&pred).Error
	if err == nil {
		contribution := round2(pred.FactorScore * pred.Weight)
		detail.Factor = &PipelineFactorDetail{
			FactorScore:    pred.FactorScore,
			Weight:         pred.Weight,
			Contribution:   contribution,
			DegradedReason: pred.DegradedReason,
		}
		// 解析预测 JSON
		if pred.ForecastJSON != "" {
			var forecast data.KronosPrediction
			if json.Unmarshal([]byte(pred.ForecastJSON), &forecast) == nil && forecast.Summary != nil {
				detail.AI = &PipelineAIDetail{
					Forecast: &PipelineForecast{
						Direction:  forecast.Summary.Direction,
						ChangePct:  forecast.Summary.ChangePct,
						PredEnd:    forecast.Summary.PredEnd,
						PredHigh:   forecast.Summary.PredHigh,
						PredLow:    forecast.Summary.PredLow,
						Confidence: forecast.Summary.Confidence,
						PredLen:    forecast.Summary.PredLen,
					},
					FactorScore: pred.FactorScore,
				}
			}
		}
		if detail.AI == nil {
			detail.AI = &PipelineAIDetail{
				FactorScore:    pred.FactorScore,
				Degraded:       pred.DegradedReason != "",
				DegradedReason: pred.DegradedReason,
			}
		}
	}
	
	// 3. K线数据（近30天）
	bars, err := s.getRecentKlineBars(code, 30)
	if err == nil {
		detail.KlineBars = bars
	}
	
	// 4. 名称
	var obs models.KhunterObservation
	if err := db.Dao.Where("code = ? AND signal_date = ?", code, signalDate).First(&obs).Error; err == nil {
		detail.Name = obs.Name
	}
	
	return detail, nil
}

// getTechProfile 获取技术画像（复用 khunter.BuildTechProfile）
func (s *AnalysisCenterService) getTechProfile(code string) (*PipelineTechDetail, error) {
	// 查询近 60 根日K
	store := datasource.NewKLineStore()
	ctx := context.Background()
	endDate := timeutil.TodayStr()
	startDate := timeutil.DateStr(time.Now().AddDate(0, -3, 0))
	
	bars, err := store.QueryKLinesPreferAdjusted(ctx, code, "day", startDate, endDate)
	if err != nil || len(bars) == 0 {
		return nil, err
	}
	
	profile := khunter.BuildTechProfile(bars)
	if profile == nil {
		return nil, nil
	}
	
	// 从 map[string]any 中提取字段
	trend, _ := profile["trend"].(string)
	volume, _ := profile["volume"].(string)
	position, _ := profile["position"].(string)
	momentum, _ := profile["momentum"].(string)
	
	// 转换 sections
	sections := make([]struct {
		Title string `json:"title"`
		Text  string `json:"text"`
	}, 0)
	if secs, ok := profile["sections"].([]models.TechSection); ok {
		for _, sec := range secs {
			sections = append(sections, struct {
				Title string `json:"title"`
				Text  string `json:"text"`
			}{
				Title: sec.Title,
				Text:  sec.Text,
			})
		}
	}
	
	return &PipelineTechDetail{
		Trend:    trend,
		Volume:   volume,
		Position: position,
		Momentum: momentum,
		Sections: sections,
	}, nil
}

// getRecentKlineBars 获取最近 N 根日K
func (s *AnalysisCenterService) getRecentKlineBars(code string, limit int) ([]PipelineKlineBar, error) {
	store := datasource.NewKLineStore()
	ctx := context.Background()
	endDate := timeutil.TodayStr()
	startDate := timeutil.DateStr(time.Now().AddDate(0, -6, 0))
	
	bars, err := store.QueryKLinesPreferAdjusted(ctx, code, "day", startDate, endDate)
	if err != nil {
		return nil, err
	}
	
	// 取最后 limit 根
	if len(bars) > limit {
		bars = bars[len(bars)-limit:]
	}
	
	result := make([]PipelineKlineBar, 0, len(bars))
	for _, b := range bars {
		result = append(result, PipelineKlineBar{
			Date:   b.TradeDate,
			Open:   b.Open,
			High:   b.High,
			Low:    b.Low,
			Close:  b.Close,
			Volume: b.Volume,
		})
	}
	return result, nil
}

// latestObservationDateOrEmpty 最新观察日期（空串兜底）
func latestObservationDateOrEmpty() string {
	d, err := latestObservationDate()
	if err != nil {
		return ""
	}
	return d
}
