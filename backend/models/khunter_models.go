package models

import "time"

// KhunterSignal 形态策略信号
type KhunterSignal struct {
	ID          uint      `gorm:"primarykey"`
	Code        string    `gorm:"size:20;index:idx_khs_code_date"`
	Name        string    `gorm:"size:50"`
	Strategy    string    `gorm:"size:50;index:idx_khs_strategy_date"`
	SignalDate  string    `gorm:"size:10;index:idx_khs_code_date;index:idx_khs_strategy_date"`
	KeyDate     string    `gorm:"size:10"`
	KeyDateType string    `gorm:"size:20"`
	Close       float64
	VolumeRatio float64
	Reasons     string `gorm:"type:text"` // JSON array
	Details     string `gorm:"type:text"` // JSON object
	CreatedAt   time.Time
}

func (KhunterSignal) TableName() string { return "khunter_signals" }

// KhunterScore 五维评分
type KhunterScore struct {
	ID          uint      `gorm:"primarykey"`
	Code        string    `gorm:"size:20;index:idx_khsc_code_date,unique"`
	ScoreDate   string    `gorm:"size:10;index:idx_khsc_code_date,unique"`
	Technical   float64
	Moneyflow   float64
	Fundamental float64
	Sector      float64
	Event       float64
	Total       float64
	Level       string `gorm:"size:20"` // 强烈推荐/推荐/中性/谨慎/回避/淘汰
	VetoReason  string `gorm:"size:200"`
	Degraded    bool   // 某维度数据缺失按中性分处理
	Details     string `gorm:"type:text"`
	CreatedAt   time.Time
}

func (KhunterScore) TableName() string { return "khunter_scores" }

// KhunterHunting 狩猎场
type KhunterHunting struct {
	ID           uint      `gorm:"primarykey"`
	Code         string    `gorm:"size:20;index"`
	Name         string    `gorm:"size:50"`
	EnterDate    string    `gorm:"size:10"`
	EnterScore   float64
	SupportPrice float64
	Status       string `gorm:"size:10"` // 追踪中 / 已移除
	TrackDays    int
	CreatedAt    time.Time
	UpdatedAt    time.Time
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
	ID            uint      `gorm:"primarykey"`
	Date          string    `gorm:"size:10;uniqueIndex"`
	Var1d         float64
	Var5d         float64
	Level         string `gorm:"size:10"` // 正常/注意/危险/崩溃
	PositionLimit float64
	ScoreExtra    float64
	CreatedAt     time.Time
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
