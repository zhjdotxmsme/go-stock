package memory

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"gorm.io/gorm"
)

// memoryRow 记忆表 GORM 模型。
type memoryRow struct {
	ID            uint      `gorm:"primarykey"`
	AgentRole     string    `gorm:"index:idx_mem_role;size:50"`
	SituationHash string    `gorm:"size:64;index"`
	SituationText string    `gorm:"type:text"`
	LessonText    string    `gorm:"type:text"`
	ReturnsPct    float64   `gorm:"index:idx_mem_role_returns,priority:2"`
	CreatedAt     time.Time `gorm:"index:idx_mem_role_returns,priority:3"`
}

func (memoryRow) TableName() string { return "agent_reflection_memories" }

// ftsTable FTS5 虚表名（contentless 外部内容表：rowid 与 memoryRow.ID 对应）。
const ftsTable = "agent_reflection_memories_fts"

// SQLiteMemory SQLite 实现的 AgentMemory。
// 检索优先走 FTS5 全文索引（构造时探测可用性）；FTS5 不可用或查询失败时
// 降级为 LIKE 分词匹配（接口不变，实现细节对调用方隐藏）。
type SQLiteMemory struct {
	db   *gorm.DB
	role string
	fts  bool
}

// NewSQLiteMemory 构造指定角色的 SQLite 记忆库（自动建表）。
// db 由调用方注入（生产为业务库，测试可用 SQLite 内存库）；不依赖 db.Dao 全局。
func NewSQLiteMemory(db *gorm.DB, agentRole string) (*SQLiteMemory, error) {
	if err := db.AutoMigrate(&memoryRow{}); err != nil {
		return nil, fmt.Errorf("记忆表迁移失败: %w", err)
	}
	m := &SQLiteMemory{db: db, role: agentRole}
	m.fts = m.probeFTS5()
	return m, nil
}

// probeFTS5 探测 FTS5 可用性：尝试创建虚表并写入/删除一行。
func (m *SQLiteMemory) probeFTS5() bool {
	create := fmt.Sprintf(`CREATE VIRTUAL TABLE IF NOT EXISTS %s USING fts5(situation_text, lesson_text)`, ftsTable)
	if err := m.db.Exec(create).Error; err != nil {
		return false
	}
	// 部分构建允许建表但禁用写入，实测一次写删
	if err := m.db.Exec(fmt.Sprintf(`INSERT INTO %s(rowid, situation_text, lesson_text) VALUES (-1, 'probe', 'probe')`, ftsTable)).Error; err != nil {
		return false
	}
	m.db.Exec(fmt.Sprintf(`DELETE FROM %s WHERE rowid = -1`, ftsTable))
	return true
}

// FTSAvailable 返回当前是否启用 FTS5 检索（测试与诊断用）。
func (m *SQLiteMemory) FTSAvailable() bool { return m.fts }

// Save 保存一条记忆；同步维护 FTS 索引（可用时）。
func (m *SQLiteMemory) Save(ctx context.Context, record MemoryRecord) error {
	row := memoryRow{
		AgentRole:     m.role,
		SituationHash: record.SituationHash,
		SituationText: record.SituationText,
		LessonText:    record.LessonText,
		ReturnsPct:    record.ReturnsPct,
		CreatedAt:     record.CreatedAt,
	}
	if row.CreatedAt.IsZero() {
		row.CreatedAt = time.Now()
	}
	if row.SituationHash == "" {
		row.SituationHash = HashSituation(row.SituationText)
	}
	if err := m.db.WithContext(ctx).Create(&row).Error; err != nil {
		return fmt.Errorf("保存记忆失败: %w", err)
	}
	if m.fts {
		// FTS 同步失败不影响主存储（检索会自动降级 LIKE）
		m.db.WithContext(ctx).Exec(
			fmt.Sprintf(`INSERT INTO %s(rowid, situation_text, lesson_text) VALUES (?, ?, ?)`, ftsTable),
			row.ID, row.SituationText, row.LessonText)
	}
	return nil
}

// Retrieve 按情境文本检索 topN 条记忆，收益率倒序（方案：FTS5 MATCH + returns_pct 排序）。
func (m *SQLiteMemory) Retrieve(ctx context.Context, situation string, topN int) ([]MemoryRecord, error) {
	if topN <= 0 {
		topN = 2
	}
	tokens := tokenize(situation)
	if m.fts && len(tokens) > 0 {
		rows, err := m.retrieveFTS(ctx, tokens, topN)
		if err == nil {
			return rows, nil
		}
		// FTS 查询失败降级 LIKE
	}
	return m.retrieveLIKE(ctx, tokens, topN)
}

// retrieveFTS FTS5 全文检索：分词 OR 匹配，收益率倒序。
func (m *SQLiteMemory) retrieveFTS(ctx context.Context, tokens []string, topN int) ([]MemoryRecord, error) {
	quoted := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		quoted = append(quoted, `"`+strings.ReplaceAll(tok, `"`, `""`)+`"`)
	}
	query := strings.Join(quoted, " OR ")
	var rows []memoryRow
	err := m.db.WithContext(ctx).
		Raw(fmt.Sprintf(`SELECT m.* FROM %s f JOIN %s m ON m.id = f.rowid
			WHERE %s MATCH ? AND m.agent_role = ?
			ORDER BY m.returns_pct DESC, m.created_at DESC LIMIT ?`, ftsTable, memoryRow{}.TableName(), ftsTable),
			query, m.role, topN).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return toRecords(rows), nil
}

// retrieveLIKE LIKE 降级检索：分词 OR 模糊匹配（覆盖情境与经验文本），收益率倒序。
func (m *SQLiteMemory) retrieveLIKE(ctx context.Context, tokens []string, topN int) ([]MemoryRecord, error) {
	tx := m.db.WithContext(ctx).Model(&memoryRow{}).Where("agent_role = ?", m.role)
	if len(tokens) > 0 {
		var conds []string
		var args []any
		for _, tok := range tokens {
			conds = append(conds, "situation_text LIKE ? OR lesson_text LIKE ?")
			like := "%" + tok + "%"
			args = append(args, like, like)
		}
		tx = tx.Where(strings.Join(conds, " OR "), args...)
	}
	var rows []memoryRow
	if err := tx.Order("returns_pct DESC, created_at DESC").Limit(topN).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("检索记忆失败: %w", err)
	}
	return toRecords(rows), nil
}

// Count 返回当前角色的记忆条数。
func (m *SQLiteMemory) Count(ctx context.Context) (int64, error) {
	var n int64
	err := m.db.WithContext(ctx).Model(&memoryRow{}).Where("agent_role = ?", m.role).Count(&n).Error
	return n, err
}

// tokenize 按空白与常见标点切分检索词，过滤单字符之外的空 token。
func tokenize(s string) []string {
	tokens := strings.FieldsFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(",，、。.;；:：|/\\()（）[]【】\"'\"", r)
	})
	out := tokens[:0]
	for _, tok := range tokens {
		if tok != "" {
			out = append(out, tok)
		}
	}
	return out
}

// toRecords 行模型转接口模型。
func toRecords(rows []memoryRow) []MemoryRecord {
	records := make([]MemoryRecord, len(rows))
	for i, r := range rows {
		records[i] = MemoryRecord(r)
	}
	return records
}
