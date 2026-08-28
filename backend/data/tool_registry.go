package data

import (
	"strings"

	"github.com/samber/lo"
)

// ToolContext 封装一次工具调用时需要用到的上下文
type ToolContext struct {
	Question             string
	Messages             *[]map[string]any
	CurrentAIContent     *strings.Builder
	ReasoningContentText *strings.Builder
	CurrentCallID        string
	FuncName             string
	Ch                   chan map[string]any
	StreamResponseID     string
	Model                string
	Source               string
}

// ToolHandler 统一的工具处理函数签名
type ToolHandler func(o *OpenAi, args string, ctx *ToolContext) error

var toolHandlers = map[string]ToolHandler{}

// registerToolHandler 注册一个工具处理函数
func registerToolHandler(name string, handler ToolHandler) {
	toolHandlers[name] = handler
}

// ToolDefinition 将 schema 与 executor 绑定为一体（A1 step 2）：
// 旧链此前靠 "schema 名 ↔ handler 名" 的字符串约定在两个文件树之间弱耦合，
// 漂移只会在 LLM 调用时才暴露。经 registerToolDefinition 注册的定义在
// 装配时即完成绑定，Tools() 的装配门禁会对整表做双向一致性校验。
type ToolDefinition struct {
	Schema      Tool
	Handler     ToolHandler
	RequiredKey string // 空 = 无 API Key 要求
}

var (
	toolDefinitions      = map[string]ToolDefinition{}
	toolDefinitionOrder  []string
	toolDefinitionsBound bool

	// 最近一次 Tools() 装配门禁的结果（测试与诊断用；过滤前视图）。
	lastSchemaHandlerDrift []string
	lastHandlerSchemaDrift []string
)

// registerToolDefinition 注册一个 schema+executor 一体的工具定义。
func registerToolDefinition(def ToolDefinition) {
	name := def.Schema.Function.Name
	if name == "" {
		panic("registerToolDefinition: schema function name is empty")
	}
	if _, exists := toolDefinitions[name]; !exists {
		toolDefinitionOrder = append(toolDefinitionOrder, name)
	}
	toolDefinitions[name] = def
	toolHandlers[name] = def.Handler
}

// definitionsAsTools returns registered definitions in registration order,
// applying the API-key filter.
func definitionsAsTools() []Tool {
	tools := make([]Tool, 0, len(toolDefinitionOrder))
	for _, name := range toolDefinitionOrder {
		def := toolDefinitions[name]
		if def.RequiredKey != "" && !isApiKeyConfigured(def.RequiredKey) {
			continue
		}
		tools = append(tools, def.Schema)
	}
	return tools
}

// bindDefinitionsToSchemas binds assembled schemas to their registered
// handlers, producing the definition table for the whole legacy chain.
// Returns the list of schema names that have NO handler (drift).
func bindDefinitionsToSchemas(tools []Tool) []string {
	toolDefinitionsBound = true
	var drift []string
	for _, t := range tools {
		name := t.Function.Name
		if _, ok := toolHandlers[name]; !ok {
			drift = append(drift, name)
		}
	}
	return drift
}

// handlerNamesWithoutSchema lists registered handlers that no assembled
// schema references (the other direction of drift).
func handlerNamesWithoutSchema(tools []Tool) []string {
	schemaNames := map[string]bool{}
	for _, t := range tools {
		schemaNames[t.Function.Name] = true
	}
	var orphan []string
	for name := range toolHandlers {
		if !schemaNames[name] {
			orphan = append(orphan, name)
		}
	}
	return orphan
}

// toolRequiredKey 工具名与所需 API Key 类型的映射
// key 为工具名，value 为所需的 API Key 类型标识
var toolRequiredKey = map[string]string{
	// IwencaiApiKey 依赖的工具（同花顺问财）
	"QueryIwencai":          "IwencaiApiKey",
	"SearchReport":          "IwencaiApiKey",
	"QueryInsResearch":      "IwencaiApiKey",
	"QueryZhishu":           "IwencaiApiKey",
	"QueryEvent":            "IwencaiApiKey",
	"SearchNews":            "IwencaiApiKey",
	"SearchInvestor":        "IwencaiApiKey",
	"SelectAStock":          "IwencaiApiKey",
	"QueryMacro":            "IwencaiApiKey",
	"SelectSector":          "IwencaiApiKey",
	"QueryBasicInfo":        "IwencaiApiKey",
	"QueryFinance":          "IwencaiApiKey",
	"QueryIndustry":         "IwencaiApiKey",
	"QueryFutures":          "IwencaiApiKey",
	"SelectETF":             "IwencaiApiKey",
	"QueryManagement":       "IwencaiApiKey",
	"QueryStockConnect":     "IwencaiApiKey",
	"SelectFundManager":     "IwencaiApiKey",
	"SelectConvertibleBond": "IwencaiApiKey",
	"SelectFundCompany":     "IwencaiApiKey",
	"SelectFund":            "IwencaiApiKey",
	"SelectFuturesOption":   "IwencaiApiKey",
	"SelectHKStock":         "IwencaiApiKey",
	"SelectUSStock":         "IwencaiApiKey",
	"QueryFundFinance":      "IwencaiApiKey",
	"QueryBusinessData":     "IwencaiApiKey",
	"SearchAnnouncement":    "IwencaiApiKey",

	// EmApiKey 依赖的工具（东方财富妙想）
	"StockEarningsReview":       "EmApiKey",
	"FinancialQA":               "EmApiKey",
	"IndustryResearch":          "EmApiKey",
	"TrackingReport":            "EmApiKey",
	"FinanceDataQuery":          "EmApiKey",
	"FinanceSearch":             "EmApiKey",
	"ComparableCompanyAnalysis": "EmApiKey",
	"HotspotDiscovery":          "EmApiKey",

	// QgqpBId 依赖的工具（东财用户标识，SearchStock系列）
	"SearchStockByIndicators": "QgqpBId",
	"SearchBk":                "QgqpBId",
	"SearchETF":               "QgqpBId",

	// DingRobot+DingPushEnable 依赖的工具（钉钉推送）
	"SendDingDingMessage": "DingRobot",
	"SendToDingDing":      "DingRobot",
}

// isApiKeyConfigured 检查指定类型的 API Key 是否已配置
func isApiKeyConfigured(keyType string) bool {
	config := GetSettingConfigSafe()
	if config == nil || config.Settings == nil {
		return false
	}
	switch keyType {
	case "IwencaiApiKey":
		return strings.TrimSpace(config.IwencaiApiKey) != ""
	case "EmApiKey":
		return strings.TrimSpace(config.EmApiKey) != ""
	case "QgqpBId":
		return strings.TrimSpace(config.QgqpBId) != ""
	case "DingRobot":
		return config.DingPushEnable && strings.TrimSpace(config.DingRobot) != ""
	}
	return true
}

// FilterToolsByApiKey 过滤掉未配置 API Key 的工具 Schema（用于 OpenAI 直连模式）
func FilterToolsByApiKey(tools []Tool) []Tool {
	return lo.Filter(tools, func(t Tool, _ int) bool {
		requiredKey, exists := toolRequiredKey[t.Function.Name]
		if !exists {
			return true // 无 Key 要求的工具保留
		}
		return isApiKeyConfigured(requiredKey)
	})
}

// IsToolKeyConfigured 检查单个工具所需的 API Key 是否已配置（用于 Eino Agent 模式）
func IsToolKeyConfigured(toolName string) bool {
	requiredKey, exists := toolRequiredKey[toolName]
	if !exists {
		return true
	}
	return isApiKeyConfigured(requiredKey)
}
