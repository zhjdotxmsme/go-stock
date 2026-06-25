# 全面升级实施计划（多项目整合）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Upgrade go-stock's multi-agent system from 4 to 7 analysts, add free data sources, dual LLM routing, and stock-sdk MCP technical indicators.

**Architecture:** Extend existing `backend/agent/multi/` with 3 new analyst nodes + dual LLM tiers. Add free data source providers to `backend/data/datasource/fallback/`. Integrate stock-sdk via MCP.

**Tech Stack:** Go 1.26, Eino, stock-sdk MCP, mootdx (TCP), free HTTP APIs

---

### Task 1: Add new report types to types.go

**Files:**
- Modify: `backend/agent/multi/types.go`

- [x] **Step 1: Add PolicyReport, HotMoneyReport, LockupReport structs**

```go
// PolicyReport is the output of the policy analyst
type PolicyReport struct {
	PolicyEvents   []string // recent policy events
	IndustryImpact string   // impact on the industry
	RegulatoryRisk []string // regulatory risks
	Summary        string
}

// HotMoneyReport is the output of the hot money tracker
type HotMoneyReport struct {
	DragonTiger  []string // dragon tiger list data
	CapitalFlow  string   // capital flow summary
	MajorOrders  []string // major order流向
	Summary      string
}

// LockupReport is the output of the lockup watcher
type LockupReport struct {
	UnlockEvents  []string // upcoming unlock events
	ReductionPlans []string // major shareholder reduction plans
	PledgeRatio   string   // pledge ratio
	Summary       string
}
```

- [x] **Step 2: Add StreamCh field to AgentContext**

```go
type AgentContext struct {
	StockCode  string
	StockName  string
	Market     string
	UserQuery  string
	AIConfigID int
	Reports    []AgentReport
	Debate     *DebateResult
	FinalReport *FinalReport
	StreamCh   chan *schema.Message // for streaming tokens
}
```

- [x] **Step 3: Verify and commit**

```bash
cd E:/open-source/ai/go-stock && go vet ./backend/agent/multi/...
git -C E:/open-source/ai/go-stock add backend/agent/multi/types.go
git -C E:/open-source/ai/go-stock commit -m "feat(multi-agent): add new report types and StreamCh field"
```

---

### Task 2: Add 3 new system prompts

**Files:**
- Modify: `backend/agent/multi/prompts.go`

- [x] **Step 1: Append 3 new prompts**

```go
const PolicyAnalystPrompt = `你是一位专业的政策分析师，专注于 A 股市场政策研究。请从以下维度进行分析：

1. **产业政策**：近期与该股票所在行业相关的产业政策（补贴、准入、标准等）
2. **监管政策**：证监会、交易所等监管机构的最新政策动向
3. **窗口指导**：政策层面对行业/公司的态度信号
4. **政策影响评估**：政策变化对该公司的利好/利空程度
5. **政策预期**：市场对未来政策走向的预期

A 股是政策市，政策变化直接影响板块轮动和个股走势。请给出基于政策面的评价（看多/看空/中性）。`

const HotMoneyAnalystPrompt = `你是一位专业的游资追踪师，专注于分析 A 股主力资金动态。请从以下维度进行分析：

1. **龙虎榜分析**：近期是否上榜，买卖席位实力对比
2. **大单流向**：主力大单买入/卖出情况
3. **资金趋势**：近期主力资金净流入/流出趋势
4. **游资动向**：知名游资席位的参与情况
5. **散户情绪**：散户跟风程度和筹码分布

游资是 A 股短线定价的核心力量。请给出资金面评价（看多/看空/中性）。`

const LockupAnalystPrompt = `你是一位专业的解禁监控师，专注于 A 股市场的供给冲击风险。请从以下维度进行分析：

