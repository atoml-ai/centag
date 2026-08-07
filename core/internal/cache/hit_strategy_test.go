package cache

import (
	"context"
	"testing"

	"centag/core/pkg/config"
	"centag/core/pkg/plugin"
)

func TestNormalizeQueryText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "collapse spaces and zwsp", in: "  hello   world\u200b  ", want: "hello world"},
		{name: "empty", in: "   ", want: ""},
		{name: "bom strip", in: "\ufeffhi", want: "hi"},
		{name: "newlines to space", in: "a\n\tb", want: "a b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeQueryText(tt.in); got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
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

func TestApplyHitStrategies_ExpandNilAndUnknownSkipped(t *testing.T) {
	prev := config.Get()
	config.Set(&config.Config{Cache: config.CacheConfig{
		Backend:       config.CacheBackendExact,
		HitStrategies: []string{"normalize", "expand", "no-such-strategy"},
	}})
	t.Cleanup(func() { config.Set(prev) })

	cands := ApplyHitStrategies(context.Background(), "  alpha  ", nil, nil)
	if len(cands) != 1 || cands[0] != "alpha" {
		t.Fatalf("nil expander + unknown custom should leave normalize only: %v", cands)
	}
}

func TestApplyHitStrategies_EmptyQueryFallback(t *testing.T) {
	prev := config.Get()
	config.Set(&config.Config{Cache: config.CacheConfig{
		Backend:       config.CacheBackendExact,
		HitStrategies: []string{"normalize"},
	}})
	t.Cleanup(func() { config.Set(prev) })

	cands := ApplyHitStrategies(context.Background(), "   ", nil, nil)
	if len(cands) != 1 || cands[0] != "   " {
		// whitespace-only normalizes away; function returns original when empty candidates + non-empty query
		// query is non-empty spaces → NormalizeQueryText("") → candidates empty → fallback to original
		t.Fatalf("got %v", cands)
	}
}
