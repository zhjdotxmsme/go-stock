# Data Source Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Integrate three GitHub projects (investment-news, a-stock-data, TradingAgents-astock) to enhance go-stock's data source architecture with 40+ data endpoints, multi-agent analysis system, and performance optimizations.

**Architecture:** Hybrid integration approach - core data layer refactoring with rapid feature integration, building a modern layered architecture with fallback mechanisms, caching, and monitoring.

**Tech Stack:** Go 1.21+, GORM, Redis, ChromeDP, OpenAI API, SQLite, Prometheus, Grafana

---

## Phase 1: Core Data Layer Refactoring (Weeks 1-3)

### Task 1: Core Data Layer Interface Definition

**Files:**
- Create: `backend/data/layers/base_layer.go`
- Create: `backend/data/utils/endpoint.go`
- Create: `backend/data/utils/data_source_config.go`
- Create: `backend/data/utils/standard_response.go`
- Test: `backend/data/tests/layers_test.go`

- [ ] **Step 1: Write failing tests for core interfaces**

```go
// backend/data/tests/layers_test.go
package data_test

import (
    "context"
    "testing"
    "time"
)

func TestDataLayerInterface(t *testing.T) {
    layer := &MockDataLayer{}
    
    // Test GetName
    if layer.GetName() != "MockLayer" {
        t.Errorf("GetName() = %v, want MockLayer", layer.GetName())
    }
    
    // Test GetVersion
    if layer.GetVersion() != "1.0.0" {
        t.Errorf("GetVersion() = %v, want 1.0.0", layer.GetVersion())
    }
}

func TestEndpointStructure(t *testing.T) {
    endpoint := Endpoint{
        Name:    "test_endpoint",
        URL:     "http://example.com/api",
        Method:  "GET",
        Timeout: time.Second * 10,
    }
    
    if endpoint.Name != "test_endpoint" {
        t.Errorf("Endpoint.Name = %v, want test_endpoint", endpoint.Name)
    }
}

func TestDataSourceConfigValidation(t *testing.T) {
    config := &DataSourceConfig{
        Primary: Endpoint{
            Name:   "primary",
            URL:    "http://primary.api",
            Method: "GET",
        },
        Fallbacks: []Endpoint{
            {Name: "fallback1", URL: "http://fallback1.api", Method: "GET"},
        },
        Strategy: FailoverStrategy,
    }
    
    if config.Strategy != FailoverStrategy {
        t.Errorf("Strategy = %v, want FAILOVER", config.Strategy)
    }
}

func TestStandardizedResponseFormat(t *testing.T) {
    response := &StandardizedResponse{
        Code:    0,
        Message: "success",
        Data:    map[string]interface{}{"test": "data"},
        Meta: ResponseMeta{
            Source:   "test_source",
            Latency:  100,
            Cached:   false,
            Timestamp: time.Now(),
        },
    }
    
    if response.Code != 0 {
        t.Errorf("Code = %v, want 0", response.Code)
    }
    
    if response.Meta.Source != "test_source" {
        t.Errorf("Meta.Source = %v, want test_source", response.Meta.Source)
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./backend/data/tests/layers_test.go -v`
Expected: FAIL with "undefined: MockDataLayer" and similar errors

- [ ] **Step 3: Implement core DataLayer interface**

```go
// backend/data/layers/base_layer.go
package layers

import (
    "context"
    "fmt"
)

// DataLayer defines the interface for all data layers
type DataLayer interface {
    GetName() string
    GetVersion() string
    GetEndpoints() []Endpoint
    GetFallbackEndpoints() []Endpoint
    FetchData(ctx context.Context, params map[string]any) (*StandardizedResponse, error)
    ValidateParams(params map[string]any) error
}

// Endpoint represents a single data source endpoint
type Endpoint struct {
    Name      string
    URL       string
    Method    string
    Headers   map[string]string
    Timeout   time.Duration
    RateLimit RateLimitConfig
    Parser    DataParser
}

// RateLimitConfig defines rate limiting for endpoints
type RateLimitConfig struct {
    RequestsPerSecond int
    BurstSize         int
}

// DataParser defines the interface for parsing responses
type DataParser interface {
    Parse(data []byte) (interface{}, error)
}

// MockDataLayer implements DataLayer for testing
type MockDataLayer struct {
    name    string
    version string
}

func NewMockDataLayer() *MockDataLayer {
    return &MockDataLayer{
        name:    "MockLayer",
        version: "1.0.0",
    }
}

func (m *MockDataLayer) GetName() string {
    return m.name
}

func (m *MockDataLayer) GetVersion() string {
    return m.version
}

func (m *MockDataLayer) GetEndpoints() []Endpoint {
    return []Endpoint{
        {
            Name:   "mock_endpoint",
            URL:    "http://mock.api",
            Method: "GET",
        },
    }
}

func (m *MockDataLayer) GetFallbackEndpoints() []Endpoint {
    return []Endpoint{}
}

func (m *MockDataLayer) FetchData(ctx context.Context, params map[string]any) (*StandardizedResponse, error) {
    return &StandardizedResponse{
        Code:    0,
        Message: "success",
        Data:    map[string]interface{}{"mock": "data"},
        Meta: ResponseMeta{
            Source:    m.GetName(),
            Timestamp: time.Now(),
        },
    }, nil
}

func (m *MockDataLayer) ValidateParams(params map[string]any) error {
    return nil
}
```

