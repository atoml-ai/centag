package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"centag/core/pkg/config"
	"centag/core/pkg/plugin"
)

type slowHitStrategy struct {
	delay time.Duration
}

func (s slowHitStrategy) Name() string { return "slow-test" }

func (s slowHitStrategy) Prepare(ctx context.Context, query string, history []plugin.Message) ([]string, error) {
	select {
	case <-time.After(s.delay):
		return []string{query + "-slow"}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type errHitStrategy struct{}

func (errHitStrategy) Name() string { return "err-test" }

func (errHitStrategy) Prepare(ctx context.Context, query string, history []plugin.Message) ([]string, error) {
	return nil, errors.New("boom")
}

type sessionAwareStrategy struct{}

func (sessionAwareStrategy) Name() string { return "session-aware-test" }

func (sessionAwareStrategy) Prepare(ctx context.Context, query string, history []plugin.Message) ([]string, error) {
	tag := "none"
	for _, m := range history {
		if m.Role == "system" && m.Content != "" {
			tag = m.Content
			break
		}
	}
	return []string{query + "|" + tag}, nil
}

func TestApplyHitStrategies_CustomTimeoutFailOpen(t *testing.T) {
	RegisterHitStrategy(slowHitStrategy{delay: 500 * time.Millisecond})
	prev := config.Get()
	config.Set(&config.Config{Cache: config.CacheConfig{
		Backend:       config.CacheBackendExact,
		HitStrategies: []string{"normalize", "slow-test"},
	}})
	t.Cleanup(func() { config.Set(prev) })

	cands := ApplyHitStrategies(context.Background(), "  hello  ", nil, nil)
	if len(cands) != 1 || cands[0] != "hello" {
		t.Fatalf("fail-open want [hello], got %v", cands)
	}
}

func TestApplyHitStrategies_CustomErrorFailOpen(t *testing.T) {
	RegisterHitStrategy(errHitStrategy{})
	prev := config.Get()
	config.Set(&config.Config{Cache: config.CacheConfig{
		Backend:       config.CacheBackendExact,
		HitStrategies: []string{"err-test"},
	}})
	t.Cleanup(func() { config.Set(prev) })

	cands := ApplyHitStrategies(context.Background(), "q", nil, nil)
	if len(cands) != 1 || cands[0] != "q" {
		t.Fatalf("fail-open want [q], got %v", cands)
	}
}

func TestApplyHitStrategies_SessionHistoryIsolation(t *testing.T) {
	RegisterHitStrategy(sessionAwareStrategy{})
	prev := config.Get()
	config.Set(&config.Config{Cache: config.CacheConfig{
		Backend:       config.CacheBackendExact,
		HitStrategies: []string{"session-aware-test"},
	}})
	t.Cleanup(func() { config.Set(prev) })

	histA := []plugin.Message{{Role: "system", Content: "session-A"}}
	histB := []plugin.Message{{Role: "system", Content: "session-B"}}
	a := ApplyHitStrategies(context.Background(), "same-q", histA, nil)
	b := ApplyHitStrategies(context.Background(), "same-q", histB, nil)
	if len(a) < 2 || len(b) < 2 {
		t.Fatalf("a=%v b=%v", a, b)
	}
	var tagA, tagB string
	for _, c := range a {
		if c != "same-q" {
			tagA = c
		}
	}
	for _, c := range b {
		if c != "same-q" {
			tagB = c
		}
	}
	if tagA == tagB {
		t.Fatalf("expected different session expansions, both=%q", tagA)
	}
	if tagA != "same-q|session-A" || tagB != "same-q|session-B" {
		t.Fatalf("a=%q b=%q", tagA, tagB)
	}
}

func TestGetRequestKey_ModelIsolation(t *testing.T) {
	mgr, err := NewManager(&CacheConfig{Enabled: true, DefaultTTL: 60})
	if err != nil {
		t.Fatal(err)
	}
	pc := NewProxyCache(mgr, true)
	msgs := []interface{}{
		map[string]interface{}{"role": "user", "content": "hello"},
	}
	k1, err := pc.GetRequestKey("model-a", msgs, 0.7, 0, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	k2, err := pc.GetRequestKey("model-b", msgs, 0.7, 0, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if k1 == k2 {
		t.Fatal("different models must not share cache key")
	}
}
