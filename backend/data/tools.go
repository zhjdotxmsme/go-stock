package data

import "strings"

// @Author spark
// @Date 2026/3/7 18:48
// @Desc
//-----------------------------------------------------------------------------------

// toolSchemaStockCodes 可选多只股票，与 code/stockCode 合并解析（去重）；也可仅在 code/stockCode 中用英文逗号分隔。
var toolSchemaStockCodes = map[string]any{
	"type": "array",
	"items": map[string]any{
		"type": "string",
	},
	"description": "可选，多只股票代码列表；与主字段合并后去重。也可仅在主字段中用英文逗号分隔多只。",
}

func Tools(tools []Tool) []Tool {
	tools = appendSearchTools(tools)
	tools = appendKLineTools(tools)
	tools = appendAnalysisTools(tools)
	tools = appendNewsTools(tools)
	tools = appendStockTools(tools)
	tools = appendMarketTools(tools)
	tools = appendFundTools(tools)
	tools = appendWencaiTools(tools)
	tools = appendFinanceTools(tools)
	tools = appendSentimentTools(tools)

	tools = appendAgentParityTools(tools)

	// 根据 API Key 配置过滤工具，未配置对应 Key 的工具不注册
	tools = FilterToolsByApiKey(tools)

	return tools
}

type dataToolGroup string

const (
	dataToolGroupBase          dataToolGroup = "base"
	dataToolGroupStockAnalysis dataToolGroup = "stock_analysis"
	dataToolGroupMarket        dataToolGroup = "market"
	dataToolGroupScreening     dataToolGroup = "screening"
	dataToolGroupMoneyFlow     dataToolGroup = "money_flow"
	dataToolGroupNewsResearch  dataToolGroup = "news_research"
	dataToolGroupAIAnalysis    dataToolGroup = "ai_analysis"
	dataToolGroupOperations    dataToolGroup = "operations"
)

