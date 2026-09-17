package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/emitter"
	"go-stock/backend/internal/adapter/datasource"
	"go-stock/backend/internal/adapter/repository/sqlite"
	"go-stock/backend/internal/service/trading"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// TradingRecordHandler handles trading-record-related Wails bindings.
// It keeps data-layer types in its signatures (so App and the frontend see no
// change) and delegates business logic to the trading service.
type TradingRecordHandler struct {
	ctxFn func() context.Context
	emit  emitter.Emitter
	svc   *trading.Service

	summaryMu     sync.Mutex
	summaryCancel context.CancelFunc
}

// NewTradingRecordHandler creates a new TradingRecordHandler.
func NewTradingRecordHandler(ctxFn func() context.Context, svc *trading.Service, emit emitter.Emitter) *TradingRecordHandler {
	if emit == nil {
		emit = emitter.Discard
	}
	return &TradingRecordHandler{ctxFn: ctxFn, svc: svc, emit: emit}
}

// NewDefaultTradingRecordHandler wires the production dependencies
// (sqlite repository + datasource router 实时行情) and returns the handler.
// The wiring lives here because backend/internal packages cannot be
// imported by the main package at the repository root.
func NewDefaultTradingRecordHandler(ctxFn func() context.Context, emit emitter.Emitter) *TradingRecordHandler {
	router := datasource.NewDefaultRouter()
	priceFn := func(stockCode string) (float64, error) {
		q, err := router.GetQuote(ctxFn(), stockCode)
		if err != nil || q == nil {
			return 0, err
		}
		if q.Price > 0 {
			return q.Price, nil
		}
		// 停牌股现价为 0 时退化为卖一价（与原 data 直查行为一致）
		if ask1, ok := q.Extra["ask1"].(float64); ok {
			return ask1, nil
		}
		return 0, nil
	}
	return NewTradingRecordHandler(ctxFn, trading.NewService(sqlite.NewStockRepository(), priceFn), emit)
}

// AddTradingRecord 添加交易记录
func (h *TradingRecordHandler) AddTradingRecord(record data.TradingRecord) (uint, error) {
	return h.svc.AddTradingRecord(h.currentCtx(), sqlite.TradingRecordToDomain(&record))
}

// GetTradingRecordList 获取交易记录列表（分页与筛选，返回结构与 AI 推荐列表一致）。
// 查询失败时返回 error，让前端自动刷新能感知，而不是拿到空列表静默失败。
func (h *TradingRecordHandler) GetTradingRecordList(query data.TradingRecordListQuery) (*data.TradingRecordPageData, error) {
	page, err := h.svc.GetTradingRecordList(h.currentCtx(), sqlite.TradingRecordListQueryToDomain(query))
	if err != nil {
		return nil, err
	}
	return sqlite.TradingRecordPageDataFromDomain(&page), nil
}

// GetTradingRecordById 根据ID获取单个交易记录
func (h *TradingRecordHandler) GetTradingRecordById(id uint) (*data.TradingRecord, error) {
	record, err := h.svc.GetTradingRecordById(h.currentCtx(), id)
	if err != nil || record == nil {
		return nil, err
	}
	return sqlite.TradingRecordFromDomain(record), nil
}

// GetTradingRecordStatistics 获取交易记录统计数据
func (h *TradingRecordHandler) GetTradingRecordStatistics() *data.TradingRecordStatistics {
	stats, err := h.svc.GetTradingRecordStatistics(h.currentCtx())
	if err != nil {
		return &data.TradingRecordStatistics{}
	}
	return &data.TradingRecordStatistics{
		TotalBuyAmount:  stats.TotalBuyAmount,
		TotalSellAmount: stats.TotalSellAmount,
		TotalProfit:     stats.TotalProfit,
		ProfitRate:      stats.ProfitRate,
		HoldingsAmount:  stats.HoldingsAmount,
		CurrentValue:    stats.CurrentValue,
		StockCount:      stats.StockCount,
	}
}

// UpdateTradingRecord 更新交易记录
func (h *TradingRecordHandler) UpdateTradingRecord(record data.TradingRecord) error {
	return h.svc.UpdateTradingRecord(h.currentCtx(), sqlite.TradingRecordToDomain(&record))
}

// DeleteTradingRecord 删除交易记录
func (h *TradingRecordHandler) DeleteTradingRecord(id uint) error {
	return h.svc.DeleteTradingRecord(h.currentCtx(), id)
}

// CheckFrequentTrading 检查是否频繁交易
func (h *TradingRecordHandler) CheckFrequentTrading(stockCode string) map[string]any {
	canTrade, msg := h.svc.CheckFrequentTrading(h.currentCtx(), stockCode)
	return map[string]any{
		"canTrade": canTrade,
		"msg":      msg,
	}
}

// ---------------------------------------------------------------------------
// 持仓明细 / 技术指标 / AI 每日总结
// ---------------------------------------------------------------------------

