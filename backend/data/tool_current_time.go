package data

import "time"

func init() {
	// A1 step 2 参考迁移：schema 与 executor 一体注册。
	// 其余 tool_*.go 仍用 registerToolHandler + tools_*.go 组装 schema，
	// 由 Tools() 的装配门禁兜底一致性。
	registerToolDefinition(ToolDefinition{
		Schema: Tool{
			Type: "function",
			Function: ToolFunction{
				Name:        "GetCurrentTime",
				Description: "获取当前本地时间（格式：YYYY-MM-DD HH:mm:ss）及星期几",
			},
		},
		Handler: handleGetCurrentTime,
	})
}

func handleGetCurrentTime(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	now := time.Now()
	weekday := weekdayCN(now.Weekday())
	content := "当前本地时间是：" + now.Format("2006-01-02 15:04:05") + " " + weekday

	ctx.Ch <- map[string]any{
		"code":              1,
		"question":          ctx.Question,
		"chatId":            ctx.StreamResponseID,
		"model":             ctx.Model,
		"reasoning_content": "\r\n```\r\n🔧 开始调用工具：GetCurrentTime\r\n```\r\n",
		"time":              now.Format(time.DateTime),
	}

	appendToolMessages(
		ctx.Messages,
		ctx.CurrentAIContent.String(),
		ctx.ReasoningContentText.String(),
		ctx.CurrentCallID,
		ctx.FuncName,
		funcArguments,
		content,
	)

	return nil
}

func WeekdayCN(wd time.Weekday) string {
	names := []string{"星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六"}
	return names[wd]
}

func weekdayCN(wd time.Weekday) string {
	return WeekdayCN(wd)
}
