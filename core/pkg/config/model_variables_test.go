package config

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestDefaultModelVariables(t *testing.T) {
	mv := DefaultModelVariables()
	if mv.SystemVariables == nil {
		t.Error("DefaultModelVariables() SystemVariables should be non-nil map")
	}
	if mv.UserVariables == nil {
		t.Error("DefaultModelVariables() UserVariables should be non-nil map")
	}
	if len(mv.SystemVariables) != 0 {
		t.Errorf("DefaultModelVariables() SystemVariables len = %d, want 0", len(mv.SystemVariables))
	}
	if len(mv.UserVariables) != 0 {
		t.Errorf("DefaultModelVariables() UserVariables len = %d, want 0", len(mv.UserVariables))
	}
}

func TestListSystemVariables(t *testing.T) {
	cfg := &Config{
		Proxy: ProxyConfig{
			DefaultBackendID:  "openai",
			DefaultModel:      "gpt-4o",
			FallbackBackendID: "ollama",
			FallbackModel:     "qwen2.5",
		},
		Embedding: EmbeddingConfig{
			BackendID: "ollama-local",
			Model:     "bge-m3",
		},
		ModelVariables: ModelVariables{
			SystemVariables: map[string]string{
				"system.rerank_backend": "cohere",
				"system.rerank_model":   "rerank-v3",
			},
		},
	}

	items := ListSystemVariables(cfg)

	want := map[string]string{
		"system.default_backend":  "openai",
		"system.default_model":    "gpt-4o",
		"system.fallback_backend": "ollama",
		"system.fallback_model":   "qwen2.5",
		"system.embedding_backend": "ollama-local",
		"system.embedding_model":   "bge-m3",
		"system.rerank_backend":    "cohere",
		"system.rerank_model":      "rerank-v3",
	}

	if len(items) != len(want) {
		t.Fatalf("ListSystemVariables() len = %d, want %d", len(items), len(want))
	}

	for _, it := range items {
		got, ok := want[it.Name]
		if !ok {
			t.Errorf("ListSystemVariables() unexpected item name %q", it.Name)
			continue
		}
		if it.Value != got {
			t.Errorf("ListSystemVariables() item %s value = %q, want %q", it.Name, it.Value, got)
		}
		if it.Category != "system" {
			t.Errorf("ListSystemVariables() item %s category = %q, want system", it.Name, it.Category)
		}
	}
}

func TestListSystemVariables_RerankFallback(t *testing.T) {
	cfg := &Config{
		Proxy:    ProxyConfig{DefaultBackendID: "openai"},
		Embedding: EmbeddingConfig{},
		ModelVariables: ModelVariables{SystemVariables: map[string]string{}},
	}

	items := ListSystemVariables(cfg)
	for _, it := range items {
		if it.Name == "system.rerank_backend" && it.Value != "" {
			t.Errorf("system.rerank_backend = %q, want empty when not configured", it.Value)
		}
		if it.Name == "system.rerank_model" && it.Value != "" {
			t.Errorf("system.rerank_model = %q, want empty when not configured", it.Value)
		}
	}
}

func TestListSystemVariables_NilConfig(t *testing.T) {
	if got := ListSystemVariables(nil); got != nil {
		t.Errorf("ListSystemVariables(nil) = %v, want nil", got)
	}
}

func TestListSystemVariables_NilSystemVars(t *testing.T) {
	cfg := &Config{
		Proxy:          ProxyConfig{DefaultBackendID: "openai"},
		Embedding:      EmbeddingConfig{},
		ModelVariables: ModelVariables{SystemVariables: nil},
	}
	// Should not panic and rerank defaults to empty.
	items := ListSystemVariables(cfg)
	found := false
	for _, it := range items {
		if it.Name == "system.rerank_backend" {
			found = true
			if it.Value != "" {
				t.Errorf("system.rerank_backend = %q, want empty", it.Value)
			}
		}
	}
	if !found {
		t.Error("ListSystemVariables() missing system.rerank_backend item")
	}
}

func TestListUserVariables(t *testing.T) {
	cfg := &Config{
		ModelVariables: ModelVariables{
			UserVariables: map[string]string{
				"my_api_key": "sk-123",
				"my_region":  "cn-east",
			},
		},
	}

	items := ListUserVariables(cfg)
	if len(items) != 2 {
		t.Fatalf("ListUserVariables() len = %d, want 2", len(items))
	}

	byName := map[string]ModelVariableItem{}
	for _, it := range items {
		byName[it.Name] = it
	}

	if v := byName["my_api_key"]; v.Value != "sk-123" || v.Category != "user" {
		t.Errorf("ListUserVariables() my_api_key = %+v, want value sk-123 category user", v)
	}
	if v := byName["my_region"]; v.Value != "cn-east" || v.Category != "user" {
		t.Errorf("ListUserVariables() my_region = %+v, want value cn-east category user", v)
	}
}

func TestListUserVariables_NilConfig(t *testing.T) {
	if got := ListUserVariables(nil); got != nil {
		t.Errorf("ListUserVariables(nil) = %v, want nil", got)
	}
}

func TestListUserVariables_Empty(t *testing.T) {
	cfg := &Config{
		ModelVariables: ModelVariables{UserVariables: map[string]string{}},
	}
	if got := ListUserVariables(cfg); len(got) != 0 {
		t.Errorf("ListUserVariables() len = %d, want 0", len(got))
	}
}

func TestModelVariables_RoundTripJSON(t *testing.T) {
	mv := ModelVariables{
		SystemVariables: map[string]string{"system.rerank_backend": "cohere"},
		UserVariables:   map[string]string{"a": "b"},
	}

	b, err := json.Marshal(mv)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var back ModelVariables
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !reflect.DeepEqual(mv, back) {
		t.Errorf("RoundTrip = %+v, want %+v", back, mv)
	}
}