// GetHoldingsDetail 逐股持仓明细（FIFO 推算 + 实时现价；现价获取失败时相关字段为 0）
func (h *TradingRecordHandler) GetHoldingsDetail() ([]data.HoldingsPosition, error) {
	positions, err := h.svc.GetHoldingsDetail(h.currentCtx())
	if err != nil {
		return nil, err
	}
	result := make([]data.HoldingsPosition, 0, len(positions))
	for _, p := range positions {
		result = append(result, data.HoldingsPosition{
			StockCode:     p.StockCode,
			StockName:     p.StockName,
			Volume:        p.Volume,
			CostPrice:     p.CostPrice,
			CostAmount:    p.CostAmount,
			CurrentPrice:  p.CurrentPrice,
			MarketValue:   p.MarketValue,
			ProfitAmount:  p.ProfitAmount,
			ProfitPercent: p.ProfitPercent,
		})
	}
	return result, nil
}

// GetStockTechnicalIndicators 单股全套技术指标数值与文字解读
func (h *TradingRecordHandler) GetStockTechnicalIndicators(code string) (*data.StockIndicatorsResult, error) {
	if strings.TrimSpace(code) == "" {
		return nil, fmt.Errorf("股票代码不能为空")
	}
	return data.GetHoldingsStockIndicators(h.currentCtx(), code)
}

// summaryEventName 持仓总结流式事件名（前端可自定义，空则用默认值）
func (h *TradingRecordHandler) summaryEventName(eventName string) string {
	if strings.TrimSpace(eventName) == "" {
		return "tradingHoldingsSummary"
	}
	return eventName
}

// SummarizeHoldings 流式生成持仓 AI 每日总结：逐段 EventsEmit(eventName, msg) 推送，
// 结束发 eventName="DONE"；生成完成自动按天落库（同日覆盖），中断或空内容不落库。
func (h *TradingRecordHandler) SummarizeHoldings(aiConfigId int, eventName string) {
	eventName = h.summaryEventName(eventName)
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("SummarizeHoldings panic: %v", r)
			h.emit(eventName, map[string]any{
				"code":    0,
				"content": fmt.Sprintf("持仓AI总结异常: %v", r),
			})
			h.emit(eventName, "DONE")
		}
	}()

	positions, err := h.GetHoldingsDetail()
	if err != nil {
		h.emit(eventName, map[string]any{
			"code":    0,
			"content": "获取持仓明细失败: " + err.Error(),
		})
		h.emit(eventName, "DONE")
		return
	}
	if len(positions) == 0 {
		h.emit(eventName, map[string]any{
			"code":    0,
			"content": "当前没有任何持仓记录，请先在交易日志中添加买入记录后再进行 AI 总结。",
		})
		h.emit(eventName, "DONE")
		return
	}

	// 逐股取技术指标摘要（个别失败标注后继续，不阻塞整体总结）
	indicatorSections := make([]string, 0, len(positions))
	for _, p := range positions {
		section := fmt.Sprintf("### %s(%s)\n", p.StockName, p.StockCode)
		ind, indErr := data.GetHoldingsStockIndicators(h.currentCtx(), p.StockCode)
		if indErr != nil || ind == nil || ind.Summary == nil {
			section += "- 技术指标获取失败\n"
			logger.SugaredLogger.Warnf("获取 %s(%s) 技术指标失败: %v", p.StockName, p.StockCode, indErr)
		} else {
			s := ind.Summary
			section += fmt.Sprintf("- 趋势：%s；MACD：%s；RSI14：%.1f（%s）；KDJ：%s；布林位置：%s\n- 解读：%s\n",
				s.Trend, s.MACDSignal, s.RSIValue, s.RSIStatus, s.KDJSignal, s.BollStatus, s.Summary)
		}
		indicatorSections = append(indicatorSections, section)
	}

	ctx, cancel := context.WithCancel(h.currentCtx())
	h.summaryMu.Lock()
	if h.summaryCancel != nil {
		h.summaryCancel()
	}
	h.summaryCancel = cancel
	h.summaryMu.Unlock()

	ai := data.NewDeepSeekOpenAi(ctx, aiConfigId)
	msgs := ai.NewSummaryStockNewsStream(buildHoldingsSummaryPrompt(positions, indicatorSections), nil, false, nil)

	var full strings.Builder
	for msg := range msgs {
		if content, ok := msg["content"].(string); ok {
			full.WriteString(content)
		}
		h.emit(eventName, msg)
	}

	// 被中断（ctx 取消）或空内容不落库
	if ctx.Err() == nil && full.Len() > 0 {
		h.saveHoldingsSummary(aiConfigId, positions, full.String())
	}

	h.summaryMu.Lock()
	h.summaryCancel = nil
	h.summaryMu.Unlock()

	h.emit(eventName, "DONE")
}

