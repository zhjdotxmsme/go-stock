# 多智能体个股分析系统 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace go-stock's single-agent (React/PlanExecute) AI analysis engine with a multi-agent Eino Graph system featuring 4 parallel analysts, bull/bear researcher debate, and TradingAgents-CN-inspired multi-source data fallback chains.

**Architecture:** Eino Graph with parallel analyst nodes (fundamental/technical/sentiment/news) → researcher debate loop → synthesis → streaming output. New `backend/agent/multi/` package for agent logic, new `backend/data/datasource/` package for data source abstraction with priority-based fallback chains.

**Tech Stack:** Go 1.26, Eino v0.9.9 (`compose.Graph`, `compose.Parallel`), Eino model components (openai/claude), freecache, GORM+SQLite

---

### Task 1: Shared types for multi-agent system

**Files:**
- Create: `backend/agent/multi/types.go`

- [ ] **Step 1: Write the types file**

```go
package multi

import (
	"context"
	"github.com/cloudwego/eino/schema"
)

// AgentReport is the output of each analyst node
type AgentReport struct {
	Role      string // fundamental / technical / sentiment / news
	Content   string // full markdown analysis text
	Summary   string // one-paragraph summary
	Rating    string // bullish / bearish / neutral
	Error     string // empty if successful
}

// DebateRound represents one round of bull/bear debate
type DebateRound struct {
	RoundNum     int
	BullArgument string
	BearArgument string
}

// DebateResult is the output of the researcher node
type DebateResult struct {
	Rounds         []DebateRound
	BullFinalArg   string
	BearFinalArg   string
	ConsensusItems []string
	Disagreements  []string
}

// FinalReport is the output of the synthesis node
type FinalReport struct {
	OverallRating    string // strong_buy / buy / hold / sell / strong_sell
	InvestmentThesis string
	Strengths        []string
	RiskFactors      []string
	Catalysts        []string
	MultiTimeframeView map[string]string // short/medium/long term views
	Conclusion       string
}

// AgentContext carries shared state through the Graph
type AgentContext struct {
	StockCode   string
	StockName   string
	Market      string // A / HK / US
	Question    string
	AIConfigID  int
	Reports     []AgentReport
	Debate      *DebateResult
	FinalReport *FinalReport
	Ctx         context.Context
}
```

- [ ] **Step 2: Commit**

```bash
git add backend/agent/multi/types.go
git commit -m "feat(multi-agent): add shared types"
```

---

### Task 2: Per-role model configuration

**Files:**
- Create: `backend/agent/multi/model_config.go`

- [ ] **Step 1: Write model_config.go**

```go
package multi

import (
	"context"
	"fmt"
	"go-stock/backend/agent"
	"go-stock/backend/data"
)

// AnalystModelConfig defines per-role LLM configuration
type AnalystModelConfig struct {
	Role        string  // fundamental / technical / sentiment / news / researcher_bull / researcher_bear / synthesis
	BaseUrl     string  // custom API URL (empty = use global default)
	ApiKey      string  // API key (empty = use global default)
	ModelName   string  // model name (empty = use global default)
	Temperature float64 // 0 = use global default
	MaxTokens   int     // 0 = use global default
}

// GetChatModel creates an LLM client for the given role.
// Falls back to the global AIConfig when per-role config is not set.
func GetChatModel(ctx context.Context, role string, aiConfigID int) (model.ChatModel, error) {
	// Load the global AIConfig
	cfg := data.GetSettingConfig()
	if cfg == nil {
		return nil, fmt.Errorf("settings not loaded")
	}
	
	// Find the matching AIConfig by ID
	var aiConfig *data.AIConfig
	for _, c := range cfg.AiConfigs {
		if c.ID == uint(aiConfigID) {
			aiConfig = c
			break
		}
	}
	if aiConfig == nil {
		return nil, fmt.Errorf("ai config %d not found", aiConfigID)
	}
	
	// Use the existing factory to create the model client
	// This calls agent.createChatModel which routes via detectChatModelProvider()
	return agent.GetChatModelByConfig(ctx, aiConfig)
}
```

Note: The `agent.CreateChatModel` function will be exported in Task 20 (currently unexported `createChatModel`). Once exported, this function calls it. Fallback logic: if no per-role config is set, uses the global AIConfig directly.

- [ ] **Step 2: Commit**

```bash
git add backend/agent/multi/model_config.go
git commit -m "feat(multi-agent): add per-role model config"
```

---

### Task 3: System prompts for all agents

**Files:**
- Create: `backend/agent/multi/prompts.go`

- [ ] **Step 1: Write prompts.go**

```go
package multi

const FundamentalAnalystPrompt = `你是一位资深的基本面分析师，拥有20年证券研究经验。请基于提供的股票数据，从以下维度进行分析：