var dataToolGroupMap = map[string]dataToolGroup{
	"QueryStockCodeInfo": dataToolGroupBase,
	"GetCurrentTime":     dataToolGroupBase,
	"GetHolidayInfo":     dataToolGroupBase,
	"GetHolidayYear":     dataToolGroupBase,
	"GetHolidayBatch":    dataToolGroupBase,
	"IsTradingDay":       dataToolGroupBase,
	"GetNextTradingDay":  dataToolGroupBase,
	"GetFollowedStocks":  dataToolGroupBase,

	"GetStockInfo":                dataToolGroupStockAnalysis,
	"GetStockKLine":               dataToolGroupStockAnalysis,
	"GetEastMoneyKLine":           dataToolGroupStockAnalysis,
	"GetEastMoneyKLineWithMA":     dataToolGroupStockAnalysis,
	"GetStockMinuteData":          dataToolGroupStockAnalysis,
	"GetStockFinancialInfo":       dataToolGroupStockAnalysis,
	"GetStockHolderNum":           dataToolGroupStockAnalysis,
	"GetStockRZRQInfo":            dataToolGroupStockAnalysis,
	"GetStockConceptInfo":         dataToolGroupStockAnalysis,
	"GetIndustryValuation":        dataToolGroupStockAnalysis,
	"GetTdxCompanyInfo":           dataToolGroupStockAnalysis,
	"GetTdxFinanceInfo":           dataToolGroupStockAnalysis,
	"GetTdxXDXRInfo":              dataToolGroupStockAnalysis,
	"GetTdxCompanyCategory":       dataToolGroupStockAnalysis,
	"GetTdxSymbolBelongBoard":     dataToolGroupStockAnalysis,
	"GetStockLatestFinance":       dataToolGroupStockAnalysis,
	"GetStockQtrMainFinance":      dataToolGroupStockAnalysis,
	"GetStockOrgPredict":          dataToolGroupStockAnalysis,
	"GetStockPredictSummary":      dataToolGroupStockAnalysis,
	"GetStockValuationPercentile": dataToolGroupStockAnalysis,
	"GetStockMarginTrading":       dataToolGroupStockAnalysis,
	"GetStockBlockTrade":          dataToolGroupStockAnalysis,
	"GetStockHolderTrend":         dataToolGroupStockAnalysis,
	"GetStockBillboard":           dataToolGroupStockAnalysis,
	"GetStockOperationDeptTrade":  dataToolGroupStockAnalysis,
	"ComparableCompanyAnalysis":   dataToolGroupStockAnalysis,
	"FinancialQA":                 dataToolGroupStockAnalysis,
	"GetAIAnalysisContent":        dataToolGroupStockAnalysis,
	"GetStockResearchReport":      dataToolGroupStockAnalysis,
	"GetIndustryResearchReport":   dataToolGroupStockAnalysis,
	"InteractiveAnswer":           dataToolGroupStockAnalysis,
	"GetSecuritiesCompanyOpinion": dataToolGroupStockAnalysis,
	"StockNotice":                 dataToolGroupStockAnalysis,
	"GetStockNotice":              dataToolGroupStockAnalysis,
	"SearchInvestor":              dataToolGroupStockAnalysis,
	"SearchReport":                dataToolGroupStockAnalysis,
	"QueryInsResearch":            dataToolGroupStockAnalysis,
	"QueryBasicInfo":              dataToolGroupStockAnalysis,
	"QueryFinance":                dataToolGroupStockAnalysis,
	"QueryIndustry":               dataToolGroupStockAnalysis,
	"QueryManagement":             dataToolGroupStockAnalysis,
	"QueryFundFinance":            dataToolGroupStockAnalysis,
	"QueryBusinessData":           dataToolGroupStockAnalysis,
	"StockEarningsReview":         dataToolGroupStockAnalysis,
	"IndustryResearch":            dataToolGroupStockAnalysis,
	"TrackingReport":              dataToolGroupStockAnalysis,
	"FinanceDataQuery":            dataToolGroupStockAnalysis,

	"GetMarketData":              dataToolGroupMarket,
	"GlobalStockIndexesReadable": dataToolGroupMarket,
	"GetStockChanges":            dataToolGroupMarket,
	"GetStockChangeHistoryList":  dataToolGroupMarket,
	"GetDailyChangeStats":        dataToolGroupMarket,
	"GetChangeRank":              dataToolGroupMarket,
	"QueryIwencai":               dataToolGroupMarket,
	"QueryMacro":                 dataToolGroupMarket,
	"QueryZhishu":                dataToolGroupMarket,
	"QueryEvent":                 dataToolGroupMarket,
	"QueryFutures":               dataToolGroupMarket,
	"QueryStockConnect":          dataToolGroupMarket,
	"HotspotDiscovery":           dataToolGroupMarket,
	"GetWallstreetcnMarketReal":  dataToolGroupMarket,
	"GetWallstreetcnKline":       dataToolGroupMarket,
	"GetCommodityTechnicals":     dataToolGroupMarket,
	"GetCommodityFundamentals":   dataToolGroupMarket,
	"GetCorrelationAnalysis":     dataToolGroupMarket,
	"GetCommodityReport":         dataToolGroupMarket,
	"GetDailyDimensionStats":     dataToolGroupMarket,
	"GetTypeStatsByDate":         dataToolGroupMarket,

	"SearchStockByIndicators": dataToolGroupScreening,
	"SearchBk":                dataToolGroupScreening,
	"SearchETF":               dataToolGroupScreening,
	"HotStrategyTable":        dataToolGroupScreening,
	"HotStockTable":           dataToolGroupScreening,
	"FilterStocks":            dataToolGroupScreening,
	"SelectAStock":            dataToolGroupScreening,
	"SelectSector":            dataToolGroupScreening,
	"SelectETF":               dataToolGroupScreening,
	"SelectFundManager":       dataToolGroupScreening,
	"SelectConvertibleBond":   dataToolGroupScreening,
	"SelectFundCompany":       dataToolGroupScreening,
	"SelectFund":              dataToolGroupScreening,
	"SelectFuturesOption":     dataToolGroupScreening,
	"SelectHKStock":           dataToolGroupScreening,
	"SelectUSStock":           dataToolGroupScreening,

	"GetStockMoneyData":        dataToolGroupMoneyFlow,
	"GetMutualTop10Deal":       dataToolGroupMoneyFlow,
	"GetStockHistoryMoneyData": dataToolGroupMoneyFlow,
	"GetIndustryMoneyRank":     dataToolGroupMoneyFlow,

	"GetNewsListData":          dataToolGroupNewsResearch,
	"QueryStockNews":           dataToolGroupNewsResearch,
	"GetInvestCalendar":        dataToolGroupNewsResearch,
	"GetLongTigerList":         dataToolGroupNewsResearch,
	"GetHotStockList":          dataToolGroupNewsResearch,
	"GetHotEventList":          dataToolGroupNewsResearch,
	"SearchNews":               dataToolGroupNewsResearch,
	"SearchAnnouncement":       dataToolGroupNewsResearch,
	"FinanceSearch":            dataToolGroupNewsResearch,
	"GetUplimitLadder":         dataToolGroupNewsResearch,
	"GetUplimitHotPlates":      dataToolGroupNewsResearch,
	"GetUplimitHotStocks":      dataToolGroupNewsResearch,
	"GetUplimitExplodedStocks": dataToolGroupNewsResearch,
	"GetUplimitPlateStocks":    dataToolGroupNewsResearch,
	"GetWallstreetcnLives":     dataToolGroupNewsResearch,
	"GetWallstreetcnCalendar":  dataToolGroupNewsResearch,

	"CreateAiRecommendStocks":      dataToolGroupAIAnalysis,
	"BatchCreateAiRecommendStocks": dataToolGroupAIAnalysis,
	"AiRecommendStocks":            dataToolGroupAIAnalysis,
	"GetAIAnalysisHistory":         dataToolGroupAIAnalysis,
	"GetAIAnalysisDetail":          dataToolGroupAIAnalysis,

	"SetTradingPrice":        dataToolGroupOperations,
	"SendDingDingMessage":    dataToolGroupOperations,
	"SendToDingDing":         dataToolGroupOperations,
	"SearchFund":             dataToolGroupOperations,
	"GetFundInfo":            dataToolGroupOperations,
	"GetFundKLine":           dataToolGroupOperations,
	"GetFundHistoryNetValue": dataToolGroupOperations,
	"GetFundTop10Holdings":   dataToolGroupOperations,
	"GetEconomicData":        dataToolGroupOperations,
}

