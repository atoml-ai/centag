package cache

import (
	"context"
	"fmt"
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

type stubExternalBackend struct {
	hit      *RecallHit
	err      error
	storeErr error
	stored   *RecallEntry
}

func (s *stubExternalBackend) Name() string { return "stub-external" }
func (s *stubExternalBackend) Kind() string { return "external" }
func (s *stubExternalBackend) Lookup(ctx context.Context, q RecallQuery) (*RecallHit, error) {
	return s.hit, s.err
}
func (s *stubExternalBackend) Store(ctx context.Context, e RecallEntry) error {
	cp := e
	s.stored = &cp
	return s.storeErr
}

func TestFacadeLookup_ExternalHitAndMiss(t *testing.T) {
	prev := config.Get()
	cfg := config.DefaultCacheConfig()
	cfg.Backend = config.CacheBackendExternal
	config.Set(&config.Config{Cache: cfg})
	t.Cleanup(func() { config.Set(prev) })

	mgr, err := NewManager(&CacheConfig{Enabled: true, DefaultTTL: 60})
	if err != nil {
		t.Fatal(err)
	}
	facade := NewFacade(mgr)
	if err := facade.EnsureBackendReady(); err == nil {
		t.Fatal("external without plugin must fail EnsureBackendReady")
	}

	// miss / error → fail-open, never fabricate hit
	down := &stubExternalBackend{err: fmt.Errorf("down")}
	facade.SetExternalBackend(down)
	if err := facade.EnsureBackendReady(); err != nil {
		t.Fatalf("ready after SetExternalBackend: %v", err)
	}
	entry, ok, err := facade.Lookup(context.Background(), "k", "q", 0.8, 5)
	if err != nil || ok || entry != nil {
		t.Fatalf("external error must miss: ok=%v entry=%v err=%v", ok, entry, err)
	}

	var hits int
	facade.SetHitNotifier(func(ctx context.Context, key string, data []byte) { hits++ })
	ext := &stubExternalBackend{hit: &RecallHit{
		Key: "k", Response: "external-answer",
	}}
	facade.SetExternalBackend(ext)
	entry, ok, err = facade.Lookup(context.Background(), "k", "q", 0.8, 5)
	if err != nil || !ok || entry == nil || entry.Response != "external-answer" {
		t.Fatalf("external hit: ok=%v entry=%v err=%v", ok, entry, err)
	}
	if hits != 1 {
		t.Fatalf("hit notifier=%d", hits)
	}
	if HitLabel(entry) != "HIT-EXTERNAL" {
		t.Fatalf("label=%s", HitLabel(entry))
	}

	if err := facade.Store(context.Background(), "k", &CacheEntry{
		Key: "k", Request: "q", Response: "a",
		Metadata: map[string]interface{}{"session_id": "s1", "model": "m1"},
	}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if ext.stored == nil || ext.stored.SessionID != "s1" || ext.stored.Response != "a" {
		t.Fatalf("external store payload=%+v", ext.stored)
	}
}

func TestFacadeEnsureBackendReady_SemanticMissing(t *testing.T) {
	prev := config.Get()
	cfg := config.DefaultCacheConfig()
	cfg.Backend = config.CacheBackendSemantic
	config.Set(&config.Config{Cache: cfg})
	t.Cleanup(func() { config.Set(prev) })

	mgr, err := NewManager(&CacheConfig{Enabled: true, DefaultTTL: 60})
	if err != nil {
		t.Fatal(err)
	}
	facade := NewFacade(mgr)
	if err := facade.EnsureBackendReady(); err == nil {
		t.Fatal("semantic without vector/embedding must error")
	}
	entry, ok, err := facade.Lookup(context.Background(), "k", "query", 0.8, 5)
	if err != nil || ok || entry != nil {
		t.Fatalf("semantic not ready must miss: ok=%v err=%v", ok, err)
	}
}

func TestFacadeLookupExact_DisabledOrEmpty(t *testing.T) {
	mgr, err := NewManager(&CacheConfig{Enabled: false, DefaultTTL: 60})
	if err != nil {
		t.Fatal(err)
	}
	facade := NewFacade(mgr)
	entry, ok, err := facade.LookupExact(context.Background(), "any")
	if err != nil || ok || entry != nil {
		t.Fatalf("disabled cache: ok=%v err=%v", ok, err)
	}
	entry, ok, err = facade.LookupExact(context.Background(), "")
	if err != nil || ok || entry != nil {
		t.Fatalf("empty key: ok=%v err=%v", ok, err)
	}
	if NewFacade(nil).Manager() != nil {
		t.Fatal("nil manager")
	}
}
