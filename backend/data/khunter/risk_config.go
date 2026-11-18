package khunter

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// CostConfig 交易成本假设（回测与仓位计算共用）。
type CostConfig struct {
	CommissionRate  float64 `json:"commissionRate"`
	MinCommission   float64 `json:"minCommission"`
	StampDutyRate   float64 `json:"stampDutyRate"`
	TransferFeeRate float64 `json:"transferFeeRate"`
	SlippageRate    float64 `json:"slippageRate"`
}

// RiskControlConfig 事前风险过滤与执行参数。
// 阈值集中在配置文件（config/khunter_risk_control.json），不在代码里散落硬编码；
// 初始值由 DefaultRiskControlConfig 固定，最终是否调整由训练/验证区间决定。
type RiskControlConfig struct {
	MaxFiveDayReturn      float64 `json:"maxFiveDayReturn"`      // 近5日涨幅上限，超过仅观察
	MaxMA20Deviation      float64 `json:"maxMA20Deviation"`      // MA20 正向乖离上限，超过仅观察
	MaxATRRatio           float64 `json:"maxATRRatio"`           // ATR/收盘价上限，超过仅观察
	MinMedianAmount20     float64 `json:"minMedianAmount20"`     // 近20日有效成交额中位数下限，低于不确认
	ATRPeriod             int     `json:"atrPeriod"`             // ATR 计算周期
	ATRStopMultiple       float64 `json:"atrStopMultiple"`       // 初始失效价 ATR 倍数（执行在确认/退出引擎）
	MaxEntryGap           float64 `json:"maxEntryGap"`           // 入场日最大开盘跳空（执行在确认引擎）
	ConfirmTradingDays    int     `json:"confirmTradingDays"`    // 确认窗口交易日数
	MaxHoldingTradingDays int     `json:"maxHoldingTradingDays"` // 最大持有交易日（执行在退出引擎）
	AccountRiskFraction   float64 `json:"accountRiskFraction"`   // 每笔交易风险预算占组合权益比例
	InitialCash           float64 `json:"initialCash"`           // 回测初始资金
	CostConfig
}

// DefaultRiskControlConfig 初始风险参数（spec 6.1，首轮回测起始配置）：
// 近5日涨幅 >20% 仅观察；MA20 正向乖离 >15% 仅观察；ATR(14)/收盘价 >8% 仅观察；
// 近20日有效成交额中位数 <1 亿元不确认；每笔风险预算 0.5%、ATR 止损 1.5 倍、
// 确认窗口 3 日、最大持有 5 日、最大入场跳空 2%。
func DefaultRiskControlConfig() RiskControlConfig {
	return RiskControlConfig{
		MaxFiveDayReturn:      0.20,
		MaxMA20Deviation:      0.15,
		MaxATRRatio:           0.08,
		MinMedianAmount20:     100000000,
		ATRPeriod:             14,
		ATRStopMultiple:       1.5,
		MaxEntryGap:           0.02,
		ConfirmTradingDays:    3,
		MaxHoldingTradingDays: 5,
		AccountRiskFraction:   0.005,
		InitialCash:           1000000,
		CostConfig: CostConfig{
			CommissionRate:  0.0003,
			MinCommission:   5,
			StampDutyRate:   0.0005,
			TransferFeeRate: 0,
			SlippageRate:    0.0005,
		},
	}
}

// riskControlConfigRel 相对进程工作目录定位风险控制配置文件。
// 已知限制与 kelly 配置相同：进程需从仓库根目录（或 config/ 所在目录的父目录）启动。
const riskControlConfigRel = "config/khunter_risk_control.json"

func resolveRiskControlConfigPath() string {
	if _, err := os.Stat(riskControlConfigRel); err == nil {
		return riskControlConfigRel
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), riskControlConfigRel)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return riskControlConfigRel
}

// LoadRiskControlConfig 加载风险控制配置；文件缺失或解析失败回退默认配置，
// 部分字段缺失时仅覆盖出现的键，其余保持默认。
func LoadRiskControlConfig(path string) RiskControlConfig {
	cfg := DefaultRiskControlConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	if json.Unmarshal(data, &cfg) != nil {
		return DefaultRiskControlConfig()
	}
	return cfg
}
