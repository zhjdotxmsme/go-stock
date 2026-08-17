package multi

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunPanicDoesNotCrash(t *testing.T) {
	e := NewMultiAgentEngine(0)
	ch := e.Run(context.Background(), "600000", "测试", "sh", "价格", "")
	count := 0
	for range ch {
		count++
	}
	if count == 0 {
		t.Error("expected at least some messages on channel")
	}
}

// TestRunRecoversFromPanic 验证 TestPanicHook 触发 panic 时 recover 生效：
// 进程不崩，channel 正常关闭，且发送了错误事件。
func TestRunRecoversFromPanic(t *testing.T) {
	e := NewMultiAgentEngine(0).WithConfig(EngineConfig{
		TestPanicHook: func() { panic("boom") },
	})
	ch := e.Run(context.Background(), "600000", "测试", "sh", "价格", "")

	var content strings.Builder
	for msg := range ch {
		content.WriteString(msg.Content)
	}
	if content.Len() == 0 {
		t.Error("expected error event content on channel")
	}
	if !strings.Contains(content.String(), "error") {
		t.Errorf("expected error phase event, got: %s", content.String())
	}
}

// TestRunContextCancel 验证 ctx 取消后 channel 正常关闭、goroutine 不泄漏。
func TestRunContextCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	e := NewMultiAgentEngine(0)
	ch := e.Run(ctx, "600000", "测试", "sh", "价格", "")

	done := make(chan struct{})
	go func() {
		for range ch {
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("channel 未在 5s 内关闭，goroutine 可能泄漏")
	}
}
