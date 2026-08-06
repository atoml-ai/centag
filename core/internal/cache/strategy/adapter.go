package strategy

import (
	"context"
	"fmt"
	"time"
)

// Adapter wraps a Strategy to implement CacheStrategyCapability.
// This bridges the internal cache/strategy package with the pipeline capability layer.
type Adapter struct {
	strategy Strategy
}

// NewAdapter creates a capability adapter wrapping the given strategy.
func NewAdapter(s Strategy) *Adapter {
	return &Adapter{strategy: s}
}

// Read implements CacheStrategyCapability.Read.
// It converts the simplified signature back to the Strategy interface's signature.
func (a *Adapter) Read(ctx context.Context, query string, threshold float32, topK int) (*ReadResult, error) {
	if a.strategy == nil {
		return &ReadResult{Hit: false}, fmt.Errorf("underlying strategy is nil")
	}

	opts := ReadOptions{
		Threshold: threshold,
		TopK:      topK,
	}

	result, err := a.strategy.Read(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	return &ReadResult{
		Hit:     result.Hit,
		Content: result.Content,
		Key:     result.Key,
		Score:   result.Score,
	}, nil
}

// Write implements CacheStrategyCapability.Write.
// request is the user query text used for semantic embedding (must be non-empty for semantic/hybrid).
func (a *Adapter) Write(ctx context.Context, key string, request string, content string, ttl time.Duration) error {
	if a.strategy == nil {
		return fmt.Errorf("underlying strategy is nil")
	}

	entry := &Entry{
		Key:       key,
		Request:   request,
		Response:  content,
		Timestamp: time.Now(),
		ExpiresAt: time.Now().Add(ttl),
		Metadata:  map[string]interface{}{},
	}

	opts := WriteOptions{
		TTL:               ttl,
		GenerateEmbedding: true,
	}

	return a.strategy.Write(ctx, entry, opts)
}

// Delete implements CacheStrategyCapability.Delete.
func (a *Adapter) Delete(ctx context.Context, key string) error {
	if a.strategy == nil {
		return fmt.Errorf("underlying strategy is nil")
	}
	return a.strategy.Delete(ctx, key)
}

// StrategyName returns the wrapped strategy's name.
func (a *Adapter) StrategyName() string {
	if a.strategy == nil {
		return "nil"
	}
	return a.strategy.Name()
}

// Unwrap returns the underlying Strategy (for direct access if needed).
func (a *Adapter) Unwrap() Strategy {
	return a.strategy
}

// ReadResult is the simplified read result used by CacheStrategyCapability.
// This mirrors pipeline.CacheReadResult to avoid cross-package imports.
type ReadResult struct {
	Hit     bool
	Content string
	Key     string
	Score   float64
}
