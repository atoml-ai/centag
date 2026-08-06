package strategy

import (
	"context"
	"fmt"
)

// HybridStrategy: exact first, then semantic on miss (v0.3.3 — aligns with docs / Manager).
// Used when allow_backend_stacking=true; not the default product path.
type HybridStrategy struct {
	exact    *ExactStrategy
	semantic *SemanticStrategy
}

// NewHybridStrategy 创建混合策略
func NewHybridStrategy(exact *ExactStrategy, semantic *SemanticStrategy) *HybridStrategy {
	return &HybridStrategy{
		exact:    exact,
		semantic: semantic,
	}
}

func (s *HybridStrategy) Name() string {
	return "hybrid"
}

func (s *HybridStrategy) SupportsSemantic() bool {
	return true
}

func (s *HybridStrategy) SetExactStrategy(exact *ExactStrategy) {
	s.exact = exact
}

func (s *HybridStrategy) SetSemanticStrategy(semantic *SemanticStrategy) {
	s.semantic = semantic
}

func (s *HybridStrategy) Configure(config map[string]interface{}) error {
	return nil
}

// Read tries exact first, then semantic (sequential, not race).
func (s *HybridStrategy) Read(ctx context.Context, query string, opts ReadOptions) (*Result, error) {
	if s.exact != nil {
		r, err := s.exact.Read(ctx, query, opts)
		if err != nil {
			return nil, err
		}
		if r != nil && r.Hit {
			r.SourceStrategy = "exact"
			return r, nil
		}
	}
	if s.semantic != nil {
		r, err := s.semantic.Read(ctx, query, opts)
		if err != nil {
			return nil, err
		}
		if r != nil && r.Hit {
			r.SourceStrategy = "semantic"
			return r, nil
		}
		if r != nil {
			return r, nil
		}
	}
	return &Result{Hit: false}, nil
}

// Write writes exact then semantic (both when available).
func (s *HybridStrategy) Write(ctx context.Context, entry *Entry, opts WriteOptions) error {
	var firstErr error
	if s.exact != nil {
		if err := s.exact.Write(ctx, entry, opts); err != nil {
			firstErr = fmt.Errorf("exact write failed: %w", err)
		}
	}
	if s.semantic != nil {
		if err := s.semantic.Write(ctx, entry, opts); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("semantic write failed: %w", err)
			}
		}
	}
	return firstErr
}

// Delete removes from both stores.
func (s *HybridStrategy) Delete(ctx context.Context, key string) error {
	var firstErr error
	if s.exact != nil {
		if err := s.exact.Delete(ctx, key); err != nil {
			firstErr = err
		}
	}
	if s.semantic != nil {
		if err := s.semantic.Delete(ctx, key); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
