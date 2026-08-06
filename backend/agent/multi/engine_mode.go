package multi

import (
	"context"
	"sync"

	"go-stock/backend/logger"

	"github.com/cloudwego/eino/schema"
)

// 模式编排管线（方案 §8.1 D11）：quick / full / specialist 的执行路径。
// standard 模式不经过本文件（走 engine.go 原有管线，行为不变）。

// analystRunners 分析师角色 → 执行函数（quick 模式子集选择用）。
var analystRunners = map[string]func(context.Context, *AgentContext) (*AgentReport, error){
	"fundamental": RunFundamentalAnalyst,
	"technical":   RunTechnicalAnalyst,
	"sentiment":   RunSentimentAnalyst,
	"news":        RunNewsAnalyst,
	"policy":      RunPolicyAnalyst,
	"hotmoney":    RunHotMoneyAnalyst,
	"lockup":      RunLockupAnalyst,
}

// runModePipeline 按模式阶段计划执行管线（带预算控制与挂点）。
// 事件类型与 standard 管线一致（agent:phase / agent:final），前端无需改动。
func (e *MultiAgentEngine) runModePipeline(ctx context.Context, ac *AgentContext, ch chan *schema.Message) {
	cfg := e.config.normalize()
	budget := newBudgetTracker(cfg.TotalBudget, cfg.StageMinBudget, nil)

	for _, stage := range planStages(cfg) {
		// 预算控制：剩余 < StageMinBudget 时跳过非必需阶段，而非启动注定超时的阶段
		if !stage.required && !budget.canStart() {
			emitEvent(ch, "agent:phase", map[string]string{
				"phase": stage.id, "status": "skipped",
				"label": stage.label + "（剩余预算不足，已跳过）",
			})
			logger.SugaredLogger.Infof("stage %s skipped: budget remaining %v < min %v",
				stage.id, budget.remaining(), cfg.StageMinBudget)
			continue
		}

		// 单阶段超时预算
		stageCtx := ctx
		cancel := func() {}
		if cfg.StageTimeout > 0 {
			stageCtx, cancel = context.WithTimeout(ctx, cfg.StageTimeout)
		}

		emitEvent(ch, "agent:phase", map[string]string{
			"phase": stage.id, "status": "start", "label": stage.label,
		})

		skipEndEvent := false
		switch stage.id {
		case stageAnalysts:
			if cfg.Mode == ModeQuick {
				ac.Reports = e.runAnalystsSubset(stageCtx, ac, cfg.QuickAnalysts)
			} else {
				ac.Reports = e.runParallelAnalysts(stageCtx, ac)
			}
			// D6 分歧分类（A3）：分类结果透出事件，引导文本注入合成 Prompt
			e.classifyDisagreement(ac, ch, !cfg.DisagreementGuidanceOff)
		case stageDebate:
			debateResult, err := RunDebate(stageCtx, ac, 2)
			if err != nil {
				logger.SugaredLogger.Errorf("debate error: %v", err)
			}
			ac.Debate = debateResult
		case stageRiskDebate:
			// T1 真实接线（A3）：自定义挂点优先，否则引擎默认三方风控辩论 + D4 否决。
			// 失败降级：记日志继续，不否决、不阻塞结果。
			hook := cfg.RiskDebate
			if hook == nil {
				hook = e.defaultRiskDebateHook()
			}
			if err := hook(stageCtx, ac); err != nil {
				logger.SugaredLogger.Errorf("risk debate error (继续,不否决不阻塞): %v", err)
			}
		case stageSkills:
			if len(cfg.SkillAgents) == 0 {
				emitEvent(ch, "agent:phase", map[string]string{
					"phase": stage.id, "status": "skipped",
					"label": "技能 Agent 未配置，已跳过",
				})
				skipEndEvent = true
			} else {
				for _, sa := range cfg.SkillAgents {
					if err := sa.Run(stageCtx, ac); err != nil {
						logger.SugaredLogger.Errorf("skill agent %s error: %v", sa.Name(), err)
					}
				}
			}
		case stageSynthesis:
			finalReport, err := RunSynthesis(stageCtx, ac)
			if err != nil {
				logger.SugaredLogger.Errorf("synthesis error: %v", err)
			}
			ac.FinalReport = finalReport
			// D6 分类结果回写 FinalReport（透出到 agent:final）
			if finalReport != nil {
				finalReport.DisagreementClass = ac.DisagreementClass
				finalReport.DecisionHint = ac.DecisionHint
			}
		}
		cancel()

		if !skipEndEvent {
			emitEvent(ch, "agent:phase", map[string]string{
				"phase": stage.id, "status": "end", "label": stage.label + "完成",
			})
		}
	}

	// D5 决策标尺（A4）：在风控辩论（D4 可能已调整 OverallRating）之后、落库前计算
	applyDecisionScale(ac)

	saveMultiAgentResult(ac)
	emitFinalReport(ch, ac.FinalReport)
}

// runAnalystsSubset 并行执行选定的分析师子集（并发模式与 runParallelAnalysts 一致）。
func (e *MultiAgentEngine) runAnalystsSubset(ctx context.Context, ac *AgentContext, roles []string) []AgentReport {
	type result struct {
		report *AgentReport
		err    error
	}

	var selected []func(context.Context, *AgentContext) (*AgentReport, error)
	for _, role := range roles {
		if runner, ok := analystRunners[role]; ok {
			selected = append(selected, runner)
		} else {
			logger.SugaredLogger.Warnf("unknown analyst role %q, skipped", role)
		}
	}

	resultCh := make(chan result, len(selected))
	var wg sync.WaitGroup
	wg.Add(len(selected))
	for _, runner := range selected {
		go func(run func(context.Context, *AgentContext) (*AgentReport, error)) {
			defer wg.Done()
			r, err := run(ctx, ac)
			resultCh <- result{r, err}
		}(runner)
	}
	wg.Wait()
	close(resultCh)

	var reports []AgentReport
	for r := range resultCh {
		if r.err != nil {
			logger.SugaredLogger.Errorf("analyst error: %v", r.err)
			reports = append(reports, AgentReport{
				Role: "unknown", Rating: "neutral", Error: r.err.Error(),
			})
			continue
		}
		if r.report != nil {
			reports = append(reports, *r.report)
		}
	}
	return reports
}