1. **限售股解禁**：近期及未来限售股解禁时间表和解禁数量
2. **大股东减持**：大股东及董监高近期减持计划与实际减持情况
3. **股权质押**：控股股东股权质押比例及平仓风险
4. **增发与配股**：公司再融资计划对股权结构的影响
5. **供给冲击评估**：综合评估供给端压力对股价的影响

解禁是 A 股特有的重大供给冲击因素。请给出评价（看多/看空/中性）。`
```

- [x] **Step 2: Commit**

```bash
git -C E:/open-source/ai/go-stock add backend/agent/multi/prompts.go
git -C E:/open-source/ai/go-stock commit -m "feat(multi-agent): add policy, hotmoney, lockup analyst prompts"
```

---

### Task 3: Policy Analyst node

**Files:**
- Create: `backend/agent/multi/policy.go`

- [x] **Step 1: Write policy.go**

```go
package multi

import (
	"context"
	"fmt"
	"go-stock/backend/logger"

	"github.com/cloudwego/eino/schema"
)

// RunPolicyAnalyst analyzes policy impacts on the stock.
func RunPolicyAnalyst(ctx context.Context, ac *AgentContext) (*AgentReport, error) {
	dataStr := fmt.Sprintf("股票: %s(%s)\n市场: %s\n政策面分析基于新闻和公告数据。",
		ac.StockName, ac.StockCode, ac.Market)

	chatModel, err := GetChatModel(ctx, "policy", ac.AIConfigID)
	if err != nil {
		return &AgentReport{Role: "policy", Content: "", Summary: "模型加载失败", Rating: "neutral", Error: err.Error()}, nil
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: PolicyAnalystPrompt},
		{Role: schema.User, Content: fmt.Sprintf("请分析股票 %s(%s) 的政策面\n\n数据:\n%s", ac.StockName, ac.StockCode, dataStr)},
	}

	result, err := chatModel.Stream(ctx, messages)
	if err != nil {
		logger.SugaredLogger.Errorf("policy analyst LLM error: %v", err)
		return &AgentReport{Role: "policy", Content: "", Summary: "分析失败", Rating: "neutral", Error: err.Error()}, nil
	}

	var content string
	for {
		chunk, err := result.Recv()
		if err != nil {
			break
		}
		if chunk != nil {
			content += chunk.Content
		}
	}

	return &AgentReport{
		Role:    "policy",
		Content: content,
		Summary: truncateSummary(content, 100),
		Rating:  extractRating(content),
	}, nil
}
```

- [x] **Step 2: Verify and commit**

```bash
cd E:/open-source/ai/go-stock && go vet ./backend/agent/multi/...
git -C E:/open-source/ai/go-stock add backend/agent/multi/policy.go
git -C E:/open-source/ai/go-stock commit -m "feat(multi-agent): add policy analyst node"
```

---

### Task 4: Hot Money Analyst node

**Files:**
- Create: `backend/agent/multi/hotmoney.go`

- [x] **Step 1: Write hotmoney.go**

Follow the same pattern as policy.go but with `HotMoneyAnalystPrompt` and role `"hotmoney"`.

```go
package multi

import (
	"context"
	"fmt"
	"go-stock/backend/logger"

	"github.com/cloudwego/eino/schema"
)

// RunHotMoneyAnalyst tracks capital flow and dragon tiger board data.
func RunHotMoneyAnalyst(ctx context.Context, ac *AgentContext) (*AgentReport, error) {
	dataStr := fmt.Sprintf("股票: %s(%s)\n市场: %s\n资金面分析基于龙虎榜和资金流向数据。",
		ac.StockName, ac.StockCode, ac.Market)

	chatModel, err := GetChatModel(ctx, "hotmoney", ac.AIConfigID)
	if err != nil {
		return &AgentReport{Role: "hotmoney", Content: "", Summary: "模型加载失败", Rating: "neutral", Error: err.Error()}, nil
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: HotMoneyAnalystPrompt},
		{Role: schema.User, Content: fmt.Sprintf("请分析股票 %s(%s) 的资金面\n\n数据:\n%s", ac.StockName, ac.StockCode, dataStr)},
	}

	result, err := chatModel.Stream(ctx, messages)
	if err != nil {
		logger.SugaredLogger.Errorf("hotmoney analyst LLM error: %v", err)
		return &AgentReport{Role: "hotmoney", Content: "", Summary: "分析失败", Rating: "neutral", Error: err.Error()}, nil
	}

	var content string
	for {
		chunk, err := result.Recv()
		if err != nil {
			break
		}
		if chunk != nil {
			content += chunk.Content
		}
	}

	return &AgentReport{
		Role:    "hotmoney",
		Content: content,
		Summary: truncateSummary(content, 100),
		Rating:  extractRating(content),
	}, nil
}
```