type dataToolGroupKeywords struct {
	group    dataToolGroup
	keywords []string
}

var dataToolGroupKeywordsList = []dataToolGroupKeywords{
	{dataToolGroupStockAnalysis, []string{
		"股票", "股价", "个股", "行情", "K线", "k线", "日K", "周K", "月K", "分时", "实时", "价格",
		"财务", "报表", "营收", "利润", "ROE", "PE", "PB", "EPS", "现金流", "负债率",
		"股东", "持股", "融资融券", "融券", "融资", "概念", "基本面", "技术面", "估值",
		"基本资料", "上市日期", "股本结构", "股东户数", "实控人", "主营业务", "主要客户",
		"供应商", "经营数据", "业绩点评", "财报分析", "行业研究", "跟踪报告", "金融数据查询",
		"研报", "研究报告", "机构预测", "券商预测", "目标价", "可比公司", "同行对比",
		"分析", "诊断", "评估", "怎么样", "怎么看", "能买吗", "值得买吗",
	}},
	{dataToolGroupMarket, []string{
		"大盘", "市场", "指数", "涨跌分布", "涨停", "跌停", "上涨家数", "下跌家数", "异动",
		"热点", "题材", "宏观", "GDP", "CPI", "PPI", "PMI", "社融", "M2", "LPR",
		"美元指数", "黄金", "原油", "外汇", "美股", "港股", "A股", "全球",
		"期货", "期权", "波动率", "持仓", "北向资金", "南向资金", "沪深港通", "AH溢价",
	}},
	{dataToolGroupScreening, []string{
		"筛选", "选股", "条件选股", "指标选股", "智能选股", "选板块", "板块排行",
		"选ETF", "ETF", "形态选股", "MACD金叉", "KDJ金叉", "放量突破", "连涨", "连跌",
		"基金经理", "基金公司", "选基金", "基金筛选", "基金排名", "可转债", "转债",
		"选期货", "选期权", "港股筛选", "美股筛选", "选港股", "选美股",
		"多头排列", "空头排列", "热门策略", "热门股票",
	}},
	{dataToolGroupMoneyFlow, []string{
		"资金", "流入", "流出", "净流入", "净流出", "北向", "南向", "沪股通", "深股通",
		"港股通", "主力", "外资", "行业资金", "板块资金",
	}},
	{dataToolGroupNewsResearch, []string{
		"新闻", "资讯", "消息", "公告", "最新动态", "政策", "券商", "机构观点", "评级",
		"互动", "问答", "投资者关系", "调研", "财经日历", "龙虎榜", "连板", "梯队",
		"公告搜索", "分红公告", "回购公告", "重组公告", "涨停股", "涨停明细",
		"炸板", "华尔街见闻", "见闻快讯", "7x24", "非农", "美联储", "降息", "加息",
	}},
	{dataToolGroupAIAnalysis, []string{
		"AI分析", "AI推荐", "历史分析", "分析报告", "推荐股票", "买入评级", "增持", "减持",
		"止盈", "止损", "买入价",
	}},
	{dataToolGroupOperations, []string{
		"预警", "价位", "开仓", "成本价", "钉钉", "QQ", "通知", "推送", "发送消息",
		"基金", "基金代码", "基金名称", "净值",
	}},
}

