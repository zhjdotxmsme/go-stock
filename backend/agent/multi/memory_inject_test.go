package multi

import (
	"context"
	"strings"
	"testing"
)

func TestMemoryInjection(t *testing.T) {
	ctx := context.Background()

	t.Run("nil上下文不注入", func(t *testing.T) {
		if got := memoryInjection(ctx, nil, "fundamental"); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("开关关闭不注入", func(t *testing.T) {
		ac := &AgentContext{
			MemoryInjectionOff: true,
			MemoryRetrieve: func(role, situation string) []string {
				t.Error("开关关闭时不应调用检索")
				return nil
			},
		}
		if got := memoryInjection(ctx, ac, "fundamental"); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("无记忆时逐字节一致", func(t *testing.T) {
		ac := &AgentContext{
			StockCode: "600000", StockName: "浦发银行", UserQuery: "分析一下",
			MemoryRetrieve: func(role, situation string) []string { return nil },
		}
		if got := memoryInjection(ctx, ac, "fundamental"); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("有记忆时注入经验段", func(t *testing.T) {
		var gotRole, gotSituation string
		ac := &AgentContext{
			StockCode: "600000", StockName: "浦发银行", UserQuery: "分析一下",
			MemoryRetrieve: func(role, situation string) []string {
				gotRole, gotSituation = role, situation
				return []string{"高估值时谨慎给出买入评级", "银行股关注息差变化"}
			},
		}
		got := memoryInjection(ctx, ac, "fundamental")
		if gotRole != "fundamental" {
			t.Errorf("role: got %q, want fundamental", gotRole)
		}
		if !strings.Contains(gotSituation, "浦发银行") || !strings.Contains(gotSituation, "600000") {
			t.Errorf("situation 应包含股票上下文: %q", gotSituation)
		}
		if !strings.Contains(got, "【历史经验】") ||
			!strings.Contains(got, "高估值时谨慎给出买入评级") ||
			!strings.Contains(got, "银行股关注息差变化") {
			t.Errorf("注入段格式不符: %q", got)
		}
	})

	t.Run("db未初始化时默认检索静默降级", func(t *testing.T) {
		// 测试环境 db.Dao 为 nil，默认 SQLite 路径应返回空而非 panic
		ac := &AgentContext{StockCode: "600000", StockName: "浦发银行"}
		if got := memoryInjection(ctx, ac, "technical"); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}
