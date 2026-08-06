package multi

import (
	"context"
	"strings"
	"testing"

	"go-stock/backend/agent/memory"
	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestParseSavedAnalystReports(t *testing.T) {
	content := `## 多智能体分析报告 - 浦发银行(600000)

提问: 分析一下

### fundamental (评级: bullish)
基本面良好，估值偏低。

### technical (评级: bearish)
技术面走弱。

### news - 数据不可用

## 多空辩论

第1轮 看多: ...
第1轮 看空: ...

## 最终评级: hold
结论文本
`
	reports := parseSavedAnalystReports(content)
	if len(reports) != 2 {
		t.Fatalf("got %d reports, want 2: %+v", len(reports), reports)
	}
	if reports[0].role != "fundamental" || reports[0].rating != "bullish" ||
		!strings.Contains(reports[0].content, "基本面良好") {
		t.Errorf("fundamental 段解析错误: %+v", reports[0])
	}
	if reports[1].role != "technical" || reports[1].rating != "bearish" {
		t.Errorf("technical 段解析错误: %+v", reports[1])
	}
}

func TestReflectOnLastAnalysis(t *testing.T) {
	ctx := context.Background()

	t.Run("db未初始化返回错误", func(t *testing.T) {
		if _, err := reflectOnLastAnalysis(ctx, "600000", 5.0, nil); err == nil {
			t.Error("db.Dao nil 时应返回错误")
		}
	})

	// 内存 SQLite 替换全局 db.Dao（测试结束恢复）
	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := gormDB.AutoMigrate(&models.AIResponseResult{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	oldDao := db.Dao
	db.Dao = gormDB
	defer func() { db.Dao = oldDao }()

	t.Run("无分析记录返回错误", func(t *testing.T) {
		if _, err := reflectOnLastAnalysis(ctx, "999999", 5.0, nil); err == nil {
			t.Error("无记录时应返回错误")
		}
	})

	saved := models.AIResponseResult{
		StockCode: "600000", StockName: "浦发银行", ModelName: "multi-agent-7",
		Question: "分析一下",
		Content: "### fundamental (评级: bullish)\n基本面良好。\n\n### technical (评级: bearish)\n技术面走弱。\n",
	}
	if err := gormDB.Create(&saved).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	// mock LLM：4 步反思统一返回固定文本
	mockCall := memory.LLMCallFunc(func(ctx context.Context, model, prompt string) (string, error) {
		if model != "quick" {
			t.Errorf("model: got %q, want quick", model)
		}
		return "反思输出文本", nil
	})

	t.Run("逐角色反思并写入记忆库", func(t *testing.T) {
		summary, err := reflectOnLastAnalysis(ctx, "600000", 5.0, mockCall)
		if err != nil {
			t.Fatalf("reflect: %v", err)
		}
		if !strings.Contains(summary, "fundamental") || !strings.Contains(summary, "technical") {
			t.Errorf("摘要应覆盖两个角色: %q", summary)
		}

		// 每个角色记忆库各写入 1 条
		for _, role := range []string{"fundamental", "technical"} {
			mem, err := memory.NewSQLiteMemory(gormDB, role)
			if err != nil {
				t.Fatalf("open memory %s: %v", role, err)
			}
			count, err := mem.Count(ctx)
			if err != nil {
				t.Fatalf("count %s: %v", role, err)
			}
			if count != 1 {
				t.Errorf("role %s: got %d memories, want 1", role, count)
			}
		}
	})

	t.Run("LLM失败时返回错误", func(t *testing.T) {
		failCall := memory.LLMCallFunc(func(ctx context.Context, model, prompt string) (string, error) {
			return "", context.DeadlineExceeded
		})
		if _, err := reflectOnLastAnalysis(ctx, "600000", 5.0, failCall); err == nil {
			t.Error("全部反思失败时应返回错误")
		}
	})
}
