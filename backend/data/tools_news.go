package data

// appendNewsTools 注册资讯/指数/通知/AI 推荐/公告/券商观点/时间类工具。
func appendNewsTools(tools []Tool) []Tool {
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetNewsListData",
			Description: "获取新闻资讯",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"keyWord": map[string]any{
						"type":        "string",
						"description": "搜索时的关键词，可为空",
					},
					"startTime": map[string]any{
						"type":        "string",
						"description": "开始时间（如：2026-02-23 00:00:00）",
					},
					"limit": map[string]any{
						"type":        "number",
						"description": "每页条数（未传 page/pageSize 时生效，默认 20）",
					},
					"page": map[string]any{
						"type":        "number",
						"description": "页码，从 1 开始",
					},
					"pageSize": map[string]any{
						"type":        "number",
						"description": "每页条数，与 page 配合使用",
					},
				},
				Required: []string{"startTime"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GlobalStockIndexesReadable",
			Description: "获取全球主要指数概览，并输出为 AI 易读的 Markdown 结构化文本。",
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SendToDingDing",
			Description: "将指定标题和内容以 Markdown 形式发送到钉钉机器人。用于把分析结果、摘要或通知推送到钉钉群。需在设置中开启钉钉推送并配置机器人 Webhook。通知内容需尽可能精简。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"title": map[string]any{
						"type":        "string",
						"description": "消息标题，会显示为「go-stock {title}」",
					},
					"message": map[string]any{
						"type":        "string",
						"description": "消息正文，支持 Markdown 格式，通知内容需尽可能精简",
					},
				},
				Required: []string{"title", "message"},
			},
		},
	})

	//CreateAiRecommendStocks
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "CreateAiRecommendStocks",
			Description: "创建/保存AI推荐股票记录",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"modelName": map[string]any{
						"type":        "string",
						"description": "模型名称",
					},
					"rating": map[string]any{
						"type":        "string",
						"description": "评级(买入:强烈看好，预期显著跑赢行业 / 大盘，涨幅空间大。 增持:依然看好，预期跑赢行业 / 大盘，但强度弱于买入。中性:不看多也不看空，预期基本持平市场 / 行业。减持:不看好，预期跑输行业 / 大盘，建议减仓。卖出:强烈看空，预期大幅跑输，建议回避。)",
					},
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码,如：601138.SH。注意 上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，港股股票以.HK结尾，北交所股票以.BJ结尾，",
					},
					"stockName": map[string]any{
						"type":        "string",
						"description": "股票名称",
					},
					"bkCode": map[string]any{
						"type":        "string",
						"description": "板块/行业代码",
					},
					"bkName": map[string]any{
						"type":        "string",
						"description": "板块/概念/行业名称",
					},
					"stockPrice": map[string]any{
						"type":        "string",
						"description": "推荐时股票价格",
					},
					"stockPrePrice": map[string]any{
						"type":        "string",
						"description": "前一交易日股票价格",
					},
					"stockClosePrice": map[string]any{
						"type":        "string",
						"description": "推荐时股票收盘价格",
					},
					"recommendReason": map[string]any{
						"type":        "string",
						"description": "推荐理由/驱动因素/逻辑",
					},
					"recommendBuyPrice": map[string]any{
						"type":        "string",
						"description": "ai建议买入价区间最低价和最高价之间用`-`分隔",
					},
					"recommendBuyPriceMax": map[string]any{
						"type":        "number",
						"description": "ai建议最高买入价",
					},
					"recommendBuyPriceMin": map[string]any{
						"type":        "number",
						"description": "ai建议最低买入价",
					},
					"recommendStopProfitPrice": map[string]any{
						"type":        "string",
						"description": "ai建议止盈价区间最低价和最高价之间用`-`分隔",
					},
					"recommendStopProfitPriceMax": map[string]any{
						"type":        "number",
						"description": "ai建议最高止盈价",
					},
					"recommendStopProfitPriceMin": map[string]any{
						"type":        "number",
						"description": "ai建议最低止盈价",
					},

					"recommendStopLossPrice": map[string]any{
						"type":        "string",
						"description": "ai建议止损价",
					},
					"riskRemarks": map[string]any{
						"type":        "string",
						"description": "风险提示",
					},
					"remarks": map[string]any{
						"type":        "string",
						"description": "操作总结/备注",
					},
				},
				Required: []string{"rating", "stockCode", "stockName", "bkName", "modelName", "recommendReason", "stockPrice"},
			},
		},
	})

	//BatchCreateAiRecommendStocks
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "BatchCreateAiRecommendStocks",
			Description: "批量创建/保存AI推荐股票记录，建议每次批量保存5条记录",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stocks": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"modelName": map[string]any{
									"type":        "string",
									"description": "模型名称",
								},
								"rating": map[string]any{
									"type":        "string",
									"description": "评级(买入:强烈看好，预期显著跑赢行业 / 大盘，涨幅空间大。 增持:依然看好，预期跑赢行业 / 大盘，但强度弱于买入。中性:不看多也不看空，预期基本持平市场 / 行业。减持:不看好，预期跑输行业 / 大盘，建议减仓。卖出:强烈看空，预期大幅跑输，建议回避。)",
								},
								"stockCode": map[string]any{
									"type":        "string",
									"description": "股票代码,如：601138.SH。注意 上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，港股股票以.HK结尾，北交所股票以.BJ结尾，",
								},
								"stockName": map[string]any{
									"type":        "string",
									"description": "股票名称",
								},
								"bkCode": map[string]any{
									"type":        "string",
									"description": "板块/行业代码",
								},
								"bkName": map[string]any{
									"type":        "string",
									"description": "板块/概念/行业名称",
								},
								"stockPrice": map[string]any{
									"type":        "string",
									"description": "推荐时股票价格",
								},
								"stockPrePrice": map[string]any{
									"type":        "string",
									"description": "前一交易日股票价格",
								},
								"stockClosePrice": map[string]any{
									"type":        "string",
									"description": "推荐时股票收盘价格",
								},
								"recommendReason": map[string]any{
									"type":        "string",
									"description": "推荐理由/驱动因素/逻辑",
								},
								"recommendBuyPrice": map[string]any{
									"type":        "string",
									"description": "ai建议买入价区间最低价和最高价之间用`-`分隔",
								},
								"recommendBuyPriceMin": map[string]any{
									"type":        "number",
									"description": "ai建议最低买入价",
								},
								"recommendBuyPriceMax": map[string]any{
									"type":        "number",
									"description": "ai建议最高买入价",
								},
								"recommendStopProfitPrice": map[string]any{
									"type":        "string",
									"description": "ai建议止盈价区间最低价和最高价之间用`-`分隔",
								},
								"recommendStopProfitPriceMin": map[string]any{
									"type":        "number",
									"description": "ai建议最低止盈价",
								},
								"recommendStopProfitPriceMax": map[string]any{
									"type":        "number",
									"description": "ai建议最高止盈价",
								},
								"recommendStopLossPrice": map[string]any{
									"type":        "string",
									"description": "ai建议止损价",
								},
								"riskRemarks": map[string]any{
									"type":        "string",
									"description": "风险提示",
								},
								"remarks": map[string]any{
									"type":        "string",
									"description": "操作总结/备注",
								},
							},
						},
					},
				},

				Required: []string{"rating", "stockCode", "stockName", "bkName", "modelName", "recommendReason", "stockPrice"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "AiRecommendStocks",
			Description: "获取近期AI分析/推荐股票明细列表",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"startDate": map[string]any{
						"type":        "string",
						"description": "开始时间（如：2026-02-23 00:00:00）",
					},
					"endDate": map[string]any{
						"type":        "string",
						"description": "结束时间（如：2026-02-26 23:59:59）",
					},
					"page": map[string]any{
						"type":        "string",
						"description": "分页号（如：1）",
					},
					"pageSize": map[string]any{
						"type":        "string",
						"description": "分页大小(如： 1500)",
					},
					"keyWord": map[string]any{
						"type":        "string",
						"description": "搜索关键词",
					},
				},
				Required: []string{"startDate", "endDate", "page", "pageSize"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "StockNotice",
			Description: "获取上市公司公告列表。可查询一只或多只股票的最新公告（如业绩预告、重大事项、募集资金、减持、增持、监管问题、财务异常等），多只股票用英文逗号分隔。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stock_list": map[string]any{
						"type":        "string",
						"description": "股票代码，多只用英文逗号分隔。例如：600584,600900 或 002046,601138",
					},
				},
				Required: []string{"stock_list"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetSecuritiesCompanyOpinion",
			Description: "获取券商/机构的市场分析观点/要点",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"startDate": map[string]any{
						"type":        "string",
						"description": "开始时间（如：2026-02-23）",
					},
					"endDate": map[string]any{
						"type":        "string",
						"description": "结束时间（如：2026-02-26）",
					},
				},
				Required: []string{"startDate", "endDate"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetCurrentTime",
			Description: "获取当前本地时间（格式：YYYY-MM-DD HH:mm:ss）及星期几",
		},
	})
	return tools
}
