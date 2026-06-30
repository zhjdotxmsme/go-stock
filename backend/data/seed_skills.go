package data

import (
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// InitDefaultSkills seeds the database with preset skills if none exist.
func InitDefaultSkills() {
	var count int64
	db.Dao.Model(&models.Skill{}).Count(&count)
	if count > 0 {
		return // already seeded
	}

	skills := []models.Skill{
		{
			Name:            "基本面分析",
			Description:     "评估公司财务健康度、估值指标、成长性",
			Category:        "股票分析",
			SystemPrompt:    `你是一位资深的基本面分析师。请从财务健康度(ROE、资产负债率)、盈利能力(营收增长率、净利润增长率)、估值水平(PE、PB)、成长性、风险提示等维度进行分析。`,
			TriggerKeywords: "基本面,财务,估值,PE,PB,ROE,财报",
			Enable:          true,
			SortOrder:       1,
		},
		{
			Name:            "技术面分析",
			Description:     "分析K线形态、技术指标、趋势判断",
			Category:        "股票分析",
			SystemPrompt:    `你是一位资深的技术分析师。请从趋势判断(均线系统)、技术指标(MACD、RSI、KDJ)、成交量分析、支撑与压力、形态识别等维度进行分析。`,
			TriggerKeywords: "技术面,K线,指标,MACD,RSI,KDJ,均线,趋势",
			Enable:          true,
			SortOrder:       2,
		},
		{
			Name:            "市场情绪分析",
			Description:     "分析市场情绪、舆论倾向、热点话题",
			Category:        "股票分析",
			SystemPrompt:    `你是一位专业的市场情绪分析师。请从整体情绪得分、热点话题、正面因素、负面因素、情绪趋势等维度进行评估。`,
			TriggerKeywords: "情绪,情感,舆情,市场情绪,舆论",
			Enable:          true,
			SortOrder:       3,
		},
		{
			Name:            "新闻事件分析",
			Description:     "解读宏观新闻、行业动态、公司公告对股价的影响",
			Category:        "股票分析",
			SystemPrompt:    `你是一位专业的新闻分析师。请从重大事件、宏观影响、行业动态、公司公告、事件驱动等维度解读新闻对个股的影响。`,
			TriggerKeywords: "新闻,事件,公告,宏观,行业动态,政策",
			Enable:          true,
			SortOrder:       4,
		},
		{
			Name:            "多维度综合分析",
			Description:     "综合基本面、技术面、情绪面、新闻面进行全方位分析",
			Category:        "股票分析",
			SystemPrompt:    `你是一位首席投资策略师。请综合多维度分析结果，给出总体评级、核心投资逻辑、风险提示、多时间维度看法和清晰的投资建议。`,
			TriggerKeywords: "全面分析,综合分析,深度分析,全方位,多维度",
			Enable:          true,
			SortOrder:       5,
		},
		{
			Name:        "美股数据分析",
			Description: "使用 yfinance 获取美股实时行情、历史数据、财务数据、分红信息",
			Category:    "数据查询",
			SystemPrompt: `你是一位美股数据分析师。使用 yfinance MCP 工具获取美股数据。支持查询：股票基本信息（市值、PE、行业）、实时价格、历史K线、股息记录、财务数据、分析师评级、大股东信息等。`,
			TriggerKeywords: "美股,yfinance,US stock,美股行情,美股数据,NYSE,NASDAQ",
			Enable:          false,
			SortOrder:       10,
			Source:          "generated",
			Confidence:      0.85,
			Version:         1,
		},
		{
			Name:        "A股数据分析",
			Description: "使用 akshare 获取A股实时行情、K线数据、财务报告、板块分析",
			Category:    "数据查询",
			SystemPrompt: `你是一位A股数据分析师。使用 QUANTAXIS/akshare MCP 工具获取A股数据。支持查询：A股实时行情、日K/周K/月K线、财务报告摘要、市场指数（上证/深证/创业板/沪深300）、行业板块资金流向等。`,
			TriggerKeywords: "A股,沪深,股票代码,股票行情,A股K线,上证,深证,创业板,北向资金",
			Enable:          false,
			SortOrder:       11,
			Source:          "generated",
			Confidence:      0.85,
			Version:         1,
		},
		{
			Name:        "市场研报分析",
			Description: "获取市场新闻、公司研报、行业分析、技术指标计算等综合研究能力",
			Category:    "研究分析",
			SystemPrompt: `你是一位市场研究员。使用 auto-research MCP 工具进行综合研究分析。支持：获取市场新闻和热点、公司全面概览（业务/财务/估值）、行业研究（产业链/竞争格局）、技术指标计算（SMA/RSI/MACD/布林带）、财务摘要分析、定向公司新闻搜索等。`,
			TriggerKeywords: "研报,研究,市场分析,行业分析,新闻,技术指标,财报分析",
			Enable:          false,
			SortOrder:       12,
			Source:          "generated",
			Confidence:      0.85,
			Version:         1,
		},
	}

	for _, skill := range skills {
		if err := db.Dao.Where("name = ?", skill.Name).FirstOrCreate(&skill).Error; err != nil {
			logger.SugaredLogger.Errorf("seed skill %q failed: %v", skill.Name, err)
		} else {
			logger.SugaredLogger.Infof("seed skill: %s", skill.Name)
		}
	}
}
