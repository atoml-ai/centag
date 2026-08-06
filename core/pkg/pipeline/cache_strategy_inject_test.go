package pipeline

import (
	"testing"

	"centag/core/pkg/config"
)

func TestResolveCacheStrategyPluginInjection_DefaultOff(t *testing.T) {
	use, name := resolveCacheStrategyPluginInjection(map[string]interface{}{
		"strategy": "semantic",
	})
	if use {
		t.Fatal("expected use_strategy_plugin default false — no injection")
	}
	if name != "" {
		t.Fatalf("name should be empty when not injecting, got %q", name)
	}

	use, _ = resolveCacheStrategyPluginInjection(nil)
	if use {
		t.Fatal("nil custom config must not inject")
	}
}

func TestResolveCacheStrategyPluginInjection_OptIn(t *testing.T) {
	use, name := resolveCacheStrategyPluginInjection(map[string]interface{}{
		"use_strategy_plugin": true,
		"strategy":            "semantic",
	})
	if !use {
		t.Fatal("expected injection when use_strategy_plugin=true")
	}
	if name != "semantic" {
		t.Fatalf("strategy name = %q, want semantic", name)
	}

	use, name = resolveCacheStrategyPluginInjection(map[string]interface{}{
		"use_strategy_plugin": true,
		"cache_strategy":      "exact",
		"strategy":            "semantic",
	})
	if !use || name != "exact" {
		t.Fatalf("cache_strategy should win: use=%v name=%q", use, name)
	}
}

func TestResolveCacheStrategyPluginInjection_FallsBackToGlobalBackend(t *testing.T) {
	prev := config.Get()
	config.Set(&config.Config{Cache: config.CacheConfig{Backend: config.CacheBackendExact}})
	t.Cleanup(func() {
		config.Set(prev)
	})

	use, name := resolveCacheStrategyPluginInjection(map[string]interface{}{
		"use_strategy_plugin": true,
	})
	if !use || name != config.CacheBackendExact {
		t.Fatalf("use=%v name=%q, want exact from global backend", use, name)
	}
}
