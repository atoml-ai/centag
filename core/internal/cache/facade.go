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

// StoreExact writes via Store (respects global cache.backend). Kept for call-site compatibility.
func (f *Facade) StoreExact(ctx context.Context, key string, entry *CacheEntry, ttl time.Duration) error {
	return f.Store(ctx, key, entry, ttl)
}

// Store dispatches writes by configured backend (exact / semantic / external).
func (f *Facade) Store(ctx context.Context, key string, entry *CacheEntry, ttl time.Duration) error {
	if f == nil || f.manager == nil {
		return fmt.Errorf("cache facade: manager not configured")
	}
	backend := f.EffectiveBackend()
	switch backend {
	case config.CacheBackendExternal:
		if entry != nil {
			entry.Metadata = EnrichCacheWriteMetadata(entry.Metadata, "external")
		}
		if f.external == nil {
			return fmt.Errorf("external recall backend not configured")
		}
		model, sessionID := "", ""
		req, resp := "", ""
		var meta map[string]interface{}
		if entry != nil {
			req, resp, meta = entry.Request, entry.Response, entry.Metadata
			if meta != nil {
				model, _ = meta["model"].(string)
				sessionID, _ = meta["session_id"].(string)
			}
		}
		if err := f.external.Store(ctx, RecallEntry{
			Key: key, Request: req, Response: resp, Model: model, SessionID: sessionID, TTL: ttl, Metadata: meta,
		}); err != nil {
			logger.Warn("cache facade Store external failed", zap.String("key", key), zap.Error(err))
			return err
		}
		return nil
	case config.CacheBackendSemantic:
		if entry != nil {
			entry.Metadata = EnrichCacheWriteMetadata(entry.Metadata, "semantic")
		}
		return f.manager.Set(ctx, key, entry, ttl)
	default: // exact (+ stacking hybrid write inside Manager.Set)
		if entry != nil {
			entry.Metadata = EnrichCacheWriteMetadata(entry.Metadata, "exact")
		}
		return f.manager.Set(ctx, key, entry, ttl)
	}
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
		entry := entries[0]
		if entry.Metadata == nil {
			entry.Metadata = map[string]interface{}{}
		}
		if _, ok := entry.Metadata["cache_type"]; !ok {
			entry.Metadata["cache_type"] = "semantic"
		}
		f.notifyHit(ctx, entry.Key, entry)
		return entry, true, nil
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
			entry = &CacheEntry{Key: hit.Key, Response: hit.Response, Metadata: map[string]interface{}{}}
		}
		if entry.Metadata == nil {
			entry.Metadata = map[string]interface{}{}
		}
		if _, ok := entry.Metadata["cache_type"]; !ok {
			entry.Metadata["cache_type"] = "external"
		}
		f.notifyHit(ctx, hit.Key, entry)
		return entry, true, nil
	default:
		return nil, false, nil
	}
}

// HitLabel returns X-Cache style label from entry metadata.
func HitLabel(entry *CacheEntry) string {
	if entry == nil {
		return "HIT"
	}
	ct, _ := entry.Metadata["cache_type"].(string)
	switch ct {
	case "semantic":
		return "HIT-SEMANTIC"
	case "external":
		return "HIT-EXTERNAL"
	default:
		return "HIT-EXACT"
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
