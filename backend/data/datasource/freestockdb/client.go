// Package freestockdb 实现 free-stockdb 本地行情引擎的接入。
// 引擎是 HTTP K-V 服务：GET http://<addr>/?cmd=get&t=<键表达式>。
package freestockdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client 是 free-stockdb HTTP K-V 服务的客户端。
type Client struct {
	addr string
	hc   *http.Client
}

// NewClient 创建客户端，addr 形如 "127.0.0.1:7899"。
func NewClient(addr string) *Client {
	return &Client{addr: addr, hc: &http.Client{Timeout: 10 * time.Second}}
}

// Get 执行 K-V 查询。expr 例如 "日k:600633:20260620>20260626"。
// 返回原始 JSON：点查为 object，范围/通配为 array（可能是 [key,value] 对）。
func (c *Client) Get(ctx context.Context, expr string) (json.RawMessage, error) {
	u := fmt.Sprintf("http://%s/?cmd=get&t=%s", c.addr, url.QueryEscape(expr))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stockdb: HTTP %d for %q", resp.StatusCode, expr)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	return json.RawMessage(body), nil
}

// Ping 探测服务是否可用（2s 超时）。
func (c *Client) Ping(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_, err := c.Get(ctx, "股票代码")
	return err == nil
}

// decodeValues 把 K-V 响应统一拆成值列表：
// 点查 object → [object]；[object,...] → 原样；[[key,value],...] → 取每对的 value。
func decodeValues(raw json.RawMessage) ([]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	switch trimmed[0] {
	case '{':
		return []json.RawMessage{json.RawMessage(trimmed)}, nil
	case '[':
		var items []json.RawMessage
		if err := json.Unmarshal(trimmed, &items); err != nil {
			return nil, fmt.Errorf("stockdb: decode array: %w", err)
		}
		out := make([]json.RawMessage, 0, len(items))
		for _, it := range items {
			it = bytes.TrimSpace(it)
			if len(it) == 0 {
				continue
			}
			if it[0] == '[' {
				var pair []json.RawMessage
				if err := json.Unmarshal(it, &pair); err == nil && len(pair) == 2 {
					out = append(out, pair[1])
					continue
				}
			}
			out = append(out, it)
		}
		return out, nil
	}
	return nil, fmt.Errorf("stockdb: unexpected payload: %.64s", trimmed)
}
