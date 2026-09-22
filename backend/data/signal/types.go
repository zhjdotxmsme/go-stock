// Package signal 本地统一信号引擎：指标信号 + K线形态 + 量价/资金信号的统一判定。
//
// 设计要点：
//   - 输入为日K线（datasource.KLineBar），输出命中信号列表（含元数据：名称/类别/方向/说明）。
//   - 支持历史时点判定（asOfIdx 截断），供"买入时机快照"等复盘场景使用。
//   - 资金流类信号通过 FundsFlowProvider 注入，未注入或失败时该组信号降级跳过。
//   - 判定阈值为常量并附注释；形态定义参考经典K线理论，与东财黑盒判定可能不一致，
//     差异由选股工作台「本地验证」列显式暴露。
package signal

import "go-stock/backend/data/datasource"

// Direction 信号方向
type Direction string

const (
	Bullish Direction = "bullish" // 多头信号
	Bearish Direction = "bearish" // 空头信号
	Neutral Direction = "neutral" // 中性/观察
	Warning Direction = "warning" // 预警（变盘/拐点提示）
)

// Category 信号类别
const (
	CatIndicator = "指标信号"
	CatPattern   = "K线形态"
	CatVolume    = "量价"
	CatFunds     = "资金流"
)

// SignalDef 信号定义（注册表项）
type SignalDef struct {
	Key       string    `json:"key"`       // 与形态选股页条件 key 一致（如 MACD_GOLDEN_FORK）
	Name      string    `json:"name"`      // 中文名
	Category  string    `json:"category"`  // 类别
	Direction Direction `json:"direction"` // 方向
	Tip       string    `json:"tip"`       // 通俗说明（与形态选股页悬停文案一致）
	NeedFunds bool      `json:"-"`         // 是否需要资金流数据
	detect    func(c *evalCtx) bool
}

// SignalMatch 一次判定命中的信号
type SignalMatch struct {
	Key       string    `json:"key"`
	Name      string    `json:"name"`
	Category  string    `json:"category"`
	Direction Direction `json:"direction"`
	Tip       string    `json:"tip"`
}

// FundsFlowProvider 资金流数据提供者：返回近5日主力净流入（元）与是否可用。
type FundsFlowProvider func(code string) (netInflow5d float64, ok bool)

// Engine 信号引擎。零值可用（资金流信号自动跳过）。
type Engine struct {
	Funds FundsFlowProvider
}

// evalCtx 一次判定的计算上下文（序列在 asOf 处截断）
type evalCtx struct {
	bars  []datasource.KLineBar // 截断后的 K 线（索引 0..idx）
	idx   int                   // 判定时点（截断后最后一根）
	open  []float64
	high  []float64
	low   []float64
	close []float64
	vol   []float64
	ma5   []float64
	ma10  []float64
	ma20  []float64
	ma60  []float64
	dif   []float64
	dea   []float64
	k     []float64
	d     []float64

	fundsInflow float64 // 资金流（仅 NeedFunds 信号使用）
	fundsOK     bool
}

// Registry 返回全部信号定义（元数据供前端展示，顺序即展示顺序）。
func Registry() []SignalDef {
	out := make([]SignalDef, len(registry))
	copy(out, registry)
	return out
}

// RegistryMap 返回 key → SignalDef。
func RegistryMap() map[string]SignalDef {
	m := make(map[string]SignalDef, len(registry))
	for _, d := range registry {
		m[d.Key] = d
	}
	return m
}

// exitAlertKeys 逃顶/风险预警类信号（P3 预警用）
var exitAlertKeys = map[string]bool{
	"BLACK_CLOUD_TOPS":   true, // 乌云盖顶
	"EVENING_STAR":       true, // 黄昏之星
	"SHOOTING_STAR":      true, // 射击之星
	"BEARISH_ENGULFING":  true, // 穿头破脚
	"SHORT_AVG_ARRAY":    true, // 均线空头排列
	"HIGH_FUNDS_OUTFLOW": true, // 高位资金净流出
}

// IsExitAlert 是否为逃顶/风险预警类信号。
func IsExitAlert(key string) bool { return exitAlertKeys[key] }
