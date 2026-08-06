// Package memory 实现反思记忆系统（方案 §8.1 T2）。
// 每个 Agent 角色拥有独立的反思记忆库：分析结束且实际收益已知后，
// Reflector 执行 4 步结构化反思（推理评估→改进建议→经验总结→浓缩查询），
// 经验存入记忆库；未来分析前按情境检索 top-N 条注入 Prompt。
// 存储实现注入 *gorm.DB，不依赖 db.Dao 全局；不接入调用链（触发时机属后续任务）。
package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// MemoryRecord 一条反思记忆（方案存储结构：
// agent_role / situation_hash / situation_text / lesson_text / returns_pct / created_at）。
type MemoryRecord struct {
	ID            uint      `json:"id"`
	AgentRole     string    `json:"agentRole"`     // Agent 角色（记忆库按角色隔离）
	SituationHash string    `json:"situationHash"` // 情境文本 SHA256
	SituationText string    `json:"situationText"` // 当时的市场/决策情境
	LessonText    string    `json:"lessonText"`    // 反思经验（浓缩查询句）
	ReturnsPct    float64   `json:"returnsPct"`    // 实际收益率 %
	CreatedAt     time.Time `json:"createdAt"`
}

// AgentMemory 单个 Agent 角色的独立反思记忆库。
// 实现在构造时绑定角色，接口方法不再传角色——角色隔离在接口层面体现。
type AgentMemory interface {
	// Save 保存一条记忆（AgentRole 字段忽略，以绑定的角色为准）。
	Save(ctx context.Context, record MemoryRecord) error
	// Retrieve 按情境文本检索最相关的 topN 条记忆，按收益率倒序返回（注入 Prompt 用）。
	Retrieve(ctx context.Context, situation string, topN int) ([]MemoryRecord, error)
	// Count 返回当前角色的记忆条数。
	Count(ctx context.Context) (int64, error)
}

// HashSituation 计算情境文本的 SHA256（hex），用于记忆去重与关联。
func HashSituation(situation string) string {
	sum := sha256.Sum256([]byte(situation))
	return hex.EncodeToString(sum[:])
}
