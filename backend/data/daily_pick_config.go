package data

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/logger"
	"go-stock/backend/models"

	"github.com/go-resty/resty/v2"
)

// CallLLMForConfig 调用 LLM 生成 StrategyConfig。
// 使用当前设置中第一个启用的 AI 配置，走 resty HTTP 调用（避免跨包循环引用）。
func CallLLMForConfig(query string) (*models.StrategyConfig, error) {
	config := GetSettingConfig()
	if config == nil || len(config.AiConfigs) == 0 {
		return nil, fmt.Errorf("no AI config available")
	}

	aiConfig := config.AiConfigs[0]

	prompt := buildStrategyConfigPrompt(query)

	timeout := time.Duration(aiConfig.TimeOut) * time.Second
	if timeout <= 0 || timeout > 30*time.Second {
		timeout = 15 * time.Second
	}

	client := resty.New().SetTimeout(timeout)
	if aiConfig.HttpProxyEnabled && aiConfig.HttpProxy != "" {
		client.SetProxy(aiConfig.HttpProxy)
	}

	baseURL := strings.TrimRight(aiConfig.BaseUrl, "/")
	apiURL := baseURL + "/chat/completions"

	messages := []map[string]string{
		{"role": "system", "content": "You are a stock strategy configuration expert. Output ONLY valid JSON matching the specified schema. No explanation, no markdown."},
		{"role": "user", "content": prompt},
	}

	body := map[string]any{
		"model":       aiConfig.ModelName,
		"messages":    messages,
		"temperature": 0.1,
		"max_tokens":  1024,
	}

	// 部分模型支持 response_format=json_object
	if supportsJSONMode(aiConfig.ModelName) {
		body["response_format"] = map[string]string{"type": "json_object"}
	}

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", "Bearer "+aiConfig.ApiKey).
		SetBody(body).
		Post(apiURL)

	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("LLM returned status %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(resp.Body(), &chatResp); err != nil {
		return nil, fmt.Errorf("parse LLM response: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("LLM returned empty choices")
	}

	content := chatResp.Choices[0].Message.Content
	content = cleanJSON(content)

	var strategyConfig models.StrategyConfig
	if err := json.Unmarshal([]byte(content), &strategyConfig); err != nil {
		logger.SugaredLogger.Warnf("callLLMForConfig: failed to parse JSON from LLM, raw: %s, err: %v", content, err)
		return nil, fmt.Errorf("parse strategy config: %w", err)
	}

	return &strategyConfig, nil
}

// supportsJSONMode 判断模型是否支持 response_format=json_object。
func supportsJSONMode(modelName string) bool {
	lower := strings.ToLower(modelName)
	return strings.Contains(lower, "gpt") ||
		strings.Contains(lower, "deepseek") ||
		strings.Contains(lower, "qwen") ||
		strings.Contains(lower, "glm")
}

// cleanJSON 去除 LLM 返回中可能的 markdown 代码块包裹。
func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
	}
	return strings.TrimSpace(s)
}

// buildStrategyConfigPrompt 构建包含策略注册表的 LLM prompt。
func buildStrategyConfigPrompt(query string) string {
	strategyDescs := []struct {
		Code string
		Name string
		Desc string
	}{
		{"ma_trend", "均线趋势", "基于MA5/MA10/MA20多头排列的顺势跟踪策略，可调参数: ma_fast(默认5), ma_slow(默认20)"},
		{"oversold_reversal", "超买超卖逆转", "识别RSI/WR/CCI超卖区域的反转信号，可调参数: rsi_period(默认14)"},
		{"momentum", "短线动量", "基于MACD金叉死叉和OBV能量潮的动量跟踪，可调参数: volume_min_ratio(默认1.0)"},
		{"channel_breakout", "通道突破", "基于BOLL通道突破和ATR波动率确认，可调参数: boll_period(默认20)"},
		{"kdj_short", "KDJ短线", "基于KDJ和W%R的超短线交易信号，可调参数: kdj_k_period(默认9)"},
		{"industry_strength", "行业强度", "基于行业资金流向排名的行业强度评分"},
		{"research_report", "研报热度", "基于近期机构研报数量的评分"},
		{"macro_environment", "宏观环境", "基于PMI/CPI/GDP宏观数据的评分"},
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("用户选股需求：%s\n\n可用策略列表（选择最适合的策略组合，最多选5个）：\n", query))
	for _, s := range strategyDescs {
		sb.WriteString(fmt.Sprintf("- %s (%s): %s\n", s.Code, s.Name, s.Desc))
	}

	sb.WriteString(`
可覆盖参数说明（选填）：
- rsi_period: RSI计算周期，默认14，范围5-30
- ma_fast: 快线周期，默认5，范围3-20
- ma_slow: 慢线周期，默认20，范围10-60
- boll_period: BOLL周期，默认20，范围10-50
- kdj_k_period: KDJ K值周期，默认9，范围5-21
- volume_min_ratio: 最小量比，默认1.0，范围0.5-5.0

后置过滤字段说明（选填）：
- score: 综合评分(0-100)
- price: 收盘价
- volume: 成交量
- rsi14: RSI值

请返回严格 JSON 格式，不要多余文字：
{
  "enabled_strategies": ["strategy_code1", "strategy_code2"],
  "strategy_weights": {"code": 0.6},
  "strategy_params": {"param_name": value},
  "filters": [{"field": "rsi14", "op": "<", "value": 70}],
  "top_n": 10
}`)

	return sb.String()
}

// createStockAiConfig 获取可用的 AI 配置（内部辅助）。
func createStockAiConfig() *AIConfig {
	config := GetSettingConfig()
	if config == nil || len(config.AiConfigs) == 0 {
		return nil
	}
	return config.AiConfigs[0]
}