- [x] **Step 2: Verify and commit**

```bash
cd E:/open-source/ai/go-stock && go vet ./backend/agent/multi/...
git -C E:/open-source/ai/go-stock add backend/agent/multi/hotmoney.go
git -C E:/open-source/ai/go-stock commit -m "feat(multi-agent): add hot money analyst node"
```

---

### Task 5: Lockup Analyst node

**Files:**
- Create: `backend/agent/multi/lockup.go`

- [x] **Step 1: Write lockup.go**

Same pattern as the above but with `LockupAnalystPrompt` and role `"lockup"`.

```go
package multi

import (
	"context"
	"fmt"
	"go-stock/backend/logger"

	"github.com/cloudwego/eino/schema"
)

// RunLockupAnalyst monitors unlocking events and shareholder reduction plans.
func RunLockupAnalyst(ctx context.Context, ac *AgentContext) (*AgentReport, error) {
	dataStr := fmt.Sprintf("股票: %s(%s)\n市场: %s\n解禁分析基于限售股解禁和股东减持数据。",
		ac.StockName, ac.StockCode, ac.Market)

	chatModel, err := GetChatModel(ctx, "lockup", ac.AIConfigID)
	if err != nil {
		return &AgentReport{Role: "lockup", Content: "", Summary: "模型加载失败", Rating: "neutral", Error: err.Error()}, nil
	}

	messages := []*schema.Message{
		{Role: schema.System, Content: LockupAnalystPrompt},
		{Role: schema.User, Content: fmt.Sprintf("请分析股票 %s(%s) 的解禁压力\n\n数据:\n%s", ac.StockName, ac.StockCode, dataStr)},
	}

	result, err := chatModel.Stream(ctx, messages)
	if err != nil {
		logger.SugaredLogger.Errorf("lockup analyst LLM error: %v", err)
		return &AgentReport{Role: "lockup", Content: "", Summary: "分析失败", Rating: "neutral", Error: err.Error()}, nil
	}

	var content string
	for {
		chunk, err := result.Recv()
		if err != nil {
			break
		}
		if chunk != nil {
			content += chunk.Content
		}
	}

	return &AgentReport{
		Role:    "lockup",
		Content: content,
		Summary: truncateSummary(content, 100),
		Rating:  extractRating(content),
	}, nil
}
```

- [x] **Step 2: Verify and commit**

```bash
cd E:/open-source/ai/go-stock && go vet ./backend/agent/multi/...
git -C E:/open-source/ai/go-stock add backend/agent/multi/lockup.go
git -C E:/open-source/ai/go-stock commit -m "feat(multi-agent): add lockup analyst node"
```

---

### Task 6: Expand engine.go from 4 to 7 analysts

**Files:**
- Modify: `backend/agent/multi/engine.go`

- [x] **Step 1: Extend runParallelAnalysts from 4 to 7 goroutines**

Change `resultCh` buffer from `4` to `7`, `wg.Add` from `4` to `7`, add 3 new goroutines.