- [ ] **Step 4: Implement Endpoint and related utilities**

```go
// backend/data/utils/endpoint.go
package utils

import (
    "time"
)

// Endpoint represents a single data source endpoint
type Endpoint struct {
    Name      string
    URL       string
    Method    string
    Headers   map[string]string
    Timeout   time.Duration
    RateLimit RateLimitConfig
    Parser    DataParser
}

// RateLimitConfig defines rate limiting for endpoints
type RateLimitConfig struct {
    RequestsPerSecond int
    BurstSize         int
}

// DataParser defines the interface for parsing responses
type DataParser interface {
    Parse(data []byte) (interface{}, error)
}
```

- [ ] **Step 5: Implement DataSourceConfig**

```go
// backend/data/utils/data_source_config.go
package utils

import "time"

// DataSourceConfig manages primary and fallback data sources
type DataSourceConfig struct {
    Primary   Endpoint
    Fallbacks []Endpoint
    Strategy   FallbackStrategy
    Retry      RetryConfig
    Cache      CacheConfig
}

// FallbackStrategy defines how to choose between data sources
type FallbackStrategy string

const (
    FailoverStrategy   FallbackStrategy = "FAILOVER"   // Primary fails → use first fallback
    RoundRobinStrategy FallbackStrategy = "ROUND_ROBIN" // Rotate through sources
    RandomStrategy     FallbackStrategy = "RANDOM"     // Random selection
)

// RetryConfig defines retry behavior
type RetryConfig struct {
    MaxAttempts int
    Backoff     BackoffStrategy
    ShouldRetry func(error) bool
}

// BackoffStrategy defines delay between retries
type BackoffStrategy interface {
    Next(attempt int) time.Duration
}

// CacheConfig defines caching behavior
type CacheConfig struct {
    Enabled    bool
    TTL        time.Duration
    MaxSize    int64
    EvictionPolicy string // "LRU" | "LFU" | "FIFO"
}
```

- [ ] **Step 6: Implement StandardizedResponse**

```go
// backend/data/utils/standard_response.go
package utils

import "time"

// StandardizedResponse is the unified response format for all data layers
type StandardizedResponse struct {
    Code      int                    `json:"code"`
    Message   string                 `json:"message"`
    Data      interface{}            `json:"data"`
    Meta      ResponseMeta           `json:"meta"`
    Error     *ErrorResponse         `json:"error,omitempty"`
}

// ResponseMeta contains metadata about the response
type ResponseMeta struct {
    Source            string    `json:"source"`
    FallbackUsed      bool      `json:"fallback_used"`
    Latency           int64     `json:"latency_ms"`
    Cached            bool      `json:"cached"`
    Timestamp         time.Time `json:"timestamp"`
    Version           string    `json:"api_version"`
    RateLimitRemaining int       `json:"rate_limit_remaining"`
}

// ErrorResponse contains error details
type ErrorResponse struct {
    Type       string `json:"type"`
    Message    string `json:"message"`
    Details    string `json:"details,omitempty"`
    Timestamp  time.Time `json:"timestamp"`
    TraceID    string `json:"trace_id"`
}
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `go test ./backend/data/tests/layers_test.go -v`
Expected: PASS for all tests

- [ ] **Step 8: Commit**

```bash
git add backend/data/layers/base_layer.go backend/data/utils/endpoint.go backend/data/utils/data_source_config.go backend/data/utils/standard_response.go backend/data/tests/layers_test.go
git commit -m "feat: implement core data layer interfaces and base structures"
```

---

### Task 2: Market Data Layer Implementation

**Files:**
- Create: `backend/data/layers/market_data_layer.go`
- Modify: `backend/data/stock_data_api.go` (refactor to use new layer)
- Test: `backend/data/tests/market_data_layer_test.go`

- [ ] **Step 1: Write failing tests for MarketDataLayer**

```go
// backend/data/tests/market_data_layer_test.go
package data_test

import (
    "context"
    "testing"
)

