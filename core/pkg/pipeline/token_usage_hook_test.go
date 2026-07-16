package pipeline

import (
	"context"
	"testing"
	"time"
)

func TestPersistTokenUsageFromRecord_UserIDFromExecCtx(t *testing.T) {
	done := make(chan TokenUsagePersistRequest, 1)
	PersistTokenUsage = func(_ context.Context, req TokenUsagePersistRequest) {
		done <- req
	}
	defer func() { PersistTokenUsage = nil }()

	execCtx := NewExecutionContext(&AgentPatternPipeline{ID: "p1"})
	execCtx.SetVariable("user_id", "42")

	ctx := context.WithValue(context.Background(), executionContextKey{}, execCtx)
	input := &NodeInput{
		Metadata: map[string]interface{}{
			"request_id": "req-1",
		},
	}
	record := map[string]interface{}{
		"model":             "deepseek-v4-flash",
		"backend_id":        "deepseek",
		"prompt_tokens":     100,
		"completion_tokens": 50,
		"total_tokens":      150,
	}

	persistTokenUsageFromRecord(ctx, input, record)

	var captured TokenUsagePersistRequest
	select {
	case captured = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("expected async persist to run")
	}

	if captured.UserID != 42 {
		t.Fatalf("UserID = %d, want 42", captured.UserID)
	}
	if captured.Model != "deepseek-v4-flash" {
		t.Fatalf("Model = %q", captured.Model)
	}
	if captured.TotalTokens != 150 {
		t.Fatalf("TotalTokens = %d, want 150", captured.TotalTokens)
	}
}

func TestPersistTokenUsageFromRecord_SkipsWithoutUserID(t *testing.T) {
	called := false
	PersistTokenUsage = func(_ context.Context, _ TokenUsagePersistRequest) {
		called = true
	}
	defer func() { PersistTokenUsage = nil }()

	persistTokenUsageFromRecord(context.Background(), &NodeInput{}, map[string]interface{}{
		"model":        "m",
		"total_tokens": 10,
	})

	if called {
		t.Fatal("expected persist skipped when user_id missing")
	}
}