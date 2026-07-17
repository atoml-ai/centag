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
