package pipeline

import (
	"testing"

	"centag/core/pkg/config"
)

func TestTemplateVarResolver_SystemVariables(t *testing.T) {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{
			DefaultBackendID:  "openai",
			DefaultModel:      "gpt-4o",
			FallbackBackendID: "ollama",
			FallbackModel:     "qwen2.5",
		},
		Embedding: config.EmbeddingConfig{
			BackendID: "ollama-local",
			Model:     "bge-m3",
		},
		ModelVariables: config.ModelVariables{
			SystemVariables: map[string]string{
				"system.rerank_backend": "cohere",
				"system.rerank_model":   "rerank-v3",
			},
		},
	}
	prev := config.Get()
	config.Set(cfg)
	defer config.Set(prev)

	r := NewTemplateVarResolver(&NodeInput{Content: "hello"}, nil)

	tests := []struct {
		path    string
		want    interface{}
		wantErr bool
	}{
		{path: "system.default_backend", want: "openai"},
		{path: "system.default_model", want: "gpt-4o"},
		{path: "system.fallback_backend", want: "ollama"},
		{path: "system.fallback_model", want: "qwen2.5"},
		{path: "system.embedding_backend", want: "ollama-local"},
		{path: "system.embedding_model", want: "bge-m3"},
		{path: "system.rerank_backend", want: "cohere"},
		{path: "system.rerank_model", want: "rerank-v3"},
		{path: "system.unknown_field", wantErr: true},
		{path: "system", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got, err := r.Resolve(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Resolve(%q) want error, got %v", tt.path, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Resolve(%q) error = %v", tt.path, err)
			}
			if got != tt.want {
				t.Errorf("Resolve(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestTemplateVarResolver_SystemRerankEmpty(t *testing.T) {
	cfg := &config.Config{
		Proxy:          config.ProxyConfig{DefaultBackendID: "openai"},
		Embedding:      config.EmbeddingConfig{},
		ModelVariables: config.ModelVariables{SystemVariables: nil},
	}
	prev := config.Get()
	config.Set(cfg)
	defer config.Set(prev)

	r := NewTemplateVarResolver(&NodeInput{Content: "hello"}, nil)

	for _, path := range []string{"system.rerank_backend", "system.rerank_model"} {
		got, err := r.Resolve(path)
		if err != nil {
			t.Fatalf("Resolve(%q) error = %v", path, err)
		}
		if got != "" {
			t.Errorf("Resolve(%q) = %q, want empty string", path, got)
		}
	}
}

func TestTemplateVarResolver_SystemNilConfig(t *testing.T) {
	prev := config.Get()
	config.Set(nil)
	defer config.Set(prev)

	r := NewTemplateVarResolver(&NodeInput{Content: "hello"}, nil)
	if _, err := r.Resolve("system.default_backend"); err == nil {
		t.Error("Resolve(system.default_backend) want error when config nil")
	}
}
