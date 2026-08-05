package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/cloudwego/eino/schema"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/robfig/cron/v3"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"go-stock/backend/agent"
	"go-stock/backend/agent/commodity"
	"go-stock/backend/agent/multi"
	"go-stock/backend/agent/strategy"
	"go-stock/backend/data"
	"go-stock/backend/data/notify"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// AgentHandler handles AI-agent-related Wails bindings.
type AgentHandler struct {
	ctxFn    func() context.Context
	cron     *cron.Cron
	cronMu   sync.Mutex
	cronJobs map[string]cron.EntryID

	agentMu     sync.Mutex
	agentCancel context.CancelFunc

	summaryMu     sync.Mutex
	summaryCancel context.CancelFunc

	tools []data.Tool
}

// NewAgentHandler creates a new AgentHandler.
// ctxFn should return the current App context (set after Wails startup).
func NewAgentHandler(ctxFn func() context.Context) *AgentHandler {
	c := cron.New(cron.WithSeconds(), cron.WithChain(cron.Recover(cron.DefaultLogger)))
	c.Start()

	var tools []data.Tool
	tools = data.Tools(tools)

	return &AgentHandler{
		ctxFn:    ctxFn,
		cron:     c,
		cronJobs: make(map[string]cron.EntryID),
		tools:    tools,
	}
}

func (h *AgentHandler) currentCtx() context.Context {
	if h.ctxFn != nil {
		return h.ctxFn()
	}
	return context.Background()
}

func (h *AgentHandler) setCronEntry(key string, id cron.EntryID) {
	h.cronMu.Lock()
	h.cronJobs[key] = id
	h.cronMu.Unlock()
}

func (h *AgentHandler) getCronEntry(key string) (cron.EntryID, bool) {
	h.cronMu.Lock()
	id, exists := h.cronJobs[key]
	h.cronMu.Unlock()
	return id, exists
}

func (h *AgentHandler) ChatWithAgent(question string, aiConfigId int, sysPromptId *int, memoryMode bool, memoryCount int, thinkingMode bool, agentMode string) {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("ChatWithAgent panic: %v", r)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	h.agentMu.Lock()
	if h.agentCancel != nil {
		h.agentCancel()
	}
	h.agentCancel = cancel
	h.agentMu.Unlock()

	defer func() {
		h.agentMu.Lock()
		h.agentCancel = nil
		h.agentMu.Unlock()
	}()

	ch := agent.NewStockAiAgentApi().ChatWithContext(ctx, question, aiConfigId, sysPromptId, memoryMode, memoryCount, thinkingMode, agentMode)
	for msg := range ch {
		runtime.EventsEmit(h.currentCtx(), "agent-message", agentMessageToFrontendMap(msg))
	}
	runtime.EventsEmit(h.currentCtx(), "agent-message", agentMessageToFrontendMap(&schema.Message{
		Role:    schema.Assistant,
		Content: "agent-DONE",
	}))
}

// agentMessageToFrontendMap 用标准 JSON 将 schema.Message 转为 map 再 EventsEmit，
// 保证与 json 标签一致（如 reasoning_content、extra），避免 Wails 直接传结构体时前端字段名不一致。
func agentMessageToFrontendMap(msg *schema.Message) map[string]any {
	if msg == nil {
		return map[string]any{}
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return map[string]any{
			"role":              string(msg.Role),
			"content":           msg.Content,
			"reasoning_content": msg.ReasoningContent,
		}
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]any{
			"role":              string(msg.Role),
			"content":           msg.Content,
			"reasoning_content": msg.ReasoningContent,
		}
	}
	return m
}

func (h *AgentHandler) AbortChatWithAgent() {
	h.agentMu.Lock()
	defer h.agentMu.Unlock()
	if h.agentCancel != nil {
		h.agentCancel()
		h.agentCancel = nil
	}
}

func (h *AgentHandler) NewChatStream(stock string, stockCode string, question string, aiConfigId int, sysPromptId *int, enableTools bool, think bool, agentMode string, strategyCode string) {
	defer func() {
		if err := recover(); err != nil {
			logger.SugaredLogger.Errorf("NewChatStream panic: %v", err)
			runtime.EventsEmit(h.currentCtx(), "newChatStream", map[string]any{
				"code":    0,
				"content": fmt.Sprintf("AI分析异常: %v", err),
			})
			runtime.EventsEmit(h.currentCtx(), "newChatStream", "DONE")
		}
	}()
	// Use the multi-agent engine as the primary analysis path
	engine := multi.NewMultiAgentEngine(aiConfigId)
	resultCh := engine.Run(h.currentCtx(), stockCode, stock, "", question, strategyCode)

	for msg := range resultCh {
		runtime.EventsEmit(h.currentCtx(), "newChatStream", msg)
	}
	runtime.EventsEmit(h.currentCtx(), "newChatStream", "DONE")

	// Send push notification after analysis completes
	manager := notify.NewManager()
	manager.SendAll(h.currentCtx(), notify.Message{
		Title:   fmt.Sprintf("AI分析报告: %s(%s)", stock, stockCode),
		Content: fmt.Sprintf("股票: %s(%s)\n问题: %s\n\n请打开 go-stock 查看完整分析报告。", stock, stockCode, question),
		Stock:   stockCode,
	})
}