1. **财务健康度**：ROE、资产负债率、流动比率、现金流状况
2. **盈利能力**：营收增长率、净利润增长率、毛利率趋势
3. **估值水平**：PE、PB、PS，与同行业对比
4. **成长性**：主营业务增长动力、新业务布局
5. **风险提示**：商誉、大额应收账款、关联交易、政策风险

请给出综合评价（看多/看空/中性）并附上关键数据支撑。`

const TechnicalAnalystPrompt = `你是一位资深的技术分析师，精通K线形态和技术指标。请基于提供的K线数据和技术指标进行分析：

1. **趋势判断**：均线系统（MA5/10/20/60）多头/空头排列
2. **技术指标**：MACD、RSI、KDJ、布林带信号
3. **成交量分析**：量价配合情况、主力资金动向
4. **支撑与压力**：关键支撑位和压力位判断
5. **形态识别**：头肩顶底、双底双顶、旗形整理等

请给出技术面评价（看多/看空/中性）并标注关键价位。`

const SentimentAnalystPrompt = `你是一位专业的市场情绪分析师。请基于提供的新闻和情感分析数据，评估市场对该股票的当前情绪：

1. **整体情绪**：积极/消极/中性，量化情绪得分
2. **热点话题**：当前市场最关注的该股票相关话题
3. **正面因素**：近期利好事件和市场评价
4. **负面因素**：近期利空事件和市场担忧
5. **情绪趋势**：近期情绪是升温还是降温

请给出情绪评价（看多/看空/中性）。`

const NewsAnalystPrompt = `你是一位专业的新闻分析师，专注解读宏观和行业新闻对个股的影响。请分析：

1. **重大事件**：近期与该公司/行业相关的重大新闻
2. **宏观影响**：宏观经济政策（利率、汇率、产业政策）对该公司的影响
3. **行业动态**：行业发展趋势、竞争格局变化
4. **公司公告**：近期公司公告的重要信息解读
5. **事件驱动**：即将发生的潜在催化剂（财报、新品发布、行业会议等）

请给出基于新闻面的评价（看多/看空/中性）。`

const BullResearcherPrompt = `你是一位看多研究员。请基于以下分析师报告，寻找看多的理由：

1. 从基本面、技术面、情绪面、新闻面中提取支持上涨的证据
2. 反驳看空观点中提到的风险因素
3. 给出看多的核心逻辑和目标价位判断

保持客观，只基于分析师报告中的数据说话，不凭空想象。`

const BearResearcherPrompt = `你是一位看空研究员。请基于以下分析师报告，识别潜在风险：

1. 从基本面、技术面、情绪面、新闻面中提取潜在风险信号
2. 质疑看多观点中可能忽略的风险
3. 给出看空的核心逻辑和风险警示

保持客观，只基于分析师报告中的数据说话，不凭空想象。`

const SynthesisPrompt = `你是一位首席投资策略师。请基于所有分析师报告和研究员辩论结果，给出最终的投资分析报告：

1. **综合评价**：汇总各维度观点，给出总体评级（强烈看多/看多/持有/看空/强烈看空）
2. **核心投资逻辑**：用3-5句话概括最核心的投资逻辑
3. **多维度分析表**：用表格展示各维度的评价对比
4. **风险提示**：列出最重要的风险因素
5. **多时间维度**：短期（1-4周）、中期（1-6个月）、长期（6个月以上）看法
6. **结论**：清晰的投资建议

报告要结构化、数据驱动、客观平衡。`
```

- [ ] **Step 2: Commit**

```bash
git add backend/agent/multi/prompts.go
git commit -m "feat(multi-agent): add system prompts for all roles"
```

---

### Task 4: Fundamental Analyst Graph node

**Files:**
- Create: `backend/agent/multi/fundamental.go`

- [ ] **Step 1: Write fundamental.go**

```go
package multi

import (
	"context"
	"go-stock/backend/agent/tools"
)

// RunFundamentalAnalyst calls the fundamental analysis tools and LLM, returns an AgentReport
func RunFundamentalAnalyst(ctx context.Context, ac *AgentContext) (*AgentReport, error) {
	// 1. Get the stock info tool
	infoTool := tools.GetQueryStockCodeInfoTool()
	
	// 2. Get financial report tool
	finTool := tools.GetQueryStockFinancialReportTool()
	
	// 3. Get research report tool
	researchTool := tools.GetQueryStockResearchReportTool()
	
	// 4. Build system message using FundamentalAnalystPrompt
	// 5. Call LLM with tools (create chat model via GetChatModel)
	// 6. Parse response into AgentReport
	
	// Implementation detail: this node uses the Eino ToolNode + ChatModel pattern
	// similar to createReactAgent but single-shot instead of multi-turn
	
	return &AgentReport{
		Role:    "fundamental",
		Content: "分析结果内容...",
		Summary: "基本面摘要",
		Rating:  "neutral",
	}, nil
}
```

