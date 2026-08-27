package data

import (
	"fmt"
	"go-stock/backend/logger"
	"strings"
	"time"
)

func init() {
	registerToolHandler("GetMarketData", handleGetMarketData)
}

// handleGetMarketData 处理 GetMarketData 工具调用
func handleGetMarketData(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	snap, err := FetchMarketSnapshot()
	if err != nil {
		return fmt.Errorf("获取市场数据失败: %v", err)
	}

	ctx.Ch <- map[string]any{
		"code":              1,
		"question":          ctx.Question,
		"chatId":            ctx.StreamResponseID,
		"model":             ctx.Model,
		"reasoning_content": "\r\n```\r\n🔧 开始调用工具：GetMarketData\r\n```\r\n",
		"time":              time.Now().Format(time.DateTime),
	}

	// 构建markdown格式的输出
	content := strings.Builder{}
	content.WriteString("# 市场行情数据\r\n\r\n")

	// 1. 指数行情部分
	content.WriteString("## 指数行情\r\n\r\n")
	content.WriteString("| 指数代码 | 指数名称 | 最新价格 | 涨跌(%) | 上涨家数 | 下跌家数 | 平盘家数 |\r\n")
	content.WriteString("|----------|----------|----------|---------|----------|----------|----------|\r\n")
	for _, index := range snap.IndexQuotes {
		content.WriteString(fmt.Sprintf("| %s | %s | %.2f | %.2f | %d | %d | %d |\r\n",
			index.Code, index.Name, index.Price, index.ChangePct,
			index.UpCount, index.DownCount, index.FlatCount))
	}

	// 2. 涨跌分布部分
	dis := snap.UpDownDis
	content.WriteString("\r\n## 涨跌分布\r\n\r\n")
	content.WriteString("| 涨停家数 | 跌停家数 | 平均涨幅(%) | 上涨家数总计 | 下跌家数总计 |\r\n")
	content.WriteString("|----------|----------|-------------|--------------|--------------|\r\n")
	content.WriteString(fmt.Sprintf("| %d | %d | %.2f | %d | %d |\r\n",
		dis.LimitUp, dis.LimitDown, dis.AverageRise,
		dis.RiseCount, dis.FallCount))

	content.WriteString("\r\n### 跌幅分布\r\n\r\n")
	content.WriteString("| 跌幅8%~10% | 跌幅6%~8% | 跌幅4%~6% | 跌幅2%~4% | 跌幅0%~2% |\r\n")
	content.WriteString("|-----------|-----------|-----------|-----------|-----------|\r\n")
	content.WriteString(fmt.Sprintf("| %d | %d | %d | %d | %d |\r\n",
		dis.Down10, dis.Down8, dis.Down6, dis.Down4, dis.Down2))

	content.WriteString("\r\n### 涨幅分布\r\n\r\n")
	content.WriteString("| 涨幅0%~2% | 涨幅2%~4% | 涨幅4%~6% | 涨幅6%~8% | 涨幅8%~10% |\r\n")
	content.WriteString("|-----------|-----------|-----------|-----------|------------|\r\n")
	content.WriteString(fmt.Sprintf("| %d | %d | %d | %d | %d |\r\n",
		dis.Up2, dis.Up4, dis.Up6, dis.Up8, dis.Up10))

	content.WriteString("\r\n### 其他统计\r\n\r\n")
	content.WriteString(fmt.Sprintf("- 平盘家数: %d\r\n", dis.FlatCount))
	content.WriteString(fmt.Sprintf("- 数据来源: 东方财富\r\n"))

	// 3. 今日申购（暂不可用，东方财富数据源不提供此格式）
	content.WriteString("\r\n## 今日申购\r\n\r\n")
	content.WriteString("今日申购数据暂不可用（数据源迁移至东方财富）\r\n")

	logger.SugaredLogger.Debug("%s", content.String())

	appendToolMessages(
		ctx.Messages,
		ctx.CurrentAIContent.String(),
		ctx.ReasoningContentText.String(),
		ctx.CurrentCallID,
		ctx.FuncName,
		funcArguments,
		content.String(),
	)

	return nil
}