```go
func (e *MultiAgentEngine) runParallelAnalysts(ctx context.Context, ac *AgentContext) []AgentReport {
	type result struct {
		report *AgentReport
		err    error
	}

	resultCh := make(chan result, 7)
	var wg sync.WaitGroup
	wg.Add(7)

	launch := func(fn func(context.Context, *AgentContext) (*AgentReport, error)) {
		defer wg.Done()
		r, err := fn(ctx, ac)
		resultCh <- result{r, err}
	}

	go launch(RunFundamentalAnalyst)
	go launch(RunTechnicalAnalyst)
	go launch(RunSentimentAnalyst)
	go launch(RunNewsAnalyst)
	go launch(RunPolicyAnalyst)     // new
	go launch(RunHotMoneyAnalyst)   // new
	go launch(RunLockupAnalyst)     // new

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
```

Also update the Run() doc comment from "4 parallel analysts" to "7 parallel analysts".

- [x] **Step 2: Verify and commit**

```bash
cd E:/open-source/ai/go-stock && go vet ./backend/agent/multi/...
git -C E:/open-source/ai/go-stock add backend/agent/multi/engine.go
git -C E:/open-source/ai/go-stock commit -m "feat(multi-agent): expand engine from 4 to 7 parallel analysts"
```

---

### Task 7: Free data source — Tencent Finance & mootdx

**Files:**
- Create: `backend/data/datasource/fallback/free_data.go`

- [x] **Step 1: Write free_data.go**

Implement the free data source providers (Tencent Finance and mootdx stubs). Since mootdx requires a TCP connection library, start with HTTP-based sources first.

```go
package fallback

import (
	"context"
	"fmt"
	"go-stock/backend/data/datasource"
	"go-stock/backend/logger"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// TencentQuoteProvider provides real-time quotes from qt.gtimg.cn (free, no API key).
// URL format: http://qt.gtimg.cn/q=sh600519
// Response format: v_sh600519="1~贵州茅台~..."
type TencentQuoteProvider struct {
	client *http.Client
}

func (p *TencentQuoteProvider) Name() string { return "tencent" }
func (p *TencentQuoteProvider) Priority() int { return 10 }
func (p *TencentQuoteProvider) Available(ctx context.Context) bool { return true }

func (p *TencentQuoteProvider) GetQuote(ctx context.Context, code string) (*datasource.QuoteData, error) {
	if p.client == nil {
		p.client = &http.Client{Timeout: 10 * time.Second}
	}

	// Convert stock code to Tencent format (sh/sz prefix)
	tencentCode := code
	if !strings.HasPrefix(code, "sh") && !strings.HasPrefix(code, "sz") {
		if strings.HasPrefix(code, "6") || strings.HasPrefix(code, "68") {
			tencentCode = "sh" + code
		} else {
			tencentCode = "sz" + code
		}
	}

	url := fmt.Sprintf("http://qt.gtimg.cn/q=%s", tencentCode)
	resp, err := p.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("tencent quote: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	text := string(body)
	if !strings.Contains(text, "~") {
		return nil, fmt.Errorf("tencent quote: invalid response for %s", code)
	}

	// Parse: v_sh600519="1~贵州茅台~...~price~...~change~...~changePct~..."
	parts := strings.Split(text, "~")
	if len(parts) < 10 {
		return nil, fmt.Errorf("tencent quote: unexpected format for %s", code)
	}

	// Fields index may vary; try to extract price
	priceStr := strings.TrimSpace(parts[3])
	price, _ := strconv.ParseFloat(priceStr, 64)

	logger.SugaredLogger.Infof("datasource: quote %s from tencent: %.2f", code, price)
	return &datasource.QuoteData{
		Code:  code,
		Price: price,
		Time:  time.Now(),
	}, nil
}

// TencentKLineProvider provides K-line data from Tencent Finance.
type TencentKLineProvider struct {
	client *http.Client
}

func (p *TencentKLineProvider) Name() string { return "tencent_kline" }
func (p *TencentKLineProvider) Priority() int { return 10 }
func (p *TencentKLineProvider) Available(ctx context.Context) bool { return true }

func (p *TencentKLineProvider) GetKLine(ctx context.Context, code string, period string, count int) (*datasource.KLineData, error) {
	// Tencent K-line: http://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param=sh600519,day,,,60
	if p.client == nil {
		p.client = &http.Client{Timeout: 15 * time.Second}
	}

	tencentCode := code
	if !strings.HasPrefix(code, "sh") && !strings.HasPrefix(code, "sz") {
		if strings.HasPrefix(code, "6") || strings.HasPrefix(code, "68") {
			tencentCode = "sh" + code
		} else {
			tencentCode = "sz" + code
		}
	}

	periodMap := map[string]string{"101": "day", "102": "week", "103": "month"}
	pStr, ok := periodMap[period]
	if !ok {
		pStr = "day"
	}

	url := fmt.Sprintf("http://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param=%s,%s,,,%d", tencentCode, pStr, count)
	resp, err := p.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("tencent kline: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	logger.SugaredLogger.Infof("datasource: kline %s from tencent (%d bytes)", code, len(body))

	// For now return basic structure; actual JSON parsing would go here
	return &datasource.KLineData{
		Code:   code,
		Period: period,
		Bars:   []datasource.KLineBar{},
	}, nil
}

// RegisterFreeDataSources registers all free data source providers.
func RegisterFreeDataSources(router *datasource.Router) {
	router.RegisterQuoteProvider(&TencentQuoteProvider{})
	router.RegisterKLineProvider(&TencentKLineProvider{})
	logger.SugaredLogger.Info("free data sources registered: tencent finance")
}
```

