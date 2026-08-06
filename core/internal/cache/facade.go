package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"centag/core/pkg/config"
	"centag/core/pkg/logger"

	"go.uber.org/zap"
)

// HitNotifier is invoked after a successful cache hit (fail-open at caller).
type HitNotifier func(ctx context.Context, key string, data []byte)

// Facade is the unified Lookup/Store entry for S1/S2/S3 recall backends.
// Pipeline CacheNode, ProxyCache, and hooks should share this type.
type Facade struct {
	manager  *Manager
	onHit    HitNotifier
	external CacheRecallBackend
}

// NewFacade wraps a Manager.
func NewFacade(m *Manager) *Facade {
	return &Facade{manager: m}
}

// SetExternalBackend registers the S3 external recall plugin (optional).
func (f *Facade) SetExternalBackend(b CacheRecallBackend) {
	if f != nil {
		f.external = b
	}
}

// SetHitNotifier registers an optional OnCacheHit callback (e.g. hooks.TriggerCacheHitHooks).
func (f *Facade) SetHitNotifier(n HitNotifier) {
	if f != nil {
		f.onHit = n
	}
}

// Manager returns the underlying manager.
func (f *Facade) Manager() *Manager {
	if f == nil {
		return nil
	}
	return f.manager
}

// EffectiveBackend returns the active recall backend after normalization.
func (f *Facade) EffectiveBackend() string {
	if cfg := config.Get(); cfg != nil {
		c := cfg.Cache
		config.NormalizeCacheConfig(&c)
		return c.Backend
	}
	return config.CacheBackendExact
}

// EnsureBackendReady validates dependencies for the active backend.
// For exact, a missing KV store yields a warning (memory-only still allowed).
func (f *Facade) EnsureBackendReady() error {
	if f == nil || f.manager == nil {
		return fmt.Errorf("cache facade: manager not configured")
	}
	backend := f.EffectiveBackend()
	switch backend {
	case config.CacheBackendExact:
		if f.manager.GetKVStore() == nil && f.manager.GetExactCache() != nil {
			logger.Warn("cache backend=exact: no KV store configured; using in-memory exact cache only")
		}
		if f.manager.GetExactCache() == nil {
			return fmt.Errorf("cache backend=exact: exact cache not initialized")
		}
		return nil
	case config.CacheBackendSemantic:
		if f.manager.GetSemanticCache() == nil {
			return fmt.Errorf("cache backend=semantic: semantic cache not configured (enable embedding/vector)")
		}
		return nil
	case config.CacheBackendExternal:
		if f.external == nil {
			return fmt.Errorf("cache backend=external: no plugin configured (set cache.external.plugin)")
		}
		return nil
	default:
		return fmt.Errorf("cache backend %q: unsupported", backend)
	}
}

// LookupExact performs S1 KV/exact recall by key.
func (f *Facade) LookupExact(ctx context.Context, key string) (*CacheEntry, bool, error) {
	if f == nil || f.manager == nil || key == "" {
		return nil, false, nil
	}
	if !f.manager.config.Enabled {
		return nil, false, nil
	}
	exact := f.manager.GetExactCache()
	if exact == nil {
		logger.Error("cache facade LookupExact: exact cache nil")
		return nil, false, fmt.Errorf("exact cache not initialized")
	}
	entry, err := exact.Get(ctx, key)
	if err != nil {
		logger.Error("cache facade LookupExact failed", zap.String("key", key), zap.Error(err))
		return nil, false, nil
	}
	if entry == nil {
		return nil, false, nil
	}
	f.notifyHit(ctx, key, entry)
	return entry, true, nil
}

// StoreExact writes an S1 exact cache entry (and follows Manager.Set strategy for semantic side effects).
func (f *Facade) StoreExact(ctx context.Context, key string, entry *CacheEntry, ttl time.Duration) error {
	if f == nil || f.manager == nil {
		return fmt.Errorf("cache facade: manager not configured")
	}
	if entry != nil {
		entry.Metadata = EnrichCacheWriteMetadata(entry.Metadata, "exact")
	}
	return f.manager.Set(ctx, key, entry, ttl)
}

// Lookup dispatches by configured backend. Default is mutually exclusive;
// allow_backend_stacking enables exact-then-semantic.
func (f *Facade) Lookup(ctx context.Context, key, queryText string, threshold float32, topK int) (*CacheEntry, bool, error) {
	backend := f.EffectiveBackend()
	stacking := false
	if cfg := config.Get(); cfg != nil {
		c := cfg.Cache
		config.NormalizeCacheConfig(&c)
		stacking = c.AllowBackendStacking
	}

	if err := f.EnsureBackendReady(); err != nil {
		logger.Warn("cache facade Lookup backend not ready", zap.Error(err))
		if backend != config.CacheBackendExact {
			return nil, false, nil
		}
	}

	lookupSemantic := func() (*CacheEntry, bool, error) {
		if queryText == "" {
			queryText = key
		}
		if threshold <= 0 {
			threshold = f.manager.GetSemanticThreshold()
		}
		if topK <= 0 {
			if cfg := config.Get(); cfg != nil && cfg.Cache.Semantic.TopK > 0 {
				topK = cfg.Cache.Semantic.TopK
			} else {
				topK = 5
			}
		}
		entries, err := f.manager.SearchByQuery(ctx, queryText, threshold, topK)
		if err != nil || len(entries) == 0 {
			return nil, false, nil
		}
		f.notifyHit(ctx, entries[0].Key, entries[0])
		return entries[0], true, nil
	}

	switch backend {
	case config.CacheBackendExact:
		entry, ok, err := f.LookupExact(ctx, key)
		if err != nil || ok || !stacking {
			return entry, ok, err
		}
		return lookupSemantic()
	case config.CacheBackendSemantic:
		return lookupSemantic()
	case config.CacheBackendExternal:
		if f.external == nil {
			logger.Warn("cache facade Lookup: external backend not configured")
			return nil, false, nil
		}
		hit, err := f.external.Lookup(ctx, RecallQuery{
			Key: key, Text: queryText, Threshold: threshold, TopK: topK,
		})
		if err != nil || hit == nil {
			return nil, false, nil // fail-open miss; never mock a hit
		}
		entry := hit.Entry
		if entry == nil {
			entry = &CacheEntry{Key: hit.Key, Response: hit.Response}
		}
		f.notifyHit(ctx, hit.Key, entry)
		return entry, true, nil
	default:
		return nil, false, nil
	}
}

func (f *Facade) notifyHit(ctx context.Context, key string, entry *CacheEntry) {
	if f == nil || f.onHit == nil || entry == nil {
		return
	}
	data, err := json.Marshal(entry)
	if err != nil {
		data = []byte(entry.Response)
	}
	defer func() {
		if r := recover(); r != nil {
			logger.Warn("cache hit notifier panic (fail-open)", zap.Any("recover", r))
		}
	}()
	f.onHit(ctx, key, data)
}