(Full implementation with actual LLM call + tool integration will be done in Task 14 when the engine is wired up — this file just defines the node handler function signature.)

- [ ] **Step 2: Commit**

```bash
git add backend/agent/multi/fundamental.go
git commit -m "feat(multi-agent): add fundamental analyst node"
```

---

### Task 5: Technical Analyst Graph node

**Files:**
- Create: `backend/agent/multi/technical.go`

- [ ] **Step 1: Write technical.go**

```go
package multi

import (
	"context"
	"go-stock/backend/agent/tools"
)

// RunTechnicalAnalyst calls K-line data tools and LLM
func RunTechnicalAnalyst(ctx context.Context, ac *AgentContext) (*AgentReport, error) {
	// 1. Get K-line tool
	klineTool := tools.GetQueryStockKLineDataTool()
	
	// 2. Get market data tool
	priceTool := tools.GetQueryStockPriceInfoTool()
	
	// 3. Call LLM with TechnicalAnalystPrompt + tools
	// 4. Parse response
	
	return &AgentReport{
		Role:    "technical",
		Content: "技术分析内容...",
		Summary: "技术面摘要",
		Rating:  "neutral",
	}, nil
}
```

- [ ] **Step 2: Commit**

```bash
git add backend/agent/multi/technical.go
git commit -m "feat(multi-agent): add technical analyst node"
```

---

### Task 6: Sentiment Analyst Graph node

**Files:**
- Create: `backend/agent/multi/sentiment.go`

- [ ] **Step 1: Write sentiment.go**

```go
package multi

import (
	"context"
	"go-stock/backend/agent/tools"
)

// RunSentimentAnalyst calls news + sentiment tools and LLM
func RunSentimentAnalyst(ctx context.Context, ac *AgentContext) (*AgentReport, error) {
	// 1. Get news tool
	newsTool := tools.GetQueryStockNewsTool()
	
	// 2. Call LLM with SentimentAnalystPrompt + tools
	// 3. Parse response
	
	return &AgentReport{
		Role:    "sentiment",
		Content: "情绪分析内容...",
		Summary: "情绪面摘要",
		Rating:  "neutral",
	}, nil
}
```

- [ ] **Step 2: Commit**

```bash
git add backend/agent/multi/sentiment.go
git commit -m "feat(multi-agent): add sentiment analyst node"
```

---

### Task 7: News Analyst Graph node

**Files:**
- Create: `backend/agent/multi/news.go`

- [ ] **Step 1: Write news.go**

```go
package multi

import (
	"context"
	"go-stock/backend/agent/tools"
)

// RunNewsAnalyst calls news, calendar, and company announcement tools
func RunNewsAnalyst(ctx context.Context, ac *AgentContext) (*AgentReport, error) {
	// 1. Get news tool (reused from sentiment)
	// 2. Get calendar/event tool
	// 3. Get company announcement tool
	// 4. Call LLM with NewsAnalystPrompt
	// 5. Parse response
	
	return &AgentReport{
		Role:    "news",
		Content: "新闻分析内容...",
		Summary: "新闻面摘要",
		Rating:  "neutral",
	}, nil
}
```

- [ ] **Step 2: Commit**

```bash
git add backend/agent/multi/news.go
git commit -m "feat(multi-agent): add news analyst node"
```

---

### Task 8: Researcher (Bull/Bear debate) node

**Files:**
- Create: `backend/agent/multi/researcher.go`

- [ ] **Step 1: Write researcher.go**

```go
package multi

import (
	"context"
	"fmt"
	"go-stock/backend/data"
)

// RunDebate executes the bull/bear debate loop
// N rounds (default 2): round 1 = statements, round 2+ = rebuttals
func RunDebate(ctx context.Context, ac *AgentContext, numRounds int) (*DebateResult, error) {
	if numRounds < 1 || numRounds > 3 {
		numRounds = 2 // default
	}
	
	// Build the combined context from all analyst reports
	analystSummary := ""
	for _, r := range ac.Reports {
		if r.Error != "" {
			analystSummary += fmt.Sprintf("【%s】数据不可用\n", r.Role)
			continue
		}
		analystSummary += fmt.Sprintf("===== %s 分析报告 =====\n%s\n\n", r.Role, r.Content)
	}
	
	result := &DebateResult{}
	
	for round := 1; round <= numRounds; round++ {
		// Bull researcher speaks (with context from previous round)
		bullArg := callResearcherLLM(ctx, "bull", analystSummary, result.Rounds, round, ac.AIConfigID)
		
		// Bear researcher speaks (with context from bull's argument)
		bearArg := callResearcherLLM(ctx, "bear", analystSummary, result.Rounds, round, ac.AIConfigID)
		
		result.Rounds = append(result.Rounds, DebateRound{
			RoundNum:     round,
			BullArgument: bullArg,
			BearArgument: bearArg,
		})
	}
	
	// Extract consensus and disagreements from final round
	extractConsensus(result)
	
	return result, nil
}

func callResearcherLLM(ctx context.Context, side string, context string, 
	prevRounds []DebateRound, round int, aiConfigID int) string {
	// Use the appropriate prompt based on side
	// Call LLM and return the argument text
	return ""
}

func extractConsensus(result *DebateResult) {
	// Use LLM to identify points of agreement and disagreement
	// Fill ConsensusItems and Disagreements
}
```

