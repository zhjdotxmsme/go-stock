package multi

import (
	"context"
	"fmt"

	"go-stock/backend/agent/memory"
	"go-stock/backend/db"
	"go-stock/backend/logger"
)

// A5 增强（方案 §8.1 T2）：反思记忆检索注入分析师 Prompt。
// 任一降级条件命中都返回 ""（Prompt 与历史版本逐字节一致）：
// 开关关闭 / db.Dao 未初始化 / 建表或检索失败 / 该角色暂无记忆。

// 记忆检索 top-N（注入 Prompt 条数）。
const memoryInjectTopN = 2

// memoryInjection 检索该角色的历史反思经验，拼成 Prompt 注入段。
func memoryInjection(ctx context.Context, ac *AgentContext, role string) string {
	if ac == nil || ac.MemoryInjectionOff {
		return ""
	}

	situation := truncateSummary(
		fmt.Sprintf("%s(%s) %s", ac.StockName, ac.StockCode, ac.UserQuery), 300)

	// 测试/自定义注入优先；否则走默认 SQLite 记忆库
	var lessons []string
	if ac.MemoryRetrieve != nil {
		lessons = ac.MemoryRetrieve(role, situation)
	} else {
		lessons = retrieveMemoryLessons(ctx, role, situation)
	}
	if len(lessons) == 0 {
		return ""
	}

	inj := "\n\n【历史经验】以下是你过往分析的反思经验，仅供参考（非当前数据，勿当作事实引用）：\n"
	for i, l := range lessons {
		inj += fmt.Sprintf("%d. %s\n", i+1, l)
	}
	return inj
}

// retrieveMemoryLessons 默认检索实现：按角色打开 SQLite 记忆库并检索 top-N。
// 构造时自动 AutoMigrate（建表路径在此）；任何失败静默降级为空。
func retrieveMemoryLessons(ctx context.Context, role, situation string) []string {
	if db.Dao == nil {
		return nil
	}
	mem, err := memory.NewSQLiteMemory(db.Dao, role)
	if err != nil {
		logger.SugaredLogger.Warnf("memory open failed for role %s: %v", role, err)
		return nil
	}
	recs, err := mem.Retrieve(ctx, situation, memoryInjectTopN)
	if err != nil {
		logger.SugaredLogger.Warnf("memory retrieve failed for role %s: %v", role, err)
		return nil
	}
	lessons := make([]string, 0, len(recs))
	for _, r := range recs {
		if r.LessonText != "" {
			lessons = append(lessons, r.LessonText)
		}
	}
	return lessons
}
