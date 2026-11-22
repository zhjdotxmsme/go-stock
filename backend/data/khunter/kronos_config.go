package khunter

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// KronosFactorConfig Kronos 排序因子配置（config/khunter_kronos.json）。
// 默认 enable=false：预测质量经滚动回测验证后再开启。
// 该权重只作用于推荐视图排序分，不进入五维评分，与 khunter_technical_weights.json 无关。
type KronosFactorConfig struct {
	Enable    bool    `json:"enable"`
	TopN      int     `json:"topN"`      // 参与批量预测的观察候选上限（CPU 推理成本约束）
	PredLen   int     `json:"predLen"`   // 预测天数
	Weight    float64 `json:"weight"`    // 推荐视图混分权重
	MaxWeight float64 `json:"maxWeight"` // 权重上限（防配置越界）
}

func DefaultKronosFactorConfig() KronosFactorConfig {
	return KronosFactorConfig{Enable: false, TopN: 30, PredLen: 5, Weight: 0.05, MaxWeight: 0.15}
}

const kronosFactorConfigRel = "config/khunter_kronos.json"

// resolveKronosFactorConfigPath 与 resolveRiskControlConfigPath 同款：先当前目录，
// 再可执行文件目录（安装版 cwd 不是程序目录）。
func resolveKronosFactorConfigPath(path string) string {
	if path != "" {
		return path
	}
	if _, err := os.Stat(kronosFactorConfigRel); err == nil {
		return kronosFactorConfigRel
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), kronosFactorConfigRel)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return kronosFactorConfigRel
}

func LoadKronosFactorConfig(path string) KronosFactorConfig {
	b, err := os.ReadFile(resolveKronosFactorConfigPath(path))
	if err != nil {
		return DefaultKronosFactorConfig()
	}
	cfg := DefaultKronosFactorConfig()
	if json.Unmarshal(b, &cfg) != nil {
		return DefaultKronosFactorConfig()
	}
	if cfg.TopN <= 0 {
		cfg.TopN = 30
	}
	if cfg.PredLen <= 0 {
		cfg.PredLen = 5
	}
	if cfg.Weight <= 0 {
		cfg.Weight = 0.05
	}
	if cfg.MaxWeight <= 0 || cfg.MaxWeight < cfg.Weight {
		cfg.MaxWeight = 0.15
	}
	return cfg
}
