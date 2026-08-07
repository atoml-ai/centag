package pipeline

import (
	"testing"

	"centag/core/pkg/config"
)

func TestGlobalCacheAllowsSemantic(t *testing.T) {
	prev := config.Get()
	t.Cleanup(func() { config.Set(prev) })

	config.Set(&config.Config{Cache: config.CacheConfig{Backend: config.CacheBackendExact}})
	if globalCacheAllowsSemantic() {
		t.Fatal("exact without stacking must not allow semantic")
	}

	config.Set(&config.Config{Cache: config.CacheConfig{
		Backend: config.CacheBackendExact, AllowBackendStacking: true,
	}})
	if !globalCacheAllowsSemantic() {
		t.Fatal("stacking exact should allow semantic fallthrough")
	}

	config.Set(&config.Config{Cache: config.CacheConfig{Backend: config.CacheBackendSemantic}})
	if !globalCacheAllowsSemantic() {
		t.Fatal("semantic backend allows semantic")
	}

	config.Set(&config.Config{Cache: config.CacheConfig{Backend: config.CacheBackendExternal}})
	if globalCacheAllowsSemantic() {
		t.Fatal("external must not enable local semantic vector path")
	}
}

func TestSessionIDFromCacheInput(t *testing.T) {
	if got := sessionIDFromCacheInput(&NodeInput{
		Context: map[string]interface{}{"session_id": "ctx-s"},
	}, nil); got != "ctx-s" {
		t.Fatalf("got %q", got)
	}
	if got := sessionIDFromCacheInput(&NodeInput{
		Metadata: map[string]interface{}{"session_id": "meta-s"},
	}, nil); got != "meta-s" {
		t.Fatalf("got %q", got)
	}
	exec := NewExecutionContext(nil)
	exec.SetVariable("session_id", "var-s")
	if got := sessionIDFromCacheInput(&NodeInput{}, exec); got != "var-s" {
		t.Fatalf("got %q", got)
	}
}
