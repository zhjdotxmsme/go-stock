package tools

import (
	"context"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/duke-git/lancet/v2/strutil"
)

// htmlTagRe 匹配 HTML 标签（如新闻接口为关键词高亮返回的 <em>）。
var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

// stripHTMLForAI 移除工具文本中的 HTML 标签。
//
// 上下游新闻/资讯接口会返回带 <em> 高亮标记的正文（供前端渲染），直接写入工具
// 结果会污染模型上下文：既成倍浪费 token，又容易让模型把标签抄进回答。模型侧
// 只需要纯文本，故在工具输出边界统一清洗。
func stripHTMLForAI(s string) string {
	if !strings.Contains(s, "<") {
		return s
	}
	return htmlTagRe.ReplaceAllString(s, "")
}

// ToolWrapper is the base wrapper for all data tools
type ToolWrapper struct {
	name        string
	description string
	params      map[string]*schema.ParameterInfo
	handler     func(args string) (string, error)
}

// NewToolWrapper creates a new ToolWrapper
func NewToolWrapper(name, description string, params map[string]*schema.ParameterInfo, handler func(args string) (string, error)) *ToolWrapper {
	return &ToolWrapper{
		name:        name,
		description: description,
		params:      params,
		handler:     handler,
	}
}

// Info returns the tool metadata
func (t *ToolWrapper) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        t.name,
		Desc:        t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(t.params),
	}, nil
}

// InvokableRun executes the tool with the given arguments
func (t *ToolWrapper) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	return t.handler(argumentsInJSON)
}

// GetStockCode normalizes stock code to internal format
// Expected input: sh600519, sz000001, hk00700, usAAPL
func GetStockCode(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}

	// Already lowercase prefix
	if strutil.HasPrefixAny(code, []string{"sh", "sz", "bj", "hk", "us", "gb_"}) {
		return code
	}

	// Try uppercase prefix
	if len(code) >= 2 {
		lower := strings.ToLower(code[:2])
		for _, p := range []string{"sh", "sz", "bj", "hk", "us"} {
			if lower == p {
				return p + code[2:]
			}
		}
	}

	return code
}
