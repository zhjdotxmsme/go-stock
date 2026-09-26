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

// KhunterHunting 狩猎场
type KhunterHunting struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	Code         string    `gorm:"size:20;index" json:"code"`
	Name         string    `gorm:"size:50" json:"name"`
	EnterDate    string    `gorm:"size:10" json:"enterDate"`
	EnterScore   float64   `json:"enterScore"`
	SupportPrice float64   `json:"supportPrice"`
	Status       string    `gorm:"size:10" json:"status"` // 追踪中 / 已移除
	TrackDays    int       `json:"trackDays"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (KhunterHunting) TableName() string { return "khunter_hunting" }

// KhunterMoneyFlowDaily 资金流历史落库
type KhunterMoneyFlowDaily struct {
	ID         uint      `gorm:"primarykey"`
	Code       string    `gorm:"size:20;index:idx_khmf_code_date,unique"`
	Date       string    `gorm:"size:10;index:idx_khmf_code_date,unique"`
	MainNet    float64   // 主力净额（元）
	MainRatio  float64   // 主力净占比（%）
	SuperLgNet float64   // 超大单净额
	LgNet      float64   // 大单净额
	LgNetRatio float64   // 大单净占比（%）
	MdNet      float64
	SmNet      float64 // 小单净额
	SmNetRatio float64 // 小单净占比（%）
	CreatedAt  time.Time
}

func (KhunterMoneyFlowDaily) TableName() string { return "khunter_money_flow_daily" }

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
	ID         uint      `gorm:"primarykey"`
	Code       string    `gorm:"size:20;index:idx_khev_code_date"`
	EventType  string    `gorm:"size:30"` // 业绩预增/股东增持/股东减持/...
	EventDate  string    `gorm:"size:10;index:idx_khev_code_date"`
	Score      float64
	ExpireDate string `gorm:"size:10"`
	Source     string `gorm:"size:50"`
	CreatedAt  time.Time
}

func (KhunterEvent) TableName() string { return "khunter_events" }
