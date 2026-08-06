package pipeline

import (
	"context"
	"fmt"
	"time"

	"centag/core/internal/cache/strategy"
)

// cacheStrategyAdapter wraps a strategy.Strategy to implement CacheStrategyCapability.
// This bridges the internal cache/strategy package (Strategy interface) with
// the pipeline capability layer (CacheStrategyCapability interface).
type cacheStrategyAdapter struct {
	strategy strategy.Strategy
}

// newCacheStrategyAdapter creates an adapter wrapping the given strategy.
func newCacheStrategyAdapter(s strategy.Strategy) *cacheStrategyAdapter {
	return &cacheStrategyAdapter{strategy: s}
}

// Read implements CacheStrategyCapability.Read.
// Converts the simplified signature back to Strategy's signature.
func (a *cacheStrategyAdapter) Read(ctx context.Context, query string, threshold float32, topK int) (*CacheReadResult, error) {
	if a.strategy == nil {
		return &CacheReadResult{Hit: false}, fmt.Errorf("underlying strategy is nil")
	}

	opts := strategy.ReadOptions{
		Threshold: threshold,
		TopK:      topK,
	}

	result, err := a.strategy.Read(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	return &CacheReadResult{
		Hit:     result.Hit,
		Content: result.Content,
		Key:     result.Key,
		Score:   result.Score,
	}, nil
}

// Write implements CacheStrategyCapability.Write.
// request is the query text for embedding; content is the cached response body.
func (a *cacheStrategyAdapter) Write(ctx context.Context, key string, request string, content string, ttl time.Duration) error {
	if a.strategy == nil {
		return fmt.Errorf("underlying strategy is nil")
	}

	entry := &strategy.Entry{
		Key:       key,
		Request:   request,
		Response:  content,
		Timestamp: time.Now(),
		ExpiresAt: time.Now().Add(ttl),
		Metadata:  map[string]interface{}{},
	}

	opts := strategy.WriteOptions{
		TTL:               ttl,
		GenerateEmbedding: true,
	}

	return a.strategy.Write(ctx, entry, opts)
}

// Delete implements CacheStrategyCapability.Delete.
func (a *cacheStrategyAdapter) Delete(ctx context.Context, key string) error {
	if a.strategy == nil {
		return fmt.Errorf("underlying strategy is nil")
	}
	return a.strategy.Delete(ctx, key)
}

// StrategyName returns the wrapped strategy's name.
func (a *cacheStrategyAdapter) StrategyName() string {
	if a.strategy == nil {
		return "nil"
	}
	return a.strategy.Name()
}

// cacheStrategyProvider implements CacheStrategyProvider interface.
// It fetches strategies from the strategy registry and wraps them with the adapter.
type cacheStrategyProvider struct {
	registry *strategy.Registry
}

// NewCacheStrategyProvider creates a provider backed by the given strategy registry.
func NewCacheStrategyProvider(reg *strategy.Registry) *cacheStrategyProvider {
	return &cacheStrategyProvider{registry: reg}
}

// GetStrategy implements CacheStrategyProvider.GetStrategy.
func (p *cacheStrategyProvider) GetStrategy(name string) (CacheStrategyCapability, error) {
	if p.registry == nil {
		return nil, fmt.Errorf("strategy registry not configured")
	}

	s, err := p.registry.Get(name)
	if err != nil {
		return nil, err
	}

	return newCacheStrategyAdapter(s), nil
}
