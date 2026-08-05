package data

// appendFundTools 注册基金类工具。
func appendFundTools(tools []Tool) []Tool {
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SearchFund",
			Description: "搜索基金信息，支持按基金代码或名称模糊搜索",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"keyword": map[string]any{
						"type":        "string",
						"description": "搜索关键词（基金代码或名称）",
					},
				},
				Required: []string{"keyword"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetFundInfo",
			Description: "获取基金详细信息，包括净值、涨跌幅、评级等",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"fundCode": map[string]any{
						"type":        "string",
						"description": "基金代码，如 000001",
					},
				},
				Required: []string{"fundCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetFundKLine",
			Description: "获取基金K线数据，支持多周期(日K/周K/月K/年K等)。场内基金(ETF/LOF)使用4层数据源fallback，场外基金从东方财富历史净值接口获取",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"fundCode": map[string]any{
						"type":        "string",
						"description": "基金代码，如 510050(场内ETF)、000001(场外基金)",
					},
					"klt": map[string]any{
						"type":        "string",
						"description": "K线周期: 101=日K, 102=周K, 103=月K, 104=年K",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "返回数据条数，默认100",
					},
				},
				Required: []string{"fundCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetFundHistoryNetValue",
			Description: "获取基金历史净值数据。场外基金从东方财富API获取，场内基金(ETF/LOF)从K线收盘价换算",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"fundCode": map[string]any{
						"type":        "string",
						"description": "基金代码，如 000001",
					},
					"pageIndex": map[string]any{
						"type":        "integer",
						"description": "页码，默认1",
					},
					"pageSize": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认20",
					},
					"startDate": map[string]any{
						"type":        "string",
						"description": "开始日期，格式 YYYY-MM-DD",
					},
					"endDate": map[string]any{
						"type":        "string",
						"description": "结束日期，格式 YYYY-MM-DD",
					},
				},
				Required: []string{"fundCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetFundTop10Holdings",
			Description: "获取基金前十大重仓持股信息，包括股票代码、名称、持仓占比、实时股价和涨跌幅",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"fundCode": map[string]any{
						"type":        "string",
						"description": "基金代码，如 000001",
					},
				},
				Required: []string{"fundCode"},
			},
		},
	})
	return tools
}