func (h *AgentHandler) NewCommodityAnalysisStream(code string, name string, question string, aiConfigId int) {
	defer func() {
		if err := recover(); err != nil {
			logger.SugaredLogger.Errorf("NewCommodityAnalysisStream panic: %v", err)
			runtime.EventsEmit(h.currentCtx(), "commodityAnalysisStream", map[string]any{
				"code":    0,
				"content": fmt.Sprintf("商品分析异常: %v", err),
			})
			runtime.EventsEmit(h.currentCtx(), "commodityAnalysisStream", "DONE")
		}
	}()

	engine := commodity.NewCommodityEngine(aiConfigId)
	resultCh := engine.Run(h.currentCtx(), code, name, question)

	for msg := range resultCh {
		runtime.EventsEmit(h.currentCtx(), "commodityAnalysisStream", msg)
	}
	runtime.EventsEmit(h.currentCtx(), "commodityAnalysisStream", "DONE")
}

func (h *AgentHandler) GetAllStrategies() []*strategy.Strategy {
	return strategy.GetAll()
}

func (h *AgentHandler) SummaryStockNews(question string, aiConfigId int, sysPromptId *int, enableTools bool, think bool, eventName string, historyJSON string) {
	ctx, cancel := context.WithCancel(h.currentCtx())

	// 保存当前会话的 cancel，用于前端中断
	h.summaryMu.Lock()
	if h.summaryCancel != nil {
		h.summaryCancel()
	}
	h.summaryCancel = cancel
	h.summaryMu.Unlock()

	// 允许前端自定义事件名，避免不同页面之间的事件冲突
	if strings.TrimSpace(eventName) == "" {
		eventName = "summaryStockNews"
	}

	// 解析对话历史（AI 助手记忆）：空字符串或解析失败则无历史
	var history []map[string]interface{}
	if strings.TrimSpace(historyJSON) != "" {
		var list []models.AiAssistantMessage
		if err := json.Unmarshal([]byte(historyJSON), &list); err == nil && len(list) > 0 {
			history = make([]map[string]interface{}, 0, len(list))
			for _, m := range list {
				item := map[string]interface{}{"role": m.Role, "content": m.Content}
				if m.Role == "assistant" && m.Reasoning != "" {
					item["reasoning_content"] = m.Reasoning
				}
				history = append(history, item)
			}
		}
	}

	var msgs <-chan map[string]any
	if enableTools {
		msgs = data.NewDeepSeekOpenAi(ctx, aiConfigId).NewSummaryStockNewsStreamWithTools(question, sysPromptId, h.tools, think, history)
	} else {
		msgs = data.NewDeepSeekOpenAi(ctx, aiConfigId).NewSummaryStockNewsStream(question, sysPromptId, think, history)
	}

	for msg := range msgs {
		runtime.EventsEmit(h.currentCtx(), eventName, msg)
	}

	h.summaryMu.Lock()
	h.summaryCancel = nil
	h.summaryMu.Unlock()

	runtime.EventsEmit(h.currentCtx(), eventName, "DONE")
}

// AbortSummaryStockNews 取消当前进行中的 SummaryStockNews 流式回答
func (h *AgentHandler) AbortSummaryStockNews() {
	h.summaryMu.Lock()
	defer h.summaryMu.Unlock()
	if h.summaryCancel != nil {
		h.summaryCancel()
		h.summaryCancel = nil
	}
}

func (h *AgentHandler) SetStockAICron(cronText, stockCode string) {
	data.NewStockDataApi().SetStockAICron(cronText, stockCode)
	if strutil.HasPrefixAny(stockCode, []string{"gb_"}) {
		stockCode = strings.ToUpper(stockCode)
		stockCode = strings.Replace(stockCode, "gb_", "us", 1)
		stockCode = strings.Replace(stockCode, "GB_", "us", 1)
	}
	if entryID, exists := h.getCronEntry(stockCode); exists {
		h.cron.Remove(entryID)
	}
	follow := data.NewStockDataApi().GetFollowedStockByStockCode(stockCode)
	id, _ := h.cron.AddFunc(cronText, h.addCronTask(follow))
	h.setCronEntry(stockCode, id)
}

// addCronTask 创建单只关注股票的自动分析任务（从 app.go 复制为私有辅助函数）。
func (h *AgentHandler) addCronTask(follow data.FollowedStock) func() {
	return func() {
		go runtime.EventsEmit(h.currentCtx(), "warnMsg", "开始自动分析"+follow.Name+"_"+follow.StockCode)
		ai := data.NewDeepSeekOpenAi(h.currentCtx(), follow.AiConfigId)
		thinking := data.GetSettingConfig().GetAIConfigThinking(follow.AiConfigId)
		msgs := ai.NewChatStream(follow.Name, follow.StockCode, "", nil, h.tools, thinking)
		var res strings.Builder

		chatId := ""
		question := ""
		for msg := range msgs {
			if v, ok := msg["extraContent"].(string); ok && v != "" {
				res.WriteString(v + "\n")
			}
			if v, ok := msg["content"].(string); ok && v != "" {
				res.WriteString(v)
			}
			if v, ok := msg["chatId"].(string); ok {
				chatId = v
			}
			if v, ok := msg["question"].(string); ok {
				question = v
			}
		}

		data.NewDeepSeekOpenAi(h.currentCtx(), follow.AiConfigId).SaveAIResponseResult(follow.StockCode, follow.Name, res.String(), chatId, question)
		go runtime.EventsEmit(h.currentCtx(), "warnMsg", "AI分析完成："+follow.Name+"_"+follow.StockCode)
	}
}
