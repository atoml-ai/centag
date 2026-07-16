package pipeline

import (
	"context"
	"testing"
	"time"

	"centag/core/internal/cache"
)

// TestRAGModePipeline_CacheHitSkipsRetrieval mirrors production #rag topology:
// cache_read hit → question_splitter / rag_retrieval conditions evaluate false.
func TestRAGModePipeline_CacheHitSkipsRetrieval(t *testing.T) {
	t.Setenv("PROJECT_ROOT", mustProjectRoot(t))
	tmpl := mustLoadRAGTemplate(t)
	p := CreatePipelineFromTemplate(tmpl, nil)
	if err := p.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	cacheKey := "glm-4-flash:年假政策"
	cacheMgr := newMockCacheManager()
	cacheMgr.data[cacheKey] = &cache.CacheEntry{
		Key:      cacheKey,
		Request:  "年假政策",
		Response: "员工每年享有10天带薪年假。[policy/leave.md]",
		Metadata: map[string]interface{}{"model": "glm-4-flash"},
		Timestamp: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}

	cacheRead, err := NewCacheNode(NodeConfig{
		CustomConfig: map[string]interface{}{
			"operation":    "read",
			"strategy":     "exact",
			"storage_type": "memory",
			"key_template": "{{model}}:{{hash}}",
		},
	})
	if err != nil {
		t.Fatalf("NewCacheNode: %v", err)
	}
	if cn, ok := cacheRead.(*CacheNode); ok {
		cn.SetCacheManager(cacheMgr)
	}

	execCtx := NewExecutionContext(nil)
	ctx := context.WithValue(context.Background(), executionContextKey{}, execCtx)

	input := &NodeInput{
		Content: "年假政策",
		Metadata: map[string]interface{}{
			"model": "glm-4-flash",
		},
	}
	out, err := cacheRead.Execute(ctx, input)
	if err != nil {
		t.Fatalf("cache read: %v", err)
	}
	if out.Metadata["cache_hit"] != true {
		t.Fatalf("expected cache_hit=true, got %v", out.Metadata["cache_hit"])
	}
	execCtx.SetResult("cache_read", out)

	eval := NewConditionEvaluator(execCtx)
	for _, nodeID := range []string{"question_splitter", "rag_retrieval", "generator", "answer_synthesizer", "cache_write"} {
		var cond string
		for _, n := range p.Nodes {
			if n.ID == nodeID {
				cond = n.Condition
				break
			}
		}
		if cond == "" {
			t.Fatalf("node %s missing condition", nodeID)
		}
		if eval.Evaluate(cond) {
			t.Fatalf("node %s should be skipped on cache hit, condition=%q", nodeID, cond)
		}
	}
}

// TestRAGModePipeline_CacheMissRunsRetrievalPath ensures miss path nodes are eligible.
func TestRAGModePipeline_CacheMissRunsRetrievalPath(t *testing.T) {
	t.Setenv("PROJECT_ROOT", mustProjectRoot(t))
	tmpl := mustLoadRAGTemplate(t)
	p := CreatePipelineFromTemplate(tmpl, nil)

	execCtx := NewExecutionContext(nil)
	execCtx.SetResult("cache_read", &NodeOutput{
		Metadata: map[string]interface{}{"cache_hit": false},
	})
	eval := NewConditionEvaluator(execCtx)
	for _, nodeID := range []string{"question_splitter", "rag_retrieval", "generator"} {
		var cond string
		for _, n := range p.Nodes {
			if n.ID == nodeID {
				cond = n.Condition
				break
			}
		}
		if !eval.Evaluate(cond) {
			t.Fatalf("node %s should run on cache miss, condition=%q", nodeID, cond)
		}
	}
}

func TestResolvePipelineOutputContent_CacheHitOverTokenUsage(t *testing.T) {
	execCtx := NewExecutionContext(nil)
	execCtx.SetResult("cache_read", &NodeOutput{
		Content:  "Paris is the capital of France.",
		Metadata: map[string]interface{}{"cache_hit": true},
	})
	last := &NodeOutput{Content: "user question echoed by token_usage"}
	got := resolvePipelineOutputContent(execCtx, last)
	if got != "Paris is the capital of France." {
		t.Fatalf("content = %q, want cached answer", got)
	}
}