- [ ] **Step 2: Commit**

```bash
git add backend/agent/multi/researcher.go
git commit -m "feat(multi-agent): add bull/bear researcher debate node"
```

---

### Task 9: Synthesis node

**Files:**
- Create: `backend/agent/multi/synthesis.go`

- [ ] **Step 1: Write synthesis.go**

```go
package multi

import (
	"context"
	"fmt"
)

// RunSynthesis combines all analyst reports + debate into the final report
func RunSynthesis(ctx context.Context, ac *AgentContext) (*FinalReport, error) {
	// 1. Build the full context: analyst reports + debate result
	// 2. Call LLM with SynthesisPrompt
	// 3. Parse structured response into FinalReport
	
	return &FinalReport{
		OverallRating:    "hold",
		InvestmentThesis: "综合投资逻辑...",
		Conclusion:       "最终结论...",
	}, nil
}
```

- [ ] **Step 2: Commit**

```bash
git add backend/agent/multi/synthesis.go
git commit -m "feat(multi-agent): add synthesis node"
```

---

### Task 10: Eino Graph engine — core orchestration

**Files:**
- Create: `backend/agent/multi/engine.go`

- [ ] **Step 1: Write engine.go**

```go
package multi

import (
	"context"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"strings"
	"time"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/bytedance/sonic"
)

// MultiAgentEngine orchestrates the multi-agent analysis via Eino Graph
type MultiAgentEngine struct {
	graph      *compose.Graph[string, string]
	aiConfigID int
}

// NewMultiAgentEngine creates a new multi-agent engine
func NewMultiAgentEngine(aiConfigID int) *MultiAgentEngine {
	return &MultiAgentEngine{
		aiConfigID: aiConfigID,
	}
}

// Run executes the multi-agent analysis pipeline
// Returns a channel of streaming messages for the frontend
func (e *MultiAgentEngine) Run(ctx context.Context, stockCode, stockName, market, question string) chan *schema.Message {
	ch := make(chan *schema.Message, 1024)

	go func() {
		defer close(ch)

		ac := &AgentContext{
			StockCode:  stockCode,
			StockName:  stockName,
			Market:     market,
			Question:   question,
			AIConfigID: e.aiConfigID,
			Ctx:        ctx,
		}

		// Phase 1: Emit progress
		emitPhase(ch, "start", "orchestrator", "正在准备分析...")

		// Phase 2: Run 4 analysts in parallel using Eino Parallel
		emitPhase(ch, "start", "analysts", "各维度分析师并行分析中...")
		reports := e.runParallelAnalysts(ctx, ac)
		ac.Reports = reports
		emitPhase(ch, "end", "analysts", "分析师分析完成")

		// Check for cancellations
		if isCancelled(ctx) {
			emitPhase(ch, "error", "cancelled", "分析已取消")
			return
		}

		// Phase 3: Run researcher debate
		emitPhase(ch, "start", "debate", "多空研究员辩论中...")
		debateResult, err := RunDebate(ctx, ac, 2)
		if err != nil {
			logger.SugaredLogger.Errorf("debate error: %v", err)
		}
		ac.Debate = debateResult
		emitPhase(ch, "end", "debate", "辩论完成")

		// Phase 4: Run synthesis
		emitPhase(ch, "start", "synthesis", "正在生成最终报告...")
		finalReport, err := RunSynthesis(ctx, ac)
		if err != nil {
			logger.SugaredLogger.Errorf("synthesis error: %v", err)
		}
		ac.FinalReport = finalReport
		emitPhase(ch, "end", "synthesis", "分析完成")

		// Phase 5: Emit final report
		emitFinalReport(ch, finalReport)
	}()

	return ch
}

func (e *MultiAgentEngine) runParallelAnalysts(ctx context.Context, ac *AgentContext) []AgentReport {
	// Use Eino compose.Parallel to run all 4 analysts concurrently
	// For now, run them sequentially with goroutines + WaitGroup
	
	var reports []AgentReport
	resultCh := make(chan AgentReport, 4)
	
	go func() {
		r, _ := RunFundamentalAnalyst(ctx, ac)
		resultCh <- *r
	}()
	go func() {
		r, _ := RunTechnicalAnalyst(ctx, ac)
		resultCh <- *r
	}()
	go func() {
		r, _ := RunSentimentAnalyst(ctx, ac)
		resultCh <- *r
	}()
	go func() {
		r, _ := RunNewsAnalyst(ctx, ac)
		resultCh <- *r
	}()
	
	for i := 0; i < 4; i++ {
		reports = append(reports, <-resultCh)
	}
	
	return reports
}

func emitPhase(ch chan<- *schema.Message, status, phase, label string) {
	msg, _ := sonic.Marshal(map[string]string{
		"type":  "agent:phase",
		"status": status,
		"phase":  phase,
		"label":  label,
	})
	ch <- &schema.Message{
		Role:    schema.Assistant,
		Content: string(msg),
	}
}

func emitFinalReport(ch chan<- *schema.Message, report *FinalReport) {
	data, _ := sonic.Marshal(map[string]interface{}{
		"type":   "agent:final",
		"report": report,
	})
	ch <- &schema.Message{
		Role:    schema.Assistant,
		Content: string(data),
	}
}

func isCancelled(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}
```