// AbortSummarizeHoldings 取消进行中的持仓总结流（不落库）
func (h *TradingRecordHandler) AbortSummarizeHoldings() {
	h.summaryMu.Lock()
	defer h.summaryMu.Unlock()
	if h.summaryCancel != nil {
		h.summaryCancel()
		h.summaryCancel = nil
	}
}

// GetHoldingsSummaryList 持仓总结历史列表（按日期倒序，Content 为预览）
func (h *TradingRecordHandler) GetHoldingsSummaryList(page, pageSize int) (*data.HoldingsSummaryPageData, error) {
	return data.GetHoldingsSummaryList(page, pageSize)
}

// GetHoldingsSummaryDetail 单条持仓总结全文（含持仓快照）
func (h *TradingRecordHandler) GetHoldingsSummaryDetail(id uint) (*models.HoldingsDailySummary, error) {
	return data.GetHoldingsSummaryDetail(id)
}

// saveHoldingsSummary 总结落库（按天覆盖），并附带生成时的持仓快照与汇总数据
func (h *TradingRecordHandler) saveHoldingsSummary(aiConfigId int, positions []data.HoldingsPosition, content string) {
	totalCost := 0.0
	totalMarket := 0.0
	for _, p := range positions {
		totalCost += p.CostAmount
		totalMarket += p.MarketValue
	}
	snapshot, err := json.Marshal(positions)
	if err != nil {
		logger.SugaredLogger.Warnf("序列化持仓快照失败: %v", err)
	}
	summary := &models.HoldingsDailySummary{
		SummaryDate:      time.Now().Format("2006-01-02"),
		Content:          content,
		ModelName:        data.NewDeepSeekOpenAi(h.currentCtx(), aiConfigId).GetModel(),
		HoldingsSnapshot: string(snapshot),
		StockCount:       int64(len(positions)),
		TotalProfit:      totalMarket - totalCost,
	}
	if totalCost > 0 {
		summary.ProfitRate = summary.TotalProfit / totalCost * 100
	}
	if err := data.SaveHoldingsDailySummary(summary); err != nil {
		logger.SugaredLogger.Errorf("保存持仓每日总结失败: %s", err.Error())
	}
}

// buildHoldingsSummaryPrompt 组合持仓每日总结 prompt：持仓明细表 + 各股指标摘要 + 输出结构要求
func buildHoldingsSummaryPrompt(positions []data.HoldingsPosition, indicatorSections []string) string {
	var sb strings.Builder
	sb.WriteString("你是一位专业、务实的证券投资分析师。请基于以下真实持仓数据与技术指标，生成一份详细、丰富、观点鲜明的当日持仓总结（今日日期：")
	sb.WriteString(time.Now().Format("2006-01-02"))
	sb.WriteString("）。\n\n## 当前持仓明细\n\n")
	sb.WriteString("| 代码 | 名称 | 持仓数量 | 成本价 | 最新价 | 市值 | 浮动盈亏 | 盈亏率 |\n")
	sb.WriteString("|---|---|---|---|---|---|---|---|\n")
	for _, p := range positions {
		price := fmt.Sprintf("%.2f", p.CurrentPrice)
		pnl := fmt.Sprintf("%.2f", p.ProfitAmount)
		pct := "-"
		if p.CurrentPrice > 0 {
			pct = fmt.Sprintf("%.2f%%", p.ProfitPercent)
		} else {
			price = "获取失败"
			pnl = "-"
		}
		fmt.Fprintf(&sb, "| %s | %s | %d | %.3f | %s | %.2f | %s | %s |\n",
			p.StockCode, p.StockName, p.Volume, p.CostPrice, price, p.MarketValue, pnl, pct)
	}
	sb.WriteString("\n## 各持仓技术指标摘要（日线）\n")
	for _, section := range indicatorSections {
		sb.WriteString(section)
	}
	sb.WriteString(`
## 输出要求
请输出 markdown 格式的持仓每日总结，结构如下：
1. **组合整体概述**：总成本、总市值、整体浮动盈亏与盈亏率，以及仓位集中/分散程度的整体评价；
2. **个股逐一解读**：每只持仓单独一小节，结合上方技术指标给出趋势判断、短线支撑与压力参考，以及明确的操作观点（继续持有/逢高减仓/逢低加仓/密切观察等）；
3. **总结性观点**：必须给出明确、不模棱两可的组合层面结论——整体仓位是否合理、主要风险点、明日重点关注事项、具体操作建议；
4. **风险提示**：一句话提示市场风险。

要求：内容详细丰富、逻辑清晰、观点鲜明；避免空话套话；所有判断必须与给出的持仓和指标数据保持一致。`)
	return sb.String()
}

// currentCtx returns the Wails app context (set after startup), falling back
// to context.Background when not wired — so in-flight service calls observe
// app shutdown instead of running detached.
func (h *TradingRecordHandler) currentCtx() context.Context {
	if h.ctxFn != nil {
		return h.ctxFn()
	}
	return context.Background()
}
