package multi

import (
	"context"
	"fmt"
	"go-stock/backend/agent"
	"go-stock/backend/data"
	"go-stock/backend/logger"

	"github.com/cloudwego/eino/components/model"
)

// AnalystModelConfig defines per-role LLM configuration overrides.
// Each field is optional — zero values mean "use global default".
type AnalystModelConfig struct {
	Role        string  // fundamental / technical / sentiment / news / researcher_bull / researcher_bear / synthesis
	BaseUrl     string  // custom API URL (empty = use global default)
	ApiKey      string  // API key (empty = use global default)
	ModelName   string  // model name (empty = use global default)
	Temperature float64 // 0 = use global default
	MaxTokens   int     // 0 = use global default
}

// roleTierMap 定义每个角色使用的 LLM 层级
// 优化策略：
// - synthesis 和风控裁判使用 LLMTierDeep (深度思考模型)
// - 所有分析师和辩论研究员使用 LLMTierQuick (快速模型，节约成本和时间)
var roleTierMap = map[string]LLMTier{
	// 深度思考模型 (Tier Deep) - 复杂决策和综合
	"synthesis":     LLMTierDeep, // 最终综合 - 需要深度推理
	"risk_judge":    LLMTierDeep, // 风控裁判 - 需要审慎判断
	"struct_extract": LLMTierQuick, // 结构化提取 - 简单任务

	// 快速模型 (Tier Quick) - 7个分析师 + 辩论研究员
	"fundamental":   LLMTierQuick, // 基本面分析
	"technical":     LLMTierQuick, // 技术面分析
	"sentiment":     LLMTierQuick, // 情绪面分析
	"news":          LLMTierQuick, // 新闻分析
	"policy":        LLMTierQuick, // 政策分析
	"hotmoney":      LLMTierQuick, // 资金流向分析
	"lockup":        LLMTierQuick, // 解禁分析
	"researcher_bull":  LLMTierQuick, // 多方研究员
	"researcher_bear":  LLMTierQuick, // 空方研究员
	"orchestrator":     LLMTierQuick, // 协调者
}

// GetLLMTierForRole 获取指定角色的 LLM 层级
// 如果角色未在映射中定义，返回 LLMTierQuick 作为默认
func GetLLMTierForRole(role string) LLMTier {
	if tier, ok := roleTierMap[role]; ok {
		return tier
	}
	logger.SugaredLogger.Warnf("unknown role %q, using LLMTierQuick as default", role)
	return LLMTierQuick
}

// GetChatModel creates an LLM client for the given role.
// Automatically applies role-based tier assignment (Quick/Deep) for optimal cost-performance.
func GetChatModel(ctx context.Context, role string, aiConfigID int) (model.ToolCallingChatModel, error) {
	tier := GetLLMTierForRole(role)
	return GetChatModelWithTier(ctx, role, tier, aiConfigID)
}

// GetChatModelWithTier creates an LLM client with the specified tier.
// When deep_think tier is requested but deep config is absent, falls back to quick_think.
func GetChatModelWithTier(ctx context.Context, role string, tier LLMTier, aiConfigID int) (model.ToolCallingChatModel, error) {
	cfg := data.GetSettingConfig()
	if cfg == nil {
		return nil, fmt.Errorf("settings not loaded")
	}

	// Find the matching AIConfig by ID
	var aiCfg *data.AIConfig
	for _, c := range cfg.AiConfigs {
		if int(c.ID) == aiConfigID {
			aiCfg = c
			break
		}
	}
	if aiCfg == nil {
		return nil, fmt.Errorf("AI config not found for aiConfigID=%d", aiConfigID)
	}

	effectiveModelName := aiCfg.ModelName
	if tier == LLMTierDeep && aiCfg.DeepModelName != "" {
		effectiveModelName = aiCfg.DeepModelName
	}

	logger.SugaredLogger.Infof("GetChatModelWithTier role=%q aiConfigID=%d tier=%d model=%q base=%q",
		role, aiConfigID, tier, effectiveModelName, aiCfg.BaseUrl)

	effectiveCfg := *aiCfg
	effectiveCfg.ModelName = effectiveModelName

	return agent.CreateChatModel(ctx, effectiveCfg)
}

// LLMTier represents which model tier to use for dual-LLM routing.
type LLMTier int

const (
	LLMTierQuick LLMTier = iota // fast/cheap model for analysts & researchers
	LLMTierDeep                 // powerful model for synthesis/decision
)
