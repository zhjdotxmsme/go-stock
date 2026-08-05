package data

// appendStockTools 注册个股行情、异动、自选及 AI 分析历史类工具。
func appendStockTools(tools []Tool) []Tool {
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name: "SetTradingPrice",
			Description: "设置股票的预警价位线（开仓价、止盈价、止损价），用于设置股票的买入价格和风险控制参数。设置后会同步到行情界面显示。" +
				"开仓价：买入的目标价格；止盈价：预期卖出获利价格；止损价：亏损到该价格时必须卖出止损。" +
				"注意：所有价格参数必须为正数，0 表示不设置该价格。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 000001.SZ、600000.SH（沪市）、00700.HK（港股）。注意：上海以.SH结尾，深圳以.SZ结尾，港股以.HK结尾，北交所以.BJ结尾。",
					},
					"entryPrice": map[string]any{
						"type":        "number",
						"description": "开仓价/买入价（目标买入价格），0 表示不设置",
					},
					"takeProfitPrice": map[string]any{
						"type":        "number",
						"description": "止盈价（预期卖出价格），0 表示不设置",
					},
					"stopLossPrice": map[string]any{
						"type":        "number",
						"description": "止损价（亏损止损价格），0 表示不设置",
					},
					"costPrice": map[string]any{
						"type":        "number",
						"description": "成本价（持仓成本价格），0 表示不设置",
					},
				},
				Required: []string{"stockCode", "entryPrice", "takeProfitPrice", "stopLossPrice", "costPrice"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetMarketData",
			Description: "获取市场行情数据，包括指数行情、涨跌分布和今日申购信息",
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "FilterStocks",
			Description: "根据技术指标或者关注排名或者连涨/连跌天数筛选股票。支持MACD金叉、KDJ金叉、均线排列、K线形态，人气，关注排名，连涨/连跌天数等。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"keyword": map[string]any{
						"type":        "string",
						"description": "股票名称或代码关键词搜索",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "页码，默认1",
					},
					"pageSize": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认20",
					},
					"macdGoldenFork": map[string]any{
						"type":        "boolean",
						"description": "MACD金叉",
					},
					"kdjGoldenFork": map[string]any{
						"type":        "boolean",
						"description": "KDJ金叉",
					},
					"breakThrough": map[string]any{
						"type":        "boolean",
						"description": "放量突破",
					},
					"lowFundsInflow": map[string]any{
						"type":        "boolean",
						"description": "低位资金净流入",
					},
					"highFundsOutflow": map[string]any{
						"type":        "boolean",
						"description": "高位资金净流出",
					},
					"breakUpMa5Days": map[string]any{
						"type":        "boolean",
						"description": "向上突破5日均线",
					},
					"longAvgArray": map[string]any{
						"type":        "boolean",
						"description": "均线多头排列",
					},
					"shortAvgArray": map[string]any{
						"type":        "boolean",
						"description": "均线空头排列",
					},
					"upperLargeVolume": map[string]any{
						"type":        "boolean",
						"description": "连涨放量",
					},
					"downNarrowVolume": map[string]any{
						"type":        "boolean",
						"description": "下跌无量",
					},
					"morningStar": map[string]any{
						"type":        "boolean",
						"description": "早晨之星",
					},
					"eveningStar": map[string]any{
						"type":        "boolean",
						"description": "黄昏之星",
					},
					"upNday": map[string]any{
						"type":        "integer",
						"description": "连涨天数：3/5/8天及以上",
					},
					"downNday": map[string]any{
						"type":        "integer",
						"description": "连跌天数：3/5/8/10/14天及以上",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "QueryStockCodeInfo",
			Description: "查询股票/指数信息(名称、代码、拼音、交易所等)",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"searchWord": map[string]any{
						"type":        "string",
						"description": "股票搜索关键词",
					},
				},
				Required: []string{"searchWord"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "QueryStockNews",
			Description: "按关键词搜索相关市场资讯/新闻(财联社)",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"searchWords": map[string]any{
						"type":        "string",
						"description": "搜索关键词(多个关键词使用空格分隔)",
					},
				},
				Required: []string{"searchWords"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockInfo",
			Description: "获取股票详细信息，包括实时行情、基本数据等。支持一次查询多只，将并行请求后合并结果。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码（A股：sh,sz开头;港股hk开头,美股：us开头）。多只时可用英文逗号分隔。",
					},
					"stockCodes": toolSchemaStockCodes,
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockMinuteData",
			Description: "获取股票分时数据（当日分钟级成交量和价格）",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如：600519.SH",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockChanges",
			Description: "获取股票异动数据，包括火箭发射、快速反弹、大笔买入、封涨停板、加速下跌、高台跳水、大笔卖出、封跌停板等异动类型。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"changeTypes": map[string]any{
						"type":        "string",
						"description": "异动类型，多个用逗号分隔。如：火箭发射,快速反弹,大笔买入,封涨停板,加速下跌,高台跳水,大笔卖出,封跌停板",
					},
					"pageSize": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认20",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockChangeHistoryList",
			Description: "查询股票异动历史记录。可以根据股票代码、异动类型、日期范围等条件筛选历史异动数据。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码筛选，支持模糊匹配",
					},
					"changeType": map[string]any{
						"type":        "integer",
						"description": "异动类型代码",
					},
					"startDate": map[string]any{
						"type":        "string",
						"description": "开始日期，格式：YYYY-MM-DD",
					},
					"endDate": map[string]any{
						"type":        "string",
						"description": "结束日期，格式：YYYY-MM-DD",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "页码，默认1",
					},
					"pageSize": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认20",
					},
				},
				Required: []string{"startDate", "endDate"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetFollowedStocks",
			Description: "获取用户关注/自选的股票列表",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"groupId": map[string]any{
						"type":        "integer",
						"description": "股票分组ID，不传则返回所有关注/自选的股票",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetAIAnalysisHistory",
			Description: "查询历史AI分析报告。可以根据股票代码、股票名称、问题关键词、日期范围等条件筛选历史AI分析记录。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码筛选",
					},
					"stockName": map[string]any{
						"type":        "string",
						"description": "股票名称筛选",
					},
					"question": map[string]any{
						"type":        "string",
						"description": "问题关键词搜索",
					},
					"startDate": map[string]any{
						"type":        "string",
						"description": "开始日期，格式：YYYY-MM-DD",
					},
					"endDate": map[string]any{
						"type":        "string",
						"description": "结束日期，格式：YYYY-MM-DD",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "页码，默认1",
					},
					"pageSize": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认10",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetAIAnalysisDetail",
			Description: "根据ID获取历史AI分析报告的详细内容",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"id": map[string]any{
						"type":        "integer",
						"description": "分析报告ID",
					},
				},
				Required: []string{"id"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetAIAnalysisContent",
			Description: "根据股票代码获取最新的AI分析报告内容",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如：600519.SH、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})
	return tools
}
