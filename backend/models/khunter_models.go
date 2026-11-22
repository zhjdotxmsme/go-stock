package models

import "time"

// KhunterSignal 形态策略信号
type KhunterSignal struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	Code        string    `gorm:"size:20;index:idx_khs_code_date" json:"code"`
	Name        string    `gorm:"size:50" json:"name"`
	Strategy    string    `gorm:"size:50;index:idx_khs_strategy_date" json:"strategy"`
	SignalDate  string    `gorm:"size:10;index:idx_khs_code_date;index:idx_khs_strategy_date" json:"signalDate"`
	KeyDate     string    `gorm:"size:10" json:"keyDate"`
	KeyDateType string    `gorm:"size:20" json:"keyDateType"`
	Close       float64   `json:"close"`
	VolumeRatio float64   `json:"volumeRatio"`
	Reasons     string    `gorm:"type:text" json:"reasons"` // JSON array
	Details     string    `gorm:"type:text" json:"details"` // JSON object
	CreatedAt   time.Time `json:"createdAt"`
}

func (KhunterSignal) TableName() string { return "khunter_signals" }

// KhunterScore 五维评分
type KhunterScore struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	Code        string    `gorm:"size:20;index:idx_khsc_code_date,unique" json:"code"`
	Name        string    `gorm:"size:50" json:"name"`
	ScoreDate   string    `gorm:"size:10;index:idx_khsc_code_date,unique" json:"scoreDate"`
	Technical   float64   `json:"technical"`
	Moneyflow   float64   `json:"moneyflow"`
	Fundamental float64   `json:"fundamental"`
	Sector      float64   `json:"sector"`
	Event       float64   `json:"event"`
	Total       float64   `json:"total"`
	Level       string    `gorm:"size:20" json:"level"` // 强烈推荐/推荐/中性/谨慎/回避/淘汰
	VetoReason  string    `gorm:"size:200" json:"vetoReason"`
	Degraded    bool      `json:"degraded"` // 某维度数据缺失按中性分处理
	Details     string    `gorm:"type:text" json:"details"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (KhunterScore) TableName() string { return "khunter_scores" }

// KhunterHunting 狩猎池交易记录：只有确认成功才入池。新状态只允许 持仓中 → 已退出；
// 旧版本记录统一迁移为 历史待核（不是真实持仓，不得伪装成可执行仓位）。
// 唯一键 (code, signal_date) 与 (code, enter_date)：同一股票同一信号日只有一条记录，
// 同一股票同一确认日只保留一条入池记录。
type KhunterHunting struct {
	ID               uint      `gorm:"primarykey" json:"id"`
	Code             string    `gorm:"size:20;uniqueIndex:idx_khh_code_signal,priority:1;uniqueIndex:idx_khh_code_enter,priority:1" json:"code"`
	Name             string    `gorm:"size:50" json:"name"`
	SignalDate       string    `gorm:"size:10;uniqueIndex:idx_khh_code_signal,priority:2" json:"signalDate"`
	EnterDate        string    `gorm:"size:10;uniqueIndex:idx_khh_code_enter,priority:2" json:"enterDate"`
	EnterScore       float64   `json:"enterScore"`
	SupportPrice     float64   `json:"supportPrice"`
	EntryPrice       float64   `json:"entryPrice"` // 真实确认成交价
	InitialStopPrice float64   `json:"initialStopPrice"`
	ProtectPrice     float64   `json:"protectPrice"`
	InitialRisk      float64   `json:"initialRisk"`
	Shares           int       `json:"shares"`
	Status           string    `gorm:"size:10" json:"status"` // 持仓中 / 已退出 / 历史待核
	TrackDays        int       `json:"trackDays"`
	ExitDate         string    `gorm:"size:10" json:"exitDate"`
	ExitPrice        float64   `json:"exitPrice"`
	ExitReason       string    `gorm:"size:100" json:"exitReason"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

func (KhunterHunting) TableName() string { return "khunter_hunting" }