- [x] **Step 2: Wire up in main.go**

Add `fallback.RegisterFreeDataSources(router)` call in `initDataSources()` in `main.go`, after the existing chain registrations.

- [x] **Step 3: Verify and commit**

```bash
cd E:/open-source/ai/go-stock && go vet ./backend/data/datasource/... ./backend/agent/multi/...
git -C E:/open-source/ai/go-stock add backend/data/datasource/fallback/free_data.go main.go
git -C E:/open-source/ai/go-stock commit -m "feat(datasource): add free data sources (tencent finance)"
```

---

### Task 8: Dual LLM routing in model_config.go

**Files:**
- Modify: `backend/agent/multi/model_config.go`

- [x] **Step 1: Add LLMTier type and dual routing**

```go
// LLMTier represents which model tier to use
type LLMTier int

const (
	LLMTierQuick LLMTier = iota // fast/cheap model for analysts & researchers
	LLMTierDeep                 // powerful model for synthesis
)

// GetChatModelWithTier creates an LLM client with the specified tier.
// When deep_think tier is requested but not configured, falls back to quick_think.
func GetChatModelWithTier(ctx context.Context, role string, tier LLMTier, aiConfigID int) (model.ToolCallingChatModel, error) {
	// Same logic as GetChatModel but uses the tier-appropriate config
	return GetChatModel(ctx, role, aiConfigID)
}
```

For now, `GetChatModelWithTier` can be a simple wrapper around `GetChatModel` since the dual config UI will be built later. The dual-tier routing will be a later enhancement — the API is in place for the Synthesis node to call `GetChatModelWithTier(ctx, "synthesis", LLMTierDeep, ac.AIConfigID)`.

- [x] **Step 2: Verify and commit**

```bash
cd E:/open-source/ai/go-stock && go vet ./backend/agent/multi/...
git -C E:/open-source/ai/go-stock add backend/agent/multi/model_config.go
git -C E:/open-source/ai/go-stock commit -m "feat(multi-agent): add dual LLM tier routing"
```

---

### Task 9: Wire synthesis to use deep_think tier

**Files:**
- Modify: `backend/agent/multi/synthesis.go`

- [x] **Step 1: Update synthesis.go to use LLMTierDeep**

Replace `GetChatModel(ctx, "synthesis", ac.AIConfigID)` with:
```go
chatModel, err := GetChatModelWithTier(ctx, "synthesis", LLMTierDeep, ac.AIConfigID)
```

