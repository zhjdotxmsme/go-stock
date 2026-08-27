package data

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"go-stock/backend/data/datasource"
	"go-stock/backend/logger"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-resty/resty/v2"
)

// yahooHTTPClient 是 Yahoo 专用的 HTTP 客户端。
var yahooHTTPClient *resty.Client
var yahooClientOnce sync.Once
var yahooRateLimited bool         // 记录 Yahoo HTTP 上次是否被限流，跳过重试
var yahooRateLimitReset time.Time // 限流标记重置时间

func isYahooRateLimited() bool {
	if !yahooRateLimited {
		return false
	}
	// 每 5 分钟重置一次，周期性尝试 HTTP
	if time.Since(yahooRateLimitReset) > 5*time.Minute {
		yahooRateLimited = false
		return false
	}
	return true
}

func markYahooRateLimited() {
	yahooRateLimited = true
	yahooRateLimitReset = time.Now()
}

func getYahooClient() *resty.Client {
	yahooClientOnce.Do(func() {
		tr := &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 15 * time.Second,
			}).DialContext,
			MaxIdleConnsPerHost: 2,
			IdleConnTimeout:     30 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
			// 使用 TLS 1.2，禁用 HTTP/2
			TLSNextProto: make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
		}
		hc := &http.Client{Transport: tr, Timeout: 6 * time.Second}
		yahooHTTPClient = resty.NewWithClient(hc).
			SetTimeout(6 * time.Second).
			SetRetryCount(0).
			SetRetryWaitTime(0 * time.Millisecond)
	})
	return yahooHTTPClient
}

// --- Package-level fetch functions (shared by all Yahoo API clients) ---

// yahooFetch 使用专用 HTTP 客户端请求 Yahoo API，支持子域名轮询与超时重试。
// 如果 HTTP 方式均被限流，Windows 下降级到 PowerShell（WinHTTP，不受 TLS 指纹限流影响）。
func yahooFetch(urlStr string) ([]byte, error) {
	// 如果之前 HTTP 已被限流过，直接跳过 HTTP 尝试
	if !isYahooRateLimited() {
		for _, sub := range []string{"query1", "query2"} {
			altURL := strings.Replace(urlStr, "query1.finance.yahoo.com", sub+".finance.yahoo.com", 1)
			body, err := yahooDoRequest(altURL)
			if err == nil {
				return body, nil
			}
		}
		markYahooRateLimited() // 所有 HTTP 子域名均失败，标记限流
	}
	// HTTP 方式失败（通常是被限流），降级到 PowerShell WinHTTP（仅 Windows 可用）
	body, err := yahooFetchViaPowerShell(urlStr)
	if err == nil {
		logger.SugaredLogger.Infof("Yahoo PowerShell fallback succeeded for %s", urlStr)
		return body, nil
	}
	return nil, fmt.Errorf("yahoo all subdomains (and PowerShell fallback) failed")
}

// yahooFetchViaPowerShell 通过 PowerShell 的 Invoke-WebRequest（WinHTTP）发起请求，
// 绕过 Go TLS 指纹被 Yahoo 限流的问题。
func yahooFetchViaPowerShell(urlStr string) ([]byte, error) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-WindowStyle", "Hidden", "-Command",
		`try { $r = Invoke-WebRequest -Uri '`+urlStr+`' -UseBasicParsing -TimeoutSec 10 -ErrorAction Stop; Write-Output $r.Content } catch { exit 1 }`)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("yahoo powershell fallback: %w", err)
	}
	return out, nil
}

// yahooDoRequest 执行一次 Yahoo API 请求
func yahooDoRequest(urlStr string) ([]byte, error) {
	client := getYahooClient()
	resp, err := client.R().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		SetHeader("Accept", "application/json").
		SetHeader("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8").
		SetHeader("Accept-Encoding", "gzip, deflate").
		Get(urlStr)
	if err != nil {
		return nil, fmt.Errorf("yahoo request: %w", err)
	}
	body := resp.Body()
	if len(body) > 0 && body[0] == '<' {
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return nil, fmt.Errorf("yahoo rate-limited (HTML: %s...)", snippet)
	}
	return body, nil
}

// --- Package-level fetch functions (shared by all Yahoo API clients) ---

