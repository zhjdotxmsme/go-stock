package multi

import (
	"context"
	"go-stock/backend/logger"
	"sync"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
)

// MultiAgentEngine orchestrates the multi-agent stock analysis pipeline.
type MultiAgentEngine struct {
	aiConfigID int
}

// NewMultiAgentEngine creates a new engine with the given AI config ID.
func NewMultiAgentEngine(aiConfigID int) *MultiAgentEngine {
	return &MultiAgentEngine{aiConfigID: aiConfigID}
}

// Run executes the full multi-agent analysis pipeline:
//   1. Orchestrator: inject stock context
//   2. 4 parallel analysts (fundamental, technical, sentiment, news)
//   3. Bull/Bear researcher debate (2 rounds by default)
//   4. Synthesis into final report
// Returns a channel of streaming *schema.Message for the frontend.
func (e *MultiAgentEngine) Run(ctx context.Context, stockCode, stockName, market, userQuery string) chan *schema.Message {
	ch := make(chan *schema.Message, 1024)

	go func() {
		defer close(ch)

		ac := &AgentContext{
			StockCode:  stockCode,
			StockName:  stockName,
			Market:     market,
			UserQuery:  userQuery,
			AIConfigID: e.aiConfigID,
		}

		// Phase 1: Orchestrator
		emitEvent(ch, "agent:phase", map[string]string{
			"phase": "orchestrator", "status": "start",
			"label": "正在初始化分析参数...",
		})

		// Phase 2: Run 4 analysts in parallel
		emitEvent(ch, "agent:phase", map[string]string{
			"phase": "analysts", "status": "start",
			"label": "各维度分析师并行分析中...",
		})

		reports := e.runParallelAnalysts(ctx, ac)
		ac.Reports = reports

		emitEvent(ch, "agent:phase", map[string]string{
			"phase": "analysts", "status": "end",
			"label": "分析师分析完成",
		})

		// Check cancellation
		if ctx.Err() != nil {
			emitEvent(ch, "agent:phase", map[string]string{
				"phase": "cancelled", "status": "error",
				"label": "分析已取消",
			})
			return
		}

		// Phase 3: Researcher debate
		emitEvent(ch, "agent:phase", map[string]string{
			"phase": "debate", "status": "start",
			"label": "多空研究员辩论中...",
		})

		debateResult, err := RunDebate(ctx, ac, 2)
		if err != nil {
			logger.SugaredLogger.Errorf("debate error: %v", err)
		}
		ac.Debate = debateResult

		emitEvent(ch, "agent:phase", map[string]string{
			"phase": "debate", "status": "end",
			"label": "辩论完成",
		})

		// Phase 4: Synthesis
		emitEvent(ch, "agent:phase", map[string]string{
			"phase": "synthesis", "status": "start",
			"label": "正在生成最终报告...",
		})

		finalReport, err := RunSynthesis(ctx, ac)
		if err != nil {
			logger.SugaredLogger.Errorf("synthesis error: %v", err)
		}
		ac.FinalReport = finalReport

		emitEvent(ch, "agent:phase", map[string]string{
			"phase": "synthesis", "status": "end",
			"label": "分析完成",
		})

		// Phase 5: Emit final report
		emitFinalReport(ch, finalReport)
	}()

	return ch
}

// runParallelAnalysts executes all 4 analyst nodes concurrently using goroutines.
func (e *MultiAgentEngine) runParallelAnalysts(ctx context.Context, ac *AgentContext) []AgentReport {
	type result struct {
		report *AgentReport
		err    error
	}

	resultCh := make(chan result, 4)
	var wg sync.WaitGroup
	wg.Add(4)

	go func() {
		defer wg.Done()
		r, err := RunFundamentalAnalyst(ctx, ac)
		resultCh <- result{r, err}
	}()
	go func() {
		defer wg.Done()
		r, err := RunTechnicalAnalyst(ctx, ac)
		resultCh <- result{r, err}
	}()
	go func() {
		defer wg.Done()
		r, err := RunSentimentAnalyst(ctx, ac)
		resultCh <- result{r, err}
	}()
	go func() {
		defer wg.Done()
		r, err := RunNewsAnalyst(ctx, ac)
		resultCh <- result{r, err}
	}()

	wg.Wait()
	close(resultCh)

	var reports []AgentReport
	for r := range resultCh {
		if r.err != nil {
			logger.SugaredLogger.Errorf("analyst error: %v", r.err)
			reports = append(reports, AgentReport{
				Role:    "unknown",
				Content: "",
				Summary: "",
				Rating:  "neutral",
				Error:   r.err.Error(),
			})
			continue
		}
		if r.report != nil {
			reports = append(reports, *r.report)
		}
	}
	return reports
}

// emitEvent sends a structured progress event to the channel.
func emitEvent(ch chan<- *schema.Message, eventType string, data map[string]string) {
	payload := make(map[string]interface{}, len(data)+1)
	payload["type"] = eventType
	for k, v := range data {
		payload[k] = v
	}
	raw, err := sonic.Marshal(payload)
	if err != nil {
		logger.SugaredLogger.Errorf("emitEvent marshal error: %v", err)
		return
	}
	ch <- &schema.Message{
		Role:    schema.Assistant,
		Content: string(raw),
	}
}

// emitFinalReport sends the final analysis report to the channel.
func emitFinalReport(ch chan<- *schema.Message, report *FinalReport) {
	payload := map[string]interface{}{
		"type":   "agent:final",
		"report": report,
	}
	raw, err := sonic.Marshal(payload)
	if err != nil {
		logger.SugaredLogger.Errorf("emitFinalReport marshal error: %v", err)
		return
	}
	ch <- &schema.Message{
		Role:    schema.Assistant,
		Content: string(raw),
	}
}