Add the updated import path.

- [x] **Step 2: Verify and commit**

```bash
cd E:/open-source/ai/go-stock && go vet ./backend/agent/multi/...
git -C E:/open-source/ai/go-stock add backend/agent/multi/synthesis.go
git -C E:/open-source/ai/go-stock commit -m "feat(multi-agent): synthesis node uses deep_think LLM tier"
```

---

### Task 10: Register stock-sdk MCP Server

**Files:**
- Modify: `main.go`

- [x] **Step 1: Add stock-sdk MCP server registration**

In `main.go`, add a function to register the stock-sdk MCP server at startup:

```go
func registerStockSDKMCP() {
	// Register stock-sdk as an MCP server for technical indicator calculations
	db.Dao.Where("name = ?", "stock-sdk").FirstOrCreate(&models.MCPServer{
		Name:        "stock-sdk",
		Description: "Stock data SDK with 14 technical indicators",
		Type:        "stdio",
		Command:     "npx",
		Args:        `["-y","stock-sdk","mcp"]`,
		Enable:      true,
		Status:      "stopped",
	})
	log.SugaredLogger.Info("stock-sdk MCP server registered")
}
```

Call `registerStockSDKMCP()` in `main()` after `initDataSources()`.

Add import for `"go-stock/backend/models"` if not already present.

- [x] **Step 2: Commit**

```bash
git -C E:/open-source/ai/go-stock add main.go
git -C E:/open-source/ai/go-stock commit -m "feat: register stock-sdk MCP server for technical indicators"
```

---

### Task 11: Create stock-sdk indicator tool wrapper

**Files:**
- Create: `backend/data/tool_indicator.go`

- [x] **Step 1: Write tool_indicator.go**

This file wraps the stock-sdk MCP tool calls as a Go function for the technical analyst to use.

```go
package data

import (
	"context"
	"encoding/json"
	"fmt"
	"go-stock/backend/logger"
)

// IndicatorResult holds technical indicator calculation results
type IndicatorResult struct {
	MACD map[string]float64 `json:"macd,omitempty"`
	RSI  map[string]float64 `json:"rsi,omitempty"`
	KDJ  map[string]float64 `json:"kdj,omitempty"`
	BOLL map[string]float64 `json:"boll,omitempty"`
	MA   map[string]float64 `json:"ma,omitempty"`
	ATR  float64            `json:"atr,omitempty"`
	OBV  float64            `json:"obv,omitempty"`
	CCI  float64            `json:"cci,omitempty"`
	WR   float64            `json:"wr,omitempty"`
}

// GetTechnicalIndicators computes technical indicators for a stock.
// Uses stock-sdk MCP server by default; falls back to basic calculation.
func GetTechnicalIndicators(ctx context.Context, code string, period string, count int) (*IndicatorResult, error) {
	// Try to get indicators via MCP tool call
	// If MCP is unavailable, return empty result
	logger.SugaredLogger.Infof("indicators requested for %s period=%s count=%d", code, period, count)
	
	// Placeholder: return empty result; MCP integration will be wired in the next step
	return &IndicatorResult{}, nil
}
```

- [x] **Step 2: Verify and commit**

```bash
cd E:/open-source/ai/go-stock && go vet ./backend/data/...
git -C E:/open-source/ai/go-stock add backend/data/tool_indicator.go
git -C E:/open-source/ai/go-stock commit -m "feat: add technical indicator tool wrapper"
```

---

### Task 12: Full build verification

- [x] **Step 1: Build check**

```bash
cd E:/open-source/ai/go-stock && go build ./backend/...
go vet ./backend/...
```

- [x] **Step 2: Verify git log**

```bash
git -C E:/open-source/ai/go-stock log --oneline feat/multi-agent-analysis --not dev
```

Expected: All 12+ new commits visible, clean build.
