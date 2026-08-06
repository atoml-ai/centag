package config

import (
	"fmt"
	"strings"
)

// NormalizeCacheConfig maps legacy strategy fields onto backend / stacking and
// fills defaults. It mutates cfg in place and returns human-readable warnings
// (e.g. deprecated hybrid without stacking).
func NormalizeCacheConfig(cfg *CacheConfig) []string {
	if cfg == nil {
		return nil
	}
	var warnings []string

	backend := strings.TrimSpace(strings.ToLower(cfg.Backend))
	strategy := strings.TrimSpace(strings.ToLower(cfg.Strategy))

	switch {
	case backend != "":
		// Backend wins; sync legacy Strategy for older callers.
		switch backend {
		case CacheBackendExact, CacheBackendSemantic, CacheBackendExternal:
			cfg.Backend = backend
		default:
			warnings = append(warnings, fmt.Sprintf("unknown cache.backend %q, falling back to exact", cfg.Backend))
			cfg.Backend = CacheBackendExact
		}
		if cfg.AllowBackendStacking && cfg.Backend == CacheBackendExact {
			cfg.Strategy = "hybrid"
		} else if cfg.Backend == CacheBackendExternal {
			// Closest legacy label for write-path readers that only know strategy.
			cfg.Strategy = CacheBackendSemantic
		} else {
			cfg.Strategy = cfg.Backend
		}
	case strategy != "":
		warnings = append(warnings, "cache.strategy is deprecated; use cache.backend (and cache.allow_backend_stacking for hybrid)")
		switch strategy {
		case CacheBackendExact:
			cfg.Backend = CacheBackendExact
			cfg.Strategy = CacheBackendExact
		case CacheBackendSemantic:
			cfg.Backend = CacheBackendSemantic
			cfg.Strategy = CacheBackendSemantic
		case "hybrid":
			cfg.Backend = CacheBackendExact
			if cfg.AllowBackendStacking {
				cfg.Strategy = "hybrid"
			} else {
				cfg.Strategy = CacheBackendExact
				warnings = append(warnings,
					"cache.strategy=hybrid mapped to backend=exact; set allow_backend_stacking=true to restore exact-then-semantic stacking")
			}
		default:
			warnings = append(warnings, fmt.Sprintf("unknown cache.strategy %q, falling back to exact", cfg.Strategy))
			cfg.Backend = CacheBackendExact
			cfg.Strategy = CacheBackendExact
		}
	default:
		cfg.Backend = CacheBackendExact
		cfg.Strategy = CacheBackendExact
	}

	if len(cfg.HitStrategies) == 0 {
		cfg.HitStrategies = []string{"normalize", "expand"}
	}

	if cfg.Semantic.TopK <= 0 {
		cfg.Semantic.TopK = 5
	}
	if cfg.Semantic.Threshold <= 0 {
		cfg.Semantic.Threshold = 0.8
	}
	if cfg.Semantic.DistanceType == "" {
		cfg.Semantic.DistanceType = "cosine"
	}

	return warnings
}

// EffectiveCacheBackend returns the normalized backend kind (exact|semantic|external).
func EffectiveCacheBackend(cfg CacheConfig) string {
	NormalizeCacheConfig(&cfg)
	return cfg.Backend
}
