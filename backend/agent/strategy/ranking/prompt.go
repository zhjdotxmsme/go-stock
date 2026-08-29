package ranking

import "strings"

// rankPromptTemplate 排序 Prompt 模板（中文，结构化 JSON 输出，方案 §8.1 D2）。
// %s 处注入格式化后的候选池文本。
const rankPromptTemplate = `你是一位严谨的 A 股投资分析师。下面是经过量化初筛的股票候选池，请你进行跨股票对比分析并重新排序。

要求：
1. 通读全部候选，给出整体市场观点、选股逻辑与组合风险提示。
2. 对每只候选股票给出 0-100 的 llm_score（越高越值得买入）与 0-1 的 confidence（你对该判断的置信度）。
3. ranked 数组必须覆盖候选池中的每一只股票，code 必须与输入完全一致。
4. 只输出一个 JSON 对象，不要输出任何其他文字、解释或 markdown 代码块。

输出 JSON 结构：
{
  "market_view": "整体观点",
  "selection_logic": "选股逻辑",
  "portfolio_risk": "组合风险",
  "ranked": [
    {
      "code": "600519",
      "llm_score": 82,
      "confidence": 0.85,
      "sector": "白酒",
      "theme": "消费复苏",
      "thesis": "核心论点",
      "reason": "推荐理由",
      "risk": "主要风险",
      "catalysts": ["催化剂1"],
      "risk_flags": ["风险标记1"],
      "tags": ["标签1"],
      "style_fit": "风格匹配",
      "watch_items": ["观察项1"],
      "invalidators": ["失效条件1"]
    }
  ]
}

候选池：
%s
`

// DefaultRankPromptTemplate 返回排序 Prompt 默认模板（供调用方写入
// prompt_template 表 role_key=d2_ranking 以便在提示词管理页编辑）。
func DefaultRankPromptTemplate() string {
	return rankPromptTemplate
}

// BuildRankPrompt 用格式化后的候选池文本构建完整排序 Prompt。
func BuildRankPrompt(formattedPool string) string {
	return buildRankPrompt("", formattedPool)
}

// buildRankPrompt 使用自定义模板（可来自 prompt_template 表）构建完整排序 Prompt。
// 模板为空时回退内置默认；模板不含 %s 占位符时自动把候选池拼在末尾。
func buildRankPrompt(template, formattedPool string) string {
	if template == "" {
		template = rankPromptTemplate
	}
	if !strings.Contains(template, "%s") {
		return template + "\n\n候选池：\n" + formattedPool
	}
	return strings.Replace(template, "%s", formattedPool, 1)
}