func ToolsForQuestion(question string) []Tool {
	allTools := Tools(nil)
	groups := classifyDataToolGroups(question)
	return filterDataToolsByGroups(allTools, groups)
}

func classifyDataToolGroups(question string) map[dataToolGroup]bool {
	matched := map[dataToolGroup]bool{
		dataToolGroupBase: true,
	}
	lowerQ := strings.ToLower(question)
	for _, groupKeywords := range dataToolGroupKeywordsList {
		for _, keyword := range groupKeywords.keywords {
			if strings.Contains(lowerQ, strings.ToLower(keyword)) {
				matched[groupKeywords.group] = true
				break
			}
		}
	}
	if len(matched) <= 1 {
		matched[dataToolGroupStockAnalysis] = true
		matched[dataToolGroupMarket] = true
		matched[dataToolGroupNewsResearch] = true
	}
	return matched
}

func filterDataToolsByGroups(allTools []Tool, groups map[dataToolGroup]bool) []Tool {
	filtered := make([]Tool, 0, len(allTools))
	for _, tool := range allTools {
		group, exists := dataToolGroupMap[tool.Function.Name]
		if !exists || groups[group] {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

func appendAgentParityTools(tools []Tool) []Tool {
	for _, def := range []struct {
		name        string
		description string
	}{
		{"SendDingDingMessage", "将指定标题和内容以 Markdown 形式发送到钉钉机器人。等同于 SendToDingDing。"},
	} {
		tools = append(tools, Tool{
			Type: "function",
			Function: ToolFunction{
				Name:        def.name,
				Description: def.description,
				Parameters: &FunctionParameters{
					Type: "object",
					Properties: map[string]any{
						"title": map[string]any{
							"type":        "string",
							"description": "消息标题",
						},
						"message": map[string]any{
							"type":        "string",
							"description": "消息正文，通知内容需尽可能精简",
						},
					},
					Required: []string{"title", "message"},
				},
			},
		})
	}

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetHolidayYear",
			Description: "查询指定年份的所有节假日数据，包括日期、名称、连休天数、补班安排等。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"year": map[string]any{
						"type":        "string",
						"description": "查询年份，格式：YYYY。不传则查询当前年份",
					},
				},
			},
		},
	})
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetHolidayBatch",
			Description: "批量查询多个日期的节假日信息。适合需要一次性查询多个日期是否为节假日的场景。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"dates": map[string]any{
						"type":        "string",
						"description": "查询日期列表，多个日期用逗号分隔，格式：YYYY-MM-DD",
					},
				},
				Required: []string{"dates"},
			},
		},
	})

	for _, def := range []struct {
		name        string
		description string
	}{
		{"QueryBasicInfo", "基本资料查询。查询股票、指数、基金、期货、期权、转债、债券、理财、保险等基础信息、发行主体、机构资料、费率、上市地点、上市日期等。"},
		{"QueryFinance", "财务数据查询。查询营业收入、净利润、毛利率、净利率、ROE、ROA、负债率、现金流、市盈率、市净率等财务指标。"},
		{"QueryIndustry", "行业数据查询。查询行业估值、行业财务指标、行业盈利数据、行业行情数据、板块排名等行业维度数据。"},
		{"QueryFutures", "期货期权数据查询。查询期货期权行情、波动率、库存产销、会员持仓、榜单、行权等数据。"},
		{"SelectETF", "ETF智能筛选。按行情、跟踪指数、估值、费率、规模、份额变化等条件筛选ETF。"},
		{"QueryManagement", "公司股东股本查询。查询股本结构、股权结构、股东户数、前十大股东、实控人、质押、高管等。"},
		{"QueryStockConnect", "沪深港通资金流查询。查询北向资金、南向资金、沪股通、深股通、港股通、北向持股变动、AH溢价等。"},
		{"SelectFundManager", "智能选基金经理。根据历史业绩、管理规模、投资风格、风险控制等维度筛选基金经理。"},
		{"SelectConvertibleBond", "智能选可转债。按转股溢价率、正股表现、评级、剩余期限等条件筛选可转债。"},
		{"SelectFundCompany", "智能选基金公司。根据管理规模、旗下产品业绩、投研实力、风险评级等维度筛选基金公司。"},
		{"SelectFund", "智能选基金。根据基金类型、业绩、基金经理、风险、持仓、资产配置等维度筛选基金。"},
		{"SelectFuturesOption", "智能选期货期权。通过行情、波动率、产销、会员持仓、榜单、行权等条件筛选期货期权。"},
		{"SelectHKStock", "智能选港股。通过行情指标、财务指标、行业概念、陆港通等条件筛选港股。"},
		{"SelectUSStock", "智能选美股。通过行情指标、财务指标、行业概念、业绩预测、研报评级等条件筛选美股。"},
		{"QueryFundFinance", "基金理财查询。对基金做业绩、持仓、风险、评级、获奖、基金经理、基金公司综合分析。"},
		{"QueryBusinessData", "公司经营数据查询。查询主营业务构成、主要客户、供应商、参控股公司、股权投资、重大合同等经营数据。"},
	} {
		tools = append(tools, newQueryTool(def.name, def.description, "query"))
	}

	for _, def := range []struct {
		name        string
		description string
	}{
		{"SearchAnnouncement", "公告搜索。搜索A股、港股、基金、ETF等金融标的公告，包括定期财务报告、分红派息、回购增持、资产重组等。"},
		{"StockEarningsReview", "个股业绩点评。获取上市公司业绩点评报告，包含营收分析、利润分析、财务指标解读等深度内容。"},
		{"IndustryResearch", "行业研究报告生成。根据行业关键词生成深度行业研究报告。"},
		{"TrackingReport", "个股或行业跟踪报告。根据股票或行业关键词生成跟踪报告。"},
		{"FinanceDataQuery", "金融数据查询。基于东方财富数据库，支持自然语言查询A股、港股、美股、基金、债券等结构化金融数据。"},
	} {
		tools = append(tools, newSimpleQueryTool(def.name, def.description, "query"))
	}

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetDailyDimensionStats",
			Description: "按维度查询近N日每日异动趋势，支持按股票、行业、概念、异动类型四个维度查询。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"dimension": map[string]any{
						"type":        "string",
						"description": "查询维度：stock=股票，industry=行业，concept=概念，type=异动类型",
					},
					"name": map[string]any{
						"type":        "string",
						"description": "维度名称，如股票名称/代码、行业名称、概念名称、异动类型名称",
					},
					"days": map[string]any{
						"type":        "integer",
						"description": "查询天数，默认30",
					},
				},
				Required: []string{"dimension", "name"},
			},
		},
	})
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetTypeStatsByDate",
			Description: "查询某一天的异动类型分布统计，返回该天每种异动类型的利好/利空次数和总次数。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"date": map[string]any{
						"type":        "string",
						"description": "查询日期，格式：YYYY-MM-DD",
					},
				},
				Required: []string{"date"},
			},
		},
	})
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetUplimitPlateStocks",
			Description: "获取指定板块的涨停股详情，包括板块内所有涨停股票的代码、名称、连板数、封单比、成交额、市值、概念板块等。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"plate_name": map[string]any{
						"type":        "string",
						"description": "板块名称，如：人工智能、机器人、芯片等",
					},
					"date": map[string]any{
						"type":        "string",
						"description": "查询日期，格式：YYYY-MM-DD，默认今天",
					},
				},
				Required: []string{"plate_name"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetCommodityTechnicals",
			Description: "商品技术分析。分析黄金、白银、原油等商品期货的技术面，包括趋势判断、MACD/RSI/布林带指标信号、关键支撑压力位。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"code": map[string]any{
						"type":        "string",
						"description": "品种代码，如：XAUUSD(黄金)、XAGUSD(白银)、USCL(原油)、AU(沪金)",
					},
					"period": map[string]any{
						"type":        "string",
						"description": "分析周期：day（日线）/ week（周线），默认 day",
					},
				},
				Required: []string{"code"},
			},
		},
	})
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetCommodityFundamentals",
			Description: "商品基本面分析。分析黄金、白银、原油等商品的供需格局、美元指数关联、宏观事件影响。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"code": map[string]any{
						"type":        "string",
						"description": "品种代码，如：XAUUSD(黄金)、XAGUSD(白银)、USCL(原油)、AU(沪金)",
					},
					"includeNews": map[string]any{
						"type":        "boolean",
						"description": "是否包含新闻资讯，默认 true",
					},
				},
				Required: []string{"code"},
			},
		},
	})
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetCorrelationAnalysis",
			Description: "商品关联性分析。计算多个品种之间的相关性（基于对数收益率），支持金银比、油金比等比值分析。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"primaryCode": map[string]any{
						"type":        "string",
						"description": "主品种代码，如：XAUUSD",
					},
					"secondaryCodes": map[string]any{
						"type":        "string",
						"description": "关联品种代码，多个用逗号分隔，如：XAGUSD,USCL,DXY.OTC",
					},
					"period": map[string]any{
						"type":        "string",
						"description": "分析周期：day（日线）/ week（周线），默认 day",
					},
				},
				Required: []string{"primaryCode", "secondaryCodes"},
			},
		},
	})
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetCommodityReport",
			Description: "生成商品分析报告。综合分析多个商品品种的技术面、基本面和关联性，输出结构化报告。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"codes": map[string]any{
						"type":        "string",
						"description": "品种代码列表，多个用逗号分隔，如：XAUUSD,XAGUSD,USCL",
					},
					"reportType": map[string]any{
						"type":        "string",
						"description": "报告类型：周报/月报，默认周报",
					},
				},
				Required: []string{"codes"},
			},
		},
	})

	return tools
}

func newQueryTool(name, description, queryField string) Tool {
	return Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        name,
			Description: description,
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					queryField: map[string]any{
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
				Required: []string{queryField},
			},
		},
	}
}

func newSimpleQueryTool(name, description, queryField string) Tool {
	return Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        name,
			Description: description,
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					queryField: map[string]any{
						"type":        "string",
						"description": "自然语言查询语句或关键词",
					},
					"reportDate": map[string]any{
						"type":        "string",
						"description": "仅 StockEarningsReview 可用：报告期，格式YYYY-MM-DD",
					},
				},
				Required: []string{queryField},
			},
		},
	}
}
