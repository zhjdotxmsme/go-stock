package stock

import (
	"strings"
)

// InstrumentContext 交易标的所属市场的交易规则上下文（方案 Step 3.4 / T6）。
// 依据内部标准代码前缀识别市场，供 LLM 分析师 Prompt 注入使用：
// 分析师据此才知道"涨停能不能追、T+1 还是 T+0、一手是多少股"等市场约束，
// 避免把 A 股规则套到美股/港股标的或反之。
type InstrumentContext struct {
	Market       string // 市场名称，如 "A股(沪深)" / "A股(北交所)" / "港股" / "美股"
	Currency     string // 计价货币：CNY / HKD / USD
	TradingHours string // 交易时段（当地时间）
	PriceLimit   string // 涨跌幅限制说明
	LotSize      string // 最小交易单位 / 每手股数
	Settlement   string // 交收规则：T+1 / T+2
	ShortSelling string // 做空/融券可行性
	Extra        string // 其他对分析有影响的规则差异（可空）
}

// DetectInstrumentContext 依据内部标准代码前缀识别市场并返回交易规则。
// 前缀契约与 stockcode.Normalize 一致：sh/sz → A股沪深，bj → 北交所，
// hk → 港股，us → 美股。无法识别时返回零值（调用方按"未知市场"处理）。
func DetectInstrumentContext(code string) InstrumentContext {
	prefix := normalizedPrefix(code)
	switch prefix {
	case "sh", "sz":
		return InstrumentContext{
			Market:       "A股(沪深)",
			Currency:     "人民币 CNY",
			TradingHours: "交易日 9:15-9:25 集合竞价，9:30-11:30 / 13:00-15:00 连续竞价",
			PriceLimit:   "主板 ±10%；创业板/科创板 ±20%；ST股 ±5%；新股上市初期(科创板/创业板前5日)无涨跌幅限制",
			LotSize:      "主板 100股整数倍起买；科创板最低 200股（可 1 股递增）",
			Settlement:   "T+1 交收，当日买入次一交易日方可卖出",
			ShortSelling: "普通账户不可裸卖空；融券标的有限且成本高",
			Extra:        "涨跌停板具有价格发现与流动性虹吸双重效应：涨停封板强度、跌停撬板资金是重要情绪信号",
		}
	case "bj":
		return InstrumentContext{
			Market:       "A股(北交所)",
			Currency:     "人民币 CNY",
			TradingHours: "交易日 9:15-9:25 集合竞价，9:30-11:30 / 13:00-14:57 连续竞价，14:57-15:00 收盘集合竞价",
			PriceLimit:   "±30%",
			LotSize:      "100股起，可 1 股递增",
			Settlement:   "T+1 交收",
			ShortSelling: "普通账户不可卖空",
			Extra:        "北交所流动性显著弱于沪深，小资金即可造成大幅波动，估值折价需常态化纳入考量",
		}
	case "hk":
		return InstrumentContext{
			Market:       "港股",
			Currency:     "港币 HKD",
			TradingHours: "交易日 9:00-9:30 竞价时段，9:30-12:00 / 13:00-16:00 持续交易",
			PriceLimit:   "无涨跌幅限制（有市调机制 VCM 冷静期）",
			LotSize:      "每手股数因个股而异（常见 100/200/500/1000/2000 股），下单前需查询",
			Settlement:   "T+2 交收",
			ShortSelling: "支持卖空（需借券，仅限指定可卖空名单）",
			Extra:        "港股通标的受汇率(港币/人民币)与两地资金流影响；无涨跌幅限制意味着单日极端波动风险远高于A股",
		}
	case "us":
		return InstrumentContext{
			Market:       "美股",
			Currency:     "美元 USD",
			TradingHours: "常规时段 9:30-16:00 (美东)，盘前 4:00-9:30 / 盘后 16:00-20:00",
			PriceLimit:   "无涨跌幅限制（有个股 LULD 熔断机制）",
			LotSize:      "1 股起买，无手数概念",
			Settlement:   "T+1 交收",
			ShortSelling: "支持做空（融券/期权），做空机制成熟",
			Extra:        "财报/宏观数据多在盘前盘后发布，盘后波动可能远大于盘中；中概股额外受中美监管与退市风险影响",
		}
	default:
		return InstrumentContext{}
	}
}

// PromptBlock 将交易规则格式化为可拼接到 LLM 系统提示的文本块。
// 未知市场返回空串（不注入）。
func (c InstrumentContext) PromptBlock() string {
	if c.Market == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n## 交易标的市场规则（务必在本市场规则约束下给出判断）\n")
	b.WriteString("- 市场/币种：" + c.Market + "，计价货币 " + c.Currency + "\n")
	b.WriteString("- 交易时段：" + c.TradingHours + "\n")
	b.WriteString("- 涨跌幅限制：" + c.PriceLimit + "\n")
	b.WriteString("- 最小交易单位：" + c.LotSize + "\n")
	b.WriteString("- 交收规则：" + c.Settlement + "\n")
	b.WriteString("- 做空约束：" + c.ShortSelling + "\n")
	if c.Extra != "" {
		b.WriteString("- 分析注意：" + c.Extra + "\n")
	}
	return b.String()
}

// normalizedPrefix 提取内部标准格式代码的市场前缀（小写）。
// 容忍大写与前后空白；对 "600519" 这类裸代码无法判定市场，返回空。
func normalizedPrefix(code string) string {
	c := strings.ToLower(strings.TrimSpace(code))
	for _, p := range []string{"sh", "sz", "bj", "hk", "us"} {
		if strings.HasPrefix(c, p) {
			// 排除 us* 开头的普通单词误判：前缀后必须还有代码主体
			if len(c) > len(p) {
				return p
			}
		}
	}
	return ""
}
