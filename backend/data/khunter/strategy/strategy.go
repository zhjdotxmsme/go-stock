package strategy

import "go-stock/backend/models"

// Signal 统一选股信号（对齐 KHunter 输出格式）
type Signal struct {
	StrategyName string
	Code         string
	Name         string
	Date         string // 信号日（= 选股日）
	KeyDate      string // 关键日期（形态信号日/突破日等）
	KeyDateType  string // 关键日期类型
	Close        float64
	VolumeRatio  float64 // 选股日量比（对前5日均量，不含当日）
	Reasons      []string
	Details      map[string]any
}

// Strategy 形态策略接口。bars 为正序（index 0 = 最早）。
// 返回 nil 表示未命中。
type Strategy interface {
	Name() string
	Weight() int  // 技术面评分权重
	MinBars() int // 最少 K 线根数
	Select(bars []models.KLineBar, stockName, selectionDate string) *Signal
}

var registry []Strategy

func register(s Strategy) { registry = append(registry, s) }

// Registry 返回全部已注册策略（各策略文件 init() 中注册）
func Registry() []Strategy { return registry }
