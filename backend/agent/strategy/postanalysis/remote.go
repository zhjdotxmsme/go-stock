package postanalysis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RemoteAnalyzer 通用外部 HTTP 分析器（方案 §8.1 D10 的"外部 HTTP / 远程 DSA 服务"：
// POST 候选 JSON → 接收分数调整）。http.Client 注入便于测试；超时可配。
type RemoteAnalyzer struct {
	AnalyzerName string        // 分析器名（链中标识）
	Endpoint     string        // POST 地址
	Client       *http.Client  // 注入的 HTTP 客户端（nil 时按 Timeout 新建）
	Timeout      time.Duration // 默认 10s（Client 为 nil 时生效）
}

// remoteRequest 远程分析请求体。
type remoteRequest struct {
	Candidates []CandidateInput `json:"candidates"`
}

// remoteResponse 远程分析响应体。
type remoteResponse struct {
	Deltas []struct {
		Code   string  `json:"code"`
		Delta  float64 `json:"delta"`
		Detail string  `json:"detail"`
	} `json:"deltas"`
}

// NewRemoteAnalyzer 构造远程分析器。
func NewRemoteAnalyzer(name, endpoint string, timeout time.Duration, client *http.Client) *RemoteAnalyzer {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &RemoteAnalyzer{AnalyzerName: name, Endpoint: endpoint, Timeout: timeout, Client: client}
}

func (r *RemoteAnalyzer) Name() string { return r.AnalyzerName }

// Analyze POST 候选列表并解析分数调整；网络/协议/解析错误整体返回 error（由链标记失败并继续）。
func (r *RemoteAnalyzer) Analyze(ctx context.Context, candidates []CandidateInput) ([]AnalyzerOutcome, error) {
	body, err := json.Marshal(remoteRequest{Candidates: candidates})
	if err != nil {
		return nil, fmt.Errorf("请求序列化失败: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("请求构造失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := r.Client
	if client == nil {
		client = &http.Client{Timeout: r.Timeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("远程分析请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("远程分析返回非 200: %d", resp.StatusCode)
	}
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	var parsed remoteResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("响应解析失败: %w", err)
	}

	byCode := make(map[string]AnalyzerOutcome, len(parsed.Deltas))
	for _, d := range parsed.Deltas {
		byCode[d.Code] = AnalyzerOutcome{Delta: d.Delta, Detail: d.Detail}
	}
	outcomes := make([]AnalyzerOutcome, len(candidates))
	for i, cand := range candidates {
		if outcome, ok := byCode[cand.Code]; ok {
			outcomes[i] = outcome
		}
		// 远程未返回的候选：delta 0（不调整），不视为失败
	}
	return outcomes, nil
}
