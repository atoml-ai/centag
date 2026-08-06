package cache

import (
	"context"
	"testing"

	"centag/core/pkg/config"
	"centag/core/pkg/plugin"
)

func TestNormalizeQueryText(t *testing.T) {
	in := "  hello   world\u200b  "
	got := NormalizeQueryText(in)
	if got != "hello world" {
		t.Fatalf("got %q", got)
	}
}

type stubExpander struct{}

func (stubExpander) Expand(ctx context.Context, current string, history []plugin.Message) (string, bool, error) {
	return current + " expanded", true, nil
}

func TestApplyHitStrategies_NormalizeAndExpand(t *testing.T) {
	prev := config.Get()
	config.Set(&config.Config{Cache: config.CacheConfig{
		Backend:       config.CacheBackendExact,
		HitStrategies: []string{"normalize", "expand"},
	}})
	t.Cleanup(func() { config.Set(prev) })

	cands := ApplyHitStrategies(context.Background(), "  foo   bar  ", nil, stubExpander{})
	if len(cands) < 2 {
		t.Fatalf("candidates=%v", cands)
	}
	if cands[0] != "foo bar" {
		t.Fatalf("first=%q", cands[0])
	}
	found := false
	for _, c := range cands {
		if c == "foo bar expanded" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected expanded candidate in %v", cands)
	}
}
