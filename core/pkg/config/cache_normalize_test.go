package config

import (
	"strings"
	"testing"
)

func TestDefaultCacheConfig_BackendExact(t *testing.T) {
	cfg := DefaultCacheConfig()
	if cfg.Backend != CacheBackendExact {
		t.Fatalf("Backend = %q, want %q", cfg.Backend, CacheBackendExact)
	}
	if cfg.AllowBackendStacking {
		t.Fatal("AllowBackendStacking should default false")
	}
	if cfg.Strategy != CacheBackendExact {
		t.Fatalf("Strategy mirror = %q, want %q", cfg.Strategy, CacheBackendExact)
	}
	if len(cfg.HitStrategies) < 1 {
		t.Fatal("HitStrategies should have defaults")
	}
}

func TestNormalizeCacheConfig_LegacyStrategyMapping(t *testing.T) {
	tests := []struct {
		name        string
		in          CacheConfig
		wantBackend string
		wantStrat   string
		wantStack   bool
		warnSubstr  string
	}{
		{
			name:        "legacy exact",
			in:          CacheConfig{Strategy: "exact"},
			wantBackend: CacheBackendExact,
			wantStrat:   CacheBackendExact,
			warnSubstr:  "deprecated",
		},
		{
			name:        "legacy semantic",
			in:          CacheConfig{Strategy: "semantic"},
			wantBackend: CacheBackendSemantic,
			wantStrat:   CacheBackendSemantic,
			warnSubstr:  "deprecated",
		},
		{
			name:        "legacy hybrid without stacking",
			in:          CacheConfig{Strategy: "hybrid"},
			wantBackend: CacheBackendExact,
			wantStrat:   CacheBackendExact,
			wantStack:   false,
			warnSubstr:  "allow_backend_stacking",
		},
		{
			name:        "legacy hybrid with stacking",
			in:          CacheConfig{Strategy: "hybrid", AllowBackendStacking: true},
			wantBackend: CacheBackendExact,
			wantStrat:   "hybrid",
			wantStack:   true,
			warnSubstr:  "deprecated",
		},
		{
			name:        "backend wins over strategy",
			in:          CacheConfig{Backend: CacheBackendSemantic, Strategy: "exact"},
			wantBackend: CacheBackendSemantic,
			wantStrat:   CacheBackendSemantic,
		},
		{
			name:        "empty defaults to exact",
			in:          CacheConfig{},
			wantBackend: CacheBackendExact,
			wantStrat:   CacheBackendExact,
		},
		{
			name:        "external backend",
			in:          CacheConfig{Backend: CacheBackendExternal, External: ExternalCacheConfig{Plugin: "demo"}},
			wantBackend: CacheBackendExternal,
			wantStrat:   CacheBackendSemantic,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.in
			warns := NormalizeCacheConfig(&cfg)
			if cfg.Backend != tt.wantBackend {
				t.Fatalf("Backend = %q, want %q", cfg.Backend, tt.wantBackend)
			}
			if cfg.Strategy != tt.wantStrat {
				t.Fatalf("Strategy = %q, want %q", cfg.Strategy, tt.wantStrat)
			}
			if cfg.AllowBackendStacking != tt.wantStack {
				t.Fatalf("AllowBackendStacking = %v, want %v", cfg.AllowBackendStacking, tt.wantStack)
			}
			if tt.warnSubstr != "" {
				joined := strings.Join(warns, " | ")
				if !strings.Contains(joined, tt.warnSubstr) {
					t.Fatalf("warnings %v should contain %q", warns, tt.warnSubstr)
				}
			}
			if len(cfg.HitStrategies) == 0 {
				t.Fatal("HitStrategies should be filled")
			}
		})
	}
}

func TestEffectiveCacheBackend(t *testing.T) {
	if got := EffectiveCacheBackend(CacheConfig{Strategy: "semantic"}); got != CacheBackendSemantic {
		t.Fatalf("got %q", got)
	}
	if got := EffectiveCacheBackend(DefaultCacheConfig()); got != CacheBackendExact {
		t.Fatalf("default effective = %q", got)
	}
}

func TestNormalizeCacheConfig_UnknownAndSemanticDefaults(t *testing.T) {
	cfg := CacheConfig{Backend: "weird-backend", Semantic: SemanticCacheConfig{}}
	warns := NormalizeCacheConfig(&cfg)
	if cfg.Backend != CacheBackendExact {
		t.Fatalf("unknown backend → exact, got %q", cfg.Backend)
	}
	joined := strings.Join(warns, " ")
	if !strings.Contains(joined, "unknown cache.backend") {
		t.Fatalf("warnings=%v", warns)
	}
	if cfg.Semantic.TopK != 5 || cfg.Semantic.Threshold != 0.8 || cfg.Semantic.DistanceType != "cosine" {
		t.Fatalf("semantic defaults: %+v", cfg.Semantic)
	}

	cfg2 := CacheConfig{Strategy: "mystery"}
	warns2 := NormalizeCacheConfig(&cfg2)
	if cfg2.Backend != CacheBackendExact {
		t.Fatalf("unknown strategy backend=%q", cfg2.Backend)
	}
	if !strings.Contains(strings.Join(warns2, " "), "unknown cache.strategy") {
		t.Fatalf("warnings=%v", warns2)
	}
}
