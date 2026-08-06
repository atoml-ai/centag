package cache

import (
	"context"
	"strings"
	"unicode"

	"centag/core/pkg/config"
	"centag/core/pkg/plugin"
)

// HitPrepareFunc rewrites or expands a query before backend Lookup.
type HitPrepareFunc func(ctx context.Context, query string, history []plugin.Message) ([]string, error)

// ApplyHitStrategies runs configured hit_strategies in order and returns
// candidate queries (original last if nothing produced).
// Built-ins: normalize, expand. Other names resolve via RegisterHitStrategy (timeout fail-open).
func ApplyHitStrategies(ctx context.Context, query string, history []plugin.Message, expander QueryExpander) []string {
	cfgHit := []string{"normalize", "expand"}
	if cfg := config.Get(); cfg != nil {
		c := cfg.Cache
		config.NormalizeCacheConfig(&c)
		if len(c.HitStrategies) > 0 {
			cfgHit = append([]string(nil), c.HitStrategies...)
		}
	}

	candidates := []string{query}
	for _, name := range cfgHit {
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "normalize":
			next := make([]string, 0, len(candidates))
			seen := map[string]struct{}{}
			for _, q := range candidates {
				nq := NormalizeQueryText(q)
				if nq == "" {
					continue
				}
				if _, ok := seen[nq]; ok {
					continue
				}
				seen[nq] = struct{}{}
				next = append(next, nq)
			}
			if len(next) > 0 {
				candidates = next
			}
		case "expand":
			if expander == nil {
				continue
			}
			next := append([]string(nil), candidates...)
			seen := map[string]struct{}{}
			for _, q := range candidates {
				seen[q] = struct{}{}
			}
			for _, q := range candidates {
				expanded, ok, err := expander.Expand(ctx, q, history)
				if err != nil || !ok || expanded == "" {
					continue
				}
				expanded = NormalizeQueryText(expanded)
				if _, dup := seen[expanded]; dup {
					continue
				}
				seen[expanded] = struct{}{}
				next = append(next, expanded)
			}
			candidates = next
		default:
			custom := GetHitStrategy(name)
			if custom == nil {
				continue
			}
			next := append([]string(nil), candidates...)
			seen := map[string]struct{}{}
			for _, q := range candidates {
				seen[q] = struct{}{}
			}
			for _, q := range candidates {
				prepared := runHitStrategyPrepare(ctx, custom, q, history, DefaultHitStrategyTimeout)
				for _, p := range prepared {
					p = NormalizeQueryText(p)
					if p == "" {
						continue
					}
					if _, dup := seen[p]; dup {
						continue
					}
					seen[p] = struct{}{}
					next = append(next, p)
				}
			}
			candidates = next
		}
	}
	if len(candidates) == 0 && query != "" {
		return []string{query}
	}
	return candidates
}

// NormalizeQueryText trims, collapses whitespace, and strips zero-width chars.
func NormalizeQueryText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if r == '\u200b' || r == '\ufeff' {
			continue
		}
		if unicode.IsSpace(r) {
			if prevSpace {
				continue
			}
			b.WriteByte(' ')
			prevSpace = true
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}
