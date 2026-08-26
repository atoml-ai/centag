package pipeline

import (
	"context"
	"testing"
	"time"
)

// 回归（防双计 + 来源标识）：缓存命中时计量必须恰好一行，
// 胜出元数据来自 cache_read 恢复（cache_hit=true），来源标记为 cache_replay。
func TestCacheHitUsageSingleRowAndCacheReplaySource(t *testing.T) {
	done := make(chan TokenUsagePersistRequest, 4)
	PersistTokenUsage = func(_ context.Context, req TokenUsagePersistRequest) {
		done <- req
	}
	defer func() { PersistTokenUsage = nil }()

	cacheReadResult := &NodeOutput{
		Content: "cached answer",
		Metadata: map[string]interface{}{
			"cache_hit":         true,
			"model":             "GLM-4-flash",
			"backend":           "zhipu",
			"backend_id":        "zhipu",
			"prompt_tokens":     82,
			"completion_tokens": 27,
			"total_tokens":      109,
			"user_id":           "14",
			"request_id":        "req-hit-1",
		},
	}
	llmSkipped := &NodeOutput{ // 命中路径下 LLM 节点被条件跳过：无计量元数据
		Metadata: map[string]interface{}{"cache_hit": false},
	}

	execCtx := NewExecutionContext(&AgentPatternPipeline{ID: "p-cache"})
	execCtx.results["cache_read"] = cacheReadResult
	execCtx.results["llm"] = llmSkipped

	node, err := NewTokenUsageNode(NodeConfig{
		CustomConfig: map[string]interface{}{
			"operation":    "record",
			"storage_type": "memory",
		},
	})
	if err != nil {
		t.Fatalf("NewTokenUsageNode failed: %v", err)
	}

	ctx := context.WithValue(context.Background(), executionContextKey{}, execCtx)
	input := &NodeInput{
		Content: "cached answer",
		Metadata: map[string]interface{}{
			"user_id":    "14",
			"api_key_id": 29,
		},
	}
	if _, err := node.Execute(ctx, input); err != nil {
		t.Fatalf("token usage node execute failed: %v", err)
	}

	if replay, _ := input.Metadata["cache_replay"].(bool); !replay {
		t.Fatal("expected cache_replay flag injected into node input metadata")
	}

	var captured TokenUsagePersistRequest
	select {
	case captured = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("expected exactly one persist call on cache hit")
	}
	select {
	case extra := <-done:
		t.Fatalf("double billing on cache hit: extra row total=%d", extra.TotalTokens)
	default:
	}

	if captured.Source != "cache_replay" {
		t.Fatalf("Source = %q, want cache_replay", captured.Source)
	}
	if captured.TotalTokens != 109 {
		t.Fatalf("TotalTokens = %d, want 109 (restored from cache)", captured.TotalTokens)
	}
	if captured.APIKeyID != 29 {
		t.Fatalf("APIKeyID = %d, want 29", captured.APIKeyID)
	}
}

// 真实调用路径（未命中）：不得带 cache_replay 标记，Source 保持空。
func TestRealUsageNotMarkedCacheReplay(t *testing.T) {
	done := make(chan TokenUsagePersistRequest, 1)
	PersistTokenUsage = func(_ context.Context, req TokenUsagePersistRequest) {
		done <- req
	}
	defer func() { PersistTokenUsage = nil }()

	execCtx := NewExecutionContext(&AgentPatternPipeline{ID: "p-real"})
	execCtx.results["llm"] = &NodeOutput{
		Metadata: map[string]interface{}{
			"model":             "deepseek-v4-flash",
			"backend_id":        "deepseek",
			"prompt_tokens":     100,
			"completion_tokens": 50,
			"total_tokens":      150,
			"cache_hit":         false,
		},
	}

	node, err := NewTokenUsageNode(NodeConfig{
		CustomConfig: map[string]interface{}{
			"operation":    "record",
			"storage_type": "memory",
		},
	})
	if err != nil {
		t.Fatalf("NewTokenUsageNode failed: %v", err)
	}

	ctx := context.WithValue(context.Background(), executionContextKey{}, execCtx)
	input := &NodeInput{Metadata: map[string]interface{}{"user_id": "14"}}
	if _, err := node.Execute(ctx, input); err != nil {
		t.Fatalf("token usage node execute failed: %v", err)
	}

	var captured TokenUsagePersistRequest
	select {
	case captured = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("expected one persist call")
	}
	if captured.Source != "" {
		t.Fatalf("Source = %q, want empty for real usage", captured.Source)
	}
	if replay, _ := input.Metadata["cache_replay"].(bool); replay {
		t.Fatal("real usage must not be marked cache_replay")
	}
}
