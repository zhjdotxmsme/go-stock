package multi

import (
	"context"
	"fmt"
	"strings"

	"go-stock/backend/agent/memory"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// A5 接线（方案 §8.1 T2）：反思绑定。
// 取该股票最近一次多 Agent 分析结果，结合已实现收益率，
// 对每个分析师角色执行 4 步结构化反思并写入对应角色的记忆库（供后续分析检索注入）。

// ReflectOnLastAnalysis 对 stockCode 最近一次多 Agent 分析执行反思记忆。
// returnsPct 为分析后的实际收益率 %（由调用方结算得到）；LLM 使用 quick 层。
// 返回人类可读的各角色反思摘要。
func ReflectOnLastAnalysis(ctx context.Context, stockCode string, returnsPct float64, aiConfigID int) (string, error) {
	return reflectOnLastAnalysis(ctx, stockCode, returnsPct, memory.LLMCallFunc(makeTierLLMCall(aiConfigID)))
}

// reflectOnLastAnalysis 可注入 LLM 调用的实现（测试用）。
func reflectOnLastAnalysis(ctx context.Context, stockCode string, returnsPct float64, call memory.LLMCallFunc) (string, error) {
	if db.Dao == nil {
		return "", fmt.Errorf("数据库未初始化")
	}

	var saved models.AIResponseResult
	err := db.Dao.Where("stock_code = ? AND model_name = ?", stockCode, "multi-agent-7").
		Order("created_at DESC").First(&saved).Error
	if err != nil {
		return "", fmt.Errorf("未找到 %s 的多 Agent 分析记录: %w", stockCode, err)
	}

	reports := parseSavedAnalystReports(saved.Content)
	if len(reports) == 0 {
		return "", fmt.Errorf("%s 最近一次分析没有可反思的分析师报告", stockCode)
	}

	situation := truncateSummary(
		fmt.Sprintf("%s(%s) %s", saved.StockName, saved.StockCode, saved.Question), 300)
	reflector := memory.NewReflector("quick", call)

	var sb strings.Builder
	fmt.Fprintf(&sb, "反思完成（收益率 %.2f%%）：\n", returnsPct)
	succeeded := 0
	for _, rp := range reports {
		// 构造时自动 AutoMigrate 记忆表（建表路径在此）
		mem, err := memory.NewSQLiteMemory(db.Dao, rp.role)
		if err != nil {
			logger.SugaredLogger.Warnf("reflect: memory open failed for %s: %v", rp.role, err)
			continue
		}
		res, err := reflector.ReflectAndRemember(ctx, mem, memory.ReflectionInput{
			AgentRole:  rp.role,
			StockCode:  saved.StockCode,
			StockName:  saved.StockName,
			Situation:  situation,
			Decision:   truncateSummary(fmt.Sprintf("评级 %s：%s", rp.rating, rp.content), 800),
			ReturnsPct: returnsPct,
		})
		if err != nil {
			// 单角色失败不阻塞其他角色（降级原则）
			logger.SugaredLogger.Warnf("reflect failed for %s: %v", rp.role, err)
			fmt.Fprintf(&sb, "- %s：反思失败（%v）\n", rp.role, err)
			continue
		}
		succeeded++
		fmt.Fprintf(&sb, "- %s：%s\n", rp.role, truncateSummary(res.Lesson, 120))
	}
	if succeeded == 0 {
		return "", fmt.Errorf("所有角色反思均失败")
	}
	return sb.String(), nil
}

// savedAnalystReport 从落库 markdown 还原的分析师报告段。
type savedAnalystReport struct {
	role    string
	rating  string
	content string
}

// parseSavedAnalystReports 解析 saveMultiAgentResult 落库的 markdown，
// 还原各分析师角色/评级/内容（"### role (评级: x)" 段；"数据不可用" 段跳过）。
func parseSavedAnalystReports(content string) []savedAnalystReport {
	var out []savedAnalystReport
	var cur *savedAnalystReport
	var body strings.Builder
	flush := func() {
		if cur != nil {
			cur.content = strings.TrimSpace(body.String())
			if cur.content != "" {
				out = append(out, *cur)
			}
		}
		cur = nil
		body.Reset()
	}

	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ") {
			flush()
			if role, rating := parseSectionHeader(line); role != "" {
				cur = &savedAnalystReport{role: role, rating: rating}
			}
			continue
		}
		if cur != nil {
			body.WriteString(line + "\n")
		}
	}
	flush()
	return out
}

// parseSectionHeader 解析 "### role (评级: xxx)" 段头；非该格式（含"数据不可用"）返回空。
func parseSectionHeader(line string) (role, rating string) {
	if !strings.HasPrefix(line, "### ") || strings.Contains(line, "数据不可用") {
		return "", ""
	}
	h := strings.TrimPrefix(line, "### ")
	i := strings.Index(h, " (评级: ")
	if i <= 0 {
		return "", ""
	}
	role = strings.TrimSpace(h[:i])
	rating = strings.TrimSuffix(strings.TrimSpace(h[i+len(" (评级: "):]), ")")
	return role, rating
}
