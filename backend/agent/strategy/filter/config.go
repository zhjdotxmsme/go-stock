package filter

import (
	"encoding/json"
	"fmt"
)

// HardFilterConfig 硬过滤配置（方案 §8.1 D7：快照级 13 项 + 日线级 23 项）。
// JSON 序列化，遵循 D1 scoring / D3 risk 的 JSON 配置惯例（文档原文为 YAML，
// 仓库后端无 YAML 配置先例，故不引入 YAML 依赖）。
//
// 边界语义：数值型上限 <=0 表示不限制，下限 <=0 表示不限制；
// 布尔型 false 表示该规则关闭；白名单为空表示不约束。
// 零值配置即全放行。
type HardFilterConfig struct {
	// ===== 快照级（9 类规则，15 参数）=====
	ExcludeST       bool    `json:"excludeSt"` // 排除 ST/退市股，默认 true
	STPattern       string  `json:"stPattern"` // ST 名称正则，默认 "ST|退"
	MinAmount       float64 `json:"minAmount"` // 成交额下限（元）
	MaxAmount       float64 `json:"maxAmount"`
	MinPrice        float64 `json:"minPrice"`
	MaxPrice        float64 `json:"maxPrice"`
	MinTotalMV      float64 `json:"minTotalMv"` // 总市值下限（元）
	MaxTotalMV      float64 `json:"maxTotalMv"`
	MinPE           float64 `json:"minPe"` // PE 下限（默认 0.01：自动排除亏损股；0 表示不限制）
	MaxPE           float64 `json:"maxPe"`
	MinPB           float64 `json:"minPb"`
	MaxPB           float64 `json:"maxPb"`
	MinVolumeRatio  float64 `json:"minVolumeRatio"`  // 量比下限
	MinTurnoverRate float64 `json:"minTurnoverRate"` // 换手率下限 %
	MinChangePct    float64 `json:"minChangePct"`    // 当日涨跌幅下限 %（如 -9.5 排除跌停）
	MaxChangePct    float64 `json:"maxChangePct"`    // 当日涨跌幅上限 %（如 9.5 排除涨停买不进）

	// ===== 日线级（15 类规则，23 参数）=====
	RejectMissingDaily   bool     `json:"rejectMissingDaily"` // 无日线数据时淘汰，默认 true
	MinChangePct60       float64  `json:"minChangePct60"`     // 60 日涨跌幅下限 %
	MaxChangePct60       float64  `json:"maxChangePct60"`
	RequireMABullAlign   bool     `json:"requireMaBullAlign"` // 要求 MA 多头排列
	RequireAboveMA20     bool     `json:"requireAboveMa20"`   // 要求价格站上 MA20
	MinSignalScore       float64  `json:"minSignalScore"`     // 信号分下限 0-100
	MACDWhitelist        []string `json:"macdWhitelist"`      // MACD 状态白名单（bullish/bearish/neutral）
	RSIWhitelist         []string `json:"rsiWhitelist"`       // RSI 状态白名单（overbought/oversold/neutral）
	MinBreakoutPct       float64  `json:"minBreakoutPct"`     // 20 日突破幅度下限 %
	MaxBreakoutPct       float64  `json:"maxBreakoutPct"`
	MinAmplitudePct      float64  `json:"minAmplitudePct"` // 振幅下限 %
	MaxAmplitudePct      float64  `json:"maxAmplitudePct"`
	MinVolumeRatio20d    float64  `json:"minVolumeRatio20d"` // 20 日量比下限
	MaxVolumeRatio20d    float64  `json:"maxVolumeRatio20d"`
	MinBodyPct           float64  `json:"minBodyPct"`    // K 线实体比例下限 %
	MinMA20DevPct        float64  `json:"minMa20DevPct"` // 回踩 MA20 幅度下限 %（价格相对 MA20 偏离）
	MaxMA20DevPct        float64  `json:"maxMa20DevPct"`
	MinConsolidationDays int      `json:"minConsolidationDays"` // 盘整天数下限
	MaxConsolidationDays int      `json:"maxConsolidationDays"`
	MaxVolatilityPct     float64  `json:"maxVolatilityPct"` // 波动率上限 %
	MinDrawdownPct       float64  `json:"minDrawdownPct"`   // 最大回撤下限 %（如 -25，回撤更深则淘汰）
	MaxATRPct            float64  `json:"maxAtrPct"`        // ATR/价格 上限 %
}

// DefaultHardFilterConfig 返回一组合理的默认过滤参数（短线选股视角，可按需覆盖）。
func DefaultHardFilterConfig() HardFilterConfig {
	return HardFilterConfig{
		// 快照级
		ExcludeST: true, STPattern: `ST|退`,
		MinAmount: 5e7, MinPrice: 2, MaxPrice: 300,
		MinPE: 0.01, MaxPE: 150, MinPB: 0, MaxPB: 15,
		MinVolumeRatio: 0.8, MinTurnoverRate: 1,
		MinChangePct: -9.5, MaxChangePct: 9.5,
		// 日线级
		RejectMissingDaily: true,
		MinChangePct60:     -15, MaxChangePct60: 45,
		RequireAboveMA20: true,
		MinSignalScore:   50,
		MACDWhitelist:    []string{"bullish", "neutral"},
		RSIWhitelist:     []string{"neutral", "oversold"},
		MinBreakoutPct:   -5, MaxBreakoutPct: 20,
		MaxAmplitudePct:   12,
		MinVolumeRatio20d: 0.8, MaxVolumeRatio20d: 6,
		MinBodyPct:    0.3,
		MinMA20DevPct: -8, MaxMA20DevPct: 8,
		MaxVolatilityPct: 45,
		MinDrawdownPct:   -25,
		MaxATRPct:        6,
	}
}

// LoadHardFilterConfigJSON 从 JSON 字节流加载配置；缺省字段保留默认值。
func LoadHardFilterConfigJSON(data []byte) (HardFilterConfig, error) {
	cfg := DefaultHardFilterConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("解析硬过滤配置失败: %w", err)
	}
	return cfg, nil
}