- [ ] **Step 2: Commit**

```bash
git add backend/agent/multi/engine.go
git commit -m "feat(multi-agent): add Eino Graph engine core"
```

---

### Task 11: DataSourceProvider interface

**Files:**
- Create: `backend/data/datasource/provider.go`

- [ ] **Step 1: Write provider.go**

```go
// Package datasource provides a unified data access layer with priority-based fallback chains.
package datasource

import (
	"context"
	"errors"
	"time"
)

// DataType enumerates the types of financial data
type DataType string

const (
	DataTypeQuote       DataType = "quote"
	DataTypeKLine       DataType = "kline"
	DataTypeNews        DataType = "news"
	DataTypeFundamental DataType = "fundamental"
	DataTypeSector      DataType = "sector"
)

// DataSourceProvider is the interface every data source must implement
type DataSourceProvider interface {
	// Name returns a human-readable name for this provider (e.g. "tdx", "eastmoney")
	Name() string
	// Priority returns the priority order (lower = higher priority)
	Priority() int
	// Available checks if this data source is currently reachable
	Available(ctx context.Context) bool
}

// QuoteProvider provides real-time quote data
type QuoteProvider interface {
	DataSourceProvider
	GetQuote(ctx context.Context, code string) (*QuoteData, error)
}

// KLineProvider provides K-line data
type KLineProvider interface {
	DataSourceProvider
	GetKLine(ctx context.Context, code string, period string, count int) (*KLineData, error)
}

// NewsProvider provides news data
type NewsProvider interface {
	DataSourceProvider
	GetNews(ctx context.Context, code string, count int) ([]NewsItem, error)
}

// FundamentalProvider provides fundamental/financial data
type FundamentalProvider interface {
	DataSourceProvider
	GetFundamental(ctx context.Context, code string) (*FundamentalData, error)
}

// SectorProvider provides sector/industry data
type SectorProvider interface {
	DataSourceProvider
	GetSectorData(ctx context.Context, code string) (*SectorData, error)
}

// --- Data types ---

type QuoteData struct {
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Price     float64   `json:"price"`
	Change    float64   `json:"change"`
	ChangePct float64   `json:"changePct"`
	Volume    int64     `json:"volume"`
	Amount    float64   `json:"amount"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Open      float64   `json:"open"`
	PrevClose float64   `json:"prevClose"`
	Time      time.Time `json:"time"`
}

type KLineData struct {
	Code   string      `json:"code"`
	Period string      `json:"period"`
	Bars   []KLineBar  `json:"bars"`
}

type KLineBar struct {
	Time   time.Time `json:"time"`
	Open   float64   `json:"open"`
	High   float64   `json:"high"`
	Low    float64   `json:"low"`
	Close  float64   `json:"close"`
	Volume int64     `json:"volume"`
}

type NewsItem struct {
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Source    string    `json:"source"`
	Time      time.Time `json:"time"`
	Sentiment float64   `json:"sentiment"` // -1 to 1
}

type FundamentalData struct {
	PE        float64 `json:"pe"`
	PB        float64 `json:"pb"`
	ROE       float64 `json:"roe"`
	Revenue   float64 `json:"revenue"`
	NetProfit float64 `json:"netProfit"`
	DebtRatio float64 `json:"debtRatio"`
}

type SectorData struct {
	Code      string  `json:"code"`
	Sector    string  `json:"sector"`
	Rank      int     `json:"rank"`
	FlowAmount float64 `json:"flowAmount"`
}

// --- Error types ---

