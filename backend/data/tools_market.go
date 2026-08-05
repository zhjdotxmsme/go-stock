package data

// appendMarketTools 注册市场热点、榜单及宏观日历类工具。
func appendMarketTools(tools []Tool) []Tool {
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetHotStockList",
			Description: "获取雪球热门股票排行榜",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"marketType": map[string]any{
						"type":        "string",
						"description": "市场类型：全球(10)、沪深(12)、港股(13)、美股(11)，默认10",
					},
					"size": map[string]any{
						"type":        "integer",
						"description": "返回条数，默认20",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetHotEventList",
			Description: "获取雪球热门话题/事件",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"size": map[string]any{
						"type":        "integer",
						"description": "返回条数，默认20",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetIndustryMoneyRank",
			Description: "获取行业资金流向排名（按行业分类）",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"fenlei": map[string]any{
						"type":        "string",
						"description": "行业分类：0=所有行业,1=行业分类,2=概念板块,3=地域板块，默认1",
					},
					"sort": map[string]any{
						"type":        "string",
						"description": "排序字段：netamount=净流入,netbuy=主力净流入,change=涨跌幅，默认netamount",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "返回条数，默认20",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetLongTigerList",
			Description: "获取龙虎榜数据（营业部排行榜）",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"date": map[string]any{
						"type":        "string",
						"description": "查询日期，格式：2026-03-28，默认今天",
					},
				},
				Required: []string{"date"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetEconomicData",
			Description: "获取宏观经济数据，包括GDP、CPI、PPI、PMI等",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"dataType": map[string]any{
						"type":        "string",
						"description": "数据类型：gdp=国内生产总值,cpi=居民消费价格指数,ppi=工业生产者出厂价格指数,pmi=采购经理指数",
					},
				},
				Required: []string{"dataType"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetInvestCalendar",
			Description: "获取投资日历，包括财报发布、股东大会、IPO等重要日期事件",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"yearMonth": map[string]any{
						"type":        "string",
						"description": "年月，格式：2026-03，不传则查询当月",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockNotice",
			Description: "获取个股公告信息",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCodes": map[string]any{
						"type":        "string",
						"description": "股票代码列表，逗号分隔，如：600519,000001",
					},
				},
				Required: []string{"stockCodes"},
			},
		},
	})
	return tools
}
