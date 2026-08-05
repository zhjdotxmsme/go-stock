package data

// appendSentimentTools 注册市场情绪/涨停/交易日历/华尔街见闻类工具。
func appendSentimentTools(tools []Tool) []Tool {
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "ComparableCompanyAnalysis",
			Description: "可比公司分析(东方财富妙想)。对指定公司进行可比公司分析，包括财务指标对比和估值对比，帮助判断公司相对估值水平。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "公司名称或股票代码，如：贵州茅台、东方财富",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "HotspotDiscovery",
			Description: "市场热点发现(东方财富妙想)。发现当前A股市场热点板块和题材，包括热点逻辑分析和相关个股。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"question": map[string]any{
						"type":        "string",
						"description": "热点的自然语言描述，如：今日热点、新能源热点、AI概念热点",
					},
				},
				Required: []string{"question"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetUplimitLadder",
			Description: "获取连板梯队数据，包括连板统计和连板梯队详情。适用于分析连板高度、市场情绪、龙头股识别等场景。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"date": map[string]any{
						"type":        "string",
						"description": "查询日期，格式：2026-04-17，默认今天",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetUplimitHotPlates",
			Description: "获取涨停热门板块数据",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"date": map[string]any{
						"type":        "string",
						"description": "查询日期，格式：2026-04-17，默认今天",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetUplimitHotStocks",
			Description: "获取涨停热门个股数据",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"date": map[string]any{
						"type":        "string",
						"description": "查询日期，格式：2026-04-17，默认今天",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetUplimitExplodedStocks",
			Description: "获取炸板(封板失败)个股数据",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"date": map[string]any{
						"type":        "string",
						"description": "查询日期，格式：2026-04-17，默认今天",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetDailyChangeStats",
			Description: "获取近N日每日异动统计趋势，包括每天的上涨异动数、下跌异动数、封涨停数、封跌停数和总异动数。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"days": map[string]any{
						"type":        "integer",
						"description": "查询天数，默认30",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetChangeRank",
			Description: "获取异动排行数据",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"tradeDate": map[string]any{
						"type":        "string",
						"description": "交易日期，格式：YYYY-MM-DD",
					},
					"changeType": map[string]any{
						"type":        "string",
						"description": "异动类型",
					},
				},
				Required: []string{"tradeDate"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetHolidayInfo",
			Description: "查询指定日期的节假日信息。返回该日期是否为节假日、节假日名称、是否需要补班等信息。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"date": map[string]any{
						"type":        "string",
						"description": "查询日期，格式：YYYY-MM-DD。不传则查询今天",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "IsTradingDay",
			Description: "判断指定日期是否为A股交易日",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"date": map[string]any{
						"type":        "string",
						"description": "查询日期，格式：YYYY-MM-DD。不传则查询今天",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetNextTradingDay",
			Description: "获取下一个A股交易日",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"startDate": map[string]any{
						"type":        "string",
						"description": "起始日期，格式：YYYY-MM-DD。不传则从今天开始",
					},
					"days": map[string]any{
						"type":        "integer",
						"description": "获取N个交易日后的日期",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetWallstreetcnLives",
			Description: "获取华尔街见闻实时快讯。支持全球7x24、A股、美股、港股、外汇、商品、黄金、原油、债券、加密货币等频道。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"channel": map[string]any{
						"type":        "string",
						"description": "频道：global-channel=全球7x24, a-stock-channel=A股, us-stock-channel=美股, hk-stock-channel=港股, forex-channel=外汇, commodity-channel=商品, goldc-channel=黄金, oil-channel=原油, bond-channel=债券, crypto-channel=加密货币。默认global-channel",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "条数，默认20，最大50",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetWallstreetcnMarketReal",
			Description: "获取华尔街见闻全球实时行情报价。包含美元指数、欧元/美元、美元/日元、离岸人民币、现货黄金、WTI原油等品种。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"prodCodes": map[string]any{
						"type":        "string",
						"description": "品种代码(逗号分隔)，可选：DXY.OTC=美元指数, EURUSD.OTC=欧元美元, USDJPY.OTC=美元日元, USDCNH.OTC=离岸人民币, XAUUSD.OTC=现货黄金, USCL.OTC=WTI原油。留空返回全部。",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetWallstreetcnKline",
			Description: "获取华尔街见闻K线数据。支持美元指数、外汇、黄金、原油等品种的各周期K线。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"prodCode": map[string]any{
						"type":        "string",
						"description": "品种代码：DXY.OTC=美元指数, EURUSD.OTC=欧元美元, USDJPY.OTC=美元日元, USDCNH.OTC=离岸人民币, XAUUSD.OTC=现货黄金, USCL.OTC=WTI原油",
					},
					"periodType": map[string]any{
						"type":        "integer",
						"description": "K线周期(秒)：60=1分钟, 300=5分钟, 900=15分钟, 1800=30分钟, 3600=1小时, 14400=4小时, 86400=日线。默认300",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "K线条数，默认50",
					},
				},
				Required: []string{"prodCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetWallstreetcnCalendar",
			Description: "获取华尔街见闻财经日历。包含全球重要经济数据公布时间、预期值、前值等。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"days": map[string]any{
						"type":        "integer",
						"description": "查看未来几天内的财经日历，默认3天",
					},
				},
			},
		},
	})
	return tools
}
