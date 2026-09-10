package db

import (
	"path/filepath"
	"testing"

	"go-stock/backend/models"
)

// 回归：ai_assistant_sessions 历史上不在任何 AutoMigrate 列表里，
// 全新安装会缺表 → 会话保存静默失败（前端 .catch 吞掉）。
// 这里用全新空库跑一次 Init，断言表与关键列都被建出来。
func TestAutoMigrateCreatesAiAssistantSessionTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "migrate_test.db")
	Init(path)

	if Dao == nil {
		t.Fatal("db.Dao is nil after Init")
	}
	t.Cleanup(func() {
		if sqlDB, err := Dao.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	var tables []string
	if err := Dao.Raw("select name from sqlite_master where type='table' and name = 'ai_assistant_sessions'").Scan(&tables).Error; err != nil {
		t.Fatalf("query tables: %v", err)
	}
	if len(tables) != 1 {
		t.Fatalf("ai_assistant_sessions not created, got %v", tables)
	}

	type col struct {
		Name string
		Type string
	}
	var cols []col
	if err := Dao.Raw("pragma table_info(ai_assistant_sessions)").Scan(&cols).Error; err != nil {
		t.Fatalf("pragma: %v", err)
	}
	names := map[string]bool{}
	for _, c := range cols {
		names[c.Name] = true
	}
	for _, want := range []string{"id", "session_id", "title", "created_at", "updated_at", "messages"} {
		if !names[want] {
			t.Errorf("column %q missing; got %v", want, names)
		}
	}

	// 建表后写入应当成功（旧行为：缺表时 Create 报错且被前端吞掉）
	if err := Dao.Create(&models.AiAssistantSession{
		SessionId: "s1",
		Title:     "回归用例",
		Messages:  "[]",
	}).Error; err != nil {
		t.Fatalf("insert into ai_assistant_sessions failed: %v", err)
	}
}
