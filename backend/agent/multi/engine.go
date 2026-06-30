package multi

import (
	"context"
	"fmt"
	"go-stock/backend/agent/skill_analysis"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"strings"
	"sync"
	"time"

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
//  1. Orchestrator: inject stock context
//  2. 7 parallel analysts (fundamental, technical, sentiment, news, policy, hotmoney, lockup)
//  3. Bull/Bear researcher debate (2 rounds by default)
//  4. Synthesis into final report
//
// Returns a channel of streaming *schema.Message for the frontend.
func (e *MultiAgentEngine) Run(ctx context.Context, stockCode, stockName, market, userQuery, strategyCode string) chan *schema.Message {
	ch := make(chan *schema.Message, 1024)

	go func() {
		defer close(ch)

		ac := &AgentContext{
			StockCode:    stockCode,
			StockName:    stockName,
			Market:       market,
			UserQuery:    userQuery,
			StrategyCode: strategyCode,
			AIConfigID:   e.aiConfigID,
			StreamCh:     ch,
		}

		// Skill usage tracking: record matched skills at start
		matchedIDs := skill_analysis.GetMatchedSkillIDs(userQuery)
		if len(matchedIDs) > 0 {
			sessionID := fmt.Sprintf("multi-%s-%s-%d", stockCode, time.Now().Format("20060102150405"), e.aiConfigID)
			skill_analysis.RecordMatch(userQuery, sessionID, matchedIDs)
			defer skill_analysis.UpdateResult(sessionID, 0.0, false, "")
		}

		// Fast path: simple queries skip the full multi-agent pipeline
		if isSimpleQuery(userQuery) {
			logger.SugaredLogger.Infof("simple query detected, using fast path: %q", userQuery)
			e.runSimpleQuery(ctx, ac, ch)
			return
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

		// Phase 5: Save to SQLite
		saveMultiAgentResult(ac)

		// Phase 6: Emit final report
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

	resultCh := make(chan result, 7)
	var wg sync.WaitGroup
	wg.Add(7)

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
	go func() {
		defer wg.Done()
		r, err := RunPolicyAnalyst(ctx, ac)
		resultCh <- result{r, err}
	}()
	go func() {
		defer wg.Done()
		r, err := RunHotMoneyAnalyst(ctx, ac)
		resultCh <- result{r, err}
	}()
	go func() {
		defer wg.Done()
		r, err := RunLockupAnalyst(ctx, ac)
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

// isSimpleQuery detects straightforward queries that don't need the full multi-agent pipeline.
// These are queries about price, code, name, or simple factual lookups.
func isSimpleQuery(q string) bool {
	lower := strings.ToLower(strings.TrimSpace(q))
	if lower == "" {
		return false
	}

	simplePatterns := []string{
		"价格", "股价", "行情", "最新", "现价", "今开", "昨收",
		"最高", "最低", "成交量", "成交额", "涨幅", "跌幅",
		"代码", "简称", "全称", "名字",
		"多少", "是什么", "多少钱",
		"price", "quote", "current",
	}

	for _, p := range simplePatterns {
		if strings.Contains(lower, p) {
			// Still skip if the query has analysis keywords
			analysisPatterns := []string{
				"分析", "评价", "总结", "趋势", "建议",
				"基本面", "技术面", "情绪",
				"分析一下", "全面分析", "深度",
				"analyze", "analysis",
			}
			for _, ap := range analysisPatterns {
				if strings.Contains(lower, ap) {
					return false
				}
			}
			return true
		}
	}

	// Short queries (under 10 chars) that aren't asking for analysis
	if len([]rune(lower)) < 10 && !strings.Contains(lower, "分析") {
		return true
	}

	return false
}

// runSimpleQuery handles simple queries with a quick direct response.
func (e *MultiAgentEngine) runSimpleQuery(ctx context.Context, ac *AgentContext, ch chan<- *schema.Message) {
	emitEvent(ch, "agent:phase", map[string]string{
		"phase": "quick", "status": "start",
		"label": "快速查询中...",
	})

	// Try to get real-time price
	price, priceTime := data.GetRealTimeStockPriceInfo(ctx, ac.StockCode)

	answer := fmt.Sprintf("**%s(%s)** 快速查询结果：\n\n", ac.StockName, ac.StockCode)
	if price != "" {
		answer += fmt.Sprintf("- 当前价格：**%s**\n", price)
		if priceTime != "" {
			answer += fmt.Sprintf("- 更新时间：%s\n", priceTime)
		}
	}
	answer += fmt.Sprintf("- 查询时间：%s\n", time.Now().Format("2006-01-02 15:04:05"))
	answer += "\n> 💡 如需深度分析，请输入包含「分析」关键词的问题。"

	emitEvent(ch, "agent:phase", map[string]string{
		"phase": "quick", "status": "end",
		"label": "查询完成",
	})

	report := &FinalReport{
		OverallRating:    "hold",
		Conclusion:       answer,
		InvestmentThesis: "",
	}
	emitFinalReport(ch, report)
}

// saveMultiAgentResult persists the multi-agent analysis results to SQLite.
func saveMultiAgentResult(ac *AgentContext) {
	if ac == nil {
		return
	}

	// Build a combined result string from all analyst reports
	var combined strings.Builder
	combined.WriteString(fmt.Sprintf("## 多智能体分析报告 - %s(%s)\n\n", ac.StockName, ac.StockCode))
	combined.WriteString(fmt.Sprintf("提问: %s\n\n", ac.UserQuery))

	for _, r := range ac.Reports {
		if r.Error != "" {
			combined.WriteString(fmt.Sprintf("### %s - 数据不可用\n\n", r.Role))
			continue
		}
		combined.WriteString(fmt.Sprintf("### %s (评级: %s)\n%s\n\n", r.Role, r.Rating, r.Content))
	}

	if ac.Debate != nil {
		combined.WriteString("## 多空辩论\n\n")
		for _, round := range ac.Debate.Rounds {
			combined.WriteString(fmt.Sprintf("第%d轮 看多: %s\n", round.RoundNum, round.BullArgument))
			combined.WriteString(fmt.Sprintf("第%d轮 看空: %s\n", round.RoundNum, round.BearArgument))
		}
	}

	if ac.FinalReport != nil {
		combined.WriteString(fmt.Sprintf("## 最终评级: %s\n%s\n", ac.FinalReport.OverallRating, ac.FinalReport.Conclusion))
	}

	db.Dao.Create(&models.AIResponseResult{
		StockCode: ac.StockCode,
		StockName: ac.StockName,
		ModelName: "multi-agent-7",
		Content:   combined.String(),
		Question:  ac.UserQuery,
	})

	logger.SugaredLogger.Infof("saved multi-agent result for %s(%s) to SQLite", ac.StockName, ac.StockCode)
}