// KhunterObservation 形态候选观察记录：T 日信号只观察，不假定当日收盘买入。
// 同一股票同一信号日只有一条有效观察（唯一键 (code, signal_date)）；
// 状态只允许由对应事务迁移（确认 → 已确认，过期 → 已过期），
// 同日重跑 UpsertObservation 不得把终态重置为观察中。
type KhunterObservation struct {
	ID              uint      `gorm:"primarykey" json:"id"`
	Code            string    `gorm:"size:20;uniqueIndex:idx_kho_code_signal" json:"code"`
	Name            string    `gorm:"size:50" json:"name"`
	SignalDate      string    `gorm:"size:10;uniqueIndex:idx_kho_code_signal" json:"signalDate"`
	Status          string    `gorm:"size:20" json:"status"`     // 观察中 / 已确认 / 已过期
	DataStatus      string    `gorm:"size:30" json:"dataStatus"` // valid / invalid / intraday
	TriggerPrice    float64   `json:"triggerPrice"`              // 信号日最高价 H，T+1~T+3 触及可确认
	SignalLow       float64   `json:"signalLow"`                 // 信号日最低价 L，初始失效价下界
	ATR14           float64   `json:"atr14"`
	ConfirmExpire   string    `gorm:"size:10" json:"confirmExpire"` // 确认有效期（真实交易日序列由确认引擎判定）
	Score           float64   `json:"score"`
	SignalRiskJSON  string    `gorm:"type:text" json:"signalRiskJson"`
	RiskHistoryJSON string    `gorm:"type:text" json:"riskHistoryJson"`
	RejectReason    string    `gorm:"type:text" json:"rejectReason"` // 数据闸门不放行确认的全部原因
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func (KhunterObservation) TableName() string { return "khunter_observations" }

// KhunterKronosPrediction 观察候选的 Kronos 批量预测明细（推荐视图可展开，因子透明化）。
type KhunterKronosPrediction struct {
	ID             uint      `gorm:"primarykey" json:"id"`
	Code           string    `gorm:"size:20;index:idx_kkp_code_date,unique" json:"code"`
	SignalDate     string    `gorm:"size:10;index:idx_kkp_code_date,unique" json:"signalDate"`
	PredLen        int       `json:"predLen"`
	ForecastJSON   string    `gorm:"type:text" json:"forecastJson"` // 完整预测（KronosPrediction 序列化）；降级时为空
	FactorScore    float64   `json:"factorScore"`
	Weight         float64   `json:"weight"`
	DegradedReason string    `gorm:"size:200" json:"degradedReason"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func (KhunterKronosPrediction) TableName() string { return "khunter_kronos_predictions" }

// KhunterMoneyFlowDaily 资金流历史落库
type KhunterMoneyFlowDaily struct {
	ID         uint    `gorm:"primarykey"`
	Code       string  `gorm:"size:20;index:idx_khmf_code_date,unique"`
	Date       string  `gorm:"size:10;index:idx_khmf_code_date,unique"`
	MainNet    float64 // 主力净额（元）
	MainRatio  float64 // 主力净占比（%）
	SuperLgNet float64 // 超大单净额
	LgNet      float64 // 大单净额
	LgNetRatio float64 // 大单净占比（%）
	MdNet      float64
	SmNet      float64 // 小单净额
	SmNetRatio float64 // 小单净占比（%）
	CreatedAt  time.Time
}

func (KhunterMoneyFlowDaily) TableName() string { return "khunter_money_flow_daily" }

// KhunterRunSummary 每次流水线运行的持久化汇总；同日重跑覆盖更新。
type KhunterRunSummary struct {
	ID                  uint      `gorm:"primarykey" json:"id"`
	TradeDate           string    `gorm:"size:10;uniqueIndex" json:"tradeDate"`
	StockTotal          int       `json:"stockTotal"`
	FreshStocks         int       `json:"freshStocks"`
	InsufficientHistory int       `json:"insufficientHistory"`
	MissingTradeDate    int       `json:"missingTradeDate"`
	QueryFailed         int       `json:"queryFailed"`
	SignalCount         int       `json:"signalCount"`
	CandidateCount      int       `json:"candidateCount"`
	ScoredCount         int       `json:"scoredCount"`
	HuntedCount         int       `json:"huntedCount"`
	InvalidDataCount    int       `json:"invalidDataCount"` // 数据闸门不放行观察的候选数
	IntradayCount       int       `json:"intradayCount"`    // 盘中/未同步数据保存的观察数
	ConfirmedCount      int       `json:"confirmedCount"`   // 确认入池交易数
	ExpiredCount        int       `json:"expiredCount"`     // 确认窗口过期观察数
	UntradeableCount    int       `json:"untradeableCount"` // 已确认触发但不足一手
	RiskLevel           string    `gorm:"size:20" json:"riskLevel"`
	Status              string    `gorm:"size:20" json:"status"` // success / failed
	Message             string    `gorm:"type:text" json:"message"`
	DurationMs          int64     `json:"durationMs"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

func (KhunterRunSummary) TableName() string { return "khunter_run_summaries" }

// KhunterRiskLevel 每日大盘风险档位
type KhunterRiskLevel struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	Date          string    `gorm:"size:10;uniqueIndex" json:"date"`
	Var1d         float64   `json:"var1d"`
	Var5d         float64   `json:"var5d"`
	Level         string    `gorm:"size:10" json:"level"` // 正常/注意/危险/崩溃
	PositionLimit float64   `json:"positionLimit"`
	ScoreExtra    float64   `json:"scoreExtra"`
	CreatedAt     time.Time `json:"createdAt"`
}

func (KhunterRiskLevel) TableName() string { return "khunter_risk_level" }

// KhunterEvent 事件打标结果
type KhunterEvent struct {
	ID         uint   `gorm:"primarykey"`
	Code       string `gorm:"size:20;index:idx_khev_code_date"`
	EventType  string `gorm:"size:30"` // 业绩预增/股东增持/股东减持/...
	EventDate  string `gorm:"size:10;index:idx_khev_code_date"`
	Score      float64
	ExpireDate string `gorm:"size:10"`
	Source     string `gorm:"size:50"`
	CreatedAt  time.Time
}

func (KhunterEvent) TableName() string { return "khunter_events" }

// KhunterResearchRecord 历史研究运行记录。研究模式只写本表，
// 不写信号/评分/狩猎池/运行汇总等生产表，与实时生产状态完全隔离。
type KhunterResearchRecord struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	TradeDate  string    `gorm:"size:10;uniqueIndex" json:"tradeDate"`
	Status     string    `gorm:"size:30" json:"status"` // success / failed
	ResultJSON string    `gorm:"type:text" json:"resultJson"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (KhunterResearchRecord) TableName() string { return "khunter_research_records" }

// KhunterFundamentalSnapshot 基本面时点快照。AvailableDate 为确认数据可用的日期
// （快照落库当日，不早于取数时刻）；历史研究只读 available_date <= asOfDate 的行，
// 缺快照时标记不可回测，禁止用最新值替代。
type KhunterFundamentalSnapshot struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	Code          string    `gorm:"size:20;index:idx_khfs_code_available,unique" json:"code"`
	AvailableDate string    `gorm:"size:10;index:idx_khfs_code_available,unique" json:"availableDate"`
	ReportPeriod  string    `gorm:"size:10" json:"reportPeriod"`
	Payload       string    `gorm:"type:text" json:"payload"` // JSON: scorer.FundamentalInput
	CreatedAt     time.Time `json:"createdAt"`
}

func (KhunterFundamentalSnapshot) TableName() string { return "khunter_fundamental_snapshots" }

// KhunterSectorSnapshot 板块输入时点快照（行业+概念 × 主力净流入，整组一行）。
type KhunterSectorSnapshot struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	Code          string    `gorm:"size:20;index:idx_khss_code_available,unique" json:"code"`
	AvailableDate string    `gorm:"size:10;index:idx_khss_code_available,unique" json:"availableDate"`
	Payload       string    `gorm:"type:text" json:"payload"` // JSON: []scorer.SectorInput
	CreatedAt     time.Time `json:"createdAt"`
}

func (KhunterSectorSnapshot) TableName() string { return "khunter_sector_snapshots" }
