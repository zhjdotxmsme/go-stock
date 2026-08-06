// Package postanalysis 实现可插拔后分析链（方案 §8.1 D10）。
// 选股/排序完成后，多个分析器串联对候选输出分数调整（delta），链累加；
// 单分析器失败不中断链——标记失败状态后继续执行后续分析器。
// 纯逻辑实现：输入由调用方装配，HTTP 分析器的 Client 注入便于测试，不接入调用链。
package postanalysis

import (
	"context"
	"fmt"
)

// CandidateInput 后分析候选输入（D2 ranking.Candidate 字段子集 + D3 risk 结果字段，
// 本包自定义精简结构，与调用方解耦；RiskLevel 以字符串传入避免跨包依赖）。
type CandidateInput struct {
	Code string `json:"code"`
	Name string `json:"name,omitempty"`

	// 行情与资金
	Price         float64 `json:"price,omitempty"`
	ChangePercent float64 `json:"changePercent,omitempty"` // 当日涨跌幅 %
	Amount        float64 `json:"amount,omitempty"`        // 成交额（元）
	VolumeRatio   float64 `json:"volumeRatio,omitempty"`
	TurnoverRate  float64 `json:"turnoverRate,omitempty"`

	// 价值质量
	PE float64 `json:"pe,omitempty"`
	PB float64 `json:"pb,omitempty"`

	// 信号与 LLM
	SignalScore   float64  `json:"signalScore,omitempty"`
	LLMConfidence float64  `json:"llmConfidence,omitempty"`
	Catalysts     []string `json:"catalysts,omitempty"`

	// 风控（D3 结果，由调用方装配）
	RiskLevel        string `json:"riskLevel,omitempty"`        // high/medium/low
	HotMoneyUnstable bool   `json:"hotMoneyUnstable,omitempty"` // 热钱不稳标记
}

// AnalyzerOutcome 单个分析器对单只候选的输出。
type AnalyzerOutcome struct {
	Delta  float64 `json:"delta"`            // 分数调整（可正可负）
	Detail string  `json:"detail,omitempty"` // 触发条件明细（中文）
}

// PostAnalyzer 可插拔分析器接口：对整批候选输出分数调整。
type PostAnalyzer interface {
	Name() string
	// Analyze 返回与 candidates 等长且同序的结果；整体失败返回 error（链标记失败并继续）。
	Analyze(ctx context.Context, candidates []CandidateInput) ([]AnalyzerOutcome, error)
}

// 分析器/链状态。
const (
	StatusOK       = "ok"
	StatusFailed   = "failed"
	StatusComplete = "completed" // 链：全部分析器成功
	StatusPartial  = "partial"   // 链：存在失败分析器
)

// PostAnalysisResult 单只候选的后分析结果。
type PostAnalysisResult struct {
	Code       string             `json:"code"`
	Status     string             `json:"status"` // completed / partial
	Deltas     map[string]float64 `json:"deltas"` // 分析器名 -> delta
	Details    map[string]string  `json:"details,omitempty"`
	Errors     map[string]string  `json:"errors,omitempty"`
	TotalDelta float64            `json:"totalDelta"` // 各分析器 delta 累加
}

// Chain 后分析链：分析器按注册顺序串联执行，失败不中断。
type Chain struct {
	Analyzers []PostAnalyzer
}

// NewChain 按注册顺序构造分析链。
func NewChain(analyzers ...PostAnalyzer) *Chain {
	return &Chain{Analyzers: analyzers}
}

// Analyze 对候选池执行分析链，返回与 candidates 等长同序的结果。
// 单分析器失败：该分析器对全部候选标记 failed，后续分析器继续执行。
func (c *Chain) Analyze(ctx context.Context, candidates []CandidateInput) []PostAnalysisResult {
	results := make([]PostAnalysisResult, len(candidates))
	for i, cand := range candidates {
		results[i] = PostAnalysisResult{
			Code:    cand.Code,
			Status:  StatusComplete,
			Deltas:  map[string]float64{},
			Details: map[string]string{},
		}
	}

	for _, analyzer := range c.Analyzers {
		name := analyzer.Name()
		outcomes, err := analyzer.Analyze(ctx, candidates)
		if err != nil || len(outcomes) != len(candidates) {
			errMsg := ""
			if err != nil {
				errMsg = err.Error()
			} else {
				errMsg = fmt.Sprintf("分析器 %s 返回结果数量 %d 与候选数 %d 不一致", name, len(outcomes), len(candidates))
			}
			for i := range results {
				markFailed(&results[i], name, errMsg)
			}
			continue
		}
		for i, outcome := range outcomes {
			results[i].Deltas[name] = outcome.Delta
			if outcome.Detail != "" {
				results[i].Details[name] = outcome.Detail
			}
			results[i].TotalDelta += outcome.Delta
		}
	}

	// 清理空明细 map，保持输出整洁
	for i := range results {
		if len(results[i].Details) == 0 {
			results[i].Details = nil
		}
	}
	return results
}

// markFailed 标记某分析器对某候选失败。
func markFailed(r *PostAnalysisResult, name, errMsg string) {
	r.Status = StatusPartial
	if r.Errors == nil {
		r.Errors = map[string]string{}
	}
	r.Errors[name] = errMsg
}
