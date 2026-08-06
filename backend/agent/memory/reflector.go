package memory

import (
	"context"
	"fmt"
	"strings"
)

// LLMCallFunc LLM 调用函数注入（同 D2 ranking / T1 risk_debate 模式）：
// 模型与调用实现由调用方装配，本包不直连任何 LLM 配置。
type LLMCallFunc func(ctx context.Context, model string, prompt string) (string, error)

// ReflectionInput 反思输入：当时情境、决策与已知的实际结果。
type ReflectionInput struct {
	AgentRole  string  // Agent 角色（记忆库按角色隔离）
	StockCode  string  // 股票代码
	StockName  string  // 股票名称
	Situation  string  // 当时的市场/决策情境
	Decision   string  // 当时的决策与推理
	ReturnsPct float64 // 实际收益率 %
}

// ReflectionResult 4 步结构化反思结果。
type ReflectionResult struct {
	Evaluation  string `json:"evaluation"`  // 第 1 步：推理评估（因果分析）
	Improvement string `json:"improvement"` // 第 2 步：改进建议
	Lesson      string `json:"lesson"`      // 第 3 步：经验总结
	Query       string `json:"query"`       // 第 4 步：浓缩查询句（检索用，<1000 tokens）
}

// Reflector 反思器：分析结束且实际收益已知后，执行 4 步结构化反思。
type Reflector struct {
	Model string      // 反思使用的模型
	Call  LLMCallFunc // LLM 调用注入
}

// NewReflector 构造反思器。
func NewReflector(model string, call LLMCallFunc) *Reflector {
	return &Reflector{Model: model, Call: call}
}

// Reflect 执行 4 步结构化反思：推理评估 → 改进建议 → 经验总结 → 浓缩查询。
// 各步串行，后一步 Prompt 包含前序步骤的输出；任一步失败即返回错误。
func (r *Reflector) Reflect(ctx context.Context, in ReflectionInput) (*ReflectionResult, error) {
	call := func(prompt string) (string, error) {
		out, err := r.Call(ctx, r.Model, prompt)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(out), nil
	}

	evaluation, err := call(BuildEvaluationPrompt(in))
	if err != nil {
		return nil, fmt.Errorf("反思第1步(推理评估)失败: %w", err)
	}
	improvement, err := call(BuildImprovementPrompt(in, evaluation))
	if err != nil {
		return nil, fmt.Errorf("反思第2步(改进建议)失败: %w", err)
	}
	lesson, err := call(BuildLessonPrompt(in, evaluation, improvement))
	if err != nil {
		return nil, fmt.Errorf("反思第3步(经验总结)失败: %w", err)
	}
	query, err := call(BuildQueryPrompt(lesson))
	if err != nil {
		return nil, fmt.Errorf("反思第4步(浓缩查询)失败: %w", err)
	}

	return &ReflectionResult{
		Evaluation:  evaluation,
		Improvement: improvement,
		Lesson:      lesson,
		Query:       query,
	}, nil
}

// ReflectAndRemember 执行反思并把经验存入指定角色的记忆库。
// LessonText 取浓缩查询句（检索友好）；失败时不存库并返回错误。
func (r *Reflector) ReflectAndRemember(ctx context.Context, mem AgentMemory, in ReflectionInput) (*ReflectionResult, error) {
	result, err := r.Reflect(ctx, in)
	if err != nil {
		return nil, err
	}
	lesson := result.Query
	if lesson == "" {
		lesson = result.Lesson
	}
	if err := mem.Save(ctx, MemoryRecord{
		SituationHash: HashSituation(in.Situation),
		SituationText: in.Situation,
		LessonText:    lesson,
		ReturnsPct:    in.ReturnsPct,
	}); err != nil {
		return nil, fmt.Errorf("记忆保存失败: %w", err)
	}
	return result, nil
}
