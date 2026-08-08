package pipeline

import (
	"context"
	"errors"
	"testing"

	"centag/core/pkg/backend"
	"centag/core/pkg/plugin"
)

func TestDefaultLLMProvider_CreateClient_NoBackend(t *testing.T) {
	mgr := backend.NewManager()
	pm := plugin.NewManager()
	provider := NewDefaultLLMProvider(mgr, pm)

	_, err := provider.CreateClient(context.Background(), "missing-backend", "gpt-4o-mini")
	if err == nil {
		t.Fatal("expected error when backend missing")
	}
	ce := backend.ClassifyClientError(err)
	if ce == nil || ce.Code != backend.ErrorCodeNoBackendConfigured {
		t.Fatalf("ClassifyClientError = %#v, want no_backend_configured", ce)
	}
	if !errors.Is(err, backend.ErrNoUsableBackend) && ce.Code != backend.ErrorCodeNoBackendConfigured {
		t.Fatalf("error should be configuration class: %v", err)
	}
}

func TestDefaultLLMProvider_CreateClient_MissingAPIKey(t *testing.T) {
	mgr := backend.NewManager()
	if err := mgr.Add(&backend.BackendConfig{
		ID:      "openai",
		Type:    "openai",
		BaseURL: "https://api.openai.com/v1",
		Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	provider := NewDefaultLLMProvider(mgr, plugin.NewManager())

	_, err := provider.CreateClient(context.Background(), "openai", "gpt-4o-mini")
	if err == nil {
		t.Fatal("expected error when api key missing")
	}
	ce := backend.ClassifyClientError(err)
	if ce == nil || ce.Code != backend.ErrorCodeNoBackendAPIKey {
		t.Fatalf("ClassifyClientError = %#v, want no_backend_api_key", ce)
	}
}

func TestDefaultLLMProvider_CreateClient_NilManager(t *testing.T) {
	provider := NewDefaultLLMProvider(nil, plugin.NewManager())
	_, err := provider.CreateClient(context.Background(), "x", "m")
	ce := backend.ClassifyClientError(err)
	if ce == nil || ce.Code != backend.ErrorCodeNoBackendConfigured {
		t.Fatalf("got %#v", ce)
	}
}
