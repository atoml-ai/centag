package cache

import (
	"context"
	"testing"
	"time"

	"centag/core/pkg/config"
)

func TestManagerSet_ExternalBackendSkipsExactAndSemantic(t *testing.T) {
	prev := config.Get()
	cfg := config.DefaultCacheConfig()
	cfg.Backend = config.CacheBackendExternal
	config.Set(&config.Config{Cache: cfg})
	t.Cleanup(func() { config.Set(prev) })

	mgr, err := NewManager(&CacheConfig{Enabled: true, DefaultTTL: 60})
	if err != nil {
		t.Fatal(err)
	}
	key := "ext-skip-key"
	if err := mgr.Set(context.Background(), key, &CacheEntry{
		Key: key, Request: "q", Response: "a", Metadata: map[string]interface{}{},
	}, time.Hour); err != nil {
		t.Fatal(err)
	}
	got, err := mgr.GetExactCache().Get(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatal("backend=external must not write exact via Manager.Set")
	}
}