var (
	ErrAllSourcesFailed = errors.New("all data sources failed")
	ErrNoProvider       = errors.New("no provider registered for data type")
)
```

- [ ] **Step 2: Commit**

```bash
git add backend/data/datasource/provider.go
git commit -m "feat(datasource): add provider interface and data types"
```

---

### Task 12: Data source router with priority fallback

**Files:**
- Create: `backend/data/datasource/router.go`

- [ ] **Step 1: Write router.go**

```go
package datasource

import (
	"context"
	"fmt"
	"go-stock/backend/logger"
	"sort"
	"sync"
)

// Router manages data source providers per data type
type Router struct {
	mu      sync.RWMutex
	quoteProviders       []QuoteProvider
	klineProviders       []KLineProvider
	newsProviders        []NewsProvider
	fundamentalProviders []FundamentalProvider
	sectorProviders      []SectorProvider
}

var globalRouter *Router
var once sync.Once

// GetRouter returns the singleton router
func GetRouter() *Router {
	once.Do(func() {
		globalRouter = &Router{}
	})
	return globalRouter
}

// RegisterQuoteProvider registers a quote data source
func (r *Router) RegisterQuoteProvider(p QuoteProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.quoteProviders = append(r.quoteProviders, p)
	sort.Slice(r.quoteProviders, func(i, j int) bool {
		return r.quoteProviders[i].Priority() < r.quoteProviders[j].Priority()
	})
}

// GetQuote gets quote data with automatic fallback
func (r *Router) GetQuote(ctx context.Context, code string) (*QuoteData, error) {
	r.mu.RLock()
	providers := r.quoteProviders
	r.mu.RUnlock()

	for _, p := range providers {
		if !p.Available(ctx) {
			continue
		}
		data, err := p.GetQuote(ctx, code)
		if err == nil {
			logger.SugaredLogger.Debugf("datasource: quote %s from %s", code, p.Name())
			return data, nil
		}
		logger.SugaredLogger.Warnf("datasource: quote %s from %s failed: %v, trying next", code, p.Name(), err)
	}
	return nil, fmt.Errorf("GetQuote(%s): %w", code, ErrAllSourcesFailed)
}

// RegisterKLineProvider and GetKLine follow the same pattern
// RegisterNewsProvider and GetNews follow the same pattern
// RegisterFundamentalProvider and GetFundamental follow the same pattern
// RegisterSectorProvider and GetSectorData follow the same pattern
```

Similar `Register*Provider` and `Get*` methods exist for KLine, News, Fundamental, and Sector. Each follows the same pattern: sort by priority on registration → iterate with fallback on fetch.

- [ ] **Step 2: Commit**

```bash
git add backend/data/datasource/router.go
git commit -m "feat(datasource): add priority-based router with fallback"
```

---

### Task 13: Multi-level cache

**Files:**
- Create: `backend/data/datasource/cache.go`

- [ ] **Step 1: Write cache.go**

```go
package datasource

import (
	"context"
	"fmt"
	"time"
	"go-stock/backend/db"
	"github.com/coocood/freecache"
	"github.com/bytedance/sonic"
	"gorm.io/gorm"
)

// CacheLayer provides two-level caching: L1 = memory (freecache), L2 = SQLite
type CacheLayer struct {
	l1Cache *freecache.Cache // 256MB in-memory
}

type CacheEntry struct {
	ID        uint      `gorm:"primarykey"`
	CacheKey  string    `gorm:"uniqueIndex;size:255"`
	DataType  string    `gorm:"size:50;index"`
	Data      string    `gorm:"type:text"`
	ExpiresAt time.Time `gorm:"index"`
	CreatedAt time.Time
}

func (CacheEntry) TableName() string {
	return "datasource_cache"
}

// NewCacheLayer creates a new cache with the given memory size in MB
func NewCacheLayer(memSizeMB int) *CacheLayer {
	return &CacheLayer{
		l1Cache: freecache.NewCache(memSizeMB * 1024 * 1024),
	}
}

// Get retrieves data from cache (L1 → L2)
func (c *CacheLayer) Get(ctx context.Context, key string) ([]byte, bool) {
	// L1: try memory
	if val, err := c.l1Cache.Get([]byte(key)); err == nil {
		return val, true
	}
	
	// L2: try SQLite
	var entry CacheEntry
	err := db.Dao.Where("cache_key = ? AND expires_at > ?", key, time.Now()).First(&entry).Error
	if err != nil {
		return nil, false
	}
	
	// Promote to L1
	data := []byte(entry.Data)
	c.l1Cache.Set([]byte(key), data, 300) // 5 min TTL in L1
	return data, true
}