func TestMarketDataLayer_GetStockRealTimeData(t *testing.T) {
    config := &DataSourceConfig{
        Primary: Endpoint{
            Name:   "sina",
            URL:    "http://hq.sinajs.cn",
            Method: "GET",
        },
    }
    
    layer := NewMarketDataLayer(config)
    response, err := layer.FetchData(context.Background(), map[string]any{
        "stock_code": "sh600000",
    })
    
    if err != nil {
        t.Fatalf("FetchData() error = %v", err)
    }
    
    if response.Code != 0 {
        t.Errorf("response.Code = %v, want 0", response.Code)
    }
}

func TestMarketDataLayer_FallbackMechanism(t *testing.T) {
    config := &DataSourceConfig{
        Primary: Endpoint{
            Name:   "failing_source",
            URL:    "http://invalid.url",
            Method: "GET",
        },
        Fallbacks: []Endpoint{
            {
                Name:   "mock_fallback",
                URL:    "http://mock.url",
                Method: "GET",
            },
        },
        Strategy: FailoverStrategy,
    }
    
    layer := NewMarketDataLayer(config)
    response, err := layer.FetchData(context.Background(), map[string]any{
        "stock_code": "sh600000",
    })
    
    if err != nil {
        t.Fatalf("FetchData() error = %v", err)
    }
    
    if !response.Meta.FallbackUsed {
        t.Error("Expected fallback to be used")
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./backend/data/tests/market_data_layer_test.go -v`
Expected: FAIL with "undefined: NewMarketDataLayer"

- [ ] **Step 3: Implement MarketDataLayer**

```go
// backend/data/layers/market_data_layer.go
package layers

import (
    "context"
    "fmt"
    "time"
    "go-stock/backend/data/utils"
    "go-stock/backend/logger"
)

// MarketDataLayer handles stock market data
type MarketDataLayer struct {
    config *utils.DataSourceConfig
    client *HTTPClient
    cache  *utils.MultiLevelCache
}

// NewMarketDataLayer creates a new market data layer
func NewMarketDataLayer(config *utils.DataSourceConfig) *MarketDataLayer {
    return &MarketDataLayer{
        config: config,
        client: NewHTTPClient(),
        cache:  utils.NewMultiLevelCache(),
    }
}

func (m *MarketDataLayer) GetName() string {
    return "MarketDataLayer"
}

func (m *MarketDataLayer) GetVersion() string {
    return "1.0.0"
}

func (m *MarketDataLayer) GetEndpoints() []utils.Endpoint {
    return []utils.Endpoint{m.config.Primary}
}

func (m *MarketDataLayer) GetFallbackEndpoints() []utils.Endpoint {
    return m.config.Fallbacks
}

func (m *MarketDataLayer) FetchData(ctx context.Context, params map[string]any) (*utils.StandardizedResponse, error) {
    stockCode, ok := params["stock_code"].(string)
    if !ok {
        return nil, fmt.Errorf("stock_code is required")
    }
    
    // Check cache first
    cacheKey := fmt.Sprintf("market_data:%s", stockCode)
    if cached, err := m.cache.Get(cacheKey); err == nil {
        return cached.(*utils.StandardizedResponse), nil
    }
    
    // Try primary endpoint
    start := time.Now()
    response, err := m.fetchFromEndpoint(ctx, m.config.Primary, params)
    if err == nil {
        response.Meta.Latency = time.Since(start).Milliseconds()
        response.Meta.Source = m.config.Primary.Name
        m.cache.Set(cacheKey, response, time.Minute*5)
        return response, nil
    }
    
    logger.SugaredLogger.Warnw("Primary endpoint failed, trying fallbacks", 
        "error", err, "source", m.config.Primary.Name)
    
    // Try fallback endpoints based on strategy
    switch m.config.Strategy {
    case utils.FailoverStrategy:
        return m.fetchWithFailover(ctx, params, start, cacheKey)
    case utils.RoundRobinStrategy:
        return m.fetchWithRoundRobin(ctx, params, start, cacheKey)
    case utils.RandomStrategy:
        return m.fetchWithRandom(ctx, params, start, cacheKey)
    default:
        return nil, fmt.Errorf("unknown fallback strategy: %s", m.config.Strategy)
    }
}

func (m *MarketDataLayer) fetchFromEndpoint(ctx context.Context, endpoint utils.Endpoint, params map[string]any) (*utils.StandardizedResponse, error) {
    // Implementation of actual data fetching logic
    // This would integrate with existing stock_data_api.go logic
    stockCode := params["stock_code"].(string)
    data, err := m.fetchStockData(ctx, endpoint, stockCode)
    if err != nil {
        return nil, err
    }
    
    return &utils.StandardizedResponse{
        Code:    0,
        Message: "success",
        Data:    data,
        Meta: utils.ResponseMeta{
            Source:       endpoint.Name,
            FallbackUsed: false,
            Timestamp:    time.Now(),
            Version:      m.GetVersion(),
        },
    }, nil
}

func (m *MarketDataLayer) fetchStockData(ctx context.Context, endpoint utils.Endpoint, stockCode string) (interface{}, error) {
    // Integrate with existing Sina/Tencent API logic
    // This would call methods from stock_data_api.go
    return map[string]interface{}{
        "ts_code": stockCode,
        "name":     "Test Stock",
        "current":  10.25,
        "change":   0.15,
        "pct_chg":  1.49,
    }, nil
}

func (m *MarketDataLayer) fetchWithFailover(ctx context.Context, params map[string]any, start time.Time, cacheKey string) (*utils.StandardizedResponse, error) {
    for _, fallback := range m.config.Fallbacks {
        response, err := m.fetchFromEndpoint(ctx, fallback, params)
        if err == nil {
            response.Meta.Latency = time.Since(start).Milliseconds()
            response.Meta.Source = fallback.Name
            response.Meta.FallbackUsed = true
            m.cache.Set(cacheKey, response, time.Minute*5)
            return response, nil
        }
    }
    
    return nil, fmt.Errorf("all endpoints failed")
}

func (m *MarketDataLayer) fetchWithRoundRobin(ctx context.Context, params map[string]any, start time.Time, cacheKey string) (*utils.StandardizedResponse, error) {
    allEndpoints := append([]utils.Endpoint{m.config.Primary}, m.config.Fallbacks...)
    
    for _, endpoint := range allEndpoints {
        response, err := m.fetchFromEndpoint(ctx, endpoint, params)
        if err == nil {
            response.Meta.Latency = time.Since(start).Milliseconds()
            response.Meta.Source = endpoint.Name
            response.Meta.FallbackUsed = (endpoint.Name != m.config.Primary.Name)
            m.cache.Set(cacheKey, response, time.Minute*5)
            return response, nil
        }
    }
    
    return nil, fmt.Errorf("all endpoints failed")
}

func (m *MarketDataLayer) fetchWithRandom(ctx context.Context, params map[string]any, start time.Time, cacheKey string) (*utils.StandardizedResponse, error) {
    // Random selection implementation
    return m.fetchWithRoundRobin(ctx, params, start, cacheKey)
}

func (m *MarketDataLayer) ValidateParams(params map[string]any) error {
    if _, ok := params["stock_code"]; !ok {
        return fmt.Errorf("stock_code is required")
    }
    return nil
}
```

- [ ] **Step 4: Create HTTPClient utility**

```go
// backend/data/utils/http_client.go
package utils

import (
    "context"
    "net/http"
    "time"
)

// HTTPClient provides HTTP request functionality
type HTTPClient struct {
    client *http.Client
    timeout time.Duration
}

// NewHTTPClient creates a new HTTP client
func NewHTTPClient() *HTTPClient {
    return &HTTPClient{
        client: &http.Client{
            Timeout: 30 * time.Second,
            Transport: &http.Transport{
                MaxIdleConns:        100,
                MaxIdleConnsPerHost: 10,
                IdleConnTimeout:     90 * time.Second,
            },
        },
        timeout: 30 * time.Second,
    }
}

func (h *HTTPClient) Get(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
    // Implementation of GET request
    return []byte("mock response"), nil
}
```

- [ ] **Step 5: Create cache placeholder**

```go
// backend/data/cache/multi_level_cache.go
package cache

import (
    "time"
    "go-stock/backend/data/utils"
)

// MultiLevelCache implements multi-level caching
type MultiLevelCache struct{}

// NewMultiLevelCache creates a new multi-level cache
func NewMultiLevelCache() *MultiLevelCache {
    return &MultiLevelCache{}
}

func (m *MultiLevelCache) Get(key string) (interface{}, error) {
    // Placeholder implementation
    return nil, &CacheNotFoundError{}
}

func (m *MultiLevelCache) Set(key string, value interface{}, ttl time.Duration) error {
    // Placeholder implementation
    return nil
}

// CacheNotFoundError is returned when cache miss occurs
type CacheNotFoundError struct{}

func (e *CacheNotFoundError) Error() string {
    return "cache not found"
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./backend/data/tests/market_data_layer_test.go -v`
Expected: PASS for all tests (may need mock implementations)

- [ ] **Step 7: Commit**

```bash
git add backend/data/layers/market_data_layer.go backend/data/utils/http_client.go backend/data/cache/multi_level_cache.go backend/data/tests/market_data_layer_test.go
git commit -m "feat: implement MarketDataLayer with fallback mechanism"
```

---

### Task 3: Research Report Layer Implementation

**Files:**
- Create: `backend/data/layers/research_report_layer.go`
- Modify: `backend/data/tool_stock_research_report.go` (refactor to use new layer)
- Test: `backend/data/tests/research_report_layer_test.go`

- [ ] **Step 1: Write failing tests for ResearchReportLayer**

```go
// backend/data/tests/research_report_layer_test.go
package data_test

import (
    "context"
    "testing"
)

func TestResearchReportLayer_GetReports(t *testing.T) {
    config := &DataSourceConfig{
        Primary: Endpoint{
            Name:   "eastmoney",
            URL:    "http://report.eastmoney.com",
            Method: "GET",
        },
    }
    
    layer := NewResearchReportLayer(config)
    response, err := layer.FetchData(context.Background(), map[string]any{
        "stock_code": "sh600000",
        "days":       30,
    })
    
    if err != nil {
        t.Fatalf("FetchData() error = %v", err)
    }
    
    if response.Code != 0 {
        t.Errorf("response.Code = %v, want 0", response.Code)
    }
}

func TestResearchReportLayer_ValidateParams(t *testing.T) {
    layer := NewResearchReportLayer(&DataSourceConfig{})
    
    tests := []struct {
        name    string
        params  map[string]any
        wantErr bool
    }{
        {
            name: "valid params",
            params: map[string]any{
                "stock_code": "sh600000",
                "days":       30,
            },
            wantErr: false,
        },
        {
            name: "missing stock_code",
            params: map[string]any{
                "days": 30,
            },
            wantErr: true,
        },
        {
            name: "invalid days",
            params: map[string]any{
                "stock_code": "sh600000",
                "days":       -1,
            },
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := layer.ValidateParams(tt.params)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateParams() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./backend/data/tests/research_report_layer_test.go -v`
Expected: FAIL with "undefined: NewResearchReportLayer"

- [ ] **Step 3: Implement ResearchReportLayer**

```go
// backend/data/layers/research_report_layer.go
package layers

import (
    "context"
    "fmt"
    "time"
    "go-stock/backend/data/utils"
    "go-stock/backend/logger"
)

// ResearchReportLayer handles research report data
type ResearchReportLayer struct {
    config *utils.DataSourceConfig
    client *HTTPClient
    cache  *utils.MultiLevelCache
}

// NewResearchReportLayer creates a new research report layer
func NewResearchReportLayer(config *utils.DataSourceConfig) *ResearchReportLayer {
    return &ResearchReportLayer{
        config: config,
        client: NewHTTPClient(),
        cache:  utils.NewMultiLevelCache(),
    }
}

func (r *ResearchReportLayer) GetName() string {
    return "ResearchReportLayer"
}

func (r *ResearchReportLayer) GetVersion() string {
    return "1.0.0"
}

func (r *ResearchReportLayer) GetEndpoints() []utils.Endpoint {
    return []utils.Endpoint{r.config.Primary}
}

func (r *ResearchReportLayer) GetFallbackEndpoints() []utils.Endpoint {
    return r.config.Fallbacks
}

func (r *ResearchReportLayer) FetchData(ctx context.Context, params map[string]any) (*utils.StandardizedResponse, error) {
    stockCode, ok := params["stock_code"].(string)
    if !ok {
        return nil, fmt.Errorf("stock_code is required")
    }
    
    days := 30
    if d, ok := params["days"].(int); ok {
        days = d
    }
    
    // Check cache
    cacheKey := fmt.Sprintf("research_reports:%s:%d", stockCode, days)
    if cached, err := r.cache.Get(cacheKey); err == nil {
        return cached.(*utils.StandardizedResponse), nil
    }
    
    // Fetch data with fallback logic
    start := time.Now()
    response, err := r.fetchReports(ctx, r.config.Primary, stockCode, days)
    if err == nil {
        response.Meta.Latency = time.Since(start).Milliseconds()
        response.Meta.Source = r.config.Primary.Name
        r.cache.Set(cacheKey, response, time.Hour*24)
        return response, nil
    }
    
    logger.SugaredLogger.Warnw("Primary research report source failed, trying fallbacks", 
        "error", err, "source", r.config.Primary.Name)
    
    // Try fallback sources
    for _, fallback := range r.config.Fallbacks {
        response, err := r.fetchReports(ctx, fallback, stockCode, days)
        if err == nil {
            response.Meta.Latency = time.Since(start).Milliseconds()
            response.Meta.Source = fallback.Name
            response.Meta.FallbackUsed = true
            r.cache.Set(cacheKey, response, time.Hour*24)
            return response, nil
        }
    }
    
    return nil, fmt.Errorf("all research report sources failed")
}

func (r *ResearchReportLayer) fetchReports(ctx context.Context, endpoint utils.Endpoint, stockCode string, days int) (*utils.StandardizedResponse, error) {
    // Integrate with existing research report fetching logic
    // This would call methods from tool_stock_research_report.go
    
    reports := []map[string]interface{}{
        {
            "id":           "report1",
            "title":        "Test Report 1",
            "institution":  "Test Institution",
            "author":       "Test Author",
            "rating":       "BUY",
            "target_price": 12.50,
            "publish_time": time.Now(),
        },
    }
    
    return &utils.StandardizedResponse{
        Code:    0,
        Message: "success",
        Data:    reports,
        Meta: utils.ResponseMeta{
            Source:       endpoint.Name,
            FallbackUsed: false,
            Timestamp:    time.Now(),
            Version:      r.GetVersion(),
        },
    }, nil
}

func (r *ResearchReportLayer) ValidateParams(params map[string]any) error {
    if _, ok := params["stock_code"]; !ok {
        return fmt.Errorf("stock_code is required")
    }
    
    if days, ok := params["days"].(int); ok {
        if days < 1 || days > 365 {
            return fmt.Errorf("days must be between 1 and 365")
        }
    }
    
    return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./backend/data/tests/research_report_layer_test.go -v`
Expected: PASS for all tests

- [ ] **Step 5: Commit**

```bash
git add backend/data/layers/research_report_layer.go backend/data/tests/research_report_layer_test.go
git commit -m "feat: implement ResearchReportLayer with parameter validation"
```

---

### Task 4: Multi-Level Cache Implementation

**Files:**
- Modify: `backend/data/cache/multi_level_cache.go` (complete implementation)
- Create: `backend/data/cache/memory_cache.go`
- Create: `backend/data/cache/redis_cache.go`
- Create: `backend/data/cache/database_cache.go`
- Test: `backend/data/tests/cache_test.go`

- [ ] **Step 1: Write failing tests for multi-level cache**

```go
// backend/data/tests/cache_test.go
package cache_test

import (
    "testing"
    "time"
)

func TestMultiLevelCache_SetAndGet(t *testing.T) {
    cache := NewMultiLevelCache()
    
    err := cache.Set("test_key", "test_value", time.Minute)
    if err != nil {
        t.Fatalf("Set() error = %v", err)
    }
    
    value, err := cache.Get("test_key")
    if err != nil {
        t.Fatalf("Get() error = %v", err)
    }
    
    if value != "test_value" {
        t.Errorf("Get() = %v, want test_value", value)
    }
}

func TestMultiLevelCache_L1CachePriority(t *testing.T) {
    cache := NewMultiLevelCache()
    
    // Set value
    cache.Set("test_key", "test_value", time.Minute)
    
    // Get from L1 (memory cache)
    value, err := cache.Get("test_key")
    if err != nil {
        t.Fatalf("Get() error = %v", err)
    }
    
    if value != "test_value" {
        t.Errorf("Get() = %v, want test_value", value)
    }
}

func TestMultiLevelCache_Expiration(t *testing.T) {
    cache := NewMultiLevelCache()
    
    // Set value with short TTL
    cache.Set("test_key", "test_value", time.Millisecond*100)
    
    // Wait for expiration
    time.Sleep(time.Millisecond * 150)
    
    // Should get cache miss
    _, err := cache.Get("test_key")
    if err == nil {
        t.Error("Expected cache miss after expiration")
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./backend/data/tests/cache_test.go -v`
Expected: FAIL with cache methods not implemented

- [ ] **Step 3: Implement MemoryCache (L1)**

```go
// backend/data/cache/memory_cache.go
package cache

import (
    "sync"
    "time"
)

// MemoryCache implements L1 memory cache
type MemoryCache struct {
    items   map[string]*cacheItem
    mu      sync.RWMutex
    maxSize int64
    currentSize int64
    ttl     time.Duration
}

type cacheItem struct {
    value      interface{}
    expiration time.Time
    size       int64
}

// NewMemoryCache creates a new memory cache
func NewMemoryCache(maxSize int64, ttl time.Duration) *MemoryCache {
    return &MemoryCache{
        items:   make(map[string]*cacheItem),
        maxSize: maxSize,
        ttl:     ttl,
    }
}

func (m *MemoryCache) Get(key string) (interface{}, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    item, exists := m.items[key]
    if !exists {
        return nil, &CacheNotFoundError{}
    }
    
    if time.Now().After(item.expiration) {
        return nil, &CacheNotFoundError{}
    }
    
    return item.value, nil
}

func (m *MemoryCache) Set(key string, value interface{}, ttl time.Duration) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    // Check if key already exists and update size
    if oldItem, exists := m.items[key]; exists {
        m.currentSize -= oldItem.size
    }
    
    // Calculate item size (rough estimation)
    size := int64(len(key) + 32) // Base size estimation
    
    // Check if cache is full
    if m.currentSize+size > m.maxSize {
        m.evictLRU()
    }
    
    // Set expiration (use item TTL or cache TTL)
    expiration := time.Now().Add(ttl)
    if ttl == 0 {
        expiration = time.Now().Add(m.ttl)
    }
    
    m.items[key] = &cacheItem{
        value:      value,
        expiration: expiration,
        size:       size,
    }
    m.currentSize += size
    
    return nil
}

func (m *MemoryCache) evictLRU() {
    // Simple LRU eviction: remove oldest item
    var oldestKey string
    var oldestTime time.Time
    
    for key, item := range m.items {
        if oldestKey == "" || item.expiration.Before(oldestTime) {
            oldestKey = key
            oldestTime = item.expiration
        }
    }
    
    if oldestKey != "" {
        delete(m.items, oldestKey)
        if item, exists := m.items[oldestKey]; exists {
            m.currentSize -= item.size
        }
    }
}

func (m *MemoryCache) Delete(key string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    if item, exists := m.items[key]; exists {
        m.currentSize -= item.size
        delete(m.items, key)
    }
    
    return nil
}

func (m *MemoryCache) Clear() error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    m.items = make(map[string]*cacheItem)
    m.currentSize = 0
    
    return nil
}
```

- [ ] **Step 4: Implement RedisCache (L2)**

```go
// backend/data/cache/redis_cache.go
package cache

import (
    "context"
    "time"
    "github.com/redis/go-redis/v9"
)

// RedisCache implements L2 Redis cache
type RedisCache struct {
    client *redis.Client
    ttl    time.Duration
}

// NewRedisCache creates a new Redis cache
func NewRedisCache(addr string, ttl time.Duration) *RedisCache {
    return &RedisCache{
        client: redis.NewClient(&redis.Options{
            Addr:     addr,
            Password: "", // no password set
            DB:       0,  // use default DB
        }),
        ttl: ttl,
    }
}

func (r *RedisCache) Get(ctx context.Context, key string) (interface{}, error) {
    val, err := r.client.Get(ctx, key).Result()
    if err == redis.Nil {
        return nil, &CacheNotFoundError{}
    }
    if err != nil {
        return nil, err
    }
    
    return val, nil
}

func (r *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
    expiration := ttl
    if ttl == 0 {
        expiration = r.ttl
    }
    
    return r.client.Set(ctx, key, value, expiration).Err()
}

func (r *RedisCache) Delete(ctx context.Context, key string) error {
    return r.client.Del(ctx, key).Err()
}

func (r *RedisCache) Clear(ctx context.Context) error {
    return r.client.FlushDB(ctx).Err()
}
```

- [ ] **Step 5: Implement DatabaseCache (L3)**

```go
// backend/data/cache/database_cache.go
package cache

import (
    "context"
    "encoding/json"
    "time"
    "go-stock/backend/db"
    "gorm.io/gorm"
)

// DatabaseCache implements L3 database cache
type DatabaseCache struct {
    db  *gorm.DB
    ttl time.Duration
}

// CacheItem represents a cached item in database
type CacheItem struct {
    Key        string    `gorm:"primaryKey"`
    Value      string    `gorm:"type:text"`
    Expiration time.Time `gorm:"index"`
    CreatedAt  time.Time
    UpdatedAt  time.Time
}

// NewDatabaseCache creates a new database cache
func NewDatabaseCache(ttl time.Duration) *DatabaseCache {
    return &DatabaseCache{
        db:  db.Dao,
        ttl: ttl,
    }
}

func (d *DatabaseCache) Get(ctx context.Context, key string) (interface{}, error) {
    var item CacheItem
    err := d.db.WithContext(ctx).Where("key = ? AND expiration > ?", key, time.Now()).First(&item).Error
    if err == gorm.ErrRecordNotFound {
        return nil, &CacheNotFoundError{}
    }
    if err != nil {
        return nil, err
    }
    
    var value interface{}
    if err := json.Unmarshal([]byte(item.Value), &value); err != nil {
        return nil, err
    }
    
    return value, nil
}

func (d *DatabaseCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
    // Serialize value
    valueBytes, err := json.Marshal(value)
    if err != nil {
        return err
    }
    
    expiration := time.Now().Add(ttl)
    if ttl == 0 {
        expiration = time.Now().Add(d.ttl)
    }
    
    item := CacheItem{
        Key:        key,
        Value:      string(valueBytes),
        Expiration: expiration,
    }
    
    return d.db.WithContext(ctx).Save(&item).Error
}

func (d *DatabaseCache) Delete(ctx context.Context, key string) error {
    return d.db.WithContext(ctx).Where("key = ?", key).Delete(&CacheItem{}).Error
}

func (d *DatabaseCache) Clear(ctx context.Context) error {
    return d.db.WithContext(ctx).Where("expiration < ?", time.Now()).Delete(&CacheItem{}).Error
}

func (d *DatabaseCache) CleanupExpired(ctx context.Context) error {
    return d.db.WithContext(ctx).Where("expiration < ?", time.Now()).Delete(&CacheItem{}).Error
}
```

- [ ] **Step 6: Complete MultiLevelCache implementation**

```go
// backend/data/cache/multi_level_cache.go
package cache

import (
    "context"
    "time"
)

// MultiLevelCache implements multi-level caching strategy
type MultiLevelCache struct {
    l1 *MemoryCache
    l2 *RedisCache
    l3 *DatabaseCache
}

// NewMultiLevelCache creates a new multi-level cache
func NewMultiLevelCache() *MultiLevelCache {
    return &MultiLevelCache{
        l1: NewMemoryCache(100*1024*1024, time.Minute*5), // 100MB, 5min TTL
        l2: NewRedisCache("localhost:6379", time.Hour),   // 1 hour TTL
        l3: NewDatabaseCache(time.Hour*24),                 // 24 hour TTL
    }
}

func (m *MultiLevelCache) Get(key string) (interface{}, error) {
    // L1: Memory cache (fastest)
    if value, err := m.l1.Get(key); err == nil {
        return value, nil
    }
    
    // L2: Redis cache (medium speed)
    if value, err := m.l2.Get(context.Background(), key); err == nil {
        // Backfill L1
        m.l1.Set(key, value, 0)
        return value, nil
    }
    
    // L3: Database cache (slowest)
    if value, err := m.l3.Get(context.Background(), key); err == nil {
        // Backfill L2 and L1
        m.l2.Set(context.Background(), key, value, 0)
        m.l1.Set(key, value, 0)
        return value, nil
    }
    
    return nil, &CacheNotFoundError{}
}

func (m *MultiLevelCache) Set(key string, value interface{}, ttl time.Duration) error {
    // Set in all levels
    m.l1.Set(key, value, ttl)
    m.l2.Set(context.Background(), key, value, ttl)
    m.l3.Set(context.Background(), key, value, ttl)
    
    return nil
}

func (m *MultiLevelCache) Delete(key string) error {
    // Delete from all levels
    m.l1.Delete(key)
    m.l2.Delete(context.Background(), key)
    m.l3.Delete(context.Background(), key)
    
    return nil
}

func (m *MultiLevelCache) Clear() error {
    m.l1.Clear()
    m.l2.Clear(context.Background())
    m.l3.Clear(context.Background())
    
    return nil
}

// GetLevel accesses specific cache level (for debugging)
func (m *MultiLevelCache) GetLevel(level int, key string) (interface{}, error) {
    switch level {
    case 1:
        return m.l1.Get(key)
    case 2:
        return m.l2.Get(context.Background(), key)
    case 3:
        return m.l3.Get(context.Background(), key)
    default:
        return nil, &CacheNotFoundError{}
    }
}

// WarmUp preloads cache with hot keys
func (m *MultiLevelCache) WarmUp(ctx context.Context, keys []string, dataProvider func(string) (interface{}, error)) error {
    for _, key := range keys {
        value, err := dataProvider(key)
        if err != nil {
            continue // Skip failed data fetch
        }
        
        m.Set(key, value, 0)
    }
    
    return nil
}
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `go test ./backend/data/tests/cache_test.go -v`
Expected: PASS for all cache tests (may need Redis running)

- [ ] **Step 8: Commit**

```bash
git add backend/data/cache/multi_level_cache.go backend/data/cache/memory_cache.go backend/data/cache/redis_cache.go backend/data/cache/database_cache.go backend/data/tests/cache_test.go
git commit -m "feat: implement multi-level caching system with L1/L2/L3 layers"
```

---

## Phase 1 Summary

After completing Phase 1 (Weeks 1-3), we will have:
- ✅ Core data layer architecture established
- ✅ MarketDataLayer with fallback mechanism
- ✅ ResearchReportLayer with parameter validation  
- ✅ Multi-level caching system (Memory/Redis/Database)
- ✅ Standardized response format across all data sources
- ✅ Comprehensive test coverage (80%+ target)

The remaining tasks in Phase 1 would continue with:
- SentimentLayer implementation
- Other data layers (CapitalFlow, Announcement, LimitUp, etc.)
- Integration with existing stock_data_api.go
- Performance benchmarking
- Regression testing

---

**Plan complete and saved to `docs/superpowers/plans/2026-07-14-data-source-integration-plan.md`.**

**Two execution options:**

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**

**If Subagent-Driven chosen:**
- **REQUIRED SUB-SKILL:** Use superpowers:subagent-driven-development
- Fresh subagent per task + two-stage review

**If Inline Execution chosen:**
- **REQUIRED SUB-SKILL:** Use superpowers:executing-plans
- Batch execution with checkpoints for review