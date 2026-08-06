package memory

import (
	"fmt"
)

// BuildEvaluationPrompt 第 1 步：推理评估 —— 决策是否正确？哪些因素导致了正确/错误？
func BuildEvaluationPrompt(in ReflectionInput) string {
	return fmt.Sprintf(`你是一位投资复盘专家。以下是一位 %s 分析师对 %s %s 的决策与实际结果，请进行推理评估。

【当时情境】
%s

【当时决策与推理】
%s

【实际结果】持仓期间收益率 %.2f%%

请回答：
1. 该决策最终被证明是正确的还是错误的？
2. 哪些关键因素导致了这一结果（区分运气与判断）？
3. 当时推理链条中哪一环最站得住脚、哪一环最薄弱？

请用中文输出因果分析，150-300 字。`, in.AgentRole, in.StockCode, in.StockName, in.Situation, in.Decision, in.ReturnsPct)
}

// BuildImprovementPrompt 第 2 步：改进建议 —— 如果错误，什么修订能最大化回报？
func BuildImprovementPrompt(in ReflectionInput, evaluation string) string {
	return fmt.Sprintf(`你是一位投资复盘专家。基于以下决策复盘，请给出具体改进建议。

【当时情境】
%s

【当时决策与推理】
%s

【实际结果】持仓期间收益率 %.2f%%

【推理评估结论】
%s

请回答：
1. 如果决策被证明有误，什么样的修订能最大化回报（仓位、时点、条件单、过滤规则等）？
2. 如果决策被证明正确，什么做法可以让它更稳健、可复制？
3. 给出 2-3 条可执行的具体改进方向，避免空话。

请用中文输出，150-300 字。`, in.Situation, in.Decision, in.ReturnsPct, evaluation)
}

// BuildLessonPrompt 第 3 步：经验总结 —— 学到了什么？与相似情况的关联？
func BuildLessonPrompt(in ReflectionInput, evaluation, improvement string) string {
	return fmt.Sprintf(`你是一位投资复盘专家。请把以下复盘提炼为可泛化的经验。

【当时情境】
%s

【实际结果】持仓期间收益率 %.2f%%

【推理评估】
%s

【改进建议】
%s

请回答：
1. 这次经历最重要的教训是什么？
2. 它与哪些相似市场情境可以关联（什么条件下该经验适用/失效）？
3. 提炼为 1-2 条"当……时，应当……"式的经验法则。

请用中文输出，100-250 字。`, in.Situation, in.ReturnsPct, evaluation, improvement)
}

// BuildQueryPrompt 第 4 步：浓缩查询 —— 将经验压缩为单句（<1000 tokens，向量/全文检索用）。
func BuildQueryPrompt(lesson string) string {
	return fmt.Sprintf(`请将以下投资经验浓缩为一句话检索摘要（中文，不超过 100 字），保留：市场情境关键词、决策方向、结果、核心教训。只输出这句话本身，不要任何解释。

【经验总结】
%s`, lesson)
}