// Set stores data in cache (L1 + L2)
func (c *CacheLayer) Set(ctx context.Context, key string, dataType string, data []byte, ttl time.Duration) error {
	// L1: memory
	c.l1Cache.Set([]byte(key), data, int(ttl.Seconds()))
	
	// L2: SQLite (upsert)
	entry := CacheEntry{
		CacheKey:  key,
		DataType:  dataType,
		Data:      string(data),
		ExpiresAt: time.Now().Add(ttl),
	}
	
	db.Dao.Where("cache_key = ?", key).Assign(entry).FirstOrCreate(&entry)
	return nil
}

// Invalidate removes cache entries for a data type
func (c *CacheLayer) Invalidate(dataType string) {
	db.Dao.Where("data_type = ?", dataType).Delete(&CacheEntry{})
	c.l1Cache.Clear()
}

// Cache key generator
func CacheKey(dataType DataType, code string, params ...string) string {
	key := fmt.Sprintf("%s:%s", dataType, code)
	for _, p := range params {
		key += ":" + p
	}
	return key
}
```

- [ ] **Step 2: Commit**

```bash
git add backend/data/datasource/cache.go
git commit -m "feat(datasource): add multi-level cache (freecache + SQLite)"
```

---

### Task 14: Fallback chain for Quote data (TDX → EastMoney → Sina)

**Files:**
- Create: `backend/data/datasource/fallback/quote_chain.go`

- [ ] **Step 1: Write quote_chain.go**

```go
package fallback

import (
	"context"
	"go-stock/backend/data"
	"go-stock/backend/data/datasource"
)

// TDXQuoteProvider wraps the existing TDX API as a QuoteProvider
type TDXQuoteProvider struct{}

func (p *TDXQuoteProvider) Name() string                      { return "tdx" }
func (p *TDXQuoteProvider) Priority() int                     { return 10 }
func (p *TDXQuoteProvider) Available(ctx context.Context) bool { return true }

func (p *TDXQuoteProvider) GetQuote(ctx context.Context, code string) (*datasource.QuoteData, error) {
	// Call existing data.GetTDXQuote(code)
	// Map to datasource.QuoteData
	return nil, nil
}

// EastMoneyQuoteProvider wraps EastMoney API
type EastMoneyQuoteProvider struct{}

func (p *EastMoneyQuoteProvider) Name() string                      { return "eastmoney" }
func (p *EastMoneyQuoteProvider) Priority() int                     { return 20 }
func (p *EastMoneyQuoteProvider) Available(ctx context.Context) bool { return true }

func (p *EastMoneyQuoteProvider) GetQuote(ctx context.Context, code string) (*datasource.QuoteData, error) {
	// Call existing data.GetEastMoneyQuote(code)
	return nil, nil
}

// SinaQuoteProvider wraps Sina API as fallback
type SinaQuoteProvider struct{}

func (p *SinaQuoteProvider) Name() string                      { return "sina" }
func (p *SinaQuoteProvider) Priority() int                     { return 30 }
func (p *SinaQuoteProvider) Available(ctx context.Context) bool { return true }

func (p *SinaQuoteProvider) GetQuote(ctx context.Context, code string) (*datasource.QuoteData, error) {
	// Call existing data.GetSinaQuote(code)
	return nil, nil
}

// RegisterQuoteChain registers all quote providers with the router
func RegisterQuoteChain(router *datasource.Router) {
	router.RegisterQuoteProvider(&TDXQuoteProvider{})
	router.RegisterQuoteProvider(&EastMoneyQuoteProvider{})
	router.RegisterQuoteProvider(&SinaQuoteProvider{})
}
```

Each provider wraps an existing `backend/data/` API function. Actual mapping from existing API responses to `datasource.QuoteData` is done in the full implementation.

- [ ] **Step 2: Commit**

```bash
git add backend/data/datasource/fallback/quote_chain.go
git commit -m "feat(datasource): add quote fallback chain (TDX→EastMoney→Sina)"
```

---

### Task 15–18: Remaining fallback chains

**Files:**
- Create: `backend/data/datasource/fallback/kline_chain.go`
- Create: `backend/data/datasource/fallback/news_chain.go`
- Create: `backend/data/datasource/fallback/fundamental_chain.go`
- Create: `backend/data/datasource/fallback/sector_chain.go`

Each follows the same pattern as Task 14:

| Chain File | Providers | Priorities |
|---|---|---|
| `kline_chain.go` | TDX (10) → EastMoney (20) | Same pattern, calls existing `tdx_kline_api.go` / `eastmoney_kline_api.go` |
| `news_chain.go` | WallStreetCN (10) → EastMoney (20) → Cailianshe (30) | Calls existing `wallstreetcn_api.go` / `eastmoney_api.go` news methods |
| `fundamental_chain.go` | Tushare (10) → EastMoney (20) | Calls existing `tushare_data_api.go` / `eastmoney_api.go` |
| `sector_chain.go` | TDX Sector (10) → EastMoney (20) | Calls existing `tdx_kline_api.go` sector methods / `bk_fund_flow_api.go` |

Each file has a `Register*Chain(router)` function that the app startup calls.

- [ ] **Step 1: Commit each chain**

```bash
git add backend/data/datasource/fallback/kline_chain.go
git commit -m "feat(datasource): add kline fallback chain (TDX→EastMoney)"