// YahooFinanceApi 提供 Yahoo Finance 行情与 K 线数据，支持全球市场。
type YahooFinanceApi struct {
	resolver *YahooSymbolResolver // 新增：全局代码解析器
}

// NewYahooFinanceApi creates a Yahoo Finance API client with global symbol resolution.
func NewYahooFinanceApi() *YahooFinanceApi {
	return &YahooFinanceApi{
		resolver: NewYahooSymbolResolver(),
	}
}

// resolveSymbol resolves go-stock code to Yahoo Finance symbol.
// First tries the global resolver, then falls back to legacy commodity mapping.
func (y *YahooFinanceApi) resolveSymbol(code string) (string, error) {
	// Try global resolver first (safe for nil resolver)
	if y.resolver != nil {
		if sym, err := y.resolver.Resolve(code); err == nil {
			return sym, nil
		}
	}
	// Fallback to legacy commodity mapping (for backward compatibility)
	if sym, ok := yahooCommoditySymbols[strings.ToLower(code)]; ok {
		return sym, nil
	}
	return "", fmt.Errorf("Yahoo Finance 不支持品种: %s", code)
}

// GetQuote 获取实时行情，使用 Yahoo Finance chart API (range=1d)。
// This is the Provider interface implementation (with context).
func (y *YahooFinanceApi) GetQuote(ctx context.Context, code string) (*datasource.QuoteData, error) {
	symbol, err := y.resolveSymbol(code)
	if err != nil {
		return nil, err
	}

	urlStr := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=1d", url.QueryEscape(symbol))
	body, err := yahooFetch(urlStr)
	if err != nil {
		return nil, err
	}

	chart, err := parseYahooChart(body)
	if err != nil {
		return nil, fmt.Errorf("yahoo quote parse: %w", err)
	}

	result := chart.Chart.Result[0]
	meta := result.Meta
	price := meta.RegularMarketPrice
	if price == 0 && len(result.Indicators.Quote) > 0 {
		q := result.Indicators.Quote[0]
		for i := len(q.Close) - 1; i >= 0; i-- {
			if q.Close[i] != nil {
				price = *q.Close[i]
				break
			}
		}
	}
	if price == 0 {
		return nil, fmt.Errorf("yahoo quote: no price for %s", symbol)
	}

	prevClose := meta.PreviousClose
	if prevClose == 0 && meta.ChartPreviousClose != 0 {
		prevClose = meta.ChartPreviousClose
	}
	change := price - prevClose
	changePct := 0.0
	if prevClose != 0 {
		changePct = change / prevClose * 100
	}

	name := meta.Symbol
	if name == "" {
		name = code
	}

	return &datasource.QuoteData{
		Code:      code,
		Name:      name,
		Price:     price,
		Change:    change,
		ChangePct: changePct,
		Time:      time.Now(),
		Extra: map[string]interface{}{
			"currency": meta.Currency,
			"source":   "yahoo",
		},
	}, nil
}

// GetKLine 获取历史 K 线。
// period 支持 day/week/month；count 用于估算 range。
// This is the Provider interface implementation (with context).
func (y *YahooFinanceApi) GetKLine(ctx context.Context, code, period string, count int) (*datasource.KLineData, error) {
	symbol, err := y.resolveSymbol(code)
	if err != nil {
		return nil, err
	}

	interval := "1d"
	switch period {
	case "week":
		interval = "1wk"
	case "month":
		interval = "1mo"
	}

	rangeParam := yahooRangeForCount(count, period)
	urlStr := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=%s&range=%s", url.QueryEscape(symbol), interval, rangeParam)
	body, err := yahooFetch(urlStr)
	if err != nil {
		return nil, err
	}

	chart, err := parseYahooChart(body)
	if err != nil {
		return nil, fmt.Errorf("yahoo kline parse: %w", err)
	}

	bars, err := yahooBarsFromChart(chart, code, count)
	if err != nil {
		return nil, err
	}

	return &datasource.KLineData{
		Code:   code,
		Period: period,
		Bars:   bars,
	}, nil
}

// GetKLineBars 获取历史 K 线条（兼容旧接口）
func (y *YahooFinanceApi) GetKLineBars(code, period string, count int) ([]datasource.KLineBar, error) {
	kd, err := y.GetKLine(context.Background(), code, period, count)
	if err != nil {
		return nil, err
	}
	return kd.Bars, nil
}

