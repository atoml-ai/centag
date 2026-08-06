package cache

import (
	"context"
	"sync"
	"time"

	"centag/core/pkg/logger"
	"centag/core/pkg/plugin"

	"go.uber.org/zap"
)

// HitStrategy prepares / expands queries before backend Lookup (v0.3.3).
type HitStrategy interface {
	Name() string
	Prepare(ctx context.Context, query string, history []plugin.Message) ([]string, error)
}

// DefaultHitStrategyTimeout bounds custom strategy execution (fail-open on exceed).
const DefaultHitStrategyTimeout = 200 * time.Millisecond

var hitStrategyMu sync.RWMutex
var hitStrategies = map[string]HitStrategy{}

// RegisterHitStrategy registers a custom hit strategy plugin by name.
func RegisterHitStrategy(s HitStrategy) {
	if s == nil || s.Name() == "" {
		return
	}
	hitStrategyMu.Lock()
	defer hitStrategyMu.Unlock()
	hitStrategies[s.Name()] = s
}

// GetHitStrategy returns a registered custom strategy (nil if missing).
func GetHitStrategy(name string) HitStrategy {
	hitStrategyMu.RLock()
	defer hitStrategyMu.RUnlock()
	return hitStrategies[name]
}

// ListHitStrategies returns registered custom strategy names.
func ListHitStrategies() []string {
	hitStrategyMu.RLock()
	defer hitStrategyMu.RUnlock()
	out := make([]string, 0, len(hitStrategies))
	for k := range hitStrategies {
		out = append(out, k)
	}
	return out
}

// runHitStrategyPrepare executes Prepare with timeout; on error/timeout returns nil (fail-open).
func runHitStrategyPrepare(ctx context.Context, s HitStrategy, query string, history []plugin.Message, timeout time.Duration) []string {
	if s == nil {
		return nil
	}
	if timeout <= 0 {
		timeout = DefaultHitStrategyTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	type result struct {
		cands []string
		err   error
	}
	ch := make(chan result, 1)
	go func() {
		cands, err := s.Prepare(ctx, query, history)
		ch <- result{cands, err}
	}()

	select {
	case <-ctx.Done():
		logger.Warn("hit strategy timeout (fail-open)",
			zap.String("strategy", s.Name()),
			zap.Duration("timeout", timeout))
		return nil
	case r := <-ch:
		if r.err != nil {
			logger.Warn("hit strategy error (fail-open)",
				zap.String("strategy", s.Name()),
				zap.Error(r.err))
			return nil
		}
		return r.cands
	}
}
