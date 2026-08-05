package data

// appendWencaiTools 注册问财/选股/宏观查询类工具。
func appendWencaiTools(tools []Tool) []Tool {
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "QueryIwencai",
			Description: "同花顺问财行情数据查询。支持自然语言查询股票、ETF、指数等实时价格、涨跌幅、成交量、技术指标等行情数据。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "自然语言查询语句，如：同花顺最新价格、主力资金流向、上证指数行情等",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "分页页码，默认1",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认10",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SelectAStock",
			Description: "A股智能选股(同花顺i问财)。通过自然语言查询进行A股股票筛选，支持行情指标、技术形态、财务指标、行业概念等多条件组合筛选。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "自然语言选股条件",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "分页页码，默认1",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认10",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SelectSector",
			Description: "选板块(同花顺i问财)。通过自然语言查询板块/概念信息。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "自然语言查询板块条件",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "分页页码，默认1",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认10",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "QueryMacro",
			Description: "宏观数据查询(同花顺i问财)。查询GDP、CPI、PPI、利率、汇率、社融、M2、PMI等宏观经济指标数据。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "自然语言查询语句",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "分页页码，默认1",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认10",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "QueryZhishu",
			Description: "指数数据查询(同花顺i问财)。查询上证指数、沪深300、创业板指等指数行情数据。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "自然语言查询语句",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "分页页码，默认1",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认10",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "QueryEvent",
			Description: "事件数据查询(同花顺i问财)。查询业绩预告、增发配股、股权质押、限售解禁、机构调研等事件数据。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "自然语言查询语句",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "分页页码，默认1",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认10",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SearchNews",
			Description: "财经新闻搜索(同花顺i问财)。搜索财经领域新闻资讯，覆盖官媒、主流财经媒体等。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "搜索关键词",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SearchInvestor",
			Description: "投资者关系活动搜索(同花顺i问财)。搜索上市公司投资者关系活动记录。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "搜索关键词",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SearchReport",
			Description: "研报搜索(同花顺i问财)。搜索主流投研机构发布的研究报告。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "搜索关键词",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "QueryInsResearch",
			Description: "机构研究与评级查询(同花顺i问财)。查询研报评级、业绩预测、ESG评级、券商金股等机构观点数据。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "自然语言查询语句",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "分页页码，默认1",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认10",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "FinanceSearch",
			Description: "金融资讯搜索(东方财富妙想)。支持自然语言搜索全网最新公告、研报、财经新闻。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "自然语言搜索查询",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "FinancialQA",
			Description: "金融问答。针对金融领域专业问题进行回答，包括股票分析、财务指标解读、投资策略等。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"question": map[string]any{
						"type":        "string",
						"description": "金融相关问题",
					},
				},
				Required: []string{"question"},
			},
		},
	})
	return tools
}
