package data

// appendKLineTools 注册 K 线数据类工具（日K/东财K线/均线）。
func appendKLineTools(tools []Tool) []Tool {
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockKLine",
			Description: "获取股票日K线数据。支持一次查询多只，将并行请求后合并结果。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"days": map[string]any{
						"type":        "string",
						"description": "日K数据条数",
					},
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码（A股：sh,sz开头;港股hk开头,美股：us开头）。多只时可用英文逗号分隔。",
					},
					"stockCodes": toolSchemaStockCodes,
				},
				Required: []string{"days", "stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetEastMoneyKLine",
			Description: "获取股票 K 线数据。支持日/周/月/季/年 K 线及 1/5/15/30/60 分钟线，可选前复权(qfq)或后复权(hfq)。股票代码格式：A股 000001.SZ、600000.SH，港股 00700.HK 等。支持一次查询多只，将并行请求后合并结果。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码。A股如 000001.SZ、600000.SH；港股如 00700.HK。多只时可用英文逗号分隔。",
					},
					"stockCodes": toolSchemaStockCodes,
					"kLineType": map[string]any{
						"type":        "string",
						"description": "K 线类型：day/日/101=日K，week/周/102=周K，month/月/103=月K，quarter/季/104=季K，halfYear/半年/105=半年K，year/年/106=年K；分钟线：1/5/15/30/60/120。",
					},
					"adjustFlag": map[string]any{
						"type":        "string",
						"description": "复权类型，仅日K有效：空=不复权，qfq=前复权，hfq=后复权。",
					},
					"limit": map[string]any{
						"type":        "number",
						"description": "获取 K 线根数（日K为天数，周K为周数，月K为月数，分钟为天数内分钟数等）。",
					},
				},
				Required: []string{"stockCode", "kLineType", "limit"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetEastMoneyKLineWithMA",
			Description: "获取股票 K 线数据并带多条均线（SMA，按收盘价计算）。用于技术分析时同时查看 K 线与均线。股票代码格式同 GetEastMoneyKLine。支持一次查询多只，将并行请求后合并结果。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码。A股如 000001.SZ、600000.SH；港股如 00700.HK。多只时可用英文逗号分隔。",
					},
					"stockCodes": toolSchemaStockCodes,
					"kLineType": map[string]any{
						"type":        "string",
						"description": "K 线类型：day/日/101=日K，week/周/102=周K，month/月/103=月K；分钟线：1/5/15/30/60/120。",
					},
					"limit": map[string]any{
						"type":        "number",
						"description": "获取 K 线根数（如 60 表示最近 60 根）。",
					},
					"maPeriods": map[string]any{
						"type":        "string",
						"description": "均线周期，逗号分隔，如 \"5,10,20,60\"。不传则默认 5,10,20,60,120。",
					},
				},
				Required: []string{"stockCode", "kLineType", "limit"},
			},
		},
	})
	return tools
}
