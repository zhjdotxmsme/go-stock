package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"go-stock/backend/db"
	domainsystem "go-stock/backend/internal/domain/system"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// newTestRepo 用临时 SQLite 库构造仓储，并接管 db.Dao。
// 返回清理函数，测试结束后恢复原 db.Dao。
func newTestRepo(t *testing.T) *SystemRepository {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session_test.db")
	conn, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open temp db: %v", err)
	}
	if err := conn.AutoMigrate(&domainsystem.AiAssistantSession{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	prev := db.Dao
	db.Dao = conn
	t.Cleanup(func() {
		db.Dao = prev
		// Windows 下不关闭连接会阻塞 t.TempDir 的 RemoveAll（文件被占用）
		if sqlDB, err := conn.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return NewSystemRepository()
}

// 回归：steps / jsonMarkdown 曾因域模型缺字段而被丢弃，恢复会话后「执行步骤」「分析报告」消失。
func TestAiAssistantSessionStepsAndJsonMarkdownRoundTrip(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	msgs := []domainsystem.AiAssistantMessage{
		{Role: "user", Content: "分析贵州茅台", Time: "2026-09-10 11:00:00"},
		{
			Role:         "assistant",
			Content:      "# 结论\n现价 1580 元",
			Reasoning:    "先取行情再看财务",
			Time:         "2026-09-10 11:00:05",
			ModelName:    "deepseek-chat",
			Steps:        []string{"🔄 获取行情", "🔧 调用 get_stock_info", "✅ 完成"},
			JsonMarkdown: "## 分析报告\n- PE 28\n- 现金流稳健",
		},
	}
	if err := repo.SaveAiAssistantSession(ctx, "sess-round", msgs); err != nil {
		t.Fatalf("save: %v", err)
	}

	resp, err := repo.GetAiAssistantSession(ctx, "sess-round")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(resp.Messages) != 2 {
		t.Fatalf("messages=%d want 2", len(resp.Messages))
	}
	got := resp.Messages[1]
	if len(got.Steps) != 3 || got.Steps[1] != "🔧 调用 get_stock_info" {
		t.Errorf("steps lost: %#v", got.Steps)
	}
	if got.JsonMarkdown != msgs[1].JsonMarkdown {
		t.Errorf("jsonMarkdown lost: %q", got.JsonMarkdown)
	}
	if got.Reasoning != "先取行情再看财务" || got.ModelName != "deepseek-chat" {
		t.Errorf("other fields lost: %+v", got)
	}
}

func TestAiAssistantSessionTitleDerivation(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	// 首次保存：标题取首条 user 消息
	first := []domainsystem.AiAssistantMessage{
		{Role: "assistant", Content: "欢迎语"},
		{Role: "user", Content: "帮我看看宁德时代最近的走势怎么样"},
	}
	if err := repo.SaveAiAssistantSession(ctx, "sess-title", first); err != nil {
		t.Fatalf("save: %v", err)
	}
	// 二次保存：标题不应被覆盖
	second := append(first, domainsystem.AiAssistantMessage{Role: "assistant", Content: "好的"})
	if err := repo.SaveAiAssistantSession(ctx, "sess-title", second); err != nil {
		t.Fatalf("save2: %v", err)
	}

	list, err := repo.ListAiAssistantSessions(ctx, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list len=%d want 1", len(list))
	}
	want := "帮我看看宁德时代最近的走势怎么样" // 16 个 rune，未超 30 不截断
	if list[0].Title != want {
		t.Errorf("title=%q want %q", list[0].Title, want)
	}
	if list[0].MessageCount != 3 {
		t.Errorf("count=%d want 3", list[0].MessageCount)
	}
}

func TestAiAssistantSessionTitleTruncatesByRune(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	long := "一二三四五六七八九十一二三四五六七八九十一二三四五六七八九十一二三四五" // 35 runes
	if err := repo.SaveAiAssistantSession(ctx, "sess-long", []domainsystem.AiAssistantMessage{
		{Role: "user", Content: long},
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	list, _ := repo.ListAiAssistantSessions(ctx, 0)
	if len(list) != 1 {
		t.Fatalf("list len=%d", len(list))
	}
	runes := []rune(list[0].Title)
	// 30 个字符 + 省略号
	if len(runes) != maxSessionTitleRunes+1 {
		t.Errorf("title runes=%d want %d (%q)", len(runes), maxSessionTitleRunes+1, list[0].Title)
	}
	if runes[len(runes)-1] != '…' {
		t.Errorf("title should end with ellipsis: %q", list[0].Title)
	}
}

func TestAiAssistantSessionTitleFallbackWithoutUserMessage(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	if err := repo.SaveAiAssistantSession(ctx, "sess-greet", []domainsystem.AiAssistantMessage{
		{Role: "assistant", Content: "我是您的 AI 助手"},
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	list, _ := repo.ListAiAssistantSessions(ctx, 0)
	if len(list) != 1 || list[0].Title != "新对话" {
		t.Errorf("title=%q want 新对话", list[0].Title)
	}
}

func TestAiAssistantSessionListOrderAndDelete(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	base := time.Now().Add(-time.Hour)
	for i, id := range []string{"s-old", "s-mid", "s-new"} {
		if err := repo.SaveAiAssistantSession(ctx, id, []domainsystem.AiAssistantMessage{
			{Role: "user", Content: id},
		}); err != nil {
			t.Fatalf("save %s: %v", id, err)
		}
		// 手动拉开 updated_at，避免同秒保存导致排序不稳定
		if err := db.Dao.Model(&domainsystem.AiAssistantSession{}).
			Where("session_id = ?", id).
			Update("updated_at", base.Add(time.Duration(i)*time.Minute)).Error; err != nil {
			t.Fatalf("retime %s: %v", id, err)
		}
	}

	list, err := repo.ListAiAssistantSessions(ctx, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("list len=%d want 3", len(list))
	}
	if list[0].SessionId != "s-new" || list[2].SessionId != "s-old" {
		t.Errorf("order wrong: %s %s %s", list[0].SessionId, list[1].SessionId, list[2].SessionId)
	}

	// limit 生效
	limited, _ := repo.ListAiAssistantSessions(ctx, 2)
	if len(limited) != 2 {
		t.Errorf("limit ignored: len=%d", len(limited))
	}

	// 删除后不再出现
	if err := repo.DeleteAiAssistantSession(ctx, "s-mid"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	after, _ := repo.ListAiAssistantSessions(ctx, 0)
	if len(after) != 2 {
		t.Fatalf("after delete len=%d want 2", len(after))
	}
	for _, s := range after {
		if s.SessionId == "s-mid" {
			t.Errorf("deleted session still listed")
		}
	}

	// 幂等：删除不存在的会话不报错
	if err := repo.DeleteAiAssistantSession(ctx, "nope"); err != nil {
		t.Errorf("delete missing should be no-op, got %v", err)
	}
	// 空 sessionId 直接 no-op（避免误删整表）
	if err := repo.DeleteAiAssistantSession(ctx, "  "); err != nil {
		t.Errorf("delete blank should be no-op, got %v", err)
	}
	stillThere, _ := repo.ListAiAssistantSessions(ctx, 0)
	if len(stillThere) != 2 {
		t.Errorf("blank delete wiped rows: len=%d", len(stillThere))
	}
}

func TestAiAssistantSessionEmptySaveIsNoop(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	if err := repo.SaveAiAssistantSession(ctx, "sess-empty", nil); err != nil {
		t.Fatalf("save nil: %v", err)
	}
	list, _ := repo.ListAiAssistantSessions(ctx, 0)
	if len(list) != 0 {
		t.Errorf("nil save created a row: %+v", list)
	}
}

// 兼容 title 列引入之前的旧会话：列表读时派生标题，不显示空白。
func TestAiAssistantSessionListDerivesTitleForLegacyRows(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	payload := `[{"role":"user","content":"旧的会话没有标题列"},{"role":"assistant","content":"好"}]`
	if err := db.Dao.Create(&domainsystem.AiAssistantSession{
		SessionId: "legacy-1",
		Title:     "",
		Messages:  payload,
	}).Error; err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}

	list, err := repo.ListAiAssistantSessions(ctx, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len=%d want 1", len(list))
	}
	if list[0].Title != "旧的会话没有标题列" {
		t.Errorf("title=%q want derived from first user message", list[0].Title)
	}
	if list[0].MessageCount != 2 {
		t.Errorf("count=%d want 2", list[0].MessageCount)
	}
}
