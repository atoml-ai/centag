package cache

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"centag/core/pkg/config"
)

func TestFacadeExactRoundTripAndHitNotifier(t *testing.T) {
	prev := config.Get()
	config.Set(&config.Config{Cache: config.DefaultCacheConfig()})
	t.Cleanup(func() { config.Set(prev) })

	mgr, err := NewManager(&CacheConfig{Enabled: true, DefaultTTL: 60})
	if err != nil {
		t.Fatal(err)
	}
	facade := NewFacade(mgr)
	if err := facade.EnsureBackendReady(); err != nil {
		t.Fatalf("EnsureBackendReady: %v", err)
	}
	if facade.EffectiveBackend() != config.CacheBackendExact {
		t.Fatalf("backend=%s", facade.EffectiveBackend())
	}

	var hits atomic.Int32
	facade.SetHitNotifier(func(ctx context.Context, key string, data []byte) {
		hits.Add(1)
	})

	ctx := context.Background()
	key := "facade-test-key"
	entry := &CacheEntry{
		Key:       key,
		Request:   "hello",
		Response:  "world",
		Timestamp: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
		Metadata:  map[string]interface{}{},
	}
	if err := facade.StoreExact(ctx, key, entry, time.Hour); err != nil {
		t.Fatal(err)
	}
	got, ok, err := facade.LookupExact(ctx, key)
	if err != nil || !ok || got == nil {
		t.Fatalf("lookup ok=%v err=%v entry=%v", ok, err, got)
	}
	if got.Response != "world" {
		t.Fatalf("response=%q", got.Response)
	}
	if hits.Load() != 1 {
		t.Fatalf("hit notifier calls=%d want 1", hits.Load())
	}
	if got.Metadata["cache_type"] != "exact" {
		t.Fatalf("cache_type metadata=%v", got.Metadata["cache_type"])
	}
}

func TestFacadeLookup_StackingDefaultOff(t *testing.T) {
	prev := config.Get()
	cfg := config.DefaultCacheConfig()
	cfg.Backend = config.CacheBackendExact
	cfg.AllowBackendStacking = false
	config.Set(&config.Config{Cache: cfg})
	t.Cleanup(func() { config.Set(prev) })

	mgr, err := NewManager(&CacheConfig{Enabled: true, DefaultTTL: 60})
	if err != nil {
		t.Fatal(err)
	}
	facade := NewFacade(mgr)
	entry, ok, err := facade.Lookup(context.Background(), "missing-key", "some query text", 0.8, 5)
	if err != nil {
		t.Fatal(err)
	}
	if ok || entry != nil {
		t.Fatal("exact miss without stacking must not fabricate a hit")
	}
}

func TestFacadeLookup_StackingOnExactMissFallsThrough(t *testing.T) {
	prev := config.Get()
	cfg := config.DefaultCacheConfig()
	cfg.Backend = config.CacheBackendExact
	cfg.AllowBackendStacking = true
	config.Set(&config.Config{Cache: cfg})
	t.Cleanup(func() { config.Set(prev) })

	mgr, err := NewManager(&CacheConfig{Enabled: true, DefaultTTL: 60})
	if err != nil {
		t.Fatal(err)
	}
	// No semantic cache configured: stacking fallthrough should still fail-open as miss.
	facade := NewFacade(mgr)
	entry, ok, err := facade.Lookup(context.Background(), "missing-key", "query", 0.8, 5)
	if err != nil {
		t.Fatal(err)
	}
	if ok || entry != nil {
		t.Fatal("stacking with no semantic cache should miss")
	}
}

func TestProxyCacheUsesFacade(t *testing.T) {
	prev := config.Get()
	config.Set(&config.Config{Cache: config.DefaultCacheConfig()})
	t.Cleanup(func() { config.Set(prev) })

	mgr, err := NewManager(&CacheConfig{Enabled: true, DefaultTTL: 60})
	if err != nil {
		t.Fatal(err)
	}
	pc := NewProxyCache(mgr, true)
	if pc.Facade() == nil {
		t.Fatal("expected facade on ProxyCache")
	}
	ctx := context.Background()
	key := "pc-facade-key"
	_ = pc.Facade().StoreExact(ctx, key, &CacheEntry{
		Key: key, Response: "cached", ExpiresAt: time.Now().Add(time.Hour),
		Metadata: map[string]interface{}{},
	}, time.Hour)
	entry, ok, err := pc.TryGetEntry(ctx, key)
	if err != nil || !ok || entry.Response != "cached" {
		t.Fatalf("TryGetEntry ok=%v err=%v entry=%v", ok, err, entry)
	}
}
