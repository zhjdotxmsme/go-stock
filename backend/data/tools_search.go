package data

// appendSearchTools 注册自然语言搜索类工具（选股/板块/ETF）。
func appendSearchTools(tools []Tool) []Tool {
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SearchStockByIndicators",
			Description: "根据自然语言筛选股票。可以使用K线形态、技术指标、财务指标等条件选股，支持多只股票查询（用,分隔）。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"words": map[string]any{
						"type":        "string",
						"description": "选股条件描述，支持K线形态、技术指标、财务指标等。",
					},
				},
				Required: []string{"words"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SearchBk",
			Description: "根据自然语言查询板块/概念/指数整体数据。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"words": map[string]any{
						"type":        "string",
						"description": "板块/概念/指数查询条件描述。",
					},
				},
				Required: []string{"words"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SearchETF",
			Description: "根据自然语言查询ETF数据。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"words": map[string]any{
						"type":        "string",
						"description": "ETF查询条件描述。",
					},
				},
				Required: []string{"words"},
			},
		},
	})
	return tools
}
