package data

import (
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
)

var (
	sharedTransport     *http.Transport
	sharedLoggingRT     http.RoundTripper // 全部出站 HTTP 的 info/error 日志出口
	sharedHTTPClient    *http.Client
	SharedHTTPClient    *resty.Client
	httpConfigMutex     sync.RWMutex
	currentProxyEnabled bool
	currentProxyURL     string
)

// loggingTransport 为所有经过共享连接池的出站 HTTP 请求记录日志：
// 成功（<400）记 info，失败（网络错误或 >=400）记 error，均含耗时。
// 查询串中的 token/key/secret/password 参数值会被脱敏。
type loggingTransport struct {
	inner http.RoundTripper
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := t.inner.RoundTrip(req)
	elapsed := time.Since(start).Round(time.Millisecond)
	target := redactURL(req.URL)
	if err != nil {
		logger.SugaredLogger.Errorf("[HTTP] %s %s failed after %s: %v", req.Method, target, elapsed, err)
		return resp, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		logger.SugaredLogger.Errorf("[HTTP] %s %s -> %d after %s", req.Method, target, resp.StatusCode, elapsed)
	} else {
		logger.SugaredLogger.Infof("[HTTP] %s %s -> %d after %s", req.Method, target, resp.StatusCode, elapsed)
	}
	return resp, err
}

// redactURL 返回脱敏后的请求 URL（敏感查询参数值替换为 ***）。
func redactURL(u *url.URL) string {
	if u == nil {
		return ""
	}
	q := u.Query()
	changed := false
	for k := range q {
		lk := strings.ToLower(k)
		if strings.Contains(lk, "token") || strings.Contains(lk, "key") ||
			strings.Contains(lk, "secret") || strings.Contains(lk, "password") {
			q.Set(k, "***")
			changed = true
		}
	}
	cp := *u
	if changed {
		cp.RawQuery = q.Encode()
	}
	return cp.String()
}

func init() {
	sharedTransport = &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          20,
		MaxIdleConnsPerHost:   4,
		MaxConnsPerHost:       10,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 120 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
		Proxy:                 nil,
	}

	sharedLoggingRT = &loggingTransport{inner: sharedTransport}
	sharedHTTPClient = &http.Client{
		Transport: sharedLoggingRT,
		Timeout:   300 * time.Second,
	}

	SharedHTTPClient = resty.NewWithClient(sharedHTTPClient).
		SetRetryCount(0).
		SetTimeout(300 * time.Second)
}

func UpdateHTTPClientProxy(proxyURL string) {
	httpConfigMutex.Lock()
	defer httpConfigMutex.Unlock()

	if proxyURL == "" || proxyURL == currentProxyURL {
		return
	}

	sharedTransport.Proxy = http.ProxyURL(parseProxyURL(proxyURL))
	currentProxyURL = proxyURL
	currentProxyEnabled = true
}

func DisableHTTPClientProxy() {
	httpConfigMutex.Lock()
	defer httpConfigMutex.Unlock()

	sharedTransport.Proxy = nil
	currentProxyEnabled = false
	currentProxyURL = ""
}

func UpdateHTTPClientTimeout(timeout time.Duration) {
	sharedHTTPClient.Timeout = timeout
	SharedHTTPClient.SetTimeout(timeout)
}

func parseProxyURL(proxyURL string) *url.URL {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil
	}
	return u
}

func ConfigureFromSettings(config *SettingConfig) {
	if config == nil {
		return
	}

	if config.HttpProxyEnabled && config.HttpProxy != "" {
		UpdateHTTPClientProxy(config.HttpProxy)
	} else {
		DisableHTTPClientProxy()
	}

	if config.CrawlTimeOut > 0 {
		UpdateHTTPClientTimeout(time.Duration(config.CrawlTimeOut) * time.Second)
	} else {
		UpdateHTTPClientTimeout(300 * time.Second)
	}
}

func CreateHTTPClientWithTimeout(timeout time.Duration) *resty.Client {
	httpConfigMutex.RLock()
	rt := sharedLoggingRT
	httpConfigMutex.RUnlock()

	httpClient := &http.Client{
		Transport: rt,
		Timeout:   timeout,
	}

	return resty.NewWithClient(httpClient).
		SetTimeout(timeout).
		SetRetryCount(0)
}

// ConfiguredHTTPClient returns a per-instance client that shares the global
// connection pool but carries the configured crawl timeout. Use this instead
// of referencing SharedHTTPClient directly: calling SetTimeout on the shared
// client mutates global state and races with concurrent requests using
// different timeouts.
//
// Falls back to the 300s default when the settings store is unavailable
// (db not initialized yet, e.g. unit tests or early startup) — it must never
// panic, because constructors like NewStockDataApi depend on it.
func ConfiguredHTTPClient() *resty.Client {
	timeout := 300 * time.Second
	if db.Dao != nil {
		if cfg := GetSettingConfig(); cfg != nil && cfg.CrawlTimeOut > 0 {
			timeout = time.Duration(cfg.CrawlTimeOut) * time.Second
		}
	}
	return CreateHTTPClientWithTimeout(timeout)
}

func CreateDownloadClient() *resty.Client {
	httpConfigMutex.RLock()
	transport := sharedTransport
	httpConfigMutex.RUnlock()

	downloadTransport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          5,
		MaxIdleConnsPerHost:   2,
		MaxConnsPerHost:       2,
		IdleConnTimeout:       120 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
		Proxy:                 transport.Proxy,
	}

	downloadHTTPClient := &http.Client{
		Transport: downloadTransport,
		Timeout:   0,
	}

	return resty.NewWithClient(downloadHTTPClient).
		SetTimeout(0).
		SetTransport(&loggingTransport{inner: downloadTransport}).
		SetRetryCount(2).
		SetRetryWaitTime(5 * time.Second).
		SetRetryMaxWaitTime(30 * time.Second)
}
