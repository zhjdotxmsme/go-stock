package commodity

import (
	"context"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"fmt"
	"strings"
)

type CommodityEngine struct {
	aiConfigID int
}

func NewCommodityEngine(aiConfigID int) *CommodityEngine {
	return &CommodityEngine{aiConfigID: aiConfigID}
}

func (e *CommodityEngine) Run(ctx context.Context, code, name, userQuery string) chan *schema.Message {
	ch := make(chan *schema.Message, 1024)

	go func() {
		defer close(ch)

		cc := &CommodityContext{
			Code:       code,
			Name:       name,
			UserQuery:  userQuery,
			AIConfigID: e.aiConfigID,
			StreamCh:   ch,
		}

		// Inject asset metadata from registry
		asset := data.FindCommodityByCode(code)
		if asset != nil {
			cc.Category = asset.Category
			cc.AssetType = asset.AssetType
		}

		// Phase 1: Run experts in parallel (routed by category)
		expertNames := describeExperts(cc.Category)
		emitPhase(cc, "experts", "start", fmt.Sprintf("%s 并行分析中...", expertNames))

		reports := e.runParallelExperts(ctx, cc)
		cc.Reports = reports

		emitPhase(cc, "experts", "end", "专家分析完成")

		if ctx.Err() != nil {
			emitPhase(cc, "cancelled", "error", "分析已取消")
			return
		}

		// Phase 2: Debate
		emitPhase(cc, "debate", "start", "多空研究员辩论中...")

		debateResult, err := RunDebate(ctx, cc, 2)
		if err != nil {
			logger.SugaredLogger.Errorf("commodity debate error: %v", err)
		}
		cc.Debate = debateResult

		emitPhase(cc, "debate", "end", "辩论完成")

		// Phase 3: Synthesis
		emitPhase(cc, "synthesis", "start", "正在生成最终报告...")

		finalReport, err := RunSynthesis(ctx, cc)
		if err != nil {
			logger.SugaredLogger.Errorf("commodity synthesis error: %v", err)
		}
		cc.FinalReport = finalReport

		emitPhase(cc, "synthesis", "end", "分析完成")

		// Phase 4: Save to SQLite
		saveCommodityResult(cc)

		// Phase 5: Emit final report
		emitFinalReport(ch, finalReport)
	}()

	return ch
}

func (e *CommodityEngine) runParallelExperts(ctx context.Context, cc *CommodityContext) []ExpertReport {
	experts := GetExpertsForCategory(cc.Category)

	type result struct {
		report *ExpertReport
		err    error
	}

	resultCh := make(chan result, len(experts))
	var wg sync.WaitGroup

	for _, expert := range experts {
		wg.Add(1)
		go func(exp Expert) {
			defer wg.Done()
			r, err := exp.Run(ctx, cc)
			resultCh <- result{r, err}
		}(expert)
	}

	wg.Wait()
	close(resultCh)

	var reports []ExpertReport
	for r := range resultCh {
		if r.err != nil {
			logger.SugaredLogger.Errorf("expert error: %v", r.err)
			reports = append(reports, ExpertReport{
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

// describeExperts returns a human-readable description of the experts for the given category
func describeExperts(cat models.CommodityCategory) string {
	experts := GetExpertsForCategory(cat)
	names := make([]string, 0, len(experts))
	for _, exp := range experts {
		names = append(names, exp.Role())
	}
	return fmt.Sprintf("%d位专家(%s)", len(experts), strings.Join(names, "/"))
}

func emitFinalReport(ch chan<- *schema.Message, report *CommodityReport) {
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

func saveCommodityResult(cc *CommodityContext) {
	if cc == nil {
		return
	}

	db.Dao.Create(&models.AIResponseResult{
		StockCode: cc.Code,
		StockName: cc.Name,
		ModelName: "commodity-expert",
		Content:   buildResultContent(cc),
		Question:  cc.UserQuery,
	})

	logger.SugaredLogger.Infof("saved commodity result for %s(%s) to SQLite", cc.Name, cc.Code)
}

func buildResultContent(cc *CommodityContext) string {
	var combined strings.Builder
	combined.WriteString(fmt.Sprintf("## 大宗商品多专家分析报告 - %s(%s)\n\n", cc.Name, cc.Code))
	combined.WriteString(fmt.Sprintf("分析时间: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))
	combined.WriteString(fmt.Sprintf("品种类别: %s\n\n", cc.Category))
	combined.WriteString(fmt.Sprintf("提问: %s\n\n", cc.UserQuery))

	for _, r := range cc.Reports {
		if r.Error != "" {
			combined.WriteString(fmt.Sprintf("### %s - 数据不可用\n\n", r.Role))
			continue
		}
		combined.WriteString(fmt.Sprintf("### %s (评级: %s)\n%s\n\n", r.Role, r.Rating, r.Content))
	}

	if cc.Debate != nil {
		combined.WriteString("## 多空辩论\n\n")
		for _, round := range cc.Debate.Rounds {
			combined.WriteString(fmt.Sprintf("第%d轮 看多: %s\n", round.RoundNum, truncateSummary(round.BullArgument, 500)))
			combined.WriteString(fmt.Sprintf("第%d轮 看空: %s\n", round.RoundNum, truncateSummary(round.BearArgument, 500)))
		}
	}

	if cc.FinalReport != nil {
		combined.WriteString(fmt.Sprintf("## 最终评级: %s\n%s\n", cc.FinalReport.OverallRating, cc.FinalReport.Conclusion))
	}

	return combined.String()
}

// GetLiveQuote returns a real-time price snapshot for quick queries
func GetLiveQuote(ctx context.Context, code, name string) string {
	commodityApi := data.NewCommodityApi()
	quote, err := commodityApi.GetQuote(code)
	if err != nil {
		return fmt.Sprintf("**%s(%s)** 行情数据获取失败: %v\n\n> 💡 如需深度分析，请使用多专家分析功能。", name, code, err)
	}

	answer := fmt.Sprintf("**%s(%s)** 实时行情：\n\n", name, code)
	answer += fmt.Sprintf("- 最新价：**%.2f**\n", quote.Price)
	answer += fmt.Sprintf("- 涨跌额：%+.2f\n", quote.Change)
	answer += fmt.Sprintf("- 涨跌幅：%+.2f%%\n", quote.ChangePct)
	if quote.High > 0 {
		answer += fmt.Sprintf("- 最高价：%.2f\n", quote.High)
	}
	if quote.Low > 0 {
		answer += fmt.Sprintf("- 最低价：%.2f\n", quote.Low)
	}
	answer += fmt.Sprintf("- 更新时间：%s\n", quote.Time.Format("2006-01-02 15:04:05"))
	answer += "\n> 💡 如需深度分析，请使用「多专家分析」功能。"

	return answer
}
