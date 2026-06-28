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

// GetChatModel creates an LLM client for the given role.
// Delegates to agent.CreateChatModel which handles provider routing for
// OpenAI-compatible, Claude, DeepSeek, Gemini, Ollama, and all other providers.
func GetChatModel(ctx context.Context, role string, aiConfigID int) (model.ToolCallingChatModel, error) {
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

	logger.SugaredLogger.Infof("GetChatModel role=%q aiConfigID=%d model=%q base=%q",
		role, aiConfigID, aiCfg.ModelName, aiCfg.BaseUrl)

	return agent.CreateChatModel(ctx, *aiCfg)
}

// LLMTier represents which model tier to use for dual-LLM routing.
type LLMTier int

const (
	LLMTierQuick LLMTier = iota // fast/cheap model for analysts & researchers
	LLMTierDeep                 // powerful model for synthesis/decision
)

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
