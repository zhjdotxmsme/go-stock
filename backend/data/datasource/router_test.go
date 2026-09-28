package datasource

import (
	"context"
	"strings"
	"testing"
)

// TestCallWithProviderTimeoutRecoversPanic is the regression test for the
// startup crash where a malformed TDX packet made gotdx's ParseResponse panic
// (slice bounds out of range) inside the provider goroutine, taking down the
// whole process with exit code 2. A panicking provider must surface as a
// normal error so the router can fall through to the next source.
func TestCallWithProviderTimeoutRecoversPanic(t *testing.T) {
	_, err := callWithProviderTimeout(context.Background(), DataTypeKLine,
		func(ctx context.Context) (int, error) {
			panic("slice bounds out of range [:6] with capacity 2")
		})
	if err == nil {
		t.Fatal("expected panic to be converted to error, got nil")
	}
	if !strings.Contains(err.Error(), "panic") {
		t.Fatalf("error should mention the panic, got: %v", err)
	}
}
