package data

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go-stock/backend/logger"

	"github.com/go-resty/resty/v2"
)

// 东财行情域名降级策略（移植自上游 go-stock 的同名方案）：
// 主域（push2/push2his）会被服务端限流——典型表现是连接被服务端掐断，
// 应用内高频轮询时更易触发（表现为 EOF）。push2delay 兜底域接口路径与响应
// 格式完全一致（数据为延时更新），主域故障时自动切换。
// 粘性索引（sticky）记住上次成功的域名索引：限流期间直接从兜底域开始，
// 避免每次请求都先去撞一遍限流主域的无效往返。
var (
	emKlineHostIdx atomic.Int32 // K线接口（kline/get、fflow/daykline 等）上次成功的 host 索引
	emQuoteHostIdx atomic.Int32 // 报价接口（stock/get、ulist/clist 等）上次成功的 host 索引
)

// emKlineHosts K线接口 host 候选（主域优先）
func emKlineHosts() []string {
	return []string{"https://push2his.eastmoney.com", "https://push2delay.eastmoney.com"}
}

// emQuoteHosts 报价接口 host 候选（主域优先）
func emQuoteHosts() []string {
	return []string{"https://push2.eastmoney.com", "https://push2delay.eastmoney.com"}
}

// emFallbackFetch 从粘性索引开始依次尝试每个 host，每 host 最多 attempts 次，任一次成功即返回。
// 成功时记录该 host 索引（限流期间下次直接从兜底域开始）；全部失败返回最后一次的错误。
func emFallbackFetch(hosts []string, sticky *atomic.Int32, attempts int, do func(host string) error) error {
	start := int(sticky.Load())
	if start < 0 || start >= len(hosts) {
		start = 0
	}
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for i := 0; i < len(hosts); i++ {
		idx := (start + i) % len(hosts)
		host := hosts[idx]
		for a := 0; a < attempts; a++ {
			if e := do(host); e == nil {
				sticky.Store(int32(idx))
				return nil
			} else {
				lastErr = e
			}
		}
	}
	if lastErr != nil {
		logger.SugaredLogger.Warnf("东财 host 降级全部失败（%d 个 host × %d 次）: %v", len(hosts), attempts, lastErr)
	}
	return lastErr
}

// FetchKlineAnyHost 供跨包调用方（如 handler 层港指交易日检查）使用：
// 从粘性索引开始依次尝试 K线族 host 候选（push2delay 接口同构），GET pathQuery（以 /api/ 开头），
// 响应 JSON 反序列化到 out；全部失败返回最后一次的错误。
func FetchKlineAnyHost(pathQuery string, out interface{}) error {
	return emFallbackFetch(emKlineHosts(), &emKlineHostIdx, 2, func(host string) error {
		r, e := emHTTP11Client(host, 30*time.Second).R().SetResult(out).Get(host + pathQuery)
		if e != nil {
			return e
		}
		if r.StatusCode() != 200 {
			return fmt.Errorf("HTTP %d", r.StatusCode())
		}
		return nil
	})
}

// emHostFromURL 从 URL 提取 host（不带协议与路径）
func emHostFromURL(rawURL string) string {
	h := rawURL
	for _, p := range []string{"https://", "http://"} {
		if strings.HasPrefix(h, p) {
			h = strings.TrimPrefix(h, p)
			break
		}
	}
	if i := strings.IndexAny(h, "/?#"); i >= 0 {
		h = h[:i]
	}
	return h
}

var (
	emHTTP11ClientsMu sync.Mutex
	emHTTP11Clients   = map[string]*resty.Client{}
)

// emHTTP11Client 返回访问指定东财 host 的 HTTP/1.1 客户端（按 host 缓存复用）。
// SNI 跟随目标域（原 K线客户端的 SNI 固定为 push2his，直接访问 push2delay 会 TLS 失败），
// 并禁用 HTTP/2 与自动压缩——与现有东财客户端行为一致（东财对 HTTP/2 会 EOF、需手动解 gzip）。
func emHTTP11Client(hostURL string, timeout time.Duration) *resty.Client {
	host := emHostFromURL(hostURL)
	if host == "" {
		host = "push2his.eastmoney.com"
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	emHTTP11ClientsMu.Lock()
	defer emHTTP11ClientsMu.Unlock()
	key := host + "@" + timeout.String()
	if c, ok := emHTTP11Clients[key]; ok {
		return c
	}
	tr := &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSClientConfig:       &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12},
		DisableCompression:    true,
		MaxIdleConns:          20,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ForceAttemptHTTP2:   false,
	}
	client := resty.NewWithClient(&http.Client{Transport: tr, Timeout: timeout}).
		SetTimeout(timeout).
		SetRetryCount(0)
	emHTTP11Clients[key] = client
	return client
}