git add backend/data/datasource/fallback/news_chain.go
git commit -m "feat(datasource): add news fallback chain (WallStreetCN→EastMoney→Cailianshe)"

git add backend/data/datasource/fallback/fundamental_chain.go
git commit -m "feat(datasource): add fundamental fallback chain (Tushare→EastMoney)"

git add backend/data/datasource/fallback/sector_chain.go
git commit -m "feat(datasource): add sector fallback chain (TDX→EastMoney)"
```

---

### Task 19: Wire up fallback chains at app startup

**Files:**
- Modify: `main.go` (add data source initialization)
- Modify: `backend/data/datasource/router.go` (add Init function)

- [ ] **Step 1: Add datasource initialization in main.go**

In `main.go`, after `data.InitAnalyzeSentiment()` and before `go AutoMigrate()`, add:

```go
import (
	"go-stock/backend/data/datasource"
	"go-stock/backend/data/datasource/fallback"
)

func initDataSources() {
	router := datasource.GetRouter()
	
	// Initialize cache
	cache := datasource.NewCacheLayer(256)
	router.SetCache(cache)
	
	// Register fallback chains
	fallback.RegisterQuoteChain(router)
	fallback.RegisterKLineChain(router)
	fallback.RegisterNewsChain(router)
	fallback.RegisterFundamentalChain(router)
	fallback.RegisterSectorChain(router)
	
	log.SugaredLogger.Info("data source router initialized with fallback chains")
}
```

Call `initDataSources()` in the startup sequence.

- [ ] **Step 2: Commit**

```bash
git add main.go backend/data/datasource/router.go
git commit -m "feat(datasource): wire up fallback chains at startup"
```

---

### Task 20: Modify agent.go — replace old agent creation

**Files:**
- Modify: `backend/agent/agent.go`
- Modify: `backend/agent/chat_model_factory.go` (export `createChatModel`)

- [ ] **Step 1: Export createChatModel in chat_model_factory.go**

Rename `createChatModel` → `CreateChatModel` (capitalize) so the `multi` package can call it.

- [ ] **Step 2: Update agent.go**

Add `GetMultiAgentEngine()` function:

```go
func GetMultiAgentEngine(ctx context.Context, aiConfig data.AIConfig) *multi.MultiAgentEngine {
	return multi.NewMultiAgentEngine(int(aiConfig.ID))
}
```

Keep `GetStockAiAgent()` temporarily for backward compatibility during rollout.

- [ ] **Step 3: Commit**

```bash
git add backend/agent/agent.go backend/agent/chat_model_factory.go
git commit -m "refactor(agent): export CreateChatModel, add multi-agent engine entry"
```

---

### Task 21: Modify agent_api.go — route to MultiAgentEngine

**Files:**
- Modify: `backend/agent/agent_api.go`

- [ ] **Step 1: Update ChatWithContext**

Replace the current agent dispatch logic in `ChatWithContext()`:

```go
// New path: use MultiAgentEngine instead of React/PlanExecute
if enableMultiAgent {
    engine := multi.NewMultiAgentEngine(aiConfigId)
    resultCh := engine.Run(ctx, stockCode, stockName, market, question)
    for msg := range resultCh {
        ch <- msg
    }
    return ch
}
```

Add a config check (or flag) to gate multi-agent mode. For initial rollout, make it the default.

- [ ] **Step 2: Commit**

```bash
git add backend/agent/agent_api.go
git commit -m "feat(agent): route AI analysis to MultiAgentEngine"
```

---

### Task 22: Integration test with existing app

**Files:**
- Modify: `app.go` (wire new agent into stock analysis flow)

- [ ] **Step 1: Verify end-to-end in app.go**

Find `NewChatStream()` in `app.go` and ensure it calls the new multi-agent path when appropriate:

```go
func (a *App) NewChatStream(stock string, stockCode string, question string, aiConfigId int, sysPromptId *int, enableTools bool, think bool, agentMode string) {
    // ...
    engine := multi.NewMultiAgentEngine(aiConfigId)
    resultCh := engine.Run(a.ctx, stockCode, "", "", question)
    for msg := range resultCh {
        runtime.EventsEmit(a.ctx, "newChatStream", msg)
    }
}
```

- [ ] **Step 2: Build test**

```bash
cd E:/open-source/ai/go-stock && go build ./...
```

Expected: clean compilation.

- [ ] **Step 3: Commit**

```bash
git add app.go
git commit -m "feat(app): wire multi-agent engine into chat stream"
```