// --- Backward-compatible methods (no context, used by commodity_api.go and others) ---

// GetQuoteNoCtx is the backward-compatible version without context parameter.
func (y *YahooFinanceApi) GetQuoteNoCtx(code string) (*datasource.QuoteData, error) {
	return y.GetQuote(context.Background(), code)
}

// GetKLineNoCtx is the backward-compatible version without context parameter.
func (y *YahooFinanceApi) GetKLineNoCtx(code, period string, count int) ([]datasource.KLineBar, error) {
	return y.GetKLineBars(code, period, count)
}

// yahooRangeForCount 根据期望条数和周期选择 Yahoo range 参数。
func yahooRangeForCount(count int, period string) string {
	if count <= 0 {
		count = 120
	}
	switch period {
	case "week":
		if count <= 52 {
			return "1y"
		}
		if count <= 104 {
			return "2y"
		}
		return "5y"
	case "month":
		if count <= 12 {
			return "1y"
		}
		if count <= 36 {
			return "3y"
		}
		return "10y"
	default:
		if count <= 5 {
			return "5d"
		}
		if count <= 30 {
			return "1mo"
		}
		if count <= 90 {
			return "3mo"
		}
		if count <= 180 {
			return "6mo"
		}
		if count <= 365 {
			return "1y"
		}
		if count <= 730 {
			return "2y"
		}
		return "5y"
	}
}

// --- Yahoo chart response structs ---

type yahooChartResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Currency           string  `json:"currency"`
				Symbol             string  `json:"symbol"`
				RegularMarketPrice float64 `json:"regularMarketPrice"`
				PreviousClose      float64 `json:"previousClose"`
				ChartPreviousClose float64 `json:"chartPreviousClose"`
				RegularMarketTime  int64   `json:"regularMarketTime"`
			} `json:"meta"`
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Open   []*float64 `json:"open"`
					High   []*float64 `json:"high"`
					Low    []*float64 `json:"low"`
					Close  []*float64 `json:"close"`
					Volume []*int64   `json:"volume"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}

func parseYahooChart(body []byte) (*yahooChartResponse, error) {
	var resp yahooChartResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	if resp.Chart.Error != nil {
		return nil, fmt.Errorf("yahoo api error: %s - %s", resp.Chart.Error.Code, resp.Chart.Error.Description)
	}
	if len(resp.Chart.Result) == 0 {
		return nil, fmt.Errorf("empty chart result")
	}
	return &resp, nil
}

func yahooBarsFromChart(chart *yahooChartResponse, code string, count int) ([]datasource.KLineBar, error) {
	result := chart.Chart.Result[0]
	if len(result.Timestamp) == 0 {
		return nil, fmt.Errorf("yahoo kline: no timestamps")
	}
	if len(result.Indicators.Quote) == 0 {
		return nil, fmt.Errorf("yahoo kline: no quote data")
	}

	q := result.Indicators.Quote[0]
	n := len(result.Timestamp)
	if len(q.Open) < n || len(q.High) < n || len(q.Low) < n || len(q.Close) < n {
		return nil, fmt.Errorf("yahoo kline: mismatched quote arrays")
	}

	bars := make([]datasource.KLineBar, 0, n)
	for i := 0; i < n; i++ {
		if q.Open[i] == nil || q.High[i] == nil || q.Low[i] == nil || q.Close[i] == nil {
			continue
		}
		var volume int64
		if i < len(q.Volume) && q.Volume[i] != nil {
			volume = *q.Volume[i]
		}
		bars = append(bars, datasource.KLineBar{
			Time:   time.Unix(result.Timestamp[i], 0),
			Open:   *q.Open[i],
			High:   *q.High[i],
			Low:    *q.Low[i],
			Close:  *q.Close[i],
			Volume: volume,
		})
	}

	if len(bars) == 0 {
		return nil, fmt.Errorf("yahoo kline: no valid bars")
	}

	// Yahoo returns oldest first, keep order; take most recent count if needed.
	if count > 0 && len(bars) > count {
		bars = bars[len(bars)-count:]
	}
	return bars, nil
}

// ParseYahooFloat 安全解析 interface{} 到 float64，用于测试中复用。
func ParseYahooFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(val), 64)
		return f
	case int:
		return float64(val)
	case int64:
		return float64(val)
	}
	return 0
}
