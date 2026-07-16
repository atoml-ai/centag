package pipeline

import (
	"context"
	"testing"
)

func TestRequestIDFromContext(t *testing.T) {
	if got := RequestIDFromContext(nil); got != "" {
		t.Fatalf("nil ctx: got %q", got)
	}
	ctx := context.Background()
	if got := RequestIDFromContext(ctx); got != "" {
		t.Fatalf("empty ctx: got %q", got)
	}

	execCtx := NewExecutionContext(nil)
	execCtx.SetVariable("request_id", "cdd86a39663c30b34ae019c28900b14b")
	ctx = context.WithValue(ctx, executionContextKey{}, execCtx)

	if got := RequestIDFromContext(ctx); got != "cdd86a39663c30b34ae019c28900b14b" {
		t.Fatalf("got %q", got)
	}

	fields := AppendRequestIDFields(ctx, "node_id", "generate")
	if len(fields) != 4 {
		t.Fatalf("fields len = %d, want 4", len(fields))
	}
	if fields[2] != "request_id" || fields[3] != "cdd86a39663c30b34ae019c28900b14b" {
		t.Fatalf("unexpected fields: %v", fields)
	}
}
