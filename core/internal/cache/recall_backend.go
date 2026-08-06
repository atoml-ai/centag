package cache

import (
	"context"
	"fmt"
	"time"
)

// CacheRecallBackend is the S2/S3 extension contract for cache-style recall
// (usage A: short-circuit before LLM). Frozen minimal surface for v0.3.3.
type CacheRecallBackend interface {
	Name() string
	Kind() string // "semantic" | "external"
	Lookup(ctx context.Context, q RecallQuery) (*RecallHit, error)
	Store(ctx context.Context, e RecallEntry) error
}

// RecallQuery is the normalized lookup input after hit_strategies.
type RecallQuery struct {
	Key       string
	Text      string
	Model     string
	SessionID string
	Threshold float32
	TopK      int
}

// RecallHit is a successful backend lookup.
type RecallHit struct {
	Key      string
	Response string
	Score    float64
	Entry    *CacheEntry
}

// RecallEntry is written on cache_write for extension backends.
type RecallEntry struct {
	Key       string
	Request   string
	Response  string
	Model     string
	SessionID string
	TTL       time.Duration
	Metadata  map[string]interface{}
}

// UnconfiguredRecallBackend always misses — used when backend=external but no plugin is set.
type UnconfiguredRecallBackend struct {
	PluginID string
}

func (u *UnconfiguredRecallBackend) Name() string {
	if u != nil && u.PluginID != "" {
		return u.PluginID
	}
	return "unconfigured-external"
}

func (u *UnconfiguredRecallBackend) Kind() string { return "external" }

func (u *UnconfiguredRecallBackend) Lookup(ctx context.Context, q RecallQuery) (*RecallHit, error) {
	return nil, fmt.Errorf("external recall backend %q not configured", u.Name())
}

func (u *UnconfiguredRecallBackend) Store(ctx context.Context, e RecallEntry) error {
	return fmt.Errorf("external recall backend %q not configured", u.Name())
}
