package multi

import (
	"context"
	"testing"
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
