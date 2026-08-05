package data

// appendFinanceTools 注册个股财务数据类工具。
func appendFinanceTools(tools []Tool) []Tool {
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockLatestFinance",
			Description: "获取股票最新财务主要数据，包括每股收益(EPS)、每股净资产(BPS)、净资产收益率(ROE)、营业收入、净利润及同比/环比增速等。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ、600000.SH",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockQtrMainFinance",
			Description: "获取股票季度主要财务指标，包括EPS、BPS、营业收入、净利润、同比增长率、ROE、毛利率等按季度列示。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockOrgPredict",
			Description: "获取股票机构预测数据，包括各券商/机构对未来数年的EPS和PE预测明细。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockPredictSummary",
			Description: "获取股票机构预测汇总，按年度汇总多家机构的EPS预测均值、增长率和PE估值。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockValuationPercentile",
			Description: "获取股票估值百分位数据，展示当前PE在历史30%/50%/70%分位的值，判断估值高低。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockMarginTrading",
			Description: "获取股票融资融券数据，包括融资买入额、融资余额、融券卖出量、融券余额等按日列示。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockBlockTrade",
			Description: "获取股票大宗交易数据，包括成交价、溢价率、成交金额、买方/卖方营业部等。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockHolderTrend",
			Description: "获取股票户均持股趋势数据，展示股东户数和户均持股数量随时间的变化趋势。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockBillboard",
			Description: "获取股票龙虎榜数据，包括上榜日期、上榜原因、买入/卖出总额等。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockOperationDeptTrade",
			Description: "获取股票营业部买卖明细，展示各营业部在龙虎榜上的买入/卖出金额和占比。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})
	return tools
}
