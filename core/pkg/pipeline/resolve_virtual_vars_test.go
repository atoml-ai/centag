package pipeline

import (
	"testing"

	"centag/core/pkg/backend"
	"centag/core/pkg/config"
)

func TestResolveVirtualVars_FallbackToBackendPreferredModel(t *testing.T) {
	mgr := backend.NewManager()
	_ = mgr.Add(&backend.BackendConfig{
		ID:      "b1",
		Name:    "B1",
		Type:    "openai",
		Enabled: true,
		SupportedModels: []backend.ModelMapping{
			{RequestedModel: "mimo-v2.5-free", ActualModel: "mimo-v2.5-free"},
		},
	})
	backend.SetManagerForTest(mgr)
	defer backend.SetManagerForTest(nil)

	prev := config.Get()
	config.Set(&config.Config{
		Proxy: config.ProxyConfig{
			DefaultBackendID: "b1",
			DefaultModel:     "",
		},
	})
	defer config.Set(prev)

	gotBackend, gotModel := ResolveVirtualVars("{{system.default_backend}}", "{{system.default_model}}")
	if gotBackend != "b1" {
		t.Fatalf("backend=%q", gotBackend)
	}
	if gotModel != "mimo-v2.5-free" {
		t.Fatalf("model=%q want mimo-v2.5-free", gotModel)
	}
}

func TestResolveVirtualVars_FallbackPlaceholders(t *testing.T) {
	prev := config.Get()
	config.Set(&config.Config{
		Proxy: config.ProxyConfig{
			DefaultBackendID:  "default-be",
			DefaultModel:      "default-model",
			FallbackBackendID: "fallback-be",
			FallbackModel:     "fallback-model",
		},
	})
	defer config.Set(prev)

	gotBackend, gotModel := ResolveVirtualVars("{{system.fallback_backend}}", "{{system.fallback_model}}")
	if gotBackend != "fallback-be" {
		t.Fatalf("backend=%q", gotBackend)
	}
	if gotModel != "fallback-model" {
		t.Fatalf("model=%q", gotModel)
	}
}

func TestResolveVirtualVars_ClassifyPlaceholders(t *testing.T) {
	prev := config.Get()
	config.Set(&config.Config{
		Proxy: config.ProxyConfig{
			DefaultBackendID: "default-be",
			DefaultModel:     "default-model",
		},
		ModelVariables: config.ModelVariables{
			SystemVariables: map[string]string{
				"system.classify_backend": "fast-be",
				"system.classify_model":   "fast-model",
			},
		},
	})
	defer config.Set(prev)

	gotBackend, gotModel := ResolveVirtualVars("{{system.classify_backend}}", "{{system.classify_model}}")
	if gotBackend != "fast-be" {
		t.Fatalf("backend=%q", gotBackend)
	}
	if gotModel != "fast-model" {
		t.Fatalf("model=%q", gotModel)
	}
}

func TestResolveVirtualVars_ClassifyPlaceholdersFallbackToDefault(t *testing.T) {
	prev := config.Get()
	config.Set(&config.Config{
		Proxy: config.ProxyConfig{
			DefaultBackendID: "default-be",
			DefaultModel:     "default-model",
		},
		ModelVariables: config.ModelVariables{
			SystemVariables: map[string]string{},
		},
	})
	defer config.Set(prev)

	// 未配置快速分类变量时回落到默认后端/模型
	gotBackend, gotModel := ResolveVirtualVars("{{system.classify_backend}}", "{{system.classify_model}}")
	if gotBackend != "default-be" {
		t.Fatalf("backend=%q", gotBackend)
	}
	if gotModel != "default-model" {
		t.Fatalf("model=%q", gotModel)
	}
}

func TestResolveVirtualVars_EmptyFallbackPicksDistinctFreeTier(t *testing.T) {
	mgr := backend.NewManager()
	_ = mgr.Add(&backend.BackendConfig{
		ID:         "zen",
		Name:       "Zen",
		Type:       "openai",
		Enabled:    true,
		ProbeModel: "deepseek-v4-flash-free",
		SupportedModels: []backend.ModelMapping{
			{RequestedModel: "deepseek-v4-flash-free", ActualModel: "deepseek-v4-flash-free"},
			{RequestedModel: "mimo-v2.5-free", ActualModel: "mimo-v2.5-free"},
		},
	})
	backend.SetManagerForTest(mgr)
	defer backend.SetManagerForTest(nil)

	prev := config.Get()
	config.Set(&config.Config{
		Proxy: config.ProxyConfig{
			DefaultBackendID: "zen",
			DefaultModel:     "deepseek-v4-flash-free",
			// FallbackModel 故意留空
		},
	})
	defer config.Set(prev)

	_, gotModel := ResolveVirtualVars("{{system.fallback_backend}}", "{{system.fallback_model}}")
	if gotModel != "mimo-v2.5-free" {
		t.Fatalf("empty fallback_model should pick distinct free tier, got %q", gotModel)
	}
}

func TestApplyResolvedModelToRawBody(t *testing.T) {
	raw := map[string]interface{}{"model": "{{system.default_model}}", "stream": false}
	applyResolvedModelToRawBody(raw, "real-model")
	if raw["model"] != "real-model" {
		t.Fatalf("got %#v", raw["model"])
	}
	applyResolvedModelToRawBody(raw, "{{system.default_model}}")
	if _, ok := raw["model"]; ok {
		t.Fatal("placeholder model should be removed")
	}
}
