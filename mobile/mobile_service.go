package main

import "go-stock/backend/data"

// MobileService 面向移动端前端的轻量绑定服务：只暴露 UI 必需的少量查询，
// 不携带 API Key 等敏感字段。
type MobileService struct{}

func NewMobileService() *MobileService { return &MobileService{} }

// MobileAiConfig 是 AIConfig 的脱敏子集（移动端仅需 ID/名称/模型名）。
type MobileAiConfig struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	ModelName string `json:"modelName"`
	Thinking  bool   `json:"thinking"`
}

// ListAiConfigs 返回可用的 AI 配置列表（脱敏）。
func (s *MobileService) ListAiConfigs() []MobileAiConfig {
	sc := data.GetSettingConfig()
	configs := make([]MobileAiConfig, 0, len(sc.AiConfigs))
	for _, c := range sc.AiConfigs {
		configs = append(configs, MobileAiConfig{
			ID:        c.ID,
			Name:      c.Name,
			ModelName: c.ModelName,
			Thinking:  c.Thinking,
		})
	}
	return configs
}

// GetDefaultAiConfigId 返回第一个可用 AI 配置的 ID（无配置时为 0）。
func (s *MobileService) GetDefaultAiConfigId() int {
	sc := data.GetSettingConfig()
	if len(sc.AiConfigs) == 0 {
		return 0
	}
	return int(sc.AiConfigs[0].ID)
}
