// instrument_prompt.go 把标的所属市场的交易规则注入分析师系统提示（方案 Step 3.4）。
// 分析师只看数据不知道规则约束，例如对美股标的说"涨停封板"就是对 A 股规则的错套。
package multi

import "go-stock/backend/internal/domain/stock"

// instrumentContextBlock 依据股票代码前缀返回市场规则提示块；
// 无法识别市场时返回空串（不注入），Prompt 保持原样。
func instrumentContextBlock(code string) string {
	return stock.DetectInstrumentContext(code).PromptBlock()
}